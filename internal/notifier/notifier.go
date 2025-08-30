// Package notifier provides webhook notification management and delivery.
package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/geekxflood/common/config"
	"github.com/geekxflood/nereus/internal/alerts"
	"github.com/geekxflood/nereus/internal/client"
	"github.com/geekxflood/nereus/internal/storage"
)

// NotifierConfig holds configuration for the notification system
type NotifierConfig struct {
	EnableNotifications bool              `json:"enable_notifications"`
	DefaultWebhooks     []WebhookConfig   `json:"default_webhooks"`
	MaxConcurrent       int               `json:"max_concurrent"`
	QueueSize           int               `json:"queue_size"`
	DeliveryTimeout     time.Duration     `json:"delivery_timeout"`
	RetryAttempts       int               `json:"retry_attempts"`
	RetryDelay          time.Duration     `json:"retry_delay"`
	FilterRules         []FilterRule      `json:"filter_rules"`
	Templates           map[string]string `json:"templates"`
	RateLimiting        RateLimitConfig   `json:"rate_limiting"`
}

// WebhookConfig represents a webhook endpoint configuration
type WebhookConfig struct {
	Name        string            `json:"name"`
	URL         string            `json:"url"`
	Method      string            `json:"method"`
	Headers     map[string]string `json:"headers"`
	Template    string            `json:"template"`
	Format      string            `json:"format"` // "prometheus", "alertmanager", "custom"
	Enabled     bool              `json:"enabled"`
	Filters     []string          `json:"filters"`
	Timeout     time.Duration     `json:"timeout"`
	RetryCount  int               `json:"retry_count"`
	ContentType string            `json:"content_type"`
}

// FilterRule represents a notification filter rule
type FilterRule struct {
	Name       string            `json:"name"`
	Enabled    bool              `json:"enabled"`
	Conditions []FilterCondition `json:"conditions"`
	Action     string            `json:"action"` // allow, deny, modify
	Parameters map[string]string `json:"parameters"`
}

// FilterCondition represents a condition in a filter rule
type FilterCondition struct {
	Field    string `json:"field"`
	Operator string `json:"operator"` // equals, contains, matches, greater_than, less_than
	Value    string `json:"value"`
	Negate   bool   `json:"negate"`
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Enabled           bool          `json:"enabled"`
	RequestsPerMinute int           `json:"requests_per_minute"`
	BurstSize         int           `json:"burst_size"`
	WindowSize        time.Duration `json:"window_size"`
}

// DefaultNotifierConfig returns a default notifier configuration
func DefaultNotifierConfig() *NotifierConfig {
	return &NotifierConfig{
		EnableNotifications: true,
		DefaultWebhooks:     []WebhookConfig{},
		MaxConcurrent:       10,
		QueueSize:           1000,
		DeliveryTimeout:     30 * time.Second,
		RetryAttempts:       3,
		RetryDelay:          5 * time.Second,
		FilterRules:         []FilterRule{},
		Templates: map[string]string{
			"default": `{
				"event_id": "{{.ID}}",
				"timestamp": "{{.Timestamp}}",
				"source_ip": "{{.SourceIP}}",
				"trap_oid": "{{.TrapOID}}",
				"trap_name": "{{.TrapName}}",
				"severity": "{{.Severity}}",
				"status": "{{.Status}}",
				"message": "SNMP trap received from {{.SourceIP}}: {{.TrapName}} ({{.TrapOID}})",
				"varbinds": {{.VarbindsJSON}}
			}`,
			"slack": `{
				"text": "SNMP Alert: {{.TrapName}}",
				"attachments": [{
					"color": "{{if eq .Severity \"critical\"}}danger{{else if eq .Severity \"major\"}}warning{{else}}good{{end}}",
					"fields": [
						{"title": "Source", "value": "{{.SourceIP}}", "short": true},
						{"title": "Severity", "value": "{{.Severity}}", "short": true},
						{"title": "Trap OID", "value": "{{.TrapOID}}", "short": false},
						{"title": "Timestamp", "value": "{{.Timestamp}}", "short": true}
					]
				}]
			}`,
		},
		RateLimiting: RateLimitConfig{
			Enabled:           false,
			RequestsPerMinute: 60,
			BurstSize:         10,
			WindowSize:        time.Minute,
		},
	}
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
	StatusCode   int           `json:"status_code"`
	ResponseTime time.Duration `json:"response_time"`
	Error        string        `json:"error,omitempty"`
	Webhook      string        `json:"webhook"`
	EventID      int64         `json:"event_id"`
	Attempt      int           `json:"attempt"`
}

