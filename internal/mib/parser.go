// Package mib provides consolidated MIB loading, parsing, and OID resolution functionality.
package mib

import (
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/geekxflood/common/config"
	"github.com/geekxflood/nereus/internal/types"
)

// OIDNode represents a node in the OID tree
type OIDNode struct {
	OID         string              `json:"oid"`
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Syntax      string              `json:"syntax,omitempty"`
	Access      string              `json:"access,omitempty"`
	Status      string              `json:"status,omitempty"`
	Parent      *OIDNode            `json:"-"` // Don't serialize to avoid cycles
	Children    map[string]*OIDNode `json:"children,omitempty"`
	MIBName     string              `json:"mib_name"`
	IsTable     bool                `json:"is_table,omitempty"`
	IsEntry     bool                `json:"is_entry,omitempty"`
	Index       []string            `json:"index,omitempty"`
}

// MIBInfo represents parsed MIB information
type MIBInfo struct {
	Name         string              `json:"name"`
	Description  string              `json:"description,omitempty"`
	Organization string              `json:"organization,omitempty"`
	ContactInfo  string              `json:"contact_info,omitempty"`
	LastUpdated  string              `json:"last_updated,omitempty"`
	Revision     string              `json:"revision,omitempty"`
	Imports      map[string]string   `json:"imports,omitempty"`
	Objects      map[string]*OIDNode `json:"objects,omitempty"`
	FilePath     string              `json:"file_path"`
}

// ASN.1 BER/DER tag constants for SNMP parsing
const (
	tagBoolean          = 0x01
	tagInteger          = 0x02
	tagBitString        = 0x03
	tagOctetString      = 0x04
	tagNull             = 0x05
	tagObjectIdentifier = 0x06
	tagSequence         = 0x30
	tagSet              = 0x31
	tagIPAddress        = 0x40 // Application tag 0
	tagCounter32        = 0x41 // Application tag 1
	tagGauge32          = 0x42 // Application tag 2
	tagTimeTicks        = 0x43 // Application tag 3
	tagOpaque           = 0x44 // Application tag 4
	tagCounter64        = 0x46 // Application tag 6
)

// SNMP PDU context-specific tags
const (
	tagGetRequest     = 0xA0
	tagGetNextRequest = 0xA1
	tagGetResponse    = 0xA2
	tagSetRequest     = 0xA3
	tagGetBulkRequest = 0xA5
	tagInformRequest  = 0xA6
	tagTrapV2         = 0xA7
	tagReport         = 0xA8
)

// SNMPParser handles parsing of SNMP packets
type SNMPParser struct {
	data   []byte
	offset int
}

// Manager provides consolidated MIB loading, parsing, and OID resolution functionality
type Manager struct {
	// Configuration
	config *MIBConfig

	// File loading
	files    map[string]*MIBFile
	watcher  *fsnotify.Watcher
	handlers []ReloadHandler

	// MIB parsing
	oidTree   *OIDNode
	mibs      map[string]*MIBInfo
	nameToOID map[string]string
	oidToName map[string]string
	nodes     map[string]*OIDNode

	// OID resolution caching
	oidCache    map[string]*CacheEntry
	nameCache   map[string]*CacheEntry
	lastCleanup time.Time

	// Statistics and synchronization
	stats *MIBStats
	mu    sync.RWMutex
}

// MIBConfig holds consolidated configuration for MIB operations
type MIBConfig struct {
	// Loader configuration
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

	// Resolver configuration
	CacheSize        int  `json:"cache_size"`
	EnablePartialOID bool `json:"enable_partial_oid"`
	MaxSearchDepth   int  `json:"max_search_depth"`
	PrefixMatching   bool `json:"prefix_matching"`
}

// MIBStats tracks consolidated MIB statistics
type MIBStats struct {
	// Loader stats
	DirectoriesScanned int           `json:"directories_scanned"`
	FilesLoaded        int           `json:"files_loaded"`
	ParseErrors        int           `json:"parse_errors"`
	LastScanTime       time.Time     `json:"last_scan_time"`
	ScanDuration       time.Duration `json:"scan_duration"`
	TotalFileSize      int64         `json:"total_file_size"`

	// Parser stats
	MIBsParsed    int `json:"mibs_parsed"`
	ObjectsParsed int `json:"objects_parsed"`
	TreeDepth     int `json:"tree_depth"`
	TotalNodes    int `json:"total_nodes"`
	TablesFound   int `json:"tables_found"`
	EntriesFound  int `json:"entries_found"`

	// Resolver stats
	TotalLookups     int64         `json:"total_lookups"`
	CacheHits        int64         `json:"cache_hits"`
	CacheMisses      int64         `json:"cache_misses"`
	ExactMatches     int64         `json:"exact_matches"`
	PartialMatches   int64         `json:"partial_matches"`
	ReverseLookups   int64         `json:"reverse_lookups"`
	AverageQueryTime time.Duration `json:"average_query_time"`
	CacheSize        int           `json:"cache_size"`
}

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

