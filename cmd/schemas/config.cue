package schemas

// Config defines the complete configuration schema for nereus
#Config: {
	// Server configuration for SNMP trap listener
	server: #Server

	// MIB configuration for OID parsing
	mibs: #MIBs

	// Webhook configurations for alert notifications
	webhooks: [...#Webhook]

	// Logging configuration
	logging: #Logging
}

// Server defines the SNMP trap listener configuration
#Server: {
	// Host to bind the SNMP trap listener to
	host: string | *"0.0.0.0"

	// Port to listen on for SNMP traps
	port: int & >=1 & <=65535 | *162

	// SNMP community string for trap authentication
	community: string | *"public"

	// Maximum number of concurrent trap handlers
	max_handlers?: int & >=1 | *100

	// Read timeout for UDP connections
	read_timeout?: string | *"30s"

	// Buffer size for UDP packets
	buffer_size?: int & >=512 | *8192
}

// MIBs defines the MIB file configuration
#MIBs: {
	// Path to the directory containing MIB files
	path: string

	// Whether to automatically load all MIB files on startup
	auto_load: bool | *true

	// Specific MIB files to load (if auto_load is false)
	files?: [...string]

	// Whether to recursively search subdirectories
	recursive?: bool | *true

	// Cache parsed MIB data for faster startup
	cache_enabled?: bool | *true

	// Cache file path
	cache_path?: string | *"/tmp/nereus_mib_cache.json"
}

// Webhook defines a webhook endpoint configuration
#Webhook: {
	// Unique name for the webhook
	name: string

	// URL endpoint to send webhook notifications
	url: string & =~"^https?://.+"

	// HTTP timeout for webhook requests
	timeout: string | *"30s"

	// Number of retry attempts on failure
	retry_count: int & >=0 | *3

	// Delay between retry attempts
	retry_delay?: string | *"5s"

	// HTTP headers to include in webhook requests
	headers?: [string]: string

	// Template for webhook payload (optional)
	template?: string

	// Whether this webhook is enabled
	enabled?: bool | *true

	// Filter conditions for when to trigger this webhook
	filters?: #WebhookFilters
}

// WebhookFilters defines conditions for webhook triggering
#WebhookFilters: {
	// Only trigger for specific OIDs
	oids?: [...string]

	// Only trigger for specific severity levels
	severity?: [...string]

	// Only trigger for specific source IPs
	sources?: [...string]

	// Exclude specific OIDs
	exclude_oids?: [...string]
}

// Logging defines the logging configuration
#Logging: {
	// Log level (debug, info, warn, error)
	level: "debug" | "info" | "warn" | "error" | *"info"

	// Log format (json, text)
	format: "json" | "text" | *"json"

	// Component name for structured logging
	component: string | *"nereus"

	// Whether to log to stdout
	stdout?: bool | *true

	// File path for log output (optional)
	file?: string

	// Maximum log file size before rotation
	max_size?: string | *"100MB"

	// Number of old log files to retain
	max_backups?: int & >=0 | *5

	// Maximum age of log files before deletion
	max_age?: string | *"30d"
}
