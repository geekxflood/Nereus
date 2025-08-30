// Package metrics provides Prometheus metrics integration and system monitoring
package metrics

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/geekxflood/common/config"
	"github.com/geekxflood/common/logging"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsConfig defines the configuration for the metrics system
type MetricsConfig struct {
	Enabled        bool          `json:"enabled"`
	ListenAddress  string        `json:"listen_address"`
	MetricsPath    string        `json:"metrics_path"`
	HealthPath     string        `json:"health_path"`
	ReadyPath      string        `json:"ready_path"`
	UpdateInterval time.Duration `json:"update_interval"`
	Namespace      string        `json:"namespace"`
}

// DefaultMetricsConfig returns the default metrics configuration
func DefaultMetricsConfig() *MetricsConfig {
	return &MetricsConfig{
		Enabled:        true,
		ListenAddress:  ":9090",
		MetricsPath:    "/metrics",
		HealthPath:     "/health",
		ReadyPath:      "/ready",
		UpdateInterval: 30 * time.Second,
		Namespace:      "nereus",
	}
}

// MetricsManager manages Prometheus metrics and health endpoints
type MetricsManager struct {
	config   *MetricsConfig
	logger   logging.Logger
	registry *prometheus.Registry
	server   *http.Server

	// Application metrics
	trapMetrics       *TrapMetrics
	storageMetrics    *StorageMetrics
	webhookMetrics    *WebhookMetrics
	systemMetrics     *SystemMetrics
	correlatorMetrics *CorrelatorMetrics

	// Health status
	healthStatus map[string]bool
	readyStatus  bool
	mu           sync.RWMutex

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// TrapMetrics contains SNMP trap processing metrics
type TrapMetrics struct {
	TrapsReceived  prometheus.Counter
	TrapsProcessed prometheus.Counter
	TrapsFailed    prometheus.Counter
	ProcessingTime prometheus.Histogram
	PacketSize     prometheus.Histogram
	TrapsPerSource *prometheus.CounterVec
	TrapsByType    *prometheus.CounterVec
}

// StorageMetrics contains storage system metrics
type StorageMetrics struct {
	EventsStored   prometheus.Counter
	StorageErrors  prometheus.Counter
	QueryDuration  prometheus.Histogram
	DatabaseSize   prometheus.Gauge
	ActiveQueries  prometheus.Gauge
	EventsRetained prometheus.Gauge
}

// WebhookMetrics contains webhook delivery metrics
type WebhookMetrics struct {
	WebhooksDelivered prometheus.Counter
	WebhooksFailed    prometheus.Counter
	DeliveryTime      prometheus.Histogram
	RetryAttempts     prometheus.Counter
	QueueLength       prometheus.Gauge
	ActiveWorkers     prometheus.Gauge
	WebhooksByStatus  *prometheus.CounterVec
}

// SystemMetrics contains system resource metrics
type SystemMetrics struct {
	CPUUsage        prometheus.Gauge
	MemoryUsage     prometheus.Gauge
	GoroutineCount  prometheus.Gauge
	GCDuration      prometheus.Histogram
	Uptime          prometheus.Gauge
	FileDescriptors prometheus.Gauge
}

// CorrelatorMetrics contains correlation engine metrics
type CorrelatorMetrics struct {
	EventsCorrelated prometheus.Counter
	CorrelationRules prometheus.Gauge
	ActiveGroups     prometheus.Gauge
	FlappingEvents   prometheus.Counter
	CorrelationTime  prometheus.Histogram
	RuleEvaluations  prometheus.Counter
}

// NewMetricsManager creates a new metrics manager
func NewMetricsManager(cfg config.Provider, logger logging.Logger) (*MetricsManager, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	// Load metrics configuration
	metricsConfig, err := loadMetricsConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to load metrics configuration: %w", err)
	}

	// Create custom registry
	registry := prometheus.NewRegistry()

	ctx, cancel := context.WithCancel(context.Background())

	manager := &MetricsManager{
		config:       metricsConfig,
		logger:       logger.With("component", "metrics"),
		registry:     registry,
		healthStatus: make(map[string]bool),
		readyStatus:  false,
		ctx:          ctx,
		cancel:       cancel,
	}

	// Initialize metrics
	if err := manager.initializeMetrics(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize metrics: %w", err)
	}

	return manager, nil
}

