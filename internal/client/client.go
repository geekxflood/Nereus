// Package client provides HTTP client functionality for webhook notifications.
package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/geekxflood/common/config"
)

// ClientConfig holds configuration for the HTTP client
type ClientConfig struct {
	Timeout             time.Duration     `json:"timeout"`
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
}

// DefaultClientConfig returns a default client configuration
func DefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		Timeout:             30 * time.Second,
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
	}
}

// WebhookRequest represents a webhook HTTP request
type WebhookRequest struct {
	URL         string            `json:"url"`
	Method      string            `json:"method"`
	Headers     map[string]string `json:"headers"`
	Body        interface{}       `json:"body"`
	Timeout     time.Duration     `json:"timeout,omitempty"`
	RetryCount  int               `json:"retry_count,omitempty"`
	ContentType string            `json:"content_type,omitempty"`
}

// WebhookResponse represents a webhook HTTP response
type WebhookResponse struct {
	StatusCode    int               `json:"status_code"`
	Headers       map[string]string `json:"headers"`
	Body          []byte            `json:"body"`
	ResponseTime  time.Duration     `json:"response_time"`
	ContentLength int64             `json:"content_length"`
	Success       bool              `json:"success"`
	Error         string            `json:"error,omitempty"`
}

// ClientStats tracks HTTP client statistics
type ClientStats struct {
	RequestsSent      int64            `json:"requests_sent"`
	RequestsSucceeded int64            `json:"requests_succeeded"`
	RequestsFailed    int64            `json:"requests_failed"`
	TotalRetries      int64            `json:"total_retries"`
	AverageLatency    time.Duration    `json:"average_latency"`
	TotalLatency      time.Duration    `json:"total_latency"`
	BytesSent         int64            `json:"bytes_sent"`
	BytesReceived     int64            `json:"bytes_received"`
	StatusCodes       map[int]int64    `json:"status_codes"`
	ErrorTypes        map[string]int64 `json:"error_types"`
}

// HTTPClient provides HTTP client functionality for webhooks
type HTTPClient struct {
	config     *ClientConfig
	httpClient *http.Client
	stats      *ClientStats
	mu         sync.RWMutex
}

// NewHTTPClient creates a new HTTP client with the given configuration
func NewHTTPClient(cfg config.Provider) (*HTTPClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration provider cannot be nil")
	}

	// Load configuration
	clientConfig := DefaultClientConfig()

	if timeout, err := cfg.GetDuration("client.timeout", clientConfig.Timeout); err == nil {
		clientConfig.Timeout = timeout
	}

	if maxRetries, err := cfg.GetInt("client.max_retries", clientConfig.MaxRetries); err == nil {
		clientConfig.MaxRetries = maxRetries
	}

	if retryDelay, err := cfg.GetDuration("client.retry_delay", clientConfig.RetryDelay); err == nil {
		clientConfig.RetryDelay = retryDelay
	}

	if maxIdleConns, err := cfg.GetInt("client.max_idle_conns", clientConfig.MaxIdleConns); err == nil {
		clientConfig.MaxIdleConns = maxIdleConns
	}

	if insecureSkipVerify, err := cfg.GetBool("client.insecure_skip_verify", clientConfig.InsecureSkipVerify); err == nil {
		clientConfig.InsecureSkipVerify = insecureSkipVerify
	}

	if userAgent, err := cfg.GetString("client.user_agent", clientConfig.UserAgent); err == nil {
		clientConfig.UserAgent = userAgent
	}

	// Create HTTP transport
	transport := &http.Transport{
		MaxIdleConns:        clientConfig.MaxIdleConns,
		MaxIdleConnsPerHost: clientConfig.MaxIdleConnsPerHost,
		IdleConnTimeout:     clientConfig.IdleConnTimeout,
		TLSHandshakeTimeout: clientConfig.TLSHandshakeTimeout,
		DisableCompression:  !clientConfig.EnableCompression,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: clientConfig.InsecureSkipVerify,
		},
	}

	// Create HTTP client
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   clientConfig.Timeout,
	}

	client := &HTTPClient{
		config:     clientConfig,
		httpClient: httpClient,
		stats: &ClientStats{
			StatusCodes: make(map[int]int64),
			ErrorTypes:  make(map[string]int64),
		},
	}

	return client, nil
}

