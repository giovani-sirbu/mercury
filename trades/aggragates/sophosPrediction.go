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

// SophosSmartTakeLoss is the nested `smartTakeLoss` object on
// GET /:symbol/patterns: 15 flat keys, always present, all zero when sophos
// has no verdict. A line exists iff both of its time keys are > 0. An older
// sophos without the object — or one still serving the retired
// `lowestLow100` / `freshLow` keys of the 1h window — decodes to the zero
// value, which is inert: the gate reads no level and nothing activates.
type SophosSmartTakeLoss struct {
	HasVerdict          bool    `json:"hasVerdict"`
	LowestLow           float64 `json:"lowestLow"`
	LowWithBarsLeft     float64 `json:"lowWithBarsLeft"`
	HighestHigh         float64 `json:"highestHigh"`
	HighWithBarsLeft    float64 `json:"highWithBarsLeft"`
	UpperBB             float64 `json:"upperBB"`
	LowerBB             float64 `json:"lowerBB"`
	ResistanceFromTime  int64   `json:"resistanceFromTime"`
	ResistanceFromPrice float64 `json:"resistanceFromPrice"`
	ResistanceToTime    int64   `json:"resistanceToTime"`
	ResistanceToPrice   float64 `json:"resistanceToPrice"`
	SupportFromTime     int64   `json:"supportFromTime"`
	SupportFromPrice    float64 `json:"supportFromPrice"`
	SupportToTime       int64   `json:"supportToTime"`
	SupportToPrice      float64 `json:"supportToPrice"`
}

// Indicators folds the flat wire keys into the block the gate reads.
func (s SophosSmartTakeLoss) Indicators() SmartTakeLossIndicators {
	return SmartTakeLossIndicators{
		HasVerdict:       s.HasVerdict,
		LowestLow:        s.LowestLow,
		LowWithBarsLeft:  s.LowWithBarsLeft,
		HighestHigh:      s.HighestHigh,
		HighWithBarsLeft: s.HighWithBarsLeft,
		UpperBB:          s.UpperBB,
		LowerBB:          s.LowerBB,
		Resistance: TrendLine{
			From: TrendLineAnchor{At: s.ResistanceFromTime, Price: s.ResistanceFromPrice},
			To:   TrendLineAnchor{At: s.ResistanceToTime, Price: s.ResistanceToPrice},
		},
		Support: TrendLine{
			From: TrendLineAnchor{At: s.SupportFromTime, Price: s.SupportFromPrice},
			To:   TrendLineAnchor{At: s.SupportToTime, Price: s.SupportToPrice},
		},
	}
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
	Action           string               `json:"action"`
	MarketBearish    bool                 `json:"marketBearish"`
	MarketBullish    bool                 `json:"marketBullish"`
	SignalStrength   SophosSignalStrength `json:"signalStrength"`
	StayOutReasons   []string             `json:"stayOutReasons"`
	HasRegimeVerdict bool                 `json:"hasRegimeVerdict"`
	EnterAllowed     bool                 `json:"enterAllowed"`
	AddAllowed       bool                 `json:"addAllowed"`
	Regime           string               `json:"regime"`
	Regimes          map[string]string    `json:"regimes"`
	CrashActive      bool                 `json:"crashActive"`
	CrashScore       float64              `json:"crashScore"`
	CrashReasons     []string             `json:"crashReasons"`
	SmartTakeLoss    SophosSmartTakeLoss  `json:"smartTakeLoss"`
	PatternVerdict   SophosPatternVerdict `json:"patternVerdict"`
	Fib              SophosFib            `json:"fib"`
}

// Indicators maps a decoded sophos payload onto AIIndicators. Strategy
// flags are never applied here: fetching for one flag must not arm another
// flag's gate (see StrategyParams.NeedsSophos).
func (p SophosPrediction) Indicators() AIIndicators {
	return AIIndicators{
		AIMarketBearish:    p.MarketBearish,
		AIMarketBullish:    p.MarketBullish,
		AIAction:           p.Action,
		AISignalStrength:   p.SignalStrength.Overall,
		StayOutReasons:     p.StayOutReasons,
		HasRegimeVerdict:   p.HasRegimeVerdict,
		EnterAllowed:       p.EnterAllowed,
		AddAllowed:         p.AddAllowed,
		Regime:             p.Regime,
		Regimes:            p.Regimes,
		CrashActive:        p.CrashActive,
		CrashScore:         p.CrashScore,
		CrashReasons:       p.CrashReasons,
		SmartTakeLoss:      p.SmartTakeLoss.Indicators(),
		PatternName:        p.PatternVerdict.Name,
		PatternDisplayName: p.PatternVerdict.DisplayName,
		PatternDirection:   p.PatternVerdict.Direction,
		PatternScore:       p.PatternVerdict.Score,
		PatternLevel:       p.PatternVerdict.Level,
		PatternLevelKind:   p.PatternVerdict.LevelKind,
		PatternStopLoss:    p.PatternVerdict.StopLoss,
		PatternTakeProfit:  p.PatternVerdict.TakeProfit,
		PatternInterval:    p.PatternVerdict.Interval,
		FibSwingLow:        p.Fib.SwingLow,
		FibSwingHigh:       p.Fib.SwingHigh,
		FibLevels:          p.Fib.Levels,
	}
}
