// Package reload provides hot reload functionality for configuration and MIB files
package reload

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/geekxflood/common/config"
	"github.com/geekxflood/common/logging"
)

// ReloadEvent represents a reload event
type ReloadEvent struct {
	Type      ReloadType    `json:"type"`
	Source    string        `json:"source"`
	Timestamp time.Time     `json:"timestamp"`
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration"`
}

// ReloadType defines the type of reload event
type ReloadType string

const (
	ReloadTypeConfig  ReloadType = "config"
	ReloadTypeMIB     ReloadType = "mib"
	ReloadTypeWebhook ReloadType = "webhook"
	ReloadTypeAll     ReloadType = "all"
)

// ReloadHandler is a function that handles reload events
type ReloadHandler func(event ReloadEvent) error

// ComponentReloader defines the interface for components that support hot reload
type ComponentReloader interface {
	Reload(configProvider config.Provider) error
	GetReloadStats() map[string]any
}

// ReloadConfig holds configuration for the reload manager
type ReloadConfig struct {
	Enabled              bool          `json:"enabled"`
	ConfigFile           string        `json:"config_file"`
	WatchConfigFile      bool          `json:"watch_config_file"`
	WatchMIBDirectories  bool          `json:"watch_mib_directories"`
	ReloadDelay          time.Duration `json:"reload_delay"`
	MaxReloadAttempts    int           `json:"max_reload_attempts"`
	ReloadTimeout        time.Duration `json:"reload_timeout"`
	PreserveState        bool          `json:"preserve_state"`
	ValidateBeforeReload bool          `json:"validate_before_reload"`
}

// DefaultReloadConfig returns a default reload configuration
func DefaultReloadConfig() *ReloadConfig {
	return &ReloadConfig{
		Enabled:              true,
		WatchConfigFile:      true,
		WatchMIBDirectories:  true,
		ReloadDelay:          2 * time.Second,
		MaxReloadAttempts:    3,
		ReloadTimeout:        30 * time.Second,
		PreserveState:        true,
		ValidateBeforeReload: true,
	}
}

// ReloadManager manages hot reload functionality
type ReloadManager struct {
	config           *ReloadConfig
	logger           logging.Logger
	configManager    config.Manager
	configProvider   config.Provider
	watcher          *fsnotify.Watcher
	components       map[string]ComponentReloader
	handlers         []ReloadHandler
	events           []ReloadEvent
	stats            *ReloadStats
	mu               sync.RWMutex
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	lastReloadTime   time.Time
	reloadInProgress bool
}

// ReloadStats tracks reload statistics
type ReloadStats struct {
	TotalReloads       int64         `json:"total_reloads"`
	SuccessfulReloads  int64         `json:"successful_reloads"`
	FailedReloads      int64         `json:"failed_reloads"`
	LastReloadTime     time.Time     `json:"last_reload_time"`
	LastReloadDuration time.Duration `json:"last_reload_duration"`
	AverageReloadTime  time.Duration `json:"average_reload_time"`
	ConfigReloads      int64         `json:"config_reloads"`
	MIBReloads         int64         `json:"mib_reloads"`
	WebhookReloads     int64         `json:"webhook_reloads"`
}

// NewReloadManager creates a new reload manager
func NewReloadManager(configManager config.Manager, logger logging.Logger) (*ReloadManager, error) {
	if configManager == nil {
		return nil, fmt.Errorf("config manager cannot be nil")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	configProvider := configManager.(config.Provider)
	reloadConfig := DefaultReloadConfig()

	// Load reload configuration
	if enabled, err := configProvider.GetBool("reload.enabled", reloadConfig.Enabled); err == nil {
		reloadConfig.Enabled = enabled
	}

	if watchConfig, err := configProvider.GetBool("reload.watch_config_file", reloadConfig.WatchConfigFile); err == nil {
		reloadConfig.WatchConfigFile = watchConfig
	}

	if watchMIB, err := configProvider.GetBool("reload.watch_mib_directories", reloadConfig.WatchMIBDirectories); err == nil {
		reloadConfig.WatchMIBDirectories = watchMIB
	}

	if delay, err := configProvider.GetDuration("reload.reload_delay", reloadConfig.ReloadDelay); err == nil {
		reloadConfig.ReloadDelay = delay
	}

	if attempts, err := configProvider.GetInt("reload.max_reload_attempts", reloadConfig.MaxReloadAttempts); err == nil {
		reloadConfig.MaxReloadAttempts = attempts
	}

	if timeout, err := configProvider.GetDuration("reload.reload_timeout", reloadConfig.ReloadTimeout); err == nil {
		reloadConfig.ReloadTimeout = timeout
	}

	if preserve, err := configProvider.GetBool("reload.preserve_state", reloadConfig.PreserveState); err == nil {
		reloadConfig.PreserveState = preserve
	}

	if validate, err := configProvider.GetBool("reload.validate_before_reload", reloadConfig.ValidateBeforeReload); err == nil {
		reloadConfig.ValidateBeforeReload = validate
	}

	ctx, cancel := context.WithCancel(context.Background())

	manager := &ReloadManager{
		config:         reloadConfig,
		logger:         logger.With("component", "reload"),
		configManager:  configManager,
		configProvider: configProvider,
		components:     make(map[string]ComponentReloader),
		handlers:       make([]ReloadHandler, 0),
		events:         make([]ReloadEvent, 0),
		stats:          &ReloadStats{},
		ctx:            ctx,
		cancel:         cancel,
	}

	// Initialize file watcher if enabled
	if reloadConfig.Enabled {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to create file watcher: %w", err)
		}
		manager.watcher = watcher
	}

	return manager, nil
}

