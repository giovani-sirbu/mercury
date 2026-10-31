package aggragates

// DynamicParamsIndicators is the DynamicParams flag's two reads off the chart
// row of sophos' SMC trend dashboard (SophosSmcTrend.DynamicParams): Guppy the
// Super Guppy and BMSB the Bull Market Support Band direction, each bullish
// (+1), neutral (0) or bearish (−1), with the row's timeframe label and
// whether sophos read it. The zero value is "not read" and opens a ladder on
// its configured rows; the engines consult the block only when a ladder opens.
type DynamicParamsIndicators struct {
	Timeframe string
	Guppy     int
	BMSB      int
	Valid     bool
	// FreeFall is the labels of the rows in free fall, comma-joined in payload order ("" for none) so the block stays comparable (the engines compare it with ==/!= and JSON-cache it).
	FreeFall string
}