// NotifierStats tracks notification statistics
type NotifierStats struct {
	NotificationsSent      int64                    `json:"notifications_sent"`
	NotificationsSucceeded int64                    `json:"notifications_succeeded"`
	NotificationsFailed    int64                    `json:"notifications_failed"`
	QueueLength            int                      `json:"queue_length"`
	QueueCapacity          int                      `json:"queue_capacity"`
	ActiveWorkers          int                      `json:"active_workers"`
	WebhookStats           map[string]*WebhookStats `json:"webhook_stats"`
	FilterStats            map[string]int64         `json:"filter_stats"`
	TemplateStats          map[string]int64         `json:"template_stats"`
	AverageDeliveryTime    time.Duration            `json:"average_delivery_time"`
	TotalDeliveryTime      time.Duration            `json:"total_delivery_time"`
}

// WebhookStats tracks statistics for individual webhooks
type WebhookStats struct {
	Name            string        `json:"name"`
	RequestsSent    int64         `json:"requests_sent"`
	RequestsSuccess int64         `json:"requests_success"`
	RequestsFailed  int64         `json:"requests_failed"`
	AverageLatency  time.Duration `json:"average_latency"`
	LastSuccess     *time.Time    `json:"last_success,omitempty"`
	LastFailure     *time.Time    `json:"last_failure,omitempty"`
	LastError       string        `json:"last_error,omitempty"`
}

// Notifier manages webhook notifications
type Notifier struct {
	config         *NotifierConfig
	client         *client.HTTPClient
	alertConverter *alerts.AlertConverter
	taskQueue      chan *NotificationTask
	templates      map[string]*template.Template
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	stats          *NotifierStats
	mu             sync.RWMutex
}

// NewNotifier creates a new notification manager
func NewNotifier(cfg config.Provider, httpClient *client.HTTPClient) (*Notifier, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration provider cannot be nil")
	}
	if httpClient == nil {
		return nil, fmt.Errorf("HTTP client cannot be nil")
	}

	// Load configuration
	notifierConfig := DefaultNotifierConfig()

	if enabled, err := cfg.GetBool("notifier.enable_notifications", notifierConfig.EnableNotifications); err == nil {
		notifierConfig.EnableNotifications = enabled
	}

	if maxConcurrent, err := cfg.GetInt("notifier.max_concurrent", notifierConfig.MaxConcurrent); err == nil {
		notifierConfig.MaxConcurrent = maxConcurrent
	}

	if queueSize, err := cfg.GetInt("notifier.queue_size", notifierConfig.QueueSize); err == nil {
		notifierConfig.QueueSize = queueSize
	}

	if timeout, err := cfg.GetDuration("notifier.delivery_timeout", notifierConfig.DeliveryTimeout); err == nil {
		notifierConfig.DeliveryTimeout = timeout
	}

	ctx, cancel := context.WithCancel(context.Background())

	notifier := &Notifier{
		config:         notifierConfig,
		client:         httpClient,
		alertConverter: alerts.NewAlertConverter(),
		taskQueue:      make(chan *NotificationTask, notifierConfig.QueueSize),
		templates:      make(map[string]*template.Template),
		ctx:            ctx,
		cancel:         cancel,
		stats: &NotifierStats{
			WebhookStats:  make(map[string]*WebhookStats),
			FilterStats:   make(map[string]int64),
			TemplateStats: make(map[string]int64),
		},
	}

	// Parse templates
	if err := notifier.parseTemplates(); err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	// Initialize webhook stats
	for _, webhook := range notifierConfig.DefaultWebhooks {
		notifier.stats.WebhookStats[webhook.Name] = &WebhookStats{
			Name: webhook.Name,
		}
	}

	// Start worker goroutines
	for i := 0; i < notifierConfig.MaxConcurrent; i++ {
		notifier.wg.Add(1)
		go notifier.worker(i)
	}

	return notifier, nil
}

