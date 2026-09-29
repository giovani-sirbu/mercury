// Package dynamicparams is the DynamicParams flag: while the market reads
// bearish on a higher timeframe, a ladder trades wider and — when it opens
// while the market does — deeper.
//
// Sophos reads two rows of its SMC trend dashboard on the one timeframe its
// dynamicparams.Timeframe names — the Super Guppy and the Bull Market Support
// Band (BMSB) — and serves both reads on GET /:symbol/patterns
// (aggragates.DynamicParamsIndicators). Only a bearish read counts (TierOf):
// both bearish is TierBothBearish, exactly one is TierMixed, and anything
// else — a neutral read, a bullish one, a timeframe sophos did not read — is
// TierBase. Raise turns the tier into an Increase: both bearish raises the
// percentage and the depths (IncreaseBoth), mixed raises what MixedIncrease
// names, and the base tier raises nothing (IncreaseNone).
//
// Adjust applies the increase to every settings row of the ladder: the
// percentage by BearPercentagePoints — points added to the row's own
// percentage, never a multiple of it — and the depths by BearDepths. The
// raised percentage is the row's own, so it moves the next entry and the take
// profit alike. The reads are taken afresh on every tick and nothing
// latches: the first tick they stop reading bearish, the ladder is back on
// its configured rows.
//
// The raised rows reach exactly two places, both handed over per tick by the
// engines from RaisedSettings: the strategies.Strategy.Settings the position
// is computed from, and aggragates.Params.EntrySettings, which the first entry
// of a ladder that opens while raised is sized with
// (aggragates.Params.SizingTrade). The extra depths therefore exist only for
// such a ladder: a running one gains no funding. Every other reader keeps the
// rows the trade stores — the depth priority's own depth and its view of the
// wallet's ladders, the smart take loss's last depth, regulatePriceChange,
// the minimum profit, the hold levels, the backtest's skip gates and every
// entry after the first. Nothing is written: Adjust returns a copy and never
// touches the stored rows, which the backtest shares with its run's snapshot
// and writes back to memory after every tick, so a raise cannot compound from
// one tick to the next.
//
// The flag applies to long spot parent ladders only (Applies). It holds
// nothing and gates no entry; it only fetches the /patterns route its reads
// ride on (aggragates.StrategyParams.NeedsPatternRoute). The engines write
// one row per trade on each change of the increase the reads raise (Changed),
// carrying TransitionMessage.
//
// Vocabulary: row, depth, percentage, timeframe, read, tier, increase,
// raised, configured rows.
package dynamicparams
