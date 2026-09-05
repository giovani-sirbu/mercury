package aggragates

// SophosSignalStrength is the nested signalStrength object on a sophos
// prediction payload. Only Overall is mapped onto AIIndicators today.
type SophosSignalStrength struct {
	Overall         float64 `json:"overall"`
	Prediction      float64 `json:"prediction"`
	Sentiment       float64 `json:"sentiment"`
	Technical       float64 `json:"technical"`
	SentimentWeight float64 `json:"sentimentWeight"`
}

// SophosPatternVerdict is the structured chart-pattern verdict on
// GET /:symbol/patterns. Zero = no pattern detected on this bar.
type SophosPatternVerdict struct {
	Name        string  `json:"name"`
	DisplayName string  `json:"displayName"`
	Direction   string  `json:"direction"`
	Score       float64 `json:"score"`
	Level       float64 `json:"level"`
	LevelKind   string  `json:"levelKind"`
	StopLoss    float64 `json:"stopLoss"`
	TakeProfit  float64 `json:"takeProfit"`
	Interval    string  `json:"interval"`
}

// SophosFib is the fibonacci retracement of the last up-swing; Levels
// descend. Zero = no swing.
type SophosFib struct {
	SwingLow  float64   `json:"swingLow"`
	SwingHigh float64   `json:"swingHigh"`
	Levels    []float64 `json:"levels"`
}

// SophosPrediction is the wire contract shared by hermes and sisyphus for
// GET /:symbol and GET /:symbol/patterns. Extra fields sophos may send are
// ignored; missing ones stay at the zero value and stay inert.
//
// Served but deliberately NOT decoded: `usePrediction` (ML route,
// informational), `profitExitAction`, `exitPreferred` and `regimeConfidence`
// (no reader in any engine). `enterAllowed` is decoded and documented unread
// on AIIndicators.
type SophosPrediction struct {
	Action                 string               `json:"action"`
	MarketBearish          bool                 `json:"marketBearish"`
	MarketBullish          bool                 `json:"marketBullish"`
	SignalStrength         SophosSignalStrength `json:"signalStrength"`
	StayOutReasons         []string             `json:"stayOutReasons"`
	HasRegimeVerdict       bool                 `json:"hasRegimeVerdict"`
	EnterAllowed           bool                 `json:"enterAllowed"`
	AddAllowed             bool                 `json:"addAllowed"`
	Regime                 string               `json:"regime"`
	Regimes                map[string]string    `json:"regimes"`
	CrashActive            bool                 `json:"crashActive"`
	CrashScore             float64              `json:"crashScore"`
	CrashReasons           []string             `json:"crashReasons"`
	HasContinuationVerdict bool                 `json:"hasContinuationVerdict"`
	DownContinuationRisk   float64              `json:"downContinuationRisk"`
	UpContinuationRisk     float64              `json:"upContinuationRisk"`
	ReversalUpEvidence     float64              `json:"reversalUpEvidence"`
	ReversalDownEvidence   float64              `json:"reversalDownEvidence"`
	DailyNatrPct           float64              `json:"dailyNatrPct"`
	ContinuationReasons    []string             `json:"continuationReasons"`
	PatternVerdict         SophosPatternVerdict `json:"patternVerdict"`
	Fib                    SophosFib            `json:"fib"`
}

// Indicators maps a decoded sophos payload onto AIIndicators. Strategy
// flags are never applied here: fetching for one flag must not arm another
// flag's gate (see StrategyParams.NeedsSophos).
func (p SophosPrediction) Indicators() AIIndicators {
	return AIIndicators{
		AIMarketBearish:        p.MarketBearish,
		AIMarketBullish:        p.MarketBullish,
		AIAction:               p.Action,
		AISignalStrength:       p.SignalStrength.Overall,
		StayOutReasons:         p.StayOutReasons,
		HasRegimeVerdict:       p.HasRegimeVerdict,
		EnterAllowed:           p.EnterAllowed,
		AddAllowed:             p.AddAllowed,
		Regime:                 p.Regime,
		Regimes:                p.Regimes,
		CrashActive:            p.CrashActive,
		CrashScore:             p.CrashScore,
		CrashReasons:           p.CrashReasons,
		HasContinuationVerdict: p.HasContinuationVerdict,
		DownContinuationRisk:   p.DownContinuationRisk,
		UpContinuationRisk:     p.UpContinuationRisk,
		ReversalUpEvidence:     p.ReversalUpEvidence,
		ReversalDownEvidence:   p.ReversalDownEvidence,
		DailyNatrPct:           p.DailyNatrPct,
		ContinuationReasons:    p.ContinuationReasons,
		PatternName:            p.PatternVerdict.Name,
		PatternDisplayName:     p.PatternVerdict.DisplayName,
		PatternDirection:       p.PatternVerdict.Direction,
		PatternScore:           p.PatternVerdict.Score,
		PatternLevel:           p.PatternVerdict.Level,
		PatternLevelKind:       p.PatternVerdict.LevelKind,
		PatternStopLoss:        p.PatternVerdict.StopLoss,
		PatternTakeProfit:      p.PatternVerdict.TakeProfit,
		PatternInterval:        p.PatternVerdict.Interval,
		FibSwingLow:            p.Fib.SwingLow,
		FibSwingHigh:           p.Fib.SwingHigh,
		FibLevels:              p.Fib.Levels,
	}
}
