package events

import (
	"fmt"
	"time"
)

var backoffTries = make(map[uint]time.Duration)

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
		lockDuration = backoffTries[e.Trade.ID] * 2
		if lockDuration > maxBackOff {
			lockDuration = maxBackOff
		}
	}
	backoffTries[e.Trade.ID] = lockDuration
	e.LockTrade(lockDuration)
}
