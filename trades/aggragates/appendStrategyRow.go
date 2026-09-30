package aggragates

// AppendStrategyRow is the trade with a gate's log row and the strategy event
// written beside it appended: the pair every writer that runs BEFORE the
// action chain appends, so a row never goes without its event.
//
// Both slices are copied on append. The trade value the writer holds aliases
// the backing arrays of the engine's own copy (the backtest's memory trade,
// hermes' cached one), and appending in place could write into an array
// another reader still holds. The writers inside the chain append in place
// instead, on the event they hand on.
func AppendStrategyRow(trade Trades, row TradesLogs, event TradesStrategyEvents) Trades {
	trade.Logs = append(append([]TradesLogs(nil), trade.Logs...), row)
	trade.StrategyEvents = append(append([]TradesStrategyEvents(nil), trade.StrategyEvents...), event)

	return trade
}
