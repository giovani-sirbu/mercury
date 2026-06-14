package events

import (
	"fmt"
	"sync"
	"time"
)

// backoffEntry tracks both the current backoff duration and when it was last
// touched, so the sweeper can evict trades that have been stuck erroring for
// longer than backoffEntryTTL. Previously the map only shrunk on a successful
// Next() — a permanently failing trade (deleted strategy, exchange API key
// revoked) kept its entry forever, slowly leaking memory.
type backoffEntry struct {
	duration    time.Duration
	lastTouched time.Time
}

var (
	backoffTries = make(map[uint]backoffEntry)
	rwLocker     sync.RWMutex
)

const (
	startingBackOff  = 1 * time.Second
	maxBackOff       = 60 * time.Second
	backoffEntryTTL  = 30 * time.Minute
	backoffSweepRate = 5 * time.Minute
)

func init() {
	go sweepBackoffTries()
}

// sweepBackoffTries periodically evicts backoffEntry rows older than
// backoffEntryTTL. The pass is cheap (one Lock, one range over the whole map)
// and the cadence is conservative so it never shows up in profiles.
func sweepBackoffTries() {
	t := time.NewTicker(backoffSweepRate)
	defer t.Stop()
	for range t.C {
		cutoff := time.Now().Add(-backoffEntryTTL)
		rwLocker.Lock()
		for id, entry := range backoffTries {
			if entry.lastTouched.Before(cutoff) {
				delete(backoffTries, id)
			}
		}
		rwLocker.Unlock()
	}
}

// LockTrade locks the trade with a specified duration.
//
// Nil Storage is treated as a no-op because some callers (notably backtesting
// in sisyphus, which passes a zero-value events.Events) do not populate a
// cache client. Previously that path panicked when it hit the backoff — now
// it silently skips the lock and returns nil so the caller's retry logic
// can proceed.
//
// Writes via Memory.Set, which is Redis-only by default. The lock is consumed
// exclusively via Memory.Exists from the reader side — both sides operate on
// Redis directly, so cross-process visibility is immediate.
func (e Events) LockTrade(lockDuration time.Duration) error {
	if e.Storage == nil {
		return nil
	}
	lockKey := fmt.Sprintf("trade:%d:is_locked", e.Trade.ID)
	return e.Storage.Set(lockKey, true, lockDuration)
}

// LockTradeWithBackOff locks the trade with an exponential backoff strategy.
//
// The prior implementation released rwLocker before calling Storage.Set and
// then deferred a re-acquire; combined with the outer `defer rwLocker.Unlock`,
// the lock-ordering was relying on defer LIFO and left a window where the
// map was readable from other goroutines between the two locks. This version
// computes the new duration entirely under the lock, releases it, then issues
// the (independent, thread-safe) Storage.Set outside the critical section.
func (e Events) LockTradeWithBackOff() error {
	rwLocker.Lock()
	current := backoffTries[e.Trade.ID]

	var lockDuration time.Duration
	if current.duration < startingBackOff {
		lockDuration = startingBackOff
	} else {
		lockDuration = current.duration * 2
		if lockDuration > maxBackOff {
			lockDuration = maxBackOff
		}
	}
	backoffTries[e.Trade.ID] = backoffEntry{duration: lockDuration, lastTouched: time.Now()}
	rwLocker.Unlock()

	return e.LockTrade(lockDuration)
}