// SendWebhook sends a webhook request
func (c *HTTPClient) SendWebhook(ctx context.Context, request *WebhookRequest) (*WebhookResponse, error) {
	c.mu.Lock()
	c.stats.RequestsSent++
	c.mu.Unlock()

	startTime := time.Now()

	// Validate request
	if err := c.validateRequest(request); err != nil {
		c.recordError("validation_error")
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Create HTTP request
	httpReq, err := c.createHTTPRequest(ctx, request)
	if err != nil {
		c.recordError("request_creation_error")
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Send request with retries
	var response *WebhookResponse
	var lastErr error

	maxAttempts := c.config.MaxRetries + 1
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			c.mu.Lock()
			c.stats.TotalRetries++
			c.mu.Unlock()

			// Wait before retry
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(c.config.RetryDelay * time.Duration(attempt)):
				// Continue with retry
			}
		}

		response, lastErr = c.executeRequest(httpReq)
		if lastErr == nil && response.Success {
			break
		}
	}

	// Record statistics
	responseTime := time.Since(startTime)
	c.mu.Lock()
	c.stats.TotalLatency += responseTime
	if response != nil && response.Success {
		c.stats.RequestsSucceeded++
		c.stats.StatusCodes[response.StatusCode]++
		c.stats.BytesReceived += response.ContentLength
	} else {
		c.stats.RequestsFailed++
		if lastErr != nil {
			c.stats.ErrorTypes[c.categorizeError(lastErr)]++
		}
	}
	c.stats.AverageLatency = c.stats.TotalLatency / time.Duration(c.stats.RequestsSent)
	c.mu.Unlock()

	if response != nil {
		response.ResponseTime = responseTime
	}

	return response, lastErr
}

// validateRequest validates a webhook request
func (c *HTTPClient) validateRequest(request *WebhookRequest) error {
	if request == nil {
		return fmt.Errorf("request cannot be nil")
	}

	if request.URL == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	// Validate URL format
	if _, err := url.Parse(request.URL); err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Set default method if not specified
	if request.Method == "" {
		request.Method = "POST"
	}

	// Validate HTTP method
	validMethods := []string{"GET", "POST", "PUT", "PATCH", "DELETE"}
	methodValid := false
	for _, method := range validMethods {
		if strings.ToUpper(request.Method) == method {
			methodValid = true
			break
		}
	}
	if !methodValid {
		return fmt.Errorf("invalid HTTP method: %s", request.Method)
	}

	return nil
}

