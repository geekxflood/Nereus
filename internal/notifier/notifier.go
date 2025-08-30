// Package notifier provides simplified notification services focused on Alertmanager integration.
package notifier

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"text/template"
	"time"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/geekxflood/common/config"
	"github.com/geekxflood/nereus/internal/alerts"
	"github.com/geekxflood/nereus/internal/client"
	"github.com/geekxflood/nereus/internal/storage"
)

//go:embed templates/default.cue
var defaultTemplate embed.FS

// NotifierConfig defines the configuration for the notification system
type NotifierConfig struct {
	EnableNotifications bool               `json:"enable_notifications"`
	MaxConcurrent       int                `json:"max_concurrent"`
	QueueSize           int                `json:"queue_size"`
	DeliveryTimeout     time.Duration      `json:"delivery_timeout"`
	RetryAttempts       int                `json:"retry_attempts"`
	RetryDelay          time.Duration      `json:"retry_delay"`
	DefaultWebhooks     []WebhookConfig    `json:"default_webhooks"`
	FilterRules         []FilterRule       `json:"filter_rules"`
	RateLimiting        RateLimitingConfig `json:"rate_limiting"`
}

// WebhookConfig defines a webhook endpoint configuration
type WebhookConfig struct {
	Name        string            `json:"name"`
	URL         string            `json:"url"`
	Method      string            `json:"method"`
	Format      string            `json:"format"` // "alertmanager", "prometheus", "custom"
	Template    string            `json:"template"`
	Enabled     bool              `json:"enabled"`
	Filters     []string          `json:"filters"`
	Timeout     time.Duration     `json:"timeout"`
	RetryCount  int               `json:"retry_count"`
	ContentType string            `json:"content_type"`
	Headers     map[string]string `json:"headers"`
}

// FilterRule defines a notification filter rule
type FilterRule struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Conditions  []FilterCondition `json:"conditions"`
	Enabled     bool              `json:"enabled"`
}

// FilterCondition defines a single filter condition
type FilterCondition struct {
	Field    string   `json:"field"`
	Operator string   `json:"operator"`
	Value    string   `json:"value"`
	Values   []string `json:"values"`
}

// RateLimitingConfig defines rate limiting configuration
type RateLimitingConfig struct {
	Enabled           bool          `json:"enabled"`
	RequestsPerMinute int           `json:"requests_per_minute"`
	BurstSize         int           `json:"burst_size"`
	WindowSize        time.Duration `json:"window_size"`
}

// NotificationTask represents a notification delivery task
type NotificationTask struct {
	Event     *storage.Event `json:"event"`
	Webhook   *WebhookConfig `json:"webhook"`
	Attempt   int            `json:"attempt"`
	CreatedAt time.Time      `json:"created_at"`
}

// NotificationResult represents the result of a notification delivery
type NotificationResult struct {
	Success      bool          `json:"success"`
	Error        string        `json:"error,omitempty"`
	Webhook      string        `json:"webhook"`
	EventID      int64         `json:"event_id"`
	Attempt      int           `json:"attempt"`
	ResponseTime time.Duration `json:"response_time"`
}

// NotifierStats tracks notification statistics
type NotifierStats struct {
	NotificationsSent      int64                    `json:"notifications_sent"`
	NotificationsSucceeded int64                    `json:"notifications_succeeded"`
	NotificationsFailed    int64                    `json:"notifications_failed"`
	QueueLength            int                      `json:"queue_length"`
	QueueCapacity          int                      `json:"queue_capacity"`
	QueueFull              int64                    `json:"queue_full"`
	ActiveWorkers          int                      `json:"active_workers"`
	WebhookStats           map[string]*WebhookStats `json:"webhook_stats"`
	FilterStats            map[string]int64         `json:"filter_stats"`
	AverageDeliveryTime    time.Duration            `json:"average_delivery_time"`
	TotalDeliveryTime      time.Duration            `json:"total_delivery_time"`
}

