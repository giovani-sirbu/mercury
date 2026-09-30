package aggragates

import "testing"

// fetchAndHold is what one set of flags makes the engines fetch from sophos,
// route by route, and whether it injects the first-buy hold.
type fetchAndHold struct {
	sophos, patterns, smcTrend, ai, entryHold bool
}

// fetchAndHoldOf reads the five predicates off params.
func fetchAndHoldOf(params StrategyParams) fetchAndHold {
	return fetchAndHold{
		sophos:    params.NeedsSophos(),
		patterns:  params.NeedsPatternRoute(),
		smcTrend:  params.NeedsSmcTrendRoute(),
		ai:        params.NeedsAIRoute(),
		entryHold: params.InjectsEntryHold(),
	}
}

// The first-buy gate injection is a product rule: cooldown, UseAI,
// UsePatterns and SmartTakeLoss own it — the last for its quiet slow-decline
// hold; DynamicParams fetches /smc-trend for its reads, decides the rows a
// ladder opens with and holds nothing. Pinned beside what each set of flags
// fetches, so the difference between "fetches the verdict" and "gates the
// first buy" stays deliberate rather than accidental.
func TestInjectsEntryHoldIsOwnedByEntryFlags(t *testing.T) {
	cases := []struct {
		name   string
		params StrategyParams
		want   fetchAndHold
	}{
		{"nothing on", StrategyParams{}, fetchAndHold{}},
		{"cooldown only", StrategyParams{Cooldown: true}, fetchAndHold{entryHold: true}},
		{"useAI only", StrategyParams{UseAI: true}, fetchAndHold{sophos: true, ai: true, entryHold: true}},
		{"usePatterns only", StrategyParams{UsePatterns: true}, fetchAndHold{sophos: true, patterns: true, entryHold: true}},
		{"smartTakeLoss only", StrategyParams{SmartTakeLoss: true}, fetchAndHold{sophos: true, patterns: true, entryHold: true}},
		{"dynamicParams only", StrategyParams{DynamicParams: true}, fetchAndHold{sophos: true, smcTrend: true}},
		{"smartTakeLoss with dynamicParams", StrategyParams{SmartTakeLoss: true, DynamicParams: true}, fetchAndHold{sophos: true, patterns: true, smcTrend: true, entryHold: true}},
		{"dynamicParams with cooldown", StrategyParams{DynamicParams: true, Cooldown: true}, fetchAndHold{sophos: true, smcTrend: true, entryHold: true}},
	}
	for _, tc := range cases {
		if got := fetchAndHoldOf(tc.params); got != tc.want {
			t.Errorf("%s: fetches and holds %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

// DynamicParams alone fetches sophos, on the /smc-trend route its reads come
// from and nothing else — never /patterns, which carries no reads — and gates
// no first buy: fetch is not gate.
func TestDynamicParamsFetchesTheSmcTrendRouteOnly(t *testing.T) {
	params := StrategyParams{DynamicParams: true}

	if !params.NeedsSophos() {
		t.Error("DynamicParams must fetch sophos")
	}
	if !params.NeedsSmcTrendRoute() {
		t.Error("DynamicParams must fetch /smc-trend, where its reads come from")
	}
	if params.NeedsPatternRoute() {
		t.Error("DynamicParams must not fetch /patterns, which carries no reads")
	}
	if params.NeedsAIRoute() {
		t.Error("DynamicParams must not fetch the ML route")
	}
	if params.InjectsEntryHold() {
		t.Error("DynamicParams must not gate the first buy")
	}
}
