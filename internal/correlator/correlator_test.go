package correlator

import (
	"github.com/geekxflood/nereus/internal/types"
	"testing"
	"time"
)

// mockConfigProvider implements the config.Provider interface for testing.
type mockConfigProvider struct {
	values map[string]any
}

func newMockConfigProvider() *mockConfigProvider {
	return &mockConfigProvider{
		values: map[string]any{
			"correlator.enable_deduplication":   true,
			"correlator.deduplication_window":   "5m",
			"correlator.enable_correlation":     true,
			"correlator.correlation_window":     "10m",
			"correlator.max_correlation_events": 100,
		},
	}
}

func (m *mockConfigProvider) GetString(path string, defaultValue ...string) (string, error) {
	if val, exists := m.values[path]; exists {
		if str, ok := val.(string); ok {
			return str, nil
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0], nil
	}
	return "", nil
}

func (m *mockConfigProvider) GetInt(path string, defaultValue ...int) (int, error) {
	if val, exists := m.values[path]; exists {
		if i, ok := val.(int); ok {
			return i, nil
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0], nil
	}
	return 0, nil
}

func (m *mockConfigProvider) GetFloat(path string, defaultValue ...float64) (float64, error) {
	if val, exists := m.values[path]; exists {
		if f, ok := val.(float64); ok {
			return f, nil
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0], nil
	}
	return 0, nil
}

func (m *mockConfigProvider) GetBool(path string, defaultValue ...bool) (bool, error) {
	if val, exists := m.values[path]; exists {
		if b, ok := val.(bool); ok {
			return b, nil
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0], nil
	}
	return false, nil
}

func (m *mockConfigProvider) GetDuration(path string, defaultValue ...time.Duration) (time.Duration, error) {
	if val, exists := m.values[path]; exists {
		if str, ok := val.(string); ok {
			return time.ParseDuration(str)
		}
		if d, ok := val.(time.Duration); ok {
			return d, nil
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0], nil
	}
	return 0, nil
}

func (m *mockConfigProvider) GetStringSlice(path string, defaultValue ...[]string) ([]string, error) {
	if val, exists := m.values[path]; exists {
		if slice, ok := val.([]string); ok {
			return slice, nil
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0], nil
	}
	return nil, nil
}

func (m *mockConfigProvider) GetMap(path string) (map[string]any, error) {
	if val, exists := m.values[path]; exists {
		if m, ok := val.(map[string]any); ok {
			return m, nil
		}
	}
	return nil, nil
}

func (m *mockConfigProvider) Exists(path string) bool {
	_, exists := m.values[path]
	return exists
}

func (m *mockConfigProvider) Validate() error {
	return nil
}

// mockStorage implements a minimal storage interface for testing
type mockStorage struct{}

func (m *mockStorage) StoreEvent(packet *types.SNMPPacket, sourceIP string, enrichedData map[string]any) error {
	return nil
}

func (m *mockStorage) AcknowledgeEvent(id int64, ackBy string) error {
	return nil
}

func (m *mockStorage) GetEvent(id int64) (*mockEvent, error) {
	return &mockEvent{ID: id}, nil
}

// mockEvent represents a mock event for testing
type mockEvent struct {
	ID int64
}

func TestNewCorrelator(t *testing.T) {
	cfg := newMockConfigProvider()
	mockStore := &mockStorage{}

	correlator, err := NewCorrelator(cfg, mockStore)
	if err != nil {
		t.Fatalf("Failed to create correlator: %v", err)
	}

	if correlator == nil {
		t.Fatal("Correlator is nil")
	}

	if correlator.config == nil {
		t.Error("Config not set")
	}

	if correlator.storage == nil {
		t.Error("Storage not set")
	}

	if correlator.rules == nil {
		t.Error("Rules map not initialized")
	}

	if correlator.stats == nil {
		t.Error("Stats not initialized")
	}
}

