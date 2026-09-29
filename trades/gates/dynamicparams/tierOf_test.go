package dynamicparams_test

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/dynamicparams"
)

// The wire values of a read, sophos' smctypes.Direction.
const (
	bearish = -1
	neutral = 0
	bullish = 1
)

// Only a bearish read counts: two make both bearish, one makes mixed, and a
// neutral read counts exactly as a bullish one does. A block sophos did not
// read is the base tier whatever its reads say.
func TestTierOfCountsBearishReadsOnly(t *testing.T) {
	cases := []struct {
		guppy int
		bmsb  int
		want  dynamicparams.Tier
	}{
		{bearish, bearish, dynamicparams.TierBothBearish},
		{bearish, neutral, dynamicparams.TierMixed},
		{bearish, bullish, dynamicparams.TierMixed},
		{neutral, bearish, dynamicparams.TierMixed},
		{neutral, neutral, dynamicparams.TierBase},
		{neutral, bullish, dynamicparams.TierBase},
		{bullish, bearish, dynamicparams.TierMixed},
		{bullish, neutral, dynamicparams.TierBase},
		{bullish, bullish, dynamicparams.TierBase},
	}

	for _, c := range cases {
		read := aggragates.DynamicParamsIndicators{Timeframe: "1D", Guppy: c.guppy, BMSB: c.bmsb, Valid: true}
		if got := dynamicparams.TierOf(read); got != c.want {
			t.Errorf("Guppy %d, BMSB %d: TierOf = %d, want %d", c.guppy, c.bmsb, got, c.want)
		}

		unread := read
		unread.Valid = false
		if got := dynamicparams.TierOf(unread); got != dynamicparams.TierBase {
			t.Errorf("Guppy %d, BMSB %d not read: TierOf = %d, want the base tier", c.guppy, c.bmsb, got)
		}
	}
}

// The zero block is "not read", and a value off the wire's three counts as
// no bearish read.
func TestTierOfTreatsTheZeroBlockAndStrayValuesAsBase(t *testing.T) {
	if got := dynamicparams.TierOf(aggragates.DynamicParamsIndicators{}); got != dynamicparams.TierBase {
		t.Errorf("the zero block: TierOf = %d, want the base tier", got)
	}

	stray := aggragates.DynamicParamsIndicators{Timeframe: "1D", Guppy: -2, BMSB: 2, Valid: true}
	if got := dynamicparams.TierOf(stray); got != dynamicparams.TierBase {
		t.Errorf("stray reads: TierOf = %d, want the base tier", got)
	}

	oneStray := aggragates.DynamicParamsIndicators{Timeframe: "1D", Guppy: bearish, BMSB: -2, Valid: true}
	if got := dynamicparams.TierOf(oneStray); got != dynamicparams.TierMixed {
		t.Errorf("one bearish read beside a stray one: TierOf = %d, want mixed", got)
	}
}