// SendNotification sends a notification for an event
func (n *Notifier) SendNotification(event *storage.Event) error {
	if !n.config.EnableNotifications {
		return nil
	}

	n.mu.Lock()
	n.stats.NotificationsSent++
	n.mu.Unlock()

	// Apply filters
	if !n.shouldNotify(event) {
		return nil
	}

	// Send to all configured webhooks
	for _, webhook := range n.config.DefaultWebhooks {
		if !webhook.Enabled {
			continue
		}

		// Check webhook-specific filters
		if !n.webhookShouldNotify(&webhook, event) {
			continue
		}

		task := &NotificationTask{
			Event:     event,
			Webhook:   &webhook,
			Attempt:   0,
			CreatedAt: time.Now(),
		}

		// Try to queue the task
		select {
		case n.taskQueue <- task:
			// Successfully queued
		default:
			// Queue is full, drop notification
			n.mu.Lock()
			n.stats.NotificationsFailed++
			n.mu.Unlock()
			return fmt.Errorf("notification queue is full")
		}
	}

	return nil
}

// worker processes notification tasks from the queue
func (n *Notifier) worker(workerID int) {
	defer n.wg.Done()

	for {
		select {
		case <-n.ctx.Done():
			return
		case task := <-n.taskQueue:
			if task == nil {
				return
			}

			n.mu.Lock()
			n.stats.ActiveWorkers++
			n.mu.Unlock()

			result := n.deliverNotification(task)
			n.recordResult(result)

			n.mu.Lock()
			n.stats.ActiveWorkers--
			n.mu.Unlock()
		}
	}
}

// AddWebhook adds a new webhook configuration
func (n *Notifier) AddWebhook(webhook *WebhookConfig) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if webhook.Name == "" {
		return fmt.Errorf("webhook name cannot be empty")
	}

	// Check if webhook already exists
	for i, existing := range n.config.DefaultWebhooks {
		if existing.Name == webhook.Name {
			n.config.DefaultWebhooks[i] = *webhook
			return nil
		}
	}

	// Add new webhook
	n.config.DefaultWebhooks = append(n.config.DefaultWebhooks, *webhook)

	// Initialize stats
	n.stats.WebhookStats[webhook.Name] = &WebhookStats{
		Name: webhook.Name,
	}

	return nil
}

// RemoveWebhook removes a webhook configuration
func (n *Notifier) RemoveWebhook(name string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	for i, webhook := range n.config.DefaultWebhooks {
		if webhook.Name == name {
			// Remove webhook
			n.config.DefaultWebhooks = append(n.config.DefaultWebhooks[:i], n.config.DefaultWebhooks[i+1:]...)

			// Remove stats
			delete(n.stats.WebhookStats, name)
			return nil
		}
	}

	return fmt.Errorf("webhook not found: %s", name)
}

// GetWebhook returns a webhook configuration by name
func (n *Notifier) GetWebhook(name string) (*WebhookConfig, bool) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	for _, webhook := range n.config.DefaultWebhooks {
		if webhook.Name == name {
			return &webhook, true
		}
	}

	return nil, false
}

// GetAllWebhooks returns all webhook configurations
func (n *Notifier) GetAllWebhooks() []WebhookConfig {
	n.mu.RLock()
	defer n.mu.RUnlock()

	webhooks := make([]WebhookConfig, len(n.config.DefaultWebhooks))
	copy(webhooks, n.config.DefaultWebhooks)
	return webhooks
}

