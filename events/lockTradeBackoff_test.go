package events

import (
	"sync"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// TestLockTradeWithBackOffIsRaceFree runs the backoff path in many goroutines
// at once. The prior implementation released and re-acquired rwLocker around
// Storage.Set via defer LIFO, leaving backoffTries exposed between the two
// locks. Under `go test -race` that race panicked the test binary. The
// current shape computes everything under a single Lock and releases before
// calling LockTrade; this test will fail only if that invariant regresses.
//
// Storage is left nil — LockTrade runs even without one and returns an
// error, which is fine for exercising the lock path.
func TestLockTradeWithBackOffIsRaceFree(t *testing.T) {
	const workers = 50
	const tradesPerWorker = 10

	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(base uint) {
			defer wg.Done()
			for i := 0; i < tradesPerWorker; i++ {
				ev := Events{Trade: aggragates.Trades{ID: base + uint(i)}}
				// We accept the nil-Storage error — we only care that the
				// function does not race on the backoffTries map.
				_ = ev.LockTradeWithBackOff()
			}
		}(uint(w * tradesPerWorker))
	}
	wg.Wait()
}

// TestLockTradeWithBackOffGrowsExponentially pins the backoff schedule so a
// future refactor that tweaks the math surfaces in this test instead of
// showing up as stuck trades in production.
func TestLockTradeWithBackOffGrowsExponentially(t *testing.T) {
	// Use a dedicated trade id so we don't collide with other tests in the package.
	tradeID := uint(999999)

	// Ensure a clean starting state across test re-runs.
	rwLocker.Lock()
	delete(backoffTries, tradeID)
	rwLocker.Unlock()
	t.Cleanup(func() {
		rwLocker.Lock()
		delete(backoffTries, tradeID)
		rwLocker.Unlock()
	})

	ev := Events{Trade: aggragates.Trades{ID: tradeID}}

	expected := []time.Duration{
		startingBackOff,
		startingBackOff * 2,
		startingBackOff * 4,
		startingBackOff * 8,
	}
	for i, want := range expected {
		_ = ev.LockTradeWithBackOff()
		rwLocker.RLock()
		got := backoffTries[tradeID]
		rwLocker.RUnlock()
		if got != want {
			t.Fatalf("iteration %d: got backoff %s, want %s", i, got, want)
		}
	}

	// Keep going until we hit the cap.
	for i := 0; i < 20; i++ {
		_ = ev.LockTradeWithBackOff()
	}
	rwLocker.RLock()
	got := backoffTries[tradeID]
	rwLocker.RUnlock()
	if got != maxBackOff {
		t.Fatalf("expected backoff to cap at %s, got %s", maxBackOff, got)
	}
}
