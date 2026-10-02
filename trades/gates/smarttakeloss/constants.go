package smarttakeloss

import "github.com/giovani-sirbu/mercury/trades/gates/swingdecline"

const (
	// QuietDecline switches the quiet decline, the first of the slow decline
	// gate's three features: its watch (quietDeclineWatched), its trigger, its
	// reading of a new fill, the take profit a ladder it made pending reads from
	// the newest fill (TakeProfitPercentage) and its first-fill hold (EntryHold).
	// Switched off it watches no ladder: it triggers, judges and holds nothing,
	// and the events that name it are ignored.
	QuietDecline = true
	// SwingDecline switches the swing decline, the gate's second feature: its
	// watch (swingDeclineWatched), its trigger and its reading of a new fill.
	// Switched off it watches no ladder, and the events that name it — and the
	// ones of the gate it replaced (gateSlowPattern) — are ignored. The shape it
	// reads, the SMC trend direction it needs, its thresholds and its read window
	// are the constants of gates/swingdecline; its two headline knobs are
	// swingdecline.SwingDeclineMinDepthsBetween and
	// swingdecline.SwingDeclineMinWeight.
	SwingDecline = true
	// Indecision switches the indecision direction, the gate's third feature:
	// its watch (indecisionWatched), its latch and the take profit a latched
	// ladder reads from its position price (TakeProfitPercentage). Switched off
	// it watches no ladder: nothing latches, the take profit reads the average
	// entry price alone, and the events that name it — and the ones of the gate
	// it replaced (gateIndecision) — are ignored.
	Indecision = true
	// CapitalProtectionExit switches the capital protection exit as a whole:
	// its watch of a ladder at its last depth (capitalProtectionWatched) and
	// its sale at the upper band (capitalProtectionReached). Switched off it
	// watches no ladder and sells nothing; it writes no row either way.
	CapitalProtectionExit = false
	// QuietDeclineArmDepth is the least number of filled entries
	// (ladder.CountFilledEntries) a long ladder holds before the quiet decline
	// watches it (quietDeclineWatched): Armed reports it, a row goes out for it,
	// and an event already on a shallower ladder makes nothing pending. A
	// shallow ladder has committed little of the wallet, and selling it at the
	// sell band opens the pair to a new ladder on the next print: while the leg
	// stays on and quiet each new ladder would sell out again as soon as it went
	// pending. The first-fill hold (EntryHold) reads the verdict alone and does
	// not wait for it.
	QuietDeclineArmDepth = 1
	// SwingDeclineArmDepth is the least number of filled entries
	// (ladder.CountFilledEntries) a long spot ladder holds before the swing
	// decline watches it (swingDeclineWatched). It is one more than
	// swingdecline.SwingDeclineMinDepthsBetween — the fewest fills a window
	// between two of them is ever read on — so it follows that knob and never
	// arms a ladder that swingdecline.Trigger cannot yet read.
	SwingDeclineArmDepth = 1 + swingdecline.SwingDeclineMinDepthsBetween
	// IndecisionArmDepth is the least number of filled entries
	// (ladder.CountFilledEntries) a long spot ladder holds before the
	// indecision direction watches it (indecisionWatched): Armed reports it and
	// its latch goes out for it, and an event already on a shallower ladder
	// latches nothing. It is the feature's own bound, set apart from
	// QuietDeclineArmDepth and SwingDeclineArmDepth.
	IndecisionArmDepth = 1
	// QuietDeclineNeedsRecentFill switches the recent-fill rule of the quiet
	// decline (recentFill): a watched ladder goes pending — on the last closed
	// bar's reading or on the look-back — only when its newest fill is stamped
	// at or after SlowDeclineFillFrom, the oldest of the last closed bars sophos
	// serves it for. Switched off, going pending reads no fill window. It never
	// touches a ladder already pending, the judgement of its new fill, the
	// swing decline, the first-fill hold or the indecision latch.
	QuietDeclineNeedsRecentFill = true
	// DepthPriorityHoldPausesSmartTakeLoss switches the pause a depth priority
	// hold puts on the slow decline gate and capital protection
	// (depthPriorityHeld): from a cooldown depth priority hold row stamped
	// after a ladder's newest fill until its next fill, neither reads, triggers,
	// judges or sells that ladder, and a pending gate is reset with one row. The
	// indecision latch of the sophos reading is not paused: it lands during the
	// hold, and a latched ladder's take profit reads its position price.
	// Switched off, the hold rows are ignored.
	DepthPriorityHoldPausesSmartTakeLoss = true
	// SlowDeclineMaxLossPct is the largest loss, in percent of the trade's used
	// amount (ladder.EntryNotional), the slow decline gate's sale at the sell band
	// may take. Over it the sale is held (bandSaleLoss): nothing is sold on that
	// tick, the ladder's own proposal passes through — its adds go on, new depths
	// included — and the sale goes as it always did on the first tick the loss is
	// within it. At zero or below the cap is off. Capital protection's sale is not
	// capped. The loss is the close figure acceptLoss books
	// (profit.SimulatedCloseProfit) without the third-asset fees, which Apply
	// cannot price and a backtest never has.
	SlowDeclineMaxLossPct = 2
)

// quietDecline, swingDecline, indecision, capitalProtectionExit,
// quietDeclineNeedsRecentFill, depthPriorityHoldPauses and slowDeclineMaxLossPct
// are the switches and the cap as the package reads them. They are variables
// only so the features' own tests can run them switched off or moved; nothing
// else assigns them.
var (
	quietDecline                = QuietDecline
	swingDecline                = SwingDecline
	indecision                  = Indecision
	capitalProtectionExit       = CapitalProtectionExit
	quietDeclineNeedsRecentFill = QuietDeclineNeedsRecentFill
	depthPriorityHoldPauses     = DepthPriorityHoldPausesSmartTakeLoss
	slowDeclineMaxLossPct       = float64(SlowDeclineMaxLossPct)
)
