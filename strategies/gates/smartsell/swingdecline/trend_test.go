package swingdecline

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
	"reflect"
	"slices"
	"testing"
)

// holdingReading is the staircase's window from depth 1 to 4: read, and holding
// on its shape.
func holdingReading(t *testing.T) Reading {
	t.Helper()
	reading, readable := Read(staircase(legBars, staircaseTurns...), barOpenAt(0), barOpenAt(lastTurn), 1, 4)
	if !readable || !reading.Holds {
		t.Fatalf("fixture drifted: the staircase must be read and hold, got readable %v, %+v", readable, reading)
	}
	return reading
}

// missingReading is the rally's window from depth 1 to 5: read, and not holding
// on its shape — every bounce rises past the bound, so no up leg counts.
func missingReading(t *testing.T) Reading {
	t.Helper()
	reading, readable := Read(staircase(legBars, rallyTurns...), barOpenAt(0), barOpenAt((len(rallyTurns)-1)*legBars), 1, 5)
	if !readable || reading.Holds || len(reading.Breaks) != 3 {
		t.Fatalf("fixture drifted: the rally must be read and miss on its up legs alone, got readable %v, %+v", readable, reading)
	}
	return reading
}

// The Up count rides the trend for the low interest gate's inverse mirror and
// moves nothing the declines read: whether a trend is served, meets a need and
// folds into a reading is a function of Down and Valid alone, whatever Up says
// beside them, at every need from the condition off to one past the dashboard.
func TestTrendUpMovesNothingTheDeclinesRead(t *testing.T) {
	shape := holdingReading(t)
	for need := 0; need <= dashboardRows+1; need++ {
		for down := 0; down <= dashboardRows; down++ {
			for _, valid := range []int{0, down, dashboardRows} {
				plain := series.Trend{Down: down, Valid: valid}
				for up := 1; up <= valid-down; up++ {
					trend := series.Trend{Up: up, Down: down, Valid: valid}
					if trend.Served() != plain.Served() || meets(trend, need) != meets(plain, need) {
						t.Errorf("need %d, trend %+v: served or met differently from %+v", need, trend, plain)
					}
					if got, want := shape.withTrend(trend, need), shape.withTrend(plain, need); !reflect.DeepEqual(got, want) {
						t.Errorf("need %d, trend %+v: the fold read Up, got %+v want %+v", need, trend, got, want)
					}
				}
			}
		}
	}
	allUp := series.Trend{Up: dashboardRows, Valid: dashboardRows}
	if got, want := MeetsTheNeed(allUp), MinSmcTrendDown <= 0; got != want {
		t.Errorf("a trend all up and none down meets the need only while the condition is off: got %v, want %v", got, want)
	}
	if (series.Trend{Up: dashboardRows}).Served() {
		t.Error("an Up count with no row read behind it must not serve the trend")
	}
}

// At need zero or below the condition is off: the reading comes back as it was,
// whatever the trend says and whether it was read, no reason added.
func TestWithTrendIsOffAtZeroOrBelow(t *testing.T) {
	for _, reading := range []Reading{holdingReading(t), missingReading(t)} {
		for _, need := range []int{0, -1} {
			for _, trend := range []series.Trend{allDown, {Valid: 3}, {}} {
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
	for trend, text := range map[series.Trend]string{
		{Down: 2, Valid: 3}: "smc trend direction down 2/3 of 2 needed",
		{Down: 3, Valid: 3}: "smc trend direction down 3/3 of 2 needed",
		{Down: 2, Valid: 2}: "smc trend direction down 2/2 of 2 needed",
	} {
		got := shape.withTrend(trend, 2)
		if want := append(slices.Clone(shape.Reasons), text); !got.Holds || got.Breaks != nil || !slices.Equal(got.Reasons, want) {
			t.Errorf("%+v: want the shape's reasons then %q, got holds %v, %q, breaks %q", trend, text, got.Holds, got.Reasons, got.Breaks)
		}
	}
	if got := shape.withTrend(series.Trend{Down: 1, Valid: 3}, 2); got.Holds {
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
// pinned at every need and not only at the one MinSmcTrendDown ships.
func TestWithTrendHoldsExactlyWhenARowIsReadAndTheDownCountReachesTheNeed(t *testing.T) {
	shape := holdingReading(t)
	for need := 1; need <= 4; need++ {
		for down := 0; down <= 3; down++ {
			for _, valid := range []int{0, down, 3} {
				trend := series.Trend{Down: down, Valid: valid}
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
	got := shape.withTrend(series.Trend{Down: 1, Valid: 3}, 2)
	want := []string{shape.Reasons[0], "smc trend direction down 1/3 short of 2 needed"}
	if got.Holds || got.Reasons != nil || !slices.Equal(got.Breaks, want) {
		t.Fatalf("want no hold, no reasons and the frame then the trend as breaks, got holds %v, %q, breaks %q", got.Holds, got.Reasons, got.Breaks)
	}
	if got.High != shape.High || got.Low != shape.Low || got.StartAt != shape.StartAt || got.EndAt != shape.EndAt || got.DownLegs != shape.DownLegs || got.UpLegs != shape.UpLegs {
		t.Errorf("the window's own readings are kept, got %+v want %+v", got, shape)
	}
}

// A reading that did not hold keeps the breaks that say why and names the trend
// after them, so a cancelled row says everything that missed.
func TestWithTrendFailsAReadingThatDidNotHold(t *testing.T) {
	miss := missingReading(t)
	got := miss.withTrend(series.Trend{Down: 0, Valid: 3}, 2)
	want := append(slices.Clone(miss.Breaks), "smc trend direction down 0/3 short of 2 needed")
	if got.Holds || got.Reasons != nil || !slices.Equal(got.Breaks, want) {
		t.Fatalf("want the shape's breaks then the trend, got holds %v, %q, breaks %q", got.Holds, got.Reasons, got.Breaks)
	}
}

// A trend sophos did not serve is named as not read, on a reading that held and
// on one that did not, and a count with no row read behind it is not read either.
func TestWithTrendNamesATrendNotRead(t *testing.T) {
	for name, trend := range map[string]series.Trend{"no row read": {}, "a count with no row": {Down: 3}} {
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
		{trendHoldReason(series.Trend{Down: 3, Valid: 3}, 2), "smc trend direction down 3/3 of 2 needed"},
		{trendBreakReason(series.Trend{Down: 1, Valid: 3}, 2), "smc trend direction down 1/3 short of 2 needed"},
		{trendBreakReason(series.Trend{}, 2), "smc trend direction not read"},
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
	miss.withTrend(series.Trend{}, 2)
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
		for _, trend := range []series.Trend{allDown, {Down: 1, Valid: 3}, {}} {
			for _, need := range []int{0, 2} {
				reading.withTrend(trend, need)
				if !slices.Equal(reading.Reasons, reasons) || !slices.Equal(reading.Breaks, breaks) {
					t.Fatalf("need %d, trend %+v: the reading's own slices changed, reasons %q want %q, breaks %q want %q", need, trend, reading.Reasons, reasons, reading.Breaks, breaks)
				}
			}
		}
	}
}
