package quietdecline

var (
	// Enabled switches the quiet decline: its watch (quietDeclineWatched), its trigger feeding the
	// shared action, its judgement of a new fill, the take profit of a ladder it made pending
	// (TakeProfitPercentage) and the first-fill hold (EntryHold).
	Enabled = false
	// ArmDepth is the fewest filled entries a long ladder holds before the quiet decline watches
	// it (quietDeclineWatched); the first-fill hold (EntryHold) does not wait for it.
	ArmDepth = 2
	// NeedsRecentFill switches the recent-fill rule (RecentFill): a watched ladder goes pending
	// only with its newest fill stamped at or after QuietDeclineFillFrom. It binds going pending alone.
	NeedsRecentFill = true
)
