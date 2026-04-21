package events

import (
	"fmt"
	"sync"
	"time"
)

var (
	backoffTries = make(map[uint]time.Duration)
	rwLocker     sync.RWMutex
)

const (
	startingBackOff = 1 * time.Second
	maxBackOff      = 60 * time.Second
)

// LockTrade locks the trade with a specified duration.
//
// Nil Storage is treated as a no-op because some callers (notably backtesting
// in sisyphus, which passes a zero-value events.Events) do not populate a
// cache client. Previously that path panicked when it hit the backoff — now
// it silently skips the lock and returns nil so the caller's retry logic
// can proceed.
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
	currentBackoff := backoffTries[e.Trade.ID]

	var lockDuration time.Duration
	if currentBackoff < startingBackOff {
		lockDuration = startingBackOff
	} else {
		lockDuration = currentBackoff * 2
		if lockDuration > maxBackOff {
			lockDuration = maxBackOff
		}
	}
	backoffTries[e.Trade.ID] = lockDuration
	rwLocker.Unlock()

	return e.LockTrade(lockDuration)
}