// WebhookStats tracks statistics for individual webhooks
type WebhookStats struct {
	Name            string        `json:"name"`
	RequestsSent    int64         `json:"requests_sent"`
	RequestsSuccess int64         `json:"requests_success"`
	RequestsFailed  int64         `json:"requests_failed"`
	LastUsed        time.Time     `json:"last_used"`
	AverageLatency  time.Duration `json:"average_latency"`
	TotalLatency    time.Duration `json:"total_latency"`
}

// Notifier manages webhook notifications with focus on Alertmanager integration
type Notifier struct {
	config          *NotifierConfig
	client          *client.HTTPClient
	alertConverter  *alerts.AlertConverter
	defaultTemplate *template.Template
	taskQueue       chan *NotificationTask
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	stats           *NotifierStats
	mu              sync.RWMutex
}

// NewNotifier creates a new notification service
func NewNotifier(cfg config.Provider, httpClient *client.HTTPClient) (*Notifier, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration provider cannot be nil")
	}
	if httpClient == nil {
		return nil, fmt.Errorf("HTTP client cannot be nil")
	}

	// Load notifier configuration
	notifierConfig, err := loadNotifierConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to load notifier configuration: %w", err)
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())

	// Load default template from embedded CUE file
	defaultTemplate, err := loadDefaultTemplate()
	if err != nil {
		return nil, fmt.Errorf("failed to load default template: %w", err)
	}

	// Create alert converter for Prometheus/Alertmanager integration
	alertConverter := alerts.NewAlertConverter()

	notifier := &Notifier{
		config:          notifierConfig,
		client:          httpClient,
		alertConverter:  alertConverter,
		defaultTemplate: defaultTemplate,
		taskQueue:       make(chan *NotificationTask, notifierConfig.QueueSize),
		ctx:             ctx,
		cancel:          cancel,
		stats: &NotifierStats{
			WebhookStats: make(map[string]*WebhookStats),
			FilterStats:  make(map[string]int64),
		},
	}

	// Initialize webhook stats
	for _, webhook := range notifierConfig.DefaultWebhooks {
		notifier.stats.WebhookStats[webhook.Name] = &WebhookStats{
			Name: webhook.Name,
		}
	}

	return notifier, nil
}

// loadNotifierConfig loads the notifier configuration from the config provider
func loadNotifierConfig(cfg config.Provider) (*NotifierConfig, error) {
	// Set default configuration
	defaultConfig := &NotifierConfig{
		EnableNotifications: true,
		MaxConcurrent:       5,
		QueueSize:           1000,
		DeliveryTimeout:     30 * time.Second,
		RetryAttempts:       3,
		RetryDelay:          5 * time.Second,
		DefaultWebhooks:     []WebhookConfig{},
		FilterRules:         []FilterRule{},
		RateLimiting: RateLimitingConfig{
			Enabled:           false,
			RequestsPerMinute: 60,
			BurstSize:         10,
			WindowSize:        time.Minute,
		},
	}

	// Try to load configuration from provider
	if enableNotifications, err := cfg.GetBool("notifier.enable_notifications"); err == nil {
		defaultConfig.EnableNotifications = enableNotifications
	}

	if maxConcurrent, err := cfg.GetInt("notifier.max_concurrent"); err == nil {
		defaultConfig.MaxConcurrent = maxConcurrent
	}

	if queueSize, err := cfg.GetInt("notifier.queue_size"); err == nil {
		defaultConfig.QueueSize = queueSize
	}

	if deliveryTimeout, err := cfg.GetDuration("notifier.delivery_timeout"); err == nil {
		defaultConfig.DeliveryTimeout = deliveryTimeout
	}

	if retryAttempts, err := cfg.GetInt("notifier.retry_attempts"); err == nil {
		defaultConfig.RetryAttempts = retryAttempts
	}

	if retryDelay, err := cfg.GetDuration("notifier.retry_delay"); err == nil {
		defaultConfig.RetryDelay = retryDelay
	}

	// Load webhook configurations
	if webhooks, err := loadWebhookConfigs(cfg); err == nil {
		defaultConfig.DefaultWebhooks = webhooks
	}

	return defaultConfig, nil
}

