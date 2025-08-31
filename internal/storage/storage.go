// Package storage provides persistent event storage and querying functionality.
package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/geekxflood/common/config"
	"github.com/geekxflood/nereus/internal/types"
	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// StorageConfig holds configuration for the event storage system
type StorageConfig struct {
	DatabaseType     string        `json:"database_type"`
	ConnectionString string        `json:"connection_string"`
	MaxConnections   int           `json:"max_connections"`
	RetentionDays    int           `json:"retention_days"`
	BatchSize        int           `json:"batch_size"`
	FlushInterval    time.Duration `json:"flush_interval"`
	EnableIndexes    bool          `json:"enable_indexes"`
	CompressionLevel int           `json:"compression_level"`
}

// DefaultStorageConfig returns a default storage configuration
func DefaultStorageConfig() *StorageConfig {
	return &StorageConfig{
		DatabaseType:     "sqlite3",
		ConnectionString: "./nereus_events.db",
		MaxConnections:   10,
		RetentionDays:    30,
		BatchSize:        100,
		FlushInterval:    5 * time.Second,
		EnableIndexes:    true,
		CompressionLevel: 1,
	}
}

// Event represents a stored SNMP trap event
type Event struct {
	ID            int64      `json:"id" db:"id"`
	Timestamp     time.Time  `json:"timestamp" db:"timestamp"`
	SourceIP      string     `json:"source_ip" db:"source_ip"`
	Community     string     `json:"community" db:"community"`
	Version       int        `json:"version" db:"version"`
	PDUType       int        `json:"pdu_type" db:"pdu_type"`
	RequestID     int32      `json:"request_id" db:"request_id"`
	TrapOID       string     `json:"trap_oid" db:"trap_oid"`
	TrapName      *string    `json:"trap_name" db:"trap_name"`
	Severity      string     `json:"severity" db:"severity"`
	Status        string     `json:"status" db:"status"`
	Acknowledged  bool       `json:"acknowledged" db:"acknowledged"`
	AckBy         *string    `json:"ack_by" db:"ack_by"`
	AckTime       *time.Time `json:"ack_time" db:"ack_time"`
	Count         int        `json:"count" db:"count"`
	FirstSeen     time.Time  `json:"first_seen" db:"first_seen"`
	LastSeen      time.Time  `json:"last_seen" db:"last_seen"`
	Varbinds      string     `json:"varbinds" db:"varbinds"` // JSON encoded
	Metadata      string     `json:"metadata" db:"metadata"` // JSON encoded
	Hash          string     `json:"hash" db:"hash"`
	CorrelationID string     `json:"correlation_id" db:"correlation_id"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
}

// GetID returns the event ID (implements EventInterface)
func (e *Event) GetID() int64 {
	return e.ID
}

// EventQuery represents query parameters for searching events
type EventQuery struct {
	StartTime     *time.Time `json:"start_time,omitempty"`
	EndTime       *time.Time `json:"end_time,omitempty"`
	SourceIP      string     `json:"source_ip,omitempty"`
	Community     string     `json:"community,omitempty"`
	TrapOID       string     `json:"trap_oid,omitempty"`
	TrapName      string     `json:"trap_name,omitempty"`
	Severity      string     `json:"severity,omitempty"`
	Status        string     `json:"status,omitempty"`
	Acknowledged  *bool      `json:"acknowledged,omitempty"`
	CorrelationID string     `json:"correlation_id,omitempty"`
	Limit         int        `json:"limit,omitempty"`
	Offset        int        `json:"offset,omitempty"`
	OrderBy       string     `json:"order_by,omitempty"`
	OrderDesc     bool       `json:"order_desc,omitempty"`
}

// StorageStats tracks storage statistics
type StorageStats struct {
	TotalEvents       int64            `json:"total_events"`
	EventsToday       int64            `json:"events_today"`
	EventsThisWeek    int64            `json:"events_this_week"`
	EventsThisMonth   int64            `json:"events_this_month"`
	DatabaseSize      int64            `json:"database_size"`
	OldestEvent       *time.Time       `json:"oldest_event,omitempty"`
	NewestEvent       *time.Time       `json:"newest_event,omitempty"`
	AveragePerDay     float64          `json:"average_per_day"`
	TopSources        []SourceStats    `json:"top_sources"`
	TopTraps          []TrapStats      `json:"top_traps"`
	SeverityBreakdown map[string]int64 `json:"severity_breakdown"`
	StatusBreakdown   map[string]int64 `json:"status_breakdown"`
}

// SourceStats represents statistics for a source IP
type SourceStats struct {
	SourceIP string `json:"source_ip"`
	Count    int64  `json:"count"`
}

// TrapStats represents statistics for a trap type
type TrapStats struct {
	TrapOID  string `json:"trap_oid"`
	TrapName string `json:"trap_name"`
	Count    int64  `json:"count"`
}

// Storage provides persistent event storage functionality
type Storage struct {
	config     *StorageConfig
	db         *sql.DB
	batchQueue []*Event
	mu         sync.RWMutex
	stats      *StorageStats
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// NewStorage creates a new event storage instance
func NewStorage(cfg config.Provider) (*Storage, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration provider cannot be nil")
	}

	// Load configuration
	storageConfig := DefaultStorageConfig()

	if dbType, err := cfg.GetString("storage.database_type", storageConfig.DatabaseType); err == nil {
		storageConfig.DatabaseType = dbType
	}

	if connStr, err := cfg.GetString("storage.connection_string", storageConfig.ConnectionString); err == nil {
		storageConfig.ConnectionString = connStr
	}

	if maxConn, err := cfg.GetInt("storage.max_connections", storageConfig.MaxConnections); err == nil {
		storageConfig.MaxConnections = maxConn
	}

	if retention, err := cfg.GetInt("storage.retention_days", storageConfig.RetentionDays); err == nil {
		storageConfig.RetentionDays = retention
	}

	if batchSize, err := cfg.GetInt("storage.batch_size", storageConfig.BatchSize); err == nil {
		storageConfig.BatchSize = batchSize
	}

	if flushInterval, err := cfg.GetDuration("storage.flush_interval", storageConfig.FlushInterval); err == nil {
		storageConfig.FlushInterval = flushInterval
	}

	// Open database connection
	db, err := sql.Open(storageConfig.DatabaseType, storageConfig.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(storageConfig.MaxConnections)
	db.SetMaxIdleConns(storageConfig.MaxConnections / 2)
	db.SetConnMaxLifetime(time.Hour)

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	storage := &Storage{
		config:     storageConfig,
		db:         db,
		batchQueue: make([]*Event, 0, storageConfig.BatchSize),
		stats:      &StorageStats{SeverityBreakdown: make(map[string]int64), StatusBreakdown: make(map[string]int64)},
		ctx:        ctx,
		cancel:     cancel,
	}

	// Initialize database schema
	if err := storage.initSchema(); err != nil {
		storage.Close()
		return nil, fmt.Errorf("failed to initialize database schema: %w", err)
	}

	// Start background workers
	storage.wg.Add(2)
	go storage.batchWorker()
	go storage.cleanupWorker()

	return storage, nil
}

// initSchema creates the database tables and indexes
func (s *Storage) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL,
		source_ip TEXT NOT NULL,
		community TEXT NOT NULL,
		version INTEGER NOT NULL,
		pdu_type INTEGER NOT NULL,
		request_id INTEGER NOT NULL,
		trap_oid TEXT NOT NULL,
		trap_name TEXT,
		severity TEXT DEFAULT 'info',
		status TEXT DEFAULT 'open',
		acknowledged BOOLEAN DEFAULT FALSE,
		ack_by TEXT,
		ack_time DATETIME,
		count INTEGER DEFAULT 1,
		first_seen DATETIME NOT NULL,
		last_seen DATETIME NOT NULL,
		varbinds TEXT,
		metadata TEXT,
		hash TEXT NOT NULL,
		correlation_id TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create events table: %w", err)
	}

	// Create indexes if enabled
	if s.config.EnableIndexes {
		indexes := []string{
			"CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp);",
			"CREATE INDEX IF NOT EXISTS idx_events_source_ip ON events(source_ip);",
			"CREATE INDEX IF NOT EXISTS idx_events_trap_oid ON events(trap_oid);",
			"CREATE INDEX IF NOT EXISTS idx_events_severity ON events(severity);",
			"CREATE INDEX IF NOT EXISTS idx_events_status ON events(status);",
			"CREATE INDEX IF NOT EXISTS idx_events_hash ON events(hash);",
			"CREATE INDEX IF NOT EXISTS idx_events_correlation_id ON events(correlation_id);",
			"CREATE INDEX IF NOT EXISTS idx_events_acknowledged ON events(acknowledged);",
		}

		for _, idx := range indexes {
			if _, err := s.db.Exec(idx); err != nil {
				return fmt.Errorf("failed to create index: %w", err)
			}
		}
	}

	return nil
}

// StoreEvent stores a single event (adds to batch queue)
func (s *Storage) StoreEvent(packet *types.SNMPPacket, sourceIP string, enrichedData map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Convert packet to event
	event, err := s.packetToEvent(packet, sourceIP, enrichedData)
	if err != nil {
		return fmt.Errorf("failed to convert packet to event: %w", err)
	}

	// Add to batch queue
	s.batchQueue = append(s.batchQueue, event)

	// Flush if batch is full
	if len(s.batchQueue) >= s.config.BatchSize {
		return s.flushBatch()
	}

	return nil
}

// StoreEventImmediate stores a single event immediately and returns the event ID
func (s *Storage) StoreEventImmediate(packet *types.SNMPPacket, sourceIP string, enrichedData map[string]interface{}) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Convert packet to event
	event, err := s.packetToEvent(packet, sourceIP, enrichedData)
	if err != nil {
		return 0, fmt.Errorf("failed to convert packet to event: %w", err)
	}

	// Store immediately
	result, err := s.db.Exec(`
		INSERT INTO events (
			timestamp, source_ip, community, version, pdu_type, request_id,
			trap_oid, trap_name, severity, status, acknowledged, count,
			first_seen, last_seen, varbinds, metadata, hash, correlation_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, event.Timestamp, event.SourceIP, event.Community, event.Version,
		event.PDUType, event.RequestID, event.TrapOID, event.TrapName,
		event.Severity, event.Status, event.Acknowledged, event.Count,
		event.FirstSeen, event.LastSeen, event.Varbinds, event.Metadata,
		event.Hash, event.CorrelationID)

	if err != nil {
		return 0, fmt.Errorf("failed to insert event: %w", err)
	}

	eventID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert ID: %w", err)
	}

	return eventID, nil
}

