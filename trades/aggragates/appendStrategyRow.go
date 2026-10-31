package aggragates

// AppendStrategyRow is the trade with a gate's log row and its strategy event
// appended, the pair every writer that runs before the action chain appends.
// Both slices are copied: the trade value aliases the engine's own arrays,
// which another reader may still hold.
func AppendStrategyRow(trade Trades, row TradesLogs, event TradesStrategyEvents) Trades {
	return AppendStrategyRows(trade, []TradesLogs{row}, []TradesStrategyEvents{event})
}

// AppendStrategyRows is AppendStrategyRow for a tick's log rows and the events beside them, whose
// counts may differ (a merged log row stands for several events), both slices copied.
func AppendStrategyRows(trade Trades, rows []TradesLogs, events []TradesStrategyEvents) Trades {
	trade.Logs = append(append([]TradesLogs(nil), trade.Logs...), rows...)
	trade.StrategyEvents = append(append([]TradesStrategyEvents(nil), trade.StrategyEvents...), events...)

	return trade
}