// loadWebhookConfigs loads webhook configurations from the config provider
func loadWebhookConfigs(cfg config.Provider) ([]WebhookConfig, error) {
	var webhooks []WebhookConfig

	// Try to get webhook configurations from different possible paths
	webhookPaths := []string{
		"notifier.default_webhooks",
		"webhooks",
		"notifier.webhooks",
	}

	for _, path := range webhookPaths {
		// Try to get webhook configurations as a map first
		if webhookMap, err := cfg.GetMap(path); err == nil {
			// Convert to JSON and back to parse the webhook configurations
			jsonData, err := json.Marshal(webhookMap)
			if err != nil {
				continue
			}

			var parsedWebhooks []WebhookConfig
			if err := json.Unmarshal(jsonData, &parsedWebhooks); err != nil {
				continue
			}

			// Set defaults for webhook configurations
			for i := range parsedWebhooks {
				if parsedWebhooks[i].Method == "" {
					parsedWebhooks[i].Method = "POST"
				}
				if parsedWebhooks[i].Format == "" {
					parsedWebhooks[i].Format = "alertmanager"
				}
				if parsedWebhooks[i].ContentType == "" {
					parsedWebhooks[i].ContentType = "application/json"
				}
				if parsedWebhooks[i].Timeout == 0 {
					parsedWebhooks[i].Timeout = 10 * time.Second
				}
				if parsedWebhooks[i].RetryCount == 0 {
					parsedWebhooks[i].RetryCount = 3
				}
				if parsedWebhooks[i].Headers == nil {
					parsedWebhooks[i].Headers = make(map[string]string)
				}
			}

			webhooks = parsedWebhooks
			break
		}
	}

	return webhooks, nil
}

// loadDefaultTemplate loads the default template from the embedded CUE file
func loadDefaultTemplate() (*template.Template, error) {
	// Read the embedded CUE file
	content, err := defaultTemplate.ReadFile("templates/default.cue")
	if err != nil {
		return nil, fmt.Errorf("failed to read default template: %w", err)
	}

	// Parse CUE content
	ctx := cuecontext.New()
	value := ctx.CompileBytes(content)
	if err := value.Err(); err != nil {
		return nil, fmt.Errorf("failed to compile CUE template: %w", err)
	}

	// Extract template definition
	templateDef := value.LookupPath(cue.ParsePath("#DefaultTemplate"))
	if !templateDef.Exists() {
		return nil, fmt.Errorf("template definition #DefaultTemplate not found")
	}

	// Extract template string
	templateStr := templateDef.LookupPath(cue.ParsePath("template"))
	if !templateStr.Exists() {
		return nil, fmt.Errorf("template string not found in CUE definition")
	}

	templateContent, err := templateStr.String()
	if err != nil {
		return nil, fmt.Errorf("failed to extract template string: %w", err)
	}

	// Create Go template
	tmpl, err := template.New("default").Parse(templateContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Go template: %w", err)
	}

	return tmpl, nil
}

// Start starts the notification service
func (n *Notifier) Start() error {
	if !n.config.EnableNotifications {
		slog.Info("Notifications are disabled")
		return nil
	}

	slog.Info("Starting notification service",
		"max_concurrent", n.config.MaxConcurrent,
		"queue_size", n.config.QueueSize,
		"webhooks", len(n.config.DefaultWebhooks))

	// Start worker goroutines
	for i := 0; i < n.config.MaxConcurrent; i++ {
		n.wg.Add(1)
		go n.worker(i)
	}

	return nil
}

// Stop stops the notification service gracefully
func (n *Notifier) Stop() error {
	slog.Info("Stopping notification service")

	// Cancel context to signal workers to stop
	n.cancel()

	// Wait for all workers to finish
	n.wg.Wait()

	// Close task queue
	close(n.taskQueue)

	slog.Info("Notification service stopped")
	return nil
}

