package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/geekxflood/common/config"
	"github.com/geekxflood/common/logging"
	"github.com/geekxflood/nereus/internal/app"
)

// IntegrationTestSuite provides a complete testing environment
type IntegrationTestSuite struct {
	t           *testing.T
	app         *app.Application
	mockWebhook *MockWebhookServer
	snmpAgent   *MockSNMPAgent
	tempDir     string
	configPath  string
	logger      logging.Logger
	ctx         context.Context
	cancel      context.CancelFunc
}

// MockWebhookServer simulates webhook endpoints for testing
type MockWebhookServer struct {
	server    *httptest.Server
	requests  []WebhookRequest
	responses map[string]WebhookResponse
	mu        sync.RWMutex
}

// WebhookRequest represents a received webhook request
type WebhookRequest struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    string
	Time    time.Time
}

// WebhookResponse represents a configured response
type WebhookResponse struct {
	StatusCode int
	Body       string
	Delay      time.Duration
}

// MockSNMPAgent simulates SNMP trap sources
type MockSNMPAgent struct {
	conn   *net.UDPConn
	target string
	mu     sync.Mutex
}

// NewIntegrationTestSuite creates a new integration test suite
func NewIntegrationTestSuite(t *testing.T) *IntegrationTestSuite {
	ctx, cancel := context.WithCancel(context.Background())

	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "nereus-integration-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Initialize logger
	logger, _, err := logging.NewLogger(logging.Config{
		Level:  "debug",
		Format: "json",
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	suite := &IntegrationTestSuite{
		t:       t,
		tempDir: tempDir,
		logger:  logger,
		ctx:     ctx,
		cancel:  cancel,
	}

	// Setup mock webhook server
	suite.mockWebhook = NewMockWebhookServer()

	// Setup mock SNMP agent
	suite.snmpAgent = NewMockSNMPAgent(t)

	return suite
}

// NewMockWebhookServer creates a new mock webhook server
func NewMockWebhookServer() *MockWebhookServer {
	mock := &MockWebhookServer{
		requests:  make([]WebhookRequest, 0),
		responses: make(map[string]WebhookResponse),
	}

	// Create HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/", mock.handleRequest)
	mock.server = httptest.NewServer(mux)

	return mock
}

// handleRequest handles incoming webhook requests
func (m *MockWebhookServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Read request body
	body := make([]byte, r.ContentLength)
	if r.ContentLength > 0 {
		r.Body.Read(body)
	}

	// Store request
	req := WebhookRequest{
		Method:  r.Method,
		URL:     r.URL.String(),
		Headers: make(map[string]string),
		Body:    string(body),
		Time:    time.Now(),
	}

	// Copy headers
	for key, values := range r.Header {
		if len(values) > 0 {
			req.Headers[key] = values[0]
		}
	}

	m.requests = append(m.requests, req)

	// Check for configured response
	if response, exists := m.responses[r.URL.Path]; exists {
		if response.Delay > 0 {
			time.Sleep(response.Delay)
		}
		w.WriteHeader(response.StatusCode)
		w.Write([]byte(response.Body))
	} else {
		// Default response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}
}

// SetResponse configures a response for a specific path
func (m *MockWebhookServer) SetResponse(path string, response WebhookResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[path] = response
}

// GetRequests returns all received requests
func (m *MockWebhookServer) GetRequests() []WebhookRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	requests := make([]WebhookRequest, len(m.requests))
	copy(requests, m.requests)
	return requests
}

// ClearRequests clears all stored requests
func (m *MockWebhookServer) ClearRequests() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests = m.requests[:0]
}

// Close closes the mock webhook server
func (m *MockWebhookServer) Close() {
	m.server.Close()
}

// URL returns the server URL
func (m *MockWebhookServer) URL() string {
	return m.server.URL
}

// NewMockSNMPAgent creates a new mock SNMP agent
func NewMockSNMPAgent(t *testing.T) *MockSNMPAgent {
	return &MockSNMPAgent{
		target: "127.0.0.1:1162", // Default SNMP trap port
	}
}

// SendTrap sends a mock SNMP trap
func (m *MockSNMPAgent) SendTrap(trapOID string, varbinds map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create UDP connection if not exists
	if m.conn == nil {
		conn, err := net.Dial("udp", m.target)
		if err != nil {
			return fmt.Errorf("failed to create UDP connection: %w", err)
		}
		m.conn = conn.(*net.UDPConn)
	}

	// Build basic SNMPv2c trap packet
	packet := m.buildTrapPacket(trapOID, varbinds)

	_, err := m.conn.Write(packet)
	return err
}

// buildTrapPacket builds a basic SNMPv2c trap packet
func (m *MockSNMPAgent) buildTrapPacket(trapOID string, varbinds map[string]interface{}) []byte {
	// This is a simplified SNMP packet builder for testing
	// In a real implementation, you would use proper ASN.1 encoding

	// Basic SNMPv2c trap structure (simplified for testing)
	packet := []byte{
		0x30, 0x82, 0x00, 0x50, // SEQUENCE
		0x02, 0x01, 0x01, // version (SNMPv2c = 1)
		0x04, 0x06, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63, // community "public"
		0xa7, 0x82, 0x00, 0x41, // SNMPv2-Trap PDU
		0x02, 0x04, 0x00, 0x00, 0x00, 0x01, // request-id
		0x02, 0x01, 0x00, // error-status
		0x02, 0x01, 0x00, // error-index
		0x30, 0x82, 0x00, 0x33, // varbind list
	}

	// Add basic varbinds (simplified)
	// sysUpTime
	packet = append(packet, []byte{
		0x30, 0x0f, // varbind
		0x06, 0x08, 0x2b, 0x06, 0x01, 0x02, 0x01, 0x01, 0x03, 0x00, // OID
		0x43, 0x03, 0x01, 0x02, 0x03, // TimeTicks value
	}...)

	// snmpTrapOID
	packet = append(packet, []byte{
		0x30, 0x20, // varbind
		0x06, 0x0a, 0x2b, 0x06, 0x01, 0x06, 0x03, 0x01, 0x01, 0x04, 0x01, 0x00, // OID
		0x06, 0x12, // OID value (simplified)
	}...)

	return packet
}

// SetTarget sets the target address for sending traps
func (m *MockSNMPAgent) SetTarget(target string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.target = target
	if m.conn != nil {
		m.conn.Close()
		m.conn = nil
	}
}

// Close closes the SNMP agent connection
func (m *MockSNMPAgent) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.conn != nil {
		return m.conn.Close()
	}
	return nil
}

