// Package events provides SNMP trap event processing, enrichment, and orchestration functionality.
// It coordinates between MIB resolution, event correlation, and storage to create a complete
// event processing pipeline with worker-based concurrent processing.
package events

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/geekxflood/common/config"
	"github.com/geekxflood/nereus/internal/correlator"
	"github.com/geekxflood/nereus/internal/mib"
	"github.com/geekxflood/nereus/internal/storage"
	"github.com/geekxflood/nereus/internal/types"
)

// OIDResolver defines the interface for OID resolution services
type OIDResolver interface {
	ResolveOID(oid string) (*mib.OIDInfo, error)
}

// ProcessorConfig holds configuration for the event processor
type ProcessorConfig struct {
	EnableEnrichment    bool          `json:"enable_enrichment"`
	EnableCorrelation   bool          `json:"enable_correlation"`
	EnableStorage       bool          `json:"enable_storage"`
	ProcessingTimeout   time.Duration `json:"processing_timeout"`
	MaxConcurrentEvents int           `json:"max_concurrent_events"`
	QueueSize           int           `json:"queue_size"`
	RetryAttempts       int           `json:"retry_attempts"`
	RetryDelay          time.Duration `json:"retry_delay"`
	EnableMetrics       bool          `json:"enable_metrics"`
}

// DefaultProcessorConfig returns a default processor configuration
func DefaultProcessorConfig() *ProcessorConfig {
	return &ProcessorConfig{
		EnableEnrichment:    true,
		EnableCorrelation:   true,
		EnableStorage:       true,
		ProcessingTimeout:   30 * time.Second,
		MaxConcurrentEvents: 100,
		QueueSize:           1000,
		RetryAttempts:       3,
		RetryDelay:          1 * time.Second,
		EnableMetrics:       true,
	}
}

// EventProcessor processes SNMP trap events through enrichment, correlation, and storage
type EventProcessor struct {
	config     *ProcessorConfig
	resolver   OIDResolver
	correlator *correlator.Correlator
	storage    *storage.Storage
	eventQueue chan *EventTask
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	stats      *ProcessorStats
	mu         sync.RWMutex
}

// EventTask represents a task for processing an event
type EventTask struct {
	Packet     *types.SNMPPacket
	SourceIP   string
	ReceivedAt time.Time
	Attempts   int
	ResultChan chan *EventResult
}

// EventResult represents the result of event processing
type EventResult struct {
	Success        bool           `json:"success"`
	Error          error          `json:"error,omitempty"`
	EventID        int64          `json:"event_id,omitempty"`
	EnrichedData   map[string]any `json:"enriched_data,omitempty"`
	ProcessingTime time.Duration  `json:"processing_time"`
}

// ProcessorStats tracks event processor statistics
type ProcessorStats struct {
	EventsReceived     int64         `json:"events_received"`
	EventsProcessed    int64         `json:"events_processed"`
	EventsFailed       int64         `json:"events_failed"`
	EventsEnriched     int64         `json:"events_enriched"`
	EventsCorrelated   int64         `json:"events_correlated"`
	EventsStored       int64         `json:"events_stored"`
	QueueLength        int           `json:"queue_length"`
	QueueCapacity      int           `json:"queue_capacity"`
	ActiveWorkers      int           `json:"active_workers"`
	AverageProcessTime time.Duration `json:"average_process_time"`
	TotalProcessTime   time.Duration `json:"total_process_time"`
	RetryCount         int64         `json:"retry_count"`
	TimeoutCount       int64         `json:"timeout_count"`
}

