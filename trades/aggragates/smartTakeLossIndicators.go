package aggragates

// SmartTakeLossIndicators is the chart block sophos serves on
// GET /:symbol/patterns for the SmartTakeLoss flag: the quiet slow decline's
// reading — its indecision included — and the capital protection reading,
// both taken on the closed window of sophos' smart take loss. Every zero
// field is inert in gates/smarttakeloss.Apply.
type SmartTakeLossIndicators struct {
	// SlowDeclineExit is sophos' quiet slow-decline verdict, read on the
	// closed window of its slow-decline walk: an early down leg, still down
	// sophos' SlowDeclineMinLegFallPct from its high close, on which a vote
	// passes among the readings sophos switches into it — the recent volume,
	// the volatility and the Bollinger band's width under their own
	// baselines, and the fall smooth over sophos' own span — for any ladder,
	// with sophos' SMC trend dashboard, read at the same bar, bearish on every
	// timeframe sophos' SmcTrendTimeframes names while that condition is on.
	// gates/smarttakeloss marks a long ladder it watches (from
	// SlowDeclineArmDepth filled entries) pending on it once that ladder
	// bought a depth within the fill window (SlowDeclineFillFrom) — from
	// break even up a pending ladder's take profit reads its newest fill as
	// well (TakeProfitPercentage) — and holds a long first fill while it
	// stands.
	// SlowDeclineLegQuiet is the same reading for a ladder whose own
	// smoothness holds: the leg is on and still down that far, the vote
	// passes with that smoothness counted and the SMC trend condition is met.
	// SlowDeclineSmoothFrom is the open time in ms of the earliest bar sophos
	// reads the leg smooth from, and SlowDeclineFillBefore the open time
	// in ms of the bar a fill has to be stamped before to have sophos'
	// SlowDeclineMinBarsAfterFill closed bars after the bar that holds it — at
	// none, the bar after the last closed one, so a fill counts once its own bar
	// has closed; both are served only while SlowDeclineLegQuiet and zero
	// otherwise. The gate reads a ladder's own smoothness by comparing its
	// newest fill's stamp with the two, so a ladder whose newest fill came after
	// the fast drop goes pending without the verdict. SlowDeclineSellBand is the
	// price a pending ladder sells at — the Bollinger band sophos serves on that
	// window, its mean plus the deviations sophos picks — served whenever sophos
	// can compute it, verdict or not. A zero band sells nothing.
	// SlowDeclineExitReasons names the leg, every reading of the vote with
	// its value, the count against the need, the span the smoothness was read
	// over, the bar it reads smooth from, the sell band and the SMC trend
	// while the leg is on and quiet, and only ever reaches the marker row.
	// SlowDeclineBreakReasons is served only while the leg is not on and
	// quiet, on a window sophos read: it names what broke — the leg that is
	// not on, the leg on but short of SlowDeclineMinLegFallPct, each reading
	// of the vote that fails and the count, or the SMC trend condition — and
	// only ever reaches two rows: the cancel row, and the indecision row that
	// latches a ladder (SlowDeclineIndecision). A new fill on a pending ladder
	// is judged on the first tick that serves the band: the leg on and quiet,
	// or the decline read recently for the ladder, confirms the exit, anything
	// else cancels it.
	SlowDeclineExit         bool
	SlowDeclineLegQuiet     bool
	SlowDeclineSmoothFrom   int64
	SlowDeclineFillBefore   int64
	SlowDeclineSellBand     float64
	SlowDeclineExitReasons  []string
	SlowDeclineBreakReasons []string
	// SlowDeclineIndecision is sophos' indecision reading, served only while the
	// leg is not on and quiet: the leg on and still down sophos'
	// SlowDeclineMinLegFallPct from its high close, the vote failing even with a
	// ladder's own smoothness counted, and at least sophos'
	// SlowDeclineIndecisionVoteShare of the enabled readings holding with it —
	// one short of the need at the shipped shares, the count the break reasons
	// name — with the SMC trend dashboard bearish on every timeframe sophos'
	// SmcTrendTimeframes names while that condition is on. gates/smarttakeloss
	// latches a long spot parent ladder the indecision direction watches (from
	// IndecisionArmDepth filled entries) on the first tick it is served: one row
	// naming SlowDeclineBreakReasons, and from then until the trade closes its
	// take profit is measured from the position price as well
	// (TakeProfitPercentage) and its trailing take profit's sale may close under
	// the minimum profit (SaleActions).
	SlowDeclineIndecision bool
	// SlowDeclineRecentAt is the open time in ms of the newest of sophos'
	// last SlowDeclineRecentBars closed bars on which the verdict stood: the
	// verdict of the window cut at that bar, with the SMC trend condition
	// read as of that bar's close while sophos' SlowDeclineSmcTrend is on.
	// Zero when none did, when sophos' look-back is off, or on a window
	// sophos did not read. SlowDeclineRecentFrom is the open time in ms of the
	// oldest of those bars, and SlowDeclineRecentReasons names the bar the
	// verdict stood on, then that bar's own reasons; both are served only
	// with SlowDeclineRecentAt. gates/smarttakeloss reads the decline
	// recently for a ladder whose newest fill is stamped at or after
	// SlowDeclineRecentFrom — the ladder bought a depth within those bars, a
	// fill in the bar still forming included: a new fill on a pending ladder
	// is confirmed on it, and a watched ladder goes pending on it when its
	// first fill is stamped strictly before SlowDeclineRecentAt — the ladder
	// already stood when the verdict stood — and its newest fill is within
	// the fill window (SlowDeclineFillFrom). The row names
	// SlowDeclineExitReasons when the last closed bar reads for the ladder on
	// its own, and SlowDeclineRecentReasons otherwise. It holds no first fill.
	SlowDeclineRecentAt      int64
	SlowDeclineRecentFrom    int64
	SlowDeclineRecentReasons []string
	// SlowDeclineFillFrom is the open time in ms of the oldest of sophos'
	// last SlowDeclineFillBars closed bars — the window's first bar when it
	// holds fewer — served whenever sophos read the window, whatever its
	// verdicts, and zero otherwise. While gates/smarttakeloss'
	// SlowDeclineNeedsRecentFill is on, a watched ladder goes pending, on the
	// last closed bar's reading or on the look-back, only when its newest
	// fill is stamped at or after it, a fill in the bar still forming
	// included; zero makes no ladder pending. A pending ladder's new fill,
	// the first-fill hold and the indecision latch do not read it.
	SlowDeclineFillFrom int64
	// CapitalProtectionUpperBB is the upper Bollinger band of the last closed
	// bar of the smart take loss window's interval, over sophos'
	// CapitalProtectionBandPeriod and CapitalProtectionBandStdDev; zero when
	// sophos cannot compute it, and a zero band sells nothing.
	// CapitalProtectionSmcBearish is true when sophos' SMC trend dashboard,
	// read at the same bar, reads Trend Direction DOWN on every timeframe AND
	// every timeframe has at least sophos' CapitalProtectionTrendShare of its
	// reads bearish. gates/smarttakeloss sells a long spot ladder at its last
	// depth on the first tick at or over the band while it is true.
	CapitalProtectionUpperBB    float64
	CapitalProtectionSmcBearish bool
}
