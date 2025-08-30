// Package parser provides SNMP trap parsing functionality.
package parser

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"

	"github.com/geekxflood/nereus/internal/types"
)

// ASN.1 BER/DER tag constants
const (
	tagBoolean          = 0x01
	tagInteger          = 0x02
	tagBitString        = 0x03
	tagOctetString      = 0x04
	tagNull             = 0x05
	tagObjectIdentifier = 0x06
	tagSequence         = 0x30
	tagSet              = 0x31
	tagIPAddress        = 0x40 // Application tag 0
	tagCounter32        = 0x41 // Application tag 1
	tagGauge32          = 0x42 // Application tag 2
	tagTimeTicks        = 0x43 // Application tag 3
	tagOpaque           = 0x44 // Application tag 4
	tagCounter64        = 0x46 // Application tag 6
)

// SNMP PDU context-specific tags
const (
	tagGetRequest     = 0xA0
	tagGetNextRequest = 0xA1
	tagGetResponse    = 0xA2
	tagSetRequest     = 0xA3
	tagTrap           = 0xA4
	tagGetBulkRequest = 0xA5
	tagInformRequest  = 0xA6
	tagTrapV2         = 0xA7
	tagReport         = 0xA8
)

// SNMPParser handles parsing of SNMP packets
type SNMPParser struct {
	data   []byte
	offset int
}

// NewSNMPParser creates a new SNMP parser for the given data
func NewSNMPParser(data []byte) *SNMPParser {
	return &SNMPParser{
		data:   data,
		offset: 0,
	}
}

// ParseSNMPPacket parses an SNMP packet and returns the structured data
func (p *SNMPParser) ParseSNMPPacket() (*types.SNMPPacket, error) {
	// Reset parser state
	p.offset = 0

	// Parse the outer sequence
	if err := p.expectTag(tagSequence); err != nil {
		return nil, fmt.Errorf("expected SNMP sequence: %w", err)
	}

	length, err := p.parseLength()
	if err != nil {
		return nil, fmt.Errorf("failed to parse sequence length: %w", err)
	}

	if p.offset+length > len(p.data) {
		return nil, fmt.Errorf("sequence length exceeds packet size")
	}

	// Parse SNMP version
	version, err := p.parseInteger()
	if err != nil {
		return nil, fmt.Errorf("failed to parse SNMP version: %w", err)
	}

	// Parse community string
	community, err := p.parseOctetString()
	if err != nil {
		return nil, fmt.Errorf("failed to parse community string: %w", err)
	}

	// Parse PDU based on version
	var packet *types.SNMPPacket
	switch version {
	case types.VersionSNMPv1:
		packet, err = p.parseSNMPv1PDU()
	case types.VersionSNMPv2c:
		packet, err = p.parseSNMPv2cPDU()
	default:
		return nil, fmt.Errorf("unsupported SNMP version: %d", version)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to parse PDU: %w", err)
	}

	packet.Version = int(version)
	packet.Community = string(community)
	packet.Timestamp = time.Now()

	return packet, nil
}

