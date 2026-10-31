package flatmarket

import (
	"testing"

	"github.com/giovani-sirbu/mercury/helpers"
)

// The tolerance is a share of the bars: a negative one would let fewer than no bar count, and the
// reading floors it at zero.
func TestTheToleranceIsAShare(t *testing.T) {
	if TolerancePercentage < 0 {
		t.Fatalf("TolerancePercentage %v must not be negative", TolerancePercentage)
	}
	if got := tolerance(); got != float64(TolerancePercentage) {
		t.Fatalf("tolerance() = %v, want TolerancePercentage %v", got, TolerancePercentage)
	}
}

// The window sophos serves must be readable: an exchange interval, and at least the bars a line
// needs; sophos pins its own look-back against Window.
func TestTheWindowIsReadable(t *testing.T) {
	if helpers.IntervalToMinutes(Timeframe) <= 0 {
		t.Fatalf("Timeframe %q is no exchange interval", Timeframe)
	}
	if Window < MinBars {
		t.Fatalf("Window %d bars of %s must reach MinBars %d", Window, Timeframe, MinBars)
	}
	if MaxDeviation <= 0 || ArmDepth < 1 {
		t.Fatalf("MaxDeviation %v must be positive and ArmDepth %d at least one", MaxDeviation, ArmDepth)
	}
}
