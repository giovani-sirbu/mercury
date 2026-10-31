package capitalprotection

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// served is the block sophos serves for capital protection: whether the SMC trend dashboard reads
// every timeframe bearish, and whether it reads every timeframe bullish.
func served(bearish, bullish bool) aggragates.SmartSellIndicators {
	return aggragates.SmartSellIndicators{
		CapitalProtectionSmcBearish: bearish,
		CapitalProtectionSmcBullish: bullish,
	}
}

// everythingElse is the block of a window sophos read in full that serves every other reading the
// gate takes off the block and neither trend flag of capital protection.
func everythingElse() aggragates.SmartSellIndicators {
	const stamp int64 = 1 << 40
	return aggragates.SmartSellIndicators{
		QuietDeclineVerdict:      true,
		QuietDeclineLegQuiet:     true,
		QuietDeclineSmoothFrom:   stamp,
		QuietDeclineFillBefore:   stamp,
		QuietDeclineExitReasons:  []string{"the leg is on and quiet"},
		QuietDeclineBreakReasons: []string{"the vote fails"},
		Indecision:               true,
		IndecisionRecentAt:       stamp,
		QuietDeclineRecentAt:     stamp,
		QuietDeclineRecentFrom:   stamp,
		QuietDeclineFillFrom:     stamp,
		SeriesOpens:              []int64{stamp},
		SeriesCloses:             []float64{100},
		SmcTrendDown:             3,
		SmcTrendValid:            3,
		FlatMarketBarOpens:       []int64{stamp},
		FlatMarketHighs:          []float64{100},
		FlatMarketCloses:         []float64{100},
	}
}

// The trend reading is the flag on the ladder's own side and nothing else: a long ladder triggers
// on the bearish flag alone, an inverse one on the bullish flag alone, each whatever the other
// flag says; neither flag triggers nothing on either side.
func TestTriggeredReadsTheFlagOnTheLaddersSide(t *testing.T) {
	cases := []struct {
		name             string
		bearish, bullish bool
		long, inverse    bool
	}{
		{"neither flag", false, false, false, false},
		{"the bearish flag", true, false, true, false},
		{"the bullish flag", false, true, false, true},
		{"both flags", true, true, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			block := served(tc.bearish, tc.bullish)
			if got := Triggered(false, block); got != tc.long {
				t.Errorf("a long ladder: triggered %v, want %v", got, tc.long)
			}
			if got := Triggered(true, block); got != tc.inverse {
				t.Errorf("an inverse ladder: triggered %v, want %v", got, tc.inverse)
			}
		})
	}
}

// Nothing but the two flags decides it: every other reading of the block, served in full, triggers
// nothing without them and changes no answer beside them.
func TestTriggeredAsksNothingElseOfTheBlock(t *testing.T) {
	for _, inverse := range []bool{false, true} {
		if Triggered(inverse, everythingElse()) {
			t.Errorf("inverse %v: every other reading triggers nothing on its own", inverse)
		}
		block := everythingElse()
		block.CapitalProtectionSmcBearish, block.CapitalProtectionSmcBullish = !inverse, inverse
		if !Triggered(inverse, block) {
			t.Errorf("inverse %v: the flag on the ladder's side triggers beside every other reading", inverse)
		}
	}
}
