package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/geekxflood/common/logging"
	"github.com/geekxflood/nereus/internal/infra"
	"github.com/geekxflood/nereus/internal/storage"
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

func (m *MockConfigProvider) Validate() error {
	return nil
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
	maps.Copy(result, m.data)
	return result
}

func (m *MockConfigProvider) Exists(key string) bool {
	_, exists := m.data[key]
	return exists
}

// createTestEvent creates a test storage event
func createTestEvent() *storage.Event {
	now := time.Now()
	return &storage.Event{
		ID:            123,
		Timestamp:     now,
		SourceIP:      "192.168.1.100",
		Community:     "public",
		Version:       2,
		PDUType:       4,
		RequestID:     456,
		TrapOID:       "1.3.6.1.6.3.1.1.5.3",
		TrapName:      func() *string { s := "linkDown"; return &s }(),
		Severity:      "major",
		Status:        "firing",
		Acknowledged:  false,
		Count:         1,
		FirstSeen:     now,
		LastSeen:      now,
		Varbinds:      `[{"oid":"1.3.6.1.2.1.2.2.1.1.1","value":"1"}]`,
		Metadata:      `{"interface":"eth0"}`,
		Hash:          "abc123",
		CorrelationID: "corr-456",
	}
}

// createTestConfig creates a test configuration
func createTestConfig() *MockConfigProvider {
	cfg := NewMockConfigProvider()
	cfg.Set("notifier.enable_notifications", true)
	cfg.Set("notifier.max_concurrent", 2)
	cfg.Set("notifier.queue_size", 100)
	cfg.Set("notifier.delivery_timeout", "10s")
	cfg.Set("notifier.retry_attempts", 2)
	cfg.Set("notifier.retry_delay", "1s")

	// Set webhook configuration
	webhooks := []map[string]any{
		{
			"name":         "test-webhook",
			"url":          "http://localhost:8080/webhook",
			"method":       "POST",
			"format":       "alertmanager",
			"enabled":      true,
			"timeout":      "5s",
			"retry_count":  2,
			"content_type": "application/json",
			"headers": map[string]string{
				"Authorization": "Bearer test-token",
			},
		},
	}
	cfg.Set("notifier.default_webhooks", webhooks)

	return cfg
}

func createTestInfraManager(cfg *MockConfigProvider) (*infra.Manager, error) {
	logger, _, err := logging.NewLogger(logging.Config{
		Level:  "debug",
		Format: "json",
	})
	if err != nil {
		return nil, err
	}
	return infra.NewManager(cfg, logger)
}

func TestNewNotifier(t *testing.T) {
	cfg := createTestConfig()
	infraManager, err := createTestInfraManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create infra manager: %v", err)
	}

	notifier, err := NewNotifier(cfg, infraManager)
	if err != nil {
		t.Fatalf("Failed to create notifier: %v", err)
	}

	if notifier == nil {
		t.Fatal("Notifier is nil")
	}

	if notifier.config == nil {
		t.Error("Notifier config is nil")
	}

	if notifier.client == nil {
		t.Error("Notifier client is nil")
	}

	if notifier.alertConverter == nil {
		t.Error("Notifier alert converter is nil")
	}

	if notifier.defaultTemplate == nil {
		t.Error("Notifier default template is nil")
	}

	if notifier.taskQueue == nil {
		t.Error("Notifier task queue is nil")
	}

	if notifier.stats == nil {
		t.Error("Notifier stats is nil")
	}
}

func TestNewNotifierWithNilConfig(t *testing.T) {
	cfg := createTestConfig()
	infraManager, err := createTestInfraManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create infra manager: %v", err)
	}

	_, err = NewNotifier(nil, infraManager)
	if err == nil {
		t.Error("Expected error for nil config")
	}
}

func TestNewNotifierWithNilClient(t *testing.T) {
	cfg := createTestConfig()

	_, err := NewNotifier(cfg, nil)
	if err == nil {
		t.Error("Expected error for nil client")
	}
}