// Start starts the reload manager
func (rm *ReloadManager) Start() error {
	if !rm.config.Enabled {
		rm.logger.Info("Hot reload is disabled")
		return nil
	}

	rm.logger.Info("Starting reload manager",
		"watch_config", rm.config.WatchConfigFile,
		"watch_mib", rm.config.WatchMIBDirectories,
		"reload_delay", rm.config.ReloadDelay)

	// Start file watcher
	if rm.watcher != nil {
		rm.wg.Add(1)
		go rm.watchFiles()

		// Watch configuration file if specified
		if rm.config.WatchConfigFile && rm.config.ConfigFile != "" {
			if err := rm.watcher.Add(rm.config.ConfigFile); err != nil {
				rm.logger.Warn("Failed to watch config file", "file", rm.config.ConfigFile, "error", err.Error())
			} else {
				rm.logger.Info("Watching configuration file", "file", rm.config.ConfigFile)
			}
		}
	}

	rm.logger.Info("Reload manager started successfully")
	return nil
}

// Stop stops the reload manager
func (rm *ReloadManager) Stop() error {
	if !rm.config.Enabled {
		return nil
	}

	rm.logger.Info("Stopping reload manager")

	// Cancel context to stop background goroutines
	rm.cancel()

	// Close file watcher
	if rm.watcher != nil {
		if err := rm.watcher.Close(); err != nil {
			rm.logger.Error("Error closing file watcher", "error", err.Error())
		}
	}

	// Wait for all goroutines to finish
	rm.wg.Wait()

	rm.logger.Info("Reload manager stopped")
	return nil
}

// RegisterComponent registers a component for hot reload
func (rm *ReloadManager) RegisterComponent(name string, component ComponentReloader) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.components[name] = component
	rm.logger.Info("Registered component for hot reload", "component", name)
}

// UnregisterComponent unregisters a component from hot reload
func (rm *ReloadManager) UnregisterComponent(name string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	delete(rm.components, name)
	rm.logger.Info("Unregistered component from hot reload", "component", name)
}

// AddHandler adds a reload event handler
func (rm *ReloadManager) AddHandler(handler ReloadHandler) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.handlers = append(rm.handlers, handler)
}

// SetConfigFile sets the configuration file to watch
func (rm *ReloadManager) SetConfigFile(configFile string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.config.ConfigFile = configFile
	rm.logger.Info("Set configuration file for watching", "file", configFile)
}

// TriggerReload manually triggers a reload of the specified type
func (rm *ReloadManager) TriggerReload(reloadType ReloadType, source string) error {
	if !rm.config.Enabled {
		return fmt.Errorf("hot reload is disabled")
	}

	rm.logger.Info("Manual reload triggered", "type", reloadType, "source", source)
	return rm.performReload(reloadType, source)
}

// GetStats returns reload statistics
func (rm *ReloadManager) GetStats() *ReloadStats {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	// Create a copy to avoid race conditions
	stats := *rm.stats
	return &stats
}

// GetRecentEvents returns recent reload events
func (rm *ReloadManager) GetRecentEvents(limit int) []ReloadEvent {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if limit <= 0 || limit > len(rm.events) {
		limit = len(rm.events)
	}

	// Return the most recent events
	start := len(rm.events) - limit
	events := make([]ReloadEvent, limit)
	copy(events, rm.events[start:])
	return events
}

