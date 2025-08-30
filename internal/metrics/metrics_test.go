package metrics

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/geekxflood/common/logging"
)

// MockConfigProvider implements config.Provider for testing
type MockConfigProvider struct {
	data map[string]any
}

func NewMockConfigProvider() *MockConfigProvider {
	return &MockConfigProvider{
		data: make(map[string]any),
	}
}

func (m *MockConfigProvider) Set(key string, value any) {
	m.data[key] = value
}

func (m *MockConfigProvider) Get(key string) (any, error) {
	if value, exists := m.data[key]; exists {
		return value, nil
	}
	return nil, fmt.Errorf("key not found: %s", key)
}

func (m *MockConfigProvider) GetString(key string, defaultValue ...string) (string, error) {
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

func (m *MockConfigProvider) GetInt(key string, defaultValue ...int) (int, error) {
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

func (m *MockConfigProvider) GetBool(key string, defaultValue ...bool) (bool, error) {
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

func (m *MockConfigProvider) GetDuration(key string, defaultValue ...time.Duration) (time.Duration, error) {
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

func (m *MockConfigProvider) GetFloat(key string, defaultValue ...float64) (float64, error) {
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

func (m *MockConfigProvider) GetStringSlice(key string, defaultValue ...[]string) ([]string, error) {
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

func (m *MockConfigProvider) GetMap(key string) (map[string]any, error) {
	if value, exists := m.data[key]; exists {
		if mapVal, ok := value.(map[string]any); ok {
			return mapVal, nil
		}
	}
	return nil, fmt.Errorf("key not found: %s", key)
}

func (m *MockConfigProvider) IsSet(key string) bool {
	_, exists := m.data[key]
	return exists
}

func (m *MockConfigProvider) AllKeys() []string {
	keys := make([]string, 0, len(m.data))
	for key := range m.data {
		keys = append(keys, key)
	}
	return keys
}

func (m *MockConfigProvider) AllSettings() map[string]any {
	result := make(map[string]any)
	for key, value := range m.data {
		result[key] = value
	}
	return result
}

func (m *MockConfigProvider) Exists(key string) bool {
	_, exists := m.data[key]
	return exists
}

func (m *MockConfigProvider) Validate() error {
	return nil
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

func TestDefaultMetricsConfig(t *testing.T) {
	config := DefaultMetricsConfig()

	if !config.Enabled {
		t.Error("Expected metrics to be enabled by default")
	}

	if config.ListenAddress != ":9090" {
		t.Errorf("Expected listen address ':9090', got '%s'", config.ListenAddress)
	}

	if config.MetricsPath != "/metrics" {
		t.Errorf("Expected metrics path '/metrics', got '%s'", config.MetricsPath)
	}

	if config.HealthPath != "/health" {
		t.Errorf("Expected health path '/health', got '%s'", config.HealthPath)
	}

	if config.ReadyPath != "/ready" {
		t.Errorf("Expected ready path '/ready', got '%s'", config.ReadyPath)
	}

	if config.Namespace != "nereus" {
		t.Errorf("Expected namespace 'nereus', got '%s'", config.Namespace)
	}
}

func TestLoadMetricsConfig(t *testing.T) {
	cfg := NewMockConfigProvider()
	cfg.Set("metrics.enabled", false)
	cfg.Set("metrics.listen_address", ":8080")
	cfg.Set("metrics.metrics_path", "/custom-metrics")
	cfg.Set("metrics.health_path", "/custom-health")
	cfg.Set("metrics.ready_path", "/custom-ready")
	cfg.Set("metrics.update_interval", "60s")
	cfg.Set("metrics.namespace", "custom")

	config, err := loadMetricsConfig(cfg)
	if err != nil {
		t.Fatalf("Failed to load metrics config: %v", err)
	}

	if config.Enabled {
		t.Error("Expected metrics to be disabled")
	}

	if config.ListenAddress != ":8080" {
		t.Errorf("Expected listen address ':8080', got '%s'", config.ListenAddress)
	}

	if config.MetricsPath != "/custom-metrics" {
		t.Errorf("Expected metrics path '/custom-metrics', got '%s'", config.MetricsPath)
	}

	if config.UpdateInterval != 60*time.Second {
		t.Errorf("Expected update interval 60s, got %v", config.UpdateInterval)
	}

	if config.Namespace != "custom" {
		t.Errorf("Expected namespace 'custom', got '%s'", config.Namespace)
	}
}

func TestNewMetricsManager(t *testing.T) {
	cfg := NewMockConfigProvider()
	logger := createTestLogger()

	manager, err := NewMetricsManager(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create metrics manager: %v", err)
	}

	if manager == nil {
		t.Fatal("Metrics manager is nil")
	}

	if manager.config == nil {
		t.Error("Metrics config is nil")
	}

	if manager.registry == nil {
		t.Error("Prometheus registry is nil")
	}

	if manager.trapMetrics == nil {
		t.Error("Trap metrics is nil")
	}

	if manager.storageMetrics == nil {
		t.Error("Storage metrics is nil")
	}

	if manager.webhookMetrics == nil {
		t.Error("Webhook metrics is nil")
	}

	if manager.systemMetrics == nil {
		t.Error("System metrics is nil")
	}

	if manager.correlatorMetrics == nil {
		t.Error("Correlator metrics is nil")
	}
}

func TestNewMetricsManagerWithNilLogger(t *testing.T) {
	cfg := NewMockConfigProvider()

	_, err := NewMetricsManager(cfg, nil)
	if err == nil {
		t.Error("Expected error for nil logger")
	}
}

func TestMetricsManagerDisabled(t *testing.T) {
	cfg := NewMockConfigProvider()
	cfg.Set("metrics.enabled", false)
	logger := createTestLogger()

	manager, err := NewMetricsManager(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create metrics manager: %v", err)
	}

	// Start should succeed but not actually start server
	err = manager.Start()
	if err != nil {
		t.Fatalf("Failed to start disabled metrics manager: %v", err)
	}

	// Stop should succeed
	err = manager.Stop()
	if err != nil {
		t.Fatalf("Failed to stop disabled metrics manager: %v", err)
	}
}

func TestHealthAndReadyEndpoints(t *testing.T) {
	cfg := NewMockConfigProvider()
	cfg.Set("metrics.listen_address", ":0") // Use random port
	logger := createTestLogger()

	manager, err := NewMetricsManager(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create metrics manager: %v", err)
	}

	// Test health endpoint
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	manager.healthHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Body.String() != "OK" {
		t.Errorf("Expected body 'OK', got '%s'", w.Body.String())
	}

	// Test ready endpoint (should be not ready initially)
	req = httptest.NewRequest("GET", "/ready", nil)
	w = httptest.NewRecorder()
	manager.readyHandler(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}

	if w.Body.String() != "NOT READY" {
		t.Errorf("Expected body 'NOT READY', got '%s'", w.Body.String())
	}

	// Set ready and test again
	manager.SetReady(true)

	req = httptest.NewRequest("GET", "/ready", nil)
	w = httptest.NewRecorder()
	manager.readyHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Body.String() != "READY" {
		t.Errorf("Expected body 'READY', got '%s'", w.Body.String())
	}
}

func TestComponentHealth(t *testing.T) {
	cfg := NewMockConfigProvider()
	logger := createTestLogger()

	manager, err := NewMetricsManager(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create metrics manager: %v", err)
	}

	// Initially all healthy (no components registered)
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	manager.healthHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Set a component as unhealthy
	manager.SetComponentHealth("test-component", false)

	req = httptest.NewRequest("GET", "/health", nil)
	w = httptest.NewRecorder()
	manager.healthHandler(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}

	if w.Body.String() != "UNHEALTHY" {
		t.Errorf("Expected body 'UNHEALTHY', got '%s'", w.Body.String())
	}

	// Set component back to healthy
	manager.SetComponentHealth("test-component", true)

	req = httptest.NewRequest("GET", "/health", nil)
	w = httptest.NewRecorder()
	manager.healthHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestMetricsAccessors(t *testing.T) {
	cfg := NewMockConfigProvider()
	logger := createTestLogger()

	manager, err := NewMetricsManager(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create metrics manager: %v", err)
	}

	// Test all metric accessors
	if manager.GetTrapMetrics() == nil {
		t.Error("GetTrapMetrics returned nil")
	}

	if manager.GetStorageMetrics() == nil {
		t.Error("GetStorageMetrics returned nil")
	}

	if manager.GetWebhookMetrics() == nil {
		t.Error("GetWebhookMetrics returned nil")
	}

	if manager.GetSystemMetrics() == nil {
		t.Error("GetSystemMetrics returned nil")
	}

	if manager.GetCorrelatorMetrics() == nil {
		t.Error("GetCorrelatorMetrics returned nil")
	}
}

func TestMetricsUsage(t *testing.T) {
	cfg := NewMockConfigProvider()
	logger := createTestLogger()

	manager, err := NewMetricsManager(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create metrics manager: %v", err)
	}

	// Test trap metrics
	trapMetrics := manager.GetTrapMetrics()
	trapMetrics.TrapsReceived.Inc()
	trapMetrics.TrapsProcessed.Inc()
	trapMetrics.ProcessingTime.Observe(0.1)
	trapMetrics.TrapsPerSource.WithLabelValues("192.168.1.1").Inc()
	trapMetrics.TrapsByType.WithLabelValues("1.3.6.1.6.3.1.1.5.3", "linkDown").Inc()

	// Test storage metrics
	storageMetrics := manager.GetStorageMetrics()
	storageMetrics.EventsStored.Inc()
	storageMetrics.QueryDuration.Observe(0.05)
	storageMetrics.DatabaseSize.Set(1024)

	// Test webhook metrics
	webhookMetrics := manager.GetWebhookMetrics()
	webhookMetrics.WebhooksDelivered.Inc()
	webhookMetrics.DeliveryTime.Observe(0.2)
	webhookMetrics.QueueLength.Set(5)
	webhookMetrics.WebhooksByStatus.WithLabelValues("200", "test-webhook").Inc()

	// Test system metrics
	systemMetrics := manager.GetSystemMetrics()
	systemMetrics.MemoryUsage.Set(1024 * 1024)
	systemMetrics.GoroutineCount.Set(10)

	// Test correlator metrics
	correlatorMetrics := manager.GetCorrelatorMetrics()
	correlatorMetrics.EventsCorrelated.Inc()
	correlatorMetrics.CorrelationTime.Observe(0.01)
	correlatorMetrics.ActiveGroups.Set(3)

	// All metrics operations should complete without error
	t.Log("All metrics operations completed successfully")
}
