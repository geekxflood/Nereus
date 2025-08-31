package helpers

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"

	"github.com/geekxflood/nereus/internal/types"
)

// SNMPTrapGenerator provides utilities for generating SNMP trap packets for testing
type SNMPTrapGenerator struct {
	Community string
	SourceIP  string
}

// NewSNMPTrapGenerator creates a new SNMP trap generator
func NewSNMPTrapGenerator(community, sourceIP string) *SNMPTrapGenerator {
	return &SNMPTrapGenerator{
		Community: community,
		SourceIP:  sourceIP,
	}
}

// StandardTraps contains definitions for standard SNMP traps
var StandardTraps = map[string]TrapDefinition{
	"coldStart": {
		OID:         "1.3.6.1.6.3.1.1.5.1",
		Name:        "coldStart",
		Description: "System cold restart",
		Varbinds:    []VarbindTemplate{},
	},
	"warmStart": {
		OID:         "1.3.6.1.6.3.1.1.5.2",
		Name:        "warmStart",
		Description: "System warm restart",
		Varbinds:    []VarbindTemplate{},
	},
	"linkDown": {
		OID:         "1.3.6.1.6.3.1.1.5.3",
		Name:        "linkDown",
		Description: "Network interface down",
		Varbinds: []VarbindTemplate{
			{OID: "1.3.6.1.2.1.2.2.1.1", Type: "integer", Value: "1"}, // ifIndex
		},
	},
	"linkUp": {
		OID:         "1.3.6.1.6.3.1.1.5.4",
		Name:        "linkUp",
		Description: "Network interface up",
		Varbinds: []VarbindTemplate{
			{OID: "1.3.6.1.2.1.2.2.1.1", Type: "integer", Value: "1"}, // ifIndex
		},
	},
	"authenticationFailure": {
		OID:         "1.3.6.1.6.3.1.1.5.5",
		Name:        "authenticationFailure",
		Description: "SNMP authentication failure",
		Varbinds:    []VarbindTemplate{},
	},
}

// TrapDefinition defines a trap template
type TrapDefinition struct {
	OID         string
	Name        string
	Description string
	Varbinds    []VarbindTemplate
}

// VarbindTemplate defines a varbind template
type VarbindTemplate struct {
	OID   string
	Type  string
	Value string
}

// GenerateStandardTrap generates a standard SNMP trap
func (g *SNMPTrapGenerator) GenerateStandardTrap(trapName string) (*types.SNMPPacket, error) {
	trapDef, exists := StandardTraps[trapName]
	if !exists {
		return nil, fmt.Errorf("unknown standard trap: %s", trapName)
	}

	return g.GenerateTrap(trapDef.OID, trapDef.Varbinds)
}

// GenerateTrap generates an SNMP trap with the specified OID and varbinds
func (g *SNMPTrapGenerator) GenerateTrap(trapOID string, varbindTemplates []VarbindTemplate) (*types.SNMPPacket, error) {
	// Create standard SNMPv2c trap varbinds
	varbinds := []types.Varbind{
		// sysUpTime (required for SNMPv2c traps)
		{
			OID:   "1.3.6.1.2.1.1.3.0",
			Type:  "timeticks",
			Value: fmt.Sprintf("%d", time.Now().Unix()*100), // Convert to centiseconds
		},
		// snmpTrapOID (required for SNMPv2c traps)
		{
			OID:   "1.3.6.1.6.3.1.1.4.1.0",
			Type:  "oid",
			Value: trapOID,
		},
	}

	// Add custom varbinds
	for _, template := range varbindTemplates {
		varbinds = append(varbinds, types.Varbind{
			OID:   template.OID,
			Type:  template.Type,
			Value: template.Value,
		})
	}

	return &types.SNMPPacket{
		Version:   1, // SNMPv2c
		Community: g.Community,
		PDUType:   7, // Trap
		RequestID: uint32(time.Now().Unix() & 0xFFFFFFFF),
		Varbinds:  varbinds,
	}, nil
}

// GenerateCustomTrap generates a custom trap with specified parameters
func (g *SNMPTrapGenerator) GenerateCustomTrap(trapOID string, customVarbinds map[string]string) (*types.SNMPPacket, error) {
	var varbindTemplates []VarbindTemplate
	for oid, value := range customVarbinds {
		varbindTemplates = append(varbindTemplates, VarbindTemplate{
			OID:   oid,
			Type:  "string",
			Value: value,
		})
	}

	return g.GenerateTrap(trapOID, varbindTemplates)
}

// GenerateMalformedTrap generates a malformed trap for error testing
func (g *SNMPTrapGenerator) GenerateMalformedTrap(malformationType string) (*types.SNMPPacket, error) {
	packet, err := g.GenerateStandardTrap("coldStart")
	if err != nil {
		return nil, err
	}

	switch malformationType {
	case "invalid_version":
		packet.Version = 99 // Invalid version
	case "empty_community":
		packet.Community = ""
	case "invalid_pdu_type":
		packet.PDUType = 99 // Invalid PDU type
	case "no_varbinds":
		packet.Varbinds = []types.Varbind{}
	case "invalid_oid":
		packet.Varbinds[0].OID = "invalid.oid.format"
	case "oversized_packet":
		// Create a very large varbind
		largeValue := make([]byte, 100000)
		for i := range largeValue {
			largeValue[i] = 'A'
		}
		packet.Varbinds = append(packet.Varbinds, types.Varbind{
			OID:   "1.3.6.1.4.1.99999.1",
			Type:  "string",
			Value: string(largeValue),
		})
	default:
		return nil, fmt.Errorf("unknown malformation type: %s", malformationType)
	}

	return packet, nil
}

