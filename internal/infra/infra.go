// Package infra provides consolidated infrastructure functionality including HTTP client, retry mechanisms, and hot reload capabilities.
package infra

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/geekxflood/common/config"
	"github.com/geekxflood/common/logging"
)

// InfraConfig holds consolidated configuration for infrastructure components
type InfraConfig struct {
	// HTTP Client configuration
	HTTPTimeout         time.Duration     `json:"http_timeout"`
	MaxRetries          int               `json:"max_retries"`
	RetryDelay          time.Duration     `json:"retry_delay"`
	MaxIdleConns        int               `json:"max_idle_conns"`
	MaxIdleConnsPerHost int               `json:"max_idle_conns_per_host"`
	IdleConnTimeout     time.Duration     `json:"idle_conn_timeout"`
	TLSHandshakeTimeout time.Duration     `json:"tls_handshake_timeout"`
	InsecureSkipVerify  bool              `json:"insecure_skip_verify"`
	UserAgent           string            `json:"user_agent"`
	DefaultHeaders      map[string]string `json:"default_headers"`
	EnableCompression   bool              `json:"enable_compression"`
	MaxResponseSize     int64             `json:"max_response_size"`

	// Retry configuration
	MaxAttempts             int           `json:"max_attempts"`
	InitialDelay            time.Duration `json:"initial_delay"`
	MaxDelay                time.Duration `json:"max_delay"`
	BackoffMultiplier       float64       `json:"backoff_multiplier"`
	Jitter                  bool          `json:"jitter"`
	JitterRange             float64       `json:"jitter_range"`
	EnableCircuitBreaker    bool          `json:"enable_circuit_breaker"`
	CircuitFailureThreshold int           `json:"circuit_failure_threshold"`
	CircuitSuccessThreshold int           `json:"circuit_success_threshold"`
	CircuitTimeout          time.Duration `json:"circuit_timeout"`
	CircuitHalfOpenMaxCalls int           `json:"circuit_half_open_max_calls"`

	// Reload configuration
	EnableHotReload    bool          `json:"enable_hot_reload"`
	WatchConfigFile    bool          `json:"watch_config_file"`
	ConfigFile         string        `json:"config_file"`
	DebounceDelay      time.Duration `json:"debounce_delay"`
	MaxReloadFrequency time.Duration `json:"max_reload_frequency"`
}

// DefaultInfraConfig returns a default infrastructure configuration
func DefaultInfraConfig() *InfraConfig {
	return &InfraConfig{
		// HTTP Client defaults
		HTTPTimeout:         30 * time.Second,
		MaxRetries:          3,
		RetryDelay:          1 * time.Second,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		InsecureSkipVerify:  false,
		UserAgent:           "Nereus-SNMP-Trap-Listener/1.0",
		DefaultHeaders: map[string]string{
			"Content-Type": "application/json",
			"Accept":       "application/json",
		},
		EnableCompression: true,
		MaxResponseSize:   10 * 1024 * 1024, // 10MB

		// Retry defaults
		MaxAttempts:             3,
		InitialDelay:            1 * time.Second,
		MaxDelay:                30 * time.Second,
		BackoffMultiplier:       2.0,
		Jitter:                  true,
		JitterRange:             0.1,
		EnableCircuitBreaker:    true,
		CircuitFailureThreshold: 5,
		CircuitSuccessThreshold: 3,
		CircuitTimeout:          60 * time.Second,
		CircuitHalfOpenMaxCalls: 3,

		// Reload defaults
		EnableHotReload:    true,
		WatchConfigFile:    true,
		DebounceDelay:      500 * time.Millisecond,
		MaxReloadFrequency: 5 * time.Second,
	}
}

// WebhookRequest represents a webhook HTTP request
type WebhookRequest struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    []byte            `json:"body"`
	Timeout time.Duration     `json:"timeout"`
}

// WebhookResponse represents a webhook HTTP response
type WebhookResponse struct {
	StatusCode    int               `json:"status_code"`
	Headers       map[string]string `json:"headers"`
	Body          []byte            `json:"body"`
	ContentLength int64             `json:"content_length"`
	Success       bool              `json:"success"`
	Duration      time.Duration     `json:"duration"`
	Error         string            `json:"error,omitempty"`
}

// RetryableFunc represents a function that can be retried
type RetryableFunc func(ctx context.Context, attempt int) error

