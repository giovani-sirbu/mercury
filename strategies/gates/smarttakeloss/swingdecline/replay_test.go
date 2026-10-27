package swingdecline

import (
	"slices"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
)

// at is a UTC stamp on the tapes' days.
func at(year int, month time.Month, day, hour, minute, second int) time.Time {
	return time.Date(year, month, day, hour, minute, second, 0, time.UTC)
}

// solFills are the four depths of trade 113054, SOL on 2021-10-08, with the
// stamps and prices its history carries.
func solFills() []series.Fill {
	return []series.Fill{
		{Price: 166.75, At: at(2021, time.October, 8, 14, 20, 12)},
		{Price: 160.35, At: at(2021, time.October, 8, 20, 57, 14)},
		{Price: 157.21, At: at(2021, time.October, 9, 1, 27, 40)},
		{Price: 154.13, At: at(2021, time.October, 10, 0, 55, 47)},
	}
}

// hbarFills are the four depths of trade 111284, HBAR on 2021-12-01.
func hbarFills() []series.Fill {
	return []series.Fill{
		{Price: 0.3567, At: at(2021, time.December, 1, 4, 15, 50)},
		{Price: 0.3497, At: at(2021, time.December, 1, 19, 23, 19)},
		{Price: 0.3428, At: at(2021, time.December, 2, 0, 44, 44)},
		{Price: 0.3345, At: at(2021, time.December, 3, 1, 0, 59)},
	}
}

// laterBars is the tape cut hours bars after the fourth fill's bar, the last
// closed bar a later tick reads to.
func laterBars(t *testing.T, tape series.Series, fills []series.Fill, hours int) series.Series {
	t.Helper()
	index, ok := tape.IndexOf(series.BarOpen(fills[3].At) + int64(hours)*series.BarMs)
	if !ok {
		t.Fatalf("the tape does not hold the bar %d hours after the fourth fill", hours)
	}
	return cut(tape, 0, index+1)
}

// Trade 113054 — SOL, October 2021 — filled four depths down a staircase of
// three to five percent steps that the rule, as it ships, does not read as a
// decline: on the real 1h tape, with the window led a day before the first fill,
// it fell from the 168.97 high wick of 2021-10-08 15:00 UTC to the 156.06 low
// wick of 2021-10-09 01:00 UTC, one down leg that counts and one bounce under
// the bound beside one over it; five bars after the fourth fill the second leg
// counts and the bounce that counted has run past the bound. Three fills read nothing.
func TestReplaySol(t *testing.T) {
	tape := loadTape(t, "solusdt-1h-2021-10.json")
	fills := solFills()

	reading, holds := Trigger(fills, tape, bearishTrend)
	t.Logf("SOL depth 1 to 4: %+v", reading)
	if holds || reading.Holds || reading.FromDepth != 1 || reading.ToDepth != 4 {
		t.Fatalf("the ladder must not trigger at its fourth fill as the rule ships, got holds %v, %+v", holds, reading)
	}
	if reading.StartAt != at(2021, time.October, 7, 14, 0, 0).UnixMilli() || reading.EndAt != at(2021, time.October, 10, 0, 0, 0).UnixMilli() {
		t.Errorf("want the bars 2021-10-07 14:00 to 2021-10-10 00:00 UTC, got %s", reading.Breaks[0])
	}
	if reading.High != 168.97 || reading.HighAt != at(2021, time.October, 8, 15, 0, 0).UnixMilli() || reading.Low != 156.06 || reading.LowAt != at(2021, time.October, 9, 1, 0, 0).UnixMilli() {
		t.Errorf("want the high 168.97 at 10-08 15:00 and the low 156.06 at 10-09 01:00, got %+v", reading)
	}
	if reading.DownLegs != 1 || reading.UpLegs != 1 || reading.Bounces != 1 {
		t.Errorf("want 1 down leg, 1 up leg and 1 bounce over the bound, got %+v", reading)
	}
	want := []string{
		"depth 1 to 4, 1h bars 2021-10-07 14:00 to 2021-10-10 00:00 UTC",
		"high 168.97 at 2021-10-08 15:00 UTC, low 156.06 at 2021-10-09 01:00 UTC, down 7.6%",
		"1 legs down short of 2 needed, each 6 bars and 5.0% down at least",
	}
	if !slices.Equal(reading.Breaks, want) {
		t.Errorf("breaks:\ngot  %q\nwant %q", reading.Breaks, want)
	}

	later, holds := TriggerAtLastBar(fills, laterBars(t, tape, fills, 5), bearishTrend)
	if holds || later.DownLegs != 2 || later.UpLegs != 0 || later.Bounces != 2 || later.Low != 152.38 {
		t.Errorf("five bars on, want 2 down legs, no up leg and 2 bounces over the bound to the low 152.38, got holds %v, %+v", holds, later)
	}
	if reading, holds := Trigger(fills[:3], tape, bearishTrend); holds || len(reading.Breaks) != 1 {
		t.Errorf("three fills must read nothing, got holds %v, %+v", holds, reading)
	}
}

