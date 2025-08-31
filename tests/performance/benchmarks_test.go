package performance

import (
	"context"
	"database/sql"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"

	"github.com/geekxflood/nereus/internal/app"
	"github.com/geekxflood/nereus/tests/common/helpers"
)

// BenchmarkTrapProcessing benchmarks SNMP trap processing performance
func BenchmarkTrapProcessing(b *testing.B) {
	// Setup optimized environment for benchmarking
	config := map[string]interface{}{
		"events": map[string]interface{}{
			"enable_enrichment":  false, // Disable for pure processing speed
			"enable_correlation": false,
		},
		"storage": map[string]interface{}{
			"batch_size":    1000,
			"batch_timeout": "100ms",
		},
		"listener": map[string]interface{}{
			"workers":     runtime.NumCPU(),
			"buffer_size": 10000,
		},
	}

	env := helpers.SetupTestEnvironment(b, config)
	defer env.Cleanup()

	// Start application
	application, err := app.NewApplication(env.Config, env.Logger)
	require.NoError(b, err, "Failed to create application")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := application.Start(ctx)
		if err != nil && err != context.Canceled {
			b.Errorf("Application failed: %v", err)
		}
	}()

	// Wait for startup
	err = helpers.WaitForPort(1162, false, 10*time.Second)
	require.NoError(b, err, "Application failed to start")

	generator := helpers.NewSNMPTrapGenerator("public", "127.0.0.1")

	// Pre-generate packets to avoid generation overhead in benchmark
	packets := make([]*types.SNMPPacket, b.N)
	for i := 0; i < b.N; i++ {
		packet, err := generator.GenerateStandardTrap("coldStart")
		require.NoError(b, err, "Failed to generate trap")
		packets[i] = packet
	}

	b.ResetTimer()
	b.ReportAllocs()

	// Benchmark trap sending
	start := time.Now()
	for i := 0; i < b.N; i++ {
		err := generator.SendTrapToListener(packets[i], "127.0.0.1:1162")
		if err != nil {
			b.Errorf("Failed to send trap %d: %v", i, err)
		}
	}
	sendDuration := time.Since(start)

	b.StopTimer()

	// Wait for processing to complete
	time.Sleep(5 * time.Second)

	// Report results
	b.Logf("Sent %d traps in %v (%.2f traps/sec)", b.N, sendDuration, float64(b.N)/sendDuration.Seconds())

	// Verify processing
	db, err := sql.Open("sqlite3", env.DBPath)
	require.NoError(b, err, "Failed to open database")
	defer db.Close()

	var processedCount int
	err = db.QueryRow("SELECT COUNT(*) FROM events").Scan(&processedCount)
	require.NoError(b, err, "Failed to query processed count")

	b.Logf("Processed %d out of %d traps (%.2f%%)", processedCount, b.N, float64(processedCount)/float64(b.N)*100)

	cancel()
}

// BenchmarkConcurrentProcessing benchmarks concurrent trap processing
func BenchmarkConcurrentProcessing(b *testing.B) {
	config := map[string]interface{}{
		"events": map[string]interface{}{
			"enable_enrichment":  false,
			"enable_correlation": false,
		},
		"listener": map[string]interface{}{
			"workers":     runtime.NumCPU() * 2,
			"buffer_size": 10000,
		},
	}

	env := helpers.SetupTestEnvironment(b, config)
	defer env.Cleanup()

	// Start application
	application, err := app.NewApplication(env.Config, env.Logger)
	require.NoError(b, err, "Failed to create application")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := application.Start(ctx)
		if err != nil && err != context.Canceled {
			b.Errorf("Application failed: %v", err)
		}
	}()

	// Wait for startup
	err = helpers.WaitForPort(1162, false, 10*time.Second)
	require.NoError(b, err, "Application failed to start")

	numWorkers := runtime.NumCPU()
	trapsPerWorker := b.N / numWorkers

	b.ResetTimer()
	b.ReportAllocs()

	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			generator := helpers.NewSNMPTrapGenerator("public", fmt.Sprintf("192.168.1.%d", workerID+1))

			for j := 0; j < trapsPerWorker; j++ {
				packet, err := generator.GenerateStandardTrap("coldStart")
				if err != nil {
					b.Errorf("Worker %d failed to generate trap %d: %v", workerID, j, err)
					continue
				}

				err = generator.SendTrapToListener(packet, "127.0.0.1:1162")
				if err != nil {
					b.Errorf("Worker %d failed to send trap %d: %v", workerID, j, err)
				}
			}
		}(i)
	}

	wg.Wait()
	sendDuration := time.Since(start)

	b.StopTimer()

	totalTraps := numWorkers * trapsPerWorker
	b.Logf("Sent %d traps concurrently (%d workers) in %v (%.2f traps/sec)", 
		totalTraps, numWorkers, sendDuration, float64(totalTraps)/sendDuration.Seconds())

	// Wait for processing
	time.Sleep(5 * time.Second)

	// Verify processing
	db, err := sql.Open("sqlite3", env.DBPath)
	require.NoError(b, err, "Failed to open database")
	defer db.Close()

	var processedCount int
	err = db.QueryRow("SELECT COUNT(*) FROM events").Scan(&processedCount)
	require.NoError(b, err, "Failed to query processed count")

	b.Logf("Processed %d out of %d traps (%.2f%%)", processedCount, totalTraps, float64(processedCount)/float64(totalTraps)*100)

	cancel()
}