// RetryResult represents the result of a retry operation
type RetryResult struct {
	Success      bool          `json:"success"`
	Attempts     int           `json:"attempts"`
	TotalTime    time.Duration `json:"total_time"`
	LastError    error         `json:"last_error,omitempty"`
	CircuitState string        `json:"circuit_state,omitempty"`
}

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker implements circuit breaker pattern
type CircuitBreaker struct {
	failureThreshold int
	successThreshold int
	timeout          time.Duration
	halfOpenMaxCalls int
	state            CircuitState
	failures         int
	successes        int
	lastFailureTime  time.Time
	halfOpenCalls    int
	mu               sync.RWMutex
}

// ReloadEvent represents a reload event
type ReloadEvent struct {
	Type      string        `json:"type"`
	Source    string        `json:"source"`
	Timestamp time.Time     `json:"timestamp"`
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration"`
}

// ReloadHandler is a function that handles reload events
type ReloadHandler func(event ReloadEvent) error

// ComponentReloader defines the interface for components that support hot reload
type ComponentReloader interface {
	Reload(configProvider config.Provider) error
	GetReloadStats() map[string]any
}

// InfraStats tracks consolidated infrastructure statistics
type InfraStats struct {
	// HTTP Client stats
	RequestsSent      int64            `json:"requests_sent"`
	RequestsSucceeded int64            `json:"requests_succeeded"`
	RequestsFailed    int64            `json:"requests_failed"`
	AverageLatency    time.Duration    `json:"average_latency"`
	TotalLatency      time.Duration    `json:"total_latency"`
	BytesSent         int64            `json:"bytes_sent"`
	BytesReceived     int64            `json:"bytes_received"`
	StatusCodes       map[int]int64    `json:"status_codes"`
	ErrorTypes        map[string]int64 `json:"error_types"`

	// Retry stats
	TotalRetries        int64         `json:"total_retries"`
	SuccessfulRetries   int64         `json:"successful_retries"`
	FailedRetries       int64         `json:"failed_retries"`
	AverageAttempts     float64       `json:"average_attempts"`
	AverageRetryTime    time.Duration `json:"average_retry_time"`
	TotalRetryTime      time.Duration `json:"total_retry_time"`
	CircuitBreakerTrips int64         `json:"circuit_breaker_trips"`

	// Reload stats
	TotalReloads      int64         `json:"total_reloads"`
	SuccessfulReloads int64         `json:"successful_reloads"`
	FailedReloads     int64         `json:"failed_reloads"`
	LastReloadTime    time.Time     `json:"last_reload_time"`
	AverageReloadTime time.Duration `json:"average_reload_time"`
	ConfigReloads     int64         `json:"config_reloads"`
	MIBReloads        int64         `json:"mib_reloads"`
	WebhookReloads    int64         `json:"webhook_reloads"`
}

