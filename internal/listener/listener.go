// Package listener provides SNMP trap listening and validation functionality.
package listener

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/geekxflood/common/config"
	"github.com/geekxflood/nereus/internal/mib"
	"github.com/geekxflood/nereus/internal/types"
)

// ValidationConfig holds configuration for packet validation
type ValidationConfig struct {
	MaxPacketSize      int           `json:"max_packet_size"`
	MaxVarbinds        int           `json:"max_varbinds"`
	AllowedVersions    []int         `json:"allowed_versions"`
	AllowedCommunities []string      `json:"allowed_communities"`
	BlockedSources     []string      `json:"blocked_sources"`
	AllowedSources     []string      `json:"allowed_sources"`
	MaxOIDLength       int           `json:"max_oid_length"`
	MaxStringLength    int           `json:"max_string_length"`
	ValidateTimestamp  bool          `json:"validate_timestamp"`
	MaxTimestampSkew   time.Duration `json:"max_timestamp_skew"`
}

// DefaultValidationConfig returns a default validation configuration
func DefaultValidationConfig() *ValidationConfig {
	return &ValidationConfig{
		MaxPacketSize:      65536,
		MaxVarbinds:        100,
		AllowedVersions:    []int{types.VersionSNMPv2c},
		AllowedCommunities: []string{"public"},
		BlockedSources:     []string{},
		AllowedSources:     []string{},
		MaxOIDLength:       128,
		MaxStringLength:    1024,
		ValidateTimestamp:  false,
		MaxTimestampSkew:   time.Hour,
	}
}

// PacketValidator validates SNMP packets for security and correctness
type PacketValidator struct {
	config *ValidationConfig
}

// NewPacketValidator creates a new packet validator with the given configuration
func NewPacketValidator(config *ValidationConfig) *PacketValidator {
	if config == nil {
		config = DefaultValidationConfig()
	}
	return &PacketValidator{config: config}
}

// ValidatePacket performs comprehensive validation of an SNMP packet
func (v *PacketValidator) ValidatePacket(packet *types.SNMPPacket, sourceAddr string, rawData []byte) error {
	if packet == nil {
		return &types.ValidationError{Field: "packet", Message: "packet is nil"}
	}

	// Validate packet size
	if err := v.validatePacketSize(rawData); err != nil {
		return err
	}

	// Validate source address
	if err := v.validateSourceAddress(sourceAddr); err != nil {
		return err
	}

	// Validate SNMP version
	if err := v.validateVersion(packet.Version); err != nil {
		return err
	}

	// Validate community string
	if err := v.validateCommunity(packet.Community); err != nil {
		return err
	}

	// Validate PDU type
	if err := v.validatePDUType(packet.PDUType, packet.Version); err != nil {
		return err
	}

	// Validate varbinds
	if err := v.validateVarbinds(packet.Varbinds); err != nil {
		return err
	}

	// Validate version-specific fields
	switch packet.Version {
	case types.VersionSNMPv2c:
		if err := v.validateSNMPv2cFields(packet); err != nil {
			return err
		}
	}

	// Validate timestamp if enabled
	if v.config.ValidateTimestamp {
		if err := v.validateTimestamp(packet.Timestamp); err != nil {
			return err
		}
	}

	return nil
}

// validatePacketSize checks if the packet size is within limits
func (v *PacketValidator) validatePacketSize(rawData []byte) error {
	if len(rawData) > v.config.MaxPacketSize {
		return &types.ValidationError{
			Field:   "packet_size",
			Message: fmt.Sprintf("packet size %d exceeds maximum %d", len(rawData), v.config.MaxPacketSize),
		}
	}
	return nil
}

// validateSourceAddress validates the source IP address
func (v *PacketValidator) validateSourceAddress(sourceAddr string) error {
	ip := net.ParseIP(sourceAddr)
	if ip == nil {
		return &types.ValidationError{
			Field:   "source_address",
			Message: fmt.Sprintf("invalid IP address: %s", sourceAddr),
		}
	}

	// Check blocked sources
	for _, blocked := range v.config.BlockedSources {
		if v.matchesIPPattern(ip.String(), blocked) {
			return &types.ValidationError{
				Field:   "source_address",
				Message: fmt.Sprintf("source address %s is blocked", ip.String()),
			}
		}
	}

	// Check allowed sources (if configured)
	if len(v.config.AllowedSources) > 0 {
		allowed := false
		for _, allowedPattern := range v.config.AllowedSources {
			if v.matchesIPPattern(ip.String(), allowedPattern) {
				allowed = true
				break
			}
		}
		if !allowed {
			return &types.ValidationError{
				Field:   "source_address",
				Message: fmt.Sprintf("source address %s is not in allowed list", ip.String()),
			}
		}
	}

	return nil
}