// initializeMetrics creates and registers all Prometheus metrics
func (m *MetricsManager) initializeMetrics() error {
	namespace := m.config.Namespace

	// Initialize trap metrics
	m.trapMetrics = &TrapMetrics{
		TrapsReceived: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "traps_received_total",
			Help:      "Total number of SNMP traps received",
		}),
		TrapsProcessed: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "traps_processed_total",
			Help:      "Total number of SNMP traps successfully processed",
		}),
		TrapsFailed: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "traps_failed_total",
			Help:      "Total number of SNMP traps that failed processing",
		}),
		ProcessingTime: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "trap_processing_duration_seconds",
			Help:      "Time spent processing SNMP traps",
			Buckets:   prometheus.DefBuckets,
		}),
		PacketSize: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "trap_packet_size_bytes",
			Help:      "Size of SNMP trap packets",
			Buckets:   []float64{64, 128, 256, 512, 1024, 2048, 4096, 8192},
		}),
		TrapsPerSource: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "traps_by_source_total",
			Help:      "Total number of traps by source IP",
		}, []string{"source_ip"}),
		TrapsByType: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "traps_by_type_total",
			Help:      "Total number of traps by trap OID",
		}, []string{"trap_oid", "trap_name"}),
	}

	// Initialize storage metrics
	m.storageMetrics = &StorageMetrics{
		EventsStored: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "events_stored_total",
			Help:      "Total number of events stored in database",
		}),
		StorageErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "storage_errors_total",
			Help:      "Total number of storage errors",
		}),
		QueryDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "storage_query_duration_seconds",
			Help:      "Time spent executing database queries",
			Buckets:   prometheus.DefBuckets,
		}),
		DatabaseSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "database_size_bytes",
			Help:      "Current size of the database file",
		}),
		ActiveQueries: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "storage_active_queries",
			Help:      "Number of currently active database queries",
		}),
		EventsRetained: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "events_retained_total",
			Help:      "Total number of events currently retained in storage",
		}),
	}

	// Initialize webhook metrics
	m.webhookMetrics = &WebhookMetrics{
		WebhooksDelivered: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "webhooks_delivered_total",
			Help:      "Total number of webhooks successfully delivered",
		}),
		WebhooksFailed: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "webhooks_failed_total",
			Help:      "Total number of webhook delivery failures",
		}),
		DeliveryTime: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "webhook_delivery_duration_seconds",
			Help:      "Time spent delivering webhooks",
			Buckets:   prometheus.DefBuckets,
		}),
		RetryAttempts: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "webhook_retries_total",
			Help:      "Total number of webhook retry attempts",
		}),
		QueueLength: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "webhook_queue_length",
			Help:      "Current length of the webhook delivery queue",
		}),
		ActiveWorkers: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "webhook_active_workers",
			Help:      "Number of active webhook delivery workers",
		}),
		WebhooksByStatus: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "webhooks_by_status_total",
			Help:      "Total number of webhooks by HTTP status code",
		}, []string{"status_code", "webhook_name"}),
	}

	// Initialize system metrics
	m.systemMetrics = &SystemMetrics{
		CPUUsage: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "cpu_usage_percent",
			Help:      "Current CPU usage percentage",
		}),
		MemoryUsage: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "memory_usage_bytes",
			Help:      "Current memory usage in bytes",
		}),
		GoroutineCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "goroutines_total",
			Help:      "Current number of goroutines",
		}),
		GCDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "gc_duration_seconds",
			Help:      "Time spent in garbage collection",
			Buckets:   prometheus.DefBuckets,
		}),
		Uptime: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "uptime_seconds",
			Help:      "Application uptime in seconds",
		}),
		FileDescriptors: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "file_descriptors_total",
			Help:      "Number of open file descriptors",
		}),
	}

	// Initialize correlator metrics
	m.correlatorMetrics = &CorrelatorMetrics{
		EventsCorrelated: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "events_correlated_total",
			Help:      "Total number of events processed by correlator",
		}),
		CorrelationRules: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "correlation_rules_total",
			Help:      "Number of active correlation rules",
		}),
		ActiveGroups: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "correlation_groups_active",
			Help:      "Number of active correlation groups",
		}),
		FlappingEvents: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "flapping_events_total",
			Help:      "Total number of flapping events detected",
		}),
		CorrelationTime: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "correlation_duration_seconds",
			Help:      "Time spent correlating events",
			Buckets:   prometheus.DefBuckets,
		}),
		RuleEvaluations: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "correlation_rule_evaluations_total",
			Help:      "Total number of correlation rule evaluations",
		}),
	}

	// Register all metrics
	collectors := []prometheus.Collector{
		// Trap metrics
		m.trapMetrics.TrapsReceived,
		m.trapMetrics.TrapsProcessed,
		m.trapMetrics.TrapsFailed,
		m.trapMetrics.ProcessingTime,
		m.trapMetrics.PacketSize,
		m.trapMetrics.TrapsPerSource,
		m.trapMetrics.TrapsByType,

		// Storage metrics
		m.storageMetrics.EventsStored,
		m.storageMetrics.StorageErrors,
		m.storageMetrics.QueryDuration,
		m.storageMetrics.DatabaseSize,
		m.storageMetrics.ActiveQueries,
		m.storageMetrics.EventsRetained,

		// Webhook metrics
		m.webhookMetrics.WebhooksDelivered,
		m.webhookMetrics.WebhooksFailed,
		m.webhookMetrics.DeliveryTime,
		m.webhookMetrics.RetryAttempts,
		m.webhookMetrics.QueueLength,
		m.webhookMetrics.ActiveWorkers,
		m.webhookMetrics.WebhooksByStatus,

		// System metrics
		m.systemMetrics.CPUUsage,
		m.systemMetrics.MemoryUsage,
		m.systemMetrics.GoroutineCount,
		m.systemMetrics.GCDuration,
		m.systemMetrics.Uptime,
		m.systemMetrics.FileDescriptors,

		// Correlator metrics
		m.correlatorMetrics.EventsCorrelated,
		m.correlatorMetrics.CorrelationRules,
		m.correlatorMetrics.ActiveGroups,
		m.correlatorMetrics.FlappingEvents,
		m.correlatorMetrics.CorrelationTime,
		m.correlatorMetrics.RuleEvaluations,
	}

	for _, collector := range collectors {
		if err := m.registry.Register(collector); err != nil {
			return fmt.Errorf("failed to register metric: %w", err)
		}
	}

	return nil
}

