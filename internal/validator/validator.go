// Package validator provides SNMP packet validation functionality.
package validator

import (
	"fmt"
	"net"
	"strings"
	"time"

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
		AllowedVersions:    []int{types.VersionSNMPv1, types.VersionSNMPv2c},
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
	case types.VersionSNMPv1:
		if err := v.validateSNMPv1Fields(packet); err != nil {
			return err
		}
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

// validateSourceAddress checks if the source address is allowed
func (v *PacketValidator) validateSourceAddress(sourceAddr string) error {
	if sourceAddr == "" {
		return &types.ValidationError{Field: "source_address", Message: "source address is empty"}
	}

	// Parse IP address
	ip := net.ParseIP(sourceAddr)
	if ip == nil {
		// Try to extract IP from address:port format
		host, _, err := net.SplitHostPort(sourceAddr)
		if err != nil {
			return &types.ValidationError{Field: "source_address", Message: "invalid source address format"}
		}
		ip = net.ParseIP(host)
		if ip == nil {
			return &types.ValidationError{Field: "source_address", Message: "invalid IP address"}
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
	// Exact match
	if ip == pattern {
		return true
	}

	// CIDR match
	if strings.Contains(pattern, "/") {
		_, network, err := net.ParseCIDR(pattern)
		if err == nil {
			ipAddr := net.ParseIP(ip)
			if ipAddr != nil && network.Contains(ipAddr) {
				return true
			}
		}
	}

	// Simple wildcard match (e.g., "192.168.*")
	if strings.Contains(pattern, "*") {
		pattern = strings.ReplaceAll(pattern, "*", "")
		return strings.HasPrefix(ip, pattern)
	}

	return false
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
	if community == "" {
		return &types.ValidationError{Field: "community", Message: "community string is empty"}
	}

	// Check if community is in allowed list
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
	case types.VersionSNMPv1:
		if pduType != types.PDUTypeTrap {
			return &types.ValidationError{
				Field:   "pdu_type",
				Message: fmt.Sprintf("PDU type %d is not valid for SNMPv1", pduType),
			}
		}
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
			Message: fmt.Sprintf("unknown SNMP version %d", version),
		}
	}
	return nil
}

// validateVarbinds validates the varbind list
func (v *PacketValidator) validateVarbinds(varbinds []types.Varbind) error {
	if len(varbinds) > v.config.MaxVarbinds {
		return &types.ValidationError{
			Field:   "varbinds",
			Message: fmt.Sprintf("too many varbinds: %d (max %d)", len(varbinds), v.config.MaxVarbinds),
		}
	}

	for i, vb := range varbinds {
		if err := v.validateVarbind(vb, i); err != nil {
			return err
		}
	}

	return nil
}

// validateVarbind validates a single varbind
func (v *PacketValidator) validateVarbind(vb types.Varbind, index int) error {
	// Validate OID
	if len(vb.OID) > v.config.MaxOIDLength {
		return &types.ValidationError{
			Field:   fmt.Sprintf("varbind[%d].oid", index),
			Message: fmt.Sprintf("OID too long: %d characters (max %d)", len(vb.OID), v.config.MaxOIDLength),
		}
	}

	if !v.isValidOID(vb.OID) {
		return &types.ValidationError{
			Field:   fmt.Sprintf("varbind[%d].oid", index),
			Message: fmt.Sprintf("invalid OID format: %s", vb.OID),
		}
	}

	// Validate value based on type
	switch vb.Type {
	case types.TypeOctetString:
		if data, ok := vb.Value.([]byte); ok {
			if len(data) > v.config.MaxStringLength {
				return &types.ValidationError{
					Field:   fmt.Sprintf("varbind[%d].value", index),
					Message: fmt.Sprintf("string too long: %d bytes (max %d)", len(data), v.config.MaxStringLength),
				}
			}
		}
	case types.TypeObjectIdentifier:
		if oid, ok := vb.Value.(string); ok {
			if len(oid) > v.config.MaxOIDLength {
				return &types.ValidationError{
					Field:   fmt.Sprintf("varbind[%d].value", index),
					Message: fmt.Sprintf("OID value too long: %d characters (max %d)", len(oid), v.config.MaxOIDLength),
				}
			}
			if !v.isValidOID(oid) {
				return &types.ValidationError{
					Field:   fmt.Sprintf("varbind[%d].value", index),
					Message: fmt.Sprintf("invalid OID value format: %s", oid),
				}
			}
		}
	}

	return nil
}

// isValidOID checks if an OID string has valid format
func (v *PacketValidator) isValidOID(oid string) bool {
	if oid == "" {
		return false
	}

	parts := strings.Split(oid, ".")
	if len(parts) < 2 {
		return false
	}

	for _, part := range parts {
		if part == "" {
			return false
		}
		// Check if part contains only digits
		for _, char := range part {
			if char < '0' || char > '9' {
				return false
			}
		}
	}

	return true
}

// validateSNMPv1Fields validates SNMP v1 specific fields
func (v *PacketValidator) validateSNMPv1Fields(packet *types.SNMPPacket) error {
	if packet.EnterpriseOID == "" {
		return &types.ValidationError{Field: "enterprise_oid", Message: "enterprise OID is required for SNMPv1"}
	}

	if !v.isValidOID(packet.EnterpriseOID) {
		return &types.ValidationError{Field: "enterprise_oid", Message: "invalid enterprise OID format"}
	}

	if packet.AgentAddress == "" {
		return &types.ValidationError{Field: "agent_address", Message: "agent address is required for SNMPv1"}
	}

	if net.ParseIP(packet.AgentAddress) == nil {
		return &types.ValidationError{Field: "agent_address", Message: "invalid agent address format"}
	}

	if packet.GenericTrap < 0 || packet.GenericTrap > 6 {
		return &types.ValidationError{
			Field:   "generic_trap",
			Message: fmt.Sprintf("invalid generic trap value: %d", packet.GenericTrap),
		}
	}

	return nil
}

// validateSNMPv2cFields validates SNMP v2c specific fields
func (v *PacketValidator) validateSNMPv2cFields(packet *types.SNMPPacket) error {
	// Request ID should be non-zero for most implementations
	if packet.RequestID == 0 {
		// This is a warning, not an error
		// Some implementations might use 0 as a valid request ID
	}

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
	now := time.Now()
	diff := now.Sub(timestamp)
	if diff < 0 {
		diff = -diff
	}

	if diff > v.config.MaxTimestampSkew {
		return &types.ValidationError{
			Field:   "timestamp",
			Message: fmt.Sprintf("timestamp skew too large: %v (max %v)", diff, v.config.MaxTimestampSkew),
		}
	}

	return nil
}