// SendNotification sends a notification for the given event
func (n *Notifier) SendNotification(event *storage.Event) error {
	if !n.config.EnableNotifications {
		return nil
	}

	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}

	// Send to all enabled webhooks
	var lastError error
	sent := 0

	for _, webhook := range n.config.DefaultWebhooks {
		if !webhook.Enabled {
			continue
		}

		// Apply filters
		if !n.passesFilters(event, webhook.Filters) {
			n.mu.Lock()
			n.stats.FilterStats[webhook.Name]++
			n.mu.Unlock()
			continue
		}

		// Create notification task
		task := &NotificationTask{
			Event:     event,
			Webhook:   &webhook,
			Attempt:   0,
			CreatedAt: time.Now(),
		}

		// Send to task queue
		select {
		case n.taskQueue <- task:
			sent++
		default:
			n.mu.Lock()
			n.stats.QueueFull++
			n.mu.Unlock()
			lastError = fmt.Errorf("notification queue is full")
		}
	}

	if sent == 0 && lastError != nil {
		return lastError
	}

	return nil
}

// worker processes notification tasks from the queue
func (n *Notifier) worker(id int) {
	defer n.wg.Done()

	slog.Debug("Starting notification worker", "worker_id", id)

	for {
		select {
		case <-n.ctx.Done():
			slog.Debug("Notification worker stopping", "worker_id", id)
			return
		case task, ok := <-n.taskQueue:
			if !ok {
				slog.Debug("Notification worker stopping - queue closed", "worker_id", id)
				return
			}

			n.mu.Lock()
			n.stats.ActiveWorkers++
			n.mu.Unlock()

			result := n.processTask(task)
			n.recordResult(result)

			n.mu.Lock()
			n.stats.ActiveWorkers--
			n.mu.Unlock()
		}
	}
}

// processTask processes a single notification task
func (n *Notifier) processTask(task *NotificationTask) *NotificationResult {
	startTime := time.Now()

	result := &NotificationResult{
		Webhook:      task.Webhook.Name,
		EventID:      task.Event.ID,
		Attempt:      task.Attempt,
		ResponseTime: 0,
	}

	// Generate payload based on webhook format
	payload, err := n.generatePayload(task.Event, task.Webhook)
	if err != nil {
		result.Error = fmt.Sprintf("payload generation failed: %v", err)
		result.ResponseTime = time.Since(startTime)
		return result
	}

	// Send HTTP request
	err = n.sendHTTPRequest(task.Webhook, payload)
	result.ResponseTime = time.Since(startTime)

	if err != nil {
		result.Error = err.Error()

		// Retry logic
		if task.Attempt < task.Webhook.RetryCount {
			task.Attempt++

			// Schedule retry after delay
			go func() {
				time.Sleep(n.config.RetryDelay)
				select {
				case n.taskQueue <- task:
					// Retry scheduled
				default:
					// Queue full, drop retry
					slog.Warn("Failed to schedule retry - queue full",
						"webhook", task.Webhook.Name,
						"event_id", task.Event.ID)
				}
			}()
		}

		return result
	}

	result.Success = true
	return result
}

// generatePayload generates the notification payload based on webhook format
func (n *Notifier) generatePayload(event *storage.Event, webhook *WebhookConfig) ([]byte, error) {
	switch webhook.Format {
	case "alertmanager", "prometheus":
		// Use Prometheus alert format for Alertmanager
		payload, err := n.alertConverter.ConvertEventToAlertManagerPayload(event)
		if err != nil {
			return nil, fmt.Errorf("failed to convert to Alertmanager format: %w", err)
		}
		return json.Marshal(payload)

	case "custom":
		// Use default template for custom format
		return n.renderDefaultTemplate(event)

	default:
		// Default to Alertmanager format
		payload, err := n.alertConverter.ConvertEventToAlertManagerPayload(event)
		if err != nil {
			return nil, fmt.Errorf("failed to convert to Alertmanager format: %w", err)
		}
		return json.Marshal(payload)
	}
}

// renderDefaultTemplate renders the default template with event data
func (n *Notifier) renderDefaultTemplate(event *storage.Event) ([]byte, error) {
	// Prepare template data
	data := map[string]interface{}{
		"ID":            event.ID,
		"Timestamp":     event.Timestamp.Format(time.RFC3339),
		"SourceIP":      event.SourceIP,
		"Community":     event.Community,
		"Version":       event.Version,
		"PDUType":       event.PDUType,
		"RequestID":     event.RequestID,
		"TrapOID":       event.TrapOID,
		"TrapName":      event.TrapName,
		"Severity":      event.Severity,
		"Status":        event.Status,
		"Acknowledged":  event.Acknowledged,
		"Count":         event.Count,
		"FirstSeen":     event.FirstSeen.Format(time.RFC3339),
		"LastSeen":      event.LastSeen.Format(time.RFC3339),
		"CorrelationID": event.CorrelationID,
	}

	// Add varbinds as JSON string
	if event.Varbinds != "" {
		data["VarbindsJSON"] = event.Varbinds
	} else {
		data["VarbindsJSON"] = "[]"
	}

	// Render template
	var buf bytes.Buffer
	if err := n.defaultTemplate.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("template execution failed: %w", err)
	}

	return buf.Bytes(), nil
}

