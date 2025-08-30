// Package loader provides MIB file loading and management functionality.
package loader

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/geekxflood/common/config"
)

// MIBFile represents a loaded MIB file
type MIBFile struct {
	Path         string    `json:"path"`
	Name         string    `json:"name"`
	Content      []byte    `json:"-"` // Don't serialize content
	Size         int64     `json:"size"`
	ModTime      time.Time `json:"mod_time"`
	LastLoaded   time.Time `json:"last_loaded"`
	LoadCount    int       `json:"load_count"`
	ParsedOK     bool      `json:"parsed_ok"`
	ParseError   string    `json:"parse_error,omitempty"`
	Dependencies []string  `json:"dependencies,omitempty"`
}

// LoaderConfig holds configuration for the MIB loader
type LoaderConfig struct {
	MIBDirectories    []string      `json:"mib_directories"`
	FileExtensions    []string      `json:"file_extensions"`
	MaxFileSize       int64         `json:"max_file_size"`
	EnableHotReload   bool          `json:"enable_hot_reload"`
	CacheEnabled      bool          `json:"cache_enabled"`
	CacheExpiry       time.Duration `json:"cache_expiry"`
	RecursiveScan     bool          `json:"recursive_scan"`
	IgnorePatterns    []string      `json:"ignore_patterns"`
	RequiredMIBs      []string      `json:"required_mibs"`
	ValidationEnabled bool          `json:"validation_enabled"`
}

// DefaultLoaderConfig returns a default loader configuration
func DefaultLoaderConfig() *LoaderConfig {
	return &LoaderConfig{
		MIBDirectories:    []string{"/usr/share/snmp/mibs", "./mibs", "/etc/snmp/mibs"},
		FileExtensions:    []string{".mib", ".txt", ".my"},
		MaxFileSize:       10 * 1024 * 1024, // 10MB
		EnableHotReload:   true,
		CacheEnabled:      true,
		CacheExpiry:       24 * time.Hour,
		RecursiveScan:     true,
		IgnorePatterns:    []string{".*", "_*", "*.bak", "*.tmp"},
		RequiredMIBs:      []string{"SNMPv2-SMI", "SNMPv2-TC", "SNMPv2-MIB"},
		ValidationEnabled: true,
	}
}

// Loader manages MIB file loading and caching
type Loader struct {
	config   *LoaderConfig
	files    map[string]*MIBFile
	watcher  *fsnotify.Watcher
	mu       sync.RWMutex
	stats    *LoaderStats
	handlers []ReloadHandler
}

// LoaderStats tracks loader statistics
type LoaderStats struct {
	FilesLoaded        int           `json:"files_loaded"`
	FilesWatched       int           `json:"files_watched"`
	TotalSize          int64         `json:"total_size"`
	LastScanTime       time.Time     `json:"last_scan_time"`
	ScanDuration       time.Duration `json:"scan_duration"`
	ParseErrors        int           `json:"parse_errors"`
	CacheHits          int           `json:"cache_hits"`
	CacheMisses        int           `json:"cache_misses"`
	ReloadEvents       int           `json:"reload_events"`
	DirectoriesScanned int           `json:"directories_scanned"`
}

// ReloadHandler is called when MIB files are reloaded
type ReloadHandler func(files []*MIBFile) error

// NewLoader creates a new MIB loader with the given configuration
func NewLoader(cfg config.Provider) (*Loader, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration provider cannot be nil")
	}

	// Load configuration
	loaderConfig := DefaultLoaderConfig()

	// Override with config values if available
	if dirs, err := cfg.GetStringSlice("mib.directories"); err == nil {
		loaderConfig.MIBDirectories = dirs
	}

	if exts, err := cfg.GetStringSlice("mib.file_extensions"); err == nil {
		loaderConfig.FileExtensions = exts
	}

	if size, err := cfg.GetInt("mib.max_file_size", int(loaderConfig.MaxFileSize)); err == nil {
		loaderConfig.MaxFileSize = int64(size)
	}

	if reload, err := cfg.GetBool("mib.enable_hot_reload", loaderConfig.EnableHotReload); err == nil {
		loaderConfig.EnableHotReload = reload
	}

	if cache, err := cfg.GetBool("mib.cache_enabled", loaderConfig.CacheEnabled); err == nil {
		loaderConfig.CacheEnabled = cache
	}

	if expiry, err := cfg.GetDuration("mib.cache_expiry", loaderConfig.CacheExpiry); err == nil {
		loaderConfig.CacheExpiry = expiry
	}

	loader := &Loader{
		config: loaderConfig,
		files:  make(map[string]*MIBFile),
		stats:  &LoaderStats{},
	}

	// Initialize file watcher if hot reload is enabled
	if loaderConfig.EnableHotReload {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			return nil, fmt.Errorf("failed to create file watcher: %w", err)
		}
		loader.watcher = watcher
		go loader.watchFiles()
	}

	return loader, nil
}

