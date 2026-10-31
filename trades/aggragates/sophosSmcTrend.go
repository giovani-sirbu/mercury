package aggragates

import "strings"

// smcTrendChartRowID is the id of the dashboard's chart row, the one read on the requested interval.
const smcTrendChartRowID = "chart"

// SophosSmcRead is one direction cell of a dashboard row: bullish (+1), neutral (0) or bearish (−1).
type SophosSmcRead struct {
	Direction int `json:"direction"`
}

// SophosSmcLeftZones is a row's nearest support and resistance, decoded for presence only: nil
// when sophos found none there (a JSON null or a missing key).
type SophosSmcLeftZones struct {
	Support    *struct{} `json:"support"`
	Resistance *struct{} `json:"resistance"`
}

// freeFall is sophos' free fall reading of a row's zones: a resistance and no support left.
func (zones SophosSmcLeftZones) freeFall() bool {
	return zones.Support == nil && zones.Resistance != nil
}

// SophosSmcTrendRow is one dashboard row, decoded for the keys the
// DynamicParams flag reads: id, timeframe, whether sophos read it, the
// Super Guppy and Bull Market Support Band cells, and its left zones.
type SophosSmcTrendRow struct {
	ID        string             `json:"id"`
	Timeframe string             `json:"timeframe"`
	Valid     bool               `json:"valid"`
	Guppy     SophosSmcRead      `json:"guppy"`
	BMSB      SophosSmcRead      `json:"bmsb"`
	LeftZones SophosSmcLeftZones `json:"leftZones"`
}

// SophosSmcTrend is the engines' wire contract for sophos GET /:symbol/smc-trend:
// the chart interval and the rows, chart row first; every other key is ignored.
type SophosSmcTrend struct {
	Interval string              `json:"interval"`
	Rows     []SophosSmcTrendRow `json:"rows"`
}

// DynamicParams is the DynamicParams flag's two reads off the CHART row, with
// its timeframe label and whether sophos read it, and the labels of every row
// in free fall; a payload without a chart row is the zero value, not read.
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
			FreeFall:  trend.freeFallTimeframes(),
		}
	}

	return DynamicParamsIndicators{}
}

// freeFallTimeframes is the labels of the rows in free fall, comma-joined in payload order; a row not valid is never in free fall.
func (trend SophosSmcTrend) freeFallTimeframes() string {
	var timeframes []string
	for _, row := range trend.Rows {
		if row.Valid && row.LeftZones.freeFall() {
			timeframes = append(timeframes, row.Timeframe)
		}
	}

	return strings.Join(timeframes, ",")
}
