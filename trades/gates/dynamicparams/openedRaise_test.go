package dynamicparams_test

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/dynamicparams"
)

// withRows is the trade carrying these log rows, in order, and no strategy
// event: the rows are the operator's text, and the ladder's state comes from
// its events alone.
func withRows(trade aggragates.Trades, messages ...string) aggragates.Trades {
	trade.Logs = nil
	trade.StrategyEvents = nil
	for _, message := range messages {
		trade.Logs = append(trade.Logs, aggragates.TradesLogs{Message: message, Type: aggragates.LOG_INFO})
	}
	return trade
}

// withOpened is the trade carrying exactly the opened pair the engines write
// for a ladder that opens with these amounts: built with Opened.Rows, the
// function they write it with.
func withOpened(trade aggragates.Trades, points float64, depths int) aggragates.Trades {
	row, event := dynamicparams.Opened{Points: points, Depths: depths}.Rows(trade, 0, time.Time{})
	return aggragates.AppendStrategyRow(withRows(trade), row, event)
}

// openedEventOf is an opened event whose document is exactly this JSON, for
// the documents the writers never produce.
func openedEventOf(document string) aggragates.TradesStrategyEvents {
	return aggragates.NewStrategyEvent(
		0,
		aggragates.StrategyParamDynamicParams,
		dynamicparams.GateOpened,
		json.RawMessage(document),
		time.Time{},
	)
}

// Every amount form an opened pair carries reads back as the very amounts
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
		trade := withOpened(flaggedTrade(), c.points, c.depths)
		points, depths, ok := dynamicparams.OpenedRaise(trade)
		if !ok || points != c.points || depths != c.depths {
			t.Errorf("Opened{%v, %d} reads back as %v, %d, %v", c.points, c.depths, points, depths, ok)
		}
	}
}

// A retune of the constants reaches no ladder already open: an event written
// under other amounts than the constants name now reads back as its own
// amounts, and the ladder trades its stored rows raised by those — not by
// BearPercentagePoints and BearDepths.
func TestOpenedRaiseReadsTheEventNotTheConstants(t *testing.T) {
	writtenPoints := dynamicparams.BearPercentagePoints + 0.35
	writtenDepths := dynamicparams.BearDepths + 2
	trade := withOpened(flaggedTrade(), writtenPoints, writtenDepths)

	points, depths, ok := dynamicparams.OpenedRaise(trade)
	if !ok || points != writtenPoints || depths != writtenDepths {
		t.Fatalf("the event reads back as %v, %d, %v, want the amounts it was written with %v, %d", points, depths, ok, writtenPoints, writtenDepths)
	}

	rows, raised := dynamicparams.RaisedSettings(trade)
	want := dynamicparams.RaiseBy(threeRowLadder(), writtenPoints, writtenDepths)
	if !raised || !reflect.DeepEqual(rows, want) {
		t.Fatalf("the ladder trades %+v (raised %v), want its stored rows raised by the event's amounts %+v", rows, raised, want)
	}
}

