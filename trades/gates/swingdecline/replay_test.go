package swingdecline

import (
	"reflect"
	"testing"
	"time"
)

// at is a UTC stamp on the tapes' days.
func at(year int, month time.Month, day, hour, minute, second int) time.Time {
	return time.Date(year, month, day, hour, minute, second, 0, time.UTC)
}

// solFills are the four depths of trade 113054, SOL on 2021-10-08, with the
// stamps and prices its history carries.
func solFills() []Fill {
	return []Fill{
		{Price: 166.75, At: at(2021, time.October, 8, 14, 20, 12)},
		{Price: 160.35, At: at(2021, time.October, 8, 20, 57, 14)},
		{Price: 157.21, At: at(2021, time.October, 9, 1, 27, 40)},
		{Price: 154.13, At: at(2021, time.October, 10, 0, 55, 47)},
	}
}

// hbarFills are the four depths of trade 111284, HBAR on 2021-12-01.
func hbarFills() []Fill {
	return []Fill{
		{Price: 0.3567, At: at(2021, time.December, 1, 4, 15, 50)},
		{Price: 0.3497, At: at(2021, time.December, 1, 19, 23, 19)},
		{Price: 0.3428, At: at(2021, time.December, 2, 0, 44, 44)},
		{Price: 0.3345, At: at(2021, time.December, 3, 1, 0, 59)},
	}
}

// Trade 113054 — SOL, October 2021 — filled four depths down a smooth
// staircase that never recovered: the rule holds on the window from depth 1 to
// depth 4, whose end bar is the one opening 2021-10-10 00:00 UTC, and not on
// the same fill one bar earlier, where the decline is still one flush. Read
// on the real 1h tape. Three fills read nothing. The shape is what is read
// here, under a trend that holds (bearishTrend): the trend the dashboard read at
// that bar is TestReplaySolTheTrendDecidesTheLadder's.
func TestReplaySol(t *testing.T) {
	series := loadTape(t, "solusdt-1h-2021-10.json")
	fills := solFills()

	reading, holds := Trigger(fills, series, bearishTrend)
	if !holds {
		t.Fatalf("the ladder must trigger at its fourth fill, got %+v", reading)
	}
	t.Logf("SOL depth 1 to 4: weight %.4f, legs down %d, lower highs %d, largest leg %.4f, net %.3f%%\n%q",
		reading.Weight, reading.DownLegs, reading.LowerHighs, reading.MaxLegShare, reading.NetPct, reading.Reasons)
	if reading.FromDepth != 1 || reading.ToDepth != 4 {
		t.Errorf("want the window from depth 1 to 4, got %d to %d", reading.FromDepth, reading.ToDepth)
	}
	if reading.StartAt != at(2021, time.October, 8, 14, 0, 0).UnixMilli() || reading.EndAt != at(2021, time.October, 10, 0, 0, 0).UnixMilli() {
		t.Errorf("want the bars 2021-10-08 14:00 to 2021-10-10 00:00 UTC, got %s", reading.Reasons[0])
	}
	assertNear(t, "weight", reading.Weight, 0.50, 0.01)
	assertNear(t, "largest leg share", reading.MaxLegShare, 0.51, 0.01)
	assertNear(t, "net", reading.NetPct, -7.7, 0.1)
	if reading.DownLegs != 4 || reading.LowerHighs != 3 {
		t.Errorf("want 4 legs down and 3 lower highs, got %d and %d", reading.DownLegs, reading.LowerHighs)
	}
	if reading.Reasons[0] != "depth 1 to 4, 1h bars 2021-10-08 14:00 to 2021-10-10 00:00 UTC" {
		t.Errorf("the window frame, got %q", reading.Reasons[0])
	}

	early := solFills()
	early[3].At = at(2021, time.October, 9, 23, 55, 0)
	if reading, holds := Trigger(early, series, bearishTrend); holds {
		t.Errorf("the fourth fill one bar earlier, end bar 2021-10-09 23:00, must not hold, got %+v", reading)
	} else {
		t.Logf("SOL with end bar 2021-10-09 23:00: weight %.4f, legs down %d, lower highs %d, largest leg %.4f, net %.3f%%\n%q",
			reading.Weight, reading.DownLegs, reading.LowerHighs, reading.MaxLegShare, reading.NetPct, reading.Breaks)
		// One flush is the only thing wrong with it: the other four readings hold.
		want := []string{"depth 1 to 4, 1h bars 2021-10-08 14:00 to 2021-10-09 23:00 UTC", "largest leg 64% of the fall over 60%"}
		if !reflect.DeepEqual(reading.Breaks, want) || reading.EndAt != at(2021, time.October, 9, 23, 0, 0).UnixMilli() {
			t.Errorf("the bar before breaks on its largest leg alone, got %q", reading.Breaks)
		}
	}
	if reading, holds := Trigger(fills[:3], series, bearishTrend); holds || len(reading.Breaks) != 1 {
		t.Errorf("three fills must read nothing, got holds %v, %+v", holds, reading)
	}
}

