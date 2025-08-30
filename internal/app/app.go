// Package app provides the main application orchestration and integration layer.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/geekxflood/common/config"
	"github.com/geekxflood/nereus/internal/client"
	"github.com/geekxflood/nereus/internal/correlator"
	"github.com/geekxflood/nereus/internal/events"
	"github.com/geekxflood/nereus/internal/listener"
	"github.com/geekxflood/nereus/internal/loader"
	"github.com/geekxflood/nereus/internal/mib"
	"github.com/geekxflood/nereus/internal/notifier"
	"github.com/geekxflood/nereus/internal/resolver"
	"github.com/geekxflood/nereus/internal/storage"
	"github.com/geekxflood/nereus/internal/types"
)

// ListenerInterface defines the interface for SNMP listeners
type ListenerInterface interface {
	Start() error
	Stop() error
	IsRunning() bool
	GetStats() map[string]interface{}
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
	mibLoader  *loader.Loader
	mibParser  *mib.Parser
	resolver   *resolver.Resolver
	listener   ListenerInterface
	storage    *storage.Storage
	correlator *correlator.Correlator
	processor  *events.EventProcessor
	httpClient *client.HTTPClient
	notifier   *notifier.Notifier

	// Application state
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	logger *slog.Logger

	// Statistics
	stats *AppStats
	mu    sync.RWMutex
}

// AppStats tracks application-wide statistics
type AppStats struct {
	StartTime           time.Time              `json:"start_time"`
	Uptime              time.Duration          `json:"uptime"`
	TotalTrapsReceived  int64                  `json:"total_traps_received"`
	TotalTrapsProcessed int64                  `json:"total_traps_processed"`
	TotalTrapsFailed    int64                  `json:"total_traps_failed"`
	ComponentStats      map[string]interface{} `json:"component_stats"`
	HealthStatus        string                 `json:"health_status"`
	LastError           string                 `json:"last_error,omitempty"`
	LastErrorTime       *time.Time             `json:"last_error_time,omitempty"`
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

	// Setup logger
	logger := setupLogger(appConfig.LogLevel)

	ctx, cancel := context.WithCancel(context.Background())

	app := &Application{
		config:         appConfig,
		configProvider: configProvider,
		ctx:            ctx,
		cancel:         cancel,
		logger:         logger,
		stats: &AppStats{
			StartTime:      time.Now(),
			ComponentStats: make(map[string]interface{}),
			HealthStatus:   "starting",
		},
	}

	logger.Info("Creating SNMP trap listener application",
		slog.String("name", appConfig.Name),
		slog.String("version", appConfig.Version))

	return app, nil
}

// Initialize initializes all application components
func (a *Application) Initialize() error {
	a.logger.Info("Initializing application components")

	// Initialize MIB loader
	if err := a.initializeMIBLoader(); err != nil {
		return fmt.Errorf("failed to initialize MIB loader: %w", err)
	}

	// Initialize MIB parser
	if err := a.initializeMIBParser(); err != nil {
		return fmt.Errorf("failed to initialize MIB parser: %w", err)
	}

	// Initialize OID resolver
	if err := a.initializeResolver(); err != nil {
		return fmt.Errorf("failed to initialize OID resolver: %w", err)
	}

	// Initialize storage
	if err := a.initializeStorage(); err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Initialize correlator
	if err := a.initializeCorrelator(); err != nil {
		return fmt.Errorf("failed to initialize correlator: %w", err)
	}

	// Initialize HTTP client
	if err := a.initializeHTTPClient(); err != nil {
		return fmt.Errorf("failed to initialize HTTP client: %w", err)
	}

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
		a.logger.Info("Received shutdown signal", slog.String("signal", sig.String()))
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

	if a.httpClient != nil {
		a.logger.Info("Shutting down HTTP client")
		if err := a.httpClient.Close(); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("HTTP client shutdown error: %w", err))
		}
	}

	if a.storage != nil {
		a.logger.Info("Shutting down storage")
		if err := a.storage.Close(); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("storage shutdown error: %w", err))
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
		a.logger.Error("Shutdown completed with errors", slog.Int("error_count", len(shutdownErrors)))
		return fmt.Errorf("shutdown errors: %v", shutdownErrors)
	}

	a.logger.Info("Application shutdown completed successfully")
	return nil
}

// initializeMIBLoader initializes the MIB loader component
func (a *Application) initializeMIBLoader() error {
	a.logger.Info("Initializing MIB loader")

	loader, err := loader.NewLoader(a.configProvider)
	if err != nil {
		return err
	}

	// Load all MIB files
	if err := loader.LoadAll(); err != nil {
		return fmt.Errorf("failed to load MIB files: %w", err)
	}

	a.mibLoader = loader
	return nil
}