// AddFilterRule adds a new filter rule
func (n *Notifier) AddFilterRule(rule *FilterRule) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if rule.Name == "" {
		return fmt.Errorf("filter rule name cannot be empty")
	}

	// Check if rule already exists
	for i, existing := range n.config.FilterRules {
		if existing.Name == rule.Name {
			n.config.FilterRules[i] = *rule
			return nil
		}
	}

	// Add new rule
	n.config.FilterRules = append(n.config.FilterRules, *rule)
	n.stats.FilterStats[rule.Name] = 0

	return nil
}

// RemoveFilterRule removes a filter rule
func (n *Notifier) RemoveFilterRule(name string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	for i, rule := range n.config.FilterRules {
		if rule.Name == name {
			// Remove rule
			n.config.FilterRules = append(n.config.FilterRules[:i], n.config.FilterRules[i+1:]...)

			// Remove stats
			delete(n.stats.FilterStats, name)
			return nil
		}
	}

	return fmt.Errorf("filter rule not found: %s", name)
}

// GetFilterRule returns a filter rule by name
func (n *Notifier) GetFilterRule(name string) (*FilterRule, bool) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	for _, rule := range n.config.FilterRules {
		if rule.Name == name {
			return &rule, true
		}
	}

	return nil, false
}

// GetAllFilterRules returns all filter rules
func (n *Notifier) GetAllFilterRules() []FilterRule {
	n.mu.RLock()
	defer n.mu.RUnlock()

	rules := make([]FilterRule, len(n.config.FilterRules))
	copy(rules, n.config.FilterRules)
	return rules
}

// TestWebhook tests a webhook by sending a test notification
func (n *Notifier) TestWebhook(webhookName string) (*NotificationResult, error) {
	webhook, exists := n.GetWebhook(webhookName)
	if !exists {
		return nil, fmt.Errorf("webhook not found: %s", webhookName)
	}

	// Create test event
	testEvent := &storage.Event{
		ID:        0,
		Timestamp: time.Now(),
		SourceIP:  "127.0.0.1",
		Community: "public",
		Version:   1,
		PDUType:   7,
		RequestID: 12345,
		TrapOID:   "1.3.6.1.6.3.1.1.5.1",
		TrapName:  "coldStart",
		Severity:  "info",
		Status:    "open",
		Count:     1,
		FirstSeen: time.Now(),
		LastSeen:  time.Now(),
		Varbinds:  `[{"oid":"1.3.6.1.2.1.1.3.0","type":67,"value":12345}]`,
		Metadata:  `{"test":true}`,
	}

	// Create test task
	task := &NotificationTask{
		Event:     testEvent,
		Webhook:   webhook,
		Attempt:   0,
		CreatedAt: time.Now(),
	}

	return n.deliverNotification(task), nil
}

// SendPrometheusAlert sends a notification using Prometheus alert format
func (n *Notifier) SendPrometheusAlert(event *storage.Event, webhookName string) error {
	n.mu.RLock()
	defer n.mu.RUnlock()

	// Find webhook by name in default webhooks
	var webhook *WebhookConfig
	for i := range n.config.DefaultWebhooks {
		if n.config.DefaultWebhooks[i].Name == webhookName {
			webhook = &n.config.DefaultWebhooks[i]
			break
		}
	}

	if webhook == nil {
		return fmt.Errorf("webhook %s not found", webhookName)
	}

	if !webhook.Enabled {
		return fmt.Errorf("webhook %s is disabled", webhookName)
	}

	// Create notification task with Prometheus format
	task := &NotificationTask{
		Event:     event,
		Webhook:   webhook,
		Attempt:   0,
		CreatedAt: time.Now(),
	}

	// Send to task queue
	select {
	case n.taskQueue <- task:
		return nil
	default:
		n.mu.Lock()
		n.stats.QueueFull++
		n.mu.Unlock()
		return fmt.Errorf("notification queue is full")
	}
}

