package snmp

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// mockConfigProvider implements the config.Provider interface for testing.
type mockConfigProvider struct {
	values map[string]interface{}
}

func newMockConfigProvider() *mockConfigProvider {
	return &mockConfigProvider{
		values: map[string]interface{}{
			"server.host":         "127.0.0.1",
			"server.port":         1162, // Use non-privileged port for testing
			"server.community":    "public",
			"server.max_handlers": 10,
			"server.buffer_size":  8192,
			"server.read_timeout": "5s",
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
	return "", ErrPathNotFound
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
	return 0, ErrPathNotFound
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
	return 0, ErrPathNotFound
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
	return false, ErrPathNotFound
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
	return 0, ErrPathNotFound
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
	return nil, ErrPathNotFound
}

func (m *mockConfigProvider) GetMap(path string) (map[string]any, error) {
	if val, exists := m.values[path]; exists {
		if m, ok := val.(map[string]any); ok {
			return m, nil
		}
	}
	return nil, ErrPathNotFound
}

func (m *mockConfigProvider) Exists(path string) bool {
	_, exists := m.values[path]
	return exists
}

func (m *mockConfigProvider) Validate() error {
	return nil
}

// Define a custom error for path not found to match the expected interface
var ErrPathNotFound = fmt.Errorf("path not found")

func TestNewListener(t *testing.T) {
	cfg := newMockConfigProvider()

	listener, err := NewListener(cfg)
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	if listener == nil {
		t.Fatal("Listener is nil")
	}

	if listener.config != cfg {
		t.Error("Config not set correctly")
	}

	if listener.handlers == nil {
		t.Error("Handlers channel not initialized")
	}
}

func TestNewListenerNilConfig(t *testing.T) {
	_, err := NewListener(nil)
	if err == nil {
		t.Fatal("Expected error for nil config, got nil")
	}

	expectedMsg := "configuration provider cannot be nil"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestListenerStartStop(t *testing.T) {
	cfg := newMockConfigProvider()
	listener, err := NewListener(cfg)
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	// Test initial state
	if listener.IsRunning() {
		t.Error("Listener should not be running initially")
	}

	// Start listener
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = listener.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start listener: %v", err)
	}

	// Test running state
	if !listener.IsRunning() {
		t.Error("Listener should be running after start")
	}

	// Stop listener
	err = listener.Stop()
	if err != nil {
		t.Fatalf("Failed to stop listener: %v", err)
	}

	// Test stopped state
	if listener.IsRunning() {
		t.Error("Listener should not be running after stop")
	}
}

func TestListenerDoubleStart(t *testing.T) {
	cfg := newMockConfigProvider()
	listener, err := NewListener(cfg)
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start listener first time
	err = listener.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start listener: %v", err)
	}
	defer listener.Stop()

	// Try to start again
	err = listener.Start(ctx)
	if err == nil {
		t.Fatal("Expected error when starting already running listener")
	}

	expectedMsg := "listener is already running"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestListenerGetStats(t *testing.T) {
	cfg := newMockConfigProvider()
	listener, err := NewListener(cfg)
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	// Test stats when not running
	stats := listener.GetStats()
	if stats["running"] != false {
		t.Error("Stats should show listener as not running")
	}

	// Start listener and test stats
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = listener.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start listener: %v", err)
	}
	defer listener.Stop()

	stats = listener.GetStats()
	if stats["running"] != true {
		t.Error("Stats should show listener as running")
	}

	if stats["queue_length"] == nil {
		t.Error("Stats should include queue_length")
	}

	if stats["queue_cap"] == nil {
		t.Error("Stats should include queue_cap")
	}

	if stats["local_addr"] == nil {
		t.Error("Stats should include local_addr when running")
	}
}

func TestParseSNMPPacket(t *testing.T) {
	cfg := newMockConfigProvider()
	listener, err := NewListener(cfg)
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	// Test with invalid packet (too short)
	shortPacket := []byte{0x30, 0x01}
	_, err = listener.parseSNMPPacket(shortPacket)
	if err == nil {
		t.Error("Expected error for short packet")
	}

	// Test with invalid packet (wrong tag)
	invalidPacket := []byte{0x31, 0x0a, 0x02, 0x01, 0x00, 0x04, 0x06, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63}
	_, err = listener.parseSNMPPacket(invalidPacket)
	if err == nil {
		t.Error("Expected error for invalid packet tag")
	}

	// Test with valid-looking packet
	validPacket := []byte{0x30, 0x0a, 0x02, 0x01, 0x00, 0x04, 0x06, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63}
	packet, err := listener.parseSNMPPacket(validPacket)
	if err != nil {
		t.Errorf("Unexpected error for valid packet: %v", err)
	}

	if packet == nil {
		t.Error("Packet should not be nil")
	}

	if packet.Version != 1 {
		t.Errorf("Expected version 1, got %d", packet.Version)
	}
}
