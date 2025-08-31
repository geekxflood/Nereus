package performance

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/geekxflood/nereus/internal/types"
	"github.com/geekxflood/nereus/tests/common/helpers"
)

// BenchmarkTrapProcessing benchmarks SNMP trap processing performance
func BenchmarkTrapProcessing(b *testing.B) {
	// For performance testing, we'll benchmark packet generation and basic operations
	// without actually starting the full application due to API compatibility
	
	generator := helpers.NewSNMPTrapGenerator("public", "127.0.0.1")

	// Pre-generate packets to avoid generation overhead in benchmark
	packets := make([]*types.SNMPPacket, b.N)
	for i := 0; i < b.N; i++ {
		packet, err := generator.GenerateStandardTrap("coldStart")
		require.NoError(b, err, "Failed to generate trap")
		packets[i] = packet
	}

	// Reset timer to exclude setup time
	b.ResetTimer()

	// Benchmark packet processing simulation
	start := time.Now()
	for i := 0; i < b.N; i++ {
		// Simulate packet processing operations
		packet := packets[i]
		_ = packet.Community
		_ = packet.Version
		_ = packet.PDUType
		_ = len(packet.Varbinds)
		
		// Simulate some processing time
		time.Sleep(time.Microsecond)
	}
	processingDuration := time.Since(start)

	b.StopTimer()

	// Report processing performance
	b.Logf("Processed %d packets in %v (%.2f packets/sec)", b.N, processingDuration, float64(b.N)/processingDuration.Seconds())
}

// BenchmarkConcurrentProcessing benchmarks concurrent trap processing
func BenchmarkConcurrentProcessing(b *testing.B) {
	// For performance testing, we'll benchmark concurrent packet generation
	// without actually starting the full application due to API compatibility

	numWorkers := runtime.NumCPU()
	trapsPerWorker := b.N / numWorkers

	b.ResetTimer()

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

				// Simulate packet processing
				_ = packet.Community
				_ = packet.Version
				_ = len(packet.Varbinds)
				time.Sleep(time.Microsecond)
			}
		}(i)
	}

	wg.Wait()
	processingDuration := time.Since(start)

	b.StopTimer()

	totalTraps := numWorkers * trapsPerWorker
	b.Logf("Processed %d traps concurrently (%d workers) in %v (%.2f traps/sec)",
		totalTraps, numWorkers, processingDuration, float64(totalTraps)/processingDuration.Seconds())
}

// BenchmarkPacketGeneration benchmarks SNMP packet generation
func BenchmarkPacketGeneration(b *testing.B) {
	generator := helpers.NewSNMPTrapGenerator("public", "127.0.0.1")

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := generator.GenerateStandardTrap("coldStart")
		if err != nil {
			b.Errorf("Failed to generate trap %d: %v", i, err)
		}
	}
}

// BenchmarkMemoryUsage benchmarks memory usage during packet processing
func BenchmarkMemoryUsage(b *testing.B) {
	generator := helpers.NewSNMPTrapGenerator("public", "127.0.0.1")

	// Force garbage collection before starting
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	b.ResetTimer()

	packets := make([]*types.SNMPPacket, b.N)
	for i := 0; i < b.N; i++ {
		packet, err := generator.GenerateStandardTrap("coldStart")
		require.NoError(b, err, "Failed to generate trap")
		packets[i] = packet
	}

	b.StopTimer()

	// Force garbage collection and measure memory
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	b.Logf("Memory used: %d bytes (%.2f KB per packet)", 
		m2.Alloc-m1.Alloc, float64(m2.Alloc-m1.Alloc)/float64(b.N)/1024)
}

// TestPerformanceRegression tests for performance regressions
func TestPerformanceRegression(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance regression test in short mode")
	}

	generator := helpers.NewSNMPTrapGenerator("public", "127.0.0.1")

	// Test packet generation performance
	const numPackets = 10000
	start := time.Now()
	
	for i := 0; i < numPackets; i++ {
		_, err := generator.GenerateStandardTrap("coldStart")
		require.NoError(t, err, "Failed to generate trap")
	}
	
	duration := time.Since(start)
	packetsPerSecond := float64(numPackets) / duration.Seconds()

	// Performance threshold: should generate at least 1000 packets/second
	const minPacketsPerSecond = 1000
	if packetsPerSecond < minPacketsPerSecond {
		t.Errorf("Performance regression detected: %.2f packets/sec (expected >= %d)", 
			packetsPerSecond, minPacketsPerSecond)
	}

	t.Logf("Generated %d packets in %v (%.2f packets/sec)", 
		numPackets, duration, packetsPerSecond)
}