// sendHTTPRequest sends an HTTP request to the webhook endpoint
func (n *Notifier) sendHTTPRequest(webhook *WebhookConfig, payload []byte) error {
	// Create request context with timeout
	ctx, cancel := context.WithTimeout(n.ctx, webhook.Timeout)
	defer cancel()

	// Prepare request
	req := &client.WebhookRequest{
		Method:  webhook.Method,
		URL:     webhook.URL,
		Body:    payload,
		Headers: webhook.Headers,
		Timeout: webhook.Timeout,
	}

	// Set default headers if not provided
	if req.Headers == nil {
		req.Headers = make(map[string]string)
	}
	if req.Headers["User-Agent"] == "" {
		req.Headers["User-Agent"] = "nereus-snmp-listener/1.0.0"
	}
	if req.Headers["Content-Type"] == "" {
		req.Headers["Content-Type"] = webhook.ContentType
	}

	// Send request
	resp, err := n.client.SendWebhook(ctx, req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}

	// Check response status
	if !resp.Success {
		return fmt.Errorf("HTTP request failed: %s", resp.Error)
	}

	return nil
}

// passesFilters checks if an event passes the specified filters
func (n *Notifier) passesFilters(event *storage.Event, filterNames []string) bool {
	if len(filterNames) == 0 {
		return true // No filters means pass all
	}

	// Find and apply each filter
	for _, filterName := range filterNames {
		var filter *FilterRule
		for i := range n.config.FilterRules {
			if n.config.FilterRules[i].Name == filterName {
				filter = &n.config.FilterRules[i]
				break
			}
		}

		if filter == nil || !filter.Enabled {
			continue // Skip unknown or disabled filters
		}

		if !n.evaluateFilter(event, filter) {
			return false // Event failed this filter
		}
	}

	return true // Event passed all filters
}

// evaluateFilter evaluates a single filter against an event
func (n *Notifier) evaluateFilter(event *storage.Event, filter *FilterRule) bool {
	for _, condition := range filter.Conditions {
		if !n.evaluateCondition(event, condition) {
			return false // All conditions must pass
		}
	}
	return true
}

// evaluateCondition evaluates a single filter condition against an event
func (n *Notifier) evaluateCondition(event *storage.Event, condition FilterCondition) bool {
	// Get field value from event
	fieldValue := n.getEventFieldValue(event, condition.Field)

	switch condition.Operator {
	case "equals":
		return fieldValue == condition.Value
	case "not_equals":
		return fieldValue != condition.Value
	case "contains":
		return bytes.Contains([]byte(fieldValue), []byte(condition.Value))
	case "not_contains":
		return !bytes.Contains([]byte(fieldValue), []byte(condition.Value))
	case "in":
		for _, value := range condition.Values {
			if fieldValue == value {
				return true
			}
		}
		return false
	case "not_in":
		for _, value := range condition.Values {
			if fieldValue == value {
				return false
			}
		}
		return true
	default:
		return false // Unknown operator
	}
}

// getEventFieldValue extracts a field value from an event
func (n *Notifier) getEventFieldValue(event *storage.Event, fieldName string) string {
	switch fieldName {
	case "id":
		return fmt.Sprintf("%d", event.ID)
	case "source_ip":
		return event.SourceIP
	case "community":
		return event.Community
	case "trap_oid":
		return event.TrapOID
	case "trap_name":
		return event.TrapName
	case "severity":
		return event.Severity
	case "status":
		return event.Status
	case "acknowledged":
		return fmt.Sprintf("%t", event.Acknowledged)
	case "correlation_id":
		return event.CorrelationID
	case "version":
		return fmt.Sprintf("%d", event.Version)
	case "pdu_type":
		return fmt.Sprintf("%d", event.PDUType)
	case "count":
		return fmt.Sprintf("%d", event.Count)
	default:
		return ""
	}
}

