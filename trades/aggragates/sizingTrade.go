package aggragates

// SizingTrade is the trade a first entry is sized from: trade itself, unless
// EntrySettings is set and trade has no fill yet — no history row, the
// first-entry predicate HasFunds reads — and then a copy of trade carrying
// EntrySettings as its rows. Only the copy's own slice header changes: the
// rows trade carries, and the array behind them, are never written, and the
// copy is the sizing reader's alone — it never reaches a chain, a log row or
// storage. A trade with fills comes back as it is, so an add read through it
// stays on the trade's rows.
//
// It is the first entry's rows alone. The ceiling a ladder is measured
// against — its depths and what they cost — is read on the rows it trades by
// the depth readers themselves (ladder.ConfiguredDepths, ladder.DepthOf), off
// the trade's own logs, and never through here.
func (p Params) SizingTrade(trade Trades) Trades {
	if len(p.EntrySettings) == 0 || len(trade.History) > 0 {
		return trade
	}

	trade.StrategyPair.StrategySettings = p.EntrySettings
	return trade
}
