package swingdecline

const (
	// SwingDeclineMinDepthsBetween is the least number of depths between the
	// two fills a window is cut between: at a new fill b, a window runs from an
	// earlier fill a with b − a at least this many (Trigger). It is the first
	// of the two headline knobs of the rule — it sets how long a decline must
	// last, in fills, before its shape is read — and it sets how deep a ladder
	// must be before the rule reads it at all: no ladder with fewer fills than
	// one more than this is ever read. The smart take loss arms the rule at
	// that depth (smarttakeloss.SwingDeclineArmDepth).
	SwingDeclineMinDepthsBetween = 2
	// SwingDeclineMinWeight is the least weight a window's decline must have to
	// hold (Reading.Weight): the down legs' summed fall less the up legs'
	// summed rise, over the two together. It is the second headline knob: at
	// the shipped calibration a smooth, stair-stepping decline reads about
	// half, a decline with as much bounce as fall reads none, and a window
	// whose bounces are as large as its falls does not hold.
	SwingDeclineMinWeight = 0.33
	// SwingDeclineMinDownLegs is the least number of down legs a window holds,
	// the provisional last one included (Reading.DownLegs): a decline of fewer
	// is one move, not a pattern.
	SwingDeclineMinDownLegs = 2
	// SwingDeclineMinLowerHighs is the least number of successive, strictly
	// lower starting highs among a window's down legs (Reading.LowerHighs): the
	// decline has to step down, each bounce failing under the high before it.
	SwingDeclineMinLowerHighs = 1
	// SwingDeclineMaxLegShare is the largest share of a window's whole fall
	// that one down leg may take (Reading.MaxLegShare): over it the decline is
	// one flush with a few small steps, not a staircase.
	SwingDeclineMaxLegShare = 0.6
	// SwingDeclineMinFallPct is the net move, in percent, a window's end bar
	// close must have made against its start bar close (Reading.NetPct) or
	// fallen further: negative, since a decline is a fall.
	SwingDeclineMinFallPct = -4.0
	// SwingDeclineMinSmcTrendDown is the least number of the SMC trend
	// dashboard's rows whose trend reads down — the count its Trend Direction row
	// reads, among the rows sophos read (Trend) — for a decline to hold on top of
	// its shape (withTrend). It is read as of the last bar of the series sophos
	// serves, the tick's, not the bar of the fill. At zero or below the condition
	// is off: no trend is read, the decline holds on its shape alone and no reason
	// names the trend.
	SwingDeclineMinSmcTrendDown = 2
	// SwingDeclinePivotBars is how many bars each side of a bar the bar's close
	// must beat — or tie — for the bar to be a swing high or a swing low
	// (swingPivots). A pivot is known only once that many bars have closed
	// after it, so a window's last pivot sits that many bars before its end.
	SwingDeclinePivotBars = 2
	// SwingDeclineReadBars is how many bars after the bar holding a ladder's
	// newest fill the rule still goes pending on that fill: a read is bound to
	// the fill's own bar closing, and a dropped row gets a few more ticks, but
	// a ladder already deep at deploy, or whose read bars got no tick, never
	// triggers late (the slow decline gate reads it on its going-pending path only
	// inside them). Inside them the SMC
	// trend is the tick's own, so one that turns down after the fill's bar still
	// triggers. Judging a pending ladder's new fill has no such bound.
	SwingDeclineReadBars = 2
	// SwingDeclineBarMs is the length of one bar, in ms: the 1h interval sophos
	// serves the series in (sophos' swingdecline.SwingDeclineInterval, which must
	// equal it). BarOpen floors a stamp to it.
	SwingDeclineBarMs = int64(3_600_000)
)