// Start starts the metrics server and background monitoring
func (m *MetricsManager) Start() error {
	if !m.config.Enabled {
		m.logger.Info("Metrics collection is disabled")
		return nil
	}

	m.logger.Info("Starting metrics server",
		"listen_address", m.config.ListenAddress,
		"metrics_path", m.config.MetricsPath)

	// Create HTTP server
	mux := http.NewServeMux()

	// Metrics endpoint
	mux.Handle(m.config.MetricsPath, promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	}))

	// Health endpoint
	mux.HandleFunc(m.config.HealthPath, m.healthHandler)

	// Ready endpoint
	mux.HandleFunc(m.config.ReadyPath, m.readyHandler)

	m.server = &http.Server{
		Addr:    m.config.ListenAddress,
		Handler: mux,
	}

	// Start server in background
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		if err := m.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			m.logger.Error("Metrics server error", "error", err.Error())
		}
	}()

	// Start system metrics collection
	m.wg.Add(1)
	go m.collectSystemMetrics()

	m.logger.Info("Metrics server started successfully")
	return nil
}

// Stop stops the metrics server and background monitoring
func (m *MetricsManager) Stop() error {
	if !m.config.Enabled {
		return nil
	}

	m.logger.Info("Stopping metrics server")

	// Cancel context to stop background goroutines
	m.cancel()

	// Shutdown HTTP server
	if m.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := m.server.Shutdown(ctx); err != nil {
			m.logger.Error("Error shutting down metrics server", "error", err.Error())
		}
	}

	// Wait for all goroutines to finish
	m.wg.Wait()

	m.logger.Info("Metrics server stopped")
	return nil
}

