package aggragates

import (
	"encoding/json"
	"fmt"
	"reflect"
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
			raw:  `{"action":"LONG","smartTakeLoss":{"quietDeclineVerdict":false},"dynamicParams":{"timeframe":"1D","guppy":-1,"bmsb":-1,"valid":true}}`,
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
// in order, each with the cells the flag reads and its left zones: present
// where the row carries a zone, nil where it carries a null or no key.
func TestSophosSmcTrendDecodesTheRows(t *testing.T) {
	var trend SophosSmcTrend
	if err := json.Unmarshal([]byte(smcTrendDaily), &trend); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if trend.Interval != "1d" || len(trend.Rows) != 3 {
		t.Fatalf("want the 1d interval and its three rows, got %q with %d rows", trend.Interval, len(trend.Rows))
	}
	want := []SophosSmcTrendRow{
		{ID: "chart", Timeframe: "1D", Valid: true, Guppy: SophosSmcRead{Direction: -1}, BMSB: SophosSmcRead{Direction: 1}, LeftZones: SophosSmcLeftZones{Support: &struct{}{}}},
		{ID: "tf1", Timeframe: "W", Valid: true, Guppy: SophosSmcRead{Direction: 1}, BMSB: SophosSmcRead{Direction: -1}},
		{ID: "tf2", Timeframe: "M", Valid: true, Guppy: SophosSmcRead{Direction: 1}, BMSB: SophosSmcRead{Direction: -1}},
	}
	for index, row := range trend.Rows {
		if !reflect.DeepEqual(row, want[index]) {
			t.Errorf("row %d: %+v, want %+v", index, row, want[index])
		}
	}
}

// Zone blocks the tests hand a row's leftZones: a zone is present as an object, absent as a null or
// a missing key.
const (
	zoneResistanceOnly = `{"support": null, "resistance": {"price": 150.5, "touches": 2, "distance": 3.1}}`
	zoneSupportOnly    = `{"support": {"price": 132.4, "touches": 3, "distance": -4.2}, "resistance": null}`
	zoneBoth           = `{"support": {"price": 132.4}, "resistance": {"price": 150.5}}`
	zoneBothNull       = `{"support": null, "resistance": null}`
)

// smcRow is a dashboard row of the given id and label, read or not, carrying the leftZones JSON, left
// out of the row when empty.
func smcRow(id, timeframe string, valid bool, zones string) string {
	row := fmt.Sprintf(`{"id":%q,"timeframe":%q,"valid":%v,"guppy":{"direction":-1},"bmsb":{"direction":1}`, id, timeframe, valid)
	if zones != "" {
		row += `,"leftZones":` + zones
	}

	return row + "}"
}

// smcBody is a /smc-trend body at 1d carrying the rows in the order given.
func smcBody(rows ...string) string {
	body := `{"interval":"1d","rows":[`
	for index, row := range rows {
		if index > 0 {
			body += ","
		}
		body += row
	}

	return body + "]}"
}

// FreeFall lists the labels of the rows that are valid and carry a resistance and no support — the
// chart row and the higher timeframes alike — comma-joined in payload order, and is empty for none.
// A zone is absent as a null or a missing key. A body without a chart row is not read, so its
// rows in free fall are not listed either.
func TestSophosSmcTrendDynamicParamsListsTheRowsInFreeFall(t *testing.T) {
	chartReads := DynamicParamsIndicators{Timeframe: "1D", Guppy: -1, BMSB: 1, Valid: true}
	withFreeFall := func(labels string) DynamicParamsIndicators {
		reads := chartReads
		reads.FreeFall = labels
		return reads
	}

	cases := []struct {
		name string
		raw  string
		want DynamicParamsIndicators
	}{
		{"a resistance and no support", smcBody(smcRow("chart", "1D", true, zoneResistanceOnly)), withFreeFall("1D")},
		{"a resistance and a missing support key", smcBody(smcRow("chart", "1D", true, `{"resistance": {"price": 150.5}}`)), withFreeFall("1D")},
		{"a support and no resistance", smcBody(smcRow("chart", "1D", true, zoneSupportOnly)), chartReads},
		{"neither zone, both null", smcBody(smcRow("chart", "1D", true, zoneBothNull)), chartReads},
		{"neither zone, both keys missing", smcBody(smcRow("chart", "1D", true, `{}`)), chartReads},
		{"leftZones null", smcBody(smcRow("chart", "1D", true, `null`)), chartReads},
		{"no leftZones key", smcBody(smcRow("chart", "1D", true, "")), chartReads},
		{"both zones", smcBody(smcRow("chart", "1D", true, zoneBoth)), chartReads},
		{
			name: "a resistance and no support on a row sophos did not read",
			raw:  smcBody(smcRow("chart", "1D", false, zoneResistanceOnly)),
			want: DynamicParamsIndicators{Timeframe: "1D", Guppy: -1, BMSB: 1},
		},
		{
			name: "only a higher timeframe in free fall",
			raw:  smcBody(smcRow("chart", "1D", true, zoneSupportOnly), smcRow("tf1", "W", true, zoneResistanceOnly)),
			want: withFreeFall("W"),
		},
		{
			name: "a higher timeframe not read beside the chart row in free fall",
			raw:  smcBody(smcRow("chart", "1D", true, zoneResistanceOnly), smcRow("tf1", "W", false, zoneResistanceOnly)),
			want: withFreeFall("1D"),
		},
		{
			name: "several rows, the ones in free fall in payload order",
			raw: smcBody(
				smcRow("chart", "1D", true, zoneResistanceOnly),
				smcRow("tf1", "W", true, zoneSupportOnly),
				smcRow("tf2", "M", true, zoneResistanceOnly),
			),
			want: withFreeFall("1D,M"),
		},
		{
			name: "the chart row served after the higher ones, listed where it sits",
			raw: smcBody(
				smcRow("tf2", "M", true, zoneResistanceOnly),
				smcRow("chart", "1D", true, zoneResistanceOnly),
				smcRow("tf1", "W", true, zoneResistanceOnly),
			),
			want: withFreeFall("M,1D,W"),
		},
		{
			name: "no chart row, the higher timeframes in free fall",
			raw:  smcBody(smcRow("tf1", "W", true, zoneResistanceOnly), smcRow("tf2", "M", true, zoneResistanceOnly)),
		},
		{
			name: "no rows",
			raw:  `{"interval":"1d","rows":[]}`,
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

// A zone decodes as present whatever it carries, even an empty object, and absent only as a null or
// a missing key.
func TestSophosSmcTrendDecodesTheZonesForPresenceOnly(t *testing.T) {
	var trend SophosSmcTrend
	raw := smcBody(smcRow("chart", "1D", true, `{"support": {}, "resistance": {"unknownKey": true}}`))
	if err := json.Unmarshal([]byte(raw), &trend); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if zones := trend.Rows[0].LeftZones; zones.Support == nil || zones.Resistance == nil {
		t.Errorf("zones %+v, want both present", zones)
	}
}
