// Package listener provides SNMP trap listening functionality.
package listener

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/geekxflood/common/config"
	"github.com/geekxflood/nereus/internal/parser"
	"github.com/geekxflood/nereus/internal/types"
	"github.com/geekxflood/nereus/internal/validator"
)

// Listener represents an SNMP trap listener that receives and processes SNMP traps.
type Listener struct {
	config    config.Provider
	conn      *net.UDPConn
	handlers  chan *TrapHandler
	validator *validator.PacketValidator
	stats     *types.ListenerStats
	wg        sync.WaitGroup
	mu        sync.RWMutex
	running   bool
}

// TrapHandler represents a handler for processing individual SNMP traps.
type TrapHandler struct {
	Data   []byte
	Addr   *net.UDPAddr
	Packet *types.SNMPPacket
}

// NewListener creates a new SNMP trap listener with the provided configuration.
func NewListener(cfg config.Provider) (*Listener, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration provider cannot be nil")
	}

	// Get maximum handlers from configuration
	maxHandlers, err := cfg.GetInt("server.max_handlers", 100)
	if err != nil {
		return nil, fmt.Errorf("failed to get max_handlers configuration: %w", err)
	}

	// Create packet validator with default configuration
	packetValidator := validator.NewPacketValidator(validator.DefaultValidationConfig())

	// Initialize statistics
	stats := types.NewListenerStats()

	return &Listener{
		config:    cfg,
		handlers:  make(chan *TrapHandler, maxHandlers),
		validator: packetValidator,
		stats:     stats,
	}, nil
}

// Start starts the SNMP trap listener.
func (l *Listener) Start(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.running {
		return fmt.Errorf("listener is already running")
	}

	// Get server configuration
	host, err := l.config.GetString("server.host", "0.0.0.0")
	if err != nil {
		return fmt.Errorf("failed to get server host: %w", err)
	}

	port, err := l.config.GetInt("server.port", 162)
	if err != nil {
		return fmt.Errorf("failed to get server port: %w", err)
	}

	bufferSize, err := l.config.GetInt("server.buffer_size", 8192)
	if err != nil {
		return fmt.Errorf("failed to get buffer size: %w", err)
	}

	// Create UDP address
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return fmt.Errorf("failed to resolve UDP address: %w", err)
	}

	// Bind to UDP socket
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to bind to UDP socket: %w", err)
	}

	// Set socket buffer size
	if err := conn.SetReadBuffer(bufferSize); err != nil {
		conn.Close()
		return fmt.Errorf("failed to set read buffer size: %w", err)
	}

	l.conn = conn
	l.running = true

	// Start handler workers
	maxHandlers, _ := l.config.GetInt("server.max_handlers", 100)
	for i := 0; i < maxHandlers; i++ {
		l.wg.Add(1)
		go l.handlerWorker(ctx)
	}

	// Start main listener goroutine
	l.wg.Add(1)
	go l.listen(ctx)

	return nil
}

// Stop stops the SNMP trap listener gracefully.
func (l *Listener) Stop() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.running {
		return nil
	}

	l.running = false

	// Close UDP connection to stop accepting new packets
	if l.conn != nil {
		l.conn.Close()
	}

	// Close handler channel to signal workers to stop
	close(l.handlers)

	// Wait for all goroutines to finish
	l.wg.Wait()

	return nil
}

// IsRunning returns whether the listener is currently running.
func (l *Listener) IsRunning() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.running
}