// GetStats returns notifier statistics
func (n *Notifier) GetStats() *NotifierStats {
	n.mu.RLock()
	defer n.mu.RUnlock()

	// Create a copy to avoid race conditions
	stats := &NotifierStats{
		NotificationsSent:      n.stats.NotificationsSent,
		NotificationsSucceeded: n.stats.NotificationsSucceeded,
		NotificationsFailed:    n.stats.NotificationsFailed,
		QueueLength:            len(n.taskQueue),
		QueueCapacity:          cap(n.taskQueue),
		ActiveWorkers:          n.stats.ActiveWorkers,
		AverageDeliveryTime:    n.stats.AverageDeliveryTime,
		TotalDeliveryTime:      n.stats.TotalDeliveryTime,
		WebhookStats:           make(map[string]*WebhookStats),
		FilterStats:            make(map[string]int64),
		TemplateStats:          make(map[string]int64),
	}

	// Copy webhook stats
	for name, webhookStats := range n.stats.WebhookStats {
		stats.WebhookStats[name] = &WebhookStats{
			Name:            webhookStats.Name,
			RequestsSent:    webhookStats.RequestsSent,
			RequestsSuccess: webhookStats.RequestsSuccess,
			RequestsFailed:  webhookStats.RequestsFailed,
			AverageLatency:  webhookStats.AverageLatency,
			LastSuccess:     webhookStats.LastSuccess,
			LastFailure:     webhookStats.LastFailure,
			LastError:       webhookStats.LastError,
		}
	}

	// Copy filter stats
	for name, count := range n.stats.FilterStats {
		stats.FilterStats[name] = count
	}

	// Copy template stats
	for name, count := range n.stats.TemplateStats {
		stats.TemplateStats[name] = count
	}

	return stats
}

// GetConfig returns the notifier configuration
func (n *Notifier) GetConfig() *NotifierConfig {
	return n.config
}

// UpdateConfig updates the notifier configuration
func (n *Notifier) UpdateConfig(config *NotifierConfig) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.config = config

	// Re-parse templates
	n.templates = make(map[string]*template.Template)
	return n.parseTemplates()
}

// ResetStats resets notifier statistics
func (n *Notifier) ResetStats() {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.stats = &NotifierStats{
		WebhookStats:  make(map[string]*WebhookStats),
		FilterStats:   make(map[string]int64),
		TemplateStats: make(map[string]int64),
	}

	// Reinitialize webhook stats
	for _, webhook := range n.config.DefaultWebhooks {
		n.stats.WebhookStats[webhook.Name] = &WebhookStats{
			Name: webhook.Name,
		}
	}
}

// Close shuts down the notifier
func (n *Notifier) Close() error {
	n.cancel()
	close(n.taskQueue)
	n.wg.Wait()
	return nil
}

