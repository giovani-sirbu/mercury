package smarttakeloss

import "github.com/giovani-sirbu/mercury/trades/gates/slowpattern"

const (
	// QuietSlowDeclineExit switches the quiet slow-decline exit as a whole:
	// its watch (slowDeclineWatched), its marker and cancel rows, its sale at
	// the sell band, the take profit it reads from the newest fill
	// (TakeProfitPercentage) and its first-fill hold (EntryHoldReason).
	// Switched off it watches no ladder: no row is written, nothing is sold
	// or held, the take profit reads the average entry price alone, and the
	// rows it wrote earlier are ignored.
	QuietSlowDeclineExit = true
	// CapitalProtectionExit switches the capital protection exit as a whole:
	// its watch of a ladder at its last depth (capitalProtectionWatched) and
	// its sale at the upper band (capitalProtectionReached). Switched off it
	// watches no ladder and sells nothing; it writes no row either way.
	CapitalProtectionExit = false
	// SlowDeclineArmDepth is the least number of filled entries
	// (ladder.CountFilledEntries) a long ladder holds before the quiet
	// slow-decline exit watches it (slowDeclineWatched): Armed reports it, a
	// marker row goes out for it, and a marker row already on a shallower
	// ladder makes nothing pending. A shallow ladder has committed little of
	// the wallet, and selling it at the sell band opens the pair to a new
	// ladder on the next print: while the leg stays on and quiet each new
	// ladder would sell out again as soon as it went pending. The first-fill
	// hold (EntryHoldReason) reads the verdict alone and does not wait for it.
	SlowDeclineArmDepth = 1
	// IndecisionDirection switches the indecision direction as a whole: its
	// watch (indecisionWatched), its row and the take profit a latched ladder
	// reads from its position price (TakeProfitPercentage). Switched off it
	// watches no ladder: no row is written, the take profit reads the average
	// entry price alone, and the rows it wrote earlier are ignored.
	IndecisionDirection = true
	// IndecisionArmDepth is the least number of filled entries
	// (ladder.CountFilledEntries) a long spot ladder holds before the
	// indecision direction watches it (indecisionWatched): Armed reports it
	// and its row goes out for it, and an indecision row already on a
	// shallower ladder latches nothing. It is the rule's own bound, set apart
	// from SlowDeclineArmDepth.
	IndecisionArmDepth = 1
	// SlowDeclineNeedsRecentFill switches the recent-fill rule of the quiet
	// slow-decline exit (recentFill): a watched ladder goes pending — on the
	// last closed bar's reading or on the look-back — only when its newest
	// fill is stamped at or after SlowDeclineFillFrom, the oldest of the last
	// closed bars sophos serves it for. Switched off, going pending reads no
	// fill window. It never touches a ladder already pending, the judgement of
	// its new fill, the first-fill hold or the indecision latch.
	SlowDeclineNeedsRecentFill = true
	// DepthPriorityHoldPausesSmartTakeLoss switches the pause a depth priority
	// hold puts on the quiet slow-decline exit and capital protection
	// (depthPriorityHeld): from a cooldown depth priority hold row stamped
	// after a ladder's newest fill until its next fill, neither exit reads,
	// marks, judges or sells that ladder, and a pending exit is reset with one
	// row. The indecision direction is not paused: the latch lands during the
	// hold, and a latched ladder's take profit reads its position price.
	// Switched off, the hold rows are ignored.
	DepthPriorityHoldPausesSmartTakeLoss = true
	// SlowPatternDeclineExit switches the slow pattern decline as a whole: its
	// watch (slowPatternWatched), its pending, confirming and cancelled rows,
	// its sale at the sell band, the indecision latch it writes and the take
	// profit it reads from the newest fill (TakeProfitPercentage). Switched off
	// it watches no ladder: no row is written, nothing is sold, the take profit
	// reads the average entry price alone, and the rows it wrote earlier are
	// ignored. The shape it reads, its thresholds and its read window are the
	// constants of gates/slowpattern; the two headline knobs of the rule are
	// there too, slowpattern.SlowPatternMinDepthsBetween and
	// slowpattern.SlowPatternMinWeight.
	SlowPatternDeclineExit = true
	// SlowPatternArmDepth is the least number of filled entries
	// (ladder.CountFilledEntries) a long spot ladder holds before the slow
	// pattern decline watches it (slowPatternWatched): Armed reports it and its
	// rows go out for it, and a pattern row already on a shallower ladder makes
	// nothing pending. It is one more than
	// slowpattern.SlowPatternMinDepthsBetween — the fewest fills a window
	// between two of them is ever read on — so it follows that knob and never
	// arms a ladder that Trigger cannot yet read.
	SlowPatternArmDepth = 1 + slowpattern.SlowPatternMinDepthsBetween
)

// quietSlowDeclineExit, capitalProtectionExit, indecisionDirection,
// slowDeclineNeedsRecentFill, depthPriorityHoldPauses and
// slowPatternDeclineExit are the switches as the package reads them. They are variables only so the rules' own tests can
// run them switched off; nothing else assigns them.
var (
	quietSlowDeclineExit       = QuietSlowDeclineExit
	capitalProtectionExit      = CapitalProtectionExit
	indecisionDirection        = IndecisionDirection
	slowDeclineNeedsRecentFill = SlowDeclineNeedsRecentFill
	depthPriorityHoldPauses    = DepthPriorityHoldPausesSmartTakeLoss
	slowPatternDeclineExit     = SlowPatternDeclineExit
)
