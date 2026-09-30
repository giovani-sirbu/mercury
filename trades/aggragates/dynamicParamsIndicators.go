package aggragates

// DynamicParamsIndicators is the DynamicParams flag's two reads: the chart row
// of the SMC trend dashboard sophos serves on GET /:symbol/smc-trend at
// dynamicparams.Interval, read on that interval's last closed bar
// (SophosSmcTrend.DynamicParams). Guppy is the Super Guppy read and BMSB the
// Bull Market Support Band read, each bullish (+1), neutral (0) or bearish
// (−1), the encoding of sophos' smctypes.Direction. Timeframe is the label of
// the row they were read on, and Valid whether sophos read that row at all.
//
// The zero value is "not read" — a failed fetch, a body without the chart
// row — and gates/dynamicparams treats a block that is not Valid as the
// configured rows, whatever its reads say. The engines consult it only when
// a ladder opens (dynamicparams.Opening), which turns it into the ladder's
// opened row; every later tick rebuilds the rows from that row
// (dynamicparams.RaisedSettings), never from the block. It holds nothing.
type DynamicParamsIndicators struct {
	Timeframe string
	Guppy     int
	BMSB      int
	Valid     bool
}