// NewEventProcessor creates a new event processor
func NewEventProcessor(cfg config.Provider, resolver OIDResolver, correlator *correlator.Correlator, storage *storage.Storage) (*EventProcessor, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration provider cannot be nil")
	}

	// Load configuration
	processorConfig := DefaultProcessorConfig()

	if enrichment, err := cfg.GetBool("processor.enable_enrichment", processorConfig.EnableEnrichment); err == nil {
		processorConfig.EnableEnrichment = enrichment
	}

	if correlation, err := cfg.GetBool("processor.enable_correlation", processorConfig.EnableCorrelation); err == nil {
		processorConfig.EnableCorrelation = correlation
	}

	if storageEnabled, err := cfg.GetBool("processor.enable_storage", processorConfig.EnableStorage); err == nil {
		processorConfig.EnableStorage = storageEnabled
	}

	if timeout, err := cfg.GetDuration("processor.processing_timeout", processorConfig.ProcessingTimeout); err == nil {
		processorConfig.ProcessingTimeout = timeout
	}

	if maxConcurrent, err := cfg.GetInt("processor.max_concurrent_events", processorConfig.MaxConcurrentEvents); err == nil {
		processorConfig.MaxConcurrentEvents = maxConcurrent
	}

	if queueSize, err := cfg.GetInt("processor.queue_size", processorConfig.QueueSize); err == nil {
		processorConfig.QueueSize = queueSize
	}

	ctx, cancel := context.WithCancel(context.Background())

	processor := &EventProcessor{
		config:     processorConfig,
		resolver:   resolver,
		correlator: correlator,
		storage:    storage,
		eventQueue: make(chan *EventTask, processorConfig.QueueSize),
		ctx:        ctx,
		cancel:     cancel,
		stats:      &ProcessorStats{},
	}

	// Start worker goroutines
	for i := 0; i < processorConfig.MaxConcurrentEvents; i++ {
		processor.wg.Add(1)
		go processor.worker(i)
	}

	return processor, nil
}

// ProcessEvent processes an SNMP trap event asynchronously
func (p *EventProcessor) ProcessEvent(packet *types.SNMPPacket, sourceIP string) (*EventResult, error) {
	p.mu.Lock()
	p.stats.EventsReceived++
	p.mu.Unlock()

	task := &EventTask{
		Packet:     packet,
		SourceIP:   sourceIP,
		ReceivedAt: time.Now(),
		Attempts:   0,
		ResultChan: make(chan *EventResult, 1),
	}

	// Try to queue the task
	select {
	case p.eventQueue <- task:
		// Successfully queued
	default:
		// Queue is full
		return &EventResult{
			Success: false,
			Error:   fmt.Errorf("event queue is full"),
		}, nil
	}

	// Wait for result with timeout
	select {
	case result := <-task.ResultChan:
		return result, nil
	case <-time.After(p.config.ProcessingTimeout):
		p.mu.Lock()
		p.stats.TimeoutCount++
		p.mu.Unlock()
		return &EventResult{
			Success: false,
			Error:   fmt.Errorf("processing timeout"),
		}, nil
	}
}

// ProcessEventSync processes an SNMP trap event synchronously
func (p *EventProcessor) ProcessEventSync(packet *types.SNMPPacket, sourceIP string) (*EventResult, error) {
	p.mu.Lock()
	p.stats.EventsReceived++
	p.mu.Unlock()

	startTime := time.Now()
	result := p.processEventInternal(packet, sourceIP)
	result.ProcessingTime = time.Since(startTime)

	p.mu.Lock()
	p.stats.TotalProcessTime += result.ProcessingTime
	if result.Success {
		p.stats.EventsProcessed++
	} else {
		p.stats.EventsFailed++
	}
	p.mu.Unlock()

	return result, nil
}

// worker processes events from the queue
func (p *EventProcessor) worker(_ int) {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return
		case task := <-p.eventQueue:
			if task == nil {
				return
			}

			p.mu.Lock()
			p.stats.ActiveWorkers++
			p.mu.Unlock()

			startTime := time.Now()
			result := p.processEventInternal(task.Packet, task.SourceIP)
			result.ProcessingTime = time.Since(startTime)

			p.mu.Lock()
			p.stats.ActiveWorkers--
			p.stats.TotalProcessTime += result.ProcessingTime
			if result.Success {
				p.stats.EventsProcessed++
			} else {
				p.stats.EventsFailed++

				// Retry logic
				if task.Attempts < p.config.RetryAttempts {
					task.Attempts++
					p.stats.RetryCount++

					// Retry after delay
					go func() {
						time.Sleep(p.config.RetryDelay)
						select {
						case p.eventQueue <- task:
							// Successfully requeued
						default:
							// Queue full, send failure result
							task.ResultChan <- &EventResult{
								Success: false,
								Error:   fmt.Errorf("retry failed: queue full"),
							}
						}
					}()
					p.mu.Unlock()
					continue
				}
			}
			p.mu.Unlock()

			// Send result
			select {
			case task.ResultChan <- result:
				// Result sent
			default:
				// Channel closed or full
			}
		}
	}
}