// Manager provides consolidated infrastructure functionality
type Manager struct {
	config *InfraConfig
	logger logging.Logger

	// HTTP Client
	httpClient *http.Client

	// Circuit Breaker
	circuitBreaker *CircuitBreaker

	// Hot Reload
	watcher          *fsnotify.Watcher
	configManager    config.Manager
	configProvider   config.Provider
	components       map[string]ComponentReloader
	reloadHandlers   []ReloadHandler
	reloadEvents     []ReloadEvent
	lastReloadTime   time.Time
	reloadInProgress bool

	// Statistics and synchronization
	stats  *InfraStats
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewManager creates a new consolidated infrastructure manager
func NewManager(cfg config.Provider, logger logging.Logger) (*Manager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration provider cannot be nil")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	// Load configuration
	infraConfig := DefaultInfraConfig()

	// Override with config values if available
	if timeout, err := cfg.GetDuration("client.timeout", infraConfig.HTTPTimeout); err == nil {
		infraConfig.HTTPTimeout = timeout
	}

	if maxRetries, err := cfg.GetInt("client.max_retries", infraConfig.MaxRetries); err == nil {
		infraConfig.MaxRetries = maxRetries
	}

	if retryDelay, err := cfg.GetDuration("client.retry_delay", infraConfig.RetryDelay); err == nil {
		infraConfig.RetryDelay = retryDelay
	}

	if maxIdleConns, err := cfg.GetInt("client.max_idle_conns", infraConfig.MaxIdleConns); err == nil {
		infraConfig.MaxIdleConns = maxIdleConns
	}

	if maxIdleConnsPerHost, err := cfg.GetInt("client.max_idle_conns_per_host", infraConfig.MaxIdleConnsPerHost); err == nil {
		infraConfig.MaxIdleConnsPerHost = maxIdleConnsPerHost
	}

	if idleConnTimeout, err := cfg.GetDuration("client.idle_conn_timeout", infraConfig.IdleConnTimeout); err == nil {
		infraConfig.IdleConnTimeout = idleConnTimeout
	}

	if tlsHandshakeTimeout, err := cfg.GetDuration("client.tls_handshake_timeout", infraConfig.TLSHandshakeTimeout); err == nil {
		infraConfig.TLSHandshakeTimeout = tlsHandshakeTimeout
	}

	if insecureSkipVerify, err := cfg.GetBool("client.insecure_skip_verify", infraConfig.InsecureSkipVerify); err == nil {
		infraConfig.InsecureSkipVerify = insecureSkipVerify
	}

	if userAgent, err := cfg.GetString("client.user_agent", infraConfig.UserAgent); err == nil {
		infraConfig.UserAgent = userAgent
	}

	// Retry configuration
	if maxAttempts, err := cfg.GetInt("retry.max_attempts", infraConfig.MaxAttempts); err == nil {
		infraConfig.MaxAttempts = maxAttempts
	}

	if initialDelay, err := cfg.GetDuration("retry.initial_delay", infraConfig.InitialDelay); err == nil {
		infraConfig.InitialDelay = initialDelay
	}

	if maxDelay, err := cfg.GetDuration("retry.max_delay", infraConfig.MaxDelay); err == nil {
		infraConfig.MaxDelay = maxDelay
	}

	// Note: GetFloat64 may not be available, using default value
	// if backoffMultiplier, err := cfg.GetFloat64("retry.backoff_multiplier", infraConfig.BackoffMultiplier); err == nil {
	//     infraConfig.BackoffMultiplier = backoffMultiplier
	// }

	if jitter, err := cfg.GetBool("retry.jitter", infraConfig.Jitter); err == nil {
		infraConfig.Jitter = jitter
	}

	if enableCircuitBreaker, err := cfg.GetBool("retry.enable_circuit_breaker", infraConfig.EnableCircuitBreaker); err == nil {
		infraConfig.EnableCircuitBreaker = enableCircuitBreaker
	}

	// Reload configuration
	if enableHotReload, err := cfg.GetBool("reload.enabled", infraConfig.EnableHotReload); err == nil {
		infraConfig.EnableHotReload = enableHotReload
	}

	if watchConfigFile, err := cfg.GetBool("reload.watch_config_file", infraConfig.WatchConfigFile); err == nil {
		infraConfig.WatchConfigFile = watchConfigFile
	}

	if debounceDelay, err := cfg.GetDuration("reload.debounce_delay", infraConfig.DebounceDelay); err == nil {
		infraConfig.DebounceDelay = debounceDelay
	}

	ctx, cancel := context.WithCancel(context.Background())

	manager := &Manager{
		config:         infraConfig,
		logger:         logger,
		components:     make(map[string]ComponentReloader),
		reloadHandlers: make([]ReloadHandler, 0),
		reloadEvents:   make([]ReloadEvent, 0),
		stats: &InfraStats{
			StatusCodes: make(map[int]int64),
			ErrorTypes:  make(map[string]int64),
		},
		ctx:    ctx,
		cancel: cancel,
	}

	// Initialize HTTP client
	if err := manager.initializeHTTPClient(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize HTTP client: %w", err)
	}

	// Initialize circuit breaker if enabled
	if infraConfig.EnableCircuitBreaker {
		manager.circuitBreaker = &CircuitBreaker{
			failureThreshold: infraConfig.CircuitFailureThreshold,
			successThreshold: infraConfig.CircuitSuccessThreshold,
			timeout:          infraConfig.CircuitTimeout,
			halfOpenMaxCalls: infraConfig.CircuitHalfOpenMaxCalls,
			state:            CircuitClosed,
		}
	}

	return manager, nil
}

// initializeHTTPClient initializes the HTTP client with configured settings
func (m *Manager) initializeHTTPClient() error {
	transport := &http.Transport{
		MaxIdleConns:        m.config.MaxIdleConns,
		MaxIdleConnsPerHost: m.config.MaxIdleConnsPerHost,
		IdleConnTimeout:     m.config.IdleConnTimeout,
		TLSHandshakeTimeout: m.config.TLSHandshakeTimeout,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: m.config.InsecureSkipVerify,
		},
	}

	m.httpClient = &http.Client{
		Timeout:   m.config.HTTPTimeout,
		Transport: transport,
	}

	return nil
}

