package client

import (
	"context"
	"net/http"
	"net/http/httptest"
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
			"client.timeout":              "30s",
			"client.max_retries":          3,
			"client.retry_delay":          "1s",
			"client.max_idle_conns":       10,
			"client.insecure_skip_verify": false,
			"client.user_agent":           "Test-Client/1.0",
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

func TestNewHTTPClient(t *testing.T) {
	cfg := newMockConfigProvider()

	client, err := NewHTTPClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create HTTP client: %v", err)
	}
	defer client.Close()

	if client == nil {
		t.Fatal("Client is nil")
	}

	if client.config == nil {
		t.Error("Config not set")
	}

	if client.httpClient == nil {
		t.Error("HTTP client not initialized")
	}

	if client.stats == nil {
		t.Error("Stats not initialized")
	}
}

func TestNewHTTPClientNilConfig(t *testing.T) {
	_, err := NewHTTPClient(nil)
	if err == nil {
		t.Fatal("Expected error for nil config, got nil")
	}

	expectedMsg := "configuration provider cannot be nil"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestDefaultClientConfig(t *testing.T) {
	config := DefaultClientConfig()

	if config == nil {
		t.Fatal("Config is nil")
	}

	if config.Timeout <= 0 {
		t.Error("Invalid timeout")
	}

	if config.MaxRetries < 0 {
		t.Error("Invalid max retries")
	}

	if config.UserAgent == "" {
		t.Error("User agent not set")
	}

	if config.DefaultHeaders == nil {
		t.Error("Default headers not initialized")
	}
}

func TestValidateRequest(t *testing.T) {
	cfg := newMockConfigProvider()
	client, err := NewHTTPClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create HTTP client: %v", err)
	}
	defer client.Close()

	testCases := []struct {
		name        string
		request     *WebhookRequest
		expectError bool
	}{
		{
			name:        "Nil request",
			request:     nil,
			expectError: true,
		},
		{
			name: "Empty URL",
			request: &WebhookRequest{
				URL: "",
			},
			expectError: true,
		},
		{
			name: "Invalid URL",
			request: &WebhookRequest{
				URL: "not-a-valid-url",
			},
			expectError: true,
		},
		{
			name: "Valid request",
			request: &WebhookRequest{
				URL:    "http://example.com/webhook",
				Method: "POST",
			},
			expectError: false,
		},
		{
			name: "Valid request with default method",
			request: &WebhookRequest{
				URL: "http://example.com/webhook",
			},
			expectError: false,
		},
		{
			name: "Invalid HTTP method",
			request: &WebhookRequest{
				URL:    "http://example.com/webhook",
				Method: "INVALID",
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := client.validateRequest(tc.request)
			if tc.expectError && err == nil {
				t.Errorf("Expected error for %s, got nil", tc.name)
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error for %s: %v", tc.name, err)
			}
		})
	}
}

func TestSendWebhook(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "success"}`))
	}))
	defer server.Close()

	cfg := newMockConfigProvider()
	client, err := NewHTTPClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create HTTP client: %v", err)
	}
	defer client.Close()

	request := &WebhookRequest{
		URL:    server.URL,
		Method: "POST",
		Body:   map[string]string{"test": "data"},
		Headers: map[string]string{
			"X-Test-Header": "test-value",
		},
	}

	ctx := context.Background()
	response, err := client.SendWebhook(ctx, request)
	if err != nil {
		t.Errorf("Failed to send webhook: %v", err)
	}

	if response == nil {
		t.Fatal("Response is nil")
	}

	if !response.Success {
		t.Errorf("Expected successful response, got: %s", response.Error)
	}

	if response.StatusCode != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", response.StatusCode)
	}

	if len(response.Body) == 0 {
		t.Error("Response body is empty")
	}
}

func TestSendWebhookTimeout(t *testing.T) {
	// Create test server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := newMockConfigProvider()
	cfg.values["client.timeout"] = "100ms" // Very short timeout

	client, err := NewHTTPClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create HTTP client: %v", err)
	}
	defer client.Close()

	request := &WebhookRequest{
		URL:    server.URL,
		Method: "GET",
	}

	ctx := context.Background()
	response, err := client.SendWebhook(ctx, request)

	// Should timeout
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	if response != nil && response.Success {
		t.Error("Expected failed response due to timeout")
	}
}

func TestSendWebhookRetry(t *testing.T) {
	attempts := 0

	// Create test server that fails first few attempts
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "success"}`))
	}))
	defer server.Close()

	cfg := newMockConfigProvider()
	cfg.values["client.max_retries"] = 3
	cfg.values["client.retry_delay"] = "10ms" // Short delay for testing

	client, err := NewHTTPClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create HTTP client: %v", err)
	}
	defer client.Close()

	request := &WebhookRequest{
		URL:    server.URL,
		Method: "POST",
		Body:   "test data",
	}

	ctx := context.Background()
	response, err := client.SendWebhook(ctx, request)
	if err != nil {
		t.Errorf("Failed to send webhook: %v", err)
	}

	if response == nil {
		t.Fatal("Response is nil")
	}

	if !response.Success {
		t.Errorf("Expected successful response after retries, got: %s", response.Error)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestGetStats(t *testing.T) {
	cfg := newMockConfigProvider()
	client, err := NewHTTPClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create HTTP client: %v", err)
	}
	defer client.Close()

	stats := client.GetStats()
	if stats == nil {
		t.Fatal("Stats is nil")
	}

	// Check that stats structure is properly initialized
	if stats.RequestsSent < 0 {
		t.Error("Invalid RequestsSent count")
	}

	if stats.StatusCodes == nil {
		t.Error("StatusCodes not initialized")
	}

	if stats.ErrorTypes == nil {
		t.Error("ErrorTypes not initialized")
	}
}

func TestCategorizeError(t *testing.T) {
	cfg := newMockConfigProvider()
	client, err := NewHTTPClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create HTTP client: %v", err)
	}
	defer client.Close()

	testCases := []struct {
		error    string
		expected string
	}{
		{"connection timeout", "timeout"},
		{"context deadline exceeded", "timeout"},
		{"connection refused", "connection_refused"},
		{"no such host", "dns_error"},
		{"certificate verify failed", "tls_error"},
		{"tls handshake timeout", "tls_error"},
		{"some other error", "other"},
	}

	for _, tc := range testCases {
		result := client.categorizeError(fmt.Errorf(tc.error))
		if result != tc.expected {
			t.Errorf("categorizeError(%s) = %s, expected %s", tc.error, result, tc.expected)
		}
	}
}

func TestResetStats(t *testing.T) {
	cfg := newMockConfigProvider()
	client, err := NewHTTPClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create HTTP client: %v", err)
	}
	defer client.Close()

	// Simulate some activity
	client.stats.RequestsSent = 10
	client.stats.RequestsSucceeded = 8
	client.stats.RequestsFailed = 2

	// Reset stats
	client.ResetStats()

	stats := client.GetStats()
	if stats.RequestsSent != 0 {
		t.Errorf("Expected RequestsSent to be 0 after reset, got %d", stats.RequestsSent)
	}

	if stats.RequestsSucceeded != 0 {
		t.Errorf("Expected RequestsSucceeded to be 0 after reset, got %d", stats.RequestsSucceeded)
	}

	if stats.RequestsFailed != 0 {
		t.Errorf("Expected RequestsFailed to be 0 after reset, got %d", stats.RequestsFailed)
	}
}
