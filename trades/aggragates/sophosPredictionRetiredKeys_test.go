package aggragates

import (
	"encoding/json"
	"reflect"
	"testing"
)

// The keys the block carried before the wire rename — the slow decline prefix on the quiet
// decline's and the indecision's readings, the swing decline prefix on the series and the trend,
// the low interest and non interest area bars — decode to nothing: a sophos still serving them
// leaves every gate inert, never reading one field under another's name.
func TestSophosPredictionIgnoresTheRetiredSmartTakeLossKeys(t *testing.T) {
	retired := `{"smartTakeLoss":{
		"slowDeclineExit":true,"slowDeclineLegQuiet":true,"slowDeclineSmoothFrom":1,"slowDeclineFillBefore":2,
		"slowDeclineExitReasons":["a"],"slowDeclineBreakReasons":["b"],"slowDeclineIndecision":true,
		"slowDeclineRecentAt":3,"slowDeclineRecentFrom":4,"slowDeclineRecentReasons":["c"],
		"slowDeclineIndecisionRecentAt":5,"slowDeclineIndecisionRecentReasons":["d"],"slowDeclineFillFrom":6,
		"swingDeclineOpens":[1],"swingDeclineCloses":[2],
		"swingDeclineSmcTrendDown":3,"swingDeclineSmcTrendUp":3,"swingDeclineSmcTrendValid":3,
		"swingDeclineSmcTrendTimeframes":["1h"],"swingDeclineSmcTrendRowDown":[9],"swingDeclineSmcTrendRowUp":[0],"swingDeclineSmcTrendRowReads":[9],
		"lowInterestBarOpens":[1],"lowInterestOpenPrices":[1],"lowInterestHighs":[1],"lowInterestLows":[1],"lowInterestCloses":[1],
		"nonInterestAreaBarOpens":[1],"nonInterestAreaOpenPrices":[1],"nonInterestAreaCloses":[1]
	}}`
	var prediction SophosPrediction
	if err := json.Unmarshal([]byte(retired), &prediction); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := prediction.Indicators().SmartTakeLoss; !reflect.DeepEqual(got, SmartTakeLossIndicators{}) {
		t.Fatalf("the retired keys must decode to the zero block, got %+v", got)
	}
}