// SendWebhook sends a webhook request with retry logic
func (m *Manager) SendWebhook(ctx context.Context, request *WebhookRequest) (*WebhookResponse, error) {
	m.mu.Lock()
	m.stats.RequestsSent++
	m.mu.Unlock()

	startTime := time.Now()

	// Validate request
	if err := m.validateRequest(request); err != nil {
		m.recordError("validation_error")
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Use retry mechanism
	var response *WebhookResponse
	var lastErr error

	retryFunc := func(ctx context.Context, attempt int) error {
		resp, err := m.sendHTTPRequest(ctx, request)
		if err != nil {
			lastErr = err
			return err
		}
		response = resp
		return nil
	}

	result := m.Retry(ctx, retryFunc)

	// Record statistics
	responseTime := time.Since(startTime)
	m.mu.Lock()
	m.stats.TotalLatency += responseTime
	if result.Success && response != nil {
		m.stats.RequestsSucceeded++
		m.stats.StatusCodes[response.StatusCode]++
		m.stats.BytesReceived += response.ContentLength
	} else {
		m.stats.RequestsFailed++
		if lastErr != nil {
			m.stats.ErrorTypes[m.categorizeError(lastErr)]++
		}
	}
	if m.stats.RequestsSent > 0 {
		m.stats.AverageLatency = m.stats.TotalLatency / time.Duration(m.stats.RequestsSent)
	}
	m.mu.Unlock()

	if response != nil {
		response.Duration = responseTime
	}

	return response, result.LastError
}

// validateRequest validates a webhook request
func (m *Manager) validateRequest(request *WebhookRequest) error {
	if request == nil {
		return fmt.Errorf("request cannot be nil")
	}
	if request.URL == "" {
		return fmt.Errorf("URL cannot be empty")
	}
	if request.Method == "" {
		request.Method = "POST"
	}
	if request.Timeout == 0 {
		request.Timeout = m.config.HTTPTimeout
	}
	return nil
}

// sendHTTPRequest sends a single HTTP request
func (m *Manager) sendHTTPRequest(ctx context.Context, request *WebhookRequest) (*WebhookResponse, error) {
	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, request.Method, request.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	for key, value := range m.config.DefaultHeaders {
		httpReq.Header.Set(key, value)
	}
	for key, value := range request.Headers {
		httpReq.Header.Set(key, value)
	}

	// Set user agent
	httpReq.Header.Set("User-Agent", m.config.UserAgent)

	// Set body if provided
	if request.Body != nil && len(request.Body) > 0 {
		httpReq.Body = io.NopCloser(strings.NewReader(string(request.Body)))
		httpReq.ContentLength = int64(len(request.Body))
		m.mu.Lock()
		m.stats.BytesSent += int64(len(request.Body))
		m.mu.Unlock()
	}

	// Send request
	resp, err := m.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check response size limit
	if m.config.MaxResponseSize > 0 && int64(len(body)) > m.config.MaxResponseSize {
		return nil, fmt.Errorf("response size %d exceeds limit %d", len(body), m.config.MaxResponseSize)
	}

	// Convert http.Header to map[string]string
	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0] // Take first value
		}
	}

	response := &WebhookResponse{
		StatusCode:    resp.StatusCode,
		Headers:       headers,
		Body:          body,
		ContentLength: resp.ContentLength,
		Success:       resp.StatusCode >= 200 && resp.StatusCode < 300,
	}

	if !response.Success {
		response.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	return response, nil
}

// recordError records an error in statistics
func (m *Manager) recordError(errorType string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stats.ErrorTypes[errorType]++
}

// categorizeError categorizes an error for statistics
func (m *Manager) categorizeError(err error) string {
	if err == nil {
		return "unknown"
	}

	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "timeout"):
		return "timeout"
	case strings.Contains(errStr, "connection refused"):
		return "connection_refused"
	case strings.Contains(errStr, "no such host"):
		return "dns_error"
	case strings.Contains(errStr, "certificate"):
		return "tls_error"
	default:
		return "other"
	}
}

