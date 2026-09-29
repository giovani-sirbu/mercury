package dynamicparams

import (
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// Blocks on the timeframe the rows name, one per shape a row can report.
var (
	messageBothBearish = aggragates.DynamicParamsIndicators{Timeframe: "1D", Guppy: -1, BMSB: -1, Valid: true}
	messageMixedGuppy  = aggragates.DynamicParamsIndicators{Timeframe: "1D", Guppy: -1, BMSB: 0, Valid: true}
	messageMixedBMSB   = aggragates.DynamicParamsIndicators{Timeframe: "1D", Guppy: 1, BMSB: -1, Valid: true}
	messageNotBearish  = aggragates.DynamicParamsIndicators{Timeframe: "1D", Guppy: 1, BMSB: 0, Valid: true}
)

// The rows the shipped constants write, byte for byte: every engine writes
// this text and agora filters on its prefix.
func TestTransitionMessageBytes(t *testing.T) {
	cases := []struct {
		name  string
		reads aggragates.DynamicParamsIndicators
		want  string
	}{
		{"both bearish", messageBothBearish, "dynamic params: 1D Super Guppy bearish, BMSB bearish: both bearish, percentage +0.5 and depths +1 on every row"},
		{"mixed on the BMSB", messageMixedBMSB, "dynamic params: 1D Super Guppy bullish, BMSB bearish: mixed, percentage +0.5 on every row"},
		{"mixed on the Super Guppy", messageMixedGuppy, "dynamic params: 1D Super Guppy bearish, BMSB neutral: mixed, percentage +0.5 on every row"},
		{"not bearish", messageNotBearish, "dynamic params: 1D Super Guppy bullish, BMSB neutral: not bearish, configured rows"},
		{"not read", aggragates.DynamicParamsIndicators{Timeframe: "1D"}, "dynamic params: 1D not read, configured rows"},
		{"not read over stale bearish reads", aggragates.DynamicParamsIndicators{Timeframe: "1D", Guppy: -1, BMSB: -1}, "dynamic params: 1D not read, configured rows"},
		{"the zero block", aggragates.DynamicParamsIndicators{}, "dynamic params: not read, configured rows"},
	}

	for _, c := range cases {
		got := TransitionMessage(c.reads)
		if got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
		if !strings.HasPrefix(got, TransitionPrefix) {
			t.Errorf("%s: %q does not open with %q", c.name, got, TransitionPrefix)
		}
	}
}

// Every tier under every increase the mixed tier can take, byte for byte:
// both bearish and the base tier read the same whatever MixedIncrease is,
// and the mixed tier names exactly what it raises.
func TestTransitionMessageForEveryTierAndIncrease(t *testing.T) {
	const both = "dynamic params: 1D Super Guppy bearish, BMSB bearish: both bearish, percentage +0.5 and depths +1 on every row"
	const base = "dynamic params: 1D Super Guppy bullish, BMSB neutral: not bearish, configured rows"
	const mixedHead = "dynamic params: 1D Super Guppy bullish, BMSB bearish: mixed, "

	mixedTails := map[Increase]string{
		IncreasePercentage: "percentage +0.5 on every row",
		IncreaseDepths:     "depths +1 on every row",
		IncreaseBoth:       "percentage +0.5 and depths +1 on every row",
		IncreaseNone:       "configured rows",
	}

	for mixed, tail := range mixedTails {
		if got := transitionMessageFor(messageBothBearish, mixed); got != both {
			t.Errorf("both bearish, mixed %q:\n got %q\nwant %q", mixed, got, both)
		}
		if got := transitionMessageFor(messageMixedBMSB, mixed); got != mixedHead+tail {
			t.Errorf("mixed, mixed %q:\n got %q\nwant %q", mixed, got, mixedHead+tail)
		}
		if got := transitionMessageFor(messageNotBearish, mixed); got != base {
			t.Errorf("not bearish, mixed %q:\n got %q\nwant %q", mixed, got, base)
		}
	}
}

// A read off the wire's three values reads as neutral, the way TierOf counts
// it.
func TestTransitionMessageNamesAStrayReadNeutral(t *testing.T) {
	stray := aggragates.DynamicParamsIndicators{Timeframe: "1D", Guppy: 2, BMSB: -2, Valid: true}
	want := "dynamic params: 1D Super Guppy neutral, BMSB neutral: not bearish, configured rows"
	if got := TransitionMessage(stray); got != want {
		t.Errorf("stray reads:\n got %q\nwant %q", got, want)
	}
}
