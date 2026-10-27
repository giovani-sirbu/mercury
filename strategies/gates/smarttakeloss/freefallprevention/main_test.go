package freefallprevention

import (
	"os"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/helpers"
)

// TestMain runs every test with the gate asking the 1h row alone (MinFreeFallTimeframes of one over
// FreeFallTimeframes naming the 1h), a window of ten days of 1h bars and the share of free bars it
// was written for, whatever the constants say: they are hand-tuned, and the fixtures here read the
// last closed 1h bar; the tests of other rows move both knobs for themselves (withFreeFall).
// Window is set again after the pin, as its definition reads it.
func TestMain(m *testing.M) {
	ArmDepth, MinShare, MinFreeFallTimeframes, FreeFallTimeframes = 2, 0.8, 1, []string{"1h"}
	Timeframe, WindowTimeframe = "1h", 10*24*time.Hour
	Window = int(WindowTimeframe / (time.Duration(helpers.IntervalToMinutes(Timeframe)) * time.Minute))
	os.Exit(m.Run())
}
