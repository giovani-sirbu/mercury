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
		"smartTakeLoss":{"quietDeclineVerdict":true,"quietDeclineRecentAt":1640646000000,"quietDeclineRecentFrom":1640563200000,"quietDeclineRecentReasons":["read on the 1h bar opening 2021-12-27 23:00 UTC, one of the last 24 closed bars"],"quietDeclineFillFrom":1640563200000,"capitalProtectionSmcBearish":true,"capitalProtectionSmcBullish":false},
		"patternVerdict":{"name":"asc_triangle","displayName":"ascending triangle","direction":"long","score":71,"level":96000,"levelKind":"resistance","stopLoss":94000,"takeProfit":104500,"interval":"15m"},
		"fib":{"swingLow":100,"swingHigh":110,"levels":[106.18,105,103.82,102.14]}
	}`)

	var prediction SophosPrediction
	if err := json.Unmarshal(raw, &prediction); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	mapped := prediction.Indicators()
	stl := mapped.SmartTakeLoss
	if !stl.QuietDeclineVerdict || !stl.CapitalProtectionSmcBearish || stl.CapitalProtectionSmcBullish {
		t.Fatalf("the smart take loss block must map field for field, got %+v", stl)
	}
	if stl.QuietDeclineRecentAt != 1640646000000 || stl.QuietDeclineRecentFrom != 1640563200000 || len(stl.QuietDeclineRecentReasons) != 1 || stl.QuietDeclineFillFrom != 1640563200000 {
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
		"every key zero":        `{"smartTakeLoss":{"quietDeclineVerdict":false,"quietDeclineLegQuiet":false,"quietDeclineSmoothFrom":0,"quietDeclineFillBefore":0,"quietDeclineExitReasons":null,"quietDeclineBreakReasons":null,"indecision":false,"quietDeclineRecentAt":0,"quietDeclineRecentFrom":0,"quietDeclineRecentReasons":null,"quietDeclineFillFrom":0,"capitalProtectionUpperBB":0,"capitalProtectionSmcBearish":false}}`,
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

// A payload still carrying the retired `slowDeclineSellBand` and
// `capitalProtectionUpperBB` keys, as a sophos that has not dropped them yet
// serves them, decodes without an error and to the block the same payload
// without the keys decodes to: encoding/json ignores a key the wire struct
// names no field for, so the block holds no such band and what the other keys
// serve maps as it does without it. A payload carrying nothing else decodes to
// the zero block, which gates/smarttakeloss reads as nothing to read.
func TestSophosPredictionIgnoresTheRetiredSellBandKey(t *testing.T) {
	const retired = `"slowDeclineSellBand":186.4,"capitalProtectionUpperBB":193.83`
	for name, tc := range map[string]struct {
		raw  string
		want SmartTakeLossIndicators
	}{
		"beside the keys the block reads": {
			`{"smartTakeLoss":{"quietDeclineVerdict":true,` + retired + `,"quietDeclineExitReasons":["leg down 7.0% from its high close"],"quietDeclineFillFrom":1640563200000,"capitalProtectionSmcBearish":true}}`,
			SmartTakeLossIndicators{
				QuietDeclineVerdict:         true,
				QuietDeclineExitReasons:     []string{"leg down 7.0% from its high close"},
				QuietDeclineFillFrom:        1640563200000,
				CapitalProtectionSmcBearish: true,
			},
		},
		"alone": {
			`{"smartTakeLoss":{` + retired + `}}`,
			SmartTakeLossIndicators{},
		},
	} {
		var prediction SophosPrediction
		if err := json.Unmarshal([]byte(tc.raw), &prediction); err != nil {
			t.Fatalf("%s: unmarshal: %v", name, err)
		}
		if got := prediction.Indicators().SmartTakeLoss; !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s must decode as without the retired key, got %+v, want %+v", name, got, tc.want)
		}
	}
}

// The capital protection keys map onto the block field for field and apart
// from the slow decline's: each trend flag arrives with the other or alone.
func TestSophosPredictionMapsTheCapitalProtection(t *testing.T) {
	for raw, want := range map[string]SmartTakeLossIndicators{
		`{"smartTakeLoss":{"capitalProtectionSmcBearish":true,"capitalProtectionSmcBullish":true}}`:  {CapitalProtectionSmcBearish: true, CapitalProtectionSmcBullish: true},
		`{"smartTakeLoss":{"capitalProtectionSmcBearish":true,"capitalProtectionSmcBullish":false}}`: {CapitalProtectionSmcBearish: true},
		`{"smartTakeLoss":{"capitalProtectionSmcBullish":true}}`:                                     {CapitalProtectionSmcBullish: true},
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
// untouched — the slow decline's verdict and reasons and capital protection's
// band and trend — whether or not an ML leg is merged over it and whether or
// not UseAI clears the pattern's own action: the gate and the first-fill hold
// read it from the merged block.
func TestMergeSophosVerdictsKeepsTheSlowDeclineExit(t *testing.T) {
	var decoded SophosPrediction
	raw := `{"action":"LONG","smartTakeLoss":{"quietDeclineVerdict":true,"quietDeclineExitReasons":["leg down 7.0% from its high close"],"capitalProtectionUpperBB":193.83,"capitalProtectionSmcBearish":true}}`
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
		if !got.QuietDeclineVerdict || len(got.QuietDeclineExitReasons) != 1 {
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
		SmartTakeLoss: SmartTakeLossIndicators{CapitalProtectionSmcBearish: true, CapitalProtectionSmcBullish: true},
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

// Sophos' quiet slow-decline keys map onto the block field for field — and the
// fill window arrives while the verdict is off, with no reasons.
func TestSophosPredictionMapsTheSlowDeclineExit(t *testing.T) {
	var active SophosPrediction
	raw := `{"smartTakeLoss":{"quietDeclineVerdict":true,"quietDeclineExitReasons":["leg down 7.0% from its high close","leg 60 bars long on 1h"]}}`
	if err := json.Unmarshal([]byte(raw), &active); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	stl := active.Indicators().SmartTakeLoss
	if !stl.QuietDeclineVerdict || stl.CapitalProtectionSmcBearish || stl.CapitalProtectionSmcBullish {
		t.Fatalf("the verdict must map apart from capital protection, got %+v", stl)
	}
	if len(stl.QuietDeclineExitReasons) != 2 || stl.QuietDeclineExitReasons[1] != "leg 60 bars long on 1h" {
		t.Fatalf("the reasons must map in order, got %q", stl.QuietDeclineExitReasons)
	}

	var inactive SophosPrediction
	raw = `{"smartTakeLoss":{"quietDeclineVerdict":false,"quietDeclineFillFrom":1640563200000,"quietDeclineExitReasons":null}}`
	if err := json.Unmarshal([]byte(raw), &inactive); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	stl = inactive.Indicators().SmartTakeLoss
	if stl.QuietDeclineVerdict || stl.QuietDeclineFillFrom != 1640563200000 || stl.QuietDeclineExitReasons != nil {
		t.Fatalf("an inactive reading keeps its fill window and carries no reasons, got %+v", stl)
	}
	if stl.QuietDeclineLegQuiet || stl.QuietDeclineSmoothFrom != 0 || stl.QuietDeclineFillBefore != 0 {
		t.Fatalf("a sophos without the quiet-leg keys decodes to no quiet leg, no smooth bar and no bar a fill has to precede, got %+v", stl)
	}
}

// The quiet leg, the bar sophos reads it smooth from and the bar a fill has
// to precede map onto the block apart from the verdict: a leg on and quiet
// whose vote passes only with a ladder's own smoothness arrives with both
// bars, in ms, and no verdict.
func TestSophosPredictionMapsTheQuietLegAndItsSmoothBar(t *testing.T) {
	var rough SophosPrediction
	raw := `{"smartTakeLoss":{"quietDeclineVerdict":false,"quietDeclineLegQuiet":true,"quietDeclineSmoothFrom":1640646000000,"quietDeclineFillBefore":1640732400000,"quietDeclineExitReasons":["leg down 7.0% from its high close"]}}`
	if err := json.Unmarshal([]byte(raw), &rough); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	stl := rough.Indicators().SmartTakeLoss
	if stl.QuietDeclineVerdict || !stl.QuietDeclineLegQuiet || stl.QuietDeclineSmoothFrom != 1640646000000 {
		t.Fatalf("the quiet leg and its smooth bar must map apart from the verdict, got %+v", stl)
	}
	if stl.QuietDeclineFillBefore != 1640732400000 {
		t.Fatalf("the bar a fill has to precede must map in ms, got %d", stl.QuietDeclineFillBefore)
	}
	if len(stl.QuietDeclineExitReasons) != 1 {
		t.Fatalf("the reasons ride with the quiet leg, got %+v", stl)
	}
	if stl.QuietDeclineBreakReasons != nil {
		t.Fatalf("a leg on and quiet carries no break reasons, got %q", stl.QuietDeclineBreakReasons)
	}
}

// The break reasons map in order apart from the exit's reasons: a reading
// whose leg is not on and quiet arrives with its fill window and what broke,
// and no verdict, no quiet leg and no exit reasons.
func TestSophosPredictionMapsTheBreakReasons(t *testing.T) {
	var broken SophosPrediction
	raw := `{"smartTakeLoss":{"quietDeclineVerdict":false,"quietDeclineLegQuiet":false,"quietDeclineFillFrom":1640563200000,"quietDeclineExitReasons":null,"quietDeclineBreakReasons":["NATR(14) 1.04x its median sampled every 24 bars, over 1.00x","2 of 4 readings hold with a smooth ladder, 3 needed"]}}`
	if err := json.Unmarshal([]byte(raw), &broken); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	stl := broken.Indicators().SmartTakeLoss
	if stl.QuietDeclineVerdict || stl.QuietDeclineLegQuiet || stl.QuietDeclineFillFrom != 1640563200000 || stl.QuietDeclineExitReasons != nil {
		t.Fatalf("a broken reading keeps its fill window alone, got %+v", stl)
	}
	want := []string{"NATR(14) 1.04x its median sampled every 24 bars, over 1.00x", "2 of 4 readings hold with a smooth ladder, 3 needed"}
	if len(stl.QuietDeclineBreakReasons) != len(want) || stl.QuietDeclineBreakReasons[0] != want[0] || stl.QuietDeclineBreakReasons[1] != want[1] {
		t.Fatalf("the break reasons must map in order, got %q", stl.QuietDeclineBreakReasons)
	}

	var older SophosPrediction
	if err := json.Unmarshal([]byte(`{"smartTakeLoss":{"slowDeclineMiddleBB":85.36}}`), &older); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if block := older.Indicators().SmartTakeLoss; !reflect.DeepEqual(block, SmartTakeLossIndicators{}) {
		t.Fatalf("a sophos without the keys — the retired middle band key included — decodes to the zero block, got %+v", block)
	}
}

// The indecision reading maps onto the block apart from the verdict and the
// leg on and quiet: a vote read one short of the need arrives with its fill
// window and what broke, and a sophos without the key decodes to no
// indecision.
func TestSophosPredictionMapsTheIndecision(t *testing.T) {
	broken := []string{"base volume of the last 24 bars 1.09x the median of the 720 before, over 0.90x", "2 of 4 readings hold with a smooth ladder, 3 needed"}
	var undecided SophosPrediction
	raw := `{"smartTakeLoss":{"quietDeclineVerdict":false,"quietDeclineLegQuiet":false,"quietDeclineFillFrom":1640563200000,"quietDeclineBreakReasons":["base volume of the last 24 bars 1.09x the median of the 720 before, over 0.90x","2 of 4 readings hold with a smooth ladder, 3 needed"],"indecision":true}}`
	if err := json.Unmarshal([]byte(raw), &undecided); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := SmartTakeLossIndicators{QuietDeclineFillFrom: 1640563200000, QuietDeclineBreakReasons: broken, Indecision: true}
	if got := undecided.Indicators().SmartTakeLoss; !reflect.DeepEqual(got, want) {
		t.Fatalf("the indecision reading must map field for field, got %+v want %+v", got, want)
	}

	var older SophosPrediction
	if err := json.Unmarshal([]byte(`{"smartTakeLoss":{"quietDeclineBreakReasons":["2 of 4 readings hold with a smooth ladder, 3 needed"]}}`), &older); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if block := older.Indicators().SmartTakeLoss; block.Indecision || len(block.QuietDeclineBreakReasons) != 1 {
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

// The series' four keys decode as the parallel arrays sophos serves — the open
// time in ms, the close, the high and the low of each closed 1h bar, oldest
// first — and map onto the block untouched.
func TestSophosPredictionMapsTheSwingDeclineSeries(t *testing.T) {
	raw := `{"smartTakeLoss":{"seriesOpens":[1633680000000,1633683600000,1633687200000],"seriesCloses":[168.2,167.9,167.4],"seriesHighs":[169.1,168.4,167.8],"seriesLows":[167.5,167.2,166.9]}}`
	var prediction SophosPrediction
	if err := json.Unmarshal([]byte(raw), &prediction); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := SmartTakeLossIndicators{
		SeriesOpens:  []int64{1633680000000, 1633683600000, 1633687200000},
		SeriesCloses: []float64{168.2, 167.9, 167.4},
		SeriesHighs:  []float64{169.1, 168.4, 167.8},
		SeriesLows:   []float64{167.5, 167.2, 166.9},
	}
	if got := prediction.Indicators().SmartTakeLoss; !reflect.DeepEqual(got, want) {
		t.Fatalf("the series must map as it decodes, got %+v, want %+v", got, want)
	}
}

// A sophos that serves the keys null — the window was not read — and one that
// does not serve them at all both decode to no series: the block stays nil,
// not empty, so the zero block comparison of every older reader still holds.
func TestSophosPredictionWithoutTheSwingDeclineSeriesIsInert(t *testing.T) {
	for name, raw := range map[string]string{
		"keys null":   `{"smartTakeLoss":{"seriesOpens":null,"seriesCloses":null,"seriesHighs":null,"seriesLows":null}}`,
		"keys absent": `{"smartTakeLoss":{"quietDeclineFillFrom":1640563200000}}`,
	} {
		var prediction SophosPrediction
		if err := json.Unmarshal([]byte(raw), &prediction); err != nil {
			t.Fatalf("%s: unmarshal: %v", name, err)
		}
		block := prediction.Indicators().SmartTakeLoss
		if block.SeriesOpens != nil || block.SeriesCloses != nil || block.SeriesHighs != nil || block.SeriesLows != nil {
			t.Fatalf("%s must decode to no series, got %+v", name, block)
		}
	}
}

// The indecision's look-back keys decode as sophos serves them — the open time in
// ms of the newest of the last closed bars the indecision stood on, and that
// bar's reasons — and map onto the block each into its own field, beside the
// verdict's look-back, which they never touch.
func TestSophosPredictionMapsTheIndecisionLookBack(t *testing.T) {
	raw := `{"smartTakeLoss":{"quietDeclineRecentAt":1640646000000,"quietDeclineRecentFrom":1640563200000,"quietDeclineRecentReasons":["leg down 7.0% from its high close"],"indecision":false,"indecisionRecentAt":1654761600000,"indecisionRecentReasons":["indecision read on the 1h bar opening 2022-06-09 08:00 UTC, one of the last 24 closed bars","2 of 4 readings hold with a smooth ladder, 3 needed"]}}`
	var prediction SophosPrediction
	if err := json.Unmarshal([]byte(raw), &prediction); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := SmartTakeLossIndicators{
		QuietDeclineRecentAt:      1640646000000,
		QuietDeclineRecentFrom:    1640563200000,
		QuietDeclineRecentReasons: []string{"leg down 7.0% from its high close"},
		IndecisionRecentAt:        1654761600000,
		IndecisionRecentReasons:   []string{"indecision read on the 1h bar opening 2022-06-09 08:00 UTC, one of the last 24 closed bars", "2 of 4 readings hold with a smooth ladder, 3 needed"},
	}
	if got := prediction.Indicators().SmartTakeLoss; !reflect.DeepEqual(got, want) {
		t.Fatalf("the look-back's keys must map field for field, got %+v, want %+v", got, want)
	}
}

// A sophos without the look-back keys, one serving them zero and null because no
// bar stood, and one serving both null decode to no recent bar and no reasons —
// nil, not empty — so the look-back reads nothing, and leave the flag and the
// verdict's look-back as they decoded.
func TestSophosPredictionWithoutTheIndecisionLookBackReadsNothing(t *testing.T) {
	head := `{"smartTakeLoss":{"indecision":true,"quietDeclineRecentAt":1640646000000`
	want := SmartTakeLossIndicators{Indecision: true, QuietDeclineRecentAt: 1640646000000}
	for name, raw := range map[string]string{
		"keys absent":        head + `}}`,
		"keys zero and null": head + `,"indecisionRecentAt":0,"indecisionRecentReasons":null}}`,
		"both null":          head + `,"indecisionRecentAt":null,"indecisionRecentReasons":null}}`,
	} {
		var prediction SophosPrediction
		if err := json.Unmarshal([]byte(raw), &prediction); err != nil {
			t.Fatalf("%s: unmarshal: %v", name, err)
		}
		if got := prediction.Indicators().SmartTakeLoss; !reflect.DeepEqual(got, want) {
			t.Errorf("%s must decode to no recent bar and no reasons, got %+v, want %+v", name, got, want)
		}
	}
}

// lowInterestWire is the low interest gate's part of a sophos smartTakeLoss
// object: the five bar arrays, each with numbers of its own so a key mapped into
// another's field shows, and the Trend Direction row's three counts, each of its
// own value too. lowInterestBlock is the block it must decode to.
const lowInterestWire = `"flatMarketBarOpens":[1633680000000,1633683600000,1633687200000],"flatMarketOpenPrices":[168.2,167.9,167.4],"flatMarketHighs":[168.9,168.3,167.8],"flatMarketLows":[167.5,167.1,166.9],"flatMarketCloses":[167.9,167.4,167.2],"smcTrendDown":1,"smcTrendUp":2,"smcTrendValid":3`

var lowInterestBlock = SmartTakeLossIndicators{
	SmcTrendDown:         1,
	SmcTrendUp:           2,
	SmcTrendValid:        3,
	FlatMarketBarOpens:   []int64{1633680000000, 1633683600000, 1633687200000},
	FlatMarketOpenPrices: []float64{168.2, 167.9, 167.4},
	FlatMarketHighs:      []float64{168.9, 168.3, 167.8},
	FlatMarketLows:       []float64{167.5, 167.1, 166.9},
	FlatMarketCloses:     []float64{167.9, 167.4, 167.2},
}

// The low interest gate's five keys decode as the parallel arrays sophos serves —
// the open time in ms of each closed 1h bar, then its open, high, low and close,
// oldest first — and map onto the block each into its own field, the Trend
// Direction row's Up count beside the Down and Valid counts it never touches.
func TestSophosPredictionMapsTheLowInterestBarsAndTheUpCount(t *testing.T) {
	var prediction SophosPrediction
	if err := json.Unmarshal([]byte(`{"smartTakeLoss":{`+lowInterestWire+`}}`), &prediction); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := prediction.Indicators().SmartTakeLoss; !reflect.DeepEqual(got, lowInterestBlock) {
		t.Fatalf("the bars and the Up count must map field for field, got %+v, want %+v", got, lowInterestBlock)
	}
}

// A sophos that serves the keys null — the window was not read — and one that
// does not serve them at all (a sophos from before the gate, or a cached entry
// one wrote) both decode to no bars and no Up count: the arrays stay nil, not
// empty, so the zero block comparison of every older reader still holds and the
// gate has no bars to read; what else the block carries is left as it decoded.
func TestSophosPredictionWithoutTheLowInterestBarsIsInert(t *testing.T) {
	for name, tc := range map[string]struct {
		raw  string
		want SmartTakeLossIndicators
	}{
		"keys null": {
			`{"smartTakeLoss":{"flatMarketBarOpens":null,"flatMarketOpenPrices":null,"flatMarketHighs":null,"flatMarketLows":null,"flatMarketCloses":null,"smcTrendUp":0}}`,
			SmartTakeLossIndicators{},
		},
		"keys absent": {
			`{"smartTakeLoss":{"quietDeclineFillFrom":1640563200000,"smcTrendDown":2,"smcTrendValid":3}}`,
			SmartTakeLossIndicators{QuietDeclineFillFrom: 1640563200000, SmcTrendDown: 2, SmcTrendValid: 3},
		},
	} {
		var prediction SophosPrediction
		if err := json.Unmarshal([]byte(tc.raw), &prediction); err != nil {
			t.Fatalf("%s: unmarshal: %v", name, err)
		}
		if got := prediction.Indicators().SmartTakeLoss; !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s must decode to no bars and no Up count, got %+v, want %+v", name, got, tc.want)
		}
	}
}

// The bars and the Up count ride the patterns leg through the merge as the rest
// of the block does: the gate reads them from the merged block, whether or not an
// ML leg is merged over it.
func TestMergeSophosVerdictsKeepsTheLowInterestBars(t *testing.T) {
	var decoded SophosPrediction
	if err := json.Unmarshal([]byte(`{"action":"LONG","smartTakeLoss":{`+lowInterestWire+`}}`), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	pattern := decoded.Indicators()
	ml := AIIndicators{AIAction: "SHORT", AIMarketBearish: true}
	for _, hasML := range []bool{false, true} {
		params := StrategyParams{SmartTakeLoss: true, UseAI: hasML}
		if got := MergeSophosVerdicts(params, pattern, ml, true, hasML).SmartTakeLoss; !reflect.DeepEqual(got, lowInterestBlock) {
			t.Errorf("ML leg merged %v: the bars and the Up count must survive the merge, got %+v, want %+v", hasML, got, lowInterestBlock)
		}
	}
}
