// Package correlator provides event correlation and deduplication functionality.
package correlator

import (
	"crypto/sha256"
	"fmt"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/geekxflood/common/config"
	"github.com/geekxflood/nereus/internal/types"
)

// StorageInterface defines the interface that correlator needs from storage
type StorageInterface interface {
	AcknowledgeEvent(id int64, ackBy string) error
}

// EventInterface defines the interface for events used in correlation
type EventInterface interface {
	GetID() int64
}

// CorrelatorConfig holds configuration for the event correlator
type CorrelatorConfig struct {
	EnableDeduplication  bool              `json:"enable_deduplication"`
	DeduplicationWindow  time.Duration     `json:"deduplication_window"`
	EnableCorrelation    bool              `json:"enable_correlation"`
	CorrelationWindow    time.Duration     `json:"correlation_window"`
	MaxCorrelationEvents int               `json:"max_correlation_events"`
	SeverityMapping      map[string]string `json:"severity_mapping"`
}

// DefaultCorrelatorConfig returns a default correlator configuration
func DefaultCorrelatorConfig() *CorrelatorConfig {
	return &CorrelatorConfig{
		EnableDeduplication:  true,
		DeduplicationWindow:  5 * time.Minute,
		EnableCorrelation:    true,
		CorrelationWindow:    10 * time.Minute,
		MaxCorrelationEvents: 100,
		SeverityMapping: map[string]string{
			"1.3.6.1.6.3.1.1.5.1": "critical", // coldStart
			"1.3.6.1.6.3.1.1.5.2": "critical", // warmStart
			"1.3.6.1.6.3.1.1.5.3": "major",    // linkDown
			"1.3.6.1.6.3.1.1.5.4": "info",     // linkUp
			"1.3.6.1.6.3.1.1.5.5": "major",    // authenticationFailure
		},
	}
}

// CorrelationRule represents a rule for correlating events
type CorrelationRule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Enabled     bool              `json:"enabled"`
	Conditions  []RuleCondition   `json:"conditions"`
	Actions     []RuleAction      `json:"actions"`
	TimeWindow  time.Duration     `json:"time_window"`
	MaxEvents   int               `json:"max_events"`
	Metadata    map[string]string `json:"metadata"`
}

// RuleCondition represents a condition in a correlation rule
type RuleCondition struct {
	Field    string `json:"field"`
	Operator string `json:"operator"` // equals, contains, matches, greater_than, less_than
	Value    string `json:"value"`
	Negate   bool   `json:"negate"`
}

// RuleAction represents an action to take when a rule matches
type RuleAction struct {
	Type       string            `json:"type"` // set_severity, set_status, correlate, suppress, acknowledge
	Parameters map[string]string `json:"parameters"`
}

// EventGroup represents a group of correlated events
type EventGroup struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Events       []EventInterface `json:"events"`
	FirstSeen    time.Time        `json:"first_seen"`
	LastSeen     time.Time        `json:"last_seen"`
	Count        int              `json:"count"`
	Severity     string           `json:"severity"`
	Status       string           `json:"status"`
	RuleID       string           `json:"rule_id"`
	Metadata     map[string]any   `json:"metadata"`
	Acknowledged bool             `json:"acknowledged"`
}

// CorrelatorStats tracks correlator statistics
type CorrelatorStats struct {
	EventsProcessed    int64            `json:"events_processed"`
	EventsCorrelated   int64            `json:"events_correlated"`
	EventsDeduplicated int64            `json:"events_deduplicated"`
	EventsSuppressed   int64            `json:"events_suppressed"`
	ActiveGroups       int              `json:"active_groups"`
	RulesMatched       map[string]int64 `json:"rules_matched"`
	ProcessingTime     time.Duration    `json:"processing_time"`
	AverageGroupSize   float64          `json:"average_group_size"`
}

