package e2e

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/geekxflood/nereus/internal/app"
	"github.com/geekxflood/nereus/tests/common/helpers"
)

// TestCompleteWorkflow tests the complete SNMP trap processing workflow
func TestCompleteWorkflow(t *testing.T) {
	// Setup test environment
	env := helpers.SetupTestEnvironment(t, nil)
	defer env.Cleanup()

	// Create SNMP trap generator
	generator := helpers.NewSNMPTrapGenerator("public", "127.0.0.1")

	// Start Nereus application
	application, err := app.NewApplication(env.Config, env.Logger)
	require.NoError(t, err, "Failed to create application")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start application in background
	go func() {
		err := application.Start(ctx)
		if err != nil && err != context.Canceled {
			t.Errorf("Application failed to start: %v", err)
		}
	}()

	// Wait for application to start
	err = helpers.WaitForPort(1162, false, 10*time.Second)
	require.NoError(t, err, "Application failed to start listening")

	// Wait for metrics endpoint
	err = helpers.WaitForPort(9091, false, 10*time.Second)
	require.NoError(t, err, "Metrics endpoint failed to start")

	// Test cases for different trap types
	testCases := []struct {
		name     string
		trapType string
		expected map[string]interface{}
	}{
		{
			name:     "Cold Start Trap",
			trapType: "coldStart",
			expected: map[string]interface{}{
				"trap_oid": "1.3.6.1.6.3.1.1.5.1",
				"source":   "127.0.0.1",
			},
		},
		{
			name:     "Link Down Trap",
			trapType: "linkDown",
			expected: map[string]interface{}{
				"trap_oid": "1.3.6.1.6.3.1.1.5.3",
				"source":   "127.0.0.1",
			},
		},
		{
			name:     "Authentication Failure Trap",
			trapType: "authenticationFailure",
			expected: map[string]interface{}{
				"trap_oid": "1.3.6.1.6.3.1.1.5.5",
				"source":   "127.0.0.1",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Generate and send trap
			packet, err := generator.GenerateStandardTrap(tc.trapType)
			require.NoError(t, err, "Failed to generate trap")

			err = generator.SendTrapToListener(packet, "127.0.0.1:1162")
			require.NoError(t, err, "Failed to send trap")

			// Wait for processing
			time.Sleep(2 * time.Second)

			// Verify trap was stored in database
			db, err := sql.Open("sqlite3", env.DBPath)
			require.NoError(t, err, "Failed to open database")
			defer db.Close()

			var count int
			err = db.QueryRow("SELECT COUNT(*) FROM events WHERE source_ip = ?", tc.expected["source"]).Scan(&count)
			require.NoError(t, err, "Failed to query database")
			assert.Greater(t, count, 0, "No events found in database")

			// Verify event details
			var trapOID, sourceIP string
			err = db.QueryRow("SELECT trap_oid, source_ip FROM events WHERE source_ip = ? ORDER BY id DESC LIMIT 1", tc.expected["source"]).Scan(&trapOID, &sourceIP)
			require.NoError(t, err, "Failed to query event details")

			assert.Equal(t, tc.expected["trap_oid"], trapOID, "Trap OID mismatch")
			assert.Equal(t, tc.expected["source"], sourceIP, "Source IP mismatch")
		})
	}

	// Test metrics collection
	t.Run("Metrics Collection", func(t *testing.T) {
		resp, err := http.Get("http://localhost:9091/metrics")
		require.NoError(t, err, "Failed to get metrics")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Metrics endpoint returned error")

		// Verify metrics contain expected values
		// Note: In a real implementation, you would parse the metrics response
		// and verify specific metric values
	})

	// Test health endpoint
	t.Run("Health Check", func(t *testing.T) {
		resp, err := http.Get("http://localhost:9091/health")
		require.NoError(t, err, "Failed to get health status")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Health endpoint returned error")
	})

	// Stop application
	cancel()
	time.Sleep(1 * time.Second)
}

