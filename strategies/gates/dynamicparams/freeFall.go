package dynamicparams

import (
	"strings"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// FreeFall is whether at least MinFreeFallTimeframes rows read in free fall, counting only the rows
// FreeFallTimeframes names (every row when it names none); false while MinFreeFallTimeframes is at or below zero.
func FreeFall(reads aggragates.DynamicParamsIndicators) bool {
	if MinFreeFallTimeframes <= 0 {
		return false
	}

	rows := 0
	for _, timeframe := range strings.Split(reads.FreeFall, ",") {
		if timeframe != "" && countsFreeFall(timeframe) {
			rows++
		}
	}

	return rows >= MinFreeFallTimeframes
}

// countsFreeFall is whether a row in free fall on this timeframe counts: FreeFallTimeframes names it, or names none.
func countsFreeFall(timeframe string) bool {
	if len(FreeFallTimeframes) == 0 {
		return true
	}

	for _, listed := range FreeFallTimeframes {
		if strings.EqualFold(listed, timeframe) {
			return true
		}
	}

	return false
}