// collectSystemMetrics collects system resource metrics periodically
func (m *MetricsManager) collectSystemMetrics() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.UpdateInterval)
	defer ticker.Stop()

	startTime := time.Now()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.updateSystemMetrics(startTime)
		}
	}
}

// updateSystemMetrics updates system resource metrics
func (m *MetricsManager) updateSystemMetrics(startTime time.Time) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Update memory metrics
	m.systemMetrics.MemoryUsage.Set(float64(memStats.Alloc))

	// Update goroutine count
	m.systemMetrics.GoroutineCount.Set(float64(runtime.NumGoroutine()))

	// Update uptime
	m.systemMetrics.Uptime.Set(time.Since(startTime).Seconds())

	// Update GC metrics
	m.systemMetrics.GCDuration.Observe(float64(memStats.PauseTotalNs) / 1e9)
}

// healthHandler handles health check requests
func (m *MetricsManager) healthHandler(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check if all components are healthy
	allHealthy := true
	for component, healthy := range m.healthStatus {
		if !healthy {
			allHealthy = false
			m.logger.Debug("Component unhealthy", "component", component)
		}
	}

	if allHealthy {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("UNHEALTHY"))
	}
}

// readyHandler handles readiness check requests
func (m *MetricsManager) readyHandler(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	ready := m.readyStatus
	m.mu.RUnlock()

	if ready {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("READY"))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("NOT READY"))
	}
}

// SetComponentHealth sets the health status for a component
func (m *MetricsManager) SetComponentHealth(component string, healthy bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.healthStatus[component] = healthy
	m.logger.Debug("Component health updated",
		"component", component,
		"healthy", healthy)
}

// SetReady sets the overall readiness status
func (m *MetricsManager) SetReady(ready bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.readyStatus = ready
	m.logger.Info("Readiness status updated", "ready", ready)
}

// GetTrapMetrics returns the trap metrics instance
func (m *MetricsManager) GetTrapMetrics() *TrapMetrics {
	return m.trapMetrics
}

// GetStorageMetrics returns the storage metrics instance
func (m *MetricsManager) GetStorageMetrics() *StorageMetrics {
	return m.storageMetrics
}

// GetWebhookMetrics returns the webhook metrics instance
func (m *MetricsManager) GetWebhookMetrics() *WebhookMetrics {
	return m.webhookMetrics
}

// GetSystemMetrics returns the system metrics instance
func (m *MetricsManager) GetSystemMetrics() *SystemMetrics {
	return m.systemMetrics
}

// GetCorrelatorMetrics returns the correlator metrics instance
func (m *MetricsManager) GetCorrelatorMetrics() *CorrelatorMetrics {
	return m.correlatorMetrics
}

// loadMetricsConfig loads metrics configuration from the config provider
func loadMetricsConfig(cfg config.Provider) (*MetricsConfig, error) {
	config := DefaultMetricsConfig()

	if enabled, err := cfg.GetBool("metrics.enabled"); err == nil {
		config.Enabled = enabled
	}

	if listenAddress, err := cfg.GetString("metrics.listen_address"); err == nil {
		config.ListenAddress = listenAddress
	}

	if metricsPath, err := cfg.GetString("metrics.metrics_path"); err == nil {
		config.MetricsPath = metricsPath
	}

	if healthPath, err := cfg.GetString("metrics.health_path"); err == nil {
		config.HealthPath = healthPath
	}

	if readyPath, err := cfg.GetString("metrics.ready_path"); err == nil {
		config.ReadyPath = readyPath
	}

	if updateInterval, err := cfg.GetDuration("metrics.update_interval"); err == nil {
		config.UpdateInterval = updateInterval
	}

	if namespace, err := cfg.GetString("metrics.namespace"); err == nil {
		config.Namespace = namespace
	}

	return config, nil
}
