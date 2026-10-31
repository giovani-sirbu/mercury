package smartsell

import (
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// firstBar is the open time of the oldest closed bar of the synthetic window the
// cases are served on. Every bar a case serves is an offset of it, so a case says
// how far into the window the indecision stood and never what the clock reads.
var lookBackFirstBar = time.Date(2022, time.June, 8, 4, 0, 0, 0, time.UTC)

const (
	// bar is the length of a closed bar, and windowBars how many closed bars the
	// window holds; sophos' look-back reaches over all of them. step is the least
	// an open time can move: sophos serves open times in milliseconds.
	lookBackBar        = time.Hour
	lookBackWindowBars = 24
	lookBackStep       = time.Millisecond
)

var (
	// lastClosed is the open time of the newest closed bar, forming the open time
	// of the bar after it, the one still forming, and stood the open time of a bar
	// halfway into the window.
	lookBackLastClosed = lookBackFirstBar.Add((lookBackWindowBars - 1) * lookBackBar)
	lookBackForming    = lookBackLastClosed.Add(lookBackBar)
	lookBackStood      = lookBackFirstBar.Add(lookBackWindowBars / 2 * lookBackBar)
	// epoch is the open time sophos serves as zero, and unserved the zero time,
	// which serves no bar at all.
	lookBackEpoch    = time.UnixMilli(0)
	lookBackUnserved time.Time
)

// The reasons sophos names, one set for each reading that carries them, so that a
// read taking its answer from the wrong field shows.
var (
	lookBackBreakReasons  = []string{"the vote fails even with the ladder's own smoothness"}
	lookBackRecentReasons = []string{"the indecision stood on a bar among the last closed ones"}
	lookBackQuietReasons  = []string{"the verdict stood on a bar among the last closed ones"}
)

// millis is an open time as sophos serves it, in milliseconds; the zero time
// serves zero.
func lookBackMillis(at time.Time) int64 {
	if at.IsZero() {
		return 0
	}
	return at.UnixMilli()
}

// lookBack is the block of a window on which the indecision stood on the closed
// bar opening at stoodOn, served with that bar's reasons. The zero time serves
// neither.
func lookBackLookBack(stoodOn time.Time) aggragates.SmartSellIndicators {
	block := aggragates.SmartSellIndicators{IndecisionRecentAt: lookBackMillis(stoodOn)}
	if !stoodOn.IsZero() {
		block.IndecisionRecentReasons = lookBackRecentReasons
	}
	return block
}

// withLookBack is the block carrying the look-back of the indecision as well.
func lookBackWithLookBack(block aggragates.SmartSellIndicators, stoodOn time.Time) aggragates.SmartSellIndicators {
	look := lookBackLookBack(stoodOn)
	block.IndecisionRecentAt = look.IndecisionRecentAt
	block.IndecisionRecentReasons = look.IndecisionRecentReasons
	return block
}

// flagged is the block of a window on which the indecision stands on the last
// closed bar: the flag, served with the reasons of that bar.
func lookBackFlagged() aggragates.SmartSellIndicators {
	return aggragates.SmartSellIndicators{
		Indecision:               true,
		QuietDeclineBreakReasons: lookBackBreakReasons,
	}
}

// quietLookBack is the block of a window on which the quiet decline's verdict
// stood on a bar among the last closed ones: another look-back, of another key.
func lookBackQuietLookBack() aggragates.SmartSellIndicators {
	return aggragates.SmartSellIndicators{
		QuietDeclineRecentAt:      lookBackMillis(lookBackStood),
		QuietDeclineRecentFrom:    lookBackMillis(lookBackFirstBar),
		QuietDeclineRecentReasons: lookBackQuietReasons,
	}
}

// everythingElse is the block of a window sophos read in full that serves every
// other reading the gate takes off the block, and no bar of the indecision's own
// look-back: the verdict, the leg on and quiet with its span, the flag, the quiet
// decline's look-back and the fill window.
func lookBackEverythingElse() aggragates.SmartSellIndicators {
	block := lookBackQuietLookBack()
	block.QuietDeclineVerdict = true
	block.QuietDeclineLegQuiet = true
	block.QuietDeclineSmoothFrom = lookBackMillis(lookBackFirstBar)
	block.QuietDeclineFillBefore = lookBackMillis(lookBackForming)
	block.QuietDeclineFillFrom = lookBackMillis(lookBackFirstBar)
	block.Indecision = true
	block.QuietDeclineBreakReasons = lookBackBreakReasons
	return block
}

// The indecision reads recently when sophos served a bar the indecision stood on:
// an open time above zero, the last closed bar, the oldest of the window and any
// bar between included. The read compares that open time with zero and with
// nothing else, no clock and no bound above it, so even a bar after the last
// closed one reads. No bar served, an open time of zero and one below it fail
// closed, whether or not the reasons came along.
func TestReadsRecentlyReadsTheOpenTimeOfTheBarSophosServed(t *testing.T) {
	cases := []struct {
		name  string
		block aggragates.SmartSellIndicators
		want  bool
	}{
		{"the indecision stood on the last closed bar", lookBackLookBack(lookBackLastClosed), true},
		{"on the oldest bar of the window", lookBackLookBack(lookBackFirstBar), true},
		{"on a bar halfway into the window", lookBackLookBack(lookBackStood), true},
		{"on the bar still forming", lookBackLookBack(lookBackForming), true},
		{"a bar served without its reasons", aggragates.SmartSellIndicators{IndecisionRecentAt: lookBackMillis(lookBackStood)}, true},
		{"the least open time above zero", lookBackLookBack(lookBackEpoch.Add(lookBackStep)), true},
		{"an open time of zero", lookBackLookBack(lookBackEpoch), false},
		{"an open time a step below zero", lookBackLookBack(lookBackEpoch.Add(-lookBackStep)), false},
		{"an open time as far below zero as the last closed bar lies above it", lookBackLookBack(lookBackEpoch.Add(-lookBackLastClosed.Sub(lookBackEpoch))), false},
		{"no bar served", lookBackLookBack(lookBackUnserved), false},
		{"nothing served", aggragates.SmartSellIndicators{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := indecisionReadsRecently(tc.block); got != tc.want {
				t.Errorf("reads recently %v, want %v", got, tc.want)
			}
		})
	}
}

// Nothing but that bar decides it. The flag of the last closed bar with its
// reasons, the reasons of a bar served without the bar, the verdict, the leg on
// and quiet, the quiet decline's own look-back and the fill window are read by
// the gate elsewhere, and served without a bar of the indecision's look-back
// they read nothing: the flag has its own latch, asked first, and the look-back
// is the second way to it, no restatement of it. Served with the bar, the read
// holds whatever else stands, the flag included.
func TestReadsRecentlyAsksNothingElseOfTheBlock(t *testing.T) {
	cases := []struct {
		name  string
		block aggragates.SmartSellIndicators
		want  bool
	}{
		{"the flag of the last closed bar with its reasons", lookBackFlagged(), false},
		{"the reasons of a bar, without the bar", aggragates.SmartSellIndicators{IndecisionRecentReasons: lookBackRecentReasons}, false},
		{"the verdict and the leg on and quiet", aggragates.SmartSellIndicators{QuietDeclineVerdict: true, QuietDeclineLegQuiet: true}, false},
		{"the quiet decline's look-back", lookBackQuietLookBack(), false},
		{"the fill window", aggragates.SmartSellIndicators{QuietDeclineFillFrom: lookBackMillis(lookBackFirstBar)}, false},
		{"every other reading together", lookBackEverythingElse(), false},
		{"the flag with the bar", lookBackWithLookBack(lookBackFlagged(), lookBackStood), true},
		{"the quiet decline's look-back with the bar", lookBackWithLookBack(lookBackQuietLookBack(), lookBackLastClosed), true},
		{"every other reading with the bar", lookBackWithLookBack(lookBackEverythingElse(), lookBackFirstBar), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := indecisionReadsRecently(tc.block); got != tc.want {
				t.Errorf("reads recently %v, want %v", got, tc.want)
			}
		})
	}
}
