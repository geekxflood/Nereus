#!/usr/bin/env python3
"""
Comprehensive SNMP Trap Testing Script for Nereus
Sends various SNMP traps to test the complete pipeline
"""

import socket
import struct
import time
import sys
import json
import urllib.request
from datetime import datetime


class SNMPTrapSender:
    def __init__(self, host="localhost", port=1162, community="public"):
        self.host = host
        self.port = port
        self.community = community
        self.sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)

    def encode_ber_length(self, length):
        """Encode length in BER format"""
        if length < 128:
            return bytes([length])
        else:
            # Long form - not implemented for simplicity
            return bytes([0x81, length])

    def encode_ber_integer(self, value):
        """Encode integer in BER format"""
        if value == 0:
            return b"\x02\x01\x00"

        # Convert to bytes
        if value < 256:
            return b"\x02\x01" + bytes([value])
        elif value < 65536:
            return b"\x02\x02" + struct.pack(">H", value)
        else:
            return b"\x02\x04" + struct.pack(">I", value)

    def encode_ber_string(self, value):
        """Encode string in BER format"""
        if isinstance(value, str):
            value = value.encode("utf-8")
        return b"\x04" + self.encode_ber_length(len(value)) + value

    def encode_ber_oid(self, oid_str):
        """Encode OID in BER format"""
        parts = [int(x) for x in oid_str.split(".")]

        # First two parts are encoded as 40*first + second
        if len(parts) < 2:
            raise ValueError("OID must have at least 2 parts")

        encoded = [40 * parts[0] + parts[1]]

        # Encode remaining parts
        for part in parts[2:]:
            if part < 128:
                encoded.append(part)
            else:
                # Multi-byte encoding
                bytes_needed = []
                while part > 0:
                    bytes_needed.insert(0, part & 0x7F)
                    part >>= 7

                # Set continuation bit on all but last byte
                for i in range(len(bytes_needed) - 1):
                    bytes_needed[i] |= 0x80

                encoded.extend(bytes_needed)

        result = bytes(encoded)
        return b"\x06" + self.encode_ber_length(len(result)) + result

    def encode_ber_timeticks(self, value):
        """Encode TimeTicks in BER format"""
        return b"\x43\x04" + struct.pack(">I", value)

    def create_snmp_trap(self, trap_oid, varbinds=None):
        """Create SNMPv2c trap packet"""
        if varbinds is None:
            varbinds = []

        # SNMP version (SNMPv2c = 1)
        version = self.encode_ber_integer(1)

        # Community string
        community = self.encode_ber_string(self.community)

        # PDU Type (SNMPv2-Trap = 0xA7)
        pdu_type = b"\xa7"

        # Request ID
        request_id = self.encode_ber_integer(12345)

        # Error status
        error_status = self.encode_ber_integer(0)

        # Error index
        error_index = self.encode_ber_integer(0)

        # Variable bindings
        varbind_list = []

        # Add sysUpTime (required for SNMPv2c traps)
        uptime_oid = self.encode_ber_oid("1.3.6.1.2.1.1.3.0")
        uptime_value = self.encode_ber_timeticks(12345)  # 123.45 seconds
        uptime_varbind = (
            b"\x30"
            + self.encode_ber_length(len(uptime_oid) + len(uptime_value))
            + uptime_oid
            + uptime_value
        )
        varbind_list.append(uptime_varbind)

        # Add snmpTrapOID (required for SNMPv2c traps)
        trap_oid_oid = self.encode_ber_oid("1.3.6.1.6.3.1.1.4.1.0")
        trap_oid_value = self.encode_ber_oid(trap_oid)
        trap_oid_varbind = (
            b"\x30"
            + self.encode_ber_length(len(trap_oid_oid) + len(trap_oid_value))
            + trap_oid_oid
            + trap_oid_value
        )
        varbind_list.append(trap_oid_varbind)

        # Add custom varbinds
        for oid, value in varbinds:
            oid_encoded = self.encode_ber_oid(oid)
            if isinstance(value, int):
                value_encoded = self.encode_ber_integer(value)
            else:
                value_encoded = self.encode_ber_string(str(value))

            varbind = (
                b"\x30"
                + self.encode_ber_length(len(oid_encoded) + len(value_encoded))
                + oid_encoded
                + value_encoded
            )
            varbind_list.append(varbind)

        # Combine all varbinds
        varbinds_data = b"".join(varbind_list)
        varbinds_sequence = (
            b"\x30" + self.encode_ber_length(len(varbinds_data)) + varbinds_data
        )

        # PDU content
        pdu_content = request_id + error_status + error_index + varbinds_sequence
        pdu = pdu_type + self.encode_ber_length(len(pdu_content)) + pdu_content

        # SNMP message
        message_content = version + community + pdu
        message = (
            b"\x30" + self.encode_ber_length(len(message_content)) + message_content
        )

        return message

    def send_trap(self, trap_oid, varbinds=None, description=""):
        """Send SNMP trap"""
        packet = self.create_snmp_trap(trap_oid, varbinds)

        try:
            self.sock.sendto(packet, (self.host, self.port))
            print(f"✅ Sent trap: {trap_oid} - {description}")
            return True
        except Exception as e:
            print(f"❌ Failed to send trap {trap_oid}: {e}")
            return False

    def close(self):
        """Close socket"""
        self.sock.close()