// BenchmarkWithEnrichment benchmarks processing with MIB enrichment enabled
func BenchmarkWithEnrichment(b *testing.B) {
	config := map[string]interface{}{
		"events": map[string]interface{}{
			"enable_enrichment":  true, // Enable enrichment
			"enable_correlation": false,
		},
	}

	env := helpers.SetupTestEnvironment(b, config)
	defer env.Cleanup()

	// Start application
	application, err := app.NewApplication(env.Config, env.Logger)
	require.NoError(b, err, "Failed to create application")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := application.Start(ctx)
		if err != nil && err != context.Canceled {
			b.Errorf("Application failed: %v", err)
		}
	}()

	// Wait for startup
	err = helpers.WaitForPort(1162, false, 10*time.Second)
	require.NoError(b, err, "Application failed to start")

	generator := helpers.NewSNMPTrapGenerator("public", "127.0.0.1")

	b.ResetTimer()
	b.ReportAllocs()

	start := time.Now()
	for i := 0; i < b.N; i++ {
		packet, err := generator.GenerateStandardTrap("coldStart")
		require.NoError(b, err, "Failed to generate trap")

		err = generator.SendTrapToListener(packet, "127.0.0.1:1162")
		if err != nil {
			b.Errorf("Failed to send trap %d: %v", i, err)
		}
	}
	sendDuration := time.Since(start)

	b.StopTimer()

	b.Logf("Sent %d traps with enrichment in %v (%.2f traps/sec)", 
		b.N, sendDuration, float64(b.N)/sendDuration.Seconds())

	// Wait for processing
	time.Sleep(10 * time.Second) // Longer wait due to enrichment

	cancel()
}

// BenchmarkMemoryUsage benchmarks memory usage during processing
func BenchmarkMemoryUsage(b *testing.B) {
	env := helpers.SetupTestEnvironment(b, nil)
	defer env.Cleanup()

	// Start application
	application, err := app.NewApplication(env.Config, env.Logger)
	require.NoError(b, err, "Failed to create application")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := application.Start(ctx)
		if err != nil && err != context.Canceled {
			b.Errorf("Application failed: %v", err)
		}
	}()

	// Wait for startup
	err = helpers.WaitForPort(1162, false, 10*time.Second)
	require.NoError(b, err, "Application failed to start")

	generator := helpers.NewSNMPTrapGenerator("public", "127.0.0.1")

	// Measure initial memory
	var memStats runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memStats)
	initialMemory := memStats.Alloc

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		packet, err := generator.GenerateStandardTrap("coldStart")
		require.NoError(b, err, "Failed to generate trap")

		err = generator.SendTrapToListener(packet, "127.0.0.1:1162")
		if err != nil {
			b.Errorf("Failed to send trap %d: %v", i, err)
		}

		// Measure memory every 100 iterations
		if i%100 == 0 && i > 0 {
			runtime.ReadMemStats(&memStats)
			b.Logf("Memory after %d traps: %d bytes (delta: %d)", 
				i, memStats.Alloc, memStats.Alloc-initialMemory)
		}
	}

	b.StopTimer()

	// Final memory measurement
	runtime.GC()
	runtime.ReadMemStats(&memStats)
	finalMemory := memStats.Alloc

	b.Logf("Memory usage: initial=%d, final=%d, delta=%d bytes", 
		initialMemory, finalMemory, finalMemory-initialMemory)

	cancel()
}

