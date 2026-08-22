package events

import "errors"

// ErrTradeHeld marks a chain stopped by a deliberate hold decision rather than
// by a failure. It still stops the chain — a hold means "do not act on this
// tick" — but it must not be logged as an error: the hold is already recorded
// as an INFO entry on the trade, and at tick rate the same hold repeats for as
// long as the market condition lasts. Logging it produced ~180k identical
// lines a minute during a backtest, which costs more than the decision itself.
var ErrTradeHeld = errors.New("trade held")
