package quietdecline

import (
	"reflect"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// firstBar is the open time of the oldest closed bar of the synthetic window the
// cases are read on. Every stamp a case serves or reads is an offset of it, so a
// case says how far a fill lies from a bound and never what the clock reads.
var firstBar = time.Date(2022, time.June, 8, 4, 0, 0, 0, time.UTC)

const (
	// bar is the length of a closed bar, and step the least a stamp can move a
	// comparison: sophos serves open times in milliseconds, so every read compares
	// in milliseconds, and a stamp inside a bound's own millisecond reads as the
	// bound does.
	bar  = time.Hour
	step = time.Millisecond
	// windowBars is how many closed bars the window holds. The look-back and the
	// fill window both reach over all of them, so firstBar is the oldest bar either
	// serves.
	windowBars = 24
)

var (
	// lastClosed is the open time of the newest closed bar, and forming the open
	// time of the bar after it, the one still forming and the bar a fill has to
	// precede to have its own bar closed.
	lastClosed = firstBar.Add((windowBars - 1) * bar)
	forming    = lastClosed.Add(bar)
	// stood is the open time of the bar the verdict stood on, halfway into the
	// window, and smoothFrom the open time of the earliest bar the leg reads smooth
	// from, a quarter of the way in.
	stood      = firstBar.Add(windowBars / 2 * bar)
	smoothFrom = firstBar.Add(windowBars / 4 * bar)
	// unstamped is a fill without a stamp.
	unstamped time.Time
)

// The reasons sophos names, one set for each way a read holds, so that a read
// that hands back the wrong set shows.
var (
	legReasons    = []string{"the leg is on and quiet"}
	recentReasons = []string{"the verdict stood on a bar among the last closed ones"}
)

// millis is an open time as sophos serves it, in milliseconds; the zero time
// serves zero.
func millis(at time.Time) int64 {
	if at.IsZero() {
		return 0
	}
	return at.UnixMilli()
}

// verdict is the block of a window on which the verdict stands.
func verdict() aggragates.SmartSellIndicators {
	return aggragates.SmartSellIndicators{QuietDeclineVerdict: true}
}

// legQuiet is the block of a window on which the leg is on and quiet and the
// verdict does not stand: a ladder's own smoothness reads from from, and a fill has
// to be stamped before before. A zero time serves zero.
func legQuiet(from, before time.Time) aggragates.SmartSellIndicators {
	return aggragates.SmartSellIndicators{
		QuietDeclineLegQuiet:    true,
		QuietDeclineSmoothFrom:  millis(from),
		QuietDeclineFillBefore:  millis(before),
		QuietDeclineExitReasons: legReasons,
	}
}

// recent is the block of a window on which the verdict stood on the closed bar
// opening at stoodOn, the oldest of the bars sophos looks back over opening at
// from. A zero time serves zero.
func recent(stoodOn, from time.Time) aggragates.SmartSellIndicators {
	return aggragates.SmartSellIndicators{
		QuietDeclineRecentAt:      millis(stoodOn),
		QuietDeclineRecentFrom:    millis(from),
		QuietDeclineRecentReasons: recentReasons,
	}
}

// withRecent is the block carrying the look-back of look as well.
func withRecent(block, look aggragates.SmartSellIndicators) aggragates.SmartSellIndicators {
	block.QuietDeclineRecentAt = look.QuietDeclineRecentAt
	block.QuietDeclineRecentFrom = look.QuietDeclineRecentFrom
	block.QuietDeclineRecentReasons = look.QuietDeclineRecentReasons
	return block
}

// fillWindow is the block of a window whose fill window opens at from; the zero
// time serves none.
func fillWindow(from time.Time) aggragates.SmartSellIndicators {
	return aggragates.SmartSellIndicators{QuietDeclineFillFrom: millis(from)}
}

// The verdict reads for every ladder, whatever the stamp of its newest fill. The
// leg on and quiet reads for a ladder whose newest fill is stamped at or after the
// earliest bar the leg reads smooth from and strictly before the bar a fill has to
// precede, in milliseconds: at the first bound, or anywhere inside its millisecond,
// it reads and a step before it does not; a step before the second it reads, at it
// or after it — a fill in the bar still forming — it does not. A bound not served,
// a fill without a stamp and a leg that is not on and quiet fail closed.
func TestReadsForTheLadderReadsTheVerdictOrTheLegsSmoothnessFromTheNewestFill(t *testing.T) {
	quiet := legQuiet(smoothFrom, forming)
	standing := legQuiet(smoothFrom, forming)
	standing.QuietDeclineVerdict = true
	notQuiet := legQuiet(smoothFrom, forming)
	notQuiet.QuietDeclineLegQuiet = false
	negativeSpan := legQuiet(smoothFrom, forming)
	negativeSpan.QuietDeclineSmoothFrom = -millis(smoothFrom)

	cases := []struct {
		name   string
		newest time.Time
		block  aggragates.SmartSellIndicators
		want   bool
	}{
		{"the verdict reads for a fill without a stamp", unstamped, verdict(), true},
		{"the verdict reads outside the smooth span", forming.Add(bar), standing, true},
		{"a fill at the bar the leg reads smooth from", smoothFrom, quiet, true},
		{"a fill inside that bar's first millisecond", smoothFrom.Add(step - time.Nanosecond), quiet, true},
		{"a fill a bar after it", smoothFrom.Add(bar), quiet, true},
		{"a fill a step before the bar a fill must precede", forming.Add(-step), quiet, true},
		{"a fill at the bar still forming", forming, quiet, false},
		{"a fill inside the forming bar's first millisecond", forming.Add(step - time.Nanosecond), quiet, false},
		{"a fill after the forming bar's open", forming.Add(bar), quiet, false},
		{"a fill a step before the bar the leg reads smooth from", smoothFrom.Add(-step), quiet, false},
		{"a fill without a stamp", unstamped, quiet, false},
		{"the leg not on and quiet", smoothFrom, notQuiet, false},
		{"no smooth span served", smoothFrom, legQuiet(time.Time{}, forming), false},
		{"a negative smooth span", smoothFrom, negativeSpan, false},
		{"no bar to precede served", smoothFrom, legQuiet(smoothFrom, time.Time{}), false},
		{"nothing served", smoothFrom, aggragates.SmartSellIndicators{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ReadsForTheLadder(tc.newest, tc.block); got != tc.want {
				t.Errorf("reads for the ladder %v, want %v", got, tc.want)
			}
		})
	}
}

// A pending ladder's new fill holds the quiet decline on the leg on and quiet,
// naming the leg's reasons whatever the fill's stamp, or else on the decline read
// recently from that fill, naming the recent bar's reasons. The leg's come first
// when both hold. A new fill before the oldest bar, without a stamp, or with no
// recent bar served holds nothing and names nothing.
func TestConfirmsHoldsOnTheLegOnAndQuietOrOnTheDeclineReadRecently(t *testing.T) {
	look := recent(stood, firstBar)
	cases := []struct {
		name    string
		newest  time.Time
		block   aggragates.SmartSellIndicators
		reasons []string
		want    bool
	}{
		{"the leg on and quiet, with its own reasons", lastClosed, legQuiet(smoothFrom, forming), legReasons, true},
		{"the leg on and quiet, a fill without a stamp", unstamped, legQuiet(smoothFrom, forming), legReasons, true},
		{"the leg's reasons first when the look-back reads too", lastClosed, withRecent(legQuiet(smoothFrom, forming), look), legReasons, true},
		{"the look-back, a fill at its oldest bar", firstBar, look, recentReasons, true},
		{"the look-back, a fill after the bar the verdict stood on", forming, look, recentReasons, true},
		{"the look-back, a fill a step before its oldest bar", firstBar.Add(-step), look, nil, false},
		{"the look-back, a fill without a stamp", unstamped, look, nil, false},
		{"no recent bar served", lastClosed, recent(time.Time{}, firstBar), nil, false},
		{"nothing served", lastClosed, aggragates.SmartSellIndicators{}, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reasons, got := Confirms(tc.newest, tc.block)
			if got != tc.want || !reflect.DeepEqual(reasons, tc.reasons) {
				t.Errorf("confirms %v with %q, want %v with %q", got, reasons, tc.want, tc.reasons)
			}
		})
	}
}

