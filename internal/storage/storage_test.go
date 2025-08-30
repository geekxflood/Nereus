package storage

import (
	"os"
	"testing"
	"time"

	"github.com/geekxflood/nereus/internal/types"
)

// mockConfigProvider implements the config.Provider interface for testing.
type mockConfigProvider struct {
	values map[string]interface{}
}

func newMockConfigProvider() *mockConfigProvider {
	return &mockConfigProvider{
		values: map[string]interface{}{
			"storage.database_type":     "sqlite3",
			"storage.connection_string": ":memory:",
			"storage.max_connections":   5,
			"storage.retention_days":    7,
			"storage.batch_size":        10,
			"storage.flush_interval":    "1s",
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

func TestNewStorage(t *testing.T) {
	cfg := newMockConfigProvider()
	
	storage, err := NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()
	
	if storage == nil {
		t.Fatal("Storage is nil")
	}
	
	if storage.config == nil {
		t.Error("Config not set")
	}
	
	if storage.db == nil {
		t.Error("Database not initialized")
	}
	
	if storage.stats == nil {
		t.Error("Stats not initialized")
	}
}

func TestNewStorageNilConfig(t *testing.T) {
	_, err := NewStorage(nil)
	if err == nil {
		t.Fatal("Expected error for nil config, got nil")
	}
	
	expectedMsg := "configuration provider cannot be nil"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestDefaultStorageConfig(t *testing.T) {
	config := DefaultStorageConfig()
	
	if config == nil {
		t.Fatal("Config is nil")
	}
	
	if config.DatabaseType == "" {
		t.Error("Database type not set")
	}
	
	if config.ConnectionString == "" {
		t.Error("Connection string not set")
	}
	
	if config.MaxConnections <= 0 {
		t.Error("Invalid max connections")
	}
	
	if config.RetentionDays <= 0 {
		t.Error("Invalid retention days")
	}
}

func TestStoreEvent(t *testing.T) {
	cfg := newMockConfigProvider()
	storage, err := NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create test packet
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

	enrichedData := map[string]interface{}{
		"severity":    "info",
		"trap_name":   "coldStart",
		"description": "System cold start",
	}

	// Store event
	err = storage.StoreEvent(packet, "192.168.1.100", enrichedData)
	if err != nil {
		t.Errorf("Failed to store event: %v", err)
	}

	// Force flush
	storage.mu.Lock()
	err = storage.flushBatch()
	storage.mu.Unlock()
	if err != nil {
		t.Errorf("Failed to flush batch: %v", err)
	}
}

func TestQueryEvents(t *testing.T) {
	cfg := newMockConfigProvider()
	storage, err := NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Store a test event first
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
		},
		Timestamp: time.Now(),
	}

	enrichedData := map[string]interface{}{
		"severity": "info",
	}

	err = storage.StoreEvent(packet, "192.168.1.100", enrichedData)
	if err != nil {
		t.Errorf("Failed to store event: %v", err)
	}

	// Force flush
	storage.mu.Lock()
	storage.flushBatch()
	storage.mu.Unlock()

	// Query events
	query := &EventQuery{
		SourceIP: "192.168.1.100",
		Limit:    10,
	}

	events, err := storage.QueryEvents(query)
	if err != nil {
		t.Errorf("Failed to query events: %v", err)
	}

	if len(events) == 0 {
		t.Error("No events returned from query")
	}
}

func TestGenerateEventHash(t *testing.T) {
	cfg := newMockConfigProvider()
	storage, err := NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	hash1 := storage.generateEventHash("192.168.1.1", "1.3.6.1.6.3.1.1.5.1", "public")
	hash2 := storage.generateEventHash("192.168.1.1", "1.3.6.1.6.3.1.1.5.1", "public")
	hash3 := storage.generateEventHash("192.168.1.2", "1.3.6.1.6.3.1.1.5.1", "public")

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

func TestGetStats(t *testing.T) {
	cfg := newMockConfigProvider()
	storage, err := NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	stats, err := storage.GetStats()
	if err != nil {
		t.Errorf("Failed to get stats: %v", err)
	}

	if stats == nil {
		t.Fatal("Stats is nil")
	}

	// Check that stats structure is properly initialized
	if stats.TotalEvents < 0 {
		t.Error("Invalid TotalEvents count")
	}

	if stats.SeverityBreakdown == nil {
		t.Error("SeverityBreakdown not initialized")
	}

	if stats.StatusBreakdown == nil {
		t.Error("StatusBreakdown not initialized")
	}
}

func TestCleanup(t *testing.T) {
	cfg := newMockConfigProvider()
	storage, err := NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Test cleanup (should not error even with empty database)
	storage.cleanup()
}

func TestAcknowledgeEvent(t *testing.T) {
	cfg := newMockConfigProvider()
	storage, err := NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Try to acknowledge non-existent event
	err = storage.AcknowledgeEvent(999, "test_user")
	if err != nil {
		// This is expected for non-existent event
		t.Logf("Expected error for non-existent event: %v", err)
	}
}

func TestClose(t *testing.T) {
	cfg := newMockConfigProvider()
	storage, err := NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Test close
	err = storage.Close()
	if err != nil {
		t.Errorf("Failed to close storage: %v", err)
	}
}
