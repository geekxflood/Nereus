package helpers

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/geekxflood/common/logging"
	"github.com/geekxflood/nereus/internal/types"
)

// simpleConfigProvider is a basic config provider for testing
type simpleConfigProvider struct {
	data map[string]interface{}
}

func (s *simpleConfigProvider) GetString(key string) string {
	if val, ok := s.data[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	// Handle nested keys like "app.name"
	if key == "app.name" {
		if appMap, ok := s.data["app"].(map[string]interface{}); ok {
			if name, ok := appMap["name"].(string); ok {
				return name
			}
		}
	}
	if key == "logging.level" {
		if loggingMap, ok := s.data["logging"].(map[string]interface{}); ok {
			if level, ok := loggingMap["level"].(string); ok {
				return level
			}
		}
	}
	return ""
}

func (s *simpleConfigProvider) GetBool(key string) bool {
	if val, ok := s.data[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	// Handle nested keys like "storage.enabled"
	if key == "storage.enabled" {
		if storageMap, ok := s.data["storage"].(map[string]interface{}); ok {
			if enabled, ok := storageMap["enabled"].(bool); ok {
				return enabled
			}
		}
	}
	return false
}

func (s *simpleConfigProvider) GetInt(key string) int {
	if val, ok := s.data[key]; ok {
		if i, ok := val.(int); ok {
			return i
		}
	}
	return 0
}

// TestEnvironment provides a complete test environment for Nereus testing
type TestEnvironment struct {
	Config     *simpleConfigProvider
	Logger     logging.Logger
	TempDir    string
	DBPath     string
	ConfigPath string
	MIBDir     string
	Cleanup    func()
}

// SetupTestEnvironment creates a complete test environment with temporary directories and databases
func SetupTestEnvironment(t *testing.T, configOverrides map[string]interface{}) *TestEnvironment {
	// Create temporary directory
	tempDir, err := ioutil.TempDir("", "nereus-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create subdirectories
	dbDir := filepath.Join(tempDir, "data")
	mibDir := filepath.Join(tempDir, "mibs")
	configDir := filepath.Join(tempDir, "config")

	for _, dir := range []string{dbDir, mibDir, configDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Database path
	dbPath := filepath.Join(dbDir, "nereus-test.db")

	// Create test database with schema
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Create events table schema
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		source_ip TEXT NOT NULL,
		community TEXT NOT NULL,
		version INTEGER NOT NULL,
		pdu_type INTEGER NOT NULL,
		request_id INTEGER NOT NULL,
		trap_oid TEXT NOT NULL,
		trap_name TEXT,
		severity TEXT NOT NULL DEFAULT 'info',
		status TEXT NOT NULL DEFAULT 'open',
		acknowledged BOOLEAN NOT NULL DEFAULT 0,
		ack_by TEXT,
		ack_time DATETIME,
		count INTEGER NOT NULL DEFAULT 1,
		first_seen DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		last_seen DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		varbinds TEXT NOT NULL DEFAULT '[]',
		metadata TEXT NOT NULL DEFAULT '{}',
		hash TEXT NOT NULL,
		correlation_id TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		db.Close()
		t.Fatalf("Failed to create events table: %v", err)
	}
	db.Close()

	// Create test configuration
	configPath := filepath.Join(configDir, "test-config.yaml")
	testConfig := CreateTestConfig(dbPath, mibDir, configOverrides)
	if err := WriteConfigFile(configPath, testConfig); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// For now, create a simple config provider
	// In a real implementation, you would use the actual config loading
	cfg := &simpleConfigProvider{data: testConfig}

	// Create logger
	logger, _, err := logging.NewLogger(logging.Config{
		Level:  "debug",
		Format: "json",
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Copy test MIB files
	if err := CopyTestMIBs(mibDir); err != nil {
		t.Fatalf("Failed to copy test MIBs: %v", err)
	}

	// Cleanup function
	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return &TestEnvironment{
		Config:     cfg,
		Logger:     logger,
		TempDir:    tempDir,
		DBPath:     dbPath,
		ConfigPath: configPath,
		MIBDir:     mibDir,
		Cleanup:    cleanup,
	}
}

// CreateTestConfig creates a test configuration with safe defaults
func CreateTestConfig(dbPath, mibDir string, overrides map[string]interface{}) map[string]interface{} {
	config := map[string]interface{}{
		"app": map[string]interface{}{
			"name":      "nereus-test",
			"log_level": "debug",
		},
		"logging": map[string]interface{}{
			"level":  "debug",
			"format": "json",
		},
		"metrics": map[string]interface{}{
			"enabled":        true,
			"listen_address": ":9091", // Use different port for tests
			"metrics_path":   "/metrics",
		},
		"server": map[string]interface{}{
			"host":         "127.0.0.1",
			"port":         1162, // Non-privileged port
			"community":    "public",
			"max_handlers": 10,
			"buffer_size":  8192,
			"read_timeout": "10s",
		},
		"listener": map[string]interface{}{
			"bind_address":    "127.0.0.1:1162",
			"max_packet_size": 65507,
			"read_timeout":    "10s",
			"workers":         2,
			"buffer_size":     100,
		},
		"storage": map[string]interface{}{
			"enabled":         true,
			"driver":          "sqlite",
			"connection":      dbPath,
			"max_connections": 10,
			"batch_size":      100,
			"batch_timeout":   "5s",
			"retention_days":  30,
		},
		"events": map[string]interface{}{
			"enable_enrichment":   true,
			"enable_correlation":  true,
			"enable_storage":      true,
			"enable_notification": false, // Disable for most tests
		},
		"mib": map[string]interface{}{
			"directories":        []string{mibDir},
			"file_extensions":    []string{".mib", ".txt", ".my"},
			"max_file_size":      "10MB",
			"enable_hot_reload":  false,
			"cache_enabled":      true,
			"cache_expiry":       "30m",
			"validation_enabled": false, // Disable for faster tests
		},
		"correlation": map[string]interface{}{
			"enabled":          true,
			"time_window":      "5m",
			"max_group_size":   100,
			"cleanup_interval": "10m",
		},
		"notification": map[string]interface{}{
			"enabled": false, // Disabled by default for tests
			"webhooks": []map[string]interface{}{
				{
					"name":    "test-webhook",
					"url":     "http://localhost:8080/webhook",
					"enabled": false,
				},
			},
		},
	}

	// Apply overrides
	for key, value := range overrides {
		config[key] = value
	}

	return config
}

// WriteConfigFile writes a configuration map to a YAML file
func WriteConfigFile(path string, config map[string]interface{}) error {
	// This is a simplified implementation - in practice you'd use a YAML library
	// For now, we'll create a basic YAML structure
	content := `# Test configuration for Nereus
app:
  name: nereus-test
  log_level: debug

logging:
  level: debug
  format: json

metrics:
  enabled: true
  listen_address: ":9091"
  metrics_path: "/metrics"

server:
  host: "127.0.0.1"
  port: 1162
  community: "public"
  max_handlers: 10
  buffer_size: 8192
  read_timeout: "10s"

listener:
  bind_address: "127.0.0.1:1162"
  max_packet_size: 65507
  read_timeout: "10s"
  workers: 2
  buffer_size: 100

storage:
  enabled: true
  driver: "sqlite"
  connection: "` + config["storage"].(map[string]interface{})["connection"].(string) + `"
  max_connections: 10
  batch_size: 100
  batch_timeout: "5s"
  retention_days: 30

events:
  enable_enrichment: true
  enable_correlation: true
  enable_storage: true
  enable_notification: false

mib:
  directories:
    - "` + config["mib"].(map[string]interface{})["directories"].([]string)[0] + `"
  file_extensions:
    - ".mib"
    - ".txt"
    - ".my"
  max_file_size: "10MB"
  enable_hot_reload: false
  cache_enabled: true
  cache_expiry: "30m"
  validation_enabled: false

correlation:
  enabled: true
  time_window: "5m"
  max_group_size: 100
  cleanup_interval: "10m"

notification:
  enabled: false
`

	return ioutil.WriteFile(path, []byte(content), 0644)
}

// CopyTestMIBs copies test MIB files to the test directory
func CopyTestMIBs(destDir string) error {
	// Create basic test MIB files
	testMIBs := map[string]string{
		"SNMPv2-SMI.mib": `SNMPv2-SMI DEFINITIONS ::= BEGIN
IMPORTS MODULE-IDENTITY FROM SNMPv2-SMI;
snmpV2 MODULE-IDENTITY ::= { 1 3 6 1 6 }
END`,
		"TEST-MIB.mib": `TEST-MIB DEFINITIONS ::= BEGIN
IMPORTS MODULE-IDENTITY FROM SNMPv2-SMI;
testMIB MODULE-IDENTITY ::= { 1 3 6 1 4 1 99999 }
testTrap NOTIFICATION-TYPE ::= { testMIB 1 }
END`,
	}

	for filename, content := range testMIBs {
		path := filepath.Join(destDir, filename)
		if err := ioutil.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write MIB file %s: %w", filename, err)
		}
	}

	return nil
}

// WaitForPort waits for a port to become available or occupied
func WaitForPort(port int, available bool, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 100*time.Millisecond)
		if err != nil {
			if available {
				return nil // Port is available
			}
		} else {
			conn.Close()
			if !available {
				return nil // Port is occupied
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for port %d to be %v", port, map[bool]string{true: "available", false: "occupied"}[available])
}

// CreateTestSNMPPacket creates a test SNMP packet for testing
func CreateTestSNMPPacket(trapOID string, varbinds []types.Varbind) *types.SNMPPacket {
	return &types.SNMPPacket{
		Version:   1, // SNMPv2c
		Community: "public",
		PDUType:   7, // Trap
		RequestID: 12345,
		Varbinds:  varbinds,
	}
}

// CreateTestVarbind creates a test varbind
func CreateTestVarbind(oid, value string) types.Varbind {
	return types.Varbind{
		OID:   oid,
		Type:  types.TypeOctetString,
		Value: value,
	}
}

// CleanupDatabase removes all data from the test database
func CleanupDatabase(dbPath string) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	tables := []string{"events", "correlation_groups", "notifications"}
	for _, table := range tables {
		if _, err := db.Exec(fmt.Sprintf("DELETE FROM %s", table)); err != nil {
			// Ignore errors for non-existent tables
			continue
		}
	}

	return nil
}

// WaitForCondition waits for a condition to become true with timeout
func WaitForCondition(condition func() bool, timeout time.Duration, interval time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for condition")
		case <-ticker.C:
			if condition() {
				return nil
			}
		}
	}
}

// GetFreePort returns a free port number
func GetFreePort() (int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port, nil
}