// OIDInfo represents resolved OID information
type OIDInfo struct {
	OID         string            `json:"oid"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Syntax      string            `json:"syntax,omitempty"`
	Access      string            `json:"access,omitempty"`
	Status      string            `json:"status,omitempty"`
	MIBName     string            `json:"mib_name"`
	IsTable     bool              `json:"is_table,omitempty"`
	IsEntry     bool              `json:"is_entry,omitempty"`
	Index       []string          `json:"index,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// CacheEntry represents a cached resolution result
type CacheEntry struct {
	Value     string    `json:"value"`
	Timestamp time.Time `json:"timestamp"`
	HitCount  int       `json:"hit_count"`
}

// ReloadHandler is a function that handles MIB reload events
type ReloadHandler func(files []*MIBFile) error

// ParserStats tracks parser statistics
type ParserStats struct {
	MIBsParsed    int `json:"mibs_parsed"`
	ObjectsParsed int `json:"objects_parsed"`
	ParseErrors   int `json:"parse_errors"`
	TreeDepth     int `json:"tree_depth"`
	TotalNodes    int `json:"total_nodes"`
	TablesFound   int `json:"tables_found"`
	EntriesFound  int `json:"entries_found"`
}

// Regular expressions for MIB parsing
var (
	mibNameRegex     = regexp.MustCompile(`^([A-Z][A-Z0-9-]*)\s+DEFINITIONS\s*::=\s*BEGIN`)
	objectRegex      = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9-]*)\s+OBJECT-TYPE`)
	oidRegex         = regexp.MustCompile(`::=\s*\{\s*([^}]+)\s*\}`)
	syntaxRegex      = regexp.MustCompile(`SYNTAX\s+([^\n\r]+)`)
	accessRegex      = regexp.MustCompile(`ACCESS\s+([^\n\r]+)`)
	statusRegex      = regexp.MustCompile(`STATUS\s+([^\n\r]+)`)
	descriptionRegex = regexp.MustCompile(`DESCRIPTION\s+"([^"]*)"`)
	indexRegex       = regexp.MustCompile(`INDEX\s*\{\s*([^}]+)\s*\}`)
	importsRegex     = regexp.MustCompile(`IMPORTS\s+(.*?);`)
)

// DefaultMIBConfig returns a default MIB configuration
func DefaultMIBConfig() *MIBConfig {
	return &MIBConfig{
		// Loader defaults
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

		// Resolver defaults
		CacheSize:        1000,
		EnablePartialOID: true,
		MaxSearchDepth:   10,
		PrefixMatching:   true,
	}
}

// NewManager creates a new consolidated MIB manager
func NewManager(cfg config.Provider) (*Manager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration provider cannot be nil")
	}

	// Load configuration
	mibConfig := DefaultMIBConfig()

	// Override with config values if available
	if dirs, err := cfg.GetStringSlice("mib.directories"); err == nil {
		mibConfig.MIBDirectories = dirs
	}

	if exts, err := cfg.GetStringSlice("mib.file_extensions"); err == nil {
		mibConfig.FileExtensions = exts
	}

	if size, err := cfg.GetInt("mib.max_file_size", int(mibConfig.MaxFileSize)); err == nil {
		mibConfig.MaxFileSize = int64(size)
	}

	if reload, err := cfg.GetBool("mib.enable_hot_reload", mibConfig.EnableHotReload); err == nil {
		mibConfig.EnableHotReload = reload
	}

	if cache, err := cfg.GetBool("mib.cache_enabled", mibConfig.CacheEnabled); err == nil {
		mibConfig.CacheEnabled = cache
	}

	if expiry, err := cfg.GetDuration("mib.cache_expiry", mibConfig.CacheExpiry); err == nil {
		mibConfig.CacheExpiry = expiry
	}

	// Required MIBs configuration
	if requiredMibs, err := cfg.GetStringSlice("mib.required_mibs"); err == nil {
		mibConfig.RequiredMIBs = requiredMibs
	}

	// Validation configuration
	if validationEnabled, err := cfg.GetBool("mib.validation_enabled", mibConfig.ValidationEnabled); err == nil {
		mibConfig.ValidationEnabled = validationEnabled
	}

	// Resolver configuration
	if cacheSize, err := cfg.GetInt("resolver.cache_size", mibConfig.CacheSize); err == nil {
		mibConfig.CacheSize = cacheSize
	}

	if prefixMatch, err := cfg.GetBool("resolver.prefix_matching", mibConfig.PrefixMatching); err == nil {
		mibConfig.PrefixMatching = prefixMatch
	}

	manager := &Manager{
		config:      mibConfig,
		files:       make(map[string]*MIBFile),
		handlers:    make([]ReloadHandler, 0),
		oidTree:     createRootNode(),
		mibs:        make(map[string]*MIBInfo),
		nameToOID:   make(map[string]string),
		oidToName:   make(map[string]string),
		nodes:       make(map[string]*OIDNode),
		oidCache:    make(map[string]*CacheEntry),
		nameCache:   make(map[string]*CacheEntry),
		lastCleanup: time.Now(),
		stats:       &MIBStats{},
	}

	return manager, nil
}

// createRootNode creates the root node of the OID tree
func createRootNode() *OIDNode {
	root := &OIDNode{
		OID:      "",
		Name:     "root",
		Children: make(map[string]*OIDNode),
	}

	// Add standard root nodes
	iso := &OIDNode{
		OID:      "1",
		Name:     "iso",
		Parent:   root,
		Children: make(map[string]*OIDNode),
	}
	root.Children["1"] = iso

	org := &OIDNode{
		OID:      "1.3",
		Name:     "org",
		Parent:   iso,
		Children: make(map[string]*OIDNode),
	}
	iso.Children["3"] = org

	dod := &OIDNode{
		OID:      "1.3.6",
		Name:     "dod",
		Parent:   org,
		Children: make(map[string]*OIDNode),
	}
	org.Children["6"] = dod

	internet := &OIDNode{
		OID:      "1.3.6.1",
		Name:     "internet",
		Parent:   dod,
		Children: make(map[string]*OIDNode),
	}
	dod.Children["1"] = internet

	// Add standard internet subtrees
	mgmt := &OIDNode{
		OID:      "1.3.6.1.2",
		Name:     "mgmt",
		Parent:   internet,
		Children: make(map[string]*OIDNode),
	}
	internet.Children["2"] = mgmt

	mib2 := &OIDNode{
		OID:      "1.3.6.1.2.1",
		Name:     "mib-2",
		Parent:   mgmt,
		Children: make(map[string]*OIDNode),
	}
	mgmt.Children["1"] = mib2

	private := &OIDNode{
		OID:      "1.3.6.1.4",
		Name:     "private",
		Parent:   internet,
		Children: make(map[string]*OIDNode),
	}
	internet.Children["4"] = private

	enterprises := &OIDNode{
		OID:      "1.3.6.1.4.1",
		Name:     "enterprises",
		Parent:   private,
		Children: make(map[string]*OIDNode),
	}
	private.Children["1"] = enterprises

	return root
}

