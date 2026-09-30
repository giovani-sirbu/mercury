package smarttakeloss

import (
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// Rows is the one writer the three engines share: it turns a Row Apply handed
// back into the pair to write — the INFO trade-log row and the strategy event
// beside it — both stamped with the engine's own clock (wall time in hermes
// and live-testing, the simulated tick in backtesting). Sharing one stamp is
// what pairs them: an event exists exactly when its row does, and
// (TradeID, CreatedAt) finds the pair. The same writer serves the exit row
// (ExitRow).
//
// The log row is the human-readable output: Message at the row's own price,
// never trade.PositionPrice. The event is the state: EventData of the row's
// kind, price and reasons, filed under the row's gate. The engines only append
// or persist the pair.
func Rows(trade aggragates.Trades, row Row, at time.Time) (aggragates.TradesLogs, aggragates.TradesStrategyEvents) {
	logRow := aggragates.TradesLogs{
		TradeID:   trade.ID,
		Message:   row.Message,
		Type:      aggragates.LOG_INFO,
		Price:     row.Price,
		CreatedAt: at,
		UpdatedAt: at,
	}
	data := EventData{Event: row.Event, Price: row.Price, Reasons: row.Reasons}
	event := aggragates.NewStrategyEvent(trade.ID, aggragates.StrategyParamSmartTakeLoss, row.Gate, data, at)

	return logRow, event
}

// Append is the trade with the pair Rows builds appended, the way every
// writer that runs before the action chain appends a pair
// (aggragates.AppendStrategyRow): both slices are copied, so the trade the
// caller holds is never written through. It is how the fixtures build the state
// a fold reads, with the very function the engines write it with.
func Append(trade aggragates.Trades, row Row, at time.Time) aggragates.Trades {
	logRow, event := Rows(trade, row, at)

	return aggragates.AppendStrategyRow(trade, logRow, event)
}
