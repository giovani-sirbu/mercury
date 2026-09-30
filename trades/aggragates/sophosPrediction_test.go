package aggragates

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestSophosPredictionIndicatorsMapsVerdict(t *testing.T) {
	raw := []byte(`{
		"action":"HOLD",
		"marketBearish":true,
		"marketBullish":false,
		"signalStrength":{"overall":42},
		"stayOutReasons":["low"],
		"unknownFutureKey":true,
		"smartTakeLoss":{"slowDeclineExit":true,"slowDeclineSellBand":186.4,"slowDeclineRecentAt":1640646000000,"slowDeclineRecentFrom":1640563200000,"slowDeclineRecentReasons":["read on the 1h bar opening 2021-12-27 23:00 UTC, one of the last 24 closed bars"],"slowDeclineFillFrom":1640563200000,"capitalProtectionUpperBB":193.83,"capitalProtectionSmcBearish":true},
		"patternVerdict":{"name":"asc_triangle","displayName":"ascending triangle","direction":"long","score":71,"level":96000,"levelKind":"resistance","stopLoss":94000,"takeProfit":104500,"interval":"15m"},
		"fib":{"swingLow":100,"swingHigh":110,"levels":[106.18,105,103.82,102.14]}
	}`)

	var prediction SophosPrediction
	if err := json.Unmarshal(raw, &prediction); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	mapped := prediction.Indicators()
	stl := mapped.SmartTakeLoss
	if !stl.SlowDeclineExit || stl.SlowDeclineSellBand != 186.4 || stl.CapitalProtectionUpperBB != 193.83 || !stl.CapitalProtectionSmcBearish {
		t.Fatalf("the smart take loss block must map field for field, got %+v", stl)
	}
	if stl.SlowDeclineRecentAt != 1640646000000 || stl.SlowDeclineRecentFrom != 1640563200000 || len(stl.SlowDeclineRecentReasons) != 1 || stl.SlowDeclineFillFrom != 1640563200000 {
		t.Fatalf("the recent bar, the oldest bar looked back over, their reasons and the fill window must map in ms and in order, got %+v", stl)
	}
	if mapped.AIAction != "HOLD" || mapped.AISignalStrength != 42 {
		t.Fatalf("model action must map, got %+v", mapped)
	}
	if mapped.PatternName != "asc_triangle" || mapped.PatternDisplayName != "ascending triangle" ||
		mapped.PatternDirection != "long" || mapped.PatternScore != 71 ||
		mapped.PatternLevel != 96000 || mapped.PatternLevelKind != "resistance" ||
		mapped.PatternStopLoss != 94000 || mapped.PatternTakeProfit != 104500 || mapped.PatternInterval != "15m" {
		t.Fatalf("pattern verdict must map field for field, got %+v", mapped)
	}
	if mapped.FibSwingLow != 100 || mapped.FibSwingHigh != 110 || len(mapped.FibLevels) != 4 || mapped.FibLevels[0] != 106.18 {
		t.Fatalf("fib retracement must map, got %+v", mapped)
	}
}

// An older sophos without the pattern keys decodes to zero pattern fields,
// which every pattern gate treats as "no pattern".
func TestSophosPredictionWithoutPatternVerdictIsInert(t *testing.T) {
	var prediction SophosPrediction
	if err := json.Unmarshal([]byte(`{"action":"LONG"}`), &prediction); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	mapped := prediction.Indicators()
	if mapped.PatternName != "" || mapped.PatternDirection != "" || mapped.PatternScore != 0 || mapped.PatternTakeProfit != 0 {
		t.Fatalf("missing pattern keys must map to zero, got %+v", mapped)
	}
	if len(mapped.FibLevels) != 0 || mapped.FibSwingHigh != 0 {
		t.Fatalf("missing fib keys must map to zero, got %+v", mapped)
	}
}

