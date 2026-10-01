// Package smarttakeloss is the SmartTakeLoss flag: three rules read off the
// closed window of sophos' smart take loss, each behind its own switch. Two
// turn a long ladder into a seller instead of a buyer; the third moves where
// it takes profit.
//
// QuietSlowDeclineExit switches the quiet slow-decline exit. Sophos reads an
// early down leg on that window, still down sophos' SlowDeclineMinLegFallPct
// from its high close, and takes a vote at its last bar among the readings it
// switches in — the recent volume, the volatility and the Bollinger band's
// width under their own baselines, and the fall smooth — with its SMC trend
// dashboard, read at the same bar, bearish on every timeframe sophos'
// SmcTrendTimeframes names while that condition is on, and serves the sell
// band of that window — a Bollinger band — beside the verdict. The verdict
// is that vote passing with the smoothness over sophos' own span, so it
// passes for every ladder; the leg on and quiet is that vote passing with a
// ladder's own smoothness, for which sophos also serves the open time of the
// earliest bar it reads the leg smooth from, so the gate reads a ladder's
// smoothness from the ladder's newest fill: a fast drop that filled depths
// breaks the smooth trend, and every new fill restarts the count. The fill
// counts once the closed bars sophos requires after the bar that holds it
// stand (the fill stamped before SlowDeclineFillBefore) — at none, once that
// bar has closed. Sophos also looks back over its last SlowDeclineRecentBars
// closed bars and serves the newest one the verdict stood on
// (SlowDeclineRecentAt) with the oldest of them (SlowDeclineRecentFrom): the
// decline reads RECENTLY for a ladder whose newest fill came within those
// bars, a fill in the bar still forming included (slowDeclineReadsRecently).
// A long parent ladder is WATCHED for it from SlowDeclineArmDepth filled
// entries. A watched ladder goes pending only once it bought a depth within
// the fill window sophos serves — its newest fill stamped at or after
// SlowDeclineFillFrom, a fill in the bar still forming included (recentFill)
// — while SlowDeclineNeedsRecentFill is on. The first such tick the decline
// reads for the ladder — the verdict, the leg on and quiet with the ladder
// smooth from the bar of its newest fill (slowDeclineReadsForTheLadder), or
// the decline read recently for a ladder whose first fill came before the bar
// the verdict stood on (slowDeclinePendsRecently) — writes one marker row
// (SlowDeclineMarker, carrying the newest fill's price, naming the reasons of
// the last closed bar when it reads for the ladder and those of the recent
// bar otherwise), and the trade is PENDING from that fill. A ladder opened
// after that bar does not go pending on it, so the ladder a sale at the band
// opens next on the pair does not go pending on the bar the one before it
// went pending on. A fill it takes while pending is judged on the first tick
// sophos serves a reading on, its sell band positive (judgeTheNewestFill):
// the leg still on and quiet, or the decline read recently for the ladder
// from that fill, confirms the exit — the marker row again, carrying the new
// fill's price, and the trade pending from it — so a fill that lands while
// such a recent bar is served never cancels it; anything else cancels it
// with one row (SlowDeclineCancelMarker, naming what broke, carrying the new
// fill's price). A cancelled trade is watched and not pending, and goes
// pending again only as above; a pending one stays pending, however old its
// newest fill grows, until its sale, a cancel row or a reset row. The
// judgement comes before the band is read, so the band, served only with a
// reading, never sells on a fill not judged yet. The fill that takes the
// ladder to its last depth is never judged: the trade stays pending from
// where it was. The rows hold prices, so a later fill at the very price the
// trade is pending from reads as judged.
//
// The sell band is a pending trade's one sale of this rule: it sells on the
// first tick at or over the band, while the ladder's adds go on as the ladder
// proposes them and its take profit keeps working. Under break even the take
// profit reads the average entry price alone. From break even up it is
// measured from the newest fill as well as from the average entry price on a
// spot ladder: the engines hand strategies.GetPosition the larger of the two
// moves there (TakeProfitPercentage), so the ladder itself proposes takeProfit
// at that threshold, Apply passes that close through before it reads a band,
// and the trailing take profit runs as on every ladder. A futures long keeps
// the take profit from its average entry price there. hasProfit still refuses
// a close under the minimum profit, so on a tick between break even and the
// price that clears it the take profit is proposed and refused, and no band
// is read on that tick. The same verdict holds a long parent's first fill
// (EntryHoldReason, asked by ShouldHold), so no new ladder opens into the
// decline.
//
// CapitalProtectionExit switches the capital protection exit. A long spot
// ladder outside an impasse strategy is WATCHED for it once it has filled its
// last configured depth (capitalProtectionWatched, lastDepthFilled). Sophos
// serves, on the same window, the upper Bollinger band of its last closed bar
// and whether its SMC trend dashboard reads Trend Direction DOWN on every
// timeframe with at least sophos' CapitalProtectionTrendShare of each
// timeframe's reads bearish; the first tick at or over that band while it
// does sells the ladder (capitalProtectionReached). The rule writes no marker
// row and keeps no state: every tick reads the band and the trend afresh, and
// it holds nothing.
//
// IndecisionDirection switches the indecision direction. A long spot ladder
// is WATCHED for it from IndecisionArmDepth filled entries
// (indecisionWatched). Sophos serves its indecision (SlowDeclineIndecision)
// on a window whose leg is on and still down its SlowDeclineMinLegFallPct,
// whose vote fails even with a ladder's own smoothness counted while at least
// its SlowDeclineIndecisionVoteShare of the enabled readings hold — one short
// of the need at the shipped shares — with the SMC trend dashboard bearish on
// every timeframe sophos' SmcTrendTimeframes names while that condition is
// on. Sophos also serves it on any window it read, whatever the leg and the
// vote, when its SMC trend dashboard reads the whole table bearish — the
// Trend Direction row down and at least its IndecisionSmcTrendShare of every
// timeframe's reads bearish, while its IndecisionSmcTrend is on — so a
// watched ladder that is not pending, or that sophos reads as a leg on and
// quiet, is latched too; on a leg on and quiet the row names only that
// reading. The first tick a watched ladder is served it on writes one row
// (IndecisionMarker, carrying the newest fill's price, naming
// SlowDeclineBreakReasons), and the ladder is LATCHED until it closes: no row
// takes the latch away. From break even up a latched ladder's take profit is
// measured from its position price — the `buy` row's own `percentage` — as
// well as from its average entry price: the engines hand
// strategies.GetPosition the larger of the moves (TakeProfitPercentage), so
// the take profit arms once the profit is zero or more instead of waiting for
// the average's take profit, and hasProfit still gates that arming. The
// trailing take profit's sale runs the engines' own `sell` chain, hasProfit
// included, as on every ladder: a close under the minimum profit is refused,
// and the trade stays in takeProfit and retries on the next print. Under
// break even nothing changes, and the rule sells nothing of its own.
//
// DepthPriorityHoldPausesSmartTakeLoss switches the pause a depth priority
// hold puts on the two exits, the quiet slow decline and capital protection.
// A ladder is HELD while it carries a row of the cooldown depth priority gate
// (cooldown.DepthPriorityHoldMarker) stamped after its newest fill
// (depthPriorityHeld), and its next fill ends the hold. While held, neither
// exit reads it: it goes pending on no reading, a new fill is judged on
// none, and it sells at neither band. A pending ladder is reset on its first
// held tick with one row (SlowDeclineResetMarker, carrying the newest fill's
// price), which rebuildState folds like a cancel row, so the ladder goes
// pending again only as above once the hold has ended. The indecision
// direction is not paused: a held ladder is latched on a reading as on any
// other tick, and a latched ladder's take profit reads its position price
// while the hold lasts; only the pending reading, from the newest fill,
// waits for the next fill.
//
// The two exits' sales — at the sell band and at the upper band — are the
// engines' existing sellLoss chain (cancelPendingOrder, acceptLoss, sell,
// updateTrade): a limit at the tick price, re-placed one tolerance lower by
// the sellLoss logic row on a further dip. A pending ladder's sell band is
// read first, so it names a sale both rules reach on one tick. Neither ever
// replaces a close the ladder proposes or a close the trade rests in
// (protectedPosition), so the take profit — the trailing take profit
// included — is never replaced, and no rule changes a chain: the trailing
// take profit's sale runs the engines' own `sell` chain on every ladder. A
// rule switched off watches no ladder: it writes no row, sells nothing, holds
// nothing, and the rows it wrote earlier are ignored.
//
// State lives in the trade's own rows only — trade.Logs for the slow-decline
// marker, cancel and reset rows and for the indecision row, folded in order,
// and for the depth priority hold rows, read by their stamps, trade.History
// for the fills — and is rebuilt on every tick (rebuildState);
// there is no Redis key, no column and nothing on trade.PositionPrice. A row
// an older release wrote for a rule that no longer exists carries no marker
// rebuildState reads, and is ignored.
//
// Apply is the single entry point for the exits; hermes, sisyphus backtesting
// and sisyphus live-testing call it identically after the ladder has chosen a
// position and write the rows it hands back with their own clock (LogRow,
// ExitMessage). Armed tells hermes and live-testing which trades keep their
// tick when the ladder proposes nothing, and ExitReached tells sisyphus
// backtesting which prints of a blocked trade can sell: any print under a
// funds block, and under a profit block the prints at or under the block's
// price, past which the ladder proposes the close hasProfit refused. In
// ShouldHold the flag holds nothing on an open position: its one hold there
// is the slow decline's first fill.
//
// Vocabulary: depth, fill, filled entries, last depth, tolerance, take
// profit, stop loss, held, verdict, band, watched, pending, judged,
// cancelled, reset, latched.
package smarttakeloss
