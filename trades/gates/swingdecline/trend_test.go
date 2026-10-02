package swingdecline

import (
	"reflect"
	"slices"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// holdingReading is the staircase's window from depth 1 to 4: read, and holding
// on its shape.
func holdingReading(t *testing.T) Reading {
	t.Helper()
	reading, readable := Read(staircase(4, staircaseTurns...), barOpenAt(0), barOpenAt(32), 1, 4)
	if !readable || !reading.Holds {
		t.Fatalf("fixture drifted: the staircase must be read and hold, got readable %v, %+v", readable, reading)
	}
	return reading
}

// missingReading is the flush's window from depth 1 to 5: read, and not holding
// on its shape — one leg takes more of the fall than a staircase's may.
func missingReading(t *testing.T) Reading {
	t.Helper()
	reading, readable := Read(staircase(4, flushTurns...), barOpenAt(0), barOpenAt(32), 1, 5)
	if !readable || reading.Holds || len(reading.Breaks) != 2 {
		t.Fatalf("fixture drifted: the flush must be read and miss on one reading, got readable %v, %+v", readable, reading)
	}
	return reading
}

// The trend is the two counts sophos serves as they decoded, and it is served
// only when sophos read a row: a block without the keys, or whose dashboard
// could not be read, serves none, whatever a stray count says.
func TestTrendOfMapsTheBlockAndIsServedOnRowsRead(t *testing.T) {
	for name, tc := range map[string]struct {
		block  aggragates.SmartTakeLossIndicators
		want   Trend
		served bool
	}{
		"rows read, two down":  {aggragates.SmartTakeLossIndicators{SwingDeclineSmcTrendDown: 2, SwingDeclineSmcTrendValid: 3}, Trend{Down: 2, Valid: 3}, true},
		"rows read, none down": {aggragates.SmartTakeLossIndicators{SwingDeclineSmcTrendValid: 3}, Trend{Valid: 3}, true},
		"no keys":              {aggragates.SmartTakeLossIndicators{}, Trend{}, false},
		"a count, no row read": {aggragates.SmartTakeLossIndicators{SwingDeclineSmcTrendDown: 2}, Trend{Down: 2}, false},
	} {
		if got := TrendOf(tc.block); got != tc.want || got.Served() != tc.served {
			t.Errorf("%s: got %+v served %v, want %+v served %v", name, got, got.Served(), tc.want, tc.served)
		}
	}
}

// At need zero or below the condition is off: the reading comes back as it was,
// whatever the trend says and whether it was read, no reason added.
func TestWithTrendIsOffAtZeroOrBelow(t *testing.T) {
	for _, reading := range []Reading{holdingReading(t), missingReading(t)} {
		for _, need := range []int{0, -1} {
			for _, trend := range []Trend{allDown, {Valid: 3}, {}} {
				if got := reading.withTrend(trend, need); !reflect.DeepEqual(got, reading) {
					t.Errorf("need %d, trend %+v: the reading must come back as it was, got %+v want %+v", need, trend, got, reading)
				}
			}
		}
	}
}

// A trend at the need holds — the bound included — and is named last, the frame
// and the readings before it untouched; a trend that holds never rescues a shape
// that does not.
func TestWithTrendHoldsAtTheNeedAndNamesItLast(t *testing.T) {
	shape := holdingReading(t)
	for trend, text := range map[Trend]string{
		{Down: 2, Valid: 3}: "smc trend direction down 2/3 of 2 needed",
		{Down: 3, Valid: 3}: "smc trend direction down 3/3 of 2 needed",
		{Down: 2, Valid: 2}: "smc trend direction down 2/2 of 2 needed",
	} {
		got := shape.withTrend(trend, 2)
		if want := append(slices.Clone(shape.Reasons), text); !got.Holds || got.Breaks != nil || !slices.Equal(got.Reasons, want) {
			t.Errorf("%+v: want the shape's reasons then %q, got holds %v, %q, breaks %q", trend, text, got.Holds, got.Reasons, got.Breaks)
		}
	}
	if got := shape.withTrend(Trend{Down: 1, Valid: 3}, 2); got.Holds {
		t.Errorf("one row short of the need must not hold, got %+v", got)
	}
	miss := missingReading(t)
	if got := miss.withTrend(allDown, 2); got.Holds || got.Reasons != nil || !slices.Equal(got.Breaks, miss.Breaks) {
		t.Errorf("a trend that holds leaves a shape that misses as it was, got %+v", got)
	}
}

// Whatever the need, a trend holds exactly when sophos read a row and the rows
// down reach the need: the bound included, one short of it failing, a count with
// no row read behind it never. The whole grid is read, so the comparison is
// pinned at every need and not only at the one SwingDeclineMinSmcTrendDown ships.
func TestWithTrendHoldsExactlyWhenARowIsReadAndTheDownCountReachesTheNeed(t *testing.T) {
	shape := holdingReading(t)
	for need := 1; need <= 4; need++ {
		for down := 0; down <= 3; down++ {
			for _, valid := range []int{0, down, 3} {
				trend := Trend{Down: down, Valid: valid}
				if got, want := shape.withTrend(trend, need).Holds, valid > 0 && down >= need; got != want {
					t.Errorf("need %d, trend %+v: holds %v, want %v", need, trend, got, want)
				}
			}
		}
	}
}

// A reading that held on its shape and meets no trend loses its reasons: it
// names the window and the trend in its breaks and nothing else, the readings
// of the window kept.
func TestWithTrendFailsAReadingThatHeldOnItsShape(t *testing.T) {
	shape := holdingReading(t)
	got := shape.withTrend(Trend{Down: 1, Valid: 3}, 2)
	want := []string{shape.Reasons[0], "smc trend direction down 1/3 short of 2 needed"}
	if got.Holds || got.Reasons != nil || !slices.Equal(got.Breaks, want) {
		t.Fatalf("want no hold, no reasons and the frame then the trend as breaks, got holds %v, %q, breaks %q", got.Holds, got.Reasons, got.Breaks)
	}
	if got.Weight != shape.Weight || got.StartAt != shape.StartAt || got.EndAt != shape.EndAt || got.DownLegs != shape.DownLegs {
		t.Errorf("the window's own readings are kept, got %+v want %+v", got, shape)
	}
}

// A reading that did not hold keeps the breaks that say why and names the trend
// after them, so a cancelled row says everything that missed.
func TestWithTrendFailsAReadingThatDidNotHold(t *testing.T) {
	miss := missingReading(t)
	got := miss.withTrend(Trend{Down: 0, Valid: 3}, 2)
	want := append(slices.Clone(miss.Breaks), "smc trend direction down 0/3 short of 2 needed")
	if got.Holds || got.Reasons != nil || !slices.Equal(got.Breaks, want) {
		t.Fatalf("want the shape's breaks then the trend, got holds %v, %q, breaks %q", got.Holds, got.Reasons, got.Breaks)
	}
}

// A trend sophos did not serve is named as not read, on a reading that held and
// on one that did not, and a count with no row read behind it is not read either.
func TestWithTrendNamesATrendNotRead(t *testing.T) {
	for name, trend := range map[string]Trend{"no row read": {}, "a count with no row": {Down: 3}} {
		for _, reading := range []Reading{holdingReading(t), missingReading(t)} {
			got := reading.withTrend(trend, 2)
			if got.Holds || got.Reasons != nil || got.Breaks[len(got.Breaks)-1] != "smc trend direction not read" {
				t.Errorf("%s: want no hold and the trend named as not read last, got holds %v, breaks %q", name, got.Holds, got.Breaks)
			}
		}
	}
}

// The three texts, exactly: the rows down of the rows read, against the need.
func TestTrendReasonTexts(t *testing.T) {
	for _, tc := range []struct{ got, want string }{
		{trendHoldReason(Trend{Down: 3, Valid: 3}, 2), "smc trend direction down 3/3 of 2 needed"},
		{trendBreakReason(Trend{Down: 1, Valid: 3}, 2), "smc trend direction down 1/3 short of 2 needed"},
		{trendBreakReason(Trend{}, 2), "smc trend direction not read"},
	} {
		if tc.got != tc.want {
			t.Errorf("got %q, want %q", tc.got, tc.want)
		}
	}
}

// The fold appends to copies: a reading whose reasons or breaks sit in a slice
// with room to spare never has that room written into.
func TestWithTrendNeverWritesIntoTheReadingsSlices(t *testing.T) {
	shape := holdingReading(t)
	reasons := slices.Grow(slices.Clone(shape.Reasons), 2)
	shape.Reasons = reasons
	shape.withTrend(allDown, 2)
	if spare := reasons[:len(reasons)+1][len(reasons)]; spare != "" {
		t.Errorf("the reasons' spare room was written, got %q", spare)
	}
	miss := missingReading(t)
	breaks := slices.Grow(slices.Clone(miss.Breaks), 2)
	miss.Breaks = breaks
	miss.withTrend(Trend{}, 2)
	if spare := breaks[:len(breaks)+1][len(breaks)]; spare != "" {
		t.Errorf("the breaks' spare room was written, got %q", spare)
	}
}

// The fold leaves the reading it was given as it was, in every branch: the
// reasons and the breaks of a reading that held and of one that did not are,
// element for element, what they were — a held reading that fails does not build
// its breaks in its reasons' array, nor a miss its trend in its breaks'.
func TestWithTrendLeavesTheReadingItWasGivenAsItWas(t *testing.T) {
	for _, reading := range []Reading{holdingReading(t), missingReading(t)} {
		reasons, breaks := slices.Clone(reading.Reasons), slices.Clone(reading.Breaks)
		for _, trend := range []Trend{allDown, {Down: 1, Valid: 3}, {}} {
			for _, need := range []int{0, 2} {
				reading.withTrend(trend, need)
				if !slices.Equal(reading.Reasons, reasons) || !slices.Equal(reading.Breaks, breaks) {
					t.Fatalf("need %d, trend %+v: the reading's own slices changed, reasons %q want %q, breaks %q want %q", need, trend, reading.Reasons, reasons, reading.Breaks, breaks)
				}
			}
		}
	}
}