// Retry executes a function with retry logic and circuit breaker
func (m *Manager) Retry(ctx context.Context, fn RetryableFunc) *RetryResult {
	startTime := time.Now()
	result := &RetryResult{
		Success:   false,
		Attempts:  0,
		TotalTime: 0,
	}

	m.mu.Lock()
	m.stats.TotalRetries++
	m.mu.Unlock()

	// Check circuit breaker
	if m.circuitBreaker != nil {
		if !m.circuitBreaker.CanExecute() {
			result.LastError = fmt.Errorf("circuit breaker is open")
			result.CircuitState = m.circuitBreaker.state.String()
			m.mu.Lock()
			m.stats.FailedRetries++
			m.mu.Unlock()
			return result
		}
	}

	maxAttempts := m.config.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result.Attempts = attempt

		// Execute function
		err := fn(ctx, attempt)
		if err == nil {
			// Success
			result.Success = true
			result.TotalTime = time.Since(startTime)

			// Record circuit breaker success
			if m.circuitBreaker != nil {
				m.circuitBreaker.RecordSuccess()
			}

			m.mu.Lock()
			m.stats.SuccessfulRetries++
			m.stats.TotalRetryTime += result.TotalTime
			if m.stats.TotalRetries > 0 {
				m.stats.AverageAttempts = float64(m.stats.TotalRetries) / float64(m.stats.TotalRetries)
				m.stats.AverageRetryTime = m.stats.TotalRetryTime / time.Duration(m.stats.TotalRetries)
			}
			m.mu.Unlock()

			return result
		}

		result.LastError = err

		// Record circuit breaker failure
		if m.circuitBreaker != nil {
			m.circuitBreaker.RecordFailure()
		}

		// Don't wait after the last attempt
		if attempt < maxAttempts {
			delay := m.calculateDelay(attempt)
			select {
			case <-ctx.Done():
				result.LastError = ctx.Err()
				result.TotalTime = time.Since(startTime)
				m.mu.Lock()
				m.stats.FailedRetries++
				m.mu.Unlock()
				return result
			case <-time.After(delay):
				// Continue to next attempt
			}
		}
	}

	// All attempts failed
	result.TotalTime = time.Since(startTime)
	if m.circuitBreaker != nil {
		result.CircuitState = m.circuitBreaker.state.String()
	}

	m.mu.Lock()
	m.stats.FailedRetries++
	m.stats.TotalRetryTime += result.TotalTime
	if m.stats.TotalRetries > 0 {
		m.stats.AverageAttempts = float64(m.stats.TotalRetries) / float64(m.stats.TotalRetries)
		m.stats.AverageRetryTime = m.stats.TotalRetryTime / time.Duration(m.stats.TotalRetries)
	}
	m.mu.Unlock()

	return result
}

// calculateDelay calculates the delay for the next retry attempt
func (m *Manager) calculateDelay(attempt int) time.Duration {
	delay := m.config.InitialDelay

	// Apply exponential backoff
	if m.config.BackoffMultiplier > 1.0 {
		delay = time.Duration(float64(delay) * math.Pow(m.config.BackoffMultiplier, float64(attempt-1)))
	}

	// Apply maximum delay limit
	if delay > m.config.MaxDelay {
		delay = m.config.MaxDelay
	}

	// Apply jitter if enabled
	if m.config.Jitter {
		jitterRange := m.config.JitterRange
		if jitterRange <= 0 {
			jitterRange = 0.1 // Default 10% jitter
		}

		jitter := time.Duration(float64(delay) * jitterRange * (rand.Float64()*2 - 1))
		delay += jitter

		// Ensure delay is not negative
		if delay < 0 {
			delay = m.config.InitialDelay
		}
	}

	return delay
}

// CanExecute checks if the circuit breaker allows execution
func (cb *CircuitBreaker) CanExecute() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		// Check if timeout has passed
		if time.Since(cb.lastFailureTime) > cb.timeout {
			return true // Will transition to half-open
		}
		return false
	case CircuitHalfOpen:
		return cb.halfOpenCalls < cb.halfOpenMaxCalls
	default:
		return false
	}
}

