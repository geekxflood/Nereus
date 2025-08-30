// Example custom CUE template for Nereus notifications
// This shows how to create a custom notification template following the CUE pattern

package templates

// CustomTemplate defines a custom notification template
#CustomTemplate: {
	// Template name
	name: "custom"
	
	// Template description
	description: "Custom notification template example"
	
	// Template content type
	content_type: "application/json"
	
	// Template body using Go template syntax
	template: """
		{
			"alert": {
				"id": "{{.ID}}",
				"timestamp": "{{.Timestamp}}",
				"source": {
					"ip": "{{.SourceIP}}",
					"community": "{{.Community}}"
				},
				"trap": {
					"oid": "{{.TrapOID}}",
					"name": "{{.TrapName}}",
					"severity": "{{.Severity}}",
					"status": "{{.Status}}"
				},
				"details": {
					"count": {{.Count}},
					"first_seen": "{{.FirstSeen}}",
					"last_seen": "{{.LastSeen}}",
					"acknowledged": {{.Acknowledged}}{{if .CorrelationID}},
					"correlation_id": "{{.CorrelationID}}"{{end}}
				}{{if .VarbindsJSON}},
				"varbinds": {{.VarbindsJSON}}{{end}}{{if .Metadata}},
				"metadata": {{.Metadata}}{{end}}
			},
			"notification": {
				"priority": "{{.CustomPriority}}",
				"category": "{{.CustomCategory}}",
				"environment": "{{.Environment}}",
				"generated_at": "{{.Timestamp}}",
				"generator": "nereus-snmp-listener"
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
			"Status"
		]
		
		// Optional fields that may be present
		optional_fields: [
			"Community",
			"TrapName",
			"Count",
			"FirstSeen",
			"LastSeen",
			"Acknowledged",
			"CorrelationID",
			"VarbindsJSON",
			"Metadata",
			"CustomPriority",
			"CustomCategory",
			"Environment"
		]
		
		// Custom validation rules
		custom_rules: {
			// Priority levels
			valid_priorities: ["low", "medium", "high", "critical"]
			
			// Categories
			valid_categories: ["network", "system", "application", "security"]
			
			// Environments
			valid_environments: ["development", "staging", "production"]
		}
	}
	
	// Custom field mappings
	field_mappings: {
		// Map SNMP severity to custom priority
		severity_to_priority: {
			"emergency":     "critical"
			"alert":         "critical"
			"critical":      "critical"
			"error":         "high"
			"major":         "high"
			"warning":       "medium"
			"minor":         "medium"
			"notice":        "low"
			"informational": "low"
			"info":          "low"
			"debug":         "low"
		}
		
		// Map trap OID patterns to categories
		oid_to_category: {
			"1.3.6.1.6.3.1.1.5.*":     "network"    // Standard SNMP traps
			"1.3.6.1.4.1.2021.*":      "system"     // Net-SNMP system traps
			"1.3.6.1.4.1.9.*":         "network"    // Cisco traps
			"1.3.6.1.4.1.311.*":       "system"     // Microsoft traps
		}
	}
	
	// Template usage examples
	examples: [
		{
			description: "High priority network alert"
			input: {
				ID: 123
				Timestamp: "2024-01-15T10:30:00Z"
				SourceIP: "192.168.1.100"
				Community: "public"
				TrapOID: "1.3.6.1.6.3.1.1.5.3"
				TrapName: "linkDown"
				Severity: "major"
				Status: "firing"
				Count: 1
				FirstSeen: "2024-01-15T10:30:00Z"
				LastSeen: "2024-01-15T10:30:00Z"
				Acknowledged: false
				CustomPriority: "high"
				CustomCategory: "network"
				Environment: "production"
			}
			expected_priority: "high"
			expected_category: "network"
		},
		{
			description: "Low priority informational alert"
			input: {
				ID: 124
				Timestamp: "2024-01-15T10:35:00Z"
				SourceIP: "192.168.1.50"
				TrapOID: "1.3.6.1.4.1.2021.251.1"
				TrapName: "systemRestart"
				Severity: "info"
				Status: "resolved"
				CustomPriority: "low"
				CustomCategory: "system"
				Environment: "development"
			}
			expected_priority: "low"
			expected_category: "system"
		}
	]
}

// Instructions for using custom templates:
//
// 1. Create your custom template CUE file following the #TemplateBase structure
// 2. Place it in the internal/notifier/templates/ directory
// 3. The template will be automatically loaded by the TemplateManager
// 4. Reference it in your webhook configuration using the template name
//
// Example webhook configuration:
// ```yaml
// notifier:
//   default_webhooks:
//     - name: "custom-webhook"
//       url: "https://api.example.com/alerts"
//       template: "custom"  # References this template
//       format: "custom"
// ```
//
// Template Development Tips:
//
// 1. Use CUE validation to ensure template correctness
// 2. Define required and optional fields clearly
// 3. Provide usage examples for documentation
// 4. Use consistent field naming conventions
// 5. Include proper error handling in templates
// 6. Test templates with various event scenarios
//
// Advanced Features:
//
// 1. Field Mappings: Define custom mappings for severity, categories, etc.
// 2. Validation Rules: Add custom validation beyond required/optional fields
// 3. Helper Functions: Use template helper functions for formatting
// 4. Conditional Logic: Use Go template conditionals for dynamic content
// 5. JSON Escaping: Ensure proper JSON escaping for nested data
//
// Performance Considerations:
//
// 1. Templates are compiled once at startup for better performance
// 2. Use template caching to avoid recompilation
// 3. Keep templates simple to reduce rendering time
// 4. Validate templates during development, not at runtime
// 5. Use efficient Go template syntax and avoid complex logic
