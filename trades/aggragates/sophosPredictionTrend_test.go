package aggragates

import (
	"encoding/json"
	"reflect"
	"testing"
)

// The swing decline's Trend Direction keys decode as the counts sophos serves — the rows reading
// down and the rows read — and the dashboard's rows as four parallel arrays, each with numbers of
// its own so a key mapped into another's field shows, and map onto the block untouched, beside the series.
func TestSophosPredictionMapsTheSwingDeclineSmcTrend(t *testing.T) {
	raw := `{"smartTakeLoss":{"seriesOpens":[1633680000000],"seriesCloses":[168.2],"smcTrendDown":2,"smcTrendValid":3,` +
		`"smcTrendTimeframes":["1h","4h","1D"],"smcTrendRowDown":[7,8,3],"smcTrendRowUp":[1,0,5],"smcTrendRowReads":[9,9,9]}}`
	var prediction SophosPrediction
	if err := json.Unmarshal([]byte(raw), &prediction); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := SmartTakeLossIndicators{
		SeriesOpens:        []int64{1633680000000},
		SeriesCloses:       []float64{168.2},
		SmcTrendDown:       2,
		SmcTrendValid:      3,
		SmcTrendTimeframes: []string{"1h", "4h", "1D"},
		SmcTrendRowDown:    []int64{7, 8, 3},
		SmcTrendRowUp:      []int64{1, 0, 5},
		SmcTrendRowReads:   []int64{9, 9, 9},
	}
	if got := prediction.Indicators().SmartTakeLoss; !reflect.DeepEqual(got, want) {
		t.Fatalf("the trend counts and rows must map as they decode, got %+v, want %+v", got, want)
	}
}

// A sophos without the Trend Direction keys, and one serving them zero because its dashboard could
// not be read, both decode to no valid row — the trend not read, which the swing decline never
// passes — with no dashboard rows (absent and null keys alike decode to nil), and leave the series
// as it decoded.
func TestSophosPredictionWithoutTheSwingDeclineSmcTrendIsNotRead(t *testing.T) {
	for name, raw := range map[string]string{
		"keys absent": `{"smartTakeLoss":{"seriesOpens":[1633680000000],"seriesCloses":[168.2]}}`,
		"keys zero":   `{"smartTakeLoss":{"seriesOpens":[1633680000000],"seriesCloses":[168.2],"smcTrendDown":0,"smcTrendValid":0}}`,
		"rows null":   `{"smartTakeLoss":{"seriesOpens":[1633680000000],"seriesCloses":[168.2],"smcTrendTimeframes":null,"smcTrendRowDown":null,"smcTrendRowUp":null,"smcTrendRowReads":null}}`,
	} {
		var prediction SophosPrediction
		if err := json.Unmarshal([]byte(raw), &prediction); err != nil {
			t.Fatalf("%s: unmarshal: %v", name, err)
		}
		block := prediction.Indicators().SmartTakeLoss
		if block.SmcTrendDown != 0 || block.SmcTrendValid != 0 || len(block.SeriesOpens) != 1 {
			t.Fatalf("%s must decode to no trend read beside the series, got %+v", name, block)
		}
		if block.SmcTrendTimeframes != nil || block.SmcTrendRowDown != nil || block.SmcTrendRowUp != nil || block.SmcTrendRowReads != nil {
			t.Fatalf("%s must decode to no dashboard rows, got %+v", name, block)
		}
	}
}