// LoadAll scans all configured directories and loads MIB files
func (l *Loader) LoadAll() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	startTime := time.Now()
	l.stats.DirectoriesScanned = 0
	l.stats.FilesLoaded = 0
	l.stats.ParseErrors = 0

	for _, dir := range l.config.MIBDirectories {
		if err := l.scanDirectory(dir); err != nil {
			// Log error but continue with other directories
			continue
		}
		l.stats.DirectoriesScanned++
	}

	l.stats.LastScanTime = time.Now()
	l.stats.ScanDuration = time.Since(startTime)

	// Validate required MIBs are present
	if l.config.ValidationEnabled {
		if err := l.validateRequiredMIBs(); err != nil {
			return fmt.Errorf("required MIB validation failed: %w", err)
		}
	}

	return nil
}

// scanDirectory recursively scans a directory for MIB files
func (l *Loader) scanDirectory(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("directory does not exist: %s", dir)
	}

	walkFunc := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories unless recursive scan is enabled
		if d.IsDir() {
			if !l.config.RecursiveScan && path != dir {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file should be ignored
		if l.shouldIgnoreFile(path) {
			return nil
		}

		// Check file extension
		if !l.hasValidExtension(path) {
			return nil
		}

		// Load the file
		if err := l.loadFile(path); err != nil {
			l.stats.ParseErrors++
			// Log error but continue processing
			return nil
		}

		l.stats.FilesLoaded++
		return nil
	}

	return filepath.WalkDir(dir, walkFunc)
}

// loadFile loads a single MIB file
func (l *Loader) loadFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat file %s: %w", path, err)
	}

	// Check file size
	if info.Size() > l.config.MaxFileSize {
		return fmt.Errorf("file %s exceeds maximum size limit", path)
	}

	// Check if file is already loaded and up-to-date
	if existing, exists := l.files[path]; exists && l.config.CacheEnabled {
		if existing.ModTime.Equal(info.ModTime()) && time.Since(existing.LastLoaded) < l.config.CacheExpiry {
			l.stats.CacheHits++
			return nil
		}
	}

	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", path, err)
	}

	// Create MIB file entry
	mibFile := &MIBFile{
		Path:       path,
		Name:       l.extractMIBName(path, content),
		Content:    content,
		Size:       info.Size(),
		ModTime:    info.ModTime(),
		LastLoaded: time.Now(),
		LoadCount:  1,
		ParsedOK:   true,
	}

	// Update existing file's load count
	if existing, exists := l.files[path]; exists {
		mibFile.LoadCount = existing.LoadCount + 1
	}

	// Basic validation
	if l.config.ValidationEnabled {
		if err := l.validateMIBContent(content); err != nil {
			mibFile.ParsedOK = false
			mibFile.ParseError = err.Error()
		}
	}

	l.files[path] = mibFile
	l.stats.TotalSize += info.Size()
	l.stats.CacheMisses++

	// Add to file watcher
	if l.watcher != nil {
		l.watcher.Add(path)
		l.stats.FilesWatched++
	}

	return nil
}

// extractMIBName extracts the MIB name from file path or content
func (l *Loader) extractMIBName(path string, content []byte) string {
	// Try to extract from filename first
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	// Clean up common prefixes/suffixes
	name = strings.TrimPrefix(name, "rfc")
	name = strings.TrimSuffix(name, "-mib")
	name = strings.TrimSuffix(name, "_mib")

	return strings.ToUpper(name)
}

// validateMIBContent performs basic validation of MIB content
func (l *Loader) validateMIBContent(content []byte) error {
	contentStr := string(content)

	// Check for basic MIB structure
	if !strings.Contains(contentStr, "DEFINITIONS") {
		return fmt.Errorf("missing DEFINITIONS keyword")
	}

	if !strings.Contains(contentStr, "BEGIN") {
		return fmt.Errorf("missing BEGIN keyword")
	}

	if !strings.Contains(contentStr, "END") {
		return fmt.Errorf("missing END keyword")
	}

	return nil
}