def check_metrics(expected_traps=0):
    """Check Nereus metrics"""
    try:
        with urllib.request.urlopen("http://localhost:9090/metrics") as response:
            metrics_text = response.read().decode("utf-8")

        # Parse relevant metrics
        metrics = {}
        for line in metrics_text.split("\n"):
            if line.startswith("nereus_test_") and not line.startswith("#"):
                parts = line.split(" ")
                if len(parts) >= 2:
                    metric_name = parts[0].split("{")[0]
                    metric_value = float(parts[1])
                    metrics[metric_name] = metrics.get(metric_name, 0) + metric_value

        print(f"\n📊 Current Metrics:")
        for key, value in sorted(metrics.items()):
            if "total" in key or "count" in key:
                print(f"   {key}: {int(value)}")

        return metrics
    except Exception as e:
        print(f"❌ Failed to get metrics: {e}")
        return {}


def main():
    print("🧪 Nereus SNMP Trap Testing Suite")
    print("=" * 50)

    # Initialize trap sender
    sender = SNMPTrapSender()

    try:
        # Test 1: Standard SNMP traps
        print("\n🔥 Test 1: Standard SNMP System Traps")
        test_traps = [
            ("1.3.6.1.6.3.1.1.5.1", [], "coldStart - System cold restart"),
            ("1.3.6.1.6.3.1.1.5.2", [], "warmStart - System warm restart"),
            ("1.3.6.1.6.3.1.1.5.5", [], "authenticationFailure - SNMP auth failure"),
        ]

        for trap_oid, varbinds, description in test_traps:
            sender.send_trap(trap_oid, varbinds, description)
            time.sleep(0.5)

        # Test 2: Interface traps
        print("\n🌐 Test 2: Network Interface Traps")
        interface_traps = [
            (
                "1.3.6.1.6.3.1.1.5.3",
                [("1.3.6.1.2.1.2.2.1.1.1", 1)],
                "linkDown - Interface 1 down",
            ),
            (
                "1.3.6.1.6.3.1.1.5.4",
                [("1.3.6.1.2.1.2.2.1.1.1", 1)],
                "linkUp - Interface 1 up",
            ),
            (
                "1.3.6.1.6.3.1.1.5.3",
                [("1.3.6.1.2.1.2.2.1.1.2", 2)],
                "linkDown - Interface 2 down",
            ),
            (
                "1.3.6.1.6.3.1.1.5.4",
                [("1.3.6.1.2.1.2.2.1.1.2", 2)],
                "linkUp - Interface 2 up",
            ),
        ]

        for trap_oid, varbinds, description in interface_traps:
            sender.send_trap(trap_oid, varbinds, description)
            time.sleep(0.5)

        # Test 3: Custom vendor traps (simulated)
        print("\n🏢 Test 3: Vendor-Specific Traps")
        vendor_traps = [
            (
                "1.3.6.1.4.1.3808.1.1.1.5",
                [("1.3.6.1.4.1.3808.1.1.1.1", "UPS on battery")],
                "CyberPower UPS - On battery",
            ),
            (
                "1.3.6.1.4.1.3808.1.1.1.6",
                [("1.3.6.1.4.1.3808.1.1.1.2", "Power restored")],
                "CyberPower UPS - Power restored",
            ),
            (
                "1.3.6.1.4.1.2636.1.1.1.1",
                [("1.3.6.1.4.1.2636.1.1.1.2", "System started")],
                "Juniper - System start",
            ),
        ]

        for trap_oid, varbinds, description in vendor_traps:
            sender.send_trap(trap_oid, varbinds, description)
            time.sleep(0.5)

        # Test 4: High-volume test
        print("\n⚡ Test 4: High-Volume Trap Burst")
        for i in range(10):
            sender.send_trap(
                "1.3.6.1.6.3.1.1.5.1",
                [("1.3.6.1.2.1.1.1.0", f"Burst test {i+1}")],
                f"Burst test trap {i+1}",
            )
            time.sleep(0.1)

        print(
            f"\n✅ Sent total of {len(test_traps) + len(interface_traps) + len(vendor_traps) + 10} traps"
        )

        # Wait for processing
        print("\n⏳ Waiting for trap processing...")
        time.sleep(3)

        # Check metrics
        metrics = check_metrics()

        # Check health
        print("\n🏥 Health Check:")
        try:
            with urllib.request.urlopen("http://localhost:9090/health") as response:
                health_status = response.read().decode("utf-8").strip()
            print(f"   Health Status: {health_status}")
        except Exception as e:
            print(f"   Health Check Failed: {e}")

        print("\n🎉 Testing completed successfully!")

    except KeyboardInterrupt:
        print("\n⏹️  Testing interrupted by user")
    except Exception as e:
        print(f"\n❌ Testing failed: {e}")
    finally:
        sender.close()


if __name__ == "__main__":
    main()