// parseSNMPv1PDU parses an SNMP v1 PDU
func (p *SNMPParser) parseSNMPv1PDU() (*types.SNMPPacket, error) {
	// Check PDU type
	if p.offset >= len(p.data) {
		return nil, fmt.Errorf("unexpected end of data")
	}

	pduType := p.data[p.offset]
	if pduType != tagTrap {
		return nil, fmt.Errorf("expected trap PDU, got 0x%02x", pduType)
	}

	p.offset++

	// Parse PDU length
	_, err := p.parseLength()
	if err != nil {
		return nil, fmt.Errorf("failed to parse PDU length: %w", err)
	}

	// Parse enterprise OID
	enterpriseOID, err := p.parseObjectIdentifier()
	if err != nil {
		return nil, fmt.Errorf("failed to parse enterprise OID: %w", err)
	}

	// Parse agent address
	agentAddr, err := p.parseIPAddress()
	if err != nil {
		return nil, fmt.Errorf("failed to parse agent address: %w", err)
	}

	// Parse generic trap type
	genericTrap, err := p.parseInteger()
	if err != nil {
		return nil, fmt.Errorf("failed to parse generic trap: %w", err)
	}

	// Parse specific trap type
	specificTrap, err := p.parseInteger()
	if err != nil {
		return nil, fmt.Errorf("failed to parse specific trap: %w", err)
	}

	// Parse timestamp
	timestamp, err := p.parseTimeTicks()
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp: %w", err)
	}

	// Parse varbind list
	varbinds, err := p.parseVarbindList()
	if err != nil {
		return nil, fmt.Errorf("failed to parse varbinds: %w", err)
	}

	return &types.SNMPPacket{
		PDUType:       types.PDUTypeTrap,
		RequestID:     0, // Not used in v1 traps
		Varbinds:      varbinds,
		EnterpriseOID: enterpriseOID,
		AgentAddress:  agentAddr,
		GenericTrap:   int(genericTrap),
		SpecificTrap:  int(specificTrap),
		Uptime:        uint32(timestamp),
	}, nil
}

// parseSNMPv2cPDU parses an SNMP v2c PDU
func (p *SNMPParser) parseSNMPv2cPDU() (*types.SNMPPacket, error) {
	// Check PDU type
	if p.offset >= len(p.data) {
		return nil, fmt.Errorf("unexpected end of data")
	}

	pduType := p.data[p.offset]
	if pduType != tagTrapV2 {
		return nil, fmt.Errorf("expected trap v2 PDU, got 0x%02x", pduType)
	}

	p.offset++

	// Parse PDU length
	_, err := p.parseLength()
	if err != nil {
		return nil, fmt.Errorf("failed to parse PDU length: %w", err)
	}

	// Parse request ID
	requestID, err := p.parseInteger()
	if err != nil {
		return nil, fmt.Errorf("failed to parse request ID: %w", err)
	}

	// Parse error status
	errorStatus, err := p.parseInteger()
	if err != nil {
		return nil, fmt.Errorf("failed to parse error status: %w", err)
	}

	// Parse error index
	errorIndex, err := p.parseInteger()
	if err != nil {
		return nil, fmt.Errorf("failed to parse error index: %w", err)
	}

	// Parse varbind list
	varbinds, err := p.parseVarbindList()
	if err != nil {
		return nil, fmt.Errorf("failed to parse varbinds: %w", err)
	}

	return &types.SNMPPacket{
		PDUType:     types.PDUTypeTrapV2,
		RequestID:   int32(requestID),
		ErrorStatus: int(errorStatus),
		ErrorIndex:  int(errorIndex),
		Varbinds:    varbinds,
	}, nil
}

// parseVarbindList parses a sequence of varbinds
func (p *SNMPParser) parseVarbindList() ([]types.Varbind, error) {
	// Expect sequence tag
	if err := p.expectTag(tagSequence); err != nil {
		return nil, fmt.Errorf("expected varbind sequence: %w", err)
	}

	length, err := p.parseLength()
	if err != nil {
		return nil, fmt.Errorf("failed to parse varbind sequence length: %w", err)
	}

	endOffset := p.offset + length
	var varbinds []types.Varbind

	for p.offset < endOffset {
		varbind, err := p.parseVarbind()
		if err != nil {
			return nil, fmt.Errorf("failed to parse varbind: %w", err)
		}
		varbinds = append(varbinds, varbind)
	}

	return varbinds, nil
}

// parseVarbind parses a single varbind
func (p *SNMPParser) parseVarbind() (types.Varbind, error) {
	// Expect sequence tag
	if err := p.expectTag(tagSequence); err != nil {
		return types.Varbind{}, fmt.Errorf("expected varbind sequence: %w", err)
	}

	_, err := p.parseLength()
	if err != nil {
		return types.Varbind{}, fmt.Errorf("failed to parse varbind length: %w", err)
	}

	// Parse OID
	oid, err := p.parseObjectIdentifier()
	if err != nil {
		return types.Varbind{}, fmt.Errorf("failed to parse varbind OID: %w", err)
	}

	// Parse value
	value, valueType, err := p.parseValue()
	if err != nil {
		return types.Varbind{}, fmt.Errorf("failed to parse varbind value: %w", err)
	}

	return types.Varbind{
		OID:   oid,
		Type:  valueType,
		Value: value,
	}, nil
}