// Correlator provides event correlation and deduplication services
type Correlator struct {
	config       *CorrelatorConfig
	storage      StorageInterface
	rules        map[string]*CorrelationRule
	groups       map[string]*EventGroup
	recentEvents map[string]*RecentEvent
	mu           sync.RWMutex
	stats        *CorrelatorStats
}

// RecentEvent tracks recent events for deduplication
type RecentEvent struct {
	Hash      string    `json:"hash"`
	Count     int       `json:"count"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
	EventID   int64     `json:"event_id"`
}

// NewCorrelator creates a new event correlator
func NewCorrelator(cfg config.Provider, storage StorageInterface) (*Correlator, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration provider cannot be nil")
	}
	if storage == nil {
		return nil, fmt.Errorf("storage cannot be nil")
	}

	// Load configuration
	correlatorConfig := DefaultCorrelatorConfig()

	if dedupe, err := cfg.GetBool("correlator.enable_deduplication", correlatorConfig.EnableDeduplication); err == nil {
		correlatorConfig.EnableDeduplication = dedupe
	}

	if dedupeWindow, err := cfg.GetDuration("correlator.deduplication_window", correlatorConfig.DeduplicationWindow); err == nil {
		correlatorConfig.DeduplicationWindow = dedupeWindow
	}

	if correlation, err := cfg.GetBool("correlator.enable_correlation", correlatorConfig.EnableCorrelation); err == nil {
		correlatorConfig.EnableCorrelation = correlation
	}

	if corrWindow, err := cfg.GetDuration("correlator.correlation_window", correlatorConfig.CorrelationWindow); err == nil {
		correlatorConfig.CorrelationWindow = corrWindow
	}

	correlator := &Correlator{
		config:       correlatorConfig,
		storage:      storage,
		rules:        make(map[string]*CorrelationRule),
		groups:       make(map[string]*EventGroup),
		recentEvents: make(map[string]*RecentEvent),
		stats:        &CorrelatorStats{RulesMatched: make(map[string]int64)},
	}

	// Load default rules
	correlator.loadDefaultRules()

	return correlator, nil
}

// ProcessEvent processes an event through the correlation engine
func (c *Correlator) ProcessEvent(packet *types.SNMPPacket, sourceIP string, enrichedData map[string]any) (map[string]any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	startTime := time.Now()
	c.stats.EventsProcessed++

	// Generate event hash for deduplication
	eventHash := c.generateEventHash(packet, sourceIP)

	// Check for deduplication
	if c.config.EnableDeduplication {
		if duplicate := c.checkDuplication(eventHash); duplicate != nil {
			c.stats.EventsDeduplicated++
			c.updateRecentEvent(duplicate, eventHash)

			// Update enriched data with deduplication info
			enrichedData["is_duplicate"] = true
			enrichedData["duplicate_count"] = duplicate.Count
			enrichedData["first_seen"] = duplicate.FirstSeen

			c.stats.ProcessingTime += time.Since(startTime)
			return enrichedData, nil
		}
	}

	// Flapping detection removed for simplification

	// Apply severity mapping
	c.applySeverityMapping(packet, enrichedData)

	// Apply correlation rules
	if c.config.EnableCorrelation {
		c.applyCorrelationRules(packet, sourceIP, enrichedData)
	}

	// Track recent event
	c.trackRecentEvent(eventHash)

	c.stats.ProcessingTime += time.Since(startTime)
	return enrichedData, nil
}

// generateEventHash generates a hash for event identification
func (c *Correlator) generateEventHash(packet *types.SNMPPacket, sourceIP string) string {
	// Extract trap OID from varbinds
	trapOID := ""
	if len(packet.Varbinds) > 1 && packet.Varbinds[1].OID == "1.3.6.1.6.3.1.1.4.1.0" {
		if oidValue, ok := packet.Varbinds[1].Value.(string); ok {
			trapOID = oidValue
		}
	}

	// Create hash from source IP, trap OID, and community
	hashInput := fmt.Sprintf("%s:%s:%s", sourceIP, trapOID, packet.Community)
	hasher := sha256.Sum256([]byte(hashInput))
	hash := fmt.Sprintf("%x", hasher)
	return hash
}

// checkDuplication checks if an event is a duplicate
func (c *Correlator) checkDuplication(eventHash string) *RecentEvent {
	if recent, exists := c.recentEvents[eventHash]; exists {
		if time.Since(recent.LastSeen) <= c.config.DeduplicationWindow {
			return recent
		}
		// Clean up old entry
		delete(c.recentEvents, eventHash)
	}
	return nil
}

// updateRecentEvent updates a recent event's count and timestamp
func (c *Correlator) updateRecentEvent(recent *RecentEvent, eventHash string) {
	recent.Count++
	recent.LastSeen = time.Now()
	c.recentEvents[eventHash] = recent
}

// trackRecentEvent tracks a new event for deduplication
func (c *Correlator) trackRecentEvent(eventHash string) {
	now := time.Now()
	c.recentEvents[eventHash] = &RecentEvent{
		Hash:      eventHash,
		Count:     1,
		FirstSeen: now,
		LastSeen:  now,
	}
}

// applySeverityMapping applies severity mapping based on trap OID
func (c *Correlator) applySeverityMapping(packet *types.SNMPPacket, enrichedData map[string]any) {
	// Extract trap OID from varbinds
	trapOID := ""
	if len(packet.Varbinds) > 1 && packet.Varbinds[1].OID == "1.3.6.1.6.3.1.1.4.1.0" {
		if oidValue, ok := packet.Varbinds[1].Value.(string); ok {
			trapOID = oidValue
		}
	}

	// Apply severity mapping
	if severity, exists := c.config.SeverityMapping[trapOID]; exists {
		enrichedData["severity"] = severity
	} else {
		// Default severity
		enrichedData["severity"] = "info"
	}
}

// applyCorrelationRules applies correlation rules to the event
func (c *Correlator) applyCorrelationRules(packet *types.SNMPPacket, sourceIP string, enrichedData map[string]any) {
	for _, rule := range c.rules {
		if !rule.Enabled {
			continue
		}

		if c.evaluateRule(rule, packet, sourceIP, enrichedData) {
			c.stats.RulesMatched[rule.ID]++
			c.executeRuleActions(rule, packet, sourceIP, enrichedData)
		}
	}
}

// evaluateRule evaluates if a rule matches the current event
func (c *Correlator) evaluateRule(rule *CorrelationRule, packet *types.SNMPPacket, sourceIP string, enrichedData map[string]any) bool {
	for _, condition := range rule.Conditions {
		if !c.evaluateCondition(condition, packet, sourceIP, enrichedData) {
			return false // All conditions must match
		}
	}
	return true
}

// evaluateCondition evaluates a single rule condition
func (c *Correlator) evaluateCondition(condition RuleCondition, packet *types.SNMPPacket, sourceIP string, enrichedData map[string]any) bool {
	var fieldValue string

	// Get field value
	switch condition.Field {
	case "source_ip":
		fieldValue = sourceIP
	case "community":
		fieldValue = packet.Community
	case "trap_oid":
		if len(packet.Varbinds) > 1 && packet.Varbinds[1].OID == "1.3.6.1.6.3.1.1.4.1.0" {
			if oidValue, ok := packet.Varbinds[1].Value.(string); ok {
				fieldValue = oidValue
			}
		}
	case "severity":
		if sev, exists := enrichedData["severity"]; exists {
			if sevStr, ok := sev.(string); ok {
				fieldValue = sevStr
			}
		}
	default:
		// Check enriched data
		if value, exists := enrichedData[condition.Field]; exists {
			fieldValue = fmt.Sprintf("%v", value)
		}
	}

	// Evaluate condition
	result := false
	switch condition.Operator {
	case "equals":
		result = fieldValue == condition.Value
	case "contains":
		result = strings.Contains(fieldValue, condition.Value)
	case "matches":
		// Simple pattern matching (could be enhanced with regex)
		result = strings.Contains(fieldValue, condition.Value)
	case "not_equals":
		result = fieldValue != condition.Value
	}

	// Apply negation if specified
	if condition.Negate {
		result = !result
	}

	return result
}

// executeRuleActions executes the actions for a matched rule
func (c *Correlator) executeRuleActions(rule *CorrelationRule, packet *types.SNMPPacket, sourceIP string, enrichedData map[string]any) {
	for _, action := range rule.Actions {
		switch action.Type {
		case "set_severity":
			if severity, exists := action.Parameters["severity"]; exists {
				enrichedData["severity"] = severity
			}
		case "set_status":
			if status, exists := action.Parameters["status"]; exists {
				enrichedData["status"] = status
			}
		case "correlate":
			c.correlateEvent(rule, packet, sourceIP, enrichedData)
		case "suppress":
			enrichedData["suppressed"] = true
			c.stats.EventsSuppressed++
		}
	}
}

// correlateEvent correlates an event with existing groups
func (c *Correlator) correlateEvent(rule *CorrelationRule, _ *types.SNMPPacket, sourceIP string, enrichedData map[string]any) {
	groupID := fmt.Sprintf("%s:%s", rule.ID, sourceIP)

	if group, exists := c.groups[groupID]; exists {
		// Add to existing group
		group.Count++
		group.LastSeen = time.Now()
		enrichedData["correlation_id"] = groupID
		c.stats.EventsCorrelated++
	} else {
		// Create new group
		group := &EventGroup{
			ID:        groupID,
			Name:      rule.Name,
			FirstSeen: time.Now(),
			LastSeen:  time.Now(),
			Count:     1,
			RuleID:    rule.ID,
			Metadata:  make(map[string]any),
		}

		if severity, exists := enrichedData["severity"]; exists {
			if sevStr, ok := severity.(string); ok {
				group.Severity = sevStr
			}
		}

		c.groups[groupID] = group
		enrichedData["correlation_id"] = groupID
		c.stats.EventsCorrelated++
	}
}

// loadDefaultRules loads default correlation rules
func (c *Correlator) loadDefaultRules() {
	// Link state correlation rule
	linkStateRule := &CorrelationRule{
		ID:          "link_state_correlation",
		Name:        "Link State Events",
		Description: "Correlates link up/down events from the same source",
		Enabled:     true,
		Conditions: []RuleCondition{
			{
				Field:    "trap_oid",
				Operator: "contains",
				Value:    "1.3.6.1.6.3.1.1.5",
			},
		},
		Actions: []RuleAction{
			{
				Type: "correlate",
				Parameters: map[string]string{
					"group_by": "source_ip",
				},
			},
		},
		TimeWindow: 10 * time.Minute,
		MaxEvents:  50,
	}

	c.rules[linkStateRule.ID] = linkStateRule

	// Authentication failure rule
	authFailRule := &CorrelationRule{
		ID:          "auth_failure_correlation",
		Name:        "Authentication Failures",
		Description: "Correlates authentication failure events",
		Enabled:     true,
		Conditions: []RuleCondition{
			{
				Field:    "trap_oid",
				Operator: "equals",
				Value:    "1.3.6.1.6.3.1.1.5.5",
			},
		},
		Actions: []RuleAction{
			{
				Type: "set_severity",
				Parameters: map[string]string{
					"severity": "major",
				},
			},
			{
				Type: "correlate",
				Parameters: map[string]string{
					"group_by": "source_ip",
				},
			},
		},
		TimeWindow: 5 * time.Minute,
		MaxEvents:  10,
	}

	c.rules[authFailRule.ID] = authFailRule
}

// AddRule adds a new correlation rule
func (c *Correlator) AddRule(rule *CorrelationRule) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if rule.ID == "" {
		return fmt.Errorf("rule ID cannot be empty")
	}

	c.rules[rule.ID] = rule
	c.stats.RulesMatched[rule.ID] = 0
	return nil
}

// RemoveRule removes a correlation rule
func (c *Correlator) RemoveRule(ruleID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.rules[ruleID]; !exists {
		return fmt.Errorf("rule not found: %s", ruleID)
	}

	delete(c.rules, ruleID)
	delete(c.stats.RulesMatched, ruleID)
	return nil
}

// GetRule returns a correlation rule by ID
func (c *Correlator) GetRule(ruleID string) (*CorrelationRule, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	rule, exists := c.rules[ruleID]
	return rule, exists
}

// GetAllRules returns all correlation rules
func (c *Correlator) GetAllRules() map[string]*CorrelationRule {
	c.mu.RLock()
	defer c.mu.RUnlock()

	rules := make(map[string]*CorrelationRule)
	maps.Copy(rules, c.rules)
	return rules
}

// GetEventGroups returns all active event groups
func (c *Correlator) GetEventGroups() map[string]*EventGroup {
	c.mu.RLock()
	defer c.mu.RUnlock()

	groups := make(map[string]*EventGroup)
	maps.Copy(groups, c.groups)
	return groups
}

// GetEventGroup returns a specific event group
func (c *Correlator) GetEventGroup(groupID string) (*EventGroup, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	group, exists := c.groups[groupID]
	return group, exists
}

// AcknowledgeGroup acknowledges all events in a group
func (c *Correlator) AcknowledgeGroup(groupID, ackBy string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	group, exists := c.groups[groupID]
	if !exists {
		return fmt.Errorf("group not found: %s", groupID)
	}

	group.Acknowledged = true

	// Acknowledge all events in the group
	for _, event := range group.Events {
		eventID := event.GetID()
		if err := c.storage.AcknowledgeEvent(eventID, ackBy); err != nil {
			return fmt.Errorf("failed to acknowledge event %d: %w", eventID, err)
		}
	}

	return nil
}

// CleanupExpiredGroups removes expired event groups
func (c *Correlator) CleanupExpiredGroups() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for groupID, group := range c.groups {
		// Remove groups older than correlation window
		if now.Sub(group.LastSeen) > c.config.CorrelationWindow {
			delete(c.groups, groupID)
		}
	}

	// Cleanup recent events
	for eventHash, recent := range c.recentEvents {
		if now.Sub(recent.LastSeen) > c.config.DeduplicationWindow {
			delete(c.recentEvents, eventHash)
		}
	}

	// Flapping state cleanup removed for simplification
}

// GetStats returns correlator statistics
func (c *Correlator) GetStats() *CorrelatorStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := *c.stats
	stats.ActiveGroups = len(c.groups)

	// Calculate average group size
	if stats.ActiveGroups > 0 {
		totalEvents := int64(0)
		for _, group := range c.groups {
			totalEvents += int64(group.Count)
		}
		stats.AverageGroupSize = float64(totalEvents) / float64(stats.ActiveGroups)
	}

	return &stats
}

// GetRecentEvents returns recent events for debugging
func (c *Correlator) GetRecentEvents() map[string]*RecentEvent {
	c.mu.RLock()
	defer c.mu.RUnlock()

	recent := make(map[string]*RecentEvent)
	maps.Copy(recent, c.recentEvents)
	return recent
}

// UpdateConfig updates the correlator configuration
func (c *Correlator) UpdateConfig(config *CorrelatorConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config = config
}

// GetConfig returns the current correlator configuration
func (c *Correlator) GetConfig() *CorrelatorConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config
}

// ResetStats resets correlator statistics
func (c *Correlator) ResetStats() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stats = &CorrelatorStats{RulesMatched: make(map[string]int64)}
}