// An older sophos without the smartTakeLoss object, a block served with every
// key zero, and a sophos still serving the retired pattern-window keys of the
// trend-reversal rule all decode to the zero block, which gates/smarttakeloss
// treats as "nothing to read": no verdict, no band, no bearish trend.
func TestSophosPredictionWithoutSmartTakeLossIsInert(t *testing.T) {
	for name, raw := range map[string]string{
		"no object":             `{"action":"LONG"}`,
		"every key zero":        `{"smartTakeLoss":{"slowDeclineExit":false,"slowDeclineLegQuiet":false,"slowDeclineSmoothFrom":0,"slowDeclineFillBefore":0,"slowDeclineSellBand":0,"slowDeclineExitReasons":null,"slowDeclineBreakReasons":null,"slowDeclineIndecision":false,"slowDeclineRecentAt":0,"slowDeclineRecentFrom":0,"slowDeclineRecentReasons":null,"slowDeclineFillFrom":0,"capitalProtectionUpperBB":0,"capitalProtectionSmcBearish":false}}`,
		"the retired keys only": `{"smartTakeLoss":{"hasVerdict":true,"lowestBody":179.75,"upperBB":193.83,"lowerBB":176.4,"supportBarsUnder":3}}`,
	} {
		var prediction SophosPrediction
		if err := json.Unmarshal([]byte(raw), &prediction); err != nil {
			t.Fatalf("%s: unmarshal: %v", name, err)
		}
		got := prediction.Indicators().SmartTakeLoss
		if !reflect.DeepEqual(got, SmartTakeLossIndicators{}) {
			t.Fatalf("%s must map to the zero block, got %+v", name, got)
		}
	}
}

// The capital protection keys map onto the block field for field and apart
// from the slow decline's: the band arrives with the SMC trend bearish or
// not, and the SMC trend arrives without the band.
func TestSophosPredictionMapsTheCapitalProtection(t *testing.T) {
	for raw, want := range map[string]SmartTakeLossIndicators{
		`{"smartTakeLoss":{"capitalProtectionUpperBB":193.83,"capitalProtectionSmcBearish":true}}`:  {CapitalProtectionUpperBB: 193.83, CapitalProtectionSmcBearish: true},
		`{"smartTakeLoss":{"capitalProtectionUpperBB":193.83,"capitalProtectionSmcBearish":false}}`: {CapitalProtectionUpperBB: 193.83},
		`{"smartTakeLoss":{"capitalProtectionSmcBearish":true}}`:                                    {CapitalProtectionSmcBearish: true},
	} {
		var prediction SophosPrediction
		if err := json.Unmarshal([]byte(raw), &prediction); err != nil {
			t.Fatalf("unmarshal %s: %v", raw, err)
		}
		got := prediction.Indicators().SmartTakeLoss
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: got %+v, want %+v", raw, got, want)
		}
	}
}

func TestMergeSophosVerdictsPatternsThenML(t *testing.T) {
	params := StrategyParams{UseAI: true, UsePatterns: true}
	pattern := AIIndicators{
		AIAction:         "SHORT",
		PatternName:      "desc_triangle",
		PatternDirection: "short",
		PatternScore:     66,
		FibLevels:        []float64{106.18, 105},
	}
	ml := AIIndicators{
		AIAction:         "LONG",
		AIMarketBullish:  true,
		AISignalStrength: 9,
	}
	got := MergeSophosVerdicts(params, pattern, ml, true, true)
	if got.PatternAction != "SHORT" || got.AIAction != "LONG" || !got.AIMarketBullish {
		t.Fatalf("both legs must keep their actions, got %+v", got)
	}
	if got.PatternName != "desc_triangle" || got.PatternDirection != "short" || got.PatternScore != 66 || len(got.FibLevels) != 2 {
		t.Fatalf("the pattern verdict must survive the ML merge, got %+v", got)
	}
}

// The smart take loss reading rides the patterns leg through the merge
// untouched — the slow decline's verdict, band and reasons and capital
// protection's band and trend — whether or not an ML leg is merged over it
// and whether or not UseAI clears the pattern's own action: the gate and the
// first-fill hold read it from the merged block.
func TestMergeSophosVerdictsKeepsTheSlowDeclineExit(t *testing.T) {
	var decoded SophosPrediction
	raw := `{"action":"LONG","smartTakeLoss":{"slowDeclineExit":true,"slowDeclineSellBand":186.4,"slowDeclineExitReasons":["leg down 7.0% from its high close"],"capitalProtectionUpperBB":193.83,"capitalProtectionSmcBearish":true}}`
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	pattern := decoded.Indicators()
	ml := AIIndicators{AIAction: "SHORT", AIMarketBearish: true}

	for _, merge := range []struct {
		name   string
		params StrategyParams
		hasML  bool
	}{
		{"smartTakeLoss alone", StrategyParams{SmartTakeLoss: true}, false},
		{"with the ML leg under UseAI", StrategyParams{SmartTakeLoss: true, UseAI: true}, true},
		{"with the ML leg and patterns", StrategyParams{SmartTakeLoss: true, UseAI: true, UsePatterns: true}, true},
	} {
		got := MergeSophosVerdicts(merge.params, pattern, ml, true, merge.hasML).SmartTakeLoss
		if !reflect.DeepEqual(got, pattern.SmartTakeLoss) {
			t.Errorf("%s: the block must survive the merge, got %+v want %+v", merge.name, got, pattern.SmartTakeLoss)
		}
		if !got.SlowDeclineExit || got.SlowDeclineSellBand != 186.4 || len(got.SlowDeclineExitReasons) != 1 {
			t.Errorf("%s: the slow-decline reading must arrive, got %+v", merge.name, got)
		}
	}
}

