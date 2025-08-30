package templates

// DefaultTemplate defines the default notification template
#DefaultTemplate: {
	// Template name
	name: "default"

	// Template description
	description: "Default JSON notification template for SNMP traps"

	// Template content type
	content_type: "application/json"

	// Template body using Go template syntax
	template: """
		{
			"event_id": "{{.ID}}",
			"timestamp": "{{.Timestamp}}",
			"source_ip": "{{.SourceIP}}",
			"trap_oid": "{{.TrapOID}}",
			"trap_name": "{{.TrapName}}",
			"severity": "{{.Severity}}",
			"status": "{{.Status}}",
			"message": "SNMP trap received from {{.SourceIP}}: {{.TrapName}} ({{.TrapOID}})",
			"varbinds": {{.VarbindsJSON}},
			"metadata": {
				"community": "{{.Community}}",
				"version": {{.Version}},
				"pdu_type": {{.PDUType}},
				"request_id": {{.RequestID}},
				"count": {{.Count}},
				"first_seen": "{{.FirstSeen}}",
				"last_seen": "{{.LastSeen}}",
				"acknowledged": {{.Acknowledged}},
				"correlation_id": "{{.CorrelationID}}"
			}
		}
		"""

	// Template validation schema
	validation: {
		// Required fields that must be present in the event data
		required_fields: [
			"ID",
			"Timestamp",
			"SourceIP",
			"TrapOID",
			"Severity",
			"Status",
		]

		// Optional fields that may be present
		optional_fields: [
			"TrapName",
			"VarbindsJSON",
			"Community",
			"Version",
			"PDUType",
			"RequestID",
			"Count",
			"FirstSeen",
			"LastSeen",
			"Acknowledged",
			"CorrelationID",
		]
	}


}
