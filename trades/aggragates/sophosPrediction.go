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

// SophosSmartTakeLoss is the nested `smartTakeLoss` object on GET /:symbol/patterns: flat keys, always
// present, zero when sophos read nothing, decoded field for field into SmartTakeLossIndicators.
type SophosSmartTakeLoss struct {
	QuietDeclineVerdict         bool      `json:"quietDeclineVerdict"`
	QuietDeclineLegQuiet        bool      `json:"quietDeclineLegQuiet"`
	QuietDeclineSmoothFrom      int64     `json:"quietDeclineSmoothFrom"`
	QuietDeclineFillBefore      int64     `json:"quietDeclineFillBefore"`
	QuietDeclineExitReasons     []string  `json:"quietDeclineExitReasons"`
	QuietDeclineBreakReasons    []string  `json:"quietDeclineBreakReasons"`
	Indecision                  bool      `json:"indecision"`
	QuietDeclineRecentAt        int64     `json:"quietDeclineRecentAt"`
	QuietDeclineRecentFrom      int64     `json:"quietDeclineRecentFrom"`
	QuietDeclineRecentReasons   []string  `json:"quietDeclineRecentReasons"`
	IndecisionRecentAt          int64     `json:"indecisionRecentAt"`
	IndecisionRecentReasons     []string  `json:"indecisionRecentReasons"`
	QuietDeclineFillFrom        int64     `json:"quietDeclineFillFrom"`
	CapitalProtectionSmcBearish bool      `json:"capitalProtectionSmcBearish"`
	CapitalProtectionSmcBullish bool      `json:"capitalProtectionSmcBullish"`
	SeriesOpens                 []int64   `json:"seriesOpens"`
	SeriesCloses                []float64 `json:"seriesCloses"`
	SeriesHighs                 []float64 `json:"seriesHighs"`
	SeriesLows                  []float64 `json:"seriesLows"`
	SmcTrendDown                int       `json:"smcTrendDown"`
	SmcTrendUp                  int       `json:"smcTrendUp"`
	SmcTrendValid               int       `json:"smcTrendValid"`
	SmcTrendTimeframes          []string  `json:"smcTrendTimeframes"`
	SmcTrendRowDown             []int64   `json:"smcTrendRowDown"`
	SmcTrendRowUp               []int64   `json:"smcTrendRowUp"`
	SmcTrendRowReads            []int64   `json:"smcTrendRowReads"`
	SmcTrendRowFreeFall         []bool    `json:"smcTrendRowFreeFall"`
	SmcTrendRowFreeRise         []bool    `json:"smcTrendRowFreeRise"`
	FlatMarketBarOpens          []int64   `json:"flatMarketBarOpens"`
	FlatMarketOpenPrices        []float64 `json:"flatMarketOpenPrices"`
	FlatMarketHighs             []float64 `json:"flatMarketHighs"`
	FlatMarketLows              []float64 `json:"flatMarketLows"`
	FlatMarketCloses            []float64 `json:"flatMarketCloses"`
	FreeFallOpens               []int64   `json:"freeFallOpens"`
	FreeFallMasks               []int64   `json:"freeFallMasks"`
	FreeRiseMasks               []int64   `json:"freeRiseMasks"`
}

// Indicators folds the flat wire keys into the block the gate reads, each as it decoded.
func (s SophosSmartTakeLoss) Indicators() SmartTakeLossIndicators {
	return SmartTakeLossIndicators{
		QuietDeclineVerdict:         s.QuietDeclineVerdict,
		QuietDeclineLegQuiet:        s.QuietDeclineLegQuiet,
		QuietDeclineSmoothFrom:      s.QuietDeclineSmoothFrom,
		QuietDeclineFillBefore:      s.QuietDeclineFillBefore,
		QuietDeclineExitReasons:     s.QuietDeclineExitReasons,
		QuietDeclineBreakReasons:    s.QuietDeclineBreakReasons,
		Indecision:                  s.Indecision,
		QuietDeclineRecentAt:        s.QuietDeclineRecentAt,
		QuietDeclineRecentFrom:      s.QuietDeclineRecentFrom,
		QuietDeclineRecentReasons:   s.QuietDeclineRecentReasons,
		IndecisionRecentAt:          s.IndecisionRecentAt,
		IndecisionRecentReasons:     s.IndecisionRecentReasons,
		QuietDeclineFillFrom:        s.QuietDeclineFillFrom,
		CapitalProtectionSmcBearish: s.CapitalProtectionSmcBearish,
		CapitalProtectionSmcBullish: s.CapitalProtectionSmcBullish,
		SeriesOpens:                 s.SeriesOpens,
		SeriesCloses:                s.SeriesCloses,
		SeriesHighs:                 s.SeriesHighs,
		SeriesLows:                  s.SeriesLows,
		SmcTrendDown:                s.SmcTrendDown,
		SmcTrendUp:                  s.SmcTrendUp,
		SmcTrendValid:               s.SmcTrendValid,
		SmcTrendTimeframes:          s.SmcTrendTimeframes,
		SmcTrendRowDown:             s.SmcTrendRowDown,
		SmcTrendRowUp:               s.SmcTrendRowUp,
		SmcTrendRowReads:            s.SmcTrendRowReads,
		SmcTrendRowFreeFall:         s.SmcTrendRowFreeFall,
		SmcTrendRowFreeRise:         s.SmcTrendRowFreeRise,
		FlatMarketBarOpens:          s.FlatMarketBarOpens,
		FlatMarketOpenPrices:        s.FlatMarketOpenPrices,
		FlatMarketHighs:             s.FlatMarketHighs,
		FlatMarketLows:              s.FlatMarketLows,
		FlatMarketCloses:            s.FlatMarketCloses,
		FreeFallOpens:               s.FreeFallOpens,
		FreeFallMasks:               s.FreeFallMasks,
		FreeRiseMasks:               s.FreeRiseMasks,
	}
}

// SophosPrediction is the wire contract hermes and sisyphus share for GET /:symbol and
// GET /:symbol/patterns: extra fields are ignored, missing ones stay zero and inert. usePrediction
// is not decoded (no reader), and the DynamicParams reads are GET /:symbol/smc-trend's (SophosSmcTrend).
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