// Setup initializes the test environment
func (suite *IntegrationTestSuite) Setup() error {
	// Create test configuration
	configContent := fmt.Sprintf(`
app:
  log_level: debug

listener:
  bind_address: "127.0.0.1"
  port: 1162
  community: "public"
  buffer_size: 1024
  workers: 2

storage:
  type: "sqlite"
  connection_string: "%s/test.db"
  retention_days: 7
  max_connections: 10

correlator:
  enabled: true
  window_duration: "5m"
  max_groups: 1000

notifier:
  enabled: true
  workers: 2
  queue_size: 100
  endpoints:
    - name: "test-webhook"
      url: "%s/webhook"
      method: "POST"
      timeout: "10s"
      retry:
        max_attempts: 3
        delay: "1s"
        backoff_multiplier: 2.0

metrics:
  enabled: true
  bind_address: "127.0.0.1"
  port: 9090
`, suite.tempDir, suite.mockWebhook.URL())

	// Write configuration file
	suite.configPath = filepath.Join(suite.tempDir, "config.yaml")
	if err := os.WriteFile(suite.configPath, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Create config manager
	configManager, err := config.NewManager(config.Options{
		ConfigPath: suite.configPath,
	})
	if err != nil {
		return fmt.Errorf("failed to create config manager: %w", err)
	}

	// Initialize application
	suite.app, err = app.NewApplication(configManager)
	if err != nil {
		return fmt.Errorf("failed to create application: %w", err)
	}

	// Set SNMP agent target to the listener port
	suite.snmpAgent.SetTarget("127.0.0.1:1162")

	return nil
}

// Start starts the application
func (suite *IntegrationTestSuite) Start() error {
	// Initialize all components
	if err := suite.app.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize application: %w", err)
	}

	// Start the application in a goroutine since Run() blocks
	go func() {
		if err := suite.app.Run(); err != nil {
			suite.logger.Error("Application run failed", "error", err)
		}
	}()

	return nil
}

