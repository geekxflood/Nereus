package reload

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/geekxflood/common/config"
	"github.com/geekxflood/common/logging"
)

// MockConfigManager implements config.Manager for testing
type MockConfigManager struct {
	data map[string]any
}

func NewMockConfigManager() *MockConfigManager {
	return &MockConfigManager{
		data: make(map[string]any),
	}
}

func (m *MockConfigManager) Set(key string, value any) {
	m.data[key] = value
}

func (m *MockConfigManager) Get(key string) (any, error) {
	if value, exists := m.data[key]; exists {
		return value, nil
	}
	return nil, fmt.Errorf("key not found: %s", key)
}

func (m *MockConfigManager) GetString(key string, defaultValue ...string) (string, error) {
	if value, exists := m.data[key]; exists {
		if str, ok := value.(string); ok {
			return str, nil
		}
		return fmt.Sprintf("%v", value), nil
	}
	if len(defaultValue) > 0 {
		return defaultValue[0], nil
	}
	return "", fmt.Errorf("key not found: %s", key)
}

func (m *MockConfigManager) GetInt(key string, defaultValue ...int) (int, error) {
	if value, exists := m.data[key]; exists {
		if i, ok := value.(int); ok {
			return i, nil
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0], nil
	}
	return 0, fmt.Errorf("key not found: %s", key)
}

func (m *MockConfigManager) GetBool(key string, defaultValue ...bool) (bool, error) {
	if value, exists := m.data[key]; exists {
		if b, ok := value.(bool); ok {
			return b, nil
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0], nil
	}
	return false, fmt.Errorf("key not found: %s", key)
}

func (m *MockConfigManager) GetDuration(key string, defaultValue ...time.Duration) (time.Duration, error) {
	if value, exists := m.data[key]; exists {
		if d, ok := value.(time.Duration); ok {
			return d, nil
		}
		if str, ok := value.(string); ok {
			return time.ParseDuration(str)
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0], nil
	}
	return 0, fmt.Errorf("key not found: %s", key)
}

func (m *MockConfigManager) GetFloat(key string, defaultValue ...float64) (float64, error) {
	if value, exists := m.data[key]; exists {
		if f, ok := value.(float64); ok {
			return f, nil
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0], nil
	}
	return 0, fmt.Errorf("key not found: %s", key)
}

func (m *MockConfigManager) GetStringSlice(key string, defaultValue ...[]string) ([]string, error) {
	if value, exists := m.data[key]; exists {
		if slice, ok := value.([]string); ok {
			return slice, nil
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0], nil
	}
	return nil, fmt.Errorf("key not found: %s", key)
}

func (m *MockConfigManager) GetMap(key string) (map[string]any, error) {
	if value, exists := m.data[key]; exists {
		if mapVal, ok := value.(map[string]any); ok {
			return mapVal, nil
		}
	}
	return nil, fmt.Errorf("key not found: %s", key)
}

func (m *MockConfigManager) IsSet(key string) bool {
	_, exists := m.data[key]
	return exists
}

func (m *MockConfigManager) AllKeys() []string {
	keys := make([]string, 0, len(m.data))
	for key := range m.data {
		keys = append(keys, key)
	}
	return keys
}

func (m *MockConfigManager) AllSettings() map[string]any {
	result := make(map[string]any)
	for key, value := range m.data {
		result[key] = value
	}
	return result
}

func (m *MockConfigManager) Exists(key string) bool {
	_, exists := m.data[key]
	return exists
}

func (m *MockConfigManager) Validate() error {
	return nil
}

func (m *MockConfigManager) Reload() error {
	return nil
}

func (m *MockConfigManager) Close() error {
	return nil
}

func (m *MockConfigManager) OnConfigChange(callback func(error)) {
	// Mock implementation - do nothing
}

func (m *MockConfigManager) StartHotReload(ctx context.Context) error {
	return nil
}

func (m *MockConfigManager) StopHotReload() {
	// Mock implementation - do nothing
}

// MockComponentReloader implements ComponentReloader for testing
type MockComponentReloader struct {
	name        string
	reloadCount int
	shouldFail  bool
}

func NewMockComponentReloader(name string) *MockComponentReloader {
	return &MockComponentReloader{
		name: name,
	}
}