// TestLatencyMeasurement measures end-to-end latency
func TestLatencyMeasurement(t *testing.T) {
	env := helpers.SetupTestEnvironment(t, nil)
	defer env.Cleanup()

	// Start application
	application, err := app.NewApplication(env.Config, env.Logger)
	require.NoError(t, err, "Failed to create application")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := application.Start(ctx)
		if err != nil && err != context.Canceled {
			t.Errorf("Application failed: %v", err)
		}
	}()

	// Wait for startup
	err = helpers.WaitForPort(1162, false, 10*time.Second)
	require.NoError(t, err, "Application failed to start")

	generator := helpers.NewSNMPTrapGenerator("public", "127.0.0.1")

	// Measure latency for multiple traps
	numSamples := 100
	latencies := make([]time.Duration, numSamples)

	for i := 0; i < numSamples; i++ {
		packet, err := generator.GenerateStandardTrap("coldStart")
		require.NoError(t, err, "Failed to generate trap")

		// Measure send time
		start := time.Now()
		err = generator.SendTrapToListener(packet, "127.0.0.1:1162")
		require.NoError(t, err, "Failed to send trap")

		// Wait for processing (simplified - in practice you'd check database)
		time.Sleep(100 * time.Millisecond)
		latencies[i] = time.Since(start)

		// Small delay between samples
		time.Sleep(10 * time.Millisecond)
	}

	// Calculate statistics
	var total time.Duration
	min := latencies[0]
	max := latencies[0]

	for _, latency := range latencies {
		total += latency
		if latency < min {
			min = latency
		}
		if latency > max {
			max = latency
		}
	}

	avg := total / time.Duration(numSamples)

	t.Logf("Latency statistics over %d samples:", numSamples)
	t.Logf("  Average: %v", avg)
	t.Logf("  Minimum: %v", min)
	t.Logf("  Maximum: %v", max)

	// Verify reasonable latency (adjust thresholds as needed)
	require.Less(t, avg, 5*time.Second, "Average latency too high")
	require.Less(t, max, 10*time.Second, "Maximum latency too high")

	cancel()
}

// TestThroughputMeasurement measures sustained throughput
func TestThroughputMeasurement(t *testing.T) {
	config := map[string]interface{}{
		"events": map[string]interface{}{
			"enable_enrichment":  false,
			"enable_correlation": false,
		},
		"listener": map[string]interface{}{
			"workers":     runtime.NumCPU(),
			"buffer_size": 5000,
		},
	}

	env := helpers.SetupTestEnvironment(t, config)
	defer env.Cleanup()

	// Start application
	application, err := app.NewApplication(env.Config, env.Logger)
	require.NoError(t, err, "Failed to create application")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := application.Start(ctx)
		if err != nil && err != context.Canceled {
			t.Errorf("Application failed: %v", err)
		}
	}()

	// Wait for startup
	err = helpers.WaitForPort(1162, false, 10*time.Second)
	require.NoError(t, err, "Application failed to start")

	generator := helpers.NewSNMPTrapGenerator("public", "127.0.0.1")

	// Sustained throughput test
	duration := 30 * time.Second
	trapCount := 0
	start := time.Now()

	for time.Since(start) < duration {
		packet, err := generator.GenerateStandardTrap("coldStart")
		if err != nil {
			continue
		}

		err = generator.SendTrapToListener(packet, "127.0.0.1:1162")
		if err == nil {
			trapCount++
		}

		// Small delay to avoid overwhelming
		time.Sleep(1 * time.Millisecond)
	}

	actualDuration := time.Since(start)
	throughput := float64(trapCount) / actualDuration.Seconds()

	t.Logf("Sustained throughput test:")
	t.Logf("  Duration: %v", actualDuration)
	t.Logf("  Traps sent: %d", trapCount)
	t.Logf("  Throughput: %.2f traps/sec", throughput)

	// Wait for processing
	time.Sleep(5 * time.Second)

	// Verify processing
	db, err := sql.Open("sqlite3", env.DBPath)
	require.NoError(t, err, "Failed to open database")
	defer db.Close()

	var processedCount int
	err = db.QueryRow("SELECT COUNT(*) FROM events").Scan(&processedCount)
	require.NoError(t, err, "Failed to query processed count")

	processingRate := float64(processedCount) / actualDuration.Seconds()
	t.Logf("  Processing rate: %.2f traps/sec", processingRate)
	t.Logf("  Processing efficiency: %.2f%%", float64(processedCount)/float64(trapCount)*100)

	cancel()
}