// initializeMIBParser initializes the MIB parser component
func (a *Application) initializeMIBParser() error {
	a.logger.Info("Initializing MIB parser")

	parser := mib.NewParser(a.mibLoader)

	// Parse all loaded MIB files
	if err := parser.ParseAll(); err != nil {
		return fmt.Errorf("failed to parse MIB files: %w", err)
	}

	a.mibParser = parser
	return nil
}

// initializeResolver initializes the OID resolver component
func (a *Application) initializeResolver() error {
	a.logger.Info("Initializing OID resolver")

	resolver, err := resolver.NewResolver(a.configProvider, a.mibParser)
	if err != nil {
		return err
	}

	a.resolver = resolver
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

// initializeHTTPClient initializes the HTTP client component
func (a *Application) initializeHTTPClient() error {
	a.logger.Info("Initializing HTTP client")

	httpClient, err := client.NewHTTPClient(a.configProvider)
	if err != nil {
		return err
	}

	a.httpClient = httpClient
	return nil
}

// initializeNotifier initializes the notifier component
func (a *Application) initializeNotifier() error {
	a.logger.Info("Initializing notifier")

	notifier, err := notifier.NewNotifier(a.configProvider, a.httpClient)
	if err != nil {
		return err
	}

	a.notifier = notifier
	return nil
}

// initializeEventProcessor initializes the event processor component
func (a *Application) initializeEventProcessor() error {
	a.logger.Info("Initializing event processor")

	processor, err := events.NewEventProcessor(a.configProvider, a.resolver, a.correlator, a.storage)
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
func (a *Application) handleTrap(packet *types.SNMPPacket, sourceIP string) {
	a.mu.Lock()
	a.stats.TotalTrapsReceived++
	a.mu.Unlock()

	a.logger.Debug("Received SNMP trap",
		slog.String("source_ip", sourceIP),
		slog.String("community", packet.Community),
		slog.Int("version", packet.Version),
		slog.Int("pdu_type", packet.PDUType))

	// Process the trap through the event processor
	result, err := a.processor.ProcessEventSync(packet, sourceIP)
	if err != nil {
		a.mu.Lock()
		a.stats.TotalTrapsFailed++
		a.stats.LastError = err.Error()
		now := time.Now()
		a.stats.LastErrorTime = &now
		a.mu.Unlock()

		a.logger.Error("Failed to process trap",
			slog.String("source_ip", sourceIP),
			slog.String("error", err.Error()))
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
			slog.String("source_ip", sourceIP),
			slog.String("error", result.Error.Error()))
		return
	}

	a.mu.Lock()
	a.stats.TotalTrapsProcessed++
	a.mu.Unlock()

	a.logger.Debug("Trap processed successfully",
		slog.String("source_ip", sourceIP),
		slog.Int64("event_id", result.EventID),
		slog.Duration("processing_time", result.ProcessingTime))

	// Send notifications if configured
	if a.notifier != nil && result.EventID > 0 {
		// Get the stored event for notification
		if event, err := a.storage.GetEvent(result.EventID); err == nil {
			if err := a.notifier.SendNotification(event); err != nil {
				a.logger.Warn("Failed to send notification",
					slog.Int64("event_id", result.EventID),
					slog.String("error", err.Error()))
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

	if a.httpClient != nil {
		a.stats.ComponentStats["http_client"] = a.httpClient.GetStats()
	}

	if a.notifier != nil {
		a.stats.ComponentStats["notifier"] = a.notifier.GetStats()
	}

	if a.resolver != nil {
		a.stats.ComponentStats["resolver"] = a.resolver.GetStats()
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
	stats.ComponentStats = make(map[string]interface{})
	for key, value := range a.stats.ComponentStats {
		stats.ComponentStats[key] = value
	}

	return &stats
}

// GetConfig returns the application configuration
func (a *Application) GetConfig() *AppConfig {
	return a.config
}

// GetLogger returns the application logger
func (a *Application) GetLogger() *slog.Logger {
	return a.logger
}

// IsHealthy returns whether the application is healthy
func (a *Application) IsHealthy() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.stats.HealthStatus == "healthy"
}

// setupLogger configures the application logger
func setupLogger(level string) *slog.Logger {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     logLevel,
		AddSource: level == "debug",
	}

	handler := slog.NewTextHandler(os.Stdout, opts)
	return slog.New(handler)
}
