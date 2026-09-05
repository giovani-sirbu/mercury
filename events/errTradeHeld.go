package events

import "errors"

// ErrTradeHeld marks a chain stopped by a deliberate hold decision rather than
// by a failure. It still stops the chain — a hold means "do not act on this
// tick" — but it must not be logged as an error: the hold is already recorded
// as an INFO entry on the trade, and at tick rate the same hold repeats for as
// long as the market condition lasts. Logging it produced ~180k identical
// lines a minute during a backtest, which costs more than the decision itself.
var ErrTradeHeld = errors.New("trade held")

// ErrHoldNotPersisted accompanies ErrTradeHeld when the hold wrote nothing:
// the reason already stood on the trade, so SaveHoldLog stopped the chain
// before updateTrade. Nothing was published, so anything the caller acquired
// for this run is still held and only the caller can release it — hermes
// takes a 24h trade lock whose sole release is agora's update-trade consumer,
// and without this signal a repeat hold wedged the trade for the full TTL.
// Callers that acquire nothing (the backtest, live testing) ignore it.
var ErrHoldNotPersisted = errors.New("hold not persisted")
