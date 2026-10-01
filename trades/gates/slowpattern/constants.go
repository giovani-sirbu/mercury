package slowpattern

const (
	// SlowPatternMinDepthsBetween is the least number of depths between the
	// two fills a window is cut between: at a new fill b, a window runs from an
	// earlier fill a with b − a at least this many (Trigger). It is the first
	// of the two headline knobs of the rule — it sets how long a decline must
	// last, in fills, before its shape is read — and it sets how deep a ladder
	// must be before the rule reads it at all: no ladder with fewer fills than
	// one more than this is ever read. The smart take loss arms the rule at
	// that depth (smarttakeloss.SlowPatternArmDepth).
	SlowPatternMinDepthsBetween = 3
	// SlowPatternMinWeight is the least weight a window's decline must have to
	// hold (Reading.Weight): the down legs' summed fall less the up legs'
	// summed rise, over the two together. It is the second headline knob: at
	// the shipped calibration a smooth, stair-stepping decline reads about
	// half, a decline with as much bounce as fall reads none, and a window
	// whose bounces are as large as its falls does not hold.
	SlowPatternMinWeight = 0.33
	// SlowPatternMinDownLegs is the least number of down legs a window holds,
	// the provisional last one included (Reading.DownLegs): a decline of fewer
	// is one move, not a pattern.
	SlowPatternMinDownLegs = 3
	// SlowPatternMinLowerHighs is the least number of successive, strictly
	// lower starting highs among a window's down legs (Reading.LowerHighs): the
	// decline has to step down, each bounce failing under the high before it.
	SlowPatternMinLowerHighs = 2
	// SlowPatternMaxLegShare is the largest share of a window's whole fall
	// that one down leg may take (Reading.MaxLegShare): over it the decline is
	// one flush with a few small steps, not a staircase.
	SlowPatternMaxLegShare = 0.6
	// SlowPatternMinFallPct is the net move, in percent, a window's end bar
	// close must have made against its start bar close (Reading.NetPct) or
	// fallen further: negative, since a decline is a fall.
	SlowPatternMinFallPct = -4.0
	// SlowPatternPivotBars is how many bars each side of a bar the bar's close
	// must beat — or tie — for the bar to be a swing high or a swing low
	// (swingPivots). A pivot is known only once that many bars have closed
	// after it, so a window's last pivot sits that many bars before its end.
	SlowPatternPivotBars = 3
	// SlowPatternReadBars is how many bars after the bar holding a ladder's
	// newest fill the rule still goes pending on that fill: a read is bound to
	// the fill's own bar closing, and a dropped row gets a few more ticks, but
	// a ladder already deep at deploy, or whose read bars got no tick, never
	// triggers late (smarttakeloss.slowPatternGoesPending). Judging a pending
	// ladder's new fill has no such bound.
	SlowPatternReadBars = 2
	// SlowPatternBarMs is the length of one bar, in ms: the 1h interval sophos
	// serves the series in (sophos' slowpattern.SlowPatternInterval, which must
	// equal it). BarOpen floors a stamp to it.
	SlowPatternBarMs = int64(3_600_000)
)