// LoadAll scans all configured directories and loads MIB files
func (m *Manager) LoadAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	startTime := time.Now()
	m.stats.DirectoriesScanned = 0
	m.stats.FilesLoaded = 0
	m.stats.ParseErrors = 0

	for _, dir := range m.config.MIBDirectories {
		if err := m.scanDirectory(dir); err != nil {
			// Log error but continue with other directories
			continue
		}
		m.stats.DirectoriesScanned++
	}

	m.stats.LastScanTime = time.Now()
	m.stats.ScanDuration = time.Since(startTime)

	// Parse all loaded files
	if err := m.parseAll(); err != nil {
		return fmt.Errorf("failed to parse MIB files: %w", err)
	}

	// Validate required MIBs are present
	if m.config.ValidationEnabled {
		if err := m.validateRequiredMIBs(); err != nil {
			return fmt.Errorf("required MIB validation failed: %w", err)
		}
	}

	return nil
}

// scanDirectory scans a directory for MIB files
func (m *Manager) scanDirectory(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("directory does not exist: %s", dir)
	}

	walkFunc := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if !m.config.RecursiveScan && path != dir {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file matches our criteria
		if m.shouldLoadFile(path) {
			if err := m.loadFile(path); err != nil {
				// Log error but continue with other files
				return nil
			}
		}

		return nil
	}

	return filepath.WalkDir(dir, walkFunc)
}

// shouldLoadFile determines if a file should be loaded based on configuration
func (m *Manager) shouldLoadFile(path string) bool {
	// Check file extension
	ext := strings.ToLower(filepath.Ext(path))
	validExt := false
	for _, allowedExt := range m.config.FileExtensions {
		if ext == strings.ToLower(allowedExt) {
			validExt = true
			break
		}
	}
	if !validExt {
		return false
	}

	// Check ignore patterns
	filename := filepath.Base(path)
	for _, pattern := range m.config.IgnorePatterns {
		if matched, _ := filepath.Match(pattern, filename); matched {
			return false
		}
	}

	return true
}

// loadFile loads a single MIB file
func (m *Manager) loadFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat file %s: %w", path, err)
	}

	// Check file size
	if info.Size() > m.config.MaxFileSize {
		return fmt.Errorf("file %s exceeds maximum size limit", path)
	}

	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", path, err)
	}

	// Create MIB file entry
	mibFile := &MIBFile{
		Path:       path,
		Name:       strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		Content:    content,
		Size:       info.Size(),
		ModTime:    info.ModTime(),
		LastLoaded: time.Now(),
		LoadCount:  1,
		ParsedOK:   false,
	}

	// Update existing file or add new one
	if existing, exists := m.files[path]; exists {
		mibFile.LoadCount = existing.LoadCount + 1
	}

	m.files[path] = mibFile
	m.stats.FilesLoaded++
	m.stats.TotalFileSize += info.Size()

	return nil
}

// parseAll parses all loaded MIB files
func (m *Manager) parseAll() error {
	m.stats.MIBsParsed = 0
	m.stats.ObjectsParsed = 0
	m.stats.ParseErrors = 0

	for _, file := range m.files {
		if err := m.parseMIBFile(file); err != nil {
			file.ParseError = err.Error()
			file.ParsedOK = false
			m.stats.ParseErrors++
			// Log error but continue with other files
			continue
		}
		file.ParsedOK = true
		m.stats.MIBsParsed++
	}

	// Build cross-references and resolve dependencies
	m.buildCrossReferences()
	m.calculateStats()

	return nil
}