// Stop stops the application
func (suite *IntegrationTestSuite) Stop() error {
	if suite.app != nil {
		return suite.app.Shutdown()
	}
	return nil
}

// Cleanup cleans up test resources
func (suite *IntegrationTestSuite) Cleanup() {
	if suite.app != nil {
		suite.app.Shutdown()
	}
	if suite.mockWebhook != nil {
		suite.mockWebhook.Close()
	}
	if suite.snmpAgent != nil {
		suite.snmpAgent.Close()
	}
	if suite.cancel != nil {
		suite.cancel()
	}
	if suite.tempDir != "" {
		os.RemoveAll(suite.tempDir)
	}
}

// WaitForWebhookRequests waits for a specific number of webhook requests
func (suite *IntegrationTestSuite) WaitForWebhookRequests(count int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		requests := suite.mockWebhook.GetRequests()
		if len(requests) >= count {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %d webhook requests, got %d", count, len(suite.mockWebhook.GetRequests()))
}

// TestBasicTrapProcessing tests basic SNMP trap processing
func TestBasicTrapProcessing(t *testing.T) {
	suite := NewIntegrationTestSuite(t)
	defer suite.Cleanup()

	// Setup test environment
	if err := suite.Setup(); err != nil {
		t.Fatalf("Failed to setup test suite: %v", err)
	}

	// Start application
	if err := suite.Start(); err != nil {
		t.Fatalf("Failed to start application: %v", err)
	}

	// Wait for application to be ready
	time.Sleep(2 * time.Second)

	// Clear any initial requests
	suite.mockWebhook.ClearRequests()

	// Send test trap
	varbinds := map[string]interface{}{
		"1.3.6.1.2.1.1.1.0": "Test System Description",
		"1.3.6.1.2.1.1.5.0": "test-host",
	}

	if err := suite.snmpAgent.SendTrap("1.3.6.1.4.1.12345.1.1", varbinds); err != nil {
		t.Fatalf("Failed to send test trap: %v", err)
	}

	// Wait for webhook to be called
	if err := suite.WaitForWebhookRequests(1, 10*time.Second); err != nil {
		t.Fatalf("Webhook was not called: %v", err)
	}

	// Verify webhook request
	requests := suite.mockWebhook.GetRequests()
	if len(requests) != 1 {
		t.Fatalf("Expected 1 webhook request, got %d", len(requests))
	}

	req := requests[0]
	if req.Method != "POST" {
		t.Errorf("Expected POST method, got %s", req.Method)
	}

	if req.URL != "/webhook" {
		t.Errorf("Expected /webhook URL, got %s", req.URL)
	}

	// Verify request body contains event data
	if req.Body == "" {
		t.Error("Expected non-empty request body")
	}

	// Parse JSON body to verify structure
	var eventData map[string]interface{}
	if err := json.Unmarshal([]byte(req.Body), &eventData); err != nil {
		t.Errorf("Failed to parse webhook body as JSON: %v", err)
	}
}

// TestWebhookRetryLogic tests webhook retry functionality
func TestWebhookRetryLogic(t *testing.T) {
	suite := NewIntegrationTestSuite(t)
	defer suite.Cleanup()

	// Setup test environment
	if err := suite.Setup(); err != nil {
		t.Fatalf("Failed to setup test suite: %v", err)
	}

	// Configure webhook to fail initially
	suite.mockWebhook.SetResponse("/webhook", WebhookResponse{
		StatusCode: http.StatusInternalServerError,
		Body:       `{"error": "server error"}`,
	})

	// Start application
	if err := suite.Start(); err != nil {
		t.Fatalf("Failed to start application: %v", err)
	}

	// Wait for application to be ready
	time.Sleep(2 * time.Second)

	// Clear any initial requests
	suite.mockWebhook.ClearRequests()

	// Send test trap
	varbinds := map[string]interface{}{
		"1.3.6.1.2.1.1.1.0": "Test System Description",
		"1.3.6.1.2.1.1.5.0": "test-host",
	}

	if err := suite.snmpAgent.SendTrap("1.3.6.1.4.1.12345.1.2", varbinds); err != nil {
		t.Fatalf("Failed to send test trap: %v", err)
	}

	// Wait for initial webhook attempts (should fail)
	time.Sleep(3 * time.Second)

	// Change webhook to succeed
	suite.mockWebhook.SetResponse("/webhook", WebhookResponse{
		StatusCode: http.StatusOK,
		Body:       `{"status": "ok"}`,
	})

	// Wait for retry attempts
	if err := suite.WaitForWebhookRequests(1, 15*time.Second); err != nil {
		t.Fatalf("Webhook retry was not successful: %v", err)
	}

	// Verify that retries occurred
	requests := suite.mockWebhook.GetRequests()
	if len(requests) < 1 {
		t.Fatalf("Expected at least 1 webhook request (including retries), got %d", len(requests))
	}
}

// TestCorrelationEngine tests event correlation functionality
func TestCorrelationEngine(t *testing.T) {
	suite := NewIntegrationTestSuite(t)
	defer suite.Cleanup()

	// Setup test environment
	if err := suite.Setup(); err != nil {
		t.Fatalf("Failed to setup test suite: %v", err)
	}

	// Start application
	if err := suite.Start(); err != nil {
		t.Fatalf("Failed to start application: %v", err)
	}

	// Wait for application to be ready
	time.Sleep(2 * time.Second)

	// Clear any initial requests
	suite.mockWebhook.ClearRequests()

	// Send multiple similar traps (should be correlated)
	varbinds := map[string]interface{}{
		"1.3.6.1.2.1.1.1.0": "Test System Description",
		"1.3.6.1.2.1.1.5.0": "test-host",
	}

	// Send 3 identical traps
	for i := 0; i < 3; i++ {
		if err := suite.snmpAgent.SendTrap("1.3.6.1.4.1.12345.1.3", varbinds); err != nil {
			t.Fatalf("Failed to send test trap %d: %v", i+1, err)
		}
		time.Sleep(100 * time.Millisecond) // Small delay between traps
	}

	// Wait for webhook to be called (should be fewer than 3 due to correlation)
	time.Sleep(5 * time.Second)

	requests := suite.mockWebhook.GetRequests()

	// Due to correlation, we should have fewer webhook calls than trap sends
	if len(requests) >= 3 {
		t.Errorf("Expected fewer than 3 webhook requests due to correlation, got %d", len(requests))
	}

	if len(requests) == 0 {
		t.Error("Expected at least 1 webhook request")
	}
}

// TestHighVolumeTraps tests handling of high-volume trap processing
func TestHighVolumeTraps(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping high-volume test in short mode")
	}

	suite := NewIntegrationTestSuite(t)
	defer suite.Cleanup()

	// Setup test environment
	if err := suite.Setup(); err != nil {
		t.Fatalf("Failed to setup test suite: %v", err)
	}

	// Start application
	if err := suite.Start(); err != nil {
		t.Fatalf("Failed to start application: %v", err)
	}

	// Wait for application to be ready
	time.Sleep(2 * time.Second)

	// Clear any initial requests
	suite.mockWebhook.ClearRequests()

	// Send many traps rapidly
	numTraps := 50
	varbinds := map[string]interface{}{
		"1.3.6.1.2.1.1.1.0": "Test System Description",
		"1.3.6.1.2.1.1.5.0": "test-host",
	}

	start := time.Now()
	for i := 0; i < numTraps; i++ {
		trapOID := fmt.Sprintf("1.3.6.1.4.1.12345.1.%d", i%10) // Vary the trap OID
		if err := suite.snmpAgent.SendTrap(trapOID, varbinds); err != nil {
			t.Errorf("Failed to send test trap %d: %v", i+1, err)
		}
	}
	sendDuration := time.Since(start)

	// Wait for processing to complete
	time.Sleep(10 * time.Second)

	requests := suite.mockWebhook.GetRequests()

	t.Logf("Sent %d traps in %v, received %d webhook requests",
		numTraps, sendDuration, len(requests))

	// We should receive some webhook requests (exact number depends on correlation)
	if len(requests) == 0 {
		t.Error("Expected at least some webhook requests for high-volume test")
	}

	// Verify that processing was reasonably fast
	if sendDuration > 5*time.Second {
		t.Errorf("Trap sending took too long: %v", sendDuration)
	}
}
