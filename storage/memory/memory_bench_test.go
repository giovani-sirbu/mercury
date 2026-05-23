package memory

import (
	"net"
	"os"
	"testing"
	"time"
)

// redisAvailable returns true when a Redis-compatible server answers on REDIS_URL (or localhost:6379).
// Benchmarks skip silently when no server is available so this file is safe to run in any environment.
func redisAvailable(tb testing.TB) string {
	tb.Helper()
	addr := os.Getenv("REDIS_URL")
	if addr == "" {
		addr = "localhost:6379"
	}
	conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
	if err != nil {
		tb.Skipf("Redis/Dragonfly not reachable at %s: %v", addr, err)
		return ""
	}
	_ = conn.Close()
	return addr
}

// BenchmarkMemoryGet measures the default (Redis-only) Get path. Every call
// goes over the wire — no local TinyLFU consult. Compare against
// BenchmarkLocalViewGet to see the cost of the cross-process-correctness
// guarantee we trade for here.
func BenchmarkMemoryGet(b *testing.B) {
	addr := redisAvailable(b)
	m := Memory{Address: []string{addr}, PoolSize: 3}

	key := "bench:baseline:get"
	if err := m.Set(key, "baseline-value", time.Minute); err != nil {
		b.Fatalf("seed Set failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out string
		_ = m.Get(key, &out)
	}
}

// BenchmarkMemorySet measures the default (Redis-only) Set path. Writes go
// straight to Redis without touching TinyLFU.
func BenchmarkMemorySet(b *testing.B) {
	addr := redisAvailable(b)
	m := Memory{Address: []string{addr}, PoolSize: 3}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.Set("bench:baseline:set", i, time.Minute)
	}
}

// BenchmarkMemoryGetParallel exposes connection-pool contention under
// concurrent load — roughly simulates a hermes replica serving WS ticks from
// many goroutines, every one of which now pays a Redis round trip per read.
func BenchmarkMemoryGetParallel(b *testing.B) {
	addr := redisAvailable(b)
	m := Memory{Address: []string{addr}, PoolSize: 10}

	key := "bench:baseline:parallel"
	if err := m.Set(key, map[string]float64{"BTC/USDT": 65000.5, "ETH/USDT": 3500.75}, time.Minute); err != nil {
		b.Fatalf("seed Set failed: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			var out map[string]float64
			_ = m.Get(key, &out)
		}
	})
}

// BenchmarkLocalViewGet measures the opt-in local-cache Get path (Memory.Local()).
// After the first read populates TinyLFU, subsequent reads are served from
// in-process memory without a Redis hop. Use this to decide whether a hot
// single-writer key is worth migrating from Memory.Get to Memory.Local().Get —
// look for at least an order-of-magnitude difference vs BenchmarkMemoryGet to
// justify the opt-in.
func BenchmarkLocalViewGet(b *testing.B) {
	addr := redisAvailable(b)
	m := Memory{Address: []string{addr}, PoolSize: 3}

	key := "bench:local:get"
	if err := m.Local().Set(key, "baseline-value", time.Minute); err != nil {
		b.Fatalf("seed LocalView.Set failed: %v", err)
	}

	// Warm the local cache so subsequent reads measure the steady-state hot path.
	var warm string
	_ = m.Local().Get(key, &warm)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out string
		_ = m.Local().Get(key, &out)
	}
}