// SendTrapToListener sends an SNMP trap to the specified listener
func (g *SNMPTrapGenerator) SendTrapToListener(packet *types.SNMPPacket, listenerAddr string) error {
	// Convert packet to raw bytes (simplified implementation)
	rawPacket, err := g.encodePacket(packet)
	if err != nil {
		return fmt.Errorf("failed to encode packet: %w", err)
	}

	// Send UDP packet
	conn, err := net.Dial("udp", listenerAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to listener: %w", err)
	}
	defer conn.Close()

	_, err = conn.Write(rawPacket)
	if err != nil {
		return fmt.Errorf("failed to send packet: %w", err)
	}

	return nil
}

// encodePacket encodes an SNMP packet to raw bytes (simplified BER encoding)
func (g *SNMPTrapGenerator) encodePacket(packet *types.SNMPPacket) ([]byte, error) {
	// This is a simplified implementation for testing purposes
	// In a real implementation, you would use proper ASN.1 BER encoding

	var result []byte

	// SNMP Message sequence
	result = append(result, 0x30) // SEQUENCE tag

	// Version
	result = append(result, 0x02, 0x01, byte(packet.Version))

	// Community
	communityBytes := []byte(packet.Community)
	result = append(result, 0x04, byte(len(communityBytes)))
	result = append(result, communityBytes...)

	// PDU
	result = append(result, 0xA7) // Trap PDU tag

	// Request ID
	requestIDBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(requestIDBytes, packet.RequestID)
	result = append(result, 0x02, 0x04)
	result = append(result, requestIDBytes...)

	// Error status (0)
	result = append(result, 0x02, 0x01, 0x00)

	// Error index (0)
	result = append(result, 0x02, 0x01, 0x00)

	// Varbind list
	result = append(result, 0x30) // SEQUENCE tag for varbind list

	var varbindData []byte
	for _, vb := range packet.Varbinds {
		// Varbind sequence
		varbindData = append(varbindData, 0x30) // SEQUENCE tag

		// OID
		oidBytes := g.encodeOID(vb.OID)
		varbindData = append(varbindData, 0x06, byte(len(oidBytes)))
		varbindData = append(varbindData, oidBytes...)

		// Value (simplified - treat everything as string)
		valueBytes := []byte(fmt.Sprintf("%v", vb.Value))
		varbindData = append(varbindData, 0x04, byte(len(valueBytes)))
		varbindData = append(varbindData, valueBytes...)

		// Update varbind sequence length
		varbindLen := len(oidBytes) + len(valueBytes) + 4 // +4 for tags and lengths
		varbindData[len(varbindData)-varbindLen-1] = byte(varbindLen)
	}

	// Update varbind list length
	result = append(result, byte(len(varbindData)))
	result = append(result, varbindData...)

	// Update PDU length
	pduLen := len(result) - 2 // Exclude sequence tag and length
	result[len(result)-pduLen-1] = byte(pduLen)

	// Update message length
	messageLen := len(result) - 2 // Exclude sequence tag and length
	result[1] = byte(messageLen)

	return result, nil
}

// encodeOID encodes an OID string to bytes (simplified)
func (g *SNMPTrapGenerator) encodeOID(oidStr string) []byte {
	// This is a very simplified OID encoding for testing
	// In practice, you would parse the OID string and encode properly
	return []byte{0x2B, 0x06, 0x01, 0x02, 0x01, 0x01, 0x03, 0x00} // 1.3.6.1.2.1.1.3.0
}

// GenerateBurstTraps generates multiple traps for load testing
func (g *SNMPTrapGenerator) GenerateBurstTraps(count int, trapType string, interval time.Duration) ([]*types.SNMPPacket, error) {
	var packets []*types.SNMPPacket

	for i := 0; i < count; i++ {
		packet, err := g.GenerateStandardTrap(trapType)
		if err != nil {
			return nil, fmt.Errorf("failed to generate trap %d: %w", i, err)
		}

		// Modify request ID to make each packet unique
		packet.RequestID = uint32(time.Now().UnixNano() + int64(i))

		packets = append(packets, packet)

		if interval > 0 && i < count-1 {
			time.Sleep(interval)
		}
	}

	return packets, nil
}

// VendorTraps contains vendor-specific trap definitions
var VendorTraps = map[string]TrapDefinition{
	"ciscoConfigChange": {
		OID:         "1.3.6.1.4.1.9.9.43.1.1.1",
		Name:        "ciscoConfigManEvent",
		Description: "Cisco configuration change",
		Varbinds: []VarbindTemplate{
			{OID: "1.3.6.1.4.1.9.9.43.1.1.1.1", Type: "string", Value: "config-change"},
		},
	},
	"juniperAlarm": {
		OID:         "1.3.6.1.4.1.2636.4.5.0.1",
		Name:        "jnxEventTrap",
		Description: "Juniper alarm event",
		Varbinds: []VarbindTemplate{
			{OID: "1.3.6.1.4.1.2636.3.18.1.7.1.2", Type: "string", Value: "MAJOR"},
		},
	},
	"hpServerAlert": {
		OID:         "1.3.6.1.4.1.232.0.136006144",
		Name:        "cpqHoTrapFlags",
		Description: "HP server alert",
		Varbinds: []VarbindTemplate{
			{OID: "1.3.6.1.4.1.232.11.2.11.1.0", Type: "integer", Value: "3"},
		},
	},
}

// GenerateVendorTrap generates a vendor-specific trap
func (g *SNMPTrapGenerator) GenerateVendorTrap(vendorTrap string) (*types.SNMPPacket, error) {
	trapDef, exists := VendorTraps[vendorTrap]
	if !exists {
		return nil, fmt.Errorf("unknown vendor trap: %s", vendorTrap)
	}

	return g.GenerateTrap(trapDef.OID, trapDef.Varbinds)
}
