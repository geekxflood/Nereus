// Package snmp provides SNMP trap listening and processing functionality.
package snmp

import (
	"fmt"
	"time"
)

// SNMP version constants
const (
	VersionSNMPv1  = 0
	VersionSNMPv2c = 1
	VersionSNMPv3  = 3
)

// SNMP PDU type constants
const (
	PDUTypeGetRequest     = 0
	PDUTypeGetNextRequest = 1
	PDUTypeGetResponse    = 2
	PDUTypeSetRequest     = 3
	PDUTypeTrap           = 4
	PDUTypeGetBulkRequest = 5
	PDUTypeInformRequest  = 6
	PDUTypeTrapV2         = 7
	PDUTypeReport         = 8
)

// SNMP data type constants
const (
	TypeInteger          = 0x02
	TypeOctetString      = 0x04
	TypeNull             = 0x05
	TypeObjectIdentifier = 0x06
	TypeIPAddress        = 0x40
	TypeCounter32        = 0x41
	TypeGauge32          = 0x42
	TypeTimeTicks        = 0x43
	TypeOpaque           = 0x44
	TypeCounter64        = 0x46
)

// SNMP error status constants
const (
	ErrorStatusNoError             = 0
	ErrorStatusTooBig              = 1
	ErrorStatusNoSuchName          = 2
	ErrorStatusBadValue            = 3
	ErrorStatusReadOnly            = 4
	ErrorStatusGenErr              = 5
	ErrorStatusNoAccess            = 6
	ErrorStatusWrongType           = 7
	ErrorStatusWrongLength         = 8
	ErrorStatusWrongEncoding       = 9
	ErrorStatusWrongValue          = 10
	ErrorStatusNoCreation          = 11
	ErrorStatusInconsistentValue   = 12
	ErrorStatusResourceUnavailable = 13
	ErrorStatusCommitFailed        = 14
	ErrorStatusUndoFailed          = 15
	ErrorStatusAuthorizationError  = 16
	ErrorStatusNotWritable         = 17
	ErrorStatusInconsistentName    = 18
)

