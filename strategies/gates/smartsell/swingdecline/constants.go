package swingdecline

var (
	// Enabled switches the swing decline's trigger feeding the shared action: its watch
	// (swingDeclineWatched), its reading making a watched ladder pending and its judgement of a new
	// fill (goesPending, newFillReadings); the latch on its reading (earlytakeprofit.SwingDecline) reads whatever this says.
	Enabled = false
	// ResetOnNewDepth switches the judgement of a pending ladder's new fill on the swing decline
	// (newFillReadings); off, its pending stands through new fills, each counting as judged.
	ResetOnNewDepth = true
	// ArmDepth is the fewest filled entries a long spot ladder holds before the swing decline
	// watches it (swingDeclineWatched): one more than MinDepthsBetween, the fewest a window needs.
	ArmDepth = 1 + MinDepthsBetween
	// MinDepthsBetween is the fewest depths between the two fills a window is cut between (Trigger);
	// it sets how long a decline must last in fills before it is read.
	MinDepthsBetween = 3
	// LookbackBars is how many bars before the first fill's bar its window starts (leadStart), so
	// the swing high the ladder fell from is in it; zero or below starts at the fill's bar.
	LookbackBars = 24
	// PivotBars is how many bars each side a bar's high must beat or tie to be a swing high, and
	// its low to be a swing low (swingPivots).
	PivotBars = 4
	// MinBarsBetween is the fewest bars from a down leg's swing high to its low for the leg to count (Read).
	MinBarsBetween = 6
	// MinDropPct is the least fall, in percent from a down leg's high wick to its low wick, for the
	// leg to count (Read).
	MinDropPct float64 = 5.0
	// MaxBouncePct is the most an up leg may rise, in percent from its low wick to its high wick,
	// and still count; over it the bounce is named and not counted (Read).
	MaxBouncePct float64 = 3.0
	// MinLegsDown is the fewest down legs that count a window needs, the provisional last one
	// included (Reading.DownLegs).
	MinLegsDown = 2
	// MinLegsUp is the fewest up legs that count a window needs (Reading.UpLegs).
	MinLegsUp = 2
	// MinSmcTrendDown is the fewest Trend Direction rows reading down for a decline to hold
	// (withTrend), read as of the series' last bar; zero or below switches the condition off.
	MinSmcTrendDown = 3
)