// IsReloadInProgress returns whether a reload is currently in progress
func (rm *ReloadManager) IsReloadInProgress() bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.reloadInProgress
}

// watchFiles monitors files for changes
func (rm *ReloadManager) watchFiles() {
	defer rm.wg.Done()

	debounceTimer := time.NewTimer(0)
	if !debounceTimer.Stop() {
		<-debounceTimer.C
	}

	pendingReloads := make(map[string]ReloadType)

	for {
		select {
		case <-rm.ctx.Done():
			return

		case event, ok := <-rm.watcher.Events:
			if !ok {
				return
			}

			rm.logger.Debug("File system event received",
				"file", event.Name,
				"operation", event.Op.String())

			// Determine reload type based on file
			reloadType := rm.determineReloadType(event.Name)
			if reloadType == "" {
				continue
			}

			// Add to pending reloads (debounce multiple events)
			pendingReloads[event.Name] = reloadType

			// Reset debounce timer
			debounceTimer.Reset(rm.config.ReloadDelay)

		case <-debounceTimer.C:
			// Process pending reloads
			if len(pendingReloads) > 0 {
				rm.processPendingReloads(pendingReloads)
				pendingReloads = make(map[string]ReloadType)
			}

		case err, ok := <-rm.watcher.Errors:
			if !ok {
				return
			}
			rm.logger.Error("File watcher error", "error", err.Error())
		}
	}
}

// determineReloadType determines the reload type based on file path
func (rm *ReloadManager) determineReloadType(filePath string) ReloadType {
	// Check if it's the config file
	if rm.config.ConfigFile != "" && filePath == rm.config.ConfigFile {
		return ReloadTypeConfig
	}

	// Check if it's a MIB file (basic check - could be enhanced)
	if rm.config.WatchMIBDirectories {
		// Simple heuristic: files with .mib, .txt extensions in MIB directories
		if isMIBFile(filePath) {
			return ReloadTypeMIB
		}
	}

	return ""
}

// isMIBFile checks if a file is likely a MIB file
func isMIBFile(filePath string) bool {
	// Simple check for common MIB file extensions
	extensions := []string{".mib", ".txt", ".my"}
	for _, ext := range extensions {
		if len(filePath) > len(ext) && filePath[len(filePath)-len(ext):] == ext {
			return true
		}
	}
	return false
}

// processPendingReloads processes all pending reload events
func (rm *ReloadManager) processPendingReloads(pendingReloads map[string]ReloadType) {
	// Group by reload type
	reloadTypes := make(map[ReloadType][]string)
	for file, reloadType := range pendingReloads {
		reloadTypes[reloadType] = append(reloadTypes[reloadType], file)
	}

	// Process each reload type
	for reloadType, files := range reloadTypes {
		source := fmt.Sprintf("files: %v", files)
		if err := rm.performReload(reloadType, source); err != nil {
			rm.logger.Error("Failed to perform reload",
				"type", reloadType,
				"source", source,
				"error", err.Error())
		}
	}
}

// performReload performs the actual reload operation
func (rm *ReloadManager) performReload(reloadType ReloadType, source string) error {
	rm.mu.Lock()
	if rm.reloadInProgress {
		rm.mu.Unlock()
		return fmt.Errorf("reload already in progress")
	}
	rm.reloadInProgress = true
	rm.mu.Unlock()

	defer func() {
		rm.mu.Lock()
		rm.reloadInProgress = false
		rm.mu.Unlock()
	}()

	startTime := time.Now()
	event := ReloadEvent{
		Type:      reloadType,
		Source:    source,
		Timestamp: startTime,
	}

	rm.logger.Info("Starting reload", "type", reloadType, "source", source)

	var err error
	switch reloadType {
	case ReloadTypeConfig:
		err = rm.reloadConfiguration()
	case ReloadTypeMIB:
		err = rm.reloadMIBFiles()
	case ReloadTypeWebhook:
		err = rm.reloadWebhookConfiguration()
	case ReloadTypeAll:
		err = rm.reloadAll()
	default:
		err = fmt.Errorf("unknown reload type: %s", reloadType)
	}

	// Update event with results
	event.Duration = time.Since(startTime)
	event.Success = err == nil
	if err != nil {
		event.Error = err.Error()
	}

	// Update statistics
	rm.updateStats(event)

	// Record event
	rm.recordEvent(event)

	// Notify handlers
	rm.notifyHandlers(event)

	if err != nil {
		rm.logger.Error("Reload failed",
			"type", reloadType,
			"source", source,
			"duration", event.Duration,
			"error", err.Error())
		return err
	}

	rm.logger.Info("Reload completed successfully",
		"type", reloadType,
		"source", source,
		"duration", event.Duration)

	return nil
}