// createHTTPRequest creates an HTTP request from a webhook request
func (c *HTTPClient) createHTTPRequest(ctx context.Context, request *WebhookRequest) (*http.Request, error) {
	var body io.Reader
	var contentLength int64

	// Serialize body if present
	if request.Body != nil {
		var bodyBytes []byte
		var err error

		switch v := request.Body.(type) {
		case []byte:
			bodyBytes = v
		case string:
			bodyBytes = []byte(v)
		default:
			// JSON serialize
			bodyBytes, err = json.Marshal(v)
			if err != nil {
				return nil, fmt.Errorf("failed to serialize body: %w", err)
			}
		}

		body = bytes.NewReader(bodyBytes)
		contentLength = int64(len(bodyBytes))

		c.mu.Lock()
		c.stats.BytesSent += contentLength
		c.mu.Unlock()
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, strings.ToUpper(request.Method), request.URL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set default headers
	for key, value := range c.config.DefaultHeaders {
		httpReq.Header.Set(key, value)
	}

	// Set User-Agent
	httpReq.Header.Set("User-Agent", c.config.UserAgent)

	// Set custom headers
	for key, value := range request.Headers {
		httpReq.Header.Set(key, value)
	}

	// Set content type if specified
	if request.ContentType != "" {
		httpReq.Header.Set("Content-Type", request.ContentType)
	}

	// Set content length
	if contentLength > 0 {
		httpReq.ContentLength = contentLength
	}

	return httpReq, nil
}

// executeRequest executes an HTTP request
func (c *HTTPClient) executeRequest(httpReq *http.Request) (*WebhookResponse, error) {
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return &WebhookResponse{
			Success: false,
			Error:   err.Error(),
		}, err
	}
	defer resp.Body.Close()

	// Read response body with size limit
	var bodyBytes []byte
	if resp.ContentLength >= 0 && resp.ContentLength <= c.config.MaxResponseSize {
		bodyBytes = make([]byte, resp.ContentLength)
		_, err = io.ReadFull(resp.Body, bodyBytes)
	} else {
		// Use limited reader for unknown or large content
		limitedReader := io.LimitReader(resp.Body, c.config.MaxResponseSize)
		bodyBytes, err = io.ReadAll(limitedReader)
	}

	if err != nil && err != io.EOF {
		return &WebhookResponse{
			StatusCode: resp.StatusCode,
			Success:    false,
			Error:      fmt.Sprintf("failed to read response body: %v", err),
		}, err
	}

	// Convert headers to map
	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	// Determine success based on status code
	success := resp.StatusCode >= 200 && resp.StatusCode < 300

	response := &WebhookResponse{
		StatusCode:    resp.StatusCode,
		Headers:       headers,
		Body:          bodyBytes,
		ContentLength: int64(len(bodyBytes)),
		Success:       success,
	}

	if !success {
		response.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	return response, nil
}

// recordError records an error in statistics
func (c *HTTPClient) recordError(errorType string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stats.ErrorTypes[errorType]++
}

// categorizeError categorizes an error for statistics
func (c *HTTPClient) categorizeError(err error) string {
	errStr := err.Error()

	if strings.Contains(errStr, "timeout") {
		return "timeout"
	}
	if strings.Contains(errStr, "connection refused") {
		return "connection_refused"
	}
	if strings.Contains(errStr, "no such host") {
		return "dns_error"
	}
	if strings.Contains(errStr, "certificate") || strings.Contains(errStr, "tls") {
		return "tls_error"
	}

	return "other"
}

// GetStats returns client statistics
func (c *HTTPClient) GetStats() *ClientStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Create a copy to avoid race conditions
	stats := &ClientStats{
		RequestsSent:      c.stats.RequestsSent,
		RequestsSucceeded: c.stats.RequestsSucceeded,
		RequestsFailed:    c.stats.RequestsFailed,
		TotalRetries:      c.stats.TotalRetries,
		AverageLatency:    c.stats.AverageLatency,
		TotalLatency:      c.stats.TotalLatency,
		BytesSent:         c.stats.BytesSent,
		BytesReceived:     c.stats.BytesReceived,
		StatusCodes:       make(map[int]int64),
		ErrorTypes:        make(map[string]int64),
	}

	// Copy maps
	for code, count := range c.stats.StatusCodes {
		stats.StatusCodes[code] = count
	}
	for errType, count := range c.stats.ErrorTypes {
		stats.ErrorTypes[errType] = count
	}

	return stats
}

// GetConfig returns the client configuration
func (c *HTTPClient) GetConfig() *ClientConfig {
	return c.config
}

// UpdateConfig updates the client configuration
func (c *HTTPClient) UpdateConfig(config *ClientConfig) {
	c.config = config

	// Update HTTP client timeout
	c.httpClient.Timeout = config.Timeout
}

// ResetStats resets client statistics
func (c *HTTPClient) ResetStats() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.stats = &ClientStats{
		StatusCodes: make(map[int]int64),
		ErrorTypes:  make(map[string]int64),
	}
}

// Close closes the HTTP client and cleans up resources
func (c *HTTPClient) Close() error {
	// Close idle connections
	c.httpClient.CloseIdleConnections()
	return nil
}
