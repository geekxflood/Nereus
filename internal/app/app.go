// Package app provides the main application orchestration and integration layer.
package app

import (
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/geekxflood/common/config"
	"github.com/geekxflood/common/logging"
	"github.com/geekxflood/nereus/internal/correlator"
	"github.com/geekxflood/nereus/internal/events"
	"github.com/geekxflood/nereus/internal/infra"
	"github.com/geekxflood/nereus/internal/listener"
	"github.com/geekxflood/nereus/internal/metrics"
	"github.com/geekxflood/nereus/internal/mib"
	"github.com/geekxflood/nereus/internal/notifier"
	"github.com/geekxflood/nereus/internal/storage"
	"github.com/geekxflood/nereus/internal/types"
)

// ListenerInterface defines the interface for SNMP listeners
type ListenerInterface interface {
	Start() error
	Stop() error
	IsRunning() bool
	GetStats() map[string]any
}

// AppConfig holds configuration for the main application
type AppConfig struct {
	Name            string        `json:"name"`
	Version         string        `json:"version"`
	LogLevel        string        `json:"log_level"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
	HealthCheck     HealthConfig  `json:"health_check"`
	Metrics         MetricsConfig `json:"metrics"`
}

// HealthConfig holds health check configuration
type HealthConfig struct {
	Enabled  bool          `json:"enabled"`
	Port     int           `json:"port"`
	Path     string        `json:"path"`
	Interval time.Duration `json:"interval"`
}

// MetricsConfig holds metrics configuration
type MetricsConfig struct {
	Enabled bool   `json:"enabled"`
	Port    int    `json:"port"`
	Path    string `json:"path"`
}

// DefaultAppConfig returns a default application configuration
func DefaultAppConfig() *AppConfig {
	return &AppConfig{
		Name:            "nereus",
		Version:         "1.0.0",
		LogLevel:        "info",
		ShutdownTimeout: 30 * time.Second,
		HealthCheck: HealthConfig{
			Enabled:  true,
			Port:     8080,
			Path:     "/health",
			Interval: 30 * time.Second,
		},
		Metrics: MetricsConfig{
			Enabled: true,
			Port:    8081,
			Path:    "/metrics",
		},
	}
}

// Application represents the main SNMP trap listener application
type Application struct {
	config         *AppConfig
	configProvider config.Provider

	// Core components
	mibManager     *mib.Manager
	listener       ListenerInterface
	storage        *storage.Storage
	correlator     *correlator.Correlator
	processor      *events.EventProcessor
	infraManager   *infra.Manager
	notifier       *notifier.Notifier
	metricsManager *metrics.MetricsManager

	// Logging
	logger        logging.Logger
	loggerCleanup io.Closer

	// Application state
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Statistics
	stats *AppStats
	mu    sync.RWMutex
}

// AppStats tracks application-wide statistics
type AppStats struct {
	StartTime           time.Time      `json:"start_time"`
	Uptime              time.Duration  `json:"uptime"`
	TotalTrapsReceived  int64          `json:"total_traps_received"`
	TotalTrapsProcessed int64          `json:"total_traps_processed"`
	TotalTrapsFailed    int64          `json:"total_traps_failed"`
	ComponentStats      map[string]any `json:"component_stats"`
	HealthStatus        string         `json:"health_status"`
	LastError           string         `json:"last_error,omitempty"`
	LastErrorTime       *time.Time     `json:"last_error_time,omitempty"`
}

// NewApplication creates a new SNMP trap listener application
func NewApplication(configManager config.Manager) (*Application, error) {
	if configManager == nil {
		return nil, fmt.Errorf("configuration manager cannot be nil")
	}

	// Cast to provider interface for component initialization
	configProvider := configManager.(config.Provider)

	// Load application configuration
	appConfig := DefaultAppConfig()

	if name, err := configProvider.GetString("app.name", appConfig.Name); err == nil {
		appConfig.Name = name
	}

	if version, err := configProvider.GetString("app.version", appConfig.Version); err == nil {
		appConfig.Version = version
	}

	if logLevel, err := configProvider.GetString("app.log_level", appConfig.LogLevel); err == nil {
		appConfig.LogLevel = logLevel
	}

	if shutdownTimeout, err := configProvider.GetDuration("app.shutdown_timeout", appConfig.ShutdownTimeout); err == nil {
		appConfig.ShutdownTimeout = shutdownTimeout
	}

	// Initialize logging
	logLevel, _ := configProvider.GetString("logging.level", "info")
	logFormat, _ := configProvider.GetString("logging.format", "json")

	// Fallback to app.log_level for backward compatibility
	if level, err := configProvider.GetString("app.log_level"); err == nil {
		logLevel = level
	}

	loggingConfig := logging.Config{
		Level:  logLevel,
		Format: logFormat,
	}

	logger, cleanup, err := logging.NewLogger(loggingConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	// Add application component context
	appLogger := logger.With("component", "app")

	// Initialize metrics manager
	metricsManager, err := metrics.NewMetricsManager(configProvider, logger)
	if err != nil {
		cleanup.Close()
		return nil, fmt.Errorf("failed to initialize metrics manager: %w", err)
	}

	// Initialize consolidated infrastructure manager
	infraManager, err := infra.NewManager(configProvider, appLogger)
	if err != nil {
		cleanup.Close()
		return nil, fmt.Errorf("failed to initialize infrastructure manager: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	app := &Application{
		config:         appConfig,
		configProvider: configProvider,
		logger:         appLogger,
		loggerCleanup:  cleanup,
		metricsManager: metricsManager,
		infraManager:   infraManager,
		ctx:            ctx,
		cancel:         cancel,
		stats: &AppStats{
			StartTime:      time.Now(),
			ComponentStats: make(map[string]any),
			HealthStatus:   "starting",
		},
	}

	logger.Info("Creating SNMP trap listener application",
		"name", appConfig.Name,
		"version", appConfig.Version)

	return app, nil
}

// Initialize initializes all application components
func (a *Application) Initialize() error {
	a.logger.Info("Initializing application components")

	// Start metrics manager first
	if err := a.metricsManager.Start(); err != nil {
		return fmt.Errorf("failed to start metrics manager: %w", err)
	}

	// Set initial component health
	a.metricsManager.SetComponentHealth("app", true)

	// Initialize consolidated MIB manager
	if err := a.initializeMIBManager(); err != nil {
		return fmt.Errorf("failed to initialize MIB manager: %w", err)
	}

	// Start infrastructure hot reload
	if err := a.infraManager.StartHotReload(nil, a.configProvider); err != nil {
		return fmt.Errorf("failed to start hot reload: %w", err)
	}

	// Initialize storage
	if err := a.initializeStorage(); err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Initialize correlator
	if err := a.initializeCorrelator(); err != nil {
		return fmt.Errorf("failed to initialize correlator: %w", err)
	}

	// HTTP client is now part of infrastructure manager

	// Initialize notifier
	if err := a.initializeNotifier(); err != nil {
		return fmt.Errorf("failed to initialize notifier: %w", err)
	}

	// Initialize event processor
	if err := a.initializeEventProcessor(); err != nil {
		return fmt.Errorf("failed to initialize event processor: %w", err)
	}

	// Initialize SNMP listener (last, as it starts accepting connections)
	if err := a.initializeListener(); err != nil {
		return fmt.Errorf("failed to initialize SNMP listener: %w", err)
	}

	a.stats.HealthStatus = "healthy"
	a.logger.Info("Application components initialized successfully")

	return nil
}

// Run starts the application and blocks until shutdown
func (a *Application) Run() error {
	a.logger.Info("Starting SNMP trap listener application")

	// Start the SNMP listener
	if err := a.listener.Start(); err != nil {
		return fmt.Errorf("failed to start SNMP listener: %w", err)
	}

	// Start background services
	a.wg.Add(1)
	go a.statsUpdater()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	a.logger.Info("Application started successfully. Listening for SNMP traps...")

	// Wait for shutdown signal
	select {
	case sig := <-sigChan:
		a.logger.Info("Received shutdown signal", "signal", sig.String())
		return a.Shutdown()
	case <-a.ctx.Done():
		a.logger.Info("Application context cancelled")
		return a.ctx.Err()
	}
}

// Shutdown gracefully shuts down the application
func (a *Application) Shutdown() error {
	a.logger.Info("Shutting down application")
	a.stats.HealthStatus = "shutting_down"

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), a.config.ShutdownTimeout)
	defer shutdownCancel()

	// Cancel main context
	a.cancel()

	// Shutdown components in reverse order
	var shutdownErrors []error

	if a.listener != nil {
		a.logger.Info("Shutting down SNMP listener")
		if err := a.listener.Stop(); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("listener shutdown error: %w", err))
		}
	}

	if a.processor != nil {
		a.logger.Info("Shutting down event processor")
		if err := a.processor.Close(); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("processor shutdown error: %w", err))
		}
	}

	if a.notifier != nil {
		a.logger.Info("Shutting down notifier")
		if err := a.notifier.Close(); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("notifier shutdown error: %w", err))
		}
	}

	if a.infraManager != nil {
		a.logger.Info("Shutting down infrastructure manager")
		if err := a.infraManager.Close(); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("infrastructure manager shutdown error: %w", err))
		}
	}

	if a.storage != nil {
		a.logger.Info("Shutting down storage")
		if err := a.storage.Close(); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("storage shutdown error: %w", err))
		}
	}

	if a.metricsManager != nil {
		a.logger.Info("Shutting down metrics manager")
		if err := a.metricsManager.Stop(); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("metrics manager shutdown error: %w", err))
		}
	}

	if a.loggerCleanup != nil {
		a.logger.Info("Shutting down logger")
		if err := a.loggerCleanup.Close(); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("logger cleanup error: %w", err))
		}
	}

	// Wait for background goroutines with timeout
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		a.logger.Info("All background goroutines stopped")
	case <-shutdownCtx.Done():
		a.logger.Warn("Shutdown timeout reached, forcing exit")
		shutdownErrors = append(shutdownErrors, fmt.Errorf("shutdown timeout"))
	}

	a.stats.HealthStatus = "stopped"

	if len(shutdownErrors) > 0 {
		a.logger.Error("Shutdown completed with errors", "error_count", len(shutdownErrors))
		return fmt.Errorf("shutdown errors: %v", shutdownErrors)
	}

	a.logger.Info("Application shutdown completed successfully")
	return nil
}

// initializeMIBManager initializes the consolidated MIB manager component
func (a *Application) initializeMIBManager() error {
	a.logger.Info("Initializing consolidated MIB manager")

	manager, err := mib.NewManager(a.configProvider)
	if err != nil {
		return fmt.Errorf("failed to create MIB manager: %w", err)
	}

	// Load and parse all MIB files
	if err := manager.LoadAll(); err != nil {
		return fmt.Errorf("failed to load and parse MIB files: %w", err)
	}

	// Start hot reload if enabled
	if err := manager.StartHotReload(); err != nil {
		return fmt.Errorf("failed to start hot reload: %w", err)
	}

	a.mibManager = manager
	return nil
}

// initializeStorage initializes the storage component
func (a *Application) initializeStorage() error {
	a.logger.Info("Initializing storage")

	storage, err := storage.NewStorage(a.configProvider)
	if err != nil {
		return err
	}

	a.storage = storage
	return nil
}

// initializeCorrelator initializes the correlator component
func (a *Application) initializeCorrelator() error {
	a.logger.Info("Initializing correlator")

	correlator, err := correlator.NewCorrelator(a.configProvider, a.storage)
	if err != nil {
		return err
	}

	a.correlator = correlator
	return nil
}

// HTTP client is now part of the consolidated infrastructure manager

// initializeNotifier initializes the notifier component
func (a *Application) initializeNotifier() error {
	a.logger.Info("Initializing notifier")

	notifier, err := notifier.NewNotifier(a.configProvider, a.infraManager)
	if err != nil {
		return err
	}

	a.notifier = notifier
	return nil
}

// initializeEventProcessor initializes the event processor component
func (a *Application) initializeEventProcessor() error {
	a.logger.Info("Initializing event processor")

	processor, err := events.NewEventProcessor(a.configProvider, a.mibManager, a.correlator, a.storage)
	if err != nil {
		return err
	}

	a.processor = processor
	return nil
}

// initializeListener initializes the SNMP listener component
func (a *Application) initializeListener() error {
	a.logger.Info("Initializing SNMP listener")

	listener, err := listener.NewListener(a.configProvider)
	if err != nil {
		return err
	}

	// Create a custom listener wrapper that integrates with our event processing
	wrappedListener := &IntegratedListener{
		Listener:    listener,
		application: a,
	}

	a.listener = wrappedListener
	return nil
}

// IntegratedListener wraps the base listener to integrate with event processing
type IntegratedListener struct {
	*listener.Listener
	application *Application
}

// Start starts the integrated listener with trap processing
func (il *IntegratedListener) Start() error {
	// Start the base listener with our context
	if err := il.Listener.Start(il.application.ctx); err != nil {
		return err
	}

	// Start our trap processing integration
	il.application.wg.Add(1)
	go il.trapProcessor()

	return nil
}

// Stop stops the integrated listener
func (il *IntegratedListener) Stop() error {
	return il.Listener.Stop()
}

// trapProcessor processes traps from the listener and forwards them to event processing
func (il *IntegratedListener) trapProcessor() {
	defer il.application.wg.Done()

	// This is a simplified integration approach
	// In a real implementation, we would modify the listener to expose a channel
	// or callback mechanism for processed traps

	// For now, we'll use a ticker to check for processed traps
	// This is not ideal but works for the integration demo
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-il.application.ctx.Done():
			return
		case <-ticker.C:
			// In a real implementation, we would get actual trap data here
			// For now, this is just a placeholder to show the integration pattern
		}
	}
}

// handleTrap handles incoming SNMP trap packets
// TODO: This method is currently unused but will be integrated with the listener
// when the listener is updated to support callback-based trap processing
func (a *Application) handleTrap(packet *types.SNMPPacket, sourceIP string) {
	startTime := time.Now()

	// Update metrics
	trapMetrics := a.metricsManager.GetTrapMetrics()
	trapMetrics.TrapsReceived.Inc()
	trapMetrics.TrapsPerSource.WithLabelValues(sourceIP).Inc()

	a.mu.Lock()
	a.stats.TotalTrapsReceived++
	a.mu.Unlock()

	a.logger.Debug("Received SNMP trap",
		"source_ip", sourceIP,
		"community", packet.Community,
		"version", packet.Version,
		"pdu_type", packet.PDUType)

	// Process the trap through the event processor
	result, err := a.processor.ProcessEventSync(packet, sourceIP)
	processingTime := time.Since(startTime)

	if err != nil {
		// Update metrics
		trapMetrics.TrapsFailed.Inc()
		trapMetrics.ProcessingTime.Observe(processingTime.Seconds())

		a.mu.Lock()
		a.stats.TotalTrapsFailed++
		a.stats.LastError = err.Error()
		now := time.Now()
		a.stats.LastErrorTime = &now
		a.mu.Unlock()

		a.logger.Error("Failed to process trap",
			"operation", "process_trap",
			"error", err.Error(),
			"source_ip", sourceIP,
			"processing_time", processingTime,
			"community", packet.Community,
			"version", packet.Version,
			"pdu_type", packet.PDUType)
		return
	}

	if !result.Success {
		a.mu.Lock()
		a.stats.TotalTrapsFailed++
		if result.Error != nil {
			a.stats.LastError = result.Error.Error()
			now := time.Now()
			a.stats.LastErrorTime = &now
		}
		a.mu.Unlock()

		a.logger.Error("Trap processing failed",
			"operation", "process_trap_result",
			"error", result.Error.Error(),
			"source_ip", sourceIP,
			"processing_time", processingTime)
		return
	}

	// Update metrics for successful processing
	trapMetrics.TrapsProcessed.Inc()
	trapMetrics.ProcessingTime.Observe(processingTime.Seconds())

	// Add trap type metrics using PDU type for now
	// TODO: Extract actual trap OID from varbinds when trap parsing is implemented
	pduTypeStr := fmt.Sprintf("pdu_%d", packet.PDUType)
	trapMetrics.TrapsByType.WithLabelValues(pduTypeStr, "snmp_trap").Inc()

	a.mu.Lock()
	a.stats.TotalTrapsProcessed++
	a.mu.Unlock()

	// Log performance metrics
	a.logger.Debug("Performance metric",
		"operation", "process_trap",
		"duration", processingTime,
		"source_ip", sourceIP,
		"event_id", result.EventID)

	a.logger.Debug("Trap processed successfully",
		"source_ip", sourceIP,
		"event_id", result.EventID,
		"processing_time", processingTime)

	// Send notifications if configured
	if a.notifier != nil && result.EventID > 0 {
		// Get the stored event for notification
		if event, err := a.storage.GetEvent(result.EventID); err == nil {
			if err := a.notifier.SendNotification(event); err != nil {
				a.logger.Warn("Failed to send notification",
					"event_id", result.EventID,
					"error", err.Error())
			}
		}
	}
}

// statsUpdater periodically updates application statistics
func (a *Application) statsUpdater() {
	defer a.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.updateStats()
		}
	}
}

// updateStats updates application statistics
func (a *Application) updateStats() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.stats.Uptime = time.Since(a.stats.StartTime)

	// Collect component statistics
	if a.listener != nil {
		a.stats.ComponentStats["listener"] = a.listener.GetStats()
	}

	if a.processor != nil {
		a.stats.ComponentStats["processor"] = a.processor.GetStats()
	}

	if a.storage != nil {
		if storageStats, err := a.storage.GetStats(); err == nil {
			a.stats.ComponentStats["storage"] = storageStats
		}
	}

	if a.correlator != nil {
		a.stats.ComponentStats["correlator"] = a.correlator.GetStats()
	}

	if a.infraManager != nil {
		a.stats.ComponentStats["infrastructure"] = a.infraManager.GetStats()
	}

	if a.notifier != nil {
		a.stats.ComponentStats["notifier"] = a.notifier.GetStats()
	}

	if a.mibManager != nil {
		a.stats.ComponentStats["mib_manager"] = a.mibManager.GetStats()
	}
}

// GetStats returns application statistics
func (a *Application) GetStats() *AppStats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Update uptime
	stats := *a.stats
	stats.Uptime = time.Since(a.stats.StartTime)

	// Deep copy component stats
	stats.ComponentStats = make(map[string]any)
	maps.Copy(stats.ComponentStats, a.stats.ComponentStats)

	return &stats
}

// GetConfig returns the application configuration
func (a *Application) GetConfig() *AppConfig {
	return a.config
}

// GetLogger returns the application logger
func (a *Application) GetLogger() logging.Logger {
	return a.logger
}

// IsHealthy returns whether the application is healthy
func (a *Application) IsHealthy() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.stats.HealthStatus == "healthy"
}

// GetComponentLogger returns a logger for a specific component
func (a *Application) GetComponentLogger(component string) logging.Logger {
	return a.logger.With("component", component)
}

// GetMetricsManager returns the metrics manager
func (a *Application) GetMetricsManager() *metrics.MetricsManager {
	return a.metricsManager
}

// GetInfraManager returns the infrastructure manager
func (a *Application) GetInfraManager() *infra.Manager {
	return a.infraManager
}