// parseValue parses an SNMP value and returns the value, type, and error
func (p *SNMPParser) parseValue() (interface{}, int, error) {
	if p.offset >= len(p.data) {
		return nil, 0, fmt.Errorf("unexpected end of data")
	}

	tag := p.data[p.offset]
	p.offset++

	length, err := p.parseLength()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to parse value length: %w", err)
	}

	if p.offset+length > len(p.data) {
		return nil, 0, fmt.Errorf("value length exceeds packet size")
	}

	valueData := p.data[p.offset : p.offset+length]
	p.offset += length

	switch tag {
	case tagInteger:
		if length == 0 {
			return int64(0), types.TypeInteger, nil
		}
		value := int64(0)
		for _, b := range valueData {
			value = (value << 8) | int64(b)
		}
		// Handle negative numbers (two's complement)
		if valueData[0]&0x80 != 0 && length < 8 {
			for i := length; i < 8; i++ {
				value |= int64(0xFF) << (i * 8)
			}
		}
		return value, types.TypeInteger, nil

	case tagOctetString:
		return valueData, types.TypeOctetString, nil

	case tagNull:
		return nil, types.TypeNull, nil

	case tagObjectIdentifier:
		oid, err := p.decodeObjectIdentifier(valueData)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to decode OID: %w", err)
		}
		return oid, types.TypeObjectIdentifier, nil

	case tagIPAddress:
		if length != 4 {
			return nil, 0, fmt.Errorf("invalid IP address length: %d", length)
		}
		ip := net.IPv4(valueData[0], valueData[1], valueData[2], valueData[3])
		return ip.String(), types.TypeIPAddress, nil

	case tagCounter32, tagGauge32, tagTimeTicks:
		if length == 0 {
			return uint32(0), int(tag), nil
		}
		value := uint32(0)
		for _, b := range valueData {
			value = (value << 8) | uint32(b)
		}
		return value, int(tag), nil

	case tagCounter64:
		if length == 0 {
			return uint64(0), types.TypeCounter64, nil
		}
		value := uint64(0)
		for _, b := range valueData {
			value = (value << 8) | uint64(b)
		}
		return value, types.TypeCounter64, nil

	default:
		// Return raw bytes for unknown types
		return valueData, int(tag), nil
	}
}

// Helper methods for parsing basic ASN.1 types

// expectTag checks if the current byte matches the expected tag
func (p *SNMPParser) expectTag(expectedTag byte) error {
	if p.offset >= len(p.data) {
		return fmt.Errorf("unexpected end of data")
	}

	if p.data[p.offset] != expectedTag {
		return fmt.Errorf("expected tag 0x%02x, got 0x%02x", expectedTag, p.data[p.offset])
	}

	p.offset++
	return nil
}

// parseLength parses ASN.1 BER/DER length encoding
func (p *SNMPParser) parseLength() (int, error) {
	if p.offset >= len(p.data) {
		return 0, fmt.Errorf("unexpected end of data")
	}

	firstByte := p.data[p.offset]
	p.offset++

	// Short form (length < 128)
	if firstByte&0x80 == 0 {
		return int(firstByte), nil
	}

	// Long form
	lengthBytes := int(firstByte & 0x7F)
	if lengthBytes == 0 {
		return 0, fmt.Errorf("indefinite length not supported")
	}

	if lengthBytes > 4 {
		return 0, fmt.Errorf("length too long: %d bytes", lengthBytes)
	}

	if p.offset+lengthBytes > len(p.data) {
		return 0, fmt.Errorf("length bytes exceed packet size")
	}

	length := 0
	for i := 0; i < lengthBytes; i++ {
		length = (length << 8) | int(p.data[p.offset])
		p.offset++
	}

	return length, nil
}

