package schemas

// Config defines the complete configuration schema for nereus
#Config: {
	// Application configuration
	app?: #App

	// SNMP Listener configuration
	listener?: #Listener

	// Storage configuration
	storage?: #Storage

	// Event correlation and deduplication
	correlator?: #Correlator

	// Notification configuration
	notifier?: #Notifier

	// MIB configuration for OID parsing
	mib?: #MIB

	// OID resolution configuration
	resolver?: #Resolver

	// HTTP client configuration
	client?: #Client

	// Health check configuration
	health_check?: #HealthCheck

	// Metrics configuration
	metrics?: #Metrics

	// Legacy configurations (for backward compatibility)
	server?: #Server
	mibs?:   #MIBs
	webhooks?: [...#Webhook]
	logging?: #Logging
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

	// Whether to skip TLS verification (insecure)
	insecure: bool | *false

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

// App defines the application configuration
#App: {
	// Application name
	name?: string | *"nereus-snmp-listener"

	// Application version
	version?: string | *"1.0.0"

	// Log level
	log_level?: "debug" | "info" | "warn" | "error" | *"info"

	// Shutdown timeout
	shutdown_timeout?: string | *"30s"
}

// Listener defines the SNMP trap listener configuration
#Listener: {
	// Address to bind the SNMP trap listener to
	bind_address?: string | *"0.0.0.0:162"

	// Maximum UDP packet size
	max_packet_size?: int & >=512 | *65507

	// Read timeout for UDP connections
	read_timeout?: string | *"30s"

	// Number of worker goroutines
	workers?: int & >=1 | *4

	// Buffer size for incoming packets
	buffer_size?: int & >=100 | *1000
}

// Storage defines the storage configuration
#Storage: {
	// Storage type (sqlite, postgres, mysql)
	type?: "sqlite" | "postgres" | "mysql" | *"sqlite"

	// Database connection string
	connection_string?: string | *"./data/nereus.db"

	// Maximum number of database connections
	max_connections?: int & >=1 | *10

	// Batch size for bulk operations
	batch_size?: int & >=1 | *100

	// Batch timeout for bulk operations
	batch_timeout?: string | *"5s"

	// Data retention period in days
	retention_days?: int & >=1 | *30
}

// Correlator defines the event correlation configuration
#Correlator: {
	// Enable event deduplication
	enable_deduplication?: bool | *true

	// Deduplication time window
	deduplication_window?: string | *"5m"

	// Enable event correlation
	enable_correlation?: bool | *true

	// Correlation time window
	correlation_window?: string | *"10m"

	// Maximum events per correlation group
	max_correlation_events?: int & >=1 | *100

	// Auto-acknowledge correlated events
	auto_acknowledge?: bool | *false

	// Enable flapping detection
	enable_flapping?: bool | *false

	// Flapping threshold (events per window)
	flapping_threshold?: int & >=1 | *5

	// Flapping detection window
	flapping_window?: string | *"2m"
}

// Notifier defines the notification configuration
#Notifier: {
	// Enable notifications
	enable_notifications?: bool | *true

	// Maximum concurrent notification workers
	max_concurrent?: int & >=1 | *5

	// Notification queue size
	queue_size?: int & >=100 | *1000

	// Delivery timeout for notifications
	delivery_timeout?: string | *"30s"

	// Number of retry attempts
	retry_attempts?: int & >=0 | *3

	// Delay between retry attempts
	retry_delay?: string | *"5s"

	// Default webhook configurations
	default_webhooks?: [...#WebhookConfig]

	// Filter rules for notifications
	filter_rules?: [...#FilterRule]

	// Rate limiting configuration
	rate_limiting?: #RateLimitConfig
}

// WebhookConfig defines a webhook endpoint configuration
#WebhookConfig: {
	// Unique name for the webhook
	name: string

	// URL endpoint to send webhook notifications
	url: string & =~"^https?://.+"

	// HTTP method (GET, POST, PUT, PATCH)
	method?: "GET" | "POST" | "PUT" | "PATCH" | *"POST"

	// Payload format (prometheus, alertmanager, custom)
	format?: "prometheus" | "alertmanager" | "custom" | *"custom"

	// Template name for custom format
	template?: string

	// Whether this webhook is enabled
	enabled?: bool | *true

	// Filter names to apply
	filters?: [...string]

	// Request timeout
	timeout?: string | *"10s"

	// Number of retry attempts
	retry_count?: int & >=0 | *3

	// Content type header
	content_type?: string | *"application/json"

	// HTTP headers to include
	headers?: [string]: string
}

// FilterRule defines a notification filter rule
#FilterRule: {
	// Unique name for the filter
	name: string

	// Description of the filter
	description?: string

	// Filter conditions
	conditions?: [...#FilterCondition]

	// Whether this filter is enabled
	enabled?: bool | *true
}

// FilterCondition defines a single filter condition
#FilterCondition: {
	// Field to check
	field: string

	// Comparison operator
	operator: "equals" | "not_equals" | "contains" | "not_contains" | "matches" | "not_matches" | "in" | "not_in" | "greater_than" | "less_than"

	// Value to compare against (for single value operators)
	value?: string

	// Values to compare against (for multi-value operators)
	values?: [...string]
}

// RateLimitConfig defines rate limiting configuration
#RateLimitConfig: {
	// Enable rate limiting
	enabled?: bool | *false

	// Requests per minute
	requests_per_minute?: int & >=1 | *60

	// Burst size
	burst_size?: int & >=1 | *10

	// Time window size
	window_size?: string | *"1m"
}

// MIB defines the MIB configuration (updated version of #MIBs)
#MIB: {
	// Directories containing MIB files
	directories?: [...string] | *["/usr/share/snmp/mibs", "./mibs"]

	// Specific MIB files to load
	files?: [...string] | *["SNMPv2-MIB", "IF-MIB", "HOST-RESOURCES-MIB"]

	// Whether to automatically load all MIB files
	auto_load?: bool | *true

	// Whether to validate MIB files during loading
	validate?: bool | *true
}

// Resolver defines the OID resolution configuration
#Resolver: {
	// Enable OID resolution caching
	cache_enabled?: bool | *true

	// Maximum number of cached entries
	cache_size?: int & >=100 | *10000

	// Cache entry expiry time
	cache_expiry?: string | *"1h"

	// Enable partial OID matching
	enable_partial_oid?: bool | *true

	// Maximum search depth for OID resolution
	max_search_depth?: int & >=1 | *10
}

// Client defines the HTTP client configuration
#Client: {
	// Request timeout
	timeout?: string | *"30s"

	// Maximum number of retries
	max_retries?: int & >=0 | *3

	// Delay between retries
	retry_delay?: string | *"2s"

	// Maximum idle connections
	max_idle_conns?: int & >=1 | *100

	// Skip TLS certificate verification
	insecure_skip_verify?: bool | *false

	// User agent string
	user_agent?: string | *"nereus-snmp-listener/1.0.0"
}

// HealthCheck defines the health check configuration
#HealthCheck: {
	// Enable health check endpoint
	enabled?: bool | *true

	// Address to bind health check server
	bind_address?: string | *"0.0.0.0:8080"

	// Health check endpoint path
	path?: string | *"/health"

	// Health check timeout
	timeout?: string | *"5s"
}

// Metrics defines the metrics configuration
#Metrics: {
	// Enable metrics endpoint
	enabled?: bool | *true

	// Address to bind metrics server
	bind_address?: string | *"0.0.0.0:9090"

	// Metrics endpoint path
	path?: string | *"/metrics"

	// Metrics namespace
	namespace?: string | *"nereus"
}