func (m *MockComponentReloader) Reload(configProvider config.Provider) error {
	m.reloadCount++
	if m.shouldFail {
		return fmt.Errorf("mock reload failure for %s", m.name)
	}
	return nil
}

func (m *MockComponentReloader) GetReloadStats() map[string]any {
	return map[string]any{
		"reload_count": m.reloadCount,
	}
}

func (m *MockComponentReloader) SetShouldFail(shouldFail bool) {
	m.shouldFail = shouldFail
}

// createTestLogger creates a logger for testing
func createTestLogger() logging.Logger {
	config := logging.Config{
		Level:  "debug",
		Format: "json",
	}
	logger, _, _ := logging.NewLogger(config)
	return logger
}

func TestDefaultReloadConfig(t *testing.T) {
	config := DefaultReloadConfig()

	if !config.Enabled {
		t.Error("Expected reload to be enabled by default")
	}

	if !config.WatchConfigFile {
		t.Error("Expected config file watching to be enabled by default")
	}

	if !config.WatchMIBDirectories {
		t.Error("Expected MIB directory watching to be enabled by default")
	}

	if config.ReloadDelay != 2*time.Second {
		t.Errorf("Expected reload delay 2s, got %v", config.ReloadDelay)
	}

	if config.MaxReloadAttempts != 3 {
		t.Errorf("Expected max reload attempts 3, got %d", config.MaxReloadAttempts)
	}

	if !config.PreserveState {
		t.Error("Expected preserve state to be enabled by default")
	}

	if !config.ValidateBeforeReload {
		t.Error("Expected validate before reload to be enabled by default")
	}
}

func TestNewReloadManager(t *testing.T) {
	configManager := NewMockConfigManager()
	logger := createTestLogger()

	manager, err := NewReloadManager(configManager, logger)
	if err != nil {
		t.Fatalf("Failed to create reload manager: %v", err)
	}

	if manager == nil {
		t.Fatal("Reload manager is nil")
	}

	if manager.config == nil {
		t.Error("Reload config is nil")
	}

	if manager.configManager == nil {
		t.Error("Config manager is nil")
	}

	if manager.logger == nil {
		t.Error("Logger is nil")
	}

	if manager.components == nil {
		t.Error("Components map is nil")
	}

	if manager.stats == nil {
		t.Error("Stats is nil")
	}
}

func TestNewReloadManagerWithNilConfig(t *testing.T) {
	logger := createTestLogger()

	_, err := NewReloadManager(nil, logger)
	if err == nil {
		t.Error("Expected error for nil config manager")
	}
}

func TestNewReloadManagerWithNilLogger(t *testing.T) {
	configManager := NewMockConfigManager()

	_, err := NewReloadManager(configManager, nil)
	if err == nil {
		t.Error("Expected error for nil logger")
	}
}

func TestReloadManagerDisabled(t *testing.T) {
	configManager := NewMockConfigManager()
	configManager.Set("reload.enabled", false)
	logger := createTestLogger()

	manager, err := NewReloadManager(configManager, logger)
	if err != nil {
		t.Fatalf("Failed to create reload manager: %v", err)
	}

	// Start should succeed but not actually start watcher
	err = manager.Start()
	if err != nil {
		t.Fatalf("Failed to start disabled reload manager: %v", err)
	}

	// Stop should succeed
	err = manager.Stop()
	if err != nil {
		t.Fatalf("Failed to stop disabled reload manager: %v", err)
	}
}

func TestRegisterComponent(t *testing.T) {
	configManager := NewMockConfigManager()
	logger := createTestLogger()

	manager, err := NewReloadManager(configManager, logger)
	if err != nil {
		t.Fatalf("Failed to create reload manager: %v", err)
	}

	component := NewMockComponentReloader("test-component")
	manager.RegisterComponent("test-component", component)

	if len(manager.components) != 1 {
		t.Errorf("Expected 1 component, got %d", len(manager.components))
	}

	if _, exists := manager.components["test-component"]; !exists {
		t.Error("Component not found in registry")
	}
}

func TestUnregisterComponent(t *testing.T) {
	configManager := NewMockConfigManager()
	logger := createTestLogger()

	manager, err := NewReloadManager(configManager, logger)
	if err != nil {
		t.Fatalf("Failed to create reload manager: %v", err)
	}

	component := NewMockComponentReloader("test-component")
	manager.RegisterComponent("test-component", component)
	manager.UnregisterComponent("test-component")

	if len(manager.components) != 0 {
		t.Errorf("Expected 0 components, got %d", len(manager.components))
	}
}

