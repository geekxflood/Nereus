package load

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geekxflood/common/config"
	"github.com/geekxflood/common/logging"
	"github.com/geekxflood/nereus/internal/app"
)

// LoadTestSuite provides a load testing environment
type LoadTestSuite struct {
	t              *testing.T
	app            *app.Application
	mockWebhook    *MockWebhookServer
	snmpAgents     []*MockSNMPAgent
	tempDir        string
	configPath     string
	logger         logging.Logger
	ctx            context.Context
	cancel         context.CancelFunc
	webhookCounter int64
	trapCounter    int64
}

// MockWebhookServer for load testing
type MockWebhookServer struct {
	server       *httptest.Server
	requestCount int64
	responseTime time.Duration
	failureRate  float64
	mu           sync.RWMutex
}

// MockSNMPAgent for load testing
type MockSNMPAgent struct {
	conn   *net.UDPConn
	target string
	id     int
	mu     sync.Mutex
}

// LoadTestMetrics holds performance metrics
type LoadTestMetrics struct {
	TrapsSent         int64
	TrapsPerSecond    float64
	WebhooksReceived  int64
	WebhooksPerSecond float64
	AverageLatency    time.Duration
	MaxLatency        time.Duration
	MinLatency        time.Duration
	ErrorRate         float64
	MemoryUsageMB     float64
	CPUUsagePercent   float64
	TestDuration      time.Duration
}

