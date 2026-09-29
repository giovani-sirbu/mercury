package testutil

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// FibAI is the worked fibonacci example: a 100 → 110 swing retraces to
// 106.18 / 105 / 103.82 / 102.14. A depth arming above the next lower level
// waits for it.
func FibAI() aggragates.AIIndicators {
	return aggragates.AIIndicators{
		FibSwingLow:  100,
		FibSwingHigh: 110,
		FibLevels:    []float64{106.18, 105, 103.82, 102.14},
	}
}