// validateRequiredMIBs ensures all required MIBs are loaded
func (m *Manager) validateRequiredMIBs() error {
	missing := []string{}

	for _, required := range m.config.RequiredMIBs {
		found := false
		for _, file := range m.files {
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

// parseMIBFile parses a single MIB file
func (m *Manager) parseMIBFile(file *MIBFile) error {
	content := string(file.Content)

	// Extract MIB name
	nameMatch := mibNameRegex.FindStringSubmatch(content)
	if len(nameMatch) < 2 {
		return fmt.Errorf("could not extract MIB name from %s", file.Path)
	}

	mibName := nameMatch[1]
	mibInfo := &MIBInfo{
		Name:     mibName,
		FilePath: file.Path,
		Objects:  make(map[string]*OIDNode),
		Imports:  make(map[string]string),
	}

	// Extract imports
	m.parseImports(content, mibInfo)

	// Extract MIB metadata
	m.parseMIBMetadata(content, mibInfo)

	// Parse OBJECT-TYPE definitions
	if err := m.parseObjects(content, mibInfo); err != nil {
		return fmt.Errorf("failed to parse objects in %s: %w", file.Path, err)
	}

	m.mibs[mibName] = mibInfo
	return nil
}

// parseImports extracts import statements from MIB content
func (m *Manager) parseImports(content string, mibInfo *MIBInfo) {
	matches := importsRegex.FindStringSubmatch(content)
	if len(matches) < 2 {
		return
	}

	importSection := matches[1]
	lines := strings.Split(importSection, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}

		// Simple parsing - look for "FROM ModuleName"
		if strings.Contains(line, " FROM ") {
			parts := strings.Split(line, " FROM ")
			if len(parts) == 2 {
				symbols := strings.TrimSpace(parts[0])
				module := strings.TrimSpace(parts[1])
				module = strings.TrimSuffix(module, ",")
				mibInfo.Imports[symbols] = module
			}
		}
	}
}

// parseMIBMetadata extracts metadata from MIB content
func (m *Manager) parseMIBMetadata(content string, mibInfo *MIBInfo) {
	// Extract organization
	if match := regexp.MustCompile(`ORGANIZATION\s+"([^"]*)"`).FindStringSubmatch(content); len(match) > 1 {
		mibInfo.Organization = match[1]
	}

	// Extract contact info
	if match := regexp.MustCompile(`CONTACT-INFO\s+"([^"]*)"`).FindStringSubmatch(content); len(match) > 1 {
		mibInfo.ContactInfo = match[1]
	}

	// Extract description
	if match := regexp.MustCompile(`DESCRIPTION\s+"([^"]*)"`).FindStringSubmatch(content); len(match) > 1 {
		mibInfo.Description = match[1]
	}

	// Extract last updated
	if match := regexp.MustCompile(`LAST-UPDATED\s+"([^"]*)"`).FindStringSubmatch(content); len(match) > 1 {
		mibInfo.LastUpdated = match[1]
	}

	// Extract revision
	if match := regexp.MustCompile(`REVISION\s+"([^"]*)"`).FindStringSubmatch(content); len(match) > 1 {
		mibInfo.Revision = match[1]
	}
}

// parseObjects extracts OBJECT-TYPE definitions from MIB content
func (m *Manager) parseObjects(content string, mibInfo *MIBInfo) error {
	// Find all OBJECT-TYPE definitions
	objectMatches := objectRegex.FindAllStringSubmatch(content, -1)

	for _, match := range objectMatches {
		if len(match) < 2 {
			continue
		}

		objectName := match[1]
		if err := m.parseObjectDefinition(content, objectName, mibInfo); err != nil {
			// Log error but continue with other objects
			continue
		}
		m.stats.ObjectsParsed++
	}

	return nil
}

// resolveOID resolves an OID string to numeric form
func (m *Manager) resolveOID(oidStr string, mibInfo *MIBInfo) string {
	// Handle numeric OIDs
	if strings.Contains(oidStr, ".") {
		return oidStr
	}

	// Handle symbolic OIDs like "{ mib-2 1 }"
	oidStr = strings.Trim(oidStr, "{}")
	parts := strings.Fields(oidStr)

	var oidParts []string
	for _, part := range parts {
		if num, err := strconv.Atoi(part); err == nil {
			oidParts = append(oidParts, strconv.Itoa(num))
		} else {
			// Try to resolve symbolic name
			if resolved, exists := m.nameToOID[part]; exists {
				oidParts = append(oidParts, resolved)
			} else {
				// Use well-known mappings
				switch part {
				case "iso":
					oidParts = append(oidParts, "1")
				case "org":
					oidParts = append(oidParts, "1.3")
				case "dod":
					oidParts = append(oidParts, "1.3.6")
				case "internet":
					oidParts = append(oidParts, "1.3.6.1")
				case "mgmt":
					oidParts = append(oidParts, "1.3.6.1.2")
				case "mib-2":
					oidParts = append(oidParts, "1.3.6.1.2.1")
				case "private":
					oidParts = append(oidParts, "1.3.6.1.4")
				case "enterprises":
					oidParts = append(oidParts, "1.3.6.1.4.1")
				default:
					oidParts = append(oidParts, part) // Keep as-is for later resolution
				}
			}
		}
	}

	return strings.Join(oidParts, ".")
}

// addToTree adds an OID node to the tree structure
func (m *Manager) addToTree(node *OIDNode) {
	if node.OID == "" {
		return
	}

	parts := strings.Split(node.OID, ".")
	current := m.oidTree

	// Navigate to the correct position in the tree
	for i, part := range parts {
		if part == "" {
			continue
		}

		if child, exists := current.Children[part]; exists {
			current = child
		} else {
			// Create intermediate nodes if they don't exist
			newNode := &OIDNode{
				OID:      strings.Join(parts[:i+1], "."),
				Name:     fmt.Sprintf("node_%s", part),
				Parent:   current,
				Children: make(map[string]*OIDNode),
			}
			current.Children[part] = newNode
			current = newNode
		}
	}

	// Update the final node with the parsed information
	if current != m.oidTree {
		current.Name = node.Name
		current.Description = node.Description
		current.Syntax = node.Syntax
		current.Access = node.Access
		current.Status = node.Status
		current.MIBName = node.MIBName
		current.IsTable = node.IsTable
		current.IsEntry = node.IsEntry
		current.Index = node.Index
	}
}

// buildCrossReferences builds cross-references between MIB objects
func (m *Manager) buildCrossReferences() {
	// This is a simplified implementation
	// In a full implementation, this would resolve all symbolic references
	for _, mib := range m.mibs {
		for _, obj := range mib.Objects {
			if obj.OID != "" {
				m.nameToOID[obj.Name] = obj.OID
				m.oidToName[obj.OID] = obj.Name
			}
		}
	}
}

// calculateStats calculates tree statistics
func (m *Manager) calculateStats() {
	m.stats.TotalNodes = m.countNodes(m.oidTree)
	m.stats.TreeDepth = m.calculateDepth(m.oidTree, 0)
}

// countNodes counts total nodes in the tree
func (m *Manager) countNodes(node *OIDNode) int {
	count := 1
	for _, child := range node.Children {
		count += m.countNodes(child)
	}
	return count
}

// calculateDepth calculates maximum tree depth
func (m *Manager) calculateDepth(node *OIDNode, currentDepth int) int {
	maxDepth := currentDepth
	for _, child := range node.Children {
		depth := m.calculateDepth(child, currentDepth+1)
		if depth > maxDepth {
			maxDepth = depth
		}
	}
	return maxDepth
}

// GetMIB returns MIB information by name
func (m *Manager) GetMIB(name string) (*MIBInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	mib, exists := m.mibs[name]
	return mib, exists
}

// GetAllMIBs returns all parsed MIB information
func (m *Manager) GetAllMIBs() map[string]*MIBInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*MIBInfo)
	for name, mib := range m.mibs {
		result[name] = mib
	}
	return result
}

// GetOIDTree returns the root of the OID tree
func (m *Manager) GetOIDTree() *OIDNode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.oidTree
}