func TestNewCorrelatorNilConfig(t *testing.T) {
	mockStore := &mockStorage{}

	_, err := NewCorrelator(nil, mockStore)
	if err == nil {
		t.Fatal("Expected error for nil config, got nil")
	}

	expectedMsg := "configuration provider cannot be nil"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestNewCorrelatorNilStorage(t *testing.T) {
	cfg := newMockConfigProvider()

	_, err := NewCorrelator(cfg, nil)
	if err == nil {
		t.Fatal("Expected error for nil storage, got nil")
	}

	expectedMsg := "storage cannot be nil"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestDefaultCorrelatorConfig(t *testing.T) {
	config := DefaultCorrelatorConfig()

	if config == nil {
		t.Fatal("Config is nil")
	}

	if !config.EnableDeduplication {
		t.Error("Deduplication should be enabled by default")
	}

	if config.DeduplicationWindow <= 0 {
		t.Error("Invalid deduplication window")
	}

	if !config.EnableCorrelation {
		t.Error("Correlation should be enabled by default")
	}

	if config.SeverityMapping == nil {
		t.Error("Severity mapping not initialized")
	}
}

func TestGenerateEventHash(t *testing.T) {
	cfg := newMockConfigProvider()
	mockStore := &mockStorage{}
	correlator, err := NewCorrelator(cfg, mockStore)
	if err != nil {
		t.Fatalf("Failed to create correlator: %v", err)
	}

	packet := &types.SNMPPacket{
		Version:   types.VersionSNMPv2c,
		Community: "public",
		Varbinds: []types.Varbind{
			{
				OID:   "1.3.6.1.6.3.1.1.4.1.0",
				Type:  types.TypeObjectIdentifier,
				Value: "1.3.6.1.6.3.1.1.5.1",
			},
		},
	}

	hash1 := correlator.generateEventHash(packet, "192.168.1.1")
	hash2 := correlator.generateEventHash(packet, "192.168.1.1")
	hash3 := correlator.generateEventHash(packet, "192.168.1.2")

	if hash1 != hash2 {
		t.Error("Same parameters should generate same hash")
	}

	if hash1 == hash3 {
		t.Error("Different parameters should generate different hash")
	}

	if hash1 == "" {
		t.Error("Hash should not be empty")
	}
}

func TestProcessEvent(t *testing.T) {
	cfg := newMockConfigProvider()
	mockStore := &mockStorage{}
	correlator, err := NewCorrelator(cfg, mockStore)
	if err != nil {
		t.Fatalf("Failed to create correlator: %v", err)
	}

	packet := &types.SNMPPacket{
		Version:   types.VersionSNMPv2c,
		Community: "public",
		PDUType:   types.PDUTypeTrapV2,
		RequestID: 12345,
		Varbinds: []types.Varbind{
			{
				OID:   "1.3.6.1.2.1.1.3.0",
				Type:  types.TypeTimeTicks,
				Value: uint32(12345),
			},
			{
				OID:   "1.3.6.1.6.3.1.1.4.1.0",
				Type:  types.TypeObjectIdentifier,
				Value: "1.3.6.1.6.3.1.1.5.1",
			},
		},
		Timestamp: time.Now(),
	}

	enrichedData := map[string]any{
		"test_field": "test_value",
	}

	// Process event
	result, err := correlator.ProcessEvent(packet, "192.168.1.100", enrichedData)
	if err != nil {
		t.Errorf("Failed to process event: %v", err)
	}

	if result == nil {
		t.Fatal("Result is nil")
	}

	// Check that severity was applied
	if severity, exists := result["severity"]; !exists {
		t.Error("Severity not set")
	} else if severity != "critical" {
		t.Errorf("Expected severity 'critical', got '%v'", severity)
	}

	// Check stats
	stats := correlator.GetStats()
	if stats.EventsProcessed != 1 {
		t.Errorf("Expected 1 event processed, got %d", stats.EventsProcessed)
	}
}

func TestDeduplication(t *testing.T) {
	cfg := newMockConfigProvider()
	mockStore := &mockStorage{}
	correlator, err := NewCorrelator(cfg, mockStore)
	if err != nil {
		t.Fatalf("Failed to create correlator: %v", err)
	}

	packet := &types.SNMPPacket{
		Version:   types.VersionSNMPv2c,
		Community: "public",
		Varbinds: []types.Varbind{
			{
				OID:   "1.3.6.1.6.3.1.1.4.1.0",
				Type:  types.TypeObjectIdentifier,
				Value: "1.3.6.1.6.3.1.1.5.1",
			},
		},
	}

	enrichedData := map[string]any{}

	// Process same event twice
	_, err = correlator.ProcessEvent(packet, "192.168.1.100", enrichedData)
	if err != nil {
		t.Errorf("Failed to process first event: %v", err)
	}

	result2, err := correlator.ProcessEvent(packet, "192.168.1.100", enrichedData)
	if err != nil {
		t.Errorf("Failed to process second event: %v", err)
	}

	// Second event should be marked as duplicate
	if isDuplicate, exists := result2["is_duplicate"]; !exists || !isDuplicate.(bool) {
		t.Error("Second event should be marked as duplicate")
	}

	// Check stats
	stats := correlator.GetStats()
	if stats.EventsDeduplicated != 1 {
		t.Errorf("Expected 1 deduplicated event, got %d", stats.EventsDeduplicated)
	}
}

func TestAddRule(t *testing.T) {
	cfg := newMockConfigProvider()
	mockStore := &mockStorage{}
	correlator, err := NewCorrelator(cfg, mockStore)
	if err != nil {
		t.Fatalf("Failed to create correlator: %v", err)
	}

	rule := &CorrelationRule{
		ID:          "test_rule",
		Name:        "Test Rule",
		Description: "Test correlation rule",
		Enabled:     true,
		Conditions: []RuleCondition{
			{
				Field:    "source_ip",
				Operator: "equals",
				Value:    "192.168.1.100",
			},
		},
		Actions: []RuleAction{
			{
				Type: "set_severity",
				Parameters: map[string]string{
					"severity": "major",
				},
			},
		},
	}

	err = correlator.AddRule(rule)
	if err != nil {
		t.Errorf("Failed to add rule: %v", err)
	}

	// Check that rule was added
	retrievedRule, exists := correlator.GetRule("test_rule")
	if !exists {
		t.Error("Rule not found after adding")
	}

	if retrievedRule.Name != "Test Rule" {
		t.Errorf("Expected rule name 'Test Rule', got '%s'", retrievedRule.Name)
	}
}

func TestRemoveRule(t *testing.T) {
	cfg := newMockConfigProvider()
	mockStore := &mockStorage{}
	correlator, err := NewCorrelator(cfg, mockStore)
	if err != nil {
		t.Fatalf("Failed to create correlator: %v", err)
	}

	rule := &CorrelationRule{
		ID:      "test_rule",
		Name:    "Test Rule",
		Enabled: true,
	}

	// Add rule
	err = correlator.AddRule(rule)
	if err != nil {
		t.Errorf("Failed to add rule: %v", err)
	}

	// Remove rule
	err = correlator.RemoveRule("test_rule")
	if err != nil {
		t.Errorf("Failed to remove rule: %v", err)
	}

	// Check that rule was removed
	_, exists := correlator.GetRule("test_rule")
	if exists {
		t.Error("Rule still exists after removal")
	}
}

func TestGetStats(t *testing.T) {
	cfg := newMockConfigProvider()
	mockStore := &mockStorage{}
	correlator, err := NewCorrelator(cfg, mockStore)
	if err != nil {
		t.Fatalf("Failed to create correlator: %v", err)
	}

	stats := correlator.GetStats()
	if stats == nil {
		t.Fatal("Stats is nil")
	}

	// Check that stats structure is properly initialized
	if stats.EventsProcessed < 0 {
		t.Error("Invalid EventsProcessed count")
	}

	if stats.RulesMatched == nil {
		t.Error("RulesMatched not initialized")
	}
}

func TestCleanupExpiredGroups(t *testing.T) {
	cfg := newMockConfigProvider()
	mockStore := &mockStorage{}
	correlator, err := NewCorrelator(cfg, mockStore)
	if err != nil {
		t.Fatalf("Failed to create correlator: %v", err)
	}

	// Test cleanup (should not error even with empty state)
	correlator.CleanupExpiredGroups()
}