// TrapInfo represents information about an SNMP trap.
type TrapInfo struct {
	Version       int                    `json:"version"`
	Community     string                 `json:"community"`
	AgentAddress  string                 `json:"agent_address"`
	GenericTrap   int                    `json:"generic_trap,omitempty"`
	SpecificTrap  int                    `json:"specific_trap,omitempty"`
	Timestamp     time.Time              `json:"timestamp"`
	Uptime        uint32                 `json:"uptime,omitempty"`
	TrapOID       string                 `json:"trap_oid,omitempty"`
	Varbinds      []Varbind              `json:"varbinds"`
	SourceAddress string                 `json:"source_address"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// VarbindValue represents different types of SNMP values.
type VarbindValue struct {
	Type  int         `json:"type"`
	Value interface{} `json:"value"`
}

// String returns a string representation of the varbind value.
func (v VarbindValue) String() string {
	switch v.Type {
	case TypeInteger:
		if val, ok := v.Value.(int64); ok {
			return fmt.Sprintf("%d", val)
		}
	case TypeOctetString:
		if val, ok := v.Value.([]byte); ok {
			return string(val)
		}
		if val, ok := v.Value.(string); ok {
			return val
		}
	case TypeObjectIdentifier:
		if val, ok := v.Value.(string); ok {
			return val
		}
	case TypeIPAddress:
		if val, ok := v.Value.(string); ok {
			return val
		}
	case TypeCounter32, TypeGauge32:
		if val, ok := v.Value.(uint32); ok {
			return fmt.Sprintf("%d", val)
		}
	case TypeTimeTicks:
		if val, ok := v.Value.(uint32); ok {
			return fmt.Sprintf("%d", val)
		}
	case TypeCounter64:
		if val, ok := v.Value.(uint64); ok {
			return fmt.Sprintf("%d", val)
		}
	case TypeNull:
		return "null"
	}
	return fmt.Sprintf("%v", v.Value)
}

// GetTypeName returns the human-readable name of the SNMP data type.
func (v VarbindValue) GetTypeName() string {
	switch v.Type {
	case TypeInteger:
		return "INTEGER"
	case TypeOctetString:
		return "OCTET STRING"
	case TypeNull:
		return "NULL"
	case TypeObjectIdentifier:
		return "OBJECT IDENTIFIER"
	case TypeIPAddress:
		return "IpAddress"
	case TypeCounter32:
		return "Counter32"
	case TypeGauge32:
		return "Gauge32"
	case TypeTimeTicks:
		return "TimeTicks"
	case TypeOpaque:
		return "Opaque"
	case TypeCounter64:
		return "Counter64"
	default:
		return fmt.Sprintf("Unknown(%d)", v.Type)
	}
}

// GenericTrap constants for SNMP v1 traps
const (
	GenericTrapColdStart             = 0
	GenericTrapWarmStart             = 1
	GenericTrapLinkDown              = 2
	GenericTrapLinkUp                = 3
	GenericTrapAuthenticationFailure = 4
	GenericTrapEgpNeighborLoss       = 5
	GenericTrapEnterpriseSpecific    = 6
)

// GetGenericTrapName returns the human-readable name of a generic trap type.
func GetGenericTrapName(trapType int) string {
	switch trapType {
	case GenericTrapColdStart:
		return "coldStart"
	case GenericTrapWarmStart:
		return "warmStart"
	case GenericTrapLinkDown:
		return "linkDown"
	case GenericTrapLinkUp:
		return "linkUp"
	case GenericTrapAuthenticationFailure:
		return "authenticationFailure"
	case GenericTrapEgpNeighborLoss:
		return "egpNeighborLoss"
	case GenericTrapEnterpriseSpecific:
		return "enterpriseSpecific"
	default:
		return fmt.Sprintf("unknown(%d)", trapType)
	}
}

// GetVersionName returns the human-readable name of an SNMP version.
func GetVersionName(version int) string {
	switch version {
	case VersionSNMPv1:
		return "SNMPv1"
	case VersionSNMPv2c:
		return "SNMPv2c"
	case VersionSNMPv3:
		return "SNMPv3"
	default:
		return fmt.Sprintf("Unknown(%d)", version)
	}
}

// GetPDUTypeName returns the human-readable name of a PDU type.
func GetPDUTypeName(pduType int) string {
	switch pduType {
	case PDUTypeGetRequest:
		return "GetRequest"
	case PDUTypeGetNextRequest:
		return "GetNextRequest"
	case PDUTypeGetResponse:
		return "GetResponse"
	case PDUTypeSetRequest:
		return "SetRequest"
	case PDUTypeTrap:
		return "Trap"
	case PDUTypeGetBulkRequest:
		return "GetBulkRequest"
	case PDUTypeInformRequest:
		return "InformRequest"
	case PDUTypeTrapV2:
		return "TrapV2"
	case PDUTypeReport:
		return "Report"
	default:
		return fmt.Sprintf("Unknown(%d)", pduType)
	}
}

// ValidationError represents an SNMP packet validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error in %s: %s", e.Field, e.Message)
}

// ParseError represents an SNMP packet parsing error.
type ParseError struct {
	Offset  int
	Message string
}

func (e ParseError) Error() string {
	return fmt.Sprintf("parse error at offset %d: %s", e.Offset, e.Message)
}

// ListenerStats represents statistics for the SNMP listener.
type ListenerStats struct {
	PacketsReceived    uint64            `json:"packets_received"`
	PacketsProcessed   uint64            `json:"packets_processed"`
	PacketsDropped     uint64            `json:"packets_dropped"`
	ParseErrors        uint64            `json:"parse_errors"`
	ValidationErrors   uint64            `json:"validation_errors"`
	AuthErrors         uint64            `json:"auth_errors"`
	QueueLength        int               `json:"queue_length"`
	QueueCapacity      int               `json:"queue_capacity"`
	ActiveHandlers     int               `json:"active_handlers"`
	Uptime             time.Duration     `json:"uptime"`
	LastPacketTime     time.Time         `json:"last_packet_time"`
	PacketsByVersion   map[string]uint64 `json:"packets_by_version"`
	PacketsByType      map[string]uint64 `json:"packets_by_type"`
	PacketsByCommunity map[string]uint64 `json:"packets_by_community"`
}

// NewListenerStats creates a new ListenerStats instance.
func NewListenerStats() *ListenerStats {
	return &ListenerStats{
		PacketsByVersion:   make(map[string]uint64),
		PacketsByType:      make(map[string]uint64),
		PacketsByCommunity: make(map[string]uint64),
	}
}