// packetToEvent converts an SNMP packet to a storage event
func (s *Storage) packetToEvent(packet *types.SNMPPacket, sourceIP string, enrichedData map[string]interface{}) (*Event, error) {
	now := time.Now()

	// Serialize varbinds
	varbindsJSON, err := json.Marshal(packet.Varbinds)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal varbinds: %w", err)
	}

	// Serialize metadata
	metadataJSON, err := json.Marshal(enrichedData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Extract trap OID from varbinds (typically the second varbind in SNMPv2c)
	trapOID := ""
	trapName := ""
	if len(packet.Varbinds) > 1 && packet.Varbinds[1].OID == "1.3.6.1.6.3.1.1.4.1.0" {
		if oidValue, ok := packet.Varbinds[1].Value.(string); ok {
			trapOID = oidValue
		}
	}

	// Get trap name from enriched data if available
	if name, exists := enrichedData["trap_name"]; exists {
		if nameStr, ok := name.(string); ok {
			trapName = nameStr
		}
	}

	// Generate hash for deduplication
	hash := s.generateEventHash(sourceIP, trapOID, packet.Community)

	// Determine severity from enriched data or default
	severity := "info"
	if sev, exists := enrichedData["severity"]; exists {
		if sevStr, ok := sev.(string); ok {
			severity = sevStr
		}
	}

	// Convert trapName to pointer
	var trapNamePtr *string
	if trapName != "" {
		trapNamePtr = &trapName
	}

	event := &Event{
		Timestamp:    now,
		SourceIP:     sourceIP,
		Community:    packet.Community,
		Version:      packet.Version,
		PDUType:      packet.PDUType,
		RequestID:    packet.RequestID,
		TrapOID:      trapOID,
		TrapName:     trapNamePtr,
		Severity:     severity,
		Status:       "open",
		Acknowledged: false,
		Count:        1,
		FirstSeen:    now,
		LastSeen:     now,
		Varbinds:     string(varbindsJSON),
		Metadata:     string(metadataJSON),
		Hash:         hash,
	}

	return event, nil
}

