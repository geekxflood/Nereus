package parser

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/geekxflood/nereus/internal/types"
)

// Test data for SNMP v1 trap packet
var snmpv1TrapPacket = []byte{
	0x30, 0x82, 0x00, 0x4a, // SEQUENCE, length 74
	0x02, 0x01, 0x00, // INTEGER version 0 (SNMPv1)
	0x04, 0x06, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63, // OCTET STRING "public"
	0xa4, 0x3d, // Trap PDU, length 61
	0x06, 0x08, 0x2b, 0x06, 0x01, 0x04, 0x01, 0x94, 0x78, 0x01, // Enterprise OID 1.3.6.1.4.1.18872.1
	0x40, 0x04, 0xc0, 0xa8, 0x01, 0x01, // Agent address 192.168.1.1
	0x02, 0x01, 0x06, // Generic trap 6 (enterprise specific)
	0x02, 0x01, 0x01, // Specific trap 1
	0x43, 0x04, 0x00, 0x00, 0x01, 0x2c, // Timestamp 300 (3 seconds)
	0x30, 0x1e, // Varbind list SEQUENCE, length 30
	0x30, 0x1c, // Varbind SEQUENCE, length 28
	0x06, 0x0a, 0x2b, 0x06, 0x01, 0x04, 0x01, 0x94, 0x78, 0x01, 0x01, 0x01, // OID 1.3.6.1.4.1.18872.1.1.1
	0x04, 0x0e, 0x54, 0x65, 0x73, 0x74, 0x20, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x21, 0x21, // OCTET STRING "Test message!!"
}

// Test data for SNMP v2c trap packet
var snmpv2cTrapPacket = []byte{
	0x30, 0x82, 0x00, 0x63, // SEQUENCE, length 99
	0x02, 0x01, 0x01, // INTEGER version 1 (SNMPv2c)
	0x04, 0x06, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63, // OCTET STRING "public"
	0xa7, 0x56, // Trap v2 PDU, length 86
	0x02, 0x04, 0x12, 0x34, 0x56, 0x78, // Request ID 0x12345678
	0x02, 0x01, 0x00, // Error status 0
	0x02, 0x01, 0x00, // Error index 0
	0x30, 0x48, // Varbind list SEQUENCE, length 72
	// First varbind: sysUpTime
	0x30, 0x0f,
	0x06, 0x08, 0x2b, 0x06, 0x01, 0x02, 0x01, 0x01, 0x03, 0x00, // OID 1.3.6.1.2.1.1.3.0
	0x43, 0x03, 0x00, 0x01, 0x2c, // TimeTicks 300
	// Second varbind: snmpTrapOID
	0x30, 0x19,
	0x06, 0x08, 0x2b, 0x06, 0x01, 0x06, 0x03, 0x01, 0x01, 0x04, // OID 1.3.6.1.6.3.1.1.4
	0x06, 0x0d, 0x2b, 0x06, 0x01, 0x06, 0x03, 0x01, 0x01, 0x05, 0x01, 0x00, 0x00, 0x00, 0x01, // OID value
	// Third varbind: custom data
	0x30, 0x1a,
	0x06, 0x0a, 0x2b, 0x06, 0x01, 0x04, 0x01, 0x94, 0x78, 0x01, 0x01, 0x01, // OID 1.3.6.1.4.1.18872.1.1.1
	0x04, 0x0c, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x20, 0x57, 0x6f, 0x72, 0x6c, 0x64, 0x21, // OCTET STRING "Hello World!"
}

func TestNewSNMPParser(t *testing.T) {
	data := []byte{0x30, 0x0a, 0x02, 0x01, 0x00}
	parser := NewSNMPParser(data)

	if parser == nil {
		t.Fatal("Parser should not be nil")
	}

	if len(parser.data) != len(data) {
		t.Errorf("Expected data length %d, got %d", len(data), len(parser.data))
	}

	if parser.offset != 0 {
		t.Errorf("Expected offset 0, got %d", parser.offset)
	}
}