func TestLoadDefaultTemplate(t *testing.T) {
	template, err := loadDefaultTemplate()
	if err != nil {
		t.Fatalf("Failed to load default template: %v", err)
	}

	if template == nil {
		t.Fatal("Template is nil")
	}

	// Test template execution
	event := createTestEvent()
	data := map[string]any{
		"ID":        event.ID,
		"Timestamp": event.Timestamp.Format(time.RFC3339),
		"SourceIP":  event.SourceIP,
		"TrapOID":   event.TrapOID,
		"TrapName": func() string {
			if event.TrapName != nil {
				return *event.TrapName
			}
			return ""
		}(),
		"Severity":      event.Severity,
		"Status":        event.Status,
		"VarbindsJSON":  event.Varbinds,
		"Community":     event.Community,
		"Version":       event.Version,
		"PDUType":       event.PDUType,
		"RequestID":     event.RequestID,
		"Count":         event.Count,
		"FirstSeen":     event.FirstSeen.Format(time.RFC3339),
		"LastSeen":      event.LastSeen.Format(time.RFC3339),
		"Acknowledged":  event.Acknowledged,
		"CorrelationID": event.CorrelationID,
	}

	var buf bytes.Buffer
	err = template.Execute(&buf, data)
	if err != nil {
		t.Fatalf("Failed to execute template: %v", err)
	}

	// Verify the output is valid JSON
	var result map[string]any
	err = json.Unmarshal(buf.Bytes(), &result)
	if err != nil {
		t.Fatalf("Template output is not valid JSON: %v", err)
	}

	// Check required fields
	if result["event_id"] == nil {
		t.Error("event_id field missing from template output")
	}

	if result["source_ip"] != "192.168.1.100" {
		t.Errorf("Expected source_ip '192.168.1.100', got '%v'", result["source_ip"])
	}
}

func TestNotifierStartStop(t *testing.T) {
	cfg := createTestConfig()
	infraManager, err := createTestInfraManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create infra manager: %v", err)
	}

	notifier, err := NewNotifier(cfg, infraManager)
	if err != nil {
		t.Fatalf("Failed to create notifier: %v", err)
	}

	// Test start
	err = notifier.Start()
	if err != nil {
		t.Fatalf("Failed to start notifier: %v", err)
	}

	// Give workers time to start
	time.Sleep(100 * time.Millisecond)

	// Test stop
	err = notifier.Stop()
	if err != nil {
		t.Fatalf("Failed to stop notifier: %v", err)
	}
}

func TestGeneratePayloadAlertmanager(t *testing.T) {
	cfg := createTestConfig()
	infraManager, err := createTestInfraManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create infra manager: %v", err)
	}

	notifier, err := NewNotifier(cfg, infraManager)
	if err != nil {
		t.Fatalf("Failed to create notifier: %v", err)
	}

	event := createTestEvent()
	webhook := &WebhookConfig{
		Format: "alertmanager",
	}

	payload, err := notifier.generatePayload(event, webhook)
	if err != nil {
		t.Fatalf("Failed to generate Alertmanager payload: %v", err)
	}

	// Verify it's valid JSON
	var alertPayload AlertManagerPayload
	err = json.Unmarshal(payload, &alertPayload)
	if err != nil {
		t.Fatalf("Payload is not valid Alertmanager JSON: %v", err)
	}

	// Check that alerts are present
	if len(alertPayload.Alerts) == 0 {
		t.Error("No alerts in Alertmanager payload")
	}

	// Check alert content
	alert := alertPayload.Alerts[0]
	if alert.Labels["source_ip"] != "192.168.1.100" {
		t.Errorf("Expected source_ip '192.168.1.100', got '%s'", alert.Labels["source_ip"])
	}

	if alert.Labels["severity"] != "major" {
		t.Errorf("Expected severity 'major', got '%s'", alert.Labels["severity"])
	}
}

