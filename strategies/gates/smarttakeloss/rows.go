package smarttakeloss

import (
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// Rows turns a Row into the pair the engines write, the INFO log row and the strategy event, both
// stamped with the engine's own clock so (TradeID, CreatedAt) pairs them; the row is the output,
// the event the state.
func Rows(trade aggragates.Trades, row Row, at time.Time) (aggragates.TradesLogs, aggragates.TradesStrategyEvents) {
	return logRowOf(trade, row, at), strategyEventOf(trade, row, at)
}

// TickRows turns a tick's rows into what the engines write: one event per row, in order, and one
// log row per gate whose rows merge (logLines), else per row, all stamped at.
func TickRows(trade aggragates.Trades, rows []Row, at time.Time) ([]aggragates.TradesLogs, []aggragates.TradesStrategyEvents) {
	lines := logLines(rows)
	logs := make([]aggragates.TradesLogs, 0, len(lines))
	for _, line := range lines {
		logs = append(logs, logRowOf(trade, line, at))
	}
	events := make([]aggragates.TradesStrategyEvents, 0, len(rows))
	for _, row := range rows {
		events = append(events, strategyEventOf(trade, row, at))
	}
	return logs, events
}

// logRowOf is a row's INFO log row stamped at.
func logRowOf(trade aggragates.Trades, row Row, at time.Time) aggragates.TradesLogs {
	return aggragates.TradesLogs{
		TradeID:   trade.ID,
		Message:   row.Message,
		Type:      aggragates.LOG_INFO,
		Price:     row.Price,
		CreatedAt: at,
		UpdatedAt: at,
	}
}

// strategyEventOf is a row's strategy event stamped at.
func strategyEventOf(trade aggragates.Trades, row Row, at time.Time) aggragates.TradesStrategyEvents {
	data := EventData{Event: row.Event, Price: row.Price, Reasons: row.Reasons, Features: row.Features, Trigger: row.Trigger}
	return aggragates.NewStrategyEvent(trade.ID, aggragates.StrategyParamSmartTakeLoss, row.Gate, data, at)
}

// Append is the trade with the pair Rows builds appended (aggragates.AppendStrategyRow), both
// slices copied; the fixtures build state with it.
func Append(trade aggragates.Trades, row Row, at time.Time) aggragates.Trades {
	logRow, event := Rows(trade, row, at)

	return aggragates.AppendStrategyRow(trade, logRow, event)
}