// reloadConfiguration reloads the application configuration
func (rm *ReloadManager) reloadConfiguration() error {
	rm.logger.Info("Reloading configuration")

	// Validate configuration before reloading if enabled
	if rm.config.ValidateBeforeReload {
		if err := rm.configManager.Validate(); err != nil {
			return fmt.Errorf("configuration validation failed: %w", err)
		}
	}

	// Reload configuration
	if err := rm.configManager.Reload(); err != nil {
		return fmt.Errorf("failed to reload configuration: %w", err)
	}

	// Reload all registered components with new configuration
	return rm.reloadComponents()
}

// reloadMIBFiles reloads MIB files
func (rm *ReloadManager) reloadMIBFiles() error {
	rm.logger.Info("Reloading MIB files")

	// Find MIB-related components and reload them
	for name, component := range rm.components {
		if isMIBComponent(name) {
			if err := component.Reload(rm.configProvider); err != nil {
				return fmt.Errorf("failed to reload MIB component %s: %w", name, err)
			}
		}
	}

	return nil
}

// reloadWebhookConfiguration reloads webhook configuration
func (rm *ReloadManager) reloadWebhookConfiguration() error {
	rm.logger.Info("Reloading webhook configuration")

	// Find webhook-related components and reload them
	for name, component := range rm.components {
		if isWebhookComponent(name) {
			if err := component.Reload(rm.configProvider); err != nil {
				return fmt.Errorf("failed to reload webhook component %s: %w", name, err)
			}
		}
	}

	return nil
}

// reloadAll reloads all components
func (rm *ReloadManager) reloadAll() error {
	rm.logger.Info("Reloading all components")

	// First reload configuration
	if err := rm.reloadConfiguration(); err != nil {
		return err
	}

	return nil
}

// reloadComponents reloads all registered components
func (rm *ReloadManager) reloadComponents() error {
	for name, component := range rm.components {
		rm.logger.Debug("Reloading component", "component", name)
		if err := component.Reload(rm.configProvider); err != nil {
			return fmt.Errorf("failed to reload component %s: %w", name, err)
		}
	}
	return nil
}

// Helper functions to identify component types
func isMIBComponent(name string) bool {
	mibComponents := []string{"mib_loader", "mib_parser", "resolver"}
	for _, comp := range mibComponents {
		if name == comp {
			return true
		}
	}
	return false
}

func isWebhookComponent(name string) bool {
	webhookComponents := []string{"notifier", "http_client"}
	for _, comp := range webhookComponents {
		if name == comp {
			return true
		}
	}
	return false
}

// updateStats updates reload statistics
func (rm *ReloadManager) updateStats(event ReloadEvent) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.stats.TotalReloads++
	if event.Success {
		rm.stats.SuccessfulReloads++
	} else {
		rm.stats.FailedReloads++
	}

	rm.stats.LastReloadTime = event.Timestamp
	rm.stats.LastReloadDuration = event.Duration

	// Update average reload time
	if rm.stats.SuccessfulReloads > 0 {
		totalTime := rm.stats.AverageReloadTime * time.Duration(rm.stats.SuccessfulReloads-1)
		if event.Success {
			totalTime += event.Duration
			rm.stats.AverageReloadTime = totalTime / time.Duration(rm.stats.SuccessfulReloads)
		}
	}

	// Update type-specific counters
	switch event.Type {
	case ReloadTypeConfig:
		rm.stats.ConfigReloads++
	case ReloadTypeMIB:
		rm.stats.MIBReloads++
	case ReloadTypeWebhook:
		rm.stats.WebhookReloads++
	}
}

// recordEvent records a reload event
func (rm *ReloadManager) recordEvent(event ReloadEvent) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.events = append(rm.events, event)

	// Keep only the last 100 events to prevent memory growth
	if len(rm.events) > 100 {
		rm.events = rm.events[len(rm.events)-100:]
	}
}

// notifyHandlers notifies all registered handlers of a reload event
func (rm *ReloadManager) notifyHandlers(event ReloadEvent) {
	for _, handler := range rm.handlers {
		go func(h ReloadHandler) {
			if err := h(event); err != nil {
				rm.logger.Error("Reload handler error", "error", err.Error())
			}
		}(handler)
	}
}
