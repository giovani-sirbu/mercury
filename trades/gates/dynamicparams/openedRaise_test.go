package dynamicparams_test

import (
	"reflect"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/dynamicparams"
)

// withRows is the trade carrying these log rows, in order.
func withRows(trade aggragates.Trades, messages ...string) aggragates.Trades {
	trade.Logs = nil
	for _, message := range messages {
		trade.Logs = append(trade.Logs, aggragates.TradesLogs{Message: message, Type: aggragates.LOG_INFO})
	}
	return trade
}

// Every amount form OpenedMessage writes reads back as the very amounts
// written: the percentage alone, the depths alone, both, whole and fractional
// points, one depth and several.
func TestOpenedRaiseReadsBackEveryAmountForm(t *testing.T) {
	cases := []struct {
		points float64
		depths int
	}{
		{0.4, 1},
		{0.4, 0},
		{0, 1},
		{0.5, 2},
		{1, 1},
		{0.125, 3},
		{2.75, 0},
		{0.1, 12},
		{0.3, 0},
		{0.000001, 1},
	}

	for _, c := range cases {
		trade := withRows(flaggedTrade(), dynamicparams.OpenedMessage(c.points, c.depths))
		points, depths, ok := dynamicparams.OpenedRaise(trade)
		if !ok || points != c.points || depths != c.depths {
			t.Errorf("OpenedMessage(%v, %d) reads back as %v, %d, %v", c.points, c.depths, points, depths, ok)
		}
	}
}

// A retune of the constants reaches no ladder already open: a row written
// under other amounts than the constants name now reads back as its own
// amounts, and the ladder trades its stored rows raised by those — not by
// BearPercentagePoints and BearDepths.
func TestOpenedRaiseReadsTheRowNotTheConstants(t *testing.T) {
	writtenPoints := dynamicparams.BearPercentagePoints + 0.35
	writtenDepths := dynamicparams.BearDepths + 2
	trade := withRows(flaggedTrade(), dynamicparams.OpenedMessage(writtenPoints, writtenDepths))

	points, depths, ok := dynamicparams.OpenedRaise(trade)
	if !ok || points != writtenPoints || depths != writtenDepths {
		t.Fatalf("the row reads back as %v, %d, %v, want the amounts it was written with %v, %d", points, depths, ok, writtenPoints, writtenDepths)
	}

	rows, raised := dynamicparams.RaisedSettings(trade)
	want := dynamicparams.RaiseBy(threeRowLadder(), writtenPoints, writtenDepths)
	if !raised || !reflect.DeepEqual(rows, want) {
		t.Fatalf("the ladder trades %+v (raised %v), want its stored rows raised by the row's amounts %+v", rows, raised, want)
	}
}

// The rows fold in slice order and the last opened row wins, whatever frame
// an engine writes around it.
func TestOpenedRaiseFoldsTheRowsInOrder(t *testing.T) {
	trade := withRows(flaggedTrade(),
		"Updated position to buy from new",
		dynamicparams.OpenedMessage(0.4, 1),
		"Hold entry: smartTakeLoss: quiet slow decline, first fill held",
		"Hold entry: "+dynamicparams.OpenedMessage(0.75, 0),
	)

	points, depths, ok := dynamicparams.OpenedRaise(trade)
	if !ok || points != 0.75 || depths != 0 {
		t.Fatalf("the last opened row, under its hold frame, must win: got %v, %d, %v", points, depths, ok)
	}
}

// earlierReleaseRows are the rows the per-tick release wrote on a change of
// the increase the reads raised, byte for byte as its TransitionMessage wrote
// them under its constants: not read with and without a timeframe, not
// bearish, mixed on either read, and both bearish. Trades opened under that
// release still carry them, and the raise ones name amounts the way an
// opened row does; a raise comes last, the row a looser match would read a
// ladder's raise from.
var earlierReleaseRows = []string{
	"dynamic params: not read, configured rows",
	"dynamic params: 1D not read, configured rows",
	"dynamic params: 1D Super Guppy bullish, BMSB neutral: not bearish, configured rows",
	"dynamic params: 1D Super Guppy bullish, BMSB bearish: mixed, percentage +0.5 on every row",
	"dynamic params: 1D Super Guppy bearish, BMSB neutral: mixed, percentage +0.5 on every row",
	"dynamic params: 1D Super Guppy bearish, BMSB bearish: both bearish, percentage +0.5 and depths +1 on every row",
}

// Every other row is ignored: the per-tick rows an earlier release wrote
// under the same prefix, which were never an opened row — each alone and all
// of them together — rows of other gates naming a percentage, and a trade
// with no row at all. None of them raises a ladder's rows either: the ladder
// carrying them trades the very configured slice.
func TestOpenedRaiseIgnoresEveryOtherRow(t *testing.T) {
	cases := map[string][]string{
		"no row": nil,
		"the per-tick rows an earlier release wrote": earlierReleaseRows,
		"the prefix without the marker":              {"dynamic params: raised, percentage +0.4 on every row"},
		"the marker without the prefix":              {"opened raised, percentage +0.4 and depths +1 on every row"},
		"another gate's rows": {
			"Hold stopLoss: pattern: bull_flag found, preventing stopLoss",
			"Updated position to stopLoss from buy",
		},
	}
	for _, row := range earlierReleaseRows {
		cases["the earlier release's row "+row] = []string{row}
	}

	for name, messages := range cases {
		trade := withRows(flaggedTrade(), messages...)
		points, depths, ok := dynamicparams.OpenedRaise(trade)
		if ok || points != 0 || depths != 0 {
			t.Errorf("%s: OpenedRaise = %v, %d, %v, want no opened row", name, points, depths, ok)
		}
		stored := trade.StrategyPair.StrategySettings
		if rows, raised := dynamicparams.RaisedSettings(trade); raised || &rows[0] != &stored[0] {
			t.Errorf("%s: RaisedSettings raised %v, want the very configured slice", name, raised)
		}
	}
}

// An amount that is not a finite amount over zero adds nothing: the row is
// still the ladder's opened row — so no second one is written — and a part
// it cannot read leaves its field on the configured rows.
func TestOpenedRaiseReadsAnUnreadableAmountAsNothing(t *testing.T) {
	cases := []struct {
		name    string
		message string
		points  float64
		depths  int
	}{
		{"points that are not a number", "dynamic params: opened raised, percentage +x and depths +1 on every row", 0, 1},
		{"points under zero", "dynamic params: opened raised, percentage +-0.4 and depths +1 on every row", 0, 1},
		{"points that are not finite", "dynamic params: opened raised, percentage +Inf on every row", 0, 0},
		{"points that are NaN", "dynamic params: opened raised, percentage +NaN and depths +2 on every row", 0, 2},
		{"fractional depths", "dynamic params: opened raised, percentage +0.4 and depths +1.5 on every row", 0.4, 0},
		{"depths under zero", "dynamic params: opened raised, percentage +0.4 and depths +-1 on every row", 0.4, 0},
		{"no part at all", "dynamic params: opened raised, on every row", 0, 0},
	}

	for _, c := range cases {
		trade := withRows(flaggedTrade(), c.message)
		points, depths, ok := dynamicparams.OpenedRaise(trade)
		if !ok || points != c.points || depths != c.depths {
			t.Errorf("%s: OpenedRaise = %v, %d, %v, want %v, %d and the opened row", c.name, points, depths, ok, c.points, c.depths)
		}
	}
}
