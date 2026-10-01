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
// GET /:symbol/patterns: flat keys, always present, all zero when sophos
// read nothing. The slowDecline* keys are the quiet slow-decline exit — the
// slowDeclineRecent* keys its reading on the last closed bars sophos looks
// back over, slowDeclineFillFrom the oldest bar a ladder's newest fill must
// sit in for the ladder to go pending, and slowDeclineIndecision the
// indecision, read from the vote one short of the need or from the whole SMC
// trend table bearish, which the indecision direction reads — and the
// capitalProtection* keys the capital protection exit, all read on the
// closed window of sophos' smart take loss. slowDeclineBreakReasons may be
// non-null on a leg on and quiet, carrying the whole-table reading's reason
// alone. A sophos without the object —
// or one still serving the retired pattern-window keys of the trend-reversal
// rule, or the retired `slowDeclineMiddleBB` key — decodes to no verdict, no
// quiet leg, no smooth bar, no bar a fill has to precede, no sell band, no
// break reasons, no indecision, no recent bar, no fill window, no capital
// protection band and no bearish SMC trend, which is inert. A sophos without
// the slowDeclineRecent* keys alone decodes to no recent bar, and the recent
// path reads for no ladder; one without slowDeclineFillFrom alone decodes to
// no fill window, and while the recent-fill rule is on no ladder goes
// pending; one without slowDeclineIndecision alone decodes to no indecision,
// and the indecision direction latches no ladder.
//
// The slowPattern* keys are the closed 1h series of the slow pattern decline:
// slowPatternOpens the open time in ms of each bar and slowPatternCloses its
// close, parallel and oldest first, the last bars of the window sophos read
// (slowpattern.SlowPatternBars). Sophos never learns a ladder's fills, so it
// serves the series for every ladder and mercury cuts the window between two
// fills out of it. A sophos without them, or one that could not read the
// window, decodes to no series, and the slow pattern decline reads nothing —
// it is inert; arrays of different lengths are read as no series too
// (slowpattern.Series).
type SophosSmartTakeLoss struct {
	SlowDeclineExit             bool      `json:"slowDeclineExit"`
	SlowDeclineLegQuiet         bool      `json:"slowDeclineLegQuiet"`
	SlowDeclineSmoothFrom       int64     `json:"slowDeclineSmoothFrom"`
	SlowDeclineFillBefore       int64     `json:"slowDeclineFillBefore"`
	SlowDeclineSellBand         float64   `json:"slowDeclineSellBand"`
	SlowDeclineExitReasons      []string  `json:"slowDeclineExitReasons"`
	SlowDeclineBreakReasons     []string  `json:"slowDeclineBreakReasons"`
	SlowDeclineIndecision       bool      `json:"slowDeclineIndecision"`
	SlowDeclineRecentAt         int64     `json:"slowDeclineRecentAt"`
	SlowDeclineRecentFrom       int64     `json:"slowDeclineRecentFrom"`
	SlowDeclineRecentReasons    []string  `json:"slowDeclineRecentReasons"`
	SlowDeclineFillFrom         int64     `json:"slowDeclineFillFrom"`
	CapitalProtectionUpperBB    float64   `json:"capitalProtectionUpperBB"`
	CapitalProtectionSmcBearish bool      `json:"capitalProtectionSmcBearish"`
	SlowPatternOpens            []int64   `json:"slowPatternOpens"`
	SlowPatternCloses           []float64 `json:"slowPatternCloses"`
}

// Indicators folds the flat wire keys into the block the gate reads. The two
// slow-pattern series map as they decode: a nil one stays nil, so a sophos
// without them hands the gate no series at all.
func (s SophosSmartTakeLoss) Indicators() SmartTakeLossIndicators {
	return SmartTakeLossIndicators{
		SlowDeclineExit:             s.SlowDeclineExit,
		SlowDeclineLegQuiet:         s.SlowDeclineLegQuiet,
		SlowDeclineSmoothFrom:       s.SlowDeclineSmoothFrom,
		SlowDeclineFillBefore:       s.SlowDeclineFillBefore,
		SlowDeclineSellBand:         s.SlowDeclineSellBand,
		SlowDeclineExitReasons:      s.SlowDeclineExitReasons,
		SlowDeclineBreakReasons:     s.SlowDeclineBreakReasons,
		SlowDeclineIndecision:       s.SlowDeclineIndecision,
		SlowDeclineRecentAt:         s.SlowDeclineRecentAt,
		SlowDeclineRecentFrom:       s.SlowDeclineRecentFrom,
		SlowDeclineRecentReasons:    s.SlowDeclineRecentReasons,
		SlowDeclineFillFrom:         s.SlowDeclineFillFrom,
		CapitalProtectionUpperBB:    s.CapitalProtectionUpperBB,
		CapitalProtectionSmcBearish: s.CapitalProtectionSmcBearish,
		SlowPatternOpens:            s.SlowPatternOpens,
		SlowPatternCloses:           s.SlowPatternCloses,
	}
}

// SophosPrediction is the wire contract shared by hermes and sisyphus for
// GET /:symbol and GET /:symbol/patterns. Extra fields sophos may send are
// ignored; missing ones stay at the zero value and stay inert.
//
// Served but deliberately NOT decoded: `usePrediction` (ML route,
// informational; no reader in any engine). Neither route carries the
// DynamicParams reads: those are GET /:symbol/smc-trend's (SophosSmcTrend).
type SophosPrediction struct {
	Action         string               `json:"action"`
	MarketBearish  bool                 `json:"marketBearish"`
	MarketBullish  bool                 `json:"marketBullish"`
	SignalStrength SophosSignalStrength `json:"signalStrength"`
	StayOutReasons []string             `json:"stayOutReasons"`
	SmartTakeLoss  SophosSmartTakeLoss  `json:"smartTakeLoss"`
	PatternVerdict SophosPatternVerdict `json:"patternVerdict"`
	Fib            SophosFib            `json:"fib"`
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