// The ML route does not serve the smart take loss block: an ML-only merge
// keeps the ML verdict and carries no block, even a stray one on the ML leg.
func TestMergeSophosVerdictsMLOnlyCarriesNoSmartTakeLoss(t *testing.T) {
	params := StrategyParams{UseAI: true}
	ml := AIIndicators{
		AIAction:      "HOLD",
		SmartTakeLoss: SmartTakeLossIndicators{CapitalProtectionUpperBB: 1, CapitalProtectionSmcBearish: true},
	}
	got := MergeSophosVerdicts(params, AIIndicators{}, ml, false, true)
	if got.AIAction != "HOLD" {
		t.Fatalf("ML-only must keep the ML action, got %+v", got)
	}
	if !reflect.DeepEqual(got.SmartTakeLoss, SmartTakeLossIndicators{}) {
		t.Fatalf("ML-only must not carry a smart take loss block, got %+v", got.SmartTakeLoss)
	}
}

// Data flows, the flag owns the gate: the pattern fields are carried even
// when UsePatterns is off, exactly like the smart take loss block.
func TestMergeSophosVerdictsKeepsPatternFieldsWhenUsePatternsOff(t *testing.T) {
	pattern := AIIndicators{AIAction: "LONG", PatternName: "bull_flag", PatternDirection: "long", PatternScore: 80}
	got := MergeSophosVerdicts(StrategyParams{SmartTakeLoss: true}, pattern, AIIndicators{}, true, false)
	if got.PatternName != "bull_flag" || got.PatternDirection != "long" || got.PatternScore != 80 {
		t.Fatalf("pattern fields must not be zeroed by the flag, got %+v", got)
	}
}

func TestStrategyParamsNeedsSophosAndEntryHold(t *testing.T) {
	var none StrategyParams
	if none.NeedsSophos() || none.InjectsEntryHold() {
		t.Fatal("zero params must fetch nothing and inject no entry hold")
	}

	ai := StrategyParams{UseAI: true}
	if !ai.NeedsSophos() || !ai.NeedsAIRoute() || ai.NeedsPatternRoute() || ai.NeedsSmcTrendRoute() || !ai.InjectsEntryHold() {
		t.Fatal("UseAI must fetch the ML route and inject entry hold")
	}

	patterns := StrategyParams{UsePatterns: true}
	if !patterns.NeedsSophos() || !patterns.NeedsPatternRoute() || patterns.NeedsAIRoute() || patterns.NeedsSmcTrendRoute() || !patterns.InjectsEntryHold() {
		t.Fatal("UsePatterns must fetch /patterns and inject entry hold")
	}

	stl := StrategyParams{SmartTakeLoss: true}
	if !stl.NeedsSophos() || !stl.NeedsPatternRoute() || stl.NeedsAIRoute() || stl.NeedsSmcTrendRoute() || !stl.InjectsEntryHold() {
		t.Fatal("SmartTakeLoss fetches /patterns and injects the entry hold its slow-decline verdict owns")
	}

	cool := StrategyParams{Cooldown: true}
	if cool.NeedsSophos() || cool.NeedsSmcTrendRoute() {
		t.Fatal("Cooldown uses markers, not sophos")
	}
	if !cool.InjectsEntryHold() {
		t.Fatal("Cooldown must inject first-buy shouldHold")
	}

	// Flags that read no sophos verdict: each must fetch no route and inject no
	// entry hold on its own.
	for name, params := range map[string]StrategyParams{
		"Impasse":          {Impasse: true},
		"UseForceTrailing": {UseForceTrailing: true},
		"Pairs":            {Pairs: 3},
	} {
		if params.NeedsSophos() || params.NeedsPatternRoute() || params.NeedsAIRoute() || params.NeedsSmcTrendRoute() || params.InjectsEntryHold() {
			t.Errorf("%s must fetch nothing and inject no entry hold, got sophos=%v patterns=%v ai=%v smcTrend=%v entryHold=%v",
				name, params.NeedsSophos(), params.NeedsPatternRoute(), params.NeedsAIRoute(), params.NeedsSmcTrendRoute(), params.InjectsEntryHold())
		}
	}
}