// GetStats returns consolidated MIB statistics
func (m *Manager) GetStats() *MIBStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := *m.stats
	return &stats
}

// ResolveOIDSimple resolves a numeric OID to its symbolic name
func (m *Manager) ResolveOIDSimple(oid string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	name, exists := m.oidToName[oid]
	return name, exists
}

// ResolveOID resolves a numeric OID to detailed information (implements OIDResolver interface)
func (m *Manager) ResolveOID(oid string) (*OIDInfo, error) {
	return m.ResolveOIDDetailed(oid)
}

// ResolveName resolves a symbolic name to its numeric OID
func (m *Manager) ResolveName(name string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	oid, exists := m.nameToOID[name]
	return oid, exists
}

// FindNode finds an OID node by numeric OID
func (m *Manager) FindNode(oid string) (*OIDNode, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	parts := strings.Split(oid, ".")
	current := m.oidTree

	for _, part := range parts {
		if part == "" {
			continue
		}

		if child, exists := current.Children[part]; exists {
			current = child
		} else {
			return nil, false
		}
	}

	return current, true
}

// GetNodeInfo returns detailed information about an OID node
func (m *Manager) GetNodeInfo(oid string) (*OIDNode, bool) {
	return m.FindNode(oid)
}

// parseObjectDefinition parses an object definition from MIB content
func (m *Manager) parseObjectDefinition(content, objectName string, mibInfo *MIBInfo) error {
	// This is a simplified implementation
	// In a full implementation, this would parse the complete object definition
	// including SYNTAX, ACCESS, STATUS, DESCRIPTION, etc.

	// For now, we'll just create a basic OID node
	node := &OIDNode{
		Name:        objectName,
		Description: fmt.Sprintf("Object definition for %s", objectName),
		MIBName:     mibInfo.Name,
	}

	// Try to extract OID from the definition
	oidPattern := regexp.MustCompile(objectName + `\s+OBJECT\s+IDENTIFIER\s*::=\s*\{\s*([^}]+)\s*\}`)
	if match := oidPattern.FindStringSubmatch(content); len(match) > 1 {
		// Parse the OID path (simplified)
		oidPath := strings.TrimSpace(match[1])
		// This would need more sophisticated parsing in a real implementation
		node.OID = oidPath
	}

	// Add to manager's node registry
	if m.nodes == nil {
		m.nodes = make(map[string]*OIDNode)
	}
	m.nodes[objectName] = node

	return nil
}

// ResolveOIDDetailed resolves a numeric OID to detailed information with caching
func (m *Manager) ResolveOIDDetailed(oid string) (*OIDInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.TotalLookups++

	// Check cache first
	if m.config.CacheEnabled {
		if entry, exists := m.oidCache[oid]; exists && time.Since(entry.Timestamp) < m.config.CacheExpiry {
			m.stats.CacheHits++
			entry.HitCount++

			// Parse cached result
			info, err := m.parseOIDInfo(entry.Value)
			if err == nil {
				return info, nil
			}
		}
	}

	m.stats.CacheMisses++

	// Try exact match first
	if node, found := m.FindNode(oid); found && node.Name != "" {
		m.stats.ExactMatches++
		info := m.nodeToOIDInfo(node)
		m.cacheResult(oid, info, m.oidCache)
		return info, nil
	}

	// Try partial matching if enabled
	if m.config.EnablePartialOID {
		if info, err := m.resolvePartialOID(oid); err == nil {
			m.stats.PartialMatches++
			m.cacheResult(oid, info, m.oidCache)
			return info, nil
		}
	}

	return nil, fmt.Errorf("OID %s not found", oid)
}

// ResolveNameDetailed resolves a symbolic name to detailed information with caching
func (m *Manager) ResolveNameDetailed(name string) (*OIDInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.TotalLookups++
	m.stats.ReverseLookups++

	// Check cache first
	if m.config.CacheEnabled {
		if entry, exists := m.nameCache[name]; exists && time.Since(entry.Timestamp) < m.config.CacheExpiry {
			m.stats.CacheHits++
			entry.HitCount++

			// Parse cached result
			info, err := m.parseOIDInfo(entry.Value)
			if err == nil {
				return info, nil
			}
		}
	}

	m.stats.CacheMisses++

	// Try to resolve using basic resolution
	if oid, found := m.ResolveName(name); found {
		if node, nodeFound := m.FindNode(oid); nodeFound {
			m.stats.ExactMatches++
			info := m.nodeToOIDInfo(node)
			m.cacheResult(name, info, m.nameCache)
			return info, nil
		}
	}

	// Try prefix matching if enabled
	if m.config.PrefixMatching {
		if info, err := m.resolvePrefixName(name); err == nil {
			m.stats.PartialMatches++
			m.cacheResult(name, info, m.nameCache)
			return info, nil
		}
	}

	return nil, fmt.Errorf("name %s not found", name)
}

// resolvePartialOID attempts to resolve a partial OID by finding the closest match
func (m *Manager) resolvePartialOID(oid string) (*OIDInfo, error) {
	parts := strings.Split(oid, ".")

	// Try progressively shorter OIDs
	for i := len(parts); i > 0; i-- {
		partialOID := strings.Join(parts[:i], ".")
		if node, found := m.FindNode(partialOID); found && node.Name != "" {
			info := m.nodeToOIDInfo(node)

			// Add remaining OID parts as suffix
			if i < len(parts) {
				suffix := strings.Join(parts[i:], ".")
				info.Name = fmt.Sprintf("%s.%s", info.Name, suffix)
				info.OID = oid // Use original OID
			}

			return info, nil
		}
	}

	return nil, fmt.Errorf("no partial match found for OID %s", oid)
}