// Trade 111284 — HBAR, December 2021 — the same on a tape whose decline sits
// near the weight threshold: the window from depth 1 to 4 holds at the end bar
// opening 2021-12-03 01:00 UTC.
func TestReplayHbar(t *testing.T) {
	series := loadTape(t, "hbarusdt-1h-2021-12.json")
	fills := hbarFills()

	reading, holds := Trigger(fills, series, bearishTrend)
	if !holds {
		t.Fatalf("the ladder must trigger at its fourth fill, got %+v", reading)
	}
	t.Logf("HBAR depth 1 to 4: weight %.4f, legs down %d, lower highs %d, largest leg %.4f, net %.3f%%\n%q",
		reading.Weight, reading.DownLegs, reading.LowerHighs, reading.MaxLegShare, reading.NetPct, reading.Reasons)
	if reading.FromDepth != 1 || reading.ToDepth != 4 {
		t.Errorf("want the window from depth 1 to 4, got %d to %d", reading.FromDepth, reading.ToDepth)
	}
	if reading.StartAt != at(2021, time.December, 1, 4, 0, 0).UnixMilli() || reading.EndAt != at(2021, time.December, 3, 1, 0, 0).UnixMilli() {
		t.Errorf("want the bars 2021-12-01 04:00 to 2021-12-03 01:00 UTC, got %s", reading.Reasons[0])
	}
	assertNear(t, "weight", reading.Weight, 0.36, 0.01)
	assertNear(t, "largest leg share", reading.MaxLegShare, 0.34, 0.01)
	assertNear(t, "net", reading.NetPct, -5.9, 0.1)
	if reading.DownLegs != 5 || reading.LowerHighs != 2 {
		t.Errorf("want 5 legs down and 2 lower highs, got %d and %d", reading.DownLegs, reading.LowerHighs)
	}
	if reading, holds := Trigger(fills[:3], series, bearishTrend); holds || len(reading.Breaks) != 1 {
		t.Errorf("three fills must read nothing, got holds %v, %+v", holds, reading)
	}
}

// The Trend Direction row of sophos' SMC trend dashboard, as its own dashboard
// code read it on the exchange's candles at the end bar of each ladder's fourth
// fill — the chart row on 1h, the rows above it on 4h and 1D. SOL: the 1h row
// down, the 4h and 1D rows up. HBAR: all three down. They are measured readings
// and stay as read; what they do to a ladder is SwingDeclineMinSmcTrendDown's to
// say, which the two replays below state as a function of it (trendAnswer).
var (
	solTrend  = Trend{Down: 1, Valid: 3}
	hbarTrend = Trend{Down: 3, Valid: 3}
)

// shapeReasons is how many reasons a window that holds on its shape names: its
// frame and its five readings.
const shapeReasons = 6

// assertTrendDecides fails unless Trigger answers the ladder, whose window holds
// on its shape, as its trend decides it (trendAnswer): it triggers exactly when
// the rows down reach SwingDeclineMinSmcTrendDown — always, with the condition off
// — its reasons the window's frame, the five readings and the trend last, and when
// it does not its breaks name the frame and the trend alone.
func assertTrendDecides(t *testing.T, fills []Fill, series Series, trend Trend, frame string) {
	t.Helper()
	reading, holds := Trigger(fills, series, trend)
	wantHolds, text := trendAnswer(trend)
	if holds != wantHolds || reading.Holds != wantHolds {
		t.Fatalf("trend %+v: the ladder triggers %v, want %v, got %+v", trend, holds, wantHolds, reading)
	}
	if !wantHolds {
		if reading.Reasons != nil || !reflect.DeepEqual(reading.Breaks, []string{frame, text}) {
			t.Errorf("trend %+v: a trend that fails names the window and the trend alone, got %+v", trend, reading)
		}
		return
	}
	named := shapeReasons
	if text != "" {
		named++
	}
	if reading.Breaks != nil || len(reading.Reasons) != named || reading.Reasons[0] != frame || (text != "" && reading.Reasons[named-1] != text) {
		t.Errorf("trend %+v: the reasons open with the window and end with the trend, got %q", trend, reading.Reasons)
	}
}

// Trade 113054 — SOL, October 2021: the window from depth 1 to 4 holds on its
// shape at the end bar opening 2021-10-10 00:00 UTC, and the dashboard reads the
// Trend Direction down on one row of three there. The ladder triggers when that
// one row reaches SwingDeclineMinSmcTrendDown and does not when it falls short —
// the reading then names the window and the trend alone — while the same ladder
// under a trend that meets the constant always does.
func TestReplaySolTheTrendDecidesTheLadder(t *testing.T) {
	series := loadTape(t, "solusdt-1h-2021-10.json")
	frame := "depth 1 to 4, 1h bars 2021-10-08 14:00 to 2021-10-10 00:00 UTC"
	assertTrendDecides(t, solFills(), series, solTrend, frame)
	assertTrendDecides(t, solFills(), series, bearishTrend, frame)
}

// Trade 111284 — HBAR, December 2021: the dashboard reads the Trend Direction
// down on every row at the end bar opening 2021-12-03 01:00 UTC, so the ladder
// triggers — and its reasons end with the trend — for any SwingDeclineMinSmcTrendDown
// up to the dashboard's three rows.
func TestReplayHbarTheTrendDecidesTheLadder(t *testing.T) {
	series := loadTape(t, "hbarusdt-1h-2021-12.json")
	assertTrendDecides(t, hbarFills(), series, hbarTrend, "depth 1 to 4, 1h bars 2021-12-01 04:00 to 2021-12-03 01:00 UTC")
}