// generateEventHash generates a hash for event deduplication
func (s *Storage) generateEventHash(sourceIP, trapOID, community string) string {
	return fmt.Sprintf("%s:%s:%s", sourceIP, trapOID, community)
}

// flushBatch flushes the current batch to database
func (s *Storage) flushBatch() error {
	if len(s.batchQueue) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO events (
			timestamp, source_ip, community, version, pdu_type, request_id,
			trap_oid, trap_name, severity, status, acknowledged, count,
			first_seen, last_seen, varbinds, metadata, hash, correlation_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, event := range s.batchQueue {
		_, err := stmt.Exec(
			event.Timestamp, event.SourceIP, event.Community, event.Version,
			event.PDUType, event.RequestID, event.TrapOID, event.TrapName,
			event.Severity, event.Status, event.Acknowledged, event.Count,
			event.FirstSeen, event.LastSeen, event.Varbinds, event.Metadata,
			event.Hash, event.CorrelationID,
		)
		if err != nil {
			return fmt.Errorf("failed to execute statement: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Clear batch queue
	s.batchQueue = s.batchQueue[:0]
	return nil
}

// batchWorker periodically flushes batched events
func (s *Storage) batchWorker() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			// Final flush before shutdown
			s.mu.Lock()
			s.flushBatch()
			s.mu.Unlock()
			return
		case <-ticker.C:
			s.mu.Lock()
			if len(s.batchQueue) > 0 {
				s.flushBatch()
			}
			s.mu.Unlock()
		}
	}
}

// cleanupWorker periodically removes old events based on retention policy
func (s *Storage) cleanupWorker() {
	defer s.wg.Done()

	ticker := time.NewTicker(24 * time.Hour) // Run daily
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.cleanup()
		}
	}
}