func TestParseSNMPv1Packet(t *testing.T) {
	parser := NewSNMPParser(snmpv1TrapPacket)
	packet, err := parser.ParseSNMPPacket()

	if err != nil {
		t.Fatalf("Failed to parse SNMPv1 packet: %v", err)
	}

	if packet == nil {
		t.Fatal("Packet should not be nil")
	}

	// Verify basic packet structure
	if packet.Version != types.VersionSNMPv1 {
		t.Errorf("Expected version %d, got %d", types.VersionSNMPv1, packet.Version)
	}

	if packet.Community != "public" {
		t.Errorf("Expected community 'public', got '%s'", packet.Community)
	}

	if packet.PDUType != types.PDUTypeTrap {
		t.Errorf("Expected PDU type %d, got %d", types.PDUTypeTrap, packet.PDUType)
	}

	// Verify v1-specific fields
	if packet.EnterpriseOID != "1.3.6.1.4.1.18872.1" {
		t.Errorf("Expected enterprise OID '1.3.6.1.4.1.18872.1', got '%s'", packet.EnterpriseOID)
	}

	if packet.AgentAddress != "192.168.1.1" {
		t.Errorf("Expected agent address '192.168.1.1', got '%s'", packet.AgentAddress)
	}

	if packet.GenericTrap != types.GenericTrapEnterpriseSpecific {
		t.Errorf("Expected generic trap %d, got %d", types.GenericTrapEnterpriseSpecific, packet.GenericTrap)
	}

	if packet.SpecificTrap != 1 {
		t.Errorf("Expected specific trap 1, got %d", packet.SpecificTrap)
	}

	if packet.Uptime != 300 {
		t.Errorf("Expected uptime 300, got %d", packet.Uptime)
	}

	// Verify varbinds
	if len(packet.Varbinds) != 1 {
		t.Errorf("Expected 1 varbind, got %d", len(packet.Varbinds))
	}

	if len(packet.Varbinds) > 0 {
		vb := packet.Varbinds[0]
		if vb.OID != "1.3.6.1.4.1.18872.1.1.1" {
			t.Errorf("Expected varbind OID '1.3.6.1.4.1.18872.1.1.1', got '%s'", vb.OID)
		}

		if vb.Type != types.TypeOctetString {
			t.Errorf("Expected varbind type %d, got %d", types.TypeOctetString, vb.Type)
		}

		if string(vb.Value.([]byte)) != "Test message!!" {
			t.Errorf("Expected varbind value 'Test message!!', got '%s'", string(vb.Value.([]byte)))
		}
	}
}

func TestParseSNMPv2cPacket(t *testing.T) {
	parser := NewSNMPParser(snmpv2cTrapPacket)
	packet, err := parser.ParseSNMPPacket()

	if err != nil {
		t.Fatalf("Failed to parse SNMPv2c packet: %v", err)
	}

	if packet == nil {
		t.Fatal("Packet should not be nil")
	}

	// Verify basic packet structure
	if packet.Version != types.VersionSNMPv2c {
		t.Errorf("Expected version %d, got %d", types.VersionSNMPv2c, packet.Version)
	}

	if packet.Community != "public" {
		t.Errorf("Expected community 'public', got '%s'", packet.Community)
	}

	if packet.PDUType != types.PDUTypeTrapV2 {
		t.Errorf("Expected PDU type %d, got %d", types.PDUTypeTrapV2, packet.PDUType)
	}

	if packet.RequestID != 0x12345678 {
		t.Errorf("Expected request ID 0x12345678, got 0x%x", packet.RequestID)
	}

	if packet.ErrorStatus != 0 {
		t.Errorf("Expected error status 0, got %d", packet.ErrorStatus)
	}

	if packet.ErrorIndex != 0 {
		t.Errorf("Expected error index 0, got %d", packet.ErrorIndex)
	}

	// Verify varbinds
	if len(packet.Varbinds) != 3 {
		t.Errorf("Expected 3 varbinds, got %d", len(packet.Varbinds))
	}

	// Check first varbind (sysUpTime)
	if len(packet.Varbinds) > 0 {
		vb := packet.Varbinds[0]
		if vb.OID != "1.3.6.1.2.1.1.3.0" {
			t.Errorf("Expected first varbind OID '1.3.6.1.2.1.1.3.0', got '%s'", vb.OID)
		}
		if vb.Type != types.TypeTimeTicks {
			t.Errorf("Expected first varbind type %d, got %d", types.TypeTimeTicks, vb.Type)
		}
	}

	// Check third varbind (custom data)
	if len(packet.Varbinds) > 2 {
		vb := packet.Varbinds[2]
		if vb.OID != "1.3.6.1.4.1.18872.1.1.1" {
			t.Errorf("Expected third varbind OID '1.3.6.1.4.1.18872.1.1.1', got '%s'", vb.OID)
		}
		if vb.Type != types.TypeOctetString {
			t.Errorf("Expected third varbind type %d, got %d", types.TypeOctetString, vb.Type)
		}
		if string(vb.Value.([]byte)) != "Hello World!" {
			t.Errorf("Expected third varbind value 'Hello World!', got '%s'", string(vb.Value.([]byte)))
		}
	}
}

