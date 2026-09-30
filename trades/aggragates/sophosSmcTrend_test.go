package aggragates

import (
	"encoding/json"
	"testing"
)

// smcTrendDaily is a GET /:symbol/smc-trend body read at 1d the way sophos
// serves it: the chart row, then the ladder's two higher timeframes, each with
// every cell the dashboard carries, then the summary, the settings and the
// verdict. The higher rows read the opposite of the chart row on both cells
// the flag reads, so a decode that reads any row but the chart row shows.
const smcTrendDaily = `{
	"symbol": "SOLUSDT",
	"interval": "1d",
	"rows": [
		{
			"id": "chart", "timeframe": "1D", "live": true, "valid": true, "bars": 500,
			"structure": {"bias": -1, "lastEvent": "CHoCH", "text": "▼ CHoCH"},
			"leftSR": {"direction": -1, "text": "▼"},
			"bmsb": {"direction": 1, "text": "▲"},
			"guppy": {"direction": -1, "text": "▼"},
			"bollinger": {"direction": 0, "text": "—"},
			"supertrend": {"direction": -1, "text": "▼"},
			"adx": {"direction": -1, "text": "▼"},
			"srHorizontal": {"direction": -1, "text": "▼"},
			"srDiagonal": {"direction": 0, "text": "—"},
			"leftZones": {"support": {"price": 132.4, "touches": 3, "distance": -4.2}, "resistance": null, "supportHeld": 1, "supportTests": 2, "resistanceHeld": 0, "resistanceTests": 0},
			"trend": {"direction": -1, "up": 1, "down": 5, "agree": 5, "reads": 8, "strongAt": 6, "text": "DOWN 5/8", "tone": "bear", "strong": false}
		},
		{
			"id": "tf1", "timeframe": "W", "live": false, "valid": true, "bars": 260,
			"bmsb": {"direction": -1, "text": "▼"},
			"guppy": {"direction": 1, "text": "▲"},
			"trend": {"direction": 1, "up": 5, "down": 2, "agree": 5, "reads": 8, "text": "UP 5/8"}
		},
		{
			"id": "tf2", "timeframe": "M", "live": false, "valid": true, "bars": 60,
			"bmsb": {"direction": -1, "text": "▼"},
			"guppy": {"direction": 1, "text": "▲"},
			"trend": {"direction": 1, "up": 6, "down": 1, "agree": 6, "reads": 8, "text": "UP 6/8"}
		}
	],
	"summary": {"text": "UP 2/3", "direction": 1, "up": 2, "down": 1, "valid": 3, "tone": "bull", "unanimous": false},
	"settings": {"bmsb": {"sma": 20, "ema": 21, "timeframe": "W"}, "guppy": {"traders": [3, 5, 8, 10, 12, 15], "investors": [30, 35, 40, 45, 50, 60]}},
	"structureLength": 5,
	"direction": 1
}`

// The reads are the chart row's, found by its id wherever it sits among the
// rows, with its label and whether sophos read it; a body without a chart row
// — no rows, only the higher timeframes, another route's body — is not read.
// Keys the contract does not name, at any level, are ignored.
func TestSophosSmcTrendDynamicParamsReadsTheChartRow(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want DynamicParamsIndicators
	}{
		{
			name: "the three rows the route serves at 1d",
			raw:  smcTrendDaily,
			want: DynamicParamsIndicators{Timeframe: "1D", Guppy: -1, BMSB: 1, Valid: true},
		},
		{
			name: "the chart row served after a higher one",
			raw:  `{"interval":"1d","rows":[{"id":"tf1","timeframe":"W","valid":true,"guppy":{"direction":1},"bmsb":{"direction":1}},{"id":"chart","timeframe":"1D","valid":true,"guppy":{"direction":-1},"bmsb":{"direction":-1}}]}`,
			want: DynamicParamsIndicators{Timeframe: "1D", Guppy: -1, BMSB: -1, Valid: true},
		},
		{
			name: "a chart row sophos did not read",
			raw:  `{"interval":"1d","rows":[{"id":"chart","timeframe":"1D","valid":false,"guppy":{"direction":0,"text":"—"},"bmsb":{"direction":0,"text":"—"}}]}`,
			want: DynamicParamsIndicators{Timeframe: "1D"},
		},
		{
			name: "keys the contract does not name",
			raw:  `{"interval":"1d","unknownFutureKey":{"a":1},"rows":[{"id":"chart","timeframe":"1D","valid":true,"newCell":{"direction":1},"guppy":{"direction":0,"text":"—","extra":true},"bmsb":{"direction":-1}}]}`,
			want: DynamicParamsIndicators{Timeframe: "1D", Guppy: 0, BMSB: -1, Valid: true},
		},
		{
			name: "only the higher timeframes",
			raw:  `{"interval":"1d","rows":[{"id":"tf1","timeframe":"W","valid":true,"guppy":{"direction":-1},"bmsb":{"direction":-1}},{"id":"tf2","timeframe":"M","valid":true,"guppy":{"direction":-1},"bmsb":{"direction":-1}}]}`,
		},
		{
			name: "no rows",
			raw:  `{"interval":"1d","rows":[]}`,
		},
		{
			name: "a /patterns body",
			raw:  `{"action":"LONG","smartTakeLoss":{"slowDeclineExit":false},"dynamicParams":{"timeframe":"1D","guppy":-1,"bmsb":-1,"valid":true}}`,
		},
	}
	for _, tc := range cases {
		var trend SophosSmcTrend
		if err := json.Unmarshal([]byte(tc.raw), &trend); err != nil {
			t.Fatalf("%s: unmarshal: %v", tc.name, err)
		}
		if got := trend.DynamicParams(); got != tc.want {
			t.Errorf("%s: reads %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

// The body decodes to the interval it was read at and every row it serves,
// in order, each with the cells the flag reads.
func TestSophosSmcTrendDecodesTheRows(t *testing.T) {
	var trend SophosSmcTrend
	if err := json.Unmarshal([]byte(smcTrendDaily), &trend); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if trend.Interval != "1d" || len(trend.Rows) != 3 {
		t.Fatalf("want the 1d interval and its three rows, got %q with %d rows", trend.Interval, len(trend.Rows))
	}
	want := []SophosSmcTrendRow{
		{ID: "chart", Timeframe: "1D", Valid: true, Guppy: SophosSmcRead{Direction: -1}, BMSB: SophosSmcRead{Direction: 1}},
		{ID: "tf1", Timeframe: "W", Valid: true, Guppy: SophosSmcRead{Direction: 1}, BMSB: SophosSmcRead{Direction: -1}},
		{ID: "tf2", Timeframe: "M", Valid: true, Guppy: SophosSmcRead{Direction: 1}, BMSB: SophosSmcRead{Direction: -1}},
	}
	for index, row := range trend.Rows {
		if row != want[index] {
			t.Errorf("row %d: %+v, want %+v", index, row, want[index])
		}
	}
}