// processEventInternal performs the actual event processing
func (p *EventProcessor) processEventInternal(packet *types.SNMPPacket, sourceIP string) *EventResult {
	enrichedData := make(map[string]any)

	// Add basic metadata
	enrichedData["source_ip"] = sourceIP
	enrichedData["received_at"] = time.Now()
	enrichedData["version"] = packet.Version
	enrichedData["community"] = packet.Community
	enrichedData["pdu_type"] = packet.PDUType
	enrichedData["request_id"] = packet.RequestID

	// Step 1: Enrichment
	if p.config.EnableEnrichment && p.resolver != nil {
		// Debug: Log enrichment start
		fmt.Printf("DEBUG: Starting enrichment step\n")
		if err := p.enrichEvent(packet, enrichedData); err != nil {
			fmt.Printf("DEBUG: Enrichment failed: %v\n", err)
			return &EventResult{
				Success: false,
				Error:   fmt.Errorf("enrichment failed: %w", err),
			}
		}
		fmt.Printf("DEBUG: Enrichment completed successfully\n")
		p.mu.Lock()
		p.stats.EventsEnriched++
		p.mu.Unlock()
	} else {
		fmt.Printf("DEBUG: Enrichment skipped (enabled=%v, resolver=%v)\n", p.config.EnableEnrichment, p.resolver != nil)
	}

	// Step 2: Correlation
	if p.config.EnableCorrelation && p.correlator != nil {
		fmt.Printf("DEBUG: Starting correlation step\n")
		correlatedData, err := p.correlator.ProcessEvent(packet, sourceIP, enrichedData)
		if err != nil {
			fmt.Printf("DEBUG: Correlation failed: %v\n", err)
			return &EventResult{
				Success: false,
				Error:   fmt.Errorf("correlation failed: %w", err),
			}
		}
		fmt.Printf("DEBUG: Correlation completed successfully\n")
		enrichedData = correlatedData
		p.mu.Lock()
		p.stats.EventsCorrelated++
		p.mu.Unlock()
	} else {
		fmt.Printf("DEBUG: Correlation skipped (enabled=%v, correlator=%v)\n", p.config.EnableCorrelation, p.correlator != nil)
	}

	// Step 3: Storage
	var eventID int64
	if p.config.EnableStorage && p.storage != nil {
		fmt.Printf("DEBUG: Starting storage step\n")
		// Use immediate storage to get event ID for testing
		id, err := p.storage.StoreEventImmediate(packet, sourceIP, enrichedData)
		if err != nil {
			fmt.Printf("DEBUG: Storage failed: %v\n", err)
			return &EventResult{
				Success: false,
				Error:   fmt.Errorf("storage failed: %w", err),
			}
		}
		fmt.Printf("DEBUG: Storage completed successfully, event_id=%d\n", id)
		eventID = id
		p.mu.Lock()
		p.stats.EventsStored++
		p.mu.Unlock()
	} else {
		fmt.Printf("DEBUG: Storage skipped (enabled=%v, storage=%v)\n", p.config.EnableStorage, p.storage != nil)
	}

	fmt.Printf("DEBUG: Event processing completed successfully\n")

	return &EventResult{
		Success:      true,
		EventID:      eventID,
		EnrichedData: enrichedData,
	}
}