// recordResult records the result of a notification delivery
func (n *Notifier) recordResult(result *NotificationResult) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Update global stats
	n.stats.NotificationsSent++
	n.stats.TotalDeliveryTime += result.ResponseTime
	n.stats.AverageDeliveryTime = n.stats.TotalDeliveryTime / time.Duration(n.stats.NotificationsSent)

	if result.Success {
		n.stats.NotificationsSucceeded++
	} else {
		n.stats.NotificationsFailed++
	}

	// Update webhook-specific stats
	if webhookStats, exists := n.stats.WebhookStats[result.Webhook]; exists {
		webhookStats.RequestsSent++
		webhookStats.TotalLatency += result.ResponseTime
		webhookStats.AverageLatency = webhookStats.TotalLatency / time.Duration(webhookStats.RequestsSent)
		webhookStats.LastUsed = time.Now()

		if result.Success {
			webhookStats.RequestsSuccess++
		} else {
			webhookStats.RequestsFailed++
		}
	}

	// Log result
	if result.Success {
		slog.Debug("Notification delivered successfully",
			"webhook", result.Webhook,
			"event_id", result.EventID,
			"attempt", result.Attempt,
			"response_time", result.ResponseTime)
	} else {
		slog.Warn("Notification delivery failed",
			"webhook", result.Webhook,
			"event_id", result.EventID,
			"attempt", result.Attempt,
			"error", result.Error,
			"response_time", result.ResponseTime)
	}
}

// GetStats returns current notification statistics
func (n *Notifier) GetStats() *NotifierStats {
	n.mu.RLock()
	defer n.mu.RUnlock()

	// Update queue length
	n.stats.QueueLength = len(n.taskQueue)
	n.stats.QueueCapacity = cap(n.taskQueue)

	// Create a deep copy to avoid race conditions
	statsCopy := &NotifierStats{
		NotificationsSent:      n.stats.NotificationsSent,
		NotificationsSucceeded: n.stats.NotificationsSucceeded,
		NotificationsFailed:    n.stats.NotificationsFailed,
		QueueLength:            n.stats.QueueLength,
		QueueCapacity:          n.stats.QueueCapacity,
		QueueFull:              n.stats.QueueFull,
		ActiveWorkers:          n.stats.ActiveWorkers,
		AverageDeliveryTime:    n.stats.AverageDeliveryTime,
		TotalDeliveryTime:      n.stats.TotalDeliveryTime,
		WebhookStats:           make(map[string]*WebhookStats),
		FilterStats:            make(map[string]int64),
	}

	// Copy webhook stats
	for name, stats := range n.stats.WebhookStats {
		statsCopy.WebhookStats[name] = &WebhookStats{
			Name:            stats.Name,
			RequestsSent:    stats.RequestsSent,
			RequestsSuccess: stats.RequestsSuccess,
			RequestsFailed:  stats.RequestsFailed,
			LastUsed:        stats.LastUsed,
			AverageLatency:  stats.AverageLatency,
			TotalLatency:    stats.TotalLatency,
		}
	}

	// Copy filter stats
	for name, count := range n.stats.FilterStats {
		statsCopy.FilterStats[name] = count
	}

	return statsCopy
}

// UpdateConfig updates the notifier configuration
func (n *Notifier) UpdateConfig(cfg config.Provider) error {
	newConfig, err := loadNotifierConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to load new configuration: %w", err)
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	n.config = newConfig

	// Update webhook stats for new webhooks
	for _, webhook := range newConfig.DefaultWebhooks {
		if _, exists := n.stats.WebhookStats[webhook.Name]; !exists {
			n.stats.WebhookStats[webhook.Name] = &WebhookStats{
				Name: webhook.Name,
			}
		}
	}

	slog.Info("Notifier configuration updated",
		"webhooks", len(newConfig.DefaultWebhooks),
		"filters", len(newConfig.FilterRules))

	return nil
}
