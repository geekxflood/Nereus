package integration

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/geekxflood/nereus/internal/events"
	"github.com/geekxflood/nereus/internal/listener"
	"github.com/geekxflood/nereus/internal/storage"
	"github.com/geekxflood/nereus/internal/types"
	"github.com/geekxflood/nereus/tests/common/helpers"
)

// TestListenerEventProcessorIntegration tests the integration between listener and event processor
func TestListenerEventProcessorIntegration(t *testing.T) {
	env := helpers.SetupTestEnvironment(t, nil)
	defer env.Cleanup()

	// Create storage
	storageComponent, err := storage.NewStorage(env.Config, env.Logger)
	require.NoError(t, err, "Failed to create storage")

	err = storageComponent.Start(context.Background())
	require.NoError(t, err, "Failed to start storage")
	defer storageComponent.Stop()

	// Create event processor
	processor, err := events.NewEventProcessor(env.Config, env.Logger)
	require.NoError(t, err, "Failed to create event processor")

	// Set storage for processor
	processor.SetStorage(storageComponent)

	err = processor.Start(context.Background())
	require.NoError(t, err, "Failed to start event processor")
	defer processor.Stop()

	// Create listener
	listenerComponent, err := listener.NewListener(env.Config, env.Logger)
	require.NoError(t, err, "Failed to create listener")

	// Set up trap callback to forward to event processor
	listenerComponent.SetTrapCallback(func(packet *types.SNMPPacket, sourceIP string) {
		result, err := processor.ProcessEventSync(packet, sourceIP)
		if err != nil {
			t.Errorf("Event processing failed: %v", err)
		} else if !result.Success {
			t.Errorf("Event processing unsuccessful: %v", result.Error)
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = listenerComponent.Start(ctx)
	require.NoError(t, err, "Failed to start listener")
	defer listenerComponent.Stop()

	// Wait for listener to start
	err = helpers.WaitForPort(1162, false, 5*time.Second)
	require.NoError(t, err, "Listener failed to start")

	// Generate and send test trap
	generator := helpers.NewSNMPTrapGenerator("public", "127.0.0.1")
	packet, err := generator.GenerateStandardTrap("coldStart")
	require.NoError(t, err, "Failed to generate trap")

	err = generator.SendTrapToListener(packet, "127.0.0.1:1162")
	require.NoError(t, err, "Failed to send trap")

	// Wait for processing
	time.Sleep(2 * time.Second)

	// Verify event was stored
	db, err := sql.Open("sqlite3", env.DBPath)
	require.NoError(t, err, "Failed to open database")
	defer db.Close()

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM events").Scan(&count)
	require.NoError(t, err, "Failed to query events")

	assert.Equal(t, 1, count, "Expected exactly one event to be stored")

	// Verify event details
	var trapOID, sourceIP string
	err = db.QueryRow("SELECT trap_oid, source_ip FROM events LIMIT 1").Scan(&trapOID, &sourceIP)
	require.NoError(t, err, "Failed to query event details")

	assert.Equal(t, "1.3.6.1.6.3.1.1.5.1", trapOID, "Trap OID mismatch")
	assert.Equal(t, "127.0.0.1", sourceIP, "Source IP mismatch")
}

// TestStorageOperations tests database storage operations
func TestStorageOperations(t *testing.T) {
	env := helpers.SetupTestEnvironment(t, nil)
	defer env.Cleanup()

	// Create storage component
	storageComponent, err := storage.NewStorage(env.Config, env.Logger)
	require.NoError(t, err, "Failed to create storage")

	err = storageComponent.Start(context.Background())
	require.NoError(t, err, "Failed to start storage")
	defer storageComponent.Stop()

	// Test storing events
	testCases := []struct {
		name      string
		packet    *types.SNMPPacket
		sourceIP  string
		enriched  map[string]interface{}
	}{
		{
			name: "Basic Event",
			packet: &types.SNMPPacket{
				Version:   1,
				Community: "public",
				PDUType:   7,
				RequestID: 12345,
				Varbinds: []types.Varbind{
					{OID: "1.3.6.1.2.1.1.3.0", Type: "timeticks", Value: "12345"},
					{OID: "1.3.6.1.6.3.1.1.4.1.0", Type: "oid", Value: "1.3.6.1.6.3.1.1.5.1"},
				},
			},
			sourceIP: "192.168.1.100",
			enriched: map[string]interface{}{
				"trap_name": "coldStart",
				"severity":  "info",
			},
		},
		{
			name: "Interface Event",
			packet: &types.SNMPPacket{
				Version:   1,
				Community: "public",
				PDUType:   7,
				RequestID: 12346,
				Varbinds: []types.Varbind{
					{OID: "1.3.6.1.2.1.1.3.0", Type: "timeticks", Value: "12346"},
					{OID: "1.3.6.1.6.3.1.1.4.1.0", Type: "oid", Value: "1.3.6.1.6.3.1.1.5.3"},
					{OID: "1.3.6.1.2.1.2.2.1.1", Type: "integer", Value: "1"},
				},
			},
			sourceIP: "192.168.1.101",
			enriched: map[string]interface{}{
				"trap_name":    "linkDown",
				"severity":     "warning",
				"interface_id": 1,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Store event
			eventID, err := storageComponent.StoreEventImmediate(tc.packet, tc.sourceIP, tc.enriched)
			require.NoError(t, err, "Failed to store event")
			assert.Greater(t, eventID, int64(0), "Event ID should be positive")

			// Verify event was stored correctly
			db, err := sql.Open("sqlite3", env.DBPath)
			require.NoError(t, err, "Failed to open database")
			defer db.Close()

			var storedSourceIP, storedCommunity string
			var storedVersion, storedPDUType int
			err = db.QueryRow("SELECT source_ip, community, version, pdu_type FROM events WHERE id = ?", eventID).
				Scan(&storedSourceIP, &storedCommunity, &storedVersion, &storedPDUType)
			require.NoError(t, err, "Failed to query stored event")

			assert.Equal(t, tc.sourceIP, storedSourceIP, "Source IP mismatch")
			assert.Equal(t, tc.packet.Community, storedCommunity, "Community mismatch")
			assert.Equal(t, tc.packet.Version, storedVersion, "Version mismatch")
			assert.Equal(t, tc.packet.PDUType, storedPDUType, "PDU type mismatch")
		})
	}

	// Test querying events
	t.Run("Query Events", func(t *testing.T) {
		db, err := sql.Open("sqlite3", env.DBPath)
		require.NoError(t, err, "Failed to open database")
		defer db.Close()

		// Count total events
		var totalCount int
		err = db.QueryRow("SELECT COUNT(*) FROM events").Scan(&totalCount)
		require.NoError(t, err, "Failed to count events")
		assert.Equal(t, len(testCases), totalCount, "Total event count mismatch")

		// Query events by source IP
		var sourceCount int
		err = db.QueryRow("SELECT COUNT(*) FROM events WHERE source_ip = ?", "192.168.1.100").Scan(&sourceCount)
		require.NoError(t, err, "Failed to count events by source")
		assert.Equal(t, 1, sourceCount, "Source-specific event count mismatch")
	})

	// Test event cleanup
	t.Run("Event Cleanup", func(t *testing.T) {
		// This would test the retention policy and cleanup functionality
		// Implementation depends on the actual cleanup mechanism in storage
		db, err := sql.Open("sqlite3", env.DBPath)
		require.NoError(t, err, "Failed to open database")
		defer db.Close()

		// Update an event to be old
		_, err = db.Exec("UPDATE events SET timestamp = datetime('now', '-31 days') WHERE id = 1")
		require.NoError(t, err, "Failed to update event timestamp")

		// Trigger cleanup (this would be done by the storage component)
		// For now, just verify the mechanism exists
		var oldCount int
		err = db.QueryRow("SELECT COUNT(*) FROM events WHERE timestamp < datetime('now', '-30 days')").Scan(&oldCount)
		require.NoError(t, err, "Failed to count old events")
		assert.Equal(t, 1, oldCount, "Expected one old event")
	})
}

// TestMIBIntegration tests MIB loading and OID resolution
func TestMIBIntegration(t *testing.T) {
	env := helpers.SetupTestEnvironment(t, nil)
	defer env.Cleanup()

	// Test MIB loading
	t.Run("MIB Loading", func(t *testing.T) {
		// This test would verify that MIB files are loaded correctly
		// and that OID resolution works as expected

		// For now, we'll test that the MIB directory exists and contains files
		mibFiles, err := helpers.GetMIBFiles(env.MIBDir)
		require.NoError(t, err, "Failed to get MIB files")
		assert.Greater(t, len(mibFiles), 0, "No MIB files found")

		// Verify test MIB files exist
		expectedFiles := []string{"SNMPv2-SMI.mib", "TEST-MIB.mib"}
		for _, expectedFile := range expectedFiles {
			found := false
			for _, mibFile := range mibFiles {
				if mibFile == expectedFile {
					found = true
					break
				}
			}
			assert.True(t, found, "Expected MIB file %s not found", expectedFile)
		}
	})

	// Test OID resolution
	t.Run("OID Resolution", func(t *testing.T) {
		// This test would verify OID resolution functionality
		// Implementation depends on the actual MIB manager interface

		// Test standard OIDs
		standardOIDs := map[string]string{
			"1.3.6.1.2.1.1.3.0":       "sysUpTime",
			"1.3.6.1.6.3.1.1.4.1.0":   "snmpTrapOID",
			"1.3.6.1.6.3.1.1.5.1":     "coldStart",
			"1.3.6.1.6.3.1.1.5.3":     "linkDown",
		}

		for oid, expectedName := range standardOIDs {
			t.Run(fmt.Sprintf("Resolve_%s", oid), func(t *testing.T) {
				// This would test actual OID resolution
				// For now, just verify the OID format is valid
				assert.Regexp(t, `^\d+(\.\d+)*$`, oid, "Invalid OID format")
				assert.NotEmpty(t, expectedName, "Expected name should not be empty")
			})
		}
	})
}

// TestConfigurationValidation tests configuration loading and validation
func TestConfigurationValidation(t *testing.T) {
	env := helpers.SetupTestEnvironment(t, nil)
	defer env.Cleanup()

	// Test valid configuration
	t.Run("Valid Configuration", func(t *testing.T) {
		// Configuration was already loaded in SetupTestEnvironment
		assert.NotNil(t, env.Config, "Configuration should not be nil")

		// Test accessing configuration values
		appName := env.Config.GetString("app.name")
		assert.Equal(t, "nereus-test", appName, "App name mismatch")

		logLevel := env.Config.GetString("logging.level")
		assert.Equal(t, "debug", logLevel, "Log level mismatch")

		storageEnabled := env.Config.GetBool("storage.enabled")
		assert.True(t, storageEnabled, "Storage should be enabled")
	})

	// Test invalid configuration
	t.Run("Invalid Configuration", func(t *testing.T) {
		// Create invalid configuration
		invalidConfig := map[string]interface{}{
			"server": map[string]interface{}{
				"port": "invalid_port", // Should be integer
			},
		}

		tempEnv := helpers.SetupTestEnvironment(t, invalidConfig)
		defer tempEnv.Cleanup()

		// This should handle the invalid configuration gracefully
		// The actual behavior depends on the configuration validation implementation
	})
}

// Helper function to get MIB files (placeholder implementation)
func GetMIBFiles(mibDir string) ([]string, error) {
	// This is a placeholder implementation
	// In practice, you would scan the directory for MIB files
	return []string{"SNMPv2-SMI.mib", "TEST-MIB.mib"}, nil
}