// resolvePrefixName attempts to resolve a name using prefix matching
func (m *Manager) resolvePrefixName(name string) (*OIDInfo, error) {
	// Get all MIBs and search for prefix matches
	mibs := m.GetAllMIBs()

	for _, mib := range mibs {
		for objName, node := range mib.Objects {
			if strings.HasPrefix(strings.ToLower(objName), strings.ToLower(name)) {
				return m.nodeToOIDInfo(node), nil
			}
		}
	}

	return nil, fmt.Errorf("no prefix match found for name %s", name)
}

// nodeToOIDInfo converts a MIB node to OIDInfo
func (m *Manager) nodeToOIDInfo(node *OIDNode) *OIDInfo {
	info := &OIDInfo{
		OID:         node.OID,
		Name:        node.Name,
		Description: node.Description,
		Syntax:      node.Syntax,
		Access:      node.Access,
		Status:      node.Status,
		MIBName:     node.MIBName,
		IsTable:     node.IsTable,
		IsEntry:     node.IsEntry,
		Index:       node.Index,
	}

	// Add parent information
	if node.Parent != nil && node.Parent.Name != "root" {
		info.Metadata = make(map[string]string)
		info.Metadata["parent_oid"] = node.Parent.OID
		info.Metadata["parent_name"] = node.Parent.Name
	}

	return info
}

// cacheResult caches a resolution result
func (m *Manager) cacheResult(key string, info *OIDInfo, cache map[string]*CacheEntry) {
	if !m.config.CacheEnabled {
		return
	}

	// Check if cache is full and needs cleanup
	if len(cache) >= m.config.CacheSize {
		m.cleanupCache(cache)
	}

	// Serialize info to string for caching
	value := m.serializeOIDInfo(info)

	cache[key] = &CacheEntry{
		Value:     value,
		Timestamp: time.Now(),
		HitCount:  0,
	}
}

// cleanupCache removes expired and least-used entries
func (m *Manager) cleanupCache(cache map[string]*CacheEntry) {
	now := time.Now()

	// Remove expired entries first
	for key, entry := range cache {
		if now.Sub(entry.Timestamp) > m.config.CacheExpiry {
			delete(cache, key)
		}
	}

	// If still too full, remove least-used entries
	if len(cache) >= m.config.CacheSize {
		// Find entries with lowest hit count
		minHits := int(^uint(0) >> 1) // Max int
		for _, entry := range cache {
			if entry.HitCount < minHits {
				minHits = entry.HitCount
			}
		}

		// Remove entries with minimum hit count
		for key, entry := range cache {
			if entry.HitCount == minHits && len(cache) >= m.config.CacheSize {
				delete(cache, key)
			}
		}
	}
}

// serializeOIDInfo serializes OIDInfo to a string for caching
func (m *Manager) serializeOIDInfo(info *OIDInfo) string {
	// Simple serialization - in production, use JSON or similar
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s",
		info.OID, info.Name, info.Description, info.Syntax,
		info.Access, info.Status, info.MIBName)
}

// parseOIDInfo parses a serialized OIDInfo string
func (m *Manager) parseOIDInfo(value string) (*OIDInfo, error) {
	parts := strings.Split(value, "|")
	if len(parts) < 7 {
		return nil, fmt.Errorf("invalid cached value format")
	}

	return &OIDInfo{
		OID:         parts[0],
		Name:        parts[1],
		Description: parts[2],
		Syntax:      parts[3],
		Access:      parts[4],
		Status:      parts[5],
		MIBName:     parts[6],
	}, nil
}

// ClearCache clears all cached entries
func (m *Manager) ClearCache() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.oidCache = make(map[string]*CacheEntry)
	m.nameCache = make(map[string]*CacheEntry)
}

// GetCacheInfo returns information about cached entries
func (m *Manager) GetCacheInfo() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]any{
		"oid_cache_size":  len(m.oidCache),
		"name_cache_size": len(m.nameCache),
		"cache_enabled":   m.config.CacheEnabled,
		"cache_expiry":    m.config.CacheExpiry.String(),
		"max_cache_size":  m.config.CacheSize,
	}
}

// StartHotReload starts watching MIB directories for changes
func (m *Manager) StartHotReload() error {
	if !m.config.EnableHotReload {
		return nil
	}

	var err error
	m.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	// Watch all configured directories
	for _, dir := range m.config.MIBDirectories {
		if _, err := os.Stat(dir); err == nil {
			if err := m.watcher.Add(dir); err != nil {
				return fmt.Errorf("failed to watch directory %s: %w", dir, err)
			}
		}
	}

	// Start watching in a goroutine
	go m.watchFiles()

	return nil
}

// StopHotReload stops watching for file changes
func (m *Manager) StopHotReload() error {
	if m.watcher != nil {
		return m.watcher.Close()
	}
	return nil
}

// watchFiles watches for file system changes
func (m *Manager) watchFiles() {
	for {
		select {
		case event, ok := <-m.watcher.Events:
			if !ok {
				return
			}

			// Handle file changes
			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
				if m.shouldLoadFile(event.Name) {
					// Debounce rapid changes
					time.Sleep(100 * time.Millisecond)
					m.handleFileChange(event.Name)
				}
			}

		case err, ok := <-m.watcher.Errors:
			if !ok {
				return
			}
			// Log error but continue watching
			_ = err
		}
	}
}

// handleFileChange handles a file change event
func (m *Manager) handleFileChange(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Reload the specific file
	if err := m.loadFile(path); err != nil {
		// Log error but continue
		return
	}

	// Re-parse the file
	if file, exists := m.files[path]; exists {
		if err := m.parseMIBFile(file); err != nil {
			file.ParseError = err.Error()
			file.ParsedOK = false
		} else {
			file.ParsedOK = true
		}
	}

	// Rebuild cross-references
	m.buildCrossReferences()
	m.calculateStats()

	// Clear cache to ensure fresh data
	m.oidCache = make(map[string]*CacheEntry)
	m.nameCache = make(map[string]*CacheEntry)

	// Notify handlers
	m.notifyReloadHandlers()
}