// deliverNotification delivers a notification to a webhook
func (n *Notifier) deliverNotification(task *NotificationTask) *NotificationResult {
	startTime := time.Now()

	var payload []byte
	var err error

	// Choose payload format based on webhook configuration
	switch task.Webhook.Format {
	case "prometheus", "alertmanager":
		// Use Prometheus alert format
		alertPayload, convertErr := n.alertConverter.ConvertEventToAlertManagerPayload(task.Event)
		if convertErr != nil {
			return &NotificationResult{
				Success:      false,
				Error:        fmt.Sprintf("prometheus alert conversion failed: %v", convertErr),
				Webhook:      task.Webhook.Name,
				EventID:      task.Event.ID,
				Attempt:      task.Attempt,
				ResponseTime: time.Since(startTime),
			}
		}
		payload, err = json.Marshal(alertPayload)
		if err != nil {
			return &NotificationResult{
				Success:      false,
				Error:        fmt.Sprintf("prometheus alert marshaling failed: %v", err),
				Webhook:      task.Webhook.Name,
				EventID:      task.Event.ID,
				Attempt:      task.Attempt,
				ResponseTime: time.Since(startTime),
			}
		}
	default:
		// Use custom template rendering
		templateResult, err := n.renderTemplate(task.Webhook.Template, task.Event)
		if err != nil {
			return &NotificationResult{
				Success:      false,
				Error:        fmt.Sprintf("template rendering failed: %v", err),
				Webhook:      task.Webhook.Name,
				EventID:      task.Event.ID,
				Attempt:      task.Attempt,
				ResponseTime: time.Since(startTime),
			}
		}
		// Convert template result to bytes
		if str, ok := templateResult.(string); ok {
			payload = []byte(str)
		} else {
			payload, err = json.Marshal(templateResult)
			if err != nil {
				return &NotificationResult{
					Success:      false,
					Error:        fmt.Sprintf("template result marshaling failed: %v", err),
					Webhook:      task.Webhook.Name,
					EventID:      task.Event.ID,
					Attempt:      task.Attempt,
					ResponseTime: time.Since(startTime),
				}
			}
		}
	}

	// Create webhook request
	request := &client.WebhookRequest{
		URL:         task.Webhook.URL,
		Method:      task.Webhook.Method,
		Headers:     task.Webhook.Headers,
		Body:        payload,
		ContentType: task.Webhook.ContentType,
	}

	if request.Method == "" {
		request.Method = "POST"
	}
	if request.ContentType == "" {
		request.ContentType = "application/json"
	}

	// Set timeout
	ctx := n.ctx
	if task.Webhook.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(n.ctx, task.Webhook.Timeout)
		defer cancel()
	}

	// Send webhook
	response, err := n.client.SendWebhook(ctx, request)
	responseTime := time.Since(startTime)

	result := &NotificationResult{
		Webhook:      task.Webhook.Name,
		EventID:      task.Event.ID,
		Attempt:      task.Attempt,
		ResponseTime: responseTime,
	}

	if err != nil {
		result.Success = false
		result.Error = err.Error()
	} else {
		result.Success = response.Success
		result.StatusCode = response.StatusCode
		if !response.Success {
			result.Error = response.Error
		}
	}

	return result
}

// renderTemplate renders a notification template with event data
func (n *Notifier) renderTemplate(templateName string, event *storage.Event) (interface{}, error) {
	if templateName == "" {
		templateName = "default"
	}

	tmpl, exists := n.templates[templateName]
	if !exists {
		return nil, fmt.Errorf("template not found: %s", templateName)
	}

	n.mu.Lock()
	n.stats.TemplateStats[templateName]++
	n.mu.Unlock()

	// Prepare template data
	data := map[string]interface{}{
		"ID":           event.ID,
		"Timestamp":    event.Timestamp.Format(time.RFC3339),
		"SourceIP":     event.SourceIP,
		"Community":    event.Community,
		"Version":      event.Version,
		"PDUType":      event.PDUType,
		"RequestID":    event.RequestID,
		"TrapOID":      event.TrapOID,
		"TrapName":     event.TrapName,
		"Severity":     event.Severity,
		"Status":       event.Status,
		"Acknowledged": event.Acknowledged,
		"Count":        event.Count,
		"FirstSeen":    event.FirstSeen.Format(time.RFC3339),
		"LastSeen":     event.LastSeen.Format(time.RFC3339),
	}

	// Add varbinds as JSON string
	if event.Varbinds != "" {
		data["VarbindsJSON"] = event.Varbinds
	} else {
		data["VarbindsJSON"] = "[]"
	}

	// Add metadata
	if event.Metadata != "" {
		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(event.Metadata), &metadata); err == nil {
			for key, value := range metadata {
				data[key] = value
			}
		}
	}

	// Render template
	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("template execution failed: %w", err)
	}

	// Try to parse as JSON, otherwise return as string
	var result interface{}
	if err := json.Unmarshal([]byte(buf.String()), &result); err != nil {
		result = buf.String()
	}

	return result, nil
}

// parseTemplates parses all configured templates
func (n *Notifier) parseTemplates() error {
	for name, templateStr := range n.config.Templates {
		tmpl, err := template.New(name).Parse(templateStr)
		if err != nil {
			return fmt.Errorf("failed to parse template %s: %w", name, err)
		}
		n.templates[name] = tmpl
	}
	return nil
}