// matchesIPPattern checks if an IP matches a pattern (supports CIDR and wildcards)
func (v *PacketValidator) matchesIPPattern(ip, pattern string) bool {
	// Try CIDR notation first
	if strings.Contains(pattern, "/") {
		_, network, err := net.ParseCIDR(pattern)
		if err == nil {
			ipAddr := net.ParseIP(ip)
			if ipAddr != nil {
				return network.Contains(ipAddr)
			}
		}
	}

	// Simple string match
	return ip == pattern
}

// validateVersion checks if the SNMP version is allowed
func (v *PacketValidator) validateVersion(version int) error {
	for _, allowed := range v.config.AllowedVersions {
		if version == allowed {
			return nil
		}
	}
	return &types.ValidationError{
		Field:   "version",
		Message: fmt.Sprintf("SNMP version %d is not allowed", version),
	}
}

// validateCommunity checks if the community string is allowed
func (v *PacketValidator) validateCommunity(community string) error {
	if len(v.config.AllowedCommunities) == 0 {
		return nil // No restrictions
	}

	for _, allowed := range v.config.AllowedCommunities {
		if community == allowed {
			return nil
		}
	}
	return &types.ValidationError{
		Field:   "community",
		Message: fmt.Sprintf("community string '%s' is not allowed", community),
	}
}

// validatePDUType checks if the PDU type is valid for the SNMP version
func (v *PacketValidator) validatePDUType(pduType, version int) error {
	switch version {
	case types.VersionSNMPv2c:
		if pduType != types.PDUTypeTrapV2 && pduType != types.PDUTypeInformRequest {
			return &types.ValidationError{
				Field:   "pdu_type",
				Message: fmt.Sprintf("PDU type %d is not valid for SNMPv2c", pduType),
			}
		}
	default:
		return &types.ValidationError{
			Field:   "pdu_type",
			Message: fmt.Sprintf("unknown SNMP version %d (only SNMPv2c is supported)", version),
		}
	}
	return nil
}

// validateVarbinds validates the variable bindings
func (v *PacketValidator) validateVarbinds(varbinds []types.Varbind) error {
	if len(varbinds) > v.config.MaxVarbinds {
		return &types.ValidationError{
			Field:   "varbinds",
			Message: fmt.Sprintf("too many varbinds: %d (max: %d)", len(varbinds), v.config.MaxVarbinds),
		}
	}

	for i, vb := range varbinds {
		// Validate OID length
		if len(vb.OID) > v.config.MaxOIDLength {
			return &types.ValidationError{
				Field:   fmt.Sprintf("varbind[%d].oid", i),
				Message: fmt.Sprintf("OID too long: %d characters (max: %d)", len(vb.OID), v.config.MaxOIDLength),
			}
		}

		// Validate string values
		if vb.Type == types.TypeOctetString {
			if str, ok := vb.Value.(string); ok {
				if len(str) > v.config.MaxStringLength {
					return &types.ValidationError{
						Field:   fmt.Sprintf("varbind[%d].value", i),
						Message: fmt.Sprintf("string value too long: %d characters (max: %d)", len(str), v.config.MaxStringLength),
					}
				}
			}
		}
	}

	return nil
}

// validateSNMPv2cFields validates SNMP v2c specific fields
func (v *PacketValidator) validateSNMPv2cFields(packet *types.SNMPPacket) error {
	// Error status should be 0 for traps
	if packet.ErrorStatus != 0 {
		return &types.ValidationError{
			Field:   "error_status",
			Message: fmt.Sprintf("unexpected error status %d in trap", packet.ErrorStatus),
		}
	}

	// Error index should be 0 for traps
	if packet.ErrorIndex != 0 {
		return &types.ValidationError{
			Field:   "error_index",
			Message: fmt.Sprintf("unexpected error index %d in trap", packet.ErrorIndex),
		}
	}

	return nil
}

// validateTimestamp validates the packet timestamp
func (v *PacketValidator) validateTimestamp(timestamp time.Time) error {
	if timestamp.IsZero() {
		return nil // No timestamp to validate
	}

	now := time.Now()
	skew := now.Sub(timestamp)
	if skew < 0 {
		skew = -skew
	}

	if skew > v.config.MaxTimestampSkew {
		return &types.ValidationError{
			Field:   "timestamp",
			Message: fmt.Sprintf("timestamp skew too large: %v (max: %v)", skew, v.config.MaxTimestampSkew),
		}
	}

	return nil
}

// Listener represents an SNMP trap listener that receives and processes SNMP traps.
type Listener struct {
	config    config.Provider
	conn      *net.UDPConn
	handlers  chan *TrapHandler
	validator *PacketValidator
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
	packetValidator := NewPacketValidator(DefaultValidationConfig())

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
	// Create a temporary MIB manager for parsing (we only need the parser functionality)
	// This is a lightweight approach since we don't need the full MIB loading capabilities
	tempManager := &mib.Manager{}
	snmpParser := tempManager.NewSNMPParser(data)
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