func TestParseInvalidPackets(t *testing.T) {
	testCases := []struct {
		name string
		data []byte
	}{
		{"Empty packet", []byte{}},
		{"Too short", []byte{0x30}},
		{"Invalid tag", []byte{0x31, 0x0a, 0x02, 0x01, 0x00}},
		{"Truncated length", []byte{0x30, 0x82}},
		{"Invalid length", []byte{0x30, 0x82, 0xff, 0xff}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parser := NewSNMPParser(tc.data)
			_, err := parser.ParseSNMPPacket()
			if err == nil {
				t.Errorf("Expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestParseLength(t *testing.T) {
	testCases := []struct {
		name     string
		data     []byte
		expected int
		hasError bool
	}{
		{"Short form 0", []byte{0x00}, 0, false},
		{"Short form 127", []byte{0x7f}, 127, false},
		{"Long form 1 byte", []byte{0x81, 0xff}, 255, false},
		{"Long form 2 bytes", []byte{0x82, 0x01, 0x00}, 256, false},
		{"Indefinite length", []byte{0x80}, 0, true},
		{"Too long", []byte{0x85, 0x01, 0x02, 0x03, 0x04, 0x05}, 0, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parser := NewSNMPParser(tc.data)
			length, err := parser.parseLength()

			if tc.hasError {
				if err == nil {
					t.Errorf("Expected error for %s, got nil", tc.name)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for %s: %v", tc.name, err)
				}
				if length != tc.expected {
					t.Errorf("Expected length %d for %s, got %d", tc.expected, tc.name, length)
				}
			}
		})
	}
}

func TestDecodeObjectIdentifier(t *testing.T) {
	testCases := []struct {
		name     string
		data     []byte
		expected string
		hasError bool
	}{
		{"Simple OID", []byte{0x2b, 0x06, 0x01, 0x02, 0x01, 0x01, 0x01, 0x00}, "1.3.6.1.2.1.1.1.0", false},
		{"Enterprise OID", []byte{0x2b, 0x06, 0x01, 0x04, 0x01, 0x94, 0x78, 0x01}, "1.3.6.1.4.1.18872.1", false},
		{"Empty OID", []byte{}, "", true},
		{"Large sub-identifier", []byte{0x2b, 0x81, 0x00}, "1.3.128", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parser := NewSNMPParser(tc.data)
			oid, err := parser.decodeObjectIdentifier(tc.data)

			if tc.hasError {
				if err == nil {
					t.Errorf("Expected error for %s, got nil", tc.name)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for %s: %v", tc.name, err)
				}
				if oid != tc.expected {
					t.Errorf("Expected OID '%s' for %s, got '%s'", tc.expected, tc.name, oid)
				}
			}
		})
	}
}

func TestParseValue(t *testing.T) {
	testCases := []struct {
		name         string
		data         []byte
		expectedType int
		hasError     bool
	}{
		{"Integer 0", []byte{0x02, 0x01, 0x00}, types.TypeInteger, false},
		{"Integer 255", []byte{0x02, 0x01, 0xff}, types.TypeInteger, false},
		{"Octet string", []byte{0x04, 0x05, 0x48, 0x65, 0x6c, 0x6c, 0x6f}, types.TypeOctetString, false},
		{"Null", []byte{0x05, 0x00}, types.TypeNull, false},
		{"IP Address", []byte{0x40, 0x04, 0xc0, 0xa8, 0x01, 0x01}, types.TypeIPAddress, false},
		{"Counter32", []byte{0x41, 0x04, 0x00, 0x00, 0x01, 0x2c}, types.TypeCounter32, false},
		{"Invalid IP length", []byte{0x40, 0x03, 0xc0, 0xa8, 0x01}, 0, true},
		{"Truncated value", []byte{0x02, 0x05, 0x01}, 0, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parser := NewSNMPParser(tc.data)
			_, valueType, err := parser.parseValue()

			if tc.hasError {
				if err == nil {
					t.Errorf("Expected error for %s, got nil", tc.name)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for %s: %v", tc.name, err)
				}
				if valueType != tc.expectedType {
					t.Errorf("Expected type %d for %s, got %d", tc.expectedType, tc.name, valueType)
				}
			}
		})
	}
}
