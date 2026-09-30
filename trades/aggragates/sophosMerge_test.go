package aggragates

import (
	"encoding/json"
	"reflect"
	"testing"
)

// decodedLeg is one sophos body the way every engine decodes a /patterns or
// ML answer: the SophosPrediction wire contract, mapped by its Indicators.
func decodedLeg(t *testing.T, body string) AIIndicators {
	t.Helper()
	var prediction SophosPrediction
	if err := json.Unmarshal([]byte(body), &prediction); err != nil {
		t.Fatalf("unmarshal %s: %v", body, err)
	}
	return prediction.Indicators()
}

// Neither leg carries the DynamicParams reads, so no merge of them does. A
// /patterns body still serving the dynamicParams block an earlier sophos
// build put on it and an ML body carrying the same stray block decode to legs
// without reads, and every merge of them — under the flag alone or beside
// UseAI, on the pattern leg alone, on both, on the ML leg alone and on
// neither — carries none: the reads are the smc-trend leg's
// (NeedsSmcTrendRoute), which the engines set after the merge.
func TestMergeSophosVerdictsOfDecodedLegsCarriesNoDynamicParams(t *testing.T) {
	stray := `"dynamicParams":{"timeframe":"1D","guppy":-1,"bmsb":-1,"valid":true}`
	pattern := decodedLeg(t, `{"action":"LONG","smartTakeLoss":{"slowDeclineSellBand":186.4},`+stray+`}`)
	ml := decodedLeg(t, `{"action":"SHORT","marketBearish":true,`+stray+`}`)
	if pattern.AIAction != ActionLong || ml.AIAction != ActionShort {
		t.Fatalf("fixture drifted: the legs must decode their actions, got %q and %q", pattern.AIAction, ml.AIAction)
	}

	for _, params := range []StrategyParams{
		{DynamicParams: true},
		{DynamicParams: true, UseAI: true},
		{DynamicParams: true, UseAI: true, UsePatterns: true, SmartTakeLoss: true},
	} {
		for _, legs := range []struct {
			hasPattern bool
			hasML      bool
		}{
			{hasPattern: true},
			{hasPattern: true, hasML: true},
			{hasML: true},
			{},
		} {
			merged := MergeSophosVerdicts(params, pattern, ml, legs.hasPattern, legs.hasML)
			if (legs.hasPattern && merged.PatternAction != ActionLong) || (legs.hasML && merged.AIAction != ActionShort) {
				t.Fatalf("%+v, legs %+v: the merge must take the legs it is handed, got %+v", params, legs, merged)
			}
			if merged.DynamicParams != (DynamicParamsIndicators{}) {
				t.Errorf("%+v, legs %+v: the merge must carry no reads, got %+v", params, legs, merged.DynamicParams)
			}
		}
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