// RegisterReloadHandler registers a handler for reload events
func (m *Manager) RegisterReloadHandler(handler ReloadHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers = append(m.handlers, handler)
}

// notifyReloadHandlers notifies all registered reload handlers
func (m *Manager) notifyReloadHandlers() {
	files := make([]*MIBFile, 0, len(m.files))
	for _, file := range m.files {
		files = append(files, file)
	}

	for _, handler := range m.handlers {
		go func(h ReloadHandler) {
			_ = h(files)
		}(handler)
	}
}

// SNMP Parsing Methods

// NewSNMPParser creates a new SNMP parser for the given data
func (m *Manager) NewSNMPParser(data []byte) *SNMPParser {
	return &SNMPParser{
		data:   data,
		offset: 0,
	}
}

// ParseSNMPPacket parses an SNMP packet and returns the structured data
func (p *SNMPParser) ParseSNMPPacket() (*types.SNMPPacket, error) {
	// Reset parser state
	p.offset = 0

	// Parse the outer sequence
	if err := p.expectTag(tagSequence); err != nil {
		return nil, fmt.Errorf("expected SNMP sequence: %w", err)
	}

	length, err := p.parseLength()
	if err != nil {
		return nil, fmt.Errorf("failed to parse sequence length: %w", err)
	}

	if p.offset+length > len(p.data) {
		return nil, fmt.Errorf("sequence length exceeds packet size")
	}

	// Parse SNMP version
	version, err := p.parseInteger()
	if err != nil {
		return nil, fmt.Errorf("failed to parse SNMP version: %w", err)
	}

	// Parse community string
	community, err := p.parseOctetString()
	if err != nil {
		return nil, fmt.Errorf("failed to parse community string: %w", err)
	}

	// Parse PDU based on version
	var packet *types.SNMPPacket
	switch version {
	case types.VersionSNMPv2c:
		packet, err = p.parseSNMPv2cPDU()
	default:
		return nil, fmt.Errorf("unsupported SNMP version: %d (only SNMPv2c is supported)", version)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to parse PDU: %w", err)
	}

	packet.Version = int(version)
	packet.Community = string(community)
	packet.Timestamp = time.Now()

	return packet, nil
}

// parseSNMPv2cPDU parses an SNMP v2c PDU
func (p *SNMPParser) parseSNMPv2cPDU() (*types.SNMPPacket, error) {
	// Check PDU type
	if p.offset >= len(p.data) {
		return nil, fmt.Errorf("unexpected end of data")
	}

	pduType := p.data[p.offset]
	if pduType != tagTrapV2 {
		return nil, fmt.Errorf("expected trap v2 PDU, got 0x%02x", pduType)
	}

	p.offset++

	// Parse PDU length
	_, err := p.parseLength()
	if err != nil {
		return nil, fmt.Errorf("failed to parse PDU length: %w", err)
	}

	// Parse request ID
	requestID, err := p.parseInteger()
	if err != nil {
		return nil, fmt.Errorf("failed to parse request ID: %w", err)
	}

	// Parse error status
	errorStatus, err := p.parseInteger()
	if err != nil {
		return nil, fmt.Errorf("failed to parse error status: %w", err)
	}

	// Parse error index
	errorIndex, err := p.parseInteger()
	if err != nil {
		return nil, fmt.Errorf("failed to parse error index: %w", err)
	}

	// Parse varbind list
	varbinds, err := p.parseVarbindList()
	if err != nil {
		return nil, fmt.Errorf("failed to parse varbinds: %w", err)
	}

	return &types.SNMPPacket{
		PDUType:     types.PDUTypeTrapV2,
		RequestID:   int32(requestID),
		ErrorStatus: int(errorStatus),
		ErrorIndex:  int(errorIndex),
		Varbinds:    varbinds,
	}, nil
}

// parseVarbindList parses a sequence of varbinds
func (p *SNMPParser) parseVarbindList() ([]types.Varbind, error) {
	// Expect sequence tag
	if err := p.expectTag(tagSequence); err != nil {
		return nil, fmt.Errorf("expected varbind sequence: %w", err)
	}

	length, err := p.parseLength()
	if err != nil {
		return nil, fmt.Errorf("failed to parse varbind sequence length: %w", err)
	}

	endOffset := p.offset + length
	var varbinds []types.Varbind

	for p.offset < endOffset {
		varbind, err := p.parseVarbind()
		if err != nil {
			return nil, fmt.Errorf("failed to parse varbind: %w", err)
		}
		varbinds = append(varbinds, varbind)
	}

	return varbinds, nil
}

// Helper methods for parsing basic ASN.1 types

// expectTag checks if the current byte matches the expected tag
func (p *SNMPParser) expectTag(expectedTag byte) error {
	if p.offset >= len(p.data) {
		return fmt.Errorf("unexpected end of data")
	}

	if p.data[p.offset] != expectedTag {
		return fmt.Errorf("expected tag 0x%02x, got 0x%02x", expectedTag, p.data[p.offset])
	}

	p.offset++
	return nil
}

// parseLength parses ASN.1 BER/DER length encoding
func (p *SNMPParser) parseLength() (int, error) {
	if p.offset >= len(p.data) {
		return 0, fmt.Errorf("unexpected end of data")
	}

	firstByte := p.data[p.offset]
	p.offset++

	// Short form (length < 128)
	if firstByte&0x80 == 0 {
		return int(firstByte), nil
	}

	// Long form
	lengthBytes := int(firstByte & 0x7F)
	if lengthBytes == 0 {
		return 0, fmt.Errorf("indefinite length not supported")
	}

	if lengthBytes > 4 {
		return 0, fmt.Errorf("length too long: %d bytes", lengthBytes)
	}

	if p.offset+lengthBytes > len(p.data) {
		return 0, fmt.Errorf("length bytes exceed packet size")
	}

	length := 0
	for i := 0; i < lengthBytes; i++ {
		length = (length << 8) | int(p.data[p.offset])
		p.offset++
	}

	return length, nil
}