// listen is the main listening loop that receives UDP packets.
func (l *Listener) listen(ctx context.Context) {
	defer l.wg.Done()

	// Get read timeout from configuration
	readTimeout, err := l.config.GetDuration("server.read_timeout", 30*time.Second)
	if err != nil {
		readTimeout = 30 * time.Second
	}

	buffer := make([]byte, 65536) // Maximum UDP packet size

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Set read deadline
			l.conn.SetReadDeadline(time.Now().Add(readTimeout))

			// Read UDP packet
			n, addr, err := l.conn.ReadFromUDP(buffer)
			if err != nil {
				// Check if this is a timeout or if we're shutting down
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue // Timeout is expected, continue listening
				}
				if !l.IsRunning() {
					return // Listener is shutting down
				}
				// Log error but continue listening
				continue
			}

			// Create a copy of the data for the handler
			data := make([]byte, n)
			copy(data, buffer[:n])

			// Create trap handler
			handler := &TrapHandler{
				Data: data,
				Addr: addr,
			}

			// Send to handler channel (non-blocking)
			select {
			case l.handlers <- handler:
				// Successfully queued for processing
			default:
				// Handler queue is full, drop the packet
				l.stats.PacketsDropped++
			}
		}
	}
}

// handlerWorker processes SNMP trap packets from the handler channel.
func (l *Listener) handlerWorker(ctx context.Context) {
	defer l.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case handler, ok := <-l.handlers:
			if !ok {
				return // Channel closed, worker should exit
			}

			// Process the trap
			l.processTrap(handler)
		}
	}
}

// processTrap processes an individual SNMP trap.
func (l *Listener) processTrap(handler *TrapHandler) {
	l.stats.PacketsReceived++

	// Parse the SNMP packet
	packet, err := l.parseSNMPPacket(handler.Data)
	if err != nil {
		l.stats.ParseErrors++
		// Log parsing error but don't stop processing
		return
	}

	// Validate the packet
	sourceAddr := handler.Addr.IP.String()
	if err := l.validator.ValidatePacket(packet, sourceAddr, handler.Data); err != nil {
		l.stats.ValidationErrors++
		// Log validation error but don't stop processing
		return
	}

	// Update statistics
	l.stats.PacketsProcessed++
	l.stats.LastPacketTime = time.Now()

	// Update version statistics
	versionName := types.GetVersionName(packet.Version)
	l.stats.PacketsByVersion[versionName]++

	// Update PDU type statistics
	pduTypeName := types.GetPDUTypeName(packet.PDUType)
	l.stats.PacketsByType[pduTypeName]++

	// Update community statistics
	l.stats.PacketsByCommunity[packet.Community]++

	// Set parsed packet in handler
	handler.Packet = packet

	// TODO: Forward to event processing system
	// For now, this is a placeholder for the actual trap processing logic
}

// parseSNMPPacket parses raw SNMP packet data into an SNMPPacket structure.
func (l *Listener) parseSNMPPacket(data []byte) (*types.SNMPPacket, error) {
	// Use the SNMP parser
	snmpParser := parser.NewSNMPParser(data)
	packet, err := snmpParser.ParseSNMPPacket()
	if err != nil {
		return nil, fmt.Errorf("failed to parse SNMP packet: %w", err)
	}

	return packet, nil
}

// GetStats returns listener statistics.
func (l *Listener) GetStats() map[string]interface{} {
	l.mu.RLock()
	defer l.mu.RUnlock()

	stats := map[string]interface{}{
		"running":              l.running,
		"queue_length":         len(l.handlers),
		"queue_cap":            cap(l.handlers),
		"packets_received":     l.stats.PacketsReceived,
		"packets_processed":    l.stats.PacketsProcessed,
		"packets_dropped":      l.stats.PacketsDropped,
		"parse_errors":         l.stats.ParseErrors,
		"validation_errors":    l.stats.ValidationErrors,
		"auth_errors":          l.stats.AuthErrors,
		"last_packet_time":     l.stats.LastPacketTime,
		"packets_by_version":   l.stats.PacketsByVersion,
		"packets_by_type":      l.stats.PacketsByType,
		"packets_by_community": l.stats.PacketsByCommunity,
	}

	if l.conn != nil {
		stats["local_addr"] = l.conn.LocalAddr().String()
	}

	return stats
}

// GetDetailedStats returns detailed listener statistics
func (l *Listener) GetDetailedStats() *types.ListenerStats {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// Create a copy of the stats
	statsCopy := *l.stats
	statsCopy.QueueLength = len(l.handlers)
	statsCopy.QueueCapacity = cap(l.handlers)

	return &statsCopy
}
