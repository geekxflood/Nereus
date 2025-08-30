// Package resolver provides OID resolution and caching functionality.
package resolver

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/geekxflood/common/config"
	"github.com/geekxflood/nereus/internal/mib"
)

// ResolverConfig holds configuration for the OID resolver
type ResolverConfig struct {
	CacheEnabled     bool          `json:"cache_enabled"`
	CacheSize        int           `json:"cache_size"`
	CacheExpiry      time.Duration `json:"cache_expiry"`
	EnablePartialOID bool          `json:"enable_partial_oid"`
	MaxSearchDepth   int           `json:"max_search_depth"`
	PrefixMatching   bool          `json:"prefix_matching"`
}

// DefaultResolverConfig returns a default resolver configuration
func DefaultResolverConfig() *ResolverConfig {
	return &ResolverConfig{
		CacheEnabled:     true,
		CacheSize:        10000,
		CacheExpiry:      1 * time.Hour,
		EnablePartialOID: true,
		MaxSearchDepth:   20,
		PrefixMatching:   true,
	}
}

// CacheEntry represents a cached resolution result
type CacheEntry struct {
	Value     string    `json:"value"`
	Timestamp time.Time `json:"timestamp"`
	HitCount  int       `json:"hit_count"`
}

// ResolverStats tracks resolver statistics
type ResolverStats struct {
	CacheHits       int64   `json:"cache_hits"`
	CacheMisses     int64   `json:"cache_misses"`
	TotalLookups    int64   `json:"total_lookups"`
	PartialMatches  int64   `json:"partial_matches"`
	ExactMatches    int64   `json:"exact_matches"`
	ReverseLookups  int64   `json:"reverse_lookups"`
	CacheSize       int     `json:"cache_size"`
	CacheEvictions  int64   `json:"cache_evictions"`
	AverageHitRatio float64 `json:"average_hit_ratio"`
}

