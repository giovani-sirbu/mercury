package events

import (
	"fmt"
	"sync"
	"time"
)

var backoffTries = make(map[uint]time.Duration)
var rwLocker sync.RWMutex

const startingBackOff = 1 * time.Second
const maxBackOff = 60 * time.Second

func (e Events) LockTrade(time time.Duration) error {
	lockKey := fmt.Sprintf("trade:%d:is_locked", e.Trade.ID) // Create lock trade key
	err := e.Storage.Set(lockKey, true, time)
	return err
}

func (e Events) LockTradeWithBackOff() {
	var lockDuration time.Duration

	if backoffTries[e.Trade.ID] < startingBackOff {
		lockDuration = startingBackOff
	} else {
		rwLocker.RLock()
		lockDuration = backoffTries[e.Trade.ID] * 2
		rwLocker.RUnlock()
		if lockDuration > maxBackOff {
			lockDuration = maxBackOff
		}
	}
	rwLocker.Lock()
	backoffTries[e.Trade.ID] = lockDuration
	rwLocker.Unlock()
	e.LockTrade(lockDuration)
}