// Trade 111284 — HBAR, December 2021 — the same on a tape whose decline fell
// from the 0.363 high wick of 2021-12-01 17:00 UTC to the 0.3332 low wick of
// 2021-12-02 03:00 UTC in one leg that counts, with one bounce under the bound
// and two over it; five bars after the fourth fill a second bounce counts and
// still no second down leg.
func TestReplayHbar(t *testing.T) {
	tape := loadTape(t, "hbarusdt-1h-2021-12.json")
	fills := hbarFills()

	reading, holds := Trigger(fills, tape, bearishTrend)
	t.Logf("HBAR depth 1 to 4: %+v", reading)
	if holds || reading.Holds || reading.FromDepth != 1 || reading.ToDepth != 4 {
		t.Fatalf("the ladder must not trigger at its fourth fill as the rule ships, got holds %v, %+v", holds, reading)
	}
	if reading.StartAt != at(2021, time.November, 30, 4, 0, 0).UnixMilli() || reading.EndAt != at(2021, time.December, 3, 1, 0, 0).UnixMilli() {
		t.Errorf("want the bars 2021-11-30 04:00 to 2021-12-03 01:00 UTC, got %s", reading.Breaks[0])
	}
	if reading.High != 0.363 || reading.HighAt != at(2021, time.December, 1, 17, 0, 0).UnixMilli() || reading.Low != 0.3332 || reading.LowAt != at(2021, time.December, 2, 3, 0, 0).UnixMilli() {
		t.Errorf("want the high 0.363 at 12-01 17:00 and the low 0.3332 at 12-02 03:00, got %+v", reading)
	}
	if reading.DownLegs != 1 || reading.UpLegs != 1 || reading.Bounces != 2 {
		t.Errorf("want 1 down leg, 1 up leg and 2 bounces over the bound, got %+v", reading)
	}
	if reading.Breaks[1] != "high 0.363 at 2021-12-01 17:00 UTC, low 0.3332 at 2021-12-02 03:00 UTC, down 8.2%" {
		t.Errorf("the swings, got %q", reading.Breaks[1])
	}
	later, holds := TriggerAtLastBar(fills, laterBars(t, tape, fills, 5), bearishTrend)
	if holds || later.DownLegs != 1 || later.UpLegs != 2 || later.Low != 0.3297 {
		t.Errorf("five bars on, want still 1 down leg, 2 up legs and the low 0.3297, got holds %v, %+v", holds, later)
	}
	if reading, holds := Trigger(fills[:3], tape, bearishTrend); holds || len(reading.Breaks) != 1 {
		t.Errorf("three fills must read nothing, got holds %v, %+v", holds, reading)
	}
}

// A window that misses on the real tape names the trend after what missed, and
// under a trend that meets the need names what missed alone: the trend never
// rescues a shape.
func TestReplayTheTrendIsNamedAfterTheBreaks(t *testing.T) {
	needsTheTrend(t)
	tape := loadTape(t, "solusdt-1h-2021-10.json")
	shape, _ := Trigger(solFills(), tape, bearishTrend)
	short, holds := Trigger(solFills(), tape, shortTrend)
	if holds || !slices.Equal(short.Breaks, append(slices.Clone(shape.Breaks), shortTrendBreak)) {
		t.Errorf("want the shape's breaks then the trend, got holds %v, %q", holds, short.Breaks)
	}
}
