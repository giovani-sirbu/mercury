package aggragates

import "testing"

// The first-buy gate injection is a product rule: cooldown, UseAI,
// UsePatterns and SmartTakeLoss own it — the last for its quiet slow-decline
// hold; DynamicParams fetches its reads to shape the rows and holds nothing.
// Pinned so the difference between "fetches the verdict" and "gates the
// first buy" stays deliberate rather than accidental.
func TestInjectsEntryHoldIsOwnedByEntryFlags(t *testing.T) {
	cases := []struct {
		name   string
		params StrategyParams
		want   bool
	}{
		{"nothing on", StrategyParams{}, false},
		{"cooldown only", StrategyParams{Cooldown: true}, true},
		{"useAI only", StrategyParams{UseAI: true}, true},
		{"usePatterns only", StrategyParams{UsePatterns: true}, true},
		{"smartTakeLoss only", StrategyParams{SmartTakeLoss: true}, true},
		{"dynamicParams only", StrategyParams{DynamicParams: true}, false},
		{"smartTakeLoss with dynamicParams", StrategyParams{SmartTakeLoss: true, DynamicParams: true}, true},
		{"dynamicParams with cooldown", StrategyParams{DynamicParams: true, Cooldown: true}, true},
	}
	for _, tc := range cases {
		if got := tc.params.InjectsEntryHold(); got != tc.want {
			t.Errorf("%s: InjectsEntryHold = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// DynamicParams alone fetches sophos, on the /patterns route its reads ride
// on and nothing else, and gates no first buy: fetch is not gate.
func TestDynamicParamsFetchesThePatternRouteOnly(t *testing.T) {
	params := StrategyParams{DynamicParams: true}

	if !params.NeedsSophos() {
		t.Error("DynamicParams must fetch sophos")
	}
	if !params.NeedsPatternRoute() {
		t.Error("DynamicParams must fetch /patterns, where its reads ride")
	}
	if params.NeedsAIRoute() {
		t.Error("DynamicParams must not fetch the ML route")
	}
	if params.InjectsEntryHold() {
		t.Error("DynamicParams must not gate the first buy")
	}
}
