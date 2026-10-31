package events

import "errors"

// ErrTradeHeld marks a chain stopped by a hold decision rather than a
// failure: it stops the chain but is neither logged as an error nor backed
// off, since the hold is already an INFO row on the trade and repeats at tick rate.
var ErrTradeHeld = errors.New("trade held")

// ErrHoldNotPersisted accompanies ErrTradeHeld when the hold wrote nothing
// (its reason already stood on the trade) and the chain stopped before
// updateTrade: nothing was published, so a lock the caller acquired for the
// run is still held and only the caller can release it.
var ErrHoldNotPersisted = errors.New("hold not persisted")