// The events fold in slice order and the last opened event wins, whatever
// rows stand between them.
func TestOpenedRaiseFoldsTheEventsInOrder(t *testing.T) {
	trade := withRows(flaggedTrade(),
		"Updated position to buy from new",
		"Hold entry: smartTakeLoss: quiet slow decline, first fill held",
	)
	for _, amounts := range []dynamicparams.Opened{{Points: 0.4, Depths: 1}, {Points: 0.75}} {
		row, event := amounts.Rows(trade, 0, time.Time{})
		trade = aggragates.AppendStrategyRow(trade, row, event)
	}

	points, depths, ok := dynamicparams.OpenedRaise(trade)
	if !ok || points != 0.75 || depths != 0 {
		t.Fatalf("the last opened event must win: got %v, %d, %v", points, depths, ok)
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

// The rows are the operator's text and never the ladder's state: a trade with
// every row that ever named an opening — the opened row's own text, plain or
// under a hold frame, the per-tick rows an earlier release wrote, each alone
// and all of them together — but no opened event has not opened. None of them
// raises a ladder's rows either: the ladder carrying them trades the very
// configured slice.
func TestOpenedRaiseIsNotTheRowText(t *testing.T) {
	cases := map[string][]string{
		"no row":                nil,
		"the opened row's text": {dynamicparams.OpenedMessage(0.4, 1)},
		"the opened row's text under a hold frame":   {"Hold entry: " + dynamicparams.OpenedMessage(0.75, 0)},
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
			t.Errorf("%s: OpenedRaise = %v, %d, %v, want no opened event", name, points, depths, ok)
		}
		stored := trade.StrategyPair.StrategySettings
		if rows, raised := dynamicparams.RaisedSettings(trade); raised || &rows[0] != &stored[0] {
			t.Errorf("%s: RaisedSettings raised %v, want the very configured slice", name, raised)
		}
	}
}

// An amount that is not a finite amount over zero adds nothing: the event is
// still the ladder's opened event — so no second one is written — and a part
// it cannot read leaves its field on the configured rows. A document the fold
// cannot decode, one that is empty and one of another kind are not an opened
// event at all.
func TestOpenedRaiseReadsAnUnreadableAmountAsNothing(t *testing.T) {
	cases := []struct {
		name     string
		document string
		points   float64
		depths   int
		opened   bool
	}{
		{"points under zero", `{"event":"opened","points":-0.4,"depths":1}`, 0, 1, true},
		{"depths under zero", `{"event":"opened","points":0.4,"depths":-1}`, 0.4, 0, true},
		{"zero amounts", `{"event":"opened","points":0,"depths":0}`, 0, 0, true},
		{"no part at all", `{"event":"opened"}`, 0, 0, true},
		{"fractional depths, which no writer produces", `{"event":"opened","points":0.4,"depths":1.5}`, 0, 0, false},
		{"points that are not a number", `{"event":"opened","points":"x","depths":1}`, 0, 0, false},
		{"a document that is not an object", `[1,2]`, 0, 0, false},
		{"an empty document", `{}`, 0, 0, false},
		{"another kind", `{"event":"cancelled","points":0.4,"depths":1}`, 0, 0, false},
	}

	for _, c := range cases {
		trade := flaggedTrade()
		trade.StrategyEvents = []aggragates.TradesStrategyEvents{openedEventOf(c.document)}
		points, depths, ok := dynamicparams.OpenedRaise(trade)
		if ok != c.opened || points != c.points || depths != c.depths {
			t.Errorf("%s: OpenedRaise = %v, %d, %v, want %v, %d, %v", c.name, points, depths, ok, c.points, c.depths, c.opened)
		}
	}
}

// The one amount a document cannot carry, a NaN or an infinity, never reaches
// the fold as an amount: NewStrategyEvent stores an empty document for it,
// which is no opened event, so the ladder is judged again like any ladder that
// carries none.
func TestOpenedRaiseSkipsAnEventThatCouldNotBeMarshalled(t *testing.T) {
	for _, points := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		trade := flaggedTrade()
		event := aggragates.NewStrategyEvent(
			0,
			aggragates.StrategyParamDynamicParams,
			dynamicparams.GateOpened,
			dynamicparams.OpenedEvent{Event: dynamicparams.EventOpened, Points: points, Depths: 1},
			time.Time{},
		)
		if string(event.Data) != "{}" {
			t.Fatalf("points %v: data = %s, want the empty document", points, event.Data)
		}
		trade.StrategyEvents = []aggragates.TradesStrategyEvents{event}
		if got, depths, ok := dynamicparams.OpenedRaise(trade); ok || got != 0 || depths != 0 {
			t.Errorf("points %v: OpenedRaise = %v, %d, %v, want no opened event", points, got, depths, ok)
		}
	}
}

// Only the flag's own opened gate is read: an event of the same kind filed
// under another gate or another strategy param changes nothing.
func TestOpenedRaiseReadsOnlyItsOwnGate(t *testing.T) {
	document := json.RawMessage(`{"event":"opened","points":0.4,"depths":1}`)
	foreign := []aggragates.TradesStrategyEvents{
		aggragates.NewStrategyEvent(0, aggragates.StrategyParamDynamicParams, "other", document, time.Time{}),
		aggragates.NewStrategyEvent(0, aggragates.StrategyParamSmartTakeLoss, dynamicparams.GateOpened, document, time.Time{}),
		aggragates.NewStrategyEvent(0, aggragates.StrategyParamCooldown, dynamicparams.GateOpened, document, time.Time{}),
	}

	trade := flaggedTrade()
	trade.StrategyEvents = foreign
	if points, depths, ok := dynamicparams.OpenedRaise(trade); ok || points != 0 || depths != 0 {
		t.Fatalf("OpenedRaise = %v, %d, %v, want no opened event from another gate's events", points, depths, ok)
	}
}