// parseInteger parses an ASN.1 INTEGER
func (p *SNMPParser) parseInteger() (int64, error) {
	if err := p.expectTag(tagInteger); err != nil {
		return 0, err
	}

	length, err := p.parseLength()
	if err != nil {
		return 0, err
	}

	if length == 0 {
		return 0, nil
	}

	if p.offset+length > len(p.data) {
		return 0, fmt.Errorf("integer length exceeds packet size")
	}

	value := int64(0)
	for i := 0; i < length; i++ {
		value = (value << 8) | int64(p.data[p.offset])
		p.offset++
	}

	// Handle negative numbers (two's complement)
	if length > 0 && p.data[p.offset-length]&0x80 != 0 && length < 8 {
		for i := length; i < 8; i++ {
			value |= int64(0xFF) << (i * 8)
		}
	}

	return value, nil
}

// parseOctetString parses an ASN.1 OCTET STRING
func (p *SNMPParser) parseOctetString() ([]byte, error) {
	if err := p.expectTag(tagOctetString); err != nil {
		return nil, err
	}

	length, err := p.parseLength()
	if err != nil {
		return nil, err
	}

	if p.offset+length > len(p.data) {
		return nil, fmt.Errorf("octet string length exceeds packet size")
	}

	value := make([]byte, length)
	copy(value, p.data[p.offset:p.offset+length])
	p.offset += length

	return value, nil
}

// parseObjectIdentifier parses an ASN.1 OBJECT IDENTIFIER
func (p *SNMPParser) parseObjectIdentifier() (string, error) {
	if err := p.expectTag(tagObjectIdentifier); err != nil {
		return "", err
	}

	length, err := p.parseLength()
	if err != nil {
		return "", err
	}

	if p.offset+length > len(p.data) {
		return "", fmt.Errorf("OID length exceeds packet size")
	}

	oidData := p.data[p.offset : p.offset+length]
	p.offset += length

	return p.decodeObjectIdentifier(oidData)
}

// parseVarbind parses a single varbind
func (p *SNMPParser) parseVarbind() (types.Varbind, error) {
	// Expect sequence tag
	if err := p.expectTag(tagSequence); err != nil {
		return types.Varbind{}, fmt.Errorf("expected varbind sequence: %w", err)
	}

	_, err := p.parseLength()
	if err != nil {
		return types.Varbind{}, fmt.Errorf("failed to parse varbind length: %w", err)
	}

	// Parse OID
	oid, err := p.parseObjectIdentifier()
	if err != nil {
		return types.Varbind{}, fmt.Errorf("failed to parse varbind OID: %w", err)
	}

	// Parse value
	value, valueType, err := p.parseValue()
	if err != nil {
		return types.Varbind{}, fmt.Errorf("failed to parse varbind value: %w", err)
	}

	return types.Varbind{
		OID:   oid,
		Type:  valueType,
		Value: value,
	}, nil
}

// decodeObjectIdentifier decodes OID bytes into dotted notation
func (p *SNMPParser) decodeObjectIdentifier(data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("empty OID")
	}

	// First byte encodes first two sub-identifiers
	firstByte := data[0]
	first := firstByte / 40
	second := firstByte % 40

	oid := fmt.Sprintf("%d.%d", first, second)

	i := 1
	for i < len(data) {
		value := uint64(0)
		for i < len(data) && data[i]&0x80 != 0 {
			value = (value << 7) | uint64(data[i]&0x7F)
			i++
		}
		if i < len(data) {
			value = (value << 7) | uint64(data[i]&0x7F)
			i++
		}
		oid += fmt.Sprintf(".%d", value)
	}

	return oid, nil
}

// parseValue parses an SNMP value and returns the value, type, and error
func (p *SNMPParser) parseValue() (interface{}, int, error) {
	if p.offset >= len(p.data) {
		return nil, 0, fmt.Errorf("unexpected end of data")
	}

	tag := p.data[p.offset]
	p.offset++

	length, err := p.parseLength()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to parse value length: %w", err)
	}

	if p.offset+length > len(p.data) {
		return nil, 0, fmt.Errorf("value length exceeds packet size")
	}

	valueData := p.data[p.offset : p.offset+length]
	p.offset += length

	switch tag {
	case tagInteger:
		if length == 0 {
			return int64(0), types.TypeInteger, nil
		}
		value := int64(0)
		for _, b := range valueData {
			value = (value << 8) | int64(b)
		}
		// Handle negative numbers (two's complement)
		if valueData[0]&0x80 != 0 && length < 8 {
			for i := length; i < 8; i++ {
				value |= int64(0xFF) << (i * 8)
			}
		}
		return value, types.TypeInteger, nil

	case tagOctetString:
		return valueData, types.TypeOctetString, nil

	case tagNull:
		return nil, types.TypeNull, nil

	case tagObjectIdentifier:
		oid, err := p.decodeObjectIdentifier(valueData)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to decode OID: %w", err)
		}
		return oid, types.TypeObjectIdentifier, nil

	case tagIPAddress:
		if length != 4 {
			return nil, 0, fmt.Errorf("invalid IP address length: %d", length)
		}
		ip := net.IPv4(valueData[0], valueData[1], valueData[2], valueData[3])
		return ip.String(), types.TypeIPAddress, nil

	case tagCounter32, tagGauge32, tagTimeTicks:
		if length == 0 {
			return uint32(0), int(tag), nil
		}
		value := uint32(0)
		for _, b := range valueData {
			value = (value << 8) | uint32(b)
		}
		return value, int(tag), nil

	case tagCounter64:
		if length == 0 {
			return uint64(0), types.TypeCounter64, nil
		}
		value := uint64(0)
		for _, b := range valueData {
			value = (value << 8) | uint64(b)
		}
		return value, types.TypeCounter64, nil

	default:
		// Return raw bytes for unknown types
		return valueData, int(tag), nil
	}
}
