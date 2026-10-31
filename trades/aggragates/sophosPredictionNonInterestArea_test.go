package aggragates

import (
	"encoding/json"
	"reflect"
	"testing"
)

// The non interest area's two keys decode as the parallel arrays sophos serves — the open time in
// ms and the close of each closed bar, oldest first — and map onto the block untouched.
func TestSophosPredictionMapsTheNonInterestAreaBars(t *testing.T) {
	raw := `{"smartSell":{"flatMarketBarOpens":[1633680000000,1633683600000,1633687200000],"flatMarketOpenPrices":[168.4,168.2,167.9],"flatMarketCloses":[168.2,167.9,167.4]}}`
	var prediction SophosPrediction
	if err := json.Unmarshal([]byte(raw), &prediction); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := SmartSellIndicators{
		FlatMarketBarOpens:   []int64{1633680000000, 1633683600000, 1633687200000},
		FlatMarketOpenPrices: []float64{168.4, 168.2, 167.9},
		FlatMarketCloses:     []float64{168.2, 167.9, 167.4},
	}
	if got := prediction.Indicators().SmartSell; !reflect.DeepEqual(got, want) {
		t.Fatalf("the bars must map as they decode, got %+v, want %+v", got, want)
	}
}

// A sophos that serves the keys null and one that does not serve them at all both decode to no
// bars: the arrays stay nil, so the zero block comparison of every older reader still holds.
func TestSophosPredictionWithoutTheNonInterestAreaBarsIsInert(t *testing.T) {
	for name, raw := range map[string]string{
		"keys null":   `{"smartSell":{"flatMarketBarOpens":null,"flatMarketOpenPrices":null,"flatMarketCloses":null}}`,
		"keys absent": `{"smartSell":{"quietDeclineFillFrom":1640563200000}}`,
	} {
		var prediction SophosPrediction
		if err := json.Unmarshal([]byte(raw), &prediction); err != nil {
			t.Fatalf("%s: unmarshal: %v", name, err)
		}
		block := prediction.Indicators().SmartSell
		if block.FlatMarketBarOpens != nil || block.FlatMarketOpenPrices != nil || block.FlatMarketCloses != nil {
			t.Fatalf("%s must decode to no bars, got %+v", name, block)
		}
	}
}