// NewLoadTestSuite creates a new load test suite
func NewLoadTestSuite(t *testing.T) *LoadTestSuite {
	ctx, cancel := context.WithCancel(context.Background())

	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "nereus-load-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Initialize logger
	logger, _, err := logging.NewLogger(logging.Config{
		Level:  "warn", // Reduce logging overhead during load tests
		Format: "json",
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	suite := &LoadTestSuite{
		t:       t,
		tempDir: tempDir,
		logger:  logger,
		ctx:     ctx,
		cancel:  cancel,
	}

	// Setup mock webhook server
	suite.mockWebhook = NewMockWebhookServer()

	// Setup multiple SNMP agents for concurrent load generation
	numAgents := runtime.NumCPU() * 2 // Scale with CPU cores
	suite.snmpAgents = make([]*MockSNMPAgent, numAgents)
	for i := 0; i < numAgents; i++ {
		suite.snmpAgents[i] = NewMockSNMPAgent(i)
	}

	return suite
}

// NewMockWebhookServer creates a new mock webhook server for load testing
func NewMockWebhookServer() *MockWebhookServer {
	mock := &MockWebhookServer{
		responseTime: 10 * time.Millisecond, // Fast response by default
		failureRate:  0.0,                   // No failures by default
	}

	// Create HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", mock.handleRequest)
	mock.server = httptest.NewServer(mux)

	return mock
}

// handleRequest handles incoming webhook requests
func (m *MockWebhookServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Simulate processing time
	if m.responseTime > 0 {
		time.Sleep(m.responseTime)
	}

	// Simulate failure rate
	m.mu.RLock()
	failureRate := m.failureRate
	m.mu.RUnlock()

	if failureRate > 0 && float64(atomic.LoadInt64(&m.requestCount))/(float64(atomic.LoadInt64(&m.requestCount))+1) < failureRate {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "simulated failure"}`))
	} else {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}

	atomic.AddInt64(&m.requestCount, 1)

	// Log latency for metrics
	latency := time.Since(start)
	_ = latency // Use for metrics collection if needed
}

// SetResponseTime sets the simulated response time
func (m *MockWebhookServer) SetResponseTime(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responseTime = duration
}

// SetFailureRate sets the failure rate (0.0 to 1.0)
func (m *MockWebhookServer) SetFailureRate(rate float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failureRate = rate
}

// GetRequestCount returns the total number of requests received
func (m *MockWebhookServer) GetRequestCount() int64 {
	return atomic.LoadInt64(&m.requestCount)
}

// ResetCounter resets the request counter
func (m *MockWebhookServer) ResetCounter() {
	atomic.StoreInt64(&m.requestCount, 0)
}

// Close closes the mock webhook server
func (m *MockWebhookServer) Close() {
	m.server.Close()
}

// URL returns the server URL
func (m *MockWebhookServer) URL() string {
	return m.server.URL
}

// NewMockSNMPAgent creates a new mock SNMP agent for load testing
func NewMockSNMPAgent(id int) *MockSNMPAgent {
	return &MockSNMPAgent{
		target: "127.0.0.1:1162",
		id:     id,
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

	// Build basic SNMPv2c trap packet (simplified for load testing)
	packet := m.buildTrapPacket(trapOID, varbinds)

	_, err := m.conn.Write(packet)
	return err
}

// buildTrapPacket builds a basic SNMPv2c trap packet for load testing
func (m *MockSNMPAgent) buildTrapPacket(trapOID string, varbinds map[string]interface{}) []byte {
	// Simplified SNMP packet for load testing
	packet := []byte{
		0x30, 0x82, 0x00, 0x50, // SEQUENCE
		0x02, 0x01, 0x01, // version (SNMPv2c = 1)
		0x04, 0x06, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63, // community "public"
		0xa7, 0x82, 0x00, 0x41, // SNMPv2-Trap PDU
		0x02, 0x04, 0x00, 0x00, 0x00, byte(m.id), // request-id (unique per agent)
		0x02, 0x01, 0x00, // error-status
		0x02, 0x01, 0x00, // error-index
		0x30, 0x82, 0x00, 0x33, // varbind list
	}

	// Add basic varbinds
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

// Setup initializes the load test environment
func (suite *LoadTestSuite) Setup() error {
	// Create test configuration optimized for load testing
	configContent := fmt.Sprintf(`
app:
  log_level: warn

listener:
  bind_address: "127.0.0.1"
  port: 1162
  community: "public"
  buffer_size: 10000
  workers: %d

storage:
  type: "sqlite"
  connection_string: "%s/load_test.db"
  retention_days: 1
  max_connections: 20
  batch_size: 100
  flush_interval: "1s"

correlator:
  enabled: true
  window_duration: "30s"
  max_groups: 10000

notifier:
  enabled: true
  workers: %d
  queue_size: 10000
  endpoints:
    - name: "load-test-webhook"
      url: "%s/webhook"
      method: "POST"
      timeout: "5s"
      retry:
        max_attempts: 2
        delay: "100ms"
        backoff_multiplier: 1.5

metrics:
  enabled: false
`, runtime.NumCPU()*4, suite.tempDir, runtime.NumCPU()*2, suite.mockWebhook.URL())

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

	// Set SNMP agent targets
	for _, agent := range suite.snmpAgents {
		agent.SetTarget("127.0.0.1:1162")
	}

	return nil
}

// Start starts the application for load testing
func (suite *LoadTestSuite) Start() error {
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
func (suite *LoadTestSuite) Stop() error {
	if suite.app != nil {
		return suite.app.Shutdown()
	}
	return nil
}

// Cleanup cleans up test resources
func (suite *LoadTestSuite) Cleanup() {
	if suite.app != nil {
		suite.app.Shutdown()
	}
	if suite.mockWebhook != nil {
		suite.mockWebhook.Close()
	}
	for _, agent := range suite.snmpAgents {
		if agent != nil {
			agent.Close()
		}
	}
	if suite.cancel != nil {
		suite.cancel()
	}
	if suite.tempDir != "" {
		os.RemoveAll(suite.tempDir)
	}
}

// RunLoadTest executes a load test with the given parameters
func (suite *LoadTestSuite) RunLoadTest(trapsPerSecond int, duration time.Duration) (*LoadTestMetrics, error) {
	// Reset counters
	suite.mockWebhook.ResetCounter()
	atomic.StoreInt64(&suite.trapCounter, 0)
	atomic.StoreInt64(&suite.webhookCounter, 0)

	// Calculate trap interval
	trapInterval := time.Second / time.Duration(trapsPerSecond)

	// Start memory monitoring
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	initialMemory := memStats.Alloc

	startTime := time.Now()
	endTime := startTime.Add(duration)

	// Create context for load test
	ctx, cancel := context.WithDeadline(suite.ctx, endTime)
	defer cancel()

	// Start trap generation
	var wg sync.WaitGroup
	trapsSent := int64(0)

	// Distribute load across agents
	trapsPerAgent := trapsPerSecond / len(suite.snmpAgents)
	if trapsPerAgent == 0 {
		trapsPerAgent = 1
	}

	for i, agent := range suite.snmpAgents {
		wg.Add(1)
		go func(agentID int, agent *MockSNMPAgent) {
			defer wg.Done()

			ticker := time.NewTicker(trapInterval * time.Duration(len(suite.snmpAgents)))
			defer ticker.Stop()

			trapCount := 0
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					varbinds := map[string]interface{}{
						"1.3.6.1.2.1.1.1.0": fmt.Sprintf("Load Test System %d", agentID),
						"1.3.6.1.2.1.1.5.0": fmt.Sprintf("load-test-host-%d", agentID),
					}

					trapOID := fmt.Sprintf("1.3.6.1.4.1.12345.%d.%d", agentID, trapCount%100)
					if err := agent.SendTrap(trapOID, varbinds); err != nil {
						suite.logger.Error("Failed to send trap", "error", err, "agent", agentID)
					} else {
						atomic.AddInt64(&trapsSent, 1)
					}
					trapCount++
				}
			}
		}(i, agent)
	}

	// Wait for test duration
	<-ctx.Done()
	wg.Wait()

	actualDuration := time.Since(startTime)

	// Collect final metrics
	runtime.ReadMemStats(&memStats)
	finalMemory := memStats.Alloc
	memoryUsedMB := float64(finalMemory-initialMemory) / (1024 * 1024)

	webhooksReceived := suite.mockWebhook.GetRequestCount()

	metrics := &LoadTestMetrics{
		TrapsSent:         trapsSent,
		TrapsPerSecond:    float64(trapsSent) / actualDuration.Seconds(),
		WebhooksReceived:  webhooksReceived,
		WebhooksPerSecond: float64(webhooksReceived) / actualDuration.Seconds(),
		MemoryUsageMB:     memoryUsedMB,
		TestDuration:      actualDuration,
	}

	return metrics, nil
}

// BenchmarkTrapProcessing benchmarks SNMP trap processing performance
func BenchmarkTrapProcessing(b *testing.B) {
	suite := NewLoadTestSuite(&testing.T{})
	defer suite.Cleanup()

	// Setup test environment
	if err := suite.Setup(); err != nil {
		b.Fatalf("Failed to setup test suite: %v", err)
	}

	// Start application
	if err := suite.Start(); err != nil {
		b.Fatalf("Failed to start application: %v", err)
	}

	// Wait for application to be ready
	time.Sleep(2 * time.Second)

	varbinds := map[string]interface{}{
		"1.3.6.1.2.1.1.1.0": "Benchmark System",
		"1.3.6.1.2.1.1.5.0": "benchmark-host",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		agent := suite.snmpAgents[0]
		trapCount := 0
		for pb.Next() {
			trapOID := fmt.Sprintf("1.3.6.1.4.1.12345.1.%d", trapCount%1000)
			if err := agent.SendTrap(trapOID, varbinds); err != nil {
				b.Errorf("Failed to send trap: %v", err)
			}
			trapCount++
		}
	})
}

// TestLowVolumeLoad tests performance under low volume
func TestLowVolumeLoad(t *testing.T) {
	suite := NewLoadTestSuite(t)
	defer suite.Cleanup()

	if err := suite.Setup(); err != nil {
		t.Fatalf("Failed to setup test suite: %v", err)
	}

	if err := suite.Start(); err != nil {
		t.Fatalf("Failed to start application: %v", err)
	}

	// Wait for application to be ready
	time.Sleep(2 * time.Second)

	// Run load test: 10 traps/second for 30 seconds
	metrics, err := suite.RunLoadTest(10, 30*time.Second)
	if err != nil {
		t.Fatalf("Load test failed: %v", err)
	}

	t.Logf("Low Volume Load Test Results:")
	t.Logf("  Traps Sent: %d", metrics.TrapsSent)
	t.Logf("  Traps/sec: %.2f", metrics.TrapsPerSecond)
	t.Logf("  Webhooks Received: %d", metrics.WebhooksReceived)
	t.Logf("  Webhooks/sec: %.2f", metrics.WebhooksPerSecond)
	t.Logf("  Memory Used: %.2f MB", metrics.MemoryUsageMB)
	t.Logf("  Test Duration: %v", metrics.TestDuration)

	// Verify basic functionality
	if metrics.TrapsSent == 0 {
		t.Error("No traps were sent")
	}

	if metrics.WebhooksReceived == 0 {
		t.Error("No webhooks were received")
	}

	// Memory usage should be reasonable
	if metrics.MemoryUsageMB > 100 {
		t.Errorf("Memory usage too high: %.2f MB", metrics.MemoryUsageMB)
	}
}

// TestMediumVolumeLoad tests performance under medium volume
func TestMediumVolumeLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping medium volume test in short mode")
	}

	suite := NewLoadTestSuite(t)
	defer suite.Cleanup()

	if err := suite.Setup(); err != nil {
		t.Fatalf("Failed to setup test suite: %v", err)
	}

	if err := suite.Start(); err != nil {
		t.Fatalf("Failed to start application: %v", err)
	}

	// Wait for application to be ready
	time.Sleep(2 * time.Second)

	// Run load test: 100 traps/second for 60 seconds
	metrics, err := suite.RunLoadTest(100, 60*time.Second)
	if err != nil {
		t.Fatalf("Load test failed: %v", err)
	}

	t.Logf("Medium Volume Load Test Results:")
	t.Logf("  Traps Sent: %d", metrics.TrapsSent)
	t.Logf("  Traps/sec: %.2f", metrics.TrapsPerSecond)
	t.Logf("  Webhooks Received: %d", metrics.WebhooksReceived)
	t.Logf("  Webhooks/sec: %.2f", metrics.WebhooksPerSecond)
	t.Logf("  Memory Used: %.2f MB", metrics.MemoryUsageMB)
	t.Logf("  Test Duration: %v", metrics.TestDuration)

	// Performance expectations for medium load
	expectedMinTraps := int64(90 * 60) // Allow 10% tolerance
	if metrics.TrapsSent < expectedMinTraps {
		t.Errorf("Expected at least %d traps, got %d", expectedMinTraps, metrics.TrapsSent)
	}

	// Memory usage should still be reasonable
	if metrics.MemoryUsageMB > 500 {
		t.Errorf("Memory usage too high for medium load: %.2f MB", metrics.MemoryUsageMB)
	}
}

// TestHighVolumeLoad tests performance under high volume
func TestHighVolumeLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping high volume test in short mode")
	}

	suite := NewLoadTestSuite(t)
	defer suite.Cleanup()

	if err := suite.Setup(); err != nil {
		t.Fatalf("Failed to setup test suite: %v", err)
	}

	if err := suite.Start(); err != nil {
		t.Fatalf("Failed to start application: %v", err)
	}

	// Wait for application to be ready
	time.Sleep(3 * time.Second)

	// Run load test: 1000 traps/second for 30 seconds
	metrics, err := suite.RunLoadTest(1000, 30*time.Second)
	if err != nil {
		t.Fatalf("Load test failed: %v", err)
	}

	t.Logf("High Volume Load Test Results:")
	t.Logf("  Traps Sent: %d", metrics.TrapsSent)
	t.Logf("  Traps/sec: %.2f", metrics.TrapsPerSecond)
	t.Logf("  Webhooks Received: %d", metrics.WebhooksReceived)
	t.Logf("  Webhooks/sec: %.2f", metrics.WebhooksPerSecond)
	t.Logf("  Memory Used: %.2f MB", metrics.MemoryUsageMB)
	t.Logf("  Test Duration: %v", metrics.TestDuration)

	// Performance expectations for high load
	expectedMinTraps := int64(800 * 30) // Allow 20% tolerance for high load
	if metrics.TrapsSent < expectedMinTraps {
		t.Errorf("Expected at least %d traps, got %d", expectedMinTraps, metrics.TrapsSent)
	}

	// System should handle high load without excessive memory usage
	if metrics.MemoryUsageMB > 1000 {
		t.Errorf("Memory usage too high for high load: %.2f MB", metrics.MemoryUsageMB)
	}
}
