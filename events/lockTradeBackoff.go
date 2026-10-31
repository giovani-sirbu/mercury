package events

import (
	"fmt"
	"sync"
	"time"

	"github.com/giovani-sirbu/mercury/helpers"
)

// backoffEntry is one trade's backoff: the current duration, when it was last
// touched (the sweeper evicts entries older than backoffEntryTTL) and the
// failure this episode already logged, numbers stripped.
type backoffEntry struct {
	duration    time.Duration
	lastTouched time.Time
	lastFailure string
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

// sweepBackoffTries evicts the entries older than backoffEntryTTL every backoffSweepRate.
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

// LockTrade sets the trade lock for lockDuration, straight to Redis so the
// readers' Memory.Exists sees it at once; a nil Storage (the backtest) is a no-op.
func (e Events) LockTrade(lockDuration time.Duration) error {
	if e.Storage == nil {
		return nil
	}
	lockKey := fmt.Sprintf("trade:%d:is_locked", e.Trade.ID)
	return e.Storage.Set(lockKey, true, lockDuration)
}

// LockTradeWithBackOff locks the trade for a duration that doubles per
// failure from startingBackOff up to maxBackOff; the duration is computed
// under rwLocker and the Set issued outside it.
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
	backoffTries[e.Trade.ID] = backoffEntry{duration: lockDuration, lastTouched: time.Now(), lastFailure: current.lastFailure}
	rwLocker.Unlock()

	return e.LockTrade(lockDuration)
}

// repeatsLastFailure reports whether err is the failure this trade's backoff
// episode already logged (same text, numbers stripped) and records it
// otherwise; the episode ends when Next clears the entry or the sweeper
// evicts it, so a failure that returns after a success is logged again, once.
func (e Events) repeatsLastFailure(err error) bool {
	failure := helpers.RemoveNumbersFromString(err.Error())

	rwLocker.Lock()
	defer rwLocker.Unlock()

	entry := backoffTries[e.Trade.ID]
	if entry.lastFailure == failure {
		return true
	}

	entry.lastFailure = failure
	backoffTries[e.Trade.ID] = entry

	return false
}