// The decline reads recently for a ladder whose newest fill is stamped at or after
// the oldest of the last closed bars sophos looks back over, in milliseconds, while
// a bar among them is served as the one the verdict stood on. Nothing bounds the
// stamp from above: a fill after the bar the verdict stood on, or in the bar still
// forming, counts. A fill a step before the oldest bar, a fill without a stamp, no
// recent bar served and no oldest bar served fail closed.
func TestReadsRecentlyWeighsTheNewestFillAgainstTheOldestBarOfTheLookBack(t *testing.T) {
	look := recent(stood, firstBar)
	cases := []struct {
		name   string
		newest time.Time
		block  aggragates.SmartSellIndicators
		want   bool
	}{
		{"a fill at the oldest bar", firstBar, look, true},
		{"a fill inside that bar's first millisecond", firstBar.Add(step - time.Nanosecond), look, true},
		{"a fill a bar into the look-back", firstBar.Add(bar), look, true},
		{"a fill in the bar the verdict stood on", stood, look, true},
		{"a fill after the bar the verdict stood on", lastClosed, look, true},
		{"a fill in the bar still forming", forming.Add(bar / 2), look, true},
		{"a fill a step before the oldest bar", firstBar.Add(-step), look, false},
		{"a fill a nanosecond before the oldest bar", firstBar.Add(-time.Nanosecond), look, false},
		{"a fill without a stamp", unstamped, look, false},
		{"no recent bar served", stood, recent(time.Time{}, firstBar), false},
		{"no oldest bar served", stood, recent(stood, time.Time{}), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ReadsRecently(tc.newest, tc.block); got != tc.want {
				t.Errorf("reads recently %v, want %v", got, tc.want)
			}
		})
	}
}