func TestTriggerReloadDisabled(t *testing.T) {
	configManager := NewMockConfigManager()
	configManager.Set("reload.enabled", false)
	logger := createTestLogger()

	manager, err := NewReloadManager(configManager, logger)
	if err != nil {
		t.Fatalf("Failed to create reload manager: %v", err)
	}

	err = manager.TriggerReload(ReloadTypeConfig, "manual")
	if err == nil {
		t.Error("Expected error when triggering reload on disabled manager")
	}
}

func TestTriggerReload(t *testing.T) {
	configManager := NewMockConfigManager()
	logger := createTestLogger()

	manager, err := NewReloadManager(configManager, logger)
	if err != nil {
		t.Fatalf("Failed to create reload manager: %v", err)
	}

	// Register a test component
	component := NewMockComponentReloader("test-component")
	manager.RegisterComponent("test-component", component)

	// Trigger a reload
	err = manager.TriggerReload(ReloadTypeConfig, "manual")
	if err != nil {
		t.Fatalf("Failed to trigger reload: %v", err)
	}

	// Check that component was reloaded
	if component.reloadCount != 1 {
		t.Errorf("Expected component reload count 1, got %d", component.reloadCount)
	}

	// Check stats
	stats := manager.GetStats()
	if stats.TotalReloads != 1 {
		t.Errorf("Expected total reloads 1, got %d", stats.TotalReloads)
	}

	if stats.SuccessfulReloads != 1 {
		t.Errorf("Expected successful reloads 1, got %d", stats.SuccessfulReloads)
	}

	if stats.ConfigReloads != 1 {
		t.Errorf("Expected config reloads 1, got %d", stats.ConfigReloads)
	}
}

func TestIsMIBFile(t *testing.T) {
	testCases := []struct {
		filePath string
		expected bool
	}{
		{"test.mib", true},
		{"test.txt", true},
		{"test.my", true},
		{"test.json", false},
		{"test.yaml", false},
		{"test", false},
		{"/path/to/test.mib", true},
		{"/path/to/test.conf", false},
	}

	for _, tc := range testCases {
		result := isMIBFile(tc.filePath)
		if result != tc.expected {
			t.Errorf("isMIBFile(%s) = %v, expected %v", tc.filePath, result, tc.expected)
		}
	}
}

func TestComponentTypeIdentification(t *testing.T) {
	// Test MIB component identification
	mibComponents := []string{"mib_loader", "mib_parser", "resolver"}
	for _, comp := range mibComponents {
		if !isMIBComponent(comp) {
			t.Errorf("Expected %s to be identified as MIB component", comp)
		}
	}

	// Test webhook component identification
	webhookComponents := []string{"notifier", "http_client"}
	for _, comp := range webhookComponents {
		if !isWebhookComponent(comp) {
			t.Errorf("Expected %s to be identified as webhook component", comp)
		}
	}

	// Test non-component
	if isMIBComponent("storage") {
		t.Error("Expected storage not to be identified as MIB component")
	}

	if isWebhookComponent("storage") {
		t.Error("Expected storage not to be identified as webhook component")
	}
}

func TestGetRecentEvents(t *testing.T) {
	configManager := NewMockConfigManager()
	logger := createTestLogger()

	manager, err := NewReloadManager(configManager, logger)
	if err != nil {
		t.Fatalf("Failed to create reload manager: %v", err)
	}

	// Add some test events
	for i := 0; i < 5; i++ {
		event := ReloadEvent{
			Type:      ReloadTypeConfig,
			Source:    fmt.Sprintf("test-%d", i),
			Timestamp: time.Now(),
			Success:   true,
		}
		manager.recordEvent(event)
	}

	// Get recent events
	events := manager.GetRecentEvents(3)
	if len(events) != 3 {
		t.Errorf("Expected 3 events, got %d", len(events))
	}

	// Get all events
	allEvents := manager.GetRecentEvents(0)
	if len(allEvents) != 5 {
		t.Errorf("Expected 5 events, got %d", len(allEvents))
	}
}
