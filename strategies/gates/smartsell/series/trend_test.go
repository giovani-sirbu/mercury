package series

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The trend is the three counts sophos serves as they decoded, each into its own
// field, and it is served only when sophos read a row: a block without the keys,
// or whose dashboard could not be read, serves none, whatever a stray count says.
func TestTrendOfMapsTheBlockAndIsServedOnRowsRead(t *testing.T) {
	for name, tc := range map[string]struct {
		block  aggragates.SmartSellIndicators
		want   Trend
		served bool
	}{
		"rows read, two down":         {aggragates.SmartSellIndicators{SmcTrendDown: 2, SmcTrendValid: 3}, Trend{Down: 2, Valid: 3}, true},
		"rows read, none down":        {aggragates.SmartSellIndicators{SmcTrendValid: 3}, Trend{Valid: 3}, true},
		"rows read, two up":           {aggragates.SmartSellIndicators{SmcTrendUp: 2, SmcTrendValid: 3}, Trend{Up: 2, Valid: 3}, true},
		"rows read, one up, two down": {aggragates.SmartSellIndicators{SmcTrendUp: 1, SmcTrendDown: 2, SmcTrendValid: 3}, Trend{Up: 1, Down: 2, Valid: 3}, true},
		"no keys":                     {aggragates.SmartSellIndicators{}, Trend{}, false},
		"a count, no row read":        {aggragates.SmartSellIndicators{SmcTrendDown: 2}, Trend{Down: 2}, false},
		"an up count, no row read":    {aggragates.SmartSellIndicators{SmcTrendUp: 2}, Trend{Up: 2}, false},
	} {
		if got := TrendOf(tc.block); got != tc.want || got.Served() != tc.served {
			t.Errorf("%s: got %+v served %v, want %+v served %v", name, got, got.Served(), tc.want, tc.served)
		}
	}
}

// A trend meets a need on the side's own rows: Down for a long ladder, Up for an inverse one,
// never for a side with no way; a need at or under zero is off but still asks for a side, and a
// trend not served meets nothing that is asked.
func TestMeetsReadsTheSidesRowsAgainstTheNeed(t *testing.T) {
	trend := Trend{Up: 1, Down: 2, Valid: 3}
	for name, tc := range map[string]struct {
		trend Trend
		side  string
		need  int
		want  bool
	}{
		"long, enough rows down":   {trend, aggragates.SideLong, 2, true},
		"long, one short":          {trend, aggragates.SideLong, 3, false},
		"inverse reads the up":     {trend, aggragates.SideShort, 1, true},
		"inverse, one short":       {trend, aggragates.SideShort, 2, false},
		"need off, long":           {Trend{}, aggragates.SideLong, 0, true},
		"need off, inverse":        {Trend{}, aggragates.SideShort, -1, true},
		"need off, no side":        {trend, "", 0, false},
		"no side":                  {trend, "sideways", 1, false},
		"not served, long":         {Trend{Down: 3}, aggragates.SideLong, 1, false},
		"not served, inverse":      {Trend{Up: 3}, aggragates.SideShort, 1, false},
		"rows of the side, long":   {trend, aggragates.SideLong, 2, trend.Rows(aggragates.SideLong) >= 2},
		"rows of the side, no way": {trend, "", 1, trend.Rows("") >= 1 && false},
	} {
		if got := tc.trend.Meets(tc.side, tc.need); got != tc.want {
			t.Errorf("%s: Meets(%q, %d) = %v, want %v", name, tc.side, tc.need, got, tc.want)
		}
	}
	if !Sided(aggragates.SideLong) || !Sided(aggragates.SideShort) || Sided("") || Sided("sideways") {
		t.Error("only the long and the short side have a way on the dashboard")
	}
}