// The look-back marks a watched ladder pending only when it reads recently for the
// ladder and the ladder already stood when the verdict stood: its first fill is
// stamped strictly before the bar the verdict stood on, in milliseconds. Opened at
// that bar's open, inside its first millisecond, after it, or without a stamp, the
// ladder is not marked; nor is one whose newest fill the look-back does not reach.
// A ladder of one fill is its own first fill, and one with no fill has no stamp.
func TestPendsRecentlyNeedsALadderThatStoodBeforeTheBar(t *testing.T) {
	look := recent(stood, firstBar)
	cases := []struct {
		name          string
		first, newest time.Time
		block         aggragates.SmartSellIndicators
		want          bool
	}{
		{"a first fill a step before the bar the verdict stood on", stood.Add(-step), lastClosed, look, true},
		{"a first fill at the oldest bar of the look-back", firstBar, lastClosed, look, true},
		{"a first fill at the bar's open", stood, lastClosed, look, false},
		{"a first fill inside that bar's first millisecond", stood.Add(step - time.Nanosecond), lastClosed, look, false},
		{"a first fill after the bar", stood.Add(bar), lastClosed, look, false},
		{"a first fill without a stamp", unstamped, lastClosed, look, false},
		{"a newest fill before the oldest bar", firstBar.Add(-2 * bar), firstBar.Add(-step), look, false},
		{"a newest fill without a stamp", stood.Add(-step), unstamped, look, false},
		{"a ladder of one fill that stood before the bar", firstBar, firstBar, look, true},
		{"a ladder of one fill opened after the bar", stood.Add(bar), stood.Add(bar), look, false},
		{"a ladder with no fill", unstamped, unstamped, look, false},
		{"no recent bar served", stood.Add(-step), lastClosed, recent(time.Time{}, firstBar), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PendsRecently(tc.first, tc.newest, tc.block); got != tc.want {
				t.Errorf("pends recently %v, want %v", got, tc.want)
			}
		})
	}
}

// The newest fill is compared with QuietDeclineFillFrom in milliseconds, the
// window's open included: at it, or anywhere inside its first millisecond, the fill
// is recent; a step or a nanosecond before, it is not. Nothing bounds the stamp
// from above, so a fill in the bar still forming is recent. No window served, a
// negative one, or a newest fill without a stamp fails closed. The switch of the
// rule is the gate's to apply, so the read never asks it.
func TestRecentFillWeighsTheNewestFillAgainstTheFillWindow(t *testing.T) {
	window := fillWindow(firstBar)
	negative := fillWindow(firstBar)
	negative.QuietDeclineFillFrom = -negative.QuietDeclineFillFrom

	cases := []struct {
		name   string
		newest time.Time
		block  aggragates.SmartSellIndicators
		want   bool
	}{
		{"a fill at the window's open", firstBar, window, true},
		{"a fill inside its first millisecond", firstBar.Add(step - time.Nanosecond), window, true},
		{"a fill a step after it", firstBar.Add(step), window, true},
		{"a fill in the bar still forming", forming.Add(bar / 2), window, true},
		{"a fill a step before the window's open", firstBar.Add(-step), window, false},
		{"a fill a nanosecond before it", firstBar.Add(-time.Nanosecond), window, false},
		{"a fill without a stamp", unstamped, window, false},
		{"no window served", firstBar, fillWindow(time.Time{}), false},
		{"a negative window", firstBar, negative, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RecentFill(tc.newest, tc.block); got != tc.want {
				t.Errorf("recent fill %v, want %v", got, tc.want)
			}
		})
	}
}