// RecordSuccess records a successful execution
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		cb.failures = 0
	case CircuitHalfOpen:
		cb.successes++
		cb.halfOpenCalls++
		if cb.successes >= cb.successThreshold {
			cb.state = CircuitClosed
			cb.failures = 0
			cb.successes = 0
			cb.halfOpenCalls = 0
		}
	}
}

// RecordFailure records a failed execution
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		cb.failures++
		if cb.failures >= cb.failureThreshold {
			cb.state = CircuitOpen
			cb.lastFailureTime = time.Now()
		}
	case CircuitHalfOpen:
		cb.state = CircuitOpen
		cb.lastFailureTime = time.Now()
		cb.successes = 0
		cb.halfOpenCalls = 0
	}
}

// GetStats returns consolidated infrastructure statistics
func (m *Manager) GetStats() *InfraStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := *m.stats
	return &stats
}

// GetHTTPClient returns the underlying HTTP client for advanced usage
func (m *Manager) GetHTTPClient() *http.Client {
	return m.httpClient
}

// UpdateConfig updates the infrastructure configuration
func (m *Manager) UpdateConfig(cfg config.Provider) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create new config with updated values
	newConfig := DefaultInfraConfig()

	// Apply configuration updates (similar to NewManager)
	if timeout, err := cfg.GetDuration("client.timeout", newConfig.HTTPTimeout); err == nil {
		newConfig.HTTPTimeout = timeout
	}

	if maxRetries, err := cfg.GetInt("client.max_retries", newConfig.MaxRetries); err == nil {
		newConfig.MaxRetries = maxRetries
	}

	// Update the configuration
	m.config = newConfig

	// Reinitialize HTTP client with new settings
	return m.initializeHTTPClient()
}

// Close gracefully shuts down the infrastructure manager
func (m *Manager) Close() error {
	m.cancel()
	m.wg.Wait()

	if m.watcher != nil {
		return m.watcher.Close()
	}

	return nil
}

// IsHealthy returns the health status of the infrastructure manager
func (m *Manager) IsHealthy() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check if circuit breaker is not permanently open
	if m.circuitBreaker != nil && m.circuitBreaker.state == CircuitOpen {
		// Allow some time for recovery
		if time.Since(m.circuitBreaker.lastFailureTime) < m.config.CircuitTimeout {
			return false
		}
	}

	// Check recent error rates
	if m.stats.RequestsSent > 0 {
		errorRate := float64(m.stats.RequestsFailed) / float64(m.stats.RequestsSent)
		if errorRate > 0.5 { // More than 50% error rate
			return false
		}
	}

	return true
}

// GetCircuitBreakerState returns the current circuit breaker state
func (m *Manager) GetCircuitBreakerState() string {
	if m.circuitBreaker == nil {
		return "disabled"
	}

	m.circuitBreaker.mu.RLock()
	defer m.circuitBreaker.mu.RUnlock()
	return m.circuitBreaker.state.String()
}

// ResetStats resets all statistics
func (m *Manager) ResetStats() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats = &InfraStats{
		StatusCodes: make(map[int]int64),
		ErrorTypes:  make(map[string]int64),
	}
}

// StartHotReload starts the hot reload functionality
func (m *Manager) StartHotReload(configManager config.Manager, configProvider config.Provider) error {
	if !m.config.EnableHotReload {
		return nil
	}

	m.mu.Lock()
	m.configManager = configManager
	m.configProvider = configProvider
	m.mu.Unlock()

	var err error
	m.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	// Watch config file if enabled
	if m.config.WatchConfigFile && m.config.ConfigFile != "" {
		if err := m.watcher.Add(m.config.ConfigFile); err != nil {
			return fmt.Errorf("failed to watch config file %s: %w", m.config.ConfigFile, err)
		}
	}

	// Start watching in a goroutine
	m.wg.Add(1)
	go m.watchFiles()

	m.logger.Info("Hot reload started", "config_file", m.config.ConfigFile)
	return nil
}

// StopHotReload stops the hot reload functionality
func (m *Manager) StopHotReload() error {
	if m.watcher != nil {
		return m.watcher.Close()
	}
	return nil
}

