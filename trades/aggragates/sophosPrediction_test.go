package aggragates

import (
	"encoding/json"
	"testing"
)

func TestSophosPredictionIndicatorsMapsVerdict(t *testing.T) {
	raw := []byte(`{
		"action":"HOLD",
		"marketBearish":true,
		"marketBullish":false,
		"signalStrength":{"overall":42},
		"stayOutReasons":["low"],
		"hasRegimeVerdict":true,
		"enterAllowed":true,
		"addAllowed":false,
		"unknownFutureKey":true,
		"regime":"downtrend-persist",
		"regimes":{"15m":"flat","4h":"downtrend-persist"},
		"regimeConfidence":0.8,
		"crashActive":true,
		"crashScore":91,
		"crashReasons":["breadth"],
		"hasContinuationVerdict":true,
		"downContinuationRisk":77,
		"upContinuationRisk":12,
		"reversalUpEvidence":5,
		"reversalDownEvidence":40,
		"dailyNatrPct":1.2,
		"continuationReasons":["structure"],
		"patternVerdict":{"name":"asc_triangle","displayName":"ascending triangle","direction":"long","score":71,"level":96000,"levelKind":"resistance","stopLoss":94000,"takeProfit":104500,"interval":"15m"},
		"fib":{"swingLow":100,"swingHigh":110,"levels":[106.18,105,103.82,102.14]}
	}`)

	var prediction SophosPrediction
	if err := json.Unmarshal(raw, &prediction); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	mapped := prediction.Indicators()
	if !mapped.CrashActive || mapped.DownContinuationRisk != 77 {
		t.Fatalf("crash and continuation must map, got %+v", mapped)
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
	if err := json.Unmarshal([]byte(`{"action":"LONG","hasRegimeVerdict":true}`), &prediction); err != nil {
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

func TestMergeSophosVerdictsPatternsThenML(t *testing.T) {
	params := StrategyParams{UseAI: true, UsePatterns: true}
	pattern := AIIndicators{
		AIAction:         "SHORT",
		HasRegimeVerdict: true,
		CrashActive:      true,
		Regimes:          map[string]string{"4h": "downtrend-persist"},
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
	if !got.CrashActive || got.Regimes["4h"] != "downtrend-persist" {
		t.Fatalf("regime/crash stay on the patterns leg, got %+v", got)
	}
	if got.PatternName != "desc_triangle" || got.PatternDirection != "short" || got.PatternScore != 66 || len(got.FibLevels) != 2 {
		t.Fatalf("the pattern verdict must survive the ML merge, got %+v", got)
	}
}

func TestMergeSophosVerdictsMLOnlyCopiesRegime(t *testing.T) {
	params := StrategyParams{UseAI: true}
	ml := AIIndicators{
		AIAction:             "HOLD",
		HasRegimeVerdict:     true,
		CrashActive:          true,
		DownContinuationRisk: 70,
	}
	got := MergeSophosVerdicts(params, AIIndicators{}, ml, false, true)
	if got.AIAction != "HOLD" || !got.CrashActive || got.DownContinuationRisk != 70 {
		t.Fatalf("ML-only must keep attached crash/continuation, got %+v", got)
	}
}

// Data flows, the flag owns the gate: the pattern fields are carried even
// when UsePatterns is off, exactly like crash and continuation.
func TestMergeSophosVerdictsKeepsPatternFieldsWhenUsePatternsOff(t *testing.T) {
	pattern := AIIndicators{AIAction: "LONG", HasRegimeVerdict: true, PatternName: "bull_flag", PatternDirection: "long", PatternScore: 80}
	got := MergeSophosVerdicts(StrategyParams{CrashGuard: true}, pattern, AIIndicators{}, true, false)
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
	if !ai.NeedsSophos() || !ai.NeedsAIRoute() || ai.NeedsPatternRoute() || !ai.InjectsEntryHold() {
		t.Fatal("UseAI must fetch the ML route and inject entry hold")
	}

	patterns := StrategyParams{UsePatterns: true}
	if !patterns.NeedsSophos() || !patterns.NeedsPatternRoute() || patterns.NeedsAIRoute() || !patterns.InjectsEntryHold() {
		t.Fatal("UsePatterns must fetch /patterns and inject entry hold")
	}

	crash := StrategyParams{CrashGuard: true}
	if !crash.NeedsSophos() || !crash.NeedsPatternRoute() {
		t.Fatal("CrashGuard must fetch /patterns")
	}
	if crash.InjectsEntryHold() {
		t.Fatal("CrashGuard alone must not inject first-buy shouldHold")
	}

	stl := StrategyParams{SmartTakeLoss: true}
	if !stl.NeedsSophos() || stl.InjectsEntryHold() {
		t.Fatal("SmartTakeLoss fetches sophos and does not inject entry hold")
	}

	cool := StrategyParams{Cooldown: true}
	if cool.NeedsSophos() {
		t.Fatal("Cooldown uses markers, not sophos")
	}
	if !cool.InjectsEntryHold() {
		t.Fatal("Cooldown must inject first-buy shouldHold")
	}

	// The regime block rides /patterns: RegimeHold fetches it and gates
	// only adds and exits.
	regime := StrategyParams{RegimeHold: true}
	if !regime.NeedsSophos() || !regime.NeedsPatternRoute() || regime.NeedsAIRoute() {
		t.Fatal("RegimeHold must fetch /patterns and nothing else")
	}
	if regime.InjectsEntryHold() {
		t.Fatal("RegimeHold never gates the first buy")
	}

	reserved := StrategyParams{PowerLawQuantiles: true}
	if reserved.NeedsSophos() || reserved.NeedsPatternRoute() || reserved.NeedsAIRoute() || reserved.InjectsEntryHold() {
		t.Fatal("PowerLawQuantiles is reserved: it fetches nothing and gates nothing")
	}
}