// shouldNotify checks if an event should trigger notifications based on global filters
func (n *Notifier) shouldNotify(event *storage.Event) bool {
	for _, rule := range n.config.FilterRules {
		if !rule.Enabled {
			continue
		}

		if n.evaluateFilterRule(&rule, event) {
			n.mu.Lock()
			n.stats.FilterStats[rule.Name]++
			n.mu.Unlock()

			switch rule.Action {
			case "deny":
				return false
			case "allow":
				return true
			}
		}
	}

	// Default to allow if no rules match
	return true
}

// webhookShouldNotify checks if a specific webhook should receive the notification
func (n *Notifier) webhookShouldNotify(webhook *WebhookConfig, event *storage.Event) bool {
	// Check webhook-specific filters
	for _, filterName := range webhook.Filters {
		for _, rule := range n.config.FilterRules {
			if rule.Name == filterName && rule.Enabled {
				if n.evaluateFilterRule(&rule, event) {
					if rule.Action == "deny" {
						return false
					}
				}
			}
		}
	}
	return true
}

// evaluateFilterRule evaluates a filter rule against an event
func (n *Notifier) evaluateFilterRule(rule *FilterRule, event *storage.Event) bool {
	for _, condition := range rule.Conditions {
		if !n.evaluateFilterCondition(&condition, event) {
			return false // All conditions must match
		}
	}
	return true
}

// evaluateFilterCondition evaluates a single filter condition
func (n *Notifier) evaluateFilterCondition(condition *FilterCondition, event *storage.Event) bool {
	var fieldValue string

	// Get field value
	switch condition.Field {
	case "source_ip":
		fieldValue = event.SourceIP
	case "community":
		fieldValue = event.Community
	case "trap_oid":
		fieldValue = event.TrapOID
	case "trap_name":
		fieldValue = event.TrapName
	case "severity":
		fieldValue = event.Severity
	case "status":
		fieldValue = event.Status
	default:
		// Check metadata
		if event.Metadata != "" {
			var metadata map[string]interface{}
			if err := json.Unmarshal([]byte(event.Metadata), &metadata); err == nil {
				if value, exists := metadata[condition.Field]; exists {
					fieldValue = fmt.Sprintf("%v", value)
				}
			}
		}
	}

	// Evaluate condition
	result := false
	switch condition.Operator {
	case "equals":
		result = fieldValue == condition.Value
	case "contains":
		result = strings.Contains(fieldValue, condition.Value)
	case "matches":
		result = strings.Contains(fieldValue, condition.Value)
	case "not_equals":
		result = fieldValue != condition.Value
	}

	// Apply negation if specified
	if condition.Negate {
		result = !result
	}

	return result
}

// recordResult records the result of a notification delivery
func (n *Notifier) recordResult(result *NotificationResult) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.stats.TotalDeliveryTime += result.ResponseTime

	if result.Success {
		n.stats.NotificationsSucceeded++
	} else {
		n.stats.NotificationsFailed++
	}

	// Update webhook stats
	if webhookStats, exists := n.stats.WebhookStats[result.Webhook]; exists {
		webhookStats.RequestsSent++
		if result.Success {
			webhookStats.RequestsSuccess++
			now := time.Now()
			webhookStats.LastSuccess = &now
		} else {
			webhookStats.RequestsFailed++
			now := time.Now()
			webhookStats.LastFailure = &now
			webhookStats.LastError = result.Error
		}

		// Update average latency
		if webhookStats.RequestsSent > 0 {
			totalLatency := webhookStats.AverageLatency * time.Duration(webhookStats.RequestsSent-1)
			webhookStats.AverageLatency = (totalLatency + result.ResponseTime) / time.Duration(webhookStats.RequestsSent)
		}
	}

	// Update overall average
	if n.stats.NotificationsSent > 0 {
		n.stats.AverageDeliveryTime = n.stats.TotalDeliveryTime / time.Duration(n.stats.NotificationsSent)
	}
}