// watchFiles watches for file system changes
func (m *Manager) watchFiles() {
	defer m.wg.Done()

	debounceTimer := time.NewTimer(0)
	if !debounceTimer.Stop() {
		<-debounceTimer.C
	}

	for {
		select {
		case <-m.ctx.Done():
			return

		case event, ok := <-m.watcher.Events:
			if !ok {
				return
			}

			// Handle file changes with debouncing
			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
				debounceTimer.Reset(m.config.DebounceDelay)
			}

		case err, ok := <-m.watcher.Errors:
			if !ok {
				return
			}
			m.logger.Error("File watcher error", "error", err)

		case <-debounceTimer.C:
			// Check rate limiting
			if time.Since(m.lastReloadTime) < m.config.MaxReloadFrequency {
				m.logger.Debug("Reload rate limited", "last_reload", m.lastReloadTime)
				continue
			}

			m.handleReload()
		}
	}
}

// handleReload handles a reload event
func (m *Manager) handleReload() {
	m.mu.Lock()
	if m.reloadInProgress {
		m.mu.Unlock()
		return
	}
	m.reloadInProgress = true
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.reloadInProgress = false
		m.lastReloadTime = time.Now()
		m.mu.Unlock()
	}()

	startTime := time.Now()
	event := ReloadEvent{
		Type:      "config",
		Source:    "file_watcher",
		Timestamp: startTime,
		Success:   false,
	}

	m.logger.Info("Starting configuration reload")

	// Reload configuration
	if m.configManager != nil {
		if err := m.configManager.Reload(); err != nil {
			event.Error = err.Error()
			event.Duration = time.Since(startTime)
			m.recordReloadEvent(event)
			m.logger.Error("Failed to reload configuration", "error", err)
			return
		}
	}

	// Update infrastructure configuration
	if m.configProvider != nil {
		if err := m.UpdateConfig(m.configProvider); err != nil {
			event.Error = err.Error()
			event.Duration = time.Since(startTime)
			m.recordReloadEvent(event)
			m.logger.Error("Failed to update infrastructure configuration", "error", err)
			return
		}
	}

	// Reload registered components
	for name, component := range m.components {
		if err := component.Reload(m.configProvider); err != nil {
			m.logger.Error("Failed to reload component", "component", name, "error", err)
			// Continue with other components
		}
	}

	// Notify reload handlers
	for _, handler := range m.reloadHandlers {
		if err := handler(event); err != nil {
			m.logger.Error("Reload handler failed", "error", err)
		}
	}

	event.Success = true
	event.Duration = time.Since(startTime)
	m.recordReloadEvent(event)

	m.logger.Info("Configuration reload completed", "duration", event.Duration)
}

// RegisterComponent registers a component for hot reload
func (m *Manager) RegisterComponent(name string, component ComponentReloader) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.components[name] = component
}

// UnregisterComponent unregisters a component from hot reload
func (m *Manager) UnregisterComponent(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.components, name)
}

// RegisterReloadHandler registers a handler for reload events
func (m *Manager) RegisterReloadHandler(handler ReloadHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reloadHandlers = append(m.reloadHandlers, handler)
}

// recordReloadEvent records a reload event in statistics
func (m *Manager) recordReloadEvent(event ReloadEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.TotalReloads++
	if event.Success {
		m.stats.SuccessfulReloads++
	} else {
		m.stats.FailedReloads++
	}

	m.stats.LastReloadTime = event.Timestamp
	if m.stats.TotalReloads > 0 {
		totalTime := m.stats.AverageReloadTime*time.Duration(m.stats.TotalReloads-1) + event.Duration
		m.stats.AverageReloadTime = totalTime / time.Duration(m.stats.TotalReloads)
	}

	// Update specific reload type counters
	switch event.Type {
	case "config":
		m.stats.ConfigReloads++
	case "mib":
		m.stats.MIBReloads++
	case "webhook":
		m.stats.WebhookReloads++
	}

	// Keep only recent events (last 100)
	m.reloadEvents = append(m.reloadEvents, event)
	if len(m.reloadEvents) > 100 {
		m.reloadEvents = m.reloadEvents[1:]
	}
}

// GetReloadEvents returns recent reload events
func (m *Manager) GetReloadEvents() []ReloadEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	events := make([]ReloadEvent, len(m.reloadEvents))
	copy(events, m.reloadEvents)
	return events
}

// TriggerReload manually triggers a reload
func (m *Manager) TriggerReload() {
	go m.handleReload()
}
