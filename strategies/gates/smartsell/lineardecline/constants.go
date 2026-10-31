package lineardecline

var (
	// Enabled switches the linear decline's trigger feeding the shared action: its line makes a
	// watched ladder pending and judges a new fill, as the swing decline does (goesPending,
	// newFillReadings); the latch on its line (earlytakeprofit.LinearDecline) reads whatever this says.
	Enabled = false
	// ResetOnNewDepth switches the judgement of a pending ladder's new fill on the linear decline
	// (newFillReadings); off, its pending stands through new fills, each counting as judged.
	ResetOnNewDepth = false
	// ArmDepth is the fewest filled entries a long spot ladder holds before the linear decline
	// reads it (linearDeclineReads): the rule's own bound, MinFills.
	ArmDepth = MinFills
	// MinFills is how many of the newest fills the line is read on, the window, and the fewest a
	// ladder holds before it is read (Trigger).
	MinFills = 4
	// MaxDeviationPct is the farthest, in percent, any bar's close may lie from either chord at the
	// bar's close time (Reading.Deviation, Reading.PreviousDeviation).
	MaxDeviationPct float64 = 3.0
	// MaxSlopeChange is the most the slope of the chord to the newest fill may differ from the
	// chord to the fill before it, as a share of the latter (Reading.Slope, Reading.PreviousSlope).
	MaxSlopeChange float64 = 0.1
	// MinFallPct is the net move the newest fill must have made against the window's first
	// (Reading.NetPct), negative since a decline is a fall.
	MinFallPct float64 = -5.0
	// MinSmcTrendDown is the fewest Trend Direction rows reading down for a line to hold
	// (MeetsTheNeed), read as of the tick's bar; zero or below switches the condition off.
	MinSmcTrendDown = 3
	// Window is the newest fills the line is read on: MinFills, never under previousChordFills.
	Window = max(MinFills, previousChordFills)
)

// previousChordFills is the floor under MinFills: the chord to the fill before the newest needs
// three fills to exist.
const previousChordFills = 3
