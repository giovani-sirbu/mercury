package aggragates

import (
	"reflect"
	"testing"
)

// The slow-decline verdict travels with the regime block, so it follows the
// same leg: the pattern leg's when it succeeded, whatever the ML leg says,
// and the ML leg's only when the pattern leg failed. The two legs disagree on
// every field here, so a verdict taken from the wrong leg fails.
func TestMergeSophosVerdictsTakesTheSlowDeclineFromTheRegimeLeg(t *testing.T) {
	pattern := AIIndicators{
		HasRegimeVerdict:   true,
		SlowDecline:        true,
		FreeFall:           false,
		SlowDeclineReasons: []string{"leg down 13.0% from its high close", "leg 60h long"},
	}
	ml := AIIndicators{
		HasRegimeVerdict:   true,
		SlowDecline:        false,
		FreeFall:           true,
		SlowDeclineReasons: []string{"from the ML leg"},
	}

	for _, params := range []StrategyParams{{UsePatterns: true}, {UsePatterns: true, UseAI: true}, {CrashGuard: true}} {
		both := MergeSophosVerdicts(params, pattern, ml, true, true)
		if both.SlowDecline != pattern.SlowDecline || both.FreeFall != pattern.FreeFall ||
			!reflect.DeepEqual(both.SlowDeclineReasons, pattern.SlowDeclineReasons) {
			t.Errorf("%+v: both legs up must keep the pattern leg's verdict, got %v/%v %q",
				params, both.SlowDecline, both.FreeFall, both.SlowDeclineReasons)
		}

		mlOnly := MergeSophosVerdicts(params, pattern, ml, false, true)
		if mlOnly.SlowDecline != ml.SlowDecline || mlOnly.FreeFall != ml.FreeFall ||
			!reflect.DeepEqual(mlOnly.SlowDeclineReasons, ml.SlowDeclineReasons) {
			t.Errorf("%+v: a failed pattern leg must take the ML leg's verdict, got %v/%v %q",
				params, mlOnly.SlowDecline, mlOnly.FreeFall, mlOnly.SlowDeclineReasons)
		}

		patternOnly := MergeSophosVerdicts(params, pattern, ml, true, false)
		if patternOnly.SlowDecline != pattern.SlowDecline || patternOnly.FreeFall != pattern.FreeFall {
			t.Errorf("%+v: a failed ML leg must keep the pattern leg's verdict, got %v/%v",
				params, patternOnly.SlowDecline, patternOnly.FreeFall)
		}
	}
}

// The dynamic params reads ride the patterns leg through the merge
// untouched, with or without an ML leg merged over it. The ML route does not
// serve them, so an ML-only merge carries none — a stray block on that leg
// included — and neither does a merge with no leg up: both read as not read,
// which raises no row.
func TestMergeSophosVerdictsCarriesTheDynamicParamsOnThePatternLeg(t *testing.T) {
	pattern := AIIndicators{
		AIAction:      "LONG",
		DynamicParams: DynamicParamsIndicators{Timeframe: "1D", Guppy: -1, BMSB: -1, Valid: true},
	}
	ml := AIIndicators{
		AIAction:        "SHORT",
		AIMarketBearish: true,
		DynamicParams:   DynamicParamsIndicators{Timeframe: "4h", Guppy: 1, BMSB: 1, Valid: true},
	}

	for _, merge := range []struct {
		name   string
		params StrategyParams
		hasML  bool
	}{
		{"dynamicParams alone", StrategyParams{DynamicParams: true}, false},
		{"with the ML leg under UseAI", StrategyParams{DynamicParams: true, UseAI: true}, true},
		{"with the ML leg and patterns", StrategyParams{DynamicParams: true, UseAI: true, UsePatterns: true}, true},
	} {
		got := MergeSophosVerdicts(merge.params, pattern, ml, true, merge.hasML).DynamicParams
		if got != pattern.DynamicParams {
			t.Errorf("%s: the pattern leg's block must survive the merge, got %+v want %+v", merge.name, got, pattern.DynamicParams)
		}
	}

	params := StrategyParams{DynamicParams: true, UseAI: true}
	if got := MergeSophosVerdicts(params, pattern, ml, false, true).DynamicParams; got != (DynamicParamsIndicators{}) {
		t.Errorf("an ML-only merge must carry no block, got %+v", got)
	}
	if got := MergeSophosVerdicts(params, pattern, ml, false, false).DynamicParams; got != (DynamicParamsIndicators{}) {
		t.Errorf("a merge with no leg up must carry no block, got %+v", got)
	}
}

// Neither leg up is no verdict at all: nothing may hold on it.
func TestMergeSophosVerdictsWithoutALegCarriesNoSlowDecline(t *testing.T) {
	armed := AIIndicators{HasRegimeVerdict: true, SlowDecline: true, FreeFall: true, SlowDeclineReasons: []string{"x"}}
	got := MergeSophosVerdicts(StrategyParams{CrashGuard: true, UseAI: true}, armed, armed, false, false)
	if got.SlowDecline || got.FreeFall || got.SlowDeclineReasons != nil {
		t.Fatalf("no leg must merge to no slow-decline verdict, got %v/%v %q", got.SlowDecline, got.FreeFall, got.SlowDeclineReasons)
	}
}
