// Package snmp provides SNMP trap listening and processing functionality.
package snmp

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/geekxflood/common/config"
)

// Listener represents an SNMP trap listener that receives and processes SNMP traps.
type Listener struct {
	config   config.Provider
	conn     *net.UDPConn
	handlers chan *TrapHandler
	wg       sync.WaitGroup
	mu       sync.RWMutex
	running  bool
}

// TrapHandler represents a handler for processing individual SNMP traps.
type TrapHandler struct {
	Data   []byte
	Addr   *net.UDPAddr
	Packet *SNMPPacket
}

// SNMPPacket represents a parsed SNMP packet.
type SNMPPacket struct {
	Version   int
	Community string
	PDUType   int
	RequestID int32
	Varbinds  []Varbind
	Timestamp time.Time
}

// Varbind represents an SNMP variable binding.
type Varbind struct {
	OID   string
	Type  int
	Value interface{}
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

	return &Listener{
		config:   cfg,
		handlers: make(chan *TrapHandler, maxHandlers),
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
				// In a production system, you might want to log this
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
	// Parse the SNMP packet
	packet, err := l.parseSNMPPacket(handler.Data)
	if err != nil {
		// Log parsing error but don't stop processing
		return
	}

	// Validate community string
	expectedCommunity, _ := l.config.GetString("server.community", "public")
	if packet.Community != expectedCommunity {
		// Invalid community string, ignore packet
		return
	}

	// Set parsed packet in handler
	handler.Packet = packet

	// TODO: Forward to event processing system
	// For now, this is a placeholder for the actual trap processing logic
}

// parseSNMPPacket parses raw SNMP packet data into an SNMPPacket structure.
func (l *Listener) parseSNMPPacket(data []byte) (*SNMPPacket, error) {
	// This is a placeholder implementation
	// In a real implementation, this would use a proper SNMP library
	// like github.com/gosnmp/gosnmp or implement ASN.1 BER/DER parsing

	if len(data) < 10 {
		return nil, fmt.Errorf("packet too short")
	}

	// Basic validation - check for SNMP sequence tag
	if data[0] != 0x30 {
		return nil, fmt.Errorf("invalid SNMP packet: missing sequence tag")
	}

	// Create a basic packet structure
	// This is a simplified implementation for demonstration
	packet := &SNMPPacket{
		Version:   1, // Assume SNMP v1 for now
		Community: "public", // Default community
		PDUType:   4, // Trap PDU
		RequestID: 0,
		Varbinds:  []Varbind{},
		Timestamp: time.Now(),
	}

	return packet, nil
}

// GetStats returns listener statistics.
func (l *Listener) GetStats() map[string]interface{} {
	l.mu.RLock()
	defer l.mu.RUnlock()

	stats := map[string]interface{}{
		"running":      l.running,
		"queue_length": len(l.handlers),
		"queue_cap":    cap(l.handlers),
	}

	if l.conn != nil {
		stats["local_addr"] = l.conn.LocalAddr().String()
	}

	return stats
}
