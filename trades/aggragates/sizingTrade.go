package aggragates

// SizingTrade is the trade a first entry is sized from: trade itself, unless
// EntrySettings is set and trade has no fill yet — no history row, the
// first-entry predicate HasFunds reads — and then a copy of trade carrying
// EntrySettings as its rows. Only the copy's own slice header changes: the
// rows trade carries, and the array behind them, are never written, and the
// copy is the sizing reader's alone — it never reaches a chain, a log row or
// storage. A trade with fills comes back as it is, so an add read through it
// stays on the trade's rows.
func (p Params) SizingTrade(trade Trades) Trades {
	if len(p.EntrySettings) == 0 || len(trade.History) > 0 {
		return trade
	}

	trade.StrategyPair.StrategySettings = p.EntrySettings
	return trade
}
