package aggragates

// SmartSellIndicators is the chart block sophos serves on GET /:symbol/patterns for the
// SmartSell flag, read on the closed window of sophos' smart sell; every zero field is
// inert in gates/smartsell.Apply.
type SmartSellIndicators struct {
	// QuietDeclineVerdict is sophos' quiet decline verdict; QuietDeclineLegQuiet the same for a ladder whose
	// own smoothness holds, its newest fill between QuietDeclineSmoothFrom and QuietDeclineFillBefore (ms);
	// QuietDeclineExitReasons names the reading and QuietDeclineBreakReasons what broke.
	QuietDeclineVerdict      bool
	QuietDeclineLegQuiet     bool
	QuietDeclineSmoothFrom   int64
	QuietDeclineFillBefore   int64
	QuietDeclineExitReasons  []string
	QuietDeclineBreakReasons []string
	// Indecision is sophos' indecision on the last closed bar, read from the vote one
	// short of the need, the near miss, the SMC trend tolerance or the whole table bearish; it
	// latches a watched ladder on the first tick served, naming QuietDeclineBreakReasons.
	Indecision bool
	// IndecisionRecentAt is the open time in ms of the newest of sophos' last
	// IndecisionRecentBars closed bars on which the indecision stood (zero when none), and
	// IndecisionRecentReasons that bar's reasons; a watched ladder is latched on it.
	IndecisionRecentAt      int64
	IndecisionRecentReasons []string
	// QuietDeclineRecentAt is the open time in ms of the newest of sophos' last SlowDeclineRecentBars
	// closed bars on which the verdict stood (zero when none), QuietDeclineRecentFrom the oldest of
	// those bars and QuietDeclineRecentReasons that bar's reasons; a fill at or after From reads recently.
	QuietDeclineRecentAt      int64
	QuietDeclineRecentFrom    int64
	QuietDeclineRecentReasons []string
	// QuietDeclineFillFrom is the open time in ms of the oldest of sophos' last SlowDeclineFillBars
	// closed bars, served whenever sophos read the window; under quietdecline.NeedsRecentFill a
	// ladder goes pending only with its newest fill stamped at or after it.
	QuietDeclineFillFrom int64
	// CapitalProtectionSmcBearish is whether the SMC trend dashboard reads Trend Direction down on
	// every timeframe with enough of each timeframe's reads bearish (sophos' CapitalProtectionTrendShare),
	// CapitalProtectionSmcBullish the same read up; capital protection triggers on the one against the ladder.
	CapitalProtectionSmcBearish bool
	CapitalProtectionSmcBullish bool
	// SeriesOpens and SeriesCloses are the closed 1h bars of the window sophos read, parallel and
	// oldest first, the swing and linear declines' input, and SeriesHighs and SeriesLows their
	// wicks, the swing decline's; nil or unequal arrays are no series, and no wicks.
	SeriesOpens  []int64
	SeriesCloses []float64
	SeriesHighs  []float64
	SeriesLows   []float64
	// SmcTrendDown, SmcTrendUp and SmcTrendValid are the Trend
	// Direction row of sophos' SMC trend dashboard as of the series' last bar: the rows reading
	// down, up, and read at all (zero valid is a trend not read, which never passes).
	SmcTrendDown  int
	SmcTrendUp    int
	SmcTrendValid int
	// SmcTrendTimeframes, SmcTrendRowDown, SmcTrendRowUp and
	// SmcTrendRowReads are that dashboard's rows, four parallel arrays in dashboard order
	// (1h, 4h, 1D): each row's timeframe and how many of its reads read down, up, and were read at
	// all (a row not read has no reads); nil or unequal arrays are no rows.
	SmcTrendTimeframes []string
	SmcTrendRowDown    []int64
	SmcTrendRowUp      []int64
	SmcTrendRowReads   []int64
	// SmcTrendRowFreeFall and SmcTrendRowFreeRise are, parallel to the rows, whether each row is in
	// free fall (a resistance and no support left below within its S/R look-back) or free rise (a
	// support and no resistance left above); nil from a sophos that serves neither.
	SmcTrendRowFreeFall []bool
	SmcTrendRowFreeRise []bool
	// FlatMarketBarOpens, FlatMarketOpenPrices, FlatMarketHighs, FlatMarketLows and FlatMarketCloses
	// are the last closed bars of flatmarket.Timeframe the flat market gate reads, five parallel
	// arrays oldest first (the range reads the newest flatmarket.RangeBars); nil or unequal arrays are no bars.
	FlatMarketBarOpens   []int64
	FlatMarketOpenPrices []float64
	FlatMarketHighs      []float64
	FlatMarketLows       []float64
	FlatMarketCloses     []float64
	// FreeFallOpens, FreeFallMasks and FreeRiseMasks are the last closed bars of
	// freefallprevention.Timeframe the free fall prevention gate reads, three parallel arrays oldest
	// first: each bar's open time in ms and the rows in free fall and in free rise as of its close
	// (bit r for row r of SmcTrendTimeframes); nil or unequal arrays are no series.
	FreeFallOpens []int64
	FreeFallMasks []int64
	FreeRiseMasks []int64
}
