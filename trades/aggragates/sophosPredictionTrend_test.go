package aggragates

import (
	"encoding/json"
	"reflect"
	"testing"
)

// The swing decline's Trend Direction keys decode as the two counts
// sophos serves — the rows reading down and the rows read — and map onto the
// block untouched, beside the series.
func TestSophosPredictionMapsTheSwingDeclineSmcTrend(t *testing.T) {
	raw := `{"smartTakeLoss":{"swingDeclineOpens":[1633680000000],"swingDeclineCloses":[168.2],"swingDeclineSmcTrendDown":2,"swingDeclineSmcTrendValid":3}}`
	var prediction SophosPrediction
	if err := json.Unmarshal([]byte(raw), &prediction); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := SmartTakeLossIndicators{
		SwingDeclineOpens:         []int64{1633680000000},
		SwingDeclineCloses:        []float64{168.2},
		SwingDeclineSmcTrendDown:  2,
		SwingDeclineSmcTrendValid: 3,
	}
	if got := prediction.Indicators().SmartTakeLoss; !reflect.DeepEqual(got, want) {
		t.Fatalf("the trend counts must map as they decode, got %+v, want %+v", got, want)
	}
}

// A sophos without the Trend Direction keys, and one serving them zero because
// its dashboard could not be read, both decode to no valid row — the trend not
// read, which the swing decline never passes — and leave the series as it
// decoded.
func TestSophosPredictionWithoutTheSwingDeclineSmcTrendIsNotRead(t *testing.T) {
	for name, raw := range map[string]string{
		"keys absent": `{"smartTakeLoss":{"swingDeclineOpens":[1633680000000],"swingDeclineCloses":[168.2]}}`,
		"keys zero":   `{"smartTakeLoss":{"swingDeclineOpens":[1633680000000],"swingDeclineCloses":[168.2],"swingDeclineSmcTrendDown":0,"swingDeclineSmcTrendValid":0}}`,
	} {
		var prediction SophosPrediction
		if err := json.Unmarshal([]byte(raw), &prediction); err != nil {
			t.Fatalf("%s: unmarshal: %v", name, err)
		}
		block := prediction.Indicators().SmartTakeLoss
		if block.SwingDeclineSmcTrendDown != 0 || block.SwingDeclineSmcTrendValid != 0 || len(block.SwingDeclineOpens) != 1 {
			t.Fatalf("%s must decode to no trend read beside the series, got %+v", name, block)
		}
	}
}
