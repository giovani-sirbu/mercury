package aggragates

import (
	"reflect"
	"testing"
)

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
func TestMergeSophosVerdictsWithoutALegIsNoVerdict(t *testing.T) {
	armed := AIIndicators{
		AIAction:        ActionHold,
		AIMarketBearish: true,
		PatternName:     "bull_flag",
		SmartTakeLoss:   SmartTakeLossIndicators{SlowDeclineExit: true, SlowDeclineSellBand: 186.4},
	}
	got := MergeSophosVerdicts(StrategyParams{SmartTakeLoss: true, UseAI: true}, armed, armed, false, false)
	if !reflect.DeepEqual(got, AIIndicators{}) {
		t.Fatalf("no leg must merge to no verdict, got %+v", got)
	}
}
