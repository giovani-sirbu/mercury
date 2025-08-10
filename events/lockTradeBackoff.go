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
func (e Events) LockTrade(lockDuration time.Duration) error {
	lockKey := fmt.Sprintf("trade:%d:is_locked", e.Trade.ID) // Create lock trade key
	//log.Debug(lockKey, lockDuration)
	return e.Storage.Set(lockKey, true, lockDuration)
}

// LockTradeWithBackOff locks the trade with an exponential backoff strategy.
func (e Events) LockTradeWithBackOff() error {
	rwLocker.Lock()
	defer rwLocker.Unlock() // Unlock when function exits

	// Get current backoff duration or initialize it
	currentBackoff, exists := backoffTries[e.Trade.ID]
	if !exists {
		currentBackoff = 0 // Explicitly initialize if not present
	}

	// Calculate new lock duration
	var lockDuration time.Duration
	if currentBackoff < startingBackOff {
		lockDuration = startingBackOff
	} else {
		lockDuration = currentBackoff * 2
		if lockDuration > maxBackOff {
			lockDuration = maxBackOff
		}
	}

	// Update backoffTries with the new duration
	backoffTries[e.Trade.ID] = lockDuration

	// Perform the lock (outside the lock if Storage.Set is thread-safe)
	rwLocker.Unlock()     // Unlock before calling LockTrade to avoid holding lock during Storage.Set
	defer rwLocker.Lock() // Re-lock to ensure backoffTries is safe after this point
	return e.LockTrade(lockDuration)
}
