// Package dynamicparams is the DynamicParams flag: a ladder that opens while
// the market reads bearish on a higher timeframe trades wider — and, when the
// increase names them, deeper — rows for its whole life.
//
// The reads are two cells of the chart row of sophos' SMC trend dashboard —
// the Super Guppy and the Bull Market Support Band (BMSB) — which the engines
// request on GET /:symbol/smc-trend at Interval, so they read the last closed
// bar of that interval (aggragates.SophosSmcTrend.DynamicParams,
// aggragates.DynamicParamsIndicators). Only a bearish read counts (TierOf):
// both bearish is TierBothBearish, exactly one is TierMixed, and anything
// else — a neutral read, a bullish one, a timeframe sophos did not read — is
// TierBase. Raise turns the tier into an Increase: both bearish raises the
// percentage and the depths (IncreaseBoth), mixed raises what MixedIncrease
// names, and the base tier raises nothing (IncreaseNone).
//
// The reads are consulted once per ladder, when it opens (Opening): on the
// ticks that judge a trade the flag shapes before its first entry fills and
// before it carries an opened row. When the increase raises something the
// engine writes ONE INFO row on the trade, the opened row (OpenedMessage),
// carrying the amounts the ladder trades: BearPercentagePoints added to every
// row's percentage and BearDepths to every row's depths, only the parts the
// increase names. A first entry that is held or refused funds is judged
// again on later ticks, the reads with it, until the ladder opens; once the
// trade carries its row, no second one is written.
//
// From then on every tick rebuilds the ladder's rows from its own opened row
// (OpenedRaise, RaisedSettings), the way smarttakeloss.rebuildState rebuilds
// its state: the stored rows raised by the amounts the row carries (RaiseBy),
// with no reads and nothing kept anywhere but the row. A change of the reads
// while the ladder is open changes nothing and writes nothing, a retune of
// the constants reaches only the next ladder that opens, and a ladder that
// opened without an opened row trades its configured rows until it closes.
// The next ladder consults the reads again when it opens.
//
// The raised rows reach two kinds of reader. The engines hand them per tick
// from RaisedSettings to the strategies.Strategy.Settings the position is
// computed from and to aggragates.Params.EntrySettings, which the first entry
// of a ladder that opens raised is sized with (aggragates.Params.SizingTrade):
// the extra depths therefore exist only for such a ladder, a running one
// gains no funding. And the readers of the ladder's DEPTHS take them off the
// trade's own logs, by calling RaisedSettings themselves:
// ladder.ConfiguredDepths is the ceiling of the raised rows, ladder.RemainingCost
// walks the depths the row added at the raised percentage, and ladder.DepthOf
// carries both into the wallet view of the cooldown depth priority, which every
// surface builds with it — the managed trade's own reading, sisyphus's
// backtest and live-testing engines, agora for hermes.
//
// A ladder that opened raised is therefore full only at its raised ceiling and
// keeps the wallet for the depths its row added, on whichever surface asks.
// Measured against the stored rows it would read as full at the depth its own
// first entry was sized to go past, reserve nothing for its largest entries,
// and be funds-blocked on them once its siblings have spent the wallet. The
// smart take loss's last depth reads the ceiling through ladder.ConfiguredDepths
// and follows it. Every other reader keeps the rows the trade stores: the cost
// of an add (ladder.NextEntryCost), which only the multiplier shapes,
// regulatePriceChange, the minimum profit, the hold levels, the backtest's skip
// gates and every entry after the first. RaiseBy returns a copy and never
// touches the stored rows, which the backtest shares with its run's snapshot
// and writes back to memory after every tick, so a raise cannot compound from
// one tick to the next.
//
// The flag applies to long spot parent ladders only (Applies). It holds
// nothing and gates no entry; it only fetches the /smc-trend route its reads
// come from (aggragates.StrategyParams.NeedsSmcTrendRoute), which the engines
// set on the tick's verdict after merging its other legs. agora keeps the
// rows carrying RowPrefix off the users' notifications.
//
// Vocabulary: row, depth, percentage, timeframe, read, tier, increase,
// raised, configured rows, opened row.
package dynamicparams