// shouldIgnoreFile checks if a file should be ignored based on patterns
func (l *Loader) shouldIgnoreFile(path string) bool {
	filename := filepath.Base(path)

	for _, pattern := range l.config.IgnorePatterns {
		if matched, _ := filepath.Match(pattern, filename); matched {
			return true
		}
	}

	return false
}

// hasValidExtension checks if file has a valid MIB extension
func (l *Loader) hasValidExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))

	for _, validExt := range l.config.FileExtensions {
		if ext == strings.ToLower(validExt) {
			return true
		}
	}

	return false
}

// validateRequiredMIBs ensures all required MIBs are loaded
func (l *Loader) validateRequiredMIBs() error {
	missing := []string{}

	for _, required := range l.config.RequiredMIBs {
		found := false
		for _, file := range l.files {
			if strings.EqualFold(file.Name, required) && file.ParsedOK {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, required)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required MIBs: %v", missing)
	}

	return nil
}

// watchFiles monitors MIB files for changes
func (l *Loader) watchFiles() {
	if l.watcher == nil {
		return
	}

	for {
		select {
		case event, ok := <-l.watcher.Events:
			if !ok {
				return
			}

			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
				l.handleFileChange(event.Name)
			} else if event.Op&fsnotify.Remove == fsnotify.Remove {
				l.handleFileRemoval(event.Name)
			}

		case err, ok := <-l.watcher.Errors:
			if !ok {
				return
			}
			// Log error but continue watching
			_ = err
		}
	}
}

// handleFileChange processes file change events
func (l *Loader) handleFileChange(path string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Reload the file
	if err := l.loadFile(path); err != nil {
		// Log error but continue
		return
	}

	l.stats.ReloadEvents++

	// Notify handlers
	l.notifyHandlers()
}

// handleFileRemoval processes file removal events
func (l *Loader) handleFileRemoval(path string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.files, path)
	l.stats.ReloadEvents++

	// Notify handlers
	l.notifyHandlers()
}

// notifyHandlers calls all registered reload handlers
func (l *Loader) notifyHandlers() {
	files := make([]*MIBFile, 0, len(l.files))
	for _, file := range l.files {
		if file.ParsedOK {
			files = append(files, file)
		}
	}

	for _, handler := range l.handlers {
		go func(h ReloadHandler) {
			_ = h(files)
		}(handler)
	}
}

// AddReloadHandler registers a handler for MIB reload events
func (l *Loader) AddReloadHandler(handler ReloadHandler) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.handlers = append(l.handlers, handler)
}

// GetFile returns a loaded MIB file by path
func (l *Loader) GetFile(path string) (*MIBFile, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	file, exists := l.files[path]
	return file, exists
}

// GetFileByName returns a loaded MIB file by name
func (l *Loader) GetFileByName(name string) (*MIBFile, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for _, file := range l.files {
		if strings.EqualFold(file.Name, name) && file.ParsedOK {
			return file, true
		}
	}
	return nil, false
}

// GetAllFiles returns all loaded MIB files
func (l *Loader) GetAllFiles() []*MIBFile {
	l.mu.RLock()
	defer l.mu.RUnlock()

	files := make([]*MIBFile, 0, len(l.files))
	for _, file := range l.files {
		files = append(files, file)
	}
	return files
}

// GetValidFiles returns only successfully parsed MIB files
func (l *Loader) GetValidFiles() []*MIBFile {
	l.mu.RLock()
	defer l.mu.RUnlock()

	files := make([]*MIBFile, 0, len(l.files))
	for _, file := range l.files {
		if file.ParsedOK {
			files = append(files, file)
		}
	}
	return files
}

// GetStats returns loader statistics
func (l *Loader) GetStats() *LoaderStats {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// Create a copy to avoid race conditions
	stats := *l.stats
	return &stats
}

// Reload forces a reload of all MIB files
func (l *Loader) Reload() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Clear existing files
	l.files = make(map[string]*MIBFile)
	l.stats.TotalSize = 0
	l.stats.FilesWatched = 0

	// Reload all files
	return l.LoadAll()
}

// Close shuts down the loader and releases resources
func (l *Loader) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.watcher != nil {
		return l.watcher.Close()
	}
	return nil
}

// GetConfig returns the loader configuration
func (l *Loader) GetConfig() *LoaderConfig {
	return l.config
}
