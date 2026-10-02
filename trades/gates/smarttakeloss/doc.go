// Package smarttakeloss is the SmartTakeLoss flag: the slow decline gate and
// capital protection, read off the closed window of sophos' smart take loss,
// each behind its own switches. The gate turns a long ladder into a seller
// instead of a buyer through two features, the quiet decline and the swing
// decline, and carries a third, the indecision direction, which moves where a
// ladder takes profit. Capital protection sells at the upper band.
//
// THE GATE. The three features are one gate (GateSlowDecline): one state, one
// pending, and on a tick at most one row of its own, one event beside it. A long
// ladder is WATCHED by the quiet decline (QuietDecline) from
// QuietDeclineArmDepth filled entries, by the swing decline (SwingDecline) from
// SwingDeclineArmDepth and by the indecision direction (Indecision) from
// IndecisionArmDepth, these two only on a spot ladder. The gate watches a
// ladder either decline watches (slowDeclineWatched). A feature switched off
// watches no ladder: it triggers, judges and holds nothing, and the events that
// name it are ignored.
//
// QUIET DECLINE. Sophos reads an early down leg on its window, still down
// SlowDeclineMinLegFallPct from its high close, and takes a vote at its last bar
// among the readings it switches in, with its SMC trend dashboard bearish on
// every timeframe it names while that condition is on. The verdict is that vote
// passing with the smoothness over sophos' own span, so it reads for every
// ladder; the leg on and quiet is the same vote with a ladder's own smoothness,
// counted from the ladder's newest fill (quietDeclineReadsForTheLadder); and
// sophos looks back over its last SlowDeclineRecentBars closed bars for the
// verdict (quietDeclineReadsRecently). It serves the sell band of that window,
// a Bollinger band, beside them. A watched ladder triggers on the first tick the
// quiet decline reads for it, once it bought a depth within the fill window
// sophos serves (recentFill, QuietDeclineNeedsRecentFill) and, on the
// look-back, already stood before the bar the verdict stood on
// (quietDeclinePendsRecently). The same verdict holds a long parent's first fill
// (EntryHold, asked by ShouldHold, which writes its held event), so no new
// ladder opens into the decline.
//
// SWING DECLINE. The SHAPE of the decline between a ladder's fills, not a
// market vote or a prediction. Sophos serves the closed 1h bars
// (swingdecline.Series) and the SMC trend dashboard's Trend Direction row
// (swingdecline.Trend), and swingdecline.Trigger reads the windows between the
// fills. It triggers at a NEW FILL, once the bar that holds it has closed and no
// more than swingdecline.SwingDeclineReadBars bars after, when a window from an
// earlier fill, at least swingdecline.SwingDeclineMinDepthsBetween depths back,
// holds and the trend reads down on enough rows (swingDeclineReads).
//
// INDECISION DIRECTION. A latched ladder keeps its latch until it closes, and
// from break even up its take profit is measured from its position price — the
// `buy` row's own `percentage` — as well as from its average entry price: the
// engines hand strategies.GetPosition the larger of the moves
// (TakeProfitPercentage), so the take profit arms once the profit is zero or
// more, and hasProfit still gates that arming. The feature sells nothing of its
// own. Sophos serves its indecision (SlowDeclineIndecision) on a window whose
// leg is on and still down SlowDeclineMinLegFallPct, whose vote fails even with a
// ladder's own smoothness counted while at least SlowDeclineIndecisionVoteShare
// of the enabled readings hold, or that misses the leg on and quiet by a hair
// (SlowDeclineNearMissShare), or whose SMC trend condition fails while every
// timeframe is still over SmcTrendIndecisionShare bearish, and on any window,
// whatever the leg and the vote, when the whole SMC table reads bearish
// (IndecisionSmcTrendShare). That latches a watched ladder on any tick, held
// ones included, naming SlowDeclineBreakReasons. The swing decline latches a
// ladder too, when it triggers, confirms a new fill, or reads for the judged
// newest fill of a ladder already pending (latchIndecision).
//
// ONE PENDING. Either decline makes a ladder pending from its newest fill's
// price, and it is the same pending. Not pending and not held, both declines are
// read on every tick and the ladder goes pending on whichever triggers, the row
// naming each (goesPending). A fill a pending ladder takes is judged on the
// readings that apply to it (judgeTheNewestFill): the quiet decline once a band
// is served, holding on the leg still on and quiet or the decline read
// recently; the swing decline once the series has closed the fill's bar,
// holding when its window and trend read again. Any reading that holds CONFIRMS
// the fill, the ladder pending from it on the features that held; the ladder is
// CANCELLED once every reading that applies has answered and none holds, watched
// and not pending until a decline triggers again; otherwise the fill WAITS, no
// row and no sale. The fill that takes the ladder to its last depth is never
// judged, and a fill at the very price the ladder is pending from reads as
// judged, the events holding prices.
//
// THE SALE. A pending ladder whose newest fill is judged sells on the first tick
// at or over the sell band (sellsAtTheBand). Under break even the take profit
// reads the average entry price alone and the band is the one sale; from break
// even up the ladder's own take profit, measured from the newest fill as well
// (TakeProfitPercentage), is a close Apply passes through before it reads a
// band, and hasProfit may refuse it. The sales are the engines' existing sellLoss
// chain (cancelPendingOrder, acceptLoss, sell, updateTrade): a limit at the tick
// price, re-placed one tolerance lower by the sellLoss logic row on a further
// dip. Neither sale replaces a close the ladder proposes or one the trade rests
// in (protectedPosition).
//
// LOSS CAP. The sale at the sell band is held while its loss — the close
// acceptLoss would book (profit.SimulatedCloseProfit) at the tick price, less the
// third-asset fees Apply cannot price, over ladder.EntryNotional — is over
// SlowDeclineMaxLossPct (bandSaleLoss): that tick sells nothing at the band, the
// ladder's proposal passes through, and one row (SaleHeldRow, EventSaleHeld) says
// so once per pending episode, after the tick's own: a confirmation at a new fill
// makes the row due again, the last depth's fill, never judged, does not. Capital
// protection is not capped, and a cap at zero or below is off.
//
// DEPTH PRIORITY. A ladder is HELD while it carries an event of the cooldown
// depth priority gate (cooldown.GateDepthPriority) stamped after its newest fill
// (depthPriorityHeld), and its next fill ends the hold. While held neither the
// gate nor capital protection reads it: nothing triggers, nothing is judged,
// nothing sells, and the indecision latch alone goes on. A pending gate is reset
// on the first held tick, whichever feature made it pending (slowDeclineReset),
// and goes pending again only as above once the hold has ended.
//
// CAPITAL PROTECTION. A long spot ladder outside an impasse strategy is WATCHED
// for it once it has filled its last configured depth (capitalProtectionWatched,
// lastDepthFilled). Sophos serves, on the same window, the upper Bollinger band
// of its last closed bar and whether its SMC trend dashboard reads Trend
// Direction DOWN on every timeframe with at least CapitalProtectionTrendShare of
// each timeframe's reads bearish; the first tick at or over that band while it
// does sells the ladder (capitalProtectionReached). It writes no row and keeps
// no state: every tick reads the band and the trend afresh. Its sale is the
// exit row (ExitRow) and the sold event beside it, which no fold reads.
//
// ROWS AND EVENTS. Whatever the gate does on a tick — going pending, judging a
// new fill, resetting, latching — is ONE row (buildRow) and one event, filed
// under GateSlowDecline: kind EventPending for a trigger and a confirmation,
// EventCancelled, EventReset or EventLatched. The row reads "smartTakeLoss: "
// and a head, then the reasons grouped by feature in parentheses; it carries no
// "Hold <position>:" frame, being no refusal — the first-fill hold alone is one.
// The features the row is about travel in the event (EventData.Features), and
// the fold reads them. PendingRow, ConfirmedRow, CancelledRow, ResetRow and
// LatchedRow build the rows for Apply and a fixture alike, from Findings; ExitRow
// builds the sold row, naming the decline features the ladder was pending on.
//
// STATE lives in the trade's own strategy events (trade.StrategyEvents) and its
// history: the events of the slow decline gate, folded in slice order, the
// cooldown depth priority gate's events, read by their stamps, and trade.History
// for the fills. It is rebuilt on every tick (rebuildState); nothing is kept in
// Redis, in a column or on trade.PositionPrice, and no log row is read back.
// Events written before the gate was merged — the quiet decline's feature-less
// ones, the slow pattern's, the indecision gate's — are read as they were
// (legacyStates) and never rewritten, until the first event that names features
// supersedes them. An event whose gate or kind the fold does not know is ignored.
//
// Apply is the single entry point for the exits; hermes, sisyphus backtesting
// and sisyphus live-testing call it identically after the ladder has chosen a
// position and write the rows it hands back, each as a log row and its event
// under one stamp of their own clock (Rows), then the exit row built with ExitRow
// on a sale. Armed tells hermes and live-testing which trades keep their tick
// when the ladder proposes nothing, and ExitReached tells sisyphus backtesting
// which prints of a blocked trade can sell: any print under a funds block, and
// under a profit block the prints at or under the block's price, past which the
// ladder proposes the close hasProfit refused. In ShouldHold the flag holds
// nothing on an open position: its one hold there is the quiet decline's first
// fill.
//
// Vocabulary: depth, fill, filled entries, last depth, tolerance, take
// profit, stop loss, held, verdict, band, watched, pending, judged, confirmed,
// cancelled, reset, latched, trigger.
package smarttakeloss