// enrichEvent enriches an event with OID resolution and metadata
func (p *EventProcessor) enrichEvent(packet *types.SNMPPacket, enrichedData map[string]any) error {
	// Enrich varbinds with OID names and descriptions
	enrichedVarbinds := make([]map[string]any, len(packet.Varbinds))

	for i, vb := range packet.Varbinds {
		varbindData := map[string]any{
			"oid":   vb.OID,
			"type":  vb.Type,
			"value": vb.Value,
		}

		// Resolve OID to name and description with timeout protection
		fmt.Printf("DEBUG: Resolving OID: %s\n", vb.OID)
		if p.resolver != nil {
			// Use a channel to implement timeout for OID resolution
			done := make(chan bool, 1)
			var oidInfo *mib.OIDInfo
			var err error

			go func() {
				oidInfo, err = p.resolver.ResolveOID(vb.OID)
				done <- true
			}()

			// Wait for resolution with timeout
			select {
			case <-done:
				if err == nil {
					fmt.Printf("DEBUG: OID resolved successfully: %s -> %s\n", vb.OID, oidInfo.Name)
					varbindData["name"] = oidInfo.Name
					varbindData["description"] = oidInfo.Description
					varbindData["syntax"] = oidInfo.Syntax
					varbindData["access"] = oidInfo.Access
					varbindData["mib_name"] = oidInfo.MIBName
				} else {
					fmt.Printf("DEBUG: OID resolution failed: %s -> %v\n", vb.OID, err)
				}
			case <-time.After(2 * time.Second):
				fmt.Printf("DEBUG: OID resolution timeout: %s\n", vb.OID)
			}
		}

		enrichedVarbinds[i] = varbindData
	}

	enrichedData["varbinds_enriched"] = enrichedVarbinds

	// Extract and enrich trap OID (typically second varbind in SNMPv2c)
	if len(packet.Varbinds) > 1 && packet.Varbinds[1].OID == "1.3.6.1.6.3.1.1.4.1.0" {
		if trapOID, ok := packet.Varbinds[1].Value.(string); ok {
			enrichedData["trap_oid"] = trapOID

			// Resolve trap OID with timeout protection
			fmt.Printf("DEBUG: Resolving trap OID: %s\n", trapOID)
			if p.resolver != nil {
				done := make(chan bool, 1)
				var oidInfo *mib.OIDInfo
				var err error

				go func() {
					oidInfo, err = p.resolver.ResolveOID(trapOID)
					done <- true
				}()

				select {
				case <-done:
					if err == nil {
						fmt.Printf("DEBUG: Trap OID resolved successfully: %s -> %s\n", trapOID, oidInfo.Name)
						enrichedData["trap_name"] = oidInfo.Name
						enrichedData["trap_description"] = oidInfo.Description
						enrichedData["trap_mib"] = oidInfo.MIBName
					} else {
						fmt.Printf("DEBUG: Trap OID resolution failed: %s -> %v\n", trapOID, err)
					}
				case <-time.After(500 * time.Millisecond):
					fmt.Printf("DEBUG: Trap OID resolution timeout: %s\n", trapOID)
				}
			}
		}
	}

	// Extract system uptime (typically first varbind in SNMPv2c)
	if len(packet.Varbinds) > 0 && packet.Varbinds[0].OID == "1.3.6.1.2.1.1.3.0" {
		if uptime, ok := packet.Varbinds[0].Value.(uint32); ok {
			enrichedData["system_uptime"] = uptime
			enrichedData["system_uptime_formatted"] = formatUptime(uptime)
		}
	}

	return nil
}

// formatUptime formats uptime ticks to human-readable format
func formatUptime(ticks uint32) string {
	// SNMP uptime is in hundredths of a second
	seconds := ticks / 100
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, secs)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, secs)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, secs)
	} else {
		return fmt.Sprintf("%ds", secs)
	}
}

// GetStats returns processor statistics
func (p *EventProcessor) GetStats() *ProcessorStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := *p.stats
	stats.QueueLength = len(p.eventQueue)
	stats.QueueCapacity = cap(p.eventQueue)

	// Calculate average processing time
	if stats.EventsProcessed > 0 {
		stats.AverageProcessTime = stats.TotalProcessTime / time.Duration(stats.EventsProcessed)
	}

	return &stats
}

// GetConfig returns the processor configuration
func (p *EventProcessor) GetConfig() *ProcessorConfig {
	return p.config
}

// UpdateConfig updates the processor configuration
func (p *EventProcessor) UpdateConfig(config *ProcessorConfig) {
	p.config = config
}

// Close shuts down the event processor
func (p *EventProcessor) Close() error {
	p.cancel()
	close(p.eventQueue)
	p.wg.Wait()
	return nil
}

// ResetStats resets processor statistics
func (p *EventProcessor) ResetStats() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stats = &ProcessorStats{}
}
