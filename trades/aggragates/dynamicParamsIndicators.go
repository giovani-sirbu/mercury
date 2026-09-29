package aggragates

// DynamicParamsIndicators is the block sophos serves on GET /:symbol/patterns
// for the DynamicParams flag: two rows of its SMC trend dashboard read on the
// one timeframe sophos' dynamicparams.Timeframe names. Guppy is the Super
// Guppy read and BMSB the Bull Market Support Band read, each bullish (+1),
// neutral (0) or bearish (−1), the encoding of sophos' smctypes.Direction.
// Timeframe is the dashboard row they were read on, and Valid whether sophos
// read that row at all.
//
// The zero value is "not read" — an older sophos, a failed fetch, a window
// too short for the row — and gates/dynamicparams treats a block that is not
// Valid as the configured rows, whatever its reads say. The engines shape
// rows from it only through dynamicparams.RaisedSettings; it holds nothing.
type DynamicParamsIndicators struct {
	Timeframe string
	Guppy     int
	BMSB      int
	Valid     bool
}