// The quiet slow-decline keys map onto the block field for field — and the
// band arrives while the verdict is off, with no reasons.
func TestSophosPredictionMapsTheSlowDeclineExit(t *testing.T) {
	var active SophosPrediction
	raw := `{"smartTakeLoss":{"slowDeclineExit":true,"slowDeclineSellBand":186.4,"slowDeclineExitReasons":["leg down 7.0% from its high close","leg 60 bars long on 1h"]}}`
	if err := json.Unmarshal([]byte(raw), &active); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	stl := active.Indicators().SmartTakeLoss
	if !stl.SlowDeclineExit || stl.SlowDeclineSellBand != 186.4 || stl.CapitalProtectionUpperBB != 0 || stl.CapitalProtectionSmcBearish {
		t.Fatalf("the verdict and its band must map apart from capital protection, got %+v", stl)
	}
	if len(stl.SlowDeclineExitReasons) != 2 || stl.SlowDeclineExitReasons[1] != "leg 60 bars long on 1h" {
		t.Fatalf("the reasons must map in order, got %q", stl.SlowDeclineExitReasons)
	}

	var inactive SophosPrediction
	raw = `{"smartTakeLoss":{"slowDeclineExit":false,"slowDeclineSellBand":186.4,"slowDeclineExitReasons":null}}`
	if err := json.Unmarshal([]byte(raw), &inactive); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	stl = inactive.Indicators().SmartTakeLoss
	if stl.SlowDeclineExit || stl.SlowDeclineSellBand != 186.4 || stl.SlowDeclineExitReasons != nil {
		t.Fatalf("an inactive reading keeps its band and carries no reasons, got %+v", stl)
	}
	if stl.SlowDeclineLegQuiet || stl.SlowDeclineSmoothFrom != 0 || stl.SlowDeclineFillBefore != 0 {
		t.Fatalf("a sophos without the quiet-leg keys decodes to no quiet leg, no smooth bar and no bar a fill has to precede, got %+v", stl)
	}
}

// The quiet leg, the bar sophos reads it smooth from and the bar a fill has
// to precede map onto the block apart from the verdict: a leg on and quiet
// whose vote passes only with a ladder's own smoothness arrives with both
// bars, in ms, and no verdict.
func TestSophosPredictionMapsTheQuietLegAndItsSmoothBar(t *testing.T) {
	var rough SophosPrediction
	raw := `{"smartTakeLoss":{"slowDeclineExit":false,"slowDeclineLegQuiet":true,"slowDeclineSmoothFrom":1640646000000,"slowDeclineFillBefore":1640732400000,"slowDeclineSellBand":186.4,"slowDeclineExitReasons":["leg down 7.0% from its high close"]}}`
	if err := json.Unmarshal([]byte(raw), &rough); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	stl := rough.Indicators().SmartTakeLoss
	if stl.SlowDeclineExit || !stl.SlowDeclineLegQuiet || stl.SlowDeclineSmoothFrom != 1640646000000 {
		t.Fatalf("the quiet leg and its smooth bar must map apart from the verdict, got %+v", stl)
	}
	if stl.SlowDeclineFillBefore != 1640732400000 {
		t.Fatalf("the bar a fill has to precede must map in ms, got %d", stl.SlowDeclineFillBefore)
	}
	if stl.SlowDeclineSellBand != 186.4 || len(stl.SlowDeclineExitReasons) != 1 {
		t.Fatalf("the band and the reasons ride with the quiet leg, got %+v", stl)
	}
	if stl.SlowDeclineBreakReasons != nil {
		t.Fatalf("a leg on and quiet carries no break reasons, got %q", stl.SlowDeclineBreakReasons)
	}
}