// cleanup removes events older than retention period
func (s *Storage) cleanup() {
	cutoff := time.Now().AddDate(0, 0, -s.config.RetentionDays)

	result, err := s.db.Exec("DELETE FROM events WHERE timestamp < ?", cutoff)
	if err != nil {
		// Log error but don't fail
		return
	}

	if rowsAffected, err := result.RowsAffected(); err == nil && rowsAffected > 0 {
		// Log cleanup results
		_ = rowsAffected
	}
}

// QueryEvents queries events based on the provided criteria
func (s *Storage) QueryEvents(query *EventQuery) ([]*Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Build SQL query
	sqlQuery := "SELECT * FROM events WHERE 1=1"
	args := []interface{}{}

	if query.StartTime != nil {
		sqlQuery += " AND timestamp >= ?"
		args = append(args, *query.StartTime)
	}

	if query.EndTime != nil {
		sqlQuery += " AND timestamp <= ?"
		args = append(args, *query.EndTime)
	}

	if query.SourceIP != "" {
		sqlQuery += " AND source_ip = ?"
		args = append(args, query.SourceIP)
	}

	if query.Community != "" {
		sqlQuery += " AND community = ?"
		args = append(args, query.Community)
	}

	if query.TrapOID != "" {
		sqlQuery += " AND trap_oid = ?"
		args = append(args, query.TrapOID)
	}

	if query.TrapName != "" {
		sqlQuery += " AND trap_name LIKE ?"
		args = append(args, "%"+query.TrapName+"%")
	}

	if query.Severity != "" {
		sqlQuery += " AND severity = ?"
		args = append(args, query.Severity)
	}

	if query.Status != "" {
		sqlQuery += " AND status = ?"
		args = append(args, query.Status)
	}

	if query.Acknowledged != nil {
		sqlQuery += " AND acknowledged = ?"
		args = append(args, *query.Acknowledged)
	}

	if query.CorrelationID != "" {
		sqlQuery += " AND correlation_id = ?"
		args = append(args, query.CorrelationID)
	}

	// Add ordering
	orderBy := "timestamp"
	if query.OrderBy != "" {
		orderBy = query.OrderBy
	}

	if query.OrderDesc {
		sqlQuery += fmt.Sprintf(" ORDER BY %s DESC", orderBy)
	} else {
		sqlQuery += fmt.Sprintf(" ORDER BY %s ASC", orderBy)
	}

	// Add limit and offset
	if query.Limit > 0 {
		sqlQuery += " LIMIT ?"
		args = append(args, query.Limit)
	}

	if query.Offset > 0 {
		sqlQuery += " OFFSET ?"
		args = append(args, query.Offset)
	}

	rows, err := s.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var events []*Event
	for rows.Next() {
		event := &Event{}
		err := rows.Scan(
			&event.ID, &event.Timestamp, &event.SourceIP, &event.Community,
			&event.Version, &event.PDUType, &event.RequestID, &event.TrapOID,
			&event.TrapName, &event.Severity, &event.Status, &event.Acknowledged,
			&event.AckBy, &event.AckTime, &event.Count, &event.FirstSeen,
			&event.LastSeen, &event.Varbinds, &event.Metadata, &event.Hash,
			&event.CorrelationID, &event.CreatedAt, &event.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}
		events = append(events, event)
	}

	return events, nil
}