func TestGeneratePayloadCustom(t *testing.T) {
	cfg := createTestConfig()
	infraManager, err := createTestInfraManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create infra manager: %v", err)
	}

	notifier, err := NewNotifier(cfg, infraManager)
	if err != nil {
		t.Fatalf("Failed to create notifier: %v", err)
	}

	event := createTestEvent()
	webhook := &WebhookConfig{
		Format: "custom",
	}

	payload, err := notifier.generatePayload(event, webhook)
	if err != nil {
		t.Fatalf("Failed to generate custom payload: %v", err)
	}

	// Verify it's valid JSON
	var result map[string]any
	err = json.Unmarshal(payload, &result)
	if err != nil {
		t.Fatalf("Payload is not valid JSON: %v", err)
	}

	// Check required fields from default template
	if result["event_id"] == nil {
		t.Error("event_id field missing from custom payload")
	}

	if result["source_ip"] != "192.168.1.100" {
		t.Errorf("Expected source_ip '192.168.1.100', got '%v'", result["source_ip"])
	}
}

func TestSendNotificationWithMockServer(t *testing.T) {
	// Create mock HTTP server
	receivedPayloads := make([][]byte, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read request body
		body := make([]byte, r.ContentLength)
		r.Body.Read(body)
		receivedPayloads = append(receivedPayloads, body)

		// Return success
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	// Create test configuration with mock server URL
	cfg := createTestConfig()
	webhooks := []map[string]any{
		{
			"name":         "test-webhook",
			"url":          server.URL,
			"method":       "POST",
			"format":       "alertmanager",
			"enabled":      true,
			"timeout":      "5s",
			"retry_count":  2,
			"content_type": "application/json",
		},
	}
	cfg.Set("notifier.default_webhooks", webhooks)

	infraManager, err := createTestInfraManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create infra manager: %v", err)
	}

	notifier, err := NewNotifier(cfg, infraManager)
	if err != nil {
		t.Fatalf("Failed to create notifier: %v", err)
	}

	// Start notifier
	err = notifier.Start()
	if err != nil {
		t.Fatalf("Failed to start notifier: %v", err)
	}
	defer notifier.Stop()

	// Send notification
	event := createTestEvent()
	err = notifier.SendNotification(event)
	if err != nil {
		t.Fatalf("Failed to send notification: %v", err)
	}

	// Wait for notification to be processed
	time.Sleep(500 * time.Millisecond)

	// Check that payload was received
	if len(receivedPayloads) == 0 {
		t.Error("No payloads received by mock server")
	}

	// Verify payload content
	var alertPayload AlertManagerPayload
	err = json.Unmarshal(receivedPayloads[0], &alertPayload)
	if err != nil {
		t.Fatalf("Received payload is not valid Alertmanager JSON: %v", err)
	}

	if len(alertPayload.Alerts) == 0 {
		t.Error("No alerts in received payload")
	}
}

func TestFilterEvaluation(t *testing.T) {
	cfg := createTestConfig()
	infraManager, err := createTestInfraManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create infra manager: %v", err)
	}

	notifier, err := NewNotifier(cfg, infraManager)
	if err != nil {
		t.Fatalf("Failed to create notifier: %v", err)
	}

	event := createTestEvent()

	// Test equals condition
	condition := FilterCondition{
		Field:    "severity",
		Operator: "equals",
		Value:    "major",
	}

	result := notifier.evaluateCondition(event, condition)
	if !result {
		t.Error("Expected condition to pass for severity equals major")
	}

	// Test not_equals condition
	condition = FilterCondition{
		Field:    "severity",
		Operator: "not_equals",
		Value:    "minor",
	}

	result = notifier.evaluateCondition(event, condition)
	if !result {
		t.Error("Expected condition to pass for severity not_equals minor")
	}

	// Test in condition
	condition = FilterCondition{
		Field:    "severity",
		Operator: "in",
		Values:   []string{"major", "critical"},
	}

	result = notifier.evaluateCondition(event, condition)
	if !result {
		t.Error("Expected condition to pass for severity in [major, critical]")
	}

	// Test contains condition
	condition = FilterCondition{
		Field:    "source_ip",
		Operator: "contains",
		Value:    "192.168",
	}

	result = notifier.evaluateCondition(event, condition)
	if !result {
		t.Error("Expected condition to pass for source_ip contains 192.168")
	}
}