// The break reasons map in order apart from the exit's reasons: a reading
// whose leg is not on and quiet arrives with its band and what broke, and
// no verdict, no quiet leg and no exit reasons.
func TestSophosPredictionMapsTheBreakReasons(t *testing.T) {
	var broken SophosPrediction
	raw := `{"smartTakeLoss":{"slowDeclineExit":false,"slowDeclineLegQuiet":false,"slowDeclineSellBand":85.36,"slowDeclineExitReasons":null,"slowDeclineBreakReasons":["NATR(14) 1.04x its median sampled every 24 bars, over 1.00x","2 of 4 readings hold with a smooth ladder, 3 needed"]}}`
	if err := json.Unmarshal([]byte(raw), &broken); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	stl := broken.Indicators().SmartTakeLoss
	if stl.SlowDeclineExit || stl.SlowDeclineLegQuiet || stl.SlowDeclineSellBand != 85.36 || stl.SlowDeclineExitReasons != nil {
		t.Fatalf("a broken reading keeps its band alone, got %+v", stl)
	}
	want := []string{"NATR(14) 1.04x its median sampled every 24 bars, over 1.00x", "2 of 4 readings hold with a smooth ladder, 3 needed"}
	if len(stl.SlowDeclineBreakReasons) != len(want) || stl.SlowDeclineBreakReasons[0] != want[0] || stl.SlowDeclineBreakReasons[1] != want[1] {
		t.Fatalf("the break reasons must map in order, got %q", stl.SlowDeclineBreakReasons)
	}

	var older SophosPrediction
	if err := json.Unmarshal([]byte(`{"smartTakeLoss":{"slowDeclineMiddleBB":85.36}}`), &older); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if block := older.Indicators().SmartTakeLoss; block.SlowDeclineBreakReasons != nil || block.SlowDeclineSellBand != 0 {
		t.Fatalf("a sophos without the keys — the retired band key included — decodes to no break reasons and no sell band, got %+v", block)
	}
}

// The indecision reading maps onto the block apart from the verdict and the
// leg on and quiet: a vote read one short of the need arrives with its band
// and what broke, and a sophos without the key decodes to no indecision.
func TestSophosPredictionMapsTheIndecision(t *testing.T) {
	broken := []string{"base volume of the last 24 bars 1.09x the median of the 720 before, over 0.90x", "2 of 4 readings hold with a smooth ladder, 3 needed"}
	var undecided SophosPrediction
	raw := `{"smartTakeLoss":{"slowDeclineExit":false,"slowDeclineLegQuiet":false,"slowDeclineSellBand":79.41,"slowDeclineBreakReasons":["base volume of the last 24 bars 1.09x the median of the 720 before, over 0.90x","2 of 4 readings hold with a smooth ladder, 3 needed"],"slowDeclineIndecision":true}}`
	if err := json.Unmarshal([]byte(raw), &undecided); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := SmartTakeLossIndicators{SlowDeclineSellBand: 79.41, SlowDeclineBreakReasons: broken, SlowDeclineIndecision: true}
	if got := undecided.Indicators().SmartTakeLoss; !reflect.DeepEqual(got, want) {
		t.Fatalf("the indecision reading must map field for field, got %+v want %+v", got, want)
	}

	var older SophosPrediction
	if err := json.Unmarshal([]byte(`{"smartTakeLoss":{"slowDeclineSellBand":79.41,"slowDeclineBreakReasons":["2 of 4 readings hold with a smooth ladder, 3 needed"]}}`), &older); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if block := older.Indicators().SmartTakeLoss; block.SlowDeclineIndecision || len(block.SlowDeclineBreakReasons) != 1 {
		t.Fatalf("a sophos without the key decodes to no indecision beside its break reasons, got %+v", block)
	}
}

// The pattern and ML routes carry no DynamicParams reads: a body still
// serving the dynamicParams block an earlier sophos build put on /patterns
// decodes to no reads, so no verdict merged off those legs can open a ladder
// raised. The reads are the smc-trend leg's alone (SophosSmcTrend).
func TestSophosPredictionCarriesNoDynamicParams(t *testing.T) {
	var prediction SophosPrediction
	raw := `{"action":"LONG","dynamicParams":{"timeframe":"1D","guppy":-1,"bmsb":-1,"valid":true}}`
	if err := json.Unmarshal([]byte(raw), &prediction); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := prediction.Indicators().DynamicParams; got != (DynamicParamsIndicators{}) {
		t.Fatalf("the pattern leg must carry no reads, got %+v", got)
	}
}