// TestHighVolumeWorkflow tests the system under high trap volume
func TestHighVolumeWorkflow(t *testing.T) {
	// Setup test environment with optimized configuration
	config := map[string]interface{}{
		"events": map[string]interface{}{
			"enable_enrichment": false, // Disable for performance
		},
		"listener": map[string]interface{}{
			"workers":     4,
			"buffer_size": 1000,
		},
	}

	env := helpers.SetupTestEnvironment(t, config)
	defer env.Cleanup()

	generator := helpers.NewSNMPTrapGenerator("public", "127.0.0.1")

	// Start application
	application, err := app.NewApplication(env.Config, env.Logger)
	require.NoError(t, err, "Failed to create application")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := application.Start(ctx)
		if err != nil && err != context.Canceled {
			t.Errorf("Application failed to start: %v", err)
		}
	}()

	// Wait for startup
	err = helpers.WaitForPort(1162, false, 10*time.Second)
	require.NoError(t, err, "Application failed to start")

	// Generate burst of traps
	trapCount := 100
	packets, err := generator.GenerateBurstTraps(trapCount, "coldStart", 10*time.Millisecond)
	require.NoError(t, err, "Failed to generate burst traps")

	// Send all traps
	start := time.Now()
	for _, packet := range packets {
		err = generator.SendTrapToListener(packet, "127.0.0.1:1162")
		require.NoError(t, err, "Failed to send trap")
	}
	sendDuration := time.Since(start)

	// Wait for processing
	time.Sleep(5 * time.Second)

	// Verify results
	db, err := sql.Open("sqlite3", env.DBPath)
	require.NoError(t, err, "Failed to open database")
	defer db.Close()

	var processedCount int
	err = db.QueryRow("SELECT COUNT(*) FROM events").Scan(&processedCount)
	require.NoError(t, err, "Failed to query processed count")

	t.Logf("Sent %d traps in %v, processed %d", trapCount, sendDuration, processedCount)

	// Verify at least 80% of traps were processed (allowing for some loss under high load)
	minExpected := int(float64(trapCount) * 0.8)
	assert.GreaterOrEqual(t, processedCount, minExpected, "Too many traps were lost during processing")

	cancel()
	time.Sleep(1 * time.Second)
}

// TestMIBEnrichmentWorkflow tests the MIB enrichment functionality
func TestMIBEnrichmentWorkflow(t *testing.T) {
	// Setup test environment with enrichment enabled
	config := map[string]interface{}{
		"events": map[string]interface{}{
			"enable_enrichment": true,
		},
	}

	env := helpers.SetupTestEnvironment(t, config)
	defer env.Cleanup()

	generator := helpers.NewSNMPTrapGenerator("public", "127.0.0.1")

	// Start application
	application, err := app.NewApplication(env.Config, env.Logger)
	require.NoError(t, err, "Failed to create application")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := application.Start(ctx)
		if err != nil && err != context.Canceled {
			t.Errorf("Application failed to start: %v", err)
		}
	}()

	// Wait for startup
	err = helpers.WaitForPort(1162, false, 10*time.Second)
	require.NoError(t, err, "Application failed to start")

	// Send trap with known OIDs
	packet, err := generator.GenerateStandardTrap("coldStart")
	require.NoError(t, err, "Failed to generate trap")

	err = generator.SendTrapToListener(packet, "127.0.0.1:1162")
	require.NoError(t, err, "Failed to send trap")

	// Wait for processing
	time.Sleep(3 * time.Second)

	// Verify enrichment data was added
	db, err := sql.Open("sqlite3", env.DBPath)
	require.NoError(t, err, "Failed to open database")
	defer db.Close()

	var metadata string
	err = db.QueryRow("SELECT metadata FROM events ORDER BY id DESC LIMIT 1").Scan(&metadata)
	require.NoError(t, err, "Failed to query metadata")

	// Verify metadata contains enrichment information
	assert.NotEmpty(t, metadata, "Metadata should not be empty")
	// Note: In a real implementation, you would parse the JSON metadata
	// and verify specific enrichment fields

	cancel()
	time.Sleep(1 * time.Second)
}

// TestErrorHandlingWorkflow tests error handling in the workflow
func TestErrorHandlingWorkflow(t *testing.T) {
	env := helpers.SetupTestEnvironment(t, nil)
	defer env.Cleanup()

	generator := helpers.NewSNMPTrapGenerator("public", "127.0.0.1")

	// Start application
	application, err := app.NewApplication(env.Config, env.Logger)
	require.NoError(t, err, "Failed to create application")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := application.Start(ctx)
		if err != nil && err != context.Canceled {
			t.Errorf("Application failed to start: %v", err)
		}
	}()

	// Wait for startup
	err = helpers.WaitForPort(1162, false, 10*time.Second)
	require.NoError(t, err, "Application failed to start")

	// Test malformed packets
	malformedTypes := []string{
		"invalid_version",
		"empty_community",
		"invalid_pdu_type",
		"no_varbinds",
	}

	for _, malformType := range malformedTypes {
		t.Run(fmt.Sprintf("Malformed_%s", malformType), func(t *testing.T) {
			packet, err := generator.GenerateMalformedTrap(malformType)
			require.NoError(t, err, "Failed to generate malformed trap")

			// Send malformed packet (should not crash the system)
			err = generator.SendTrapToListener(packet, "127.0.0.1:1162")
			// Note: We don't require no error here as malformed packets may be rejected

			// Wait a bit
			time.Sleep(500 * time.Millisecond)

			// Verify system is still responsive
			resp, err := http.Get("http://localhost:9091/health")
			require.NoError(t, err, "System became unresponsive after malformed packet")
			defer resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode, "Health check failed after malformed packet")
		})
	}

	cancel()
	time.Sleep(1 * time.Second)
}