// OIDInfo represents detailed information about an OID
type OIDInfo struct {
	OID         string   `json:"oid"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Syntax      string   `json:"syntax,omitempty"`
	Access      string   `json:"access,omitempty"`
	Status      string   `json:"status,omitempty"`
	MIBName     string   `json:"mib_name,omitempty"`
	IsTable     bool     `json:"is_table,omitempty"`
	IsEntry     bool     `json:"is_entry,omitempty"`
	Index       []string `json:"index,omitempty"`
	Parent      string   `json:"parent,omitempty"`
	Children    []string `json:"children,omitempty"`
}

// Resolver provides OID resolution services with caching
type Resolver struct {
	config      *ResolverConfig
	parser      *mib.Parser
	oidCache    map[string]*CacheEntry
	nameCache   map[string]*CacheEntry
	mu          sync.RWMutex
	stats       *ResolverStats
	lastCleanup time.Time
}

// NewResolver creates a new OID resolver
func NewResolver(cfg config.Provider, parser *mib.Parser) (*Resolver, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration provider cannot be nil")
	}
	if parser == nil {
		return nil, fmt.Errorf("MIB parser cannot be nil")
	}

	// Load configuration
	resolverConfig := DefaultResolverConfig()

	if cacheEnabled, err := cfg.GetBool("resolver.cache_enabled", resolverConfig.CacheEnabled); err == nil {
		resolverConfig.CacheEnabled = cacheEnabled
	}

	if cacheSize, err := cfg.GetInt("resolver.cache_size", resolverConfig.CacheSize); err == nil {
		resolverConfig.CacheSize = cacheSize
	}

	if cacheExpiry, err := cfg.GetDuration("resolver.cache_expiry", resolverConfig.CacheExpiry); err == nil {
		resolverConfig.CacheExpiry = cacheExpiry
	}

	resolver := &Resolver{
		config:      resolverConfig,
		parser:      parser,
		oidCache:    make(map[string]*CacheEntry),
		nameCache:   make(map[string]*CacheEntry),
		stats:       &ResolverStats{},
		lastCleanup: time.Now(),
	}

	return resolver, nil
}

// ResolveOID resolves a numeric OID to its symbolic name and information
func (r *Resolver) ResolveOID(oid string) (*OIDInfo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.stats.TotalLookups++

	// Check cache first
	if r.config.CacheEnabled {
		if entry, exists := r.oidCache[oid]; exists && time.Since(entry.Timestamp) < r.config.CacheExpiry {
			r.stats.CacheHits++
			entry.HitCount++

			// Parse cached result
			info, err := r.parseOIDInfo(entry.Value)
			if err == nil {
				return info, nil
			}
		}
	}

	r.stats.CacheMisses++

	// Try exact match first
	if node, found := r.parser.FindNode(oid); found && node.Name != "" {
		r.stats.ExactMatches++
		info := r.nodeToOIDInfo(node)
		r.cacheResult(oid, info, r.oidCache)
		return info, nil
	}

	// Try partial matching if enabled
	if r.config.EnablePartialOID {
		if info, err := r.resolvePartialOID(oid); err == nil {
			r.stats.PartialMatches++
			r.cacheResult(oid, info, r.oidCache)
			return info, nil
		}
	}

	return nil, fmt.Errorf("OID %s not found", oid)
}

// ResolveName resolves a symbolic name to its numeric OID and information
func (r *Resolver) ResolveName(name string) (*OIDInfo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.stats.TotalLookups++
	r.stats.ReverseLookups++

	// Check cache first
	if r.config.CacheEnabled {
		if entry, exists := r.nameCache[name]; exists && time.Since(entry.Timestamp) < r.config.CacheExpiry {
			r.stats.CacheHits++
			entry.HitCount++

			// Parse cached result
			info, err := r.parseOIDInfo(entry.Value)
			if err == nil {
				return info, nil
			}
		}
	}

	r.stats.CacheMisses++

	// Try to resolve using parser
	if oid, found := r.parser.ResolveName(name); found {
		if node, nodeFound := r.parser.FindNode(oid); nodeFound {
			r.stats.ExactMatches++
			info := r.nodeToOIDInfo(node)
			r.cacheResult(name, info, r.nameCache)
			return info, nil
		}
	}

	// Try prefix matching if enabled
	if r.config.PrefixMatching {
		if info, err := r.resolvePrefixName(name); err == nil {
			r.stats.PartialMatches++
			r.cacheResult(name, info, r.nameCache)
			return info, nil
		}
	}

	return nil, fmt.Errorf("name %s not found", name)
}

// resolvePartialOID attempts to resolve a partial OID by finding the closest match
func (r *Resolver) resolvePartialOID(oid string) (*OIDInfo, error) {
	parts := strings.Split(oid, ".")

	// Try progressively shorter OIDs
	for i := len(parts); i > 0; i-- {
		partialOID := strings.Join(parts[:i], ".")
		if node, found := r.parser.FindNode(partialOID); found && node.Name != "" {
			info := r.nodeToOIDInfo(node)

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
func (r *Resolver) resolvePrefixName(name string) (*OIDInfo, error) {
	// Get all MIBs and search for prefix matches
	mibs := r.parser.GetAllMIBs()

	for _, mib := range mibs {
		for objName, node := range mib.Objects {
			if strings.HasPrefix(strings.ToLower(objName), strings.ToLower(name)) {
				return r.nodeToOIDInfo(node), nil
			}
		}
	}

	return nil, fmt.Errorf("no prefix match found for name %s", name)
}

// nodeToOIDInfo converts a MIB node to OIDInfo
func (r *Resolver) nodeToOIDInfo(node *mib.OIDNode) *OIDInfo {
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
		info.Parent = node.Parent.OID
	}

	// Add children information
	for _, child := range node.Children {
		if child.Name != "" {
			info.Children = append(info.Children, child.OID)
		}
	}

	return info
}

// cacheResult caches a resolution result
func (r *Resolver) cacheResult(key string, info *OIDInfo, cache map[string]*CacheEntry) {
	if !r.config.CacheEnabled {
		return
	}

	// Check if cache is full and needs cleanup
	if len(cache) >= r.config.CacheSize {
		r.cleanupCache(cache)
	}

	// Serialize info to string for caching
	value := r.serializeOIDInfo(info)

	cache[key] = &CacheEntry{
		Value:     value,
		Timestamp: time.Now(),
		HitCount:  0,
	}
}

// cleanupCache removes expired and least-used entries
func (r *Resolver) cleanupCache(cache map[string]*CacheEntry) {
	now := time.Now()

	// Remove expired entries first
	for key, entry := range cache {
		if now.Sub(entry.Timestamp) > r.config.CacheExpiry {
			delete(cache, key)
			r.stats.CacheEvictions++
		}
	}

	// If still too full, remove least-used entries
	if len(cache) >= r.config.CacheSize {
		// Find entries with lowest hit count
		minHits := int(^uint(0) >> 1) // Max int
		for _, entry := range cache {
			if entry.HitCount < minHits {
				minHits = entry.HitCount
			}
		}

		// Remove entries with minimum hit count
		for key, entry := range cache {
			if entry.HitCount == minHits && len(cache) >= r.config.CacheSize {
				delete(cache, key)
				r.stats.CacheEvictions++
			}
		}
	}
}

// serializeOIDInfo serializes OIDInfo to a string for caching
func (r *Resolver) serializeOIDInfo(info *OIDInfo) string {
	// Simple serialization - in production, use JSON or similar
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s",
		info.OID, info.Name, info.Description, info.Syntax,
		info.Access, info.Status, info.MIBName)
}

// parseOIDInfo parses a serialized OIDInfo string
func (r *Resolver) parseOIDInfo(value string) (*OIDInfo, error) {
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

// GetStats returns resolver statistics
func (r *Resolver) GetStats() *ResolverStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := *r.stats
	stats.CacheSize = len(r.oidCache) + len(r.nameCache)

	// Calculate hit ratio
	if stats.TotalLookups > 0 {
		stats.AverageHitRatio = float64(stats.CacheHits) / float64(stats.TotalLookups) * 100
	}

	return &stats
}

// ClearCache clears all cached entries
func (r *Resolver) ClearCache() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.oidCache = make(map[string]*CacheEntry)
	r.nameCache = make(map[string]*CacheEntry)
}

// GetCacheInfo returns information about cached entries
func (r *Resolver) GetCacheInfo() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return map[string]interface{}{
		"oid_cache_size":  len(r.oidCache),
		"name_cache_size": len(r.nameCache),
		"cache_enabled":   r.config.CacheEnabled,
		"cache_expiry":    r.config.CacheExpiry.String(),
		"max_cache_size":  r.config.CacheSize,
	}
}