// GetEvent retrieves a single event by ID
func (s *Storage) GetEvent(id int64) (*Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	event := &Event{}
	err := s.db.QueryRow("SELECT * FROM events WHERE id = ?", id).Scan(
		&event.ID, &event.Timestamp, &event.SourceIP, &event.Community,
		&event.Version, &event.PDUType, &event.RequestID, &event.TrapOID,
		&event.TrapName, &event.Severity, &event.Status, &event.Acknowledged,
		&event.AckBy, &event.AckTime, &event.Count, &event.FirstSeen,
		&event.LastSeen, &event.Varbinds, &event.Metadata, &event.Hash,
		&event.CorrelationID, &event.CreatedAt, &event.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("event not found")
		}
		return nil, fmt.Errorf("failed to get event: %w", err)
	}

	return event, nil
}

// UpdateEvent updates an existing event
func (s *Storage) UpdateEvent(event *Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		UPDATE events SET
			severity = ?, status = ?, acknowledged = ?, ack_by = ?, ack_time = ?,
			count = ?, last_seen = ?, correlation_id = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, event.Severity, event.Status, event.Acknowledged, event.AckBy, event.AckTime,
		event.Count, event.LastSeen, event.CorrelationID, event.ID)

	if err != nil {
		return fmt.Errorf("failed to update event: %w", err)
	}

	return nil
}

// AcknowledgeEvent acknowledges an event
func (s *Storage) AcknowledgeEvent(id int64, ackBy string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	_, err := s.db.Exec(`
		UPDATE events SET
			acknowledged = TRUE, ack_by = ?, ack_time = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, ackBy, now, id)

	if err != nil {
		return fmt.Errorf("failed to acknowledge event: %w", err)
	}

	return nil
}

// GetStats returns storage statistics
func (s *Storage) GetStats() (*StorageStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &StorageStats{
		SeverityBreakdown: make(map[string]int64),
		StatusBreakdown:   make(map[string]int64),
	}

	// Get total events
	err := s.db.QueryRow("SELECT COUNT(*) FROM events").Scan(&stats.TotalEvents)
	if err != nil {
		return nil, fmt.Errorf("failed to get total events: %w", err)
	}

	// Get events today
	today := time.Now().Truncate(24 * time.Hour)
	err = s.db.QueryRow("SELECT COUNT(*) FROM events WHERE timestamp >= ?", today).Scan(&stats.EventsToday)
	if err != nil {
		return nil, fmt.Errorf("failed to get events today: %w", err)
	}

	// Get events this week
	weekStart := today.AddDate(0, 0, -int(today.Weekday()))
	err = s.db.QueryRow("SELECT COUNT(*) FROM events WHERE timestamp >= ?", weekStart).Scan(&stats.EventsThisWeek)
	if err != nil {
		return nil, fmt.Errorf("failed to get events this week: %w", err)
	}

	// Get events this month
	monthStart := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
	err = s.db.QueryRow("SELECT COUNT(*) FROM events WHERE timestamp >= ?", monthStart).Scan(&stats.EventsThisMonth)
	if err != nil {
		return nil, fmt.Errorf("failed to get events this month: %w", err)
	}

	// Get oldest and newest events
	var oldestTime, newestTime sql.NullTime
	err = s.db.QueryRow("SELECT MIN(timestamp), MAX(timestamp) FROM events").Scan(&oldestTime, &newestTime)
	if err == nil {
		if oldestTime.Valid {
			stats.OldestEvent = &oldestTime.Time
		}
		if newestTime.Valid {
			stats.NewestEvent = &newestTime.Time
		}
	}

	// Calculate average per day
	if stats.OldestEvent != nil && stats.NewestEvent != nil {
		days := stats.NewestEvent.Sub(*stats.OldestEvent).Hours() / 24
		if days > 0 {
			stats.AveragePerDay = float64(stats.TotalEvents) / days
		}
	}

	return stats, nil
}

// Close closes the storage system
func (s *Storage) Close() error {
	s.cancel()
	s.wg.Wait()

	// Final flush
	s.mu.Lock()
	s.flushBatch()
	s.mu.Unlock()

	return s.db.Close()
}