// parseInteger parses an ASN.1 INTEGER
func (p *SNMPParser) parseInteger() (int64, error) {
	if err := p.expectTag(tagInteger); err != nil {
		return 0, err
	}

	length, err := p.parseLength()
	if err != nil {
		return 0, err
	}

	if length == 0 {
		return 0, nil
	}

	if p.offset+length > len(p.data) {
		return 0, fmt.Errorf("integer length exceeds packet size")
	}

	value := int64(0)
	for i := 0; i < length; i++ {
		value = (value << 8) | int64(p.data[p.offset])
		p.offset++
	}

	// Handle negative numbers (two's complement)
	if length > 0 && p.data[p.offset-length]&0x80 != 0 && length < 8 {
		for i := length; i < 8; i++ {
			value |= int64(0xFF) << (i * 8)
		}
	}

	return value, nil
}

// parseOctetString parses an ASN.1 OCTET STRING
func (p *SNMPParser) parseOctetString() ([]byte, error) {
	if err := p.expectTag(tagOctetString); err != nil {
		return nil, err
	}

	length, err := p.parseLength()
	if err != nil {
		return nil, err
	}

	if p.offset+length > len(p.data) {
		return nil, fmt.Errorf("octet string length exceeds packet size")
	}

	value := make([]byte, length)
	copy(value, p.data[p.offset:p.offset+length])
	p.offset += length

	return value, nil
}

// parseObjectIdentifier parses an ASN.1 OBJECT IDENTIFIER
func (p *SNMPParser) parseObjectIdentifier() (string, error) {
	if err := p.expectTag(tagObjectIdentifier); err != nil {
		return "", err
	}

	length, err := p.parseLength()
	if err != nil {
		return "", err
	}

	if p.offset+length > len(p.data) {
		return "", fmt.Errorf("OID length exceeds packet size")
	}

	oidData := p.data[p.offset : p.offset+length]
	p.offset += length

	return p.decodeObjectIdentifier(oidData)
}

// decodeObjectIdentifier decodes OID bytes into dotted notation
func (p *SNMPParser) decodeObjectIdentifier(data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("empty OID")
	}

	// First byte encodes first two sub-identifiers
	firstByte := data[0]
	first := firstByte / 40
	second := firstByte % 40

	oid := fmt.Sprintf("%d.%d", first, second)

	i := 1
	for i < len(data) {
		value := uint64(0)
		for i < len(data) && data[i]&0x80 != 0 {
			value = (value << 7) | uint64(data[i]&0x7F)
			i++
		}
		if i < len(data) {
			value = (value << 7) | uint64(data[i]&0x7F)
			i++
		}
		oid += fmt.Sprintf(".%d", value)
	}

	return oid, nil
}

// parseIPAddress parses an SNMP IpAddress type
func (p *SNMPParser) parseIPAddress() (string, error) {
	if err := p.expectTag(tagIPAddress); err != nil {
		return "", err
	}

	length, err := p.parseLength()
	if err != nil {
		return "", err
	}

	if length != 4 {
		return "", fmt.Errorf("invalid IP address length: %d", length)
	}

	if p.offset+4 > len(p.data) {
		return "", fmt.Errorf("IP address exceeds packet size")
	}

	ip := net.IPv4(p.data[p.offset], p.data[p.offset+1], p.data[p.offset+2], p.data[p.offset+3])
	p.offset += 4

	return ip.String(), nil
}

// parseTimeTicks parses an SNMP TimeTicks type
func (p *SNMPParser) parseTimeTicks() (uint32, error) {
	if err := p.expectTag(tagTimeTicks); err != nil {
		return 0, err
	}

	length, err := p.parseLength()
	if err != nil {
		return 0, err
	}

	if length == 0 {
		return 0, nil
	}

	if p.offset+length > len(p.data) {
		return 0, fmt.Errorf("timeticks length exceeds packet size")
	}

	value := uint32(0)
	for i := 0; i < length; i++ {
		value = (value << 8) | uint32(p.data[p.offset])
		p.offset++
	}

	return value, nil
}
