package aggragates

// smcTrendChartRowID is the id sophos gives the chart row of its SMC trend
// dashboard: the row read on the requested interval itself. The rows of the
// higher timeframes above it are "tf1", "tf2" and on (sophos' dashboard.Row).
const smcTrendChartRowID = "chart"

// SophosSmcRead is one direction cell of a dashboard row: bullish (+1),
// neutral (0) or bearish (−1), the encoding of sophos' smctypes.Direction.
// The cell's text is not decoded.
type SophosSmcRead struct {
	Direction int `json:"direction"`
}

// SophosSmcTrendRow is one row of the dashboard GET /:symbol/smc-trend serves,
// decoded for the keys the DynamicParams flag reads: the row's id and
// timeframe label, whether sophos read it — on a row it did not, every
// direction is neutral — and its Super Guppy and Bull Market Support Band
// cells. The row's other keys are not decoded.
type SophosSmcTrendRow struct {
	ID        string        `json:"id"`
	Timeframe string        `json:"timeframe"`
	Valid     bool          `json:"valid"`
	Guppy     SophosSmcRead `json:"guppy"`
	BMSB      SophosSmcRead `json:"bmsb"`
}

// SophosSmcTrend is the engines' wire contract for sophos
// GET /:symbol/smc-trend: the chart interval the dashboard was read at and
// its rows, the chart row first. Every other key the route serves — the
// summary, the settings, the direction — is ignored, and a missing one stays
// at the zero value.
type SophosSmcTrend struct {
	Interval string              `json:"interval"`
	Rows     []SophosSmcTrendRow `json:"rows"`
}

// DynamicParams is the DynamicParams flag's two reads off the dashboard: the
// Super Guppy and Bull Market Support Band directions of the CHART row — the
// interval the engines request (dynamicparams.Interval), read on its last
// closed bar — with that row's timeframe label and whether sophos read it.
// The rows of the higher timeframes are never read. A payload without a chart
// row is not read: the zero value, which gates/dynamicparams treats as the
// configured rows.
func (trend SophosSmcTrend) DynamicParams() DynamicParamsIndicators {
	for _, row := range trend.Rows {
		if row.ID != smcTrendChartRowID {
			continue
		}

		return DynamicParamsIndicators{
			Timeframe: row.Timeframe,
			Guppy:     row.Guppy.Direction,
			BMSB:      row.BMSB.Direction,
			Valid:     row.Valid,
		}
	}

	return DynamicParamsIndicators{}
}
