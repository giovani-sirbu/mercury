package flatmarket

import (
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// rangeOnlyBars is a window of Window bars whose newest RangeBars are the range's holding bars and
// whose older bars carry bodies far over the line's bound: the range holds, the line misses.
func rangeOnlyBars(side string) Bars {
	tail := holding(side)
	lead := Window - tail.Len()
	bars := Bars{}
	for index := range lead {
		bars.Opens = append(bars.Opens, tail.Opens[0]-int64(lead-index)*series.BarMs)
		bars.OpenPrices = append(bars.OpenPrices, basePrice)
		bars.Highs = append(bars.Highs, basePrice*1.2)
		bars.Lows = append(bars.Lows, basePrice*0.8)
		bars.Closes = append(bars.Closes, basePrice*1.1)
	}
	bars.Opens = append(bars.Opens, tail.Opens...)
	bars.OpenPrices = append(bars.OpenPrices, tail.OpenPrices...)
	bars.Highs = append(bars.Highs, tail.Highs...)
	bars.Lows = append(bars.Lows, tail.Lows...)
	bars.Closes = append(bars.Closes, tail.Closes...)
	return bars
}

// The gate holds on either reading, the range named first when both hold, and a miss names the
// breaks of both; the range reads the newest RangeBars of the window, the line all of it.
func TestTriggerHoldsOnEitherReadingTheRangeNamingItFirst(t *testing.T) {
	side := aggragates.SideLong
	trend := lineMetTrend(side)
	for name, tc := range map[string]struct {
		bars    Bars
		holds   bool
		opening string
	}{
		"the range alone, the window's older bars loud": {rangeOnlyBars(side), true, "range on "},
		"the line alone, bars without wicks":            {flatLineBars(), true, "line on "},
		"both, a quiet window with wicks":               {holding(side), true, "range on "},
		"neither":                                       {Bars{}, false, ""},
	} {
		reading := Trigger(tc.bars, fixturePercentage, side, trend)
		if reading.Holds != tc.holds {
			t.Errorf("%s: holds %v, want %v (range %v, line %v)", name, reading.Holds, tc.holds, reading.Range.Holds, reading.Line.Holds)
		}
		breaks := Breaks(tc.bars, fixturePercentage, side, trend)
		if tc.holds && (len(reading.Reasons) == 0 || !strings.HasPrefix(reading.Reasons[0], tc.opening) || breaks != nil) {
			t.Errorf("%s: the reading that held opens the reasons with %q and breaks nothing, got %q / %q", name, tc.opening, reading.Reasons, breaks)
		}
		if !tc.holds && (len(reading.Reasons) != 0 || len(breaks) < 2) {
			t.Errorf("%s: a miss carries no reasons and names both readings' breaks, got %q / %q", name, reading.Reasons, breaks)
		}
	}
	only := Trigger(rangeOnlyBars(side), fixturePercentage, side, trend)
	if !only.Range.Holds || only.Line.Holds || only.Range.Of != RangeBars {
		t.Errorf("the range reads the newest %d bars of the window and the line misses on all of it, got range %+v line %+v", RangeBars, only.Range.Holds, only.Line.Holds)
	}
}

// The latch reads the range on its bars alone, no trend asked, and the line under the trend; the
// gate's own reading asks the trend of both.
func TestLatchTriggerReadsTheRangeOnItsBarsAloneAndTheLineUnderTheTrend(t *testing.T) {
	side := aggragates.SideLong
	unread := series.Trend{}
	if !LatchTrigger(holding(side), fixturePercentage, side, unread).Holds {
		t.Error("the range latches on its bars alone, no trend asked")
	}
	if LatchTrigger(flatLineBars(), fixturePercentage, side, unread).Holds {
		t.Error("the line latches under the trend only")
	}
	if !LatchTrigger(flatLineBars(), fixturePercentage, side, lineMetTrend(side)).Holds {
		t.Error("the line latches with the trend met")
	}
	if Trigger(holding(side), fixturePercentage, side, unread).Holds {
		t.Error("the gate's own reading asks the trend of the range")
	}
}
