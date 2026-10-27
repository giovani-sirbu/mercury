package aggragates

// SizingTrade is the trade a first entry is sized from: trade itself, or,
// when EntrySettings is set and trade has no fill yet, a copy carrying
// EntrySettings as its rows. The copy is the sizing reader's alone and never
// reaches a chain or storage; a trade with fills comes back as it is. The
// depth readers (ladder.ConfiguredDepths, ladder.DepthOf) read the raise off
// the trade's own strategy events, never through here.
func (p Params) SizingTrade(trade Trades) Trades {
	if len(p.EntrySettings) == 0 || len(trade.History) > 0 {
		return trade
	}

	trade.StrategyPair.StrategySettings = p.EntrySettings
	return trade
}
