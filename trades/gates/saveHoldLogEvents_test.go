package gates_test

import (
	"bytes"
	"errors"
	"math"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// The strategy event SaveHoldLog writes beside a hold's row: an event exists
// exactly when its row does, on the same trade, stamped with the same tick, and
// the dedupe that reads the rows gates the two together.

// heldNote is the document of the holds below: the kind the gates' own hold
// events carry, and which of the holds wrote it.
type heldNote struct {
	Event string `json:"event"`
	Note  string `json:"note"`
}

var (
	// holdA and holdB own a strategy param, so they write a row and an event.
	holdA = gates.Hold{Reason: "cooldown: reason A", Param: aggragates.StrategyParamCooldown, Gate: "depthSpacing", Data: heldNote{Event: gates.EventHeld, Note: "A"}}
	holdB = gates.Hold{Reason: "smartTakeLoss: reason B", Param: aggragates.StrategyParamSmartTakeLoss, Gate: "entryHold", Data: heldNote{Event: gates.EventHeld, Note: "B"}}
	// holdC is a market family's hold (usePatterns, useAI): the row alone.
	holdC = gates.Hold{Reason: "pattern: reason C"}
)

const (
	eventsTradeID = 7
	// window is the dedupe's re-log window: how long a hold row keeps the same
	// reason from being written again. It is the package's unexported
	// holdRelogAfter, which a black-box test cannot name, so a retune of that
	// constant fails these cases until this one follows it. The offsets below
	// straddle it by a minute on either side.
	window = 24 * time.Hour
)

var eventsDay = time.Date(2026, time.September, 30, 9, 0, 0, 0, time.UTC)

// updates records what the chain's updateTrade was handed.
type updates struct {
	calls int
	last  events.Events
}

func (u *updates) updateTrade(event events.Events) (events.Events, error) {
	u.calls++
	u.last = event
	return event, nil
}

// holdTick is a trade of id eventsTradeID on a stopLoss tick, ready to be held:
// the engine's proposal is stopLoss at the tick price and the position it came
// from is buy at 100. A zero now leaves the tick clock unknown.
func holdTick(now time.Time, record *updates) events.Events {
	trade := testutil.NewHoldTrade("stopLoss", false)
	trade.ID = eventsTradeID
	event := events.Events{
		Trade:  trade,
		Events: map[string]func(events.Events) (events.Events, error){"updateTrade": record.updateTrade},
		Params: aggragates.Params{OldPosition: "buy", OldPositionPrice: 100, Percentage: -2.5, Quantity: 0.25},
	}
	if !now.IsZero() {
		event.Timestamp = now.UnixMilli()
	}
	return event
}

// hold is one tick of the chain: the engine proposes stopLoss at the tick price
// on the clock `at` (zero: no clock), and the gate holds it.
func hold(event events.Events, h gates.Hold, at time.Time) (events.Events, error) {
	event.Trade.PositionType = "stopLoss"
	event.Trade.PositionPrice = 98.5
	event.Timestamp = 0
	if !at.IsZero() {
		event.Timestamp = at.UnixMilli()
	}
	return gates.SaveHoldLog(event, "stopLoss", h)
}

// sameEvent compares two events field by field: the times as instants, the
// document byte for byte.
func sameEvent(a, b aggragates.TradesStrategyEvents) bool {
	return a.ID == b.ID && a.TradeID == b.TradeID && a.Param == b.Param && a.Gate == b.Gate &&
		bytes.Equal(a.Data, b.Data) && a.CreatedAt.Equal(b.CreatedAt) && a.CreatedAt.IsZero() == b.CreatedAt.IsZero()
}

// wantEvent is the event a hold must have written at the stamp: the very
// constructor every writer and fixture builds an event with.
func wantEvent(h gates.Hold, at time.Time) aggragates.TradesStrategyEvents {
	return aggragates.NewStrategyEvent(eventsTradeID, h.Param, h.Gate, h.Data, at)
}

// assertPaired is the pairing rule: the same trade and the very same stamp, so
// (TradeID, CreatedAt) finds a row's event.
func assertPaired(t *testing.T, row aggragates.TradesLogs, event aggragates.TradesStrategyEvents) {
	t.Helper()
	if event.TradeID != row.TradeID || !event.CreatedAt.Equal(row.CreatedAt) || event.CreatedAt.IsZero() != row.CreatedAt.IsZero() {
		t.Fatalf("row (trade %d, %v) and event (trade %d, %v) are not a pair", row.TradeID, row.CreatedAt, event.TradeID, event.CreatedAt)
	}
}

func assertCounts(t *testing.T, what string, trade aggragates.Trades, rows, stored int) {
	t.Helper()
	if len(trade.Logs) != rows || len(trade.StrategyEvents) != stored {
		t.Fatalf("%s: %d rows and %d events, want %d and %d", what, len(trade.Logs), len(trade.StrategyEvents), rows, stored)
	}
}

// A hold that names a strategy param writes its row and, beside it, the event:
// same trade, stamped with the tick the row is, carrying the hold's document
// as the constructor stores it. What the row and the event ride on is the one
// updateTrade call the chain persists, so the pair reaches storage together.
func TestSaveHoldLogWritesTheEventBesideItsRow(t *testing.T) {
	var record updates
	earlierRow := aggragates.TradesLogs{TradeID: eventsTradeID, Message: "Updated position to buy from new", Type: aggragates.LOG_INFO}
	earlier := aggragates.NewStrategyEvent(eventsTradeID, aggragates.StrategyParamDynamicParams, "opened", heldNote{Event: "opened"}, eventsDay.Add(-time.Hour))
	event := holdTick(eventsDay, &record)
	event.Trade.Logs = []aggragates.TradesLogs{earlierRow}
	event.Trade.StrategyEvents = []aggragates.TradesStrategyEvents{earlier}

	written, err := hold(event, holdA, eventsDay)

	if !errors.Is(err, events.ErrTradeHeld) || errors.Is(err, events.ErrHoldNotPersisted) {
		t.Fatalf("a hold that persisted stops the chain with ErrTradeHeld alone, got %v", err)
	}
	if !strings.Contains(err.Error(), holdA.Reason) {
		t.Fatalf("the error names the reason, got %q", err)
	}
	assertCounts(t, "the hold", written.Trade, 2, 2)

	row, stored := written.Trade.Logs[1], written.Trade.StrategyEvents[1]
	if row.Message != "Hold stopLoss: "+holdA.Reason || row.Type != aggragates.LOG_INFO || row.TradeID != eventsTradeID {
		t.Fatalf("row = %+v, want the INFO hold row of the trade", row)
	}
	if row.Price != 98.5 || row.Quantity != 0.25 || row.Percentage != -2.5 {
		t.Fatalf("row = %+v, want the tick price and the engine's quantity and percentage", row)
	}
	if !row.CreatedAt.Equal(eventsDay) || !row.UpdatedAt.Equal(eventsDay) || !stored.CreatedAt.Equal(eventsDay) {
		t.Fatalf("row stamps %v/%v and event stamp %v, want the tick %v on all three", row.CreatedAt, row.UpdatedAt, stored.CreatedAt, eventsDay)
	}
	assertPaired(t, row, stored)
	if !sameEvent(stored, wantEvent(holdA, eventsDay)) {
		t.Fatalf("event = %+v, want the constructor's event of the hold's data %+v", stored, wantEvent(holdA, eventsDay))
	}
	if stored.ID != 0 || stored.Kind() != gates.EventHeld {
		t.Fatalf("event = %+v, want no ID yet and the held kind", stored)
	}

	if written.Trade.Logs[0] != earlierRow || !sameEvent(written.Trade.StrategyEvents[0], earlier) {
		t.Fatal("what the trade already carried must stay, first")
	}
	if written.Trade.PositionType != "buy" || written.Trade.PositionPrice != 100 || !written.Params.PreventInfoLog {
		t.Fatalf("the proposal must be reset to buy at 100 with the info log prevented, got %q at %v", written.Trade.PositionType, written.Trade.PositionPrice)
	}

	if record.calls != 1 {
		t.Fatalf("updateTrade ran %d times, want the once", record.calls)
	}
	assertCounts(t, "what updateTrade was handed", record.last.Trade, 2, 2)
}

// A hold with no param — the market families' — writes its row and no event,
// and leaves the events the trade already carries as they were.
func TestSaveHoldLogWithoutAParamWritesTheRowAlone(t *testing.T) {
	var record updates
	earlier := wantEvent(holdA, eventsDay.Add(-time.Hour))
	event := holdTick(eventsDay, &record)
	event.Trade.StrategyEvents = []aggragates.TradesStrategyEvents{earlier}

	written, err := hold(event, holdC, eventsDay)

	if !errors.Is(err, events.ErrTradeHeld) || errors.Is(err, events.ErrHoldNotPersisted) {
		t.Fatalf("got %v", err)
	}
	assertCounts(t, "a hold without a param", written.Trade, 1, 1)
	if written.Trade.Logs[0].Message != "Hold stopLoss: "+holdC.Reason || !written.Trade.Logs[0].CreatedAt.Equal(eventsDay) {
		t.Fatalf("row = %+v, want the hold row at the tick", written.Trade.Logs[0])
	}
	if !sameEvent(written.Trade.StrategyEvents[0], earlier) {
		t.Fatal("the event the trade already carried must be untouched")
	}
	if record.calls != 1 {
		t.Fatalf("updateTrade ran %d times, want the once", record.calls)
	}
	assertCounts(t, "what updateTrade was handed", record.last.Trade, 1, 1)
}

// The same reason again inside the window is one hold: the dedupe reads the
// rows and gates the pair together, so a collapsed hold writes neither, never
// reaches updateTrade, and says so with ErrHoldNotPersisted — even when the
// repeat's document would have differed. A hold with no param collapses the
// same way.
func TestACollapsedHoldWritesNeitherTheRowNorTheEvent(t *testing.T) {
	changed := holdA
	changed.Data = heldNote{Event: gates.EventHeld, Note: "A, but another document"}

	for name, tc := range map[string]struct {
		first, again gates.Hold
		stored       int
	}{
		"the same hold":                 {holdA, holdA, 1},
		"the same reason, another data": {holdA, changed, 1},
		"a hold with no param":          {holdC, holdC, 0},
	} {
		for _, later := range []time.Duration{time.Minute, 15 * time.Minute, window - time.Minute} {
			var record updates
			event := holdTick(eventsDay, &record)
			event, _ = hold(event, tc.first, eventsDay)

			collapsed, err := hold(event, tc.again, eventsDay.Add(later))

			if !errors.Is(err, events.ErrTradeHeld) || !errors.Is(err, events.ErrHoldNotPersisted) {
				t.Fatalf("%s at +%v: a collapsed hold is a hold that persisted nothing, got %v", name, later, err)
			}
			assertCounts(t, name+" collapsed", collapsed.Trade, 1, tc.stored)
			if record.calls != 1 {
				t.Fatalf("%s at +%v: updateTrade ran %d times, a collapsed hold must not reach it", name, later, record.calls)
			}
			if tc.stored == 1 && !sameEvent(collapsed.Trade.StrategyEvents[0], wantEvent(tc.first, eventsDay)) {
				t.Fatalf("%s at +%v: the first hold's event must stand as written", name, later)
			}
		}
	}
}

// A different reason is a new hold, even on the same gate of the same param:
// both the row and the event are written, in order, each event carrying its
// own document and paired with its own row.
func TestADifferentReasonWritesBothTheRowAndTheEvent(t *testing.T) {
	sameGate := gates.Hold{Reason: "cooldown: reason A, next depth", Param: holdA.Param, Gate: holdA.Gate, Data: heldNote{Event: gates.EventHeld, Note: "A2"}}

	for name, second := range map[string]gates.Hold{"another gate": holdB, "the same gate": sameGate} {
		var record updates
		event := holdTick(eventsDay, &record)
		event, _ = hold(event, holdA, eventsDay)

		written, err := hold(event, second, eventsDay.Add(time.Minute))

		if !errors.Is(err, events.ErrTradeHeld) || errors.Is(err, events.ErrHoldNotPersisted) {
			t.Fatalf("%s: got %v", name, err)
		}
		assertCounts(t, name, written.Trade, 2, 2)
		if !sameEvent(written.Trade.StrategyEvents[0], wantEvent(holdA, eventsDay)) || !sameEvent(written.Trade.StrategyEvents[1], wantEvent(second, eventsDay.Add(time.Minute))) {
			t.Fatalf("%s: events = %+v, want A's then the second hold's", name, written.Trade.StrategyEvents)
		}
		for i := range written.Trade.Logs {
			assertPaired(t, written.Trade.Logs[i], written.Trade.StrategyEvents[i])
		}
		if record.calls != 2 {
			t.Fatalf("%s: updateTrade ran %d times, want one per persisted hold", name, record.calls)
		}
	}
}

// Past the window the same reason is written again, row and event both, each
// stamped with its own tick.
func TestPastTheWindowTheSameReasonWritesBothAgain(t *testing.T) {
	var record updates
	event := holdTick(eventsDay, &record)
	event, _ = hold(event, holdA, eventsDay)
	again := eventsDay.Add(window + time.Minute)

	written, err := hold(event, holdA, again)

	if !errors.Is(err, events.ErrTradeHeld) || errors.Is(err, events.ErrHoldNotPersisted) {
		t.Fatalf("a hold written again persists, got %v", err)
	}
	assertCounts(t, "past the window", written.Trade, 2, 2)
	if !written.Trade.StrategyEvents[0].CreatedAt.Equal(eventsDay) || !written.Trade.StrategyEvents[1].CreatedAt.Equal(again) {
		t.Fatalf("the events carry their own ticks, got %v and %v", written.Trade.StrategyEvents[0].CreatedAt, written.Trade.StrategyEvents[1].CreatedAt)
	}
	for i := range written.Trade.Logs {
		assertPaired(t, written.Trade.Logs[i], written.Trade.StrategyEvents[i])
	}
	if record.calls != 2 {
		t.Fatalf("updateTrade ran %d times, want 2", record.calls)
	}
}

// Two reasons that alternate tick by tick collapse to one pair per reason per
// window, rows and events alike, and each is written again once its own row is
// a window old.
func TestAlternatingReasonsWithinTheWindowWriteTwoPairs(t *testing.T) {
	var record updates
	event := holdTick(eventsDay, &record)
	step := func(h gates.Hold, offset time.Duration) {
		event, _ = hold(event, h, eventsDay.Add(offset))
	}

	step(holdA, 0)
	step(holdB, 15*time.Minute)
	step(holdA, 30*time.Minute)
	step(holdB, 45*time.Minute)
	assertCounts(t, "A-B-A-B within the window", event.Trade, 2, 2)

	step(holdA, window+time.Minute)
	assertCounts(t, "A past the window", event.Trade, 3, 3)
	step(holdB, window+2*time.Minute)
	assertCounts(t, "B, whose row is still inside the window, collapses", event.Trade, 3, 3)
	step(holdB, window+16*time.Minute)
	assertCounts(t, "B past the window of its own row", event.Trade, 4, 4)

	for i := range event.Trade.Logs {
		assertPaired(t, event.Trade.Logs[i], event.Trade.StrategyEvents[i])
	}
	if record.calls != 4 {
		t.Fatalf("updateTrade ran %d times, want one per pair written", record.calls)
	}
}

// With no clock the row and the event are both unstamped, and the reason is
// never written again: without a tick time the dedupe is the plain "same
// message anywhere" rule. An unknown clock does not expire a stamped row
// either.
func TestWithoutAClockBothAreUnstampedAndNeverRelogged(t *testing.T) {
	var record updates
	event := holdTick(time.Time{}, &record)

	event, _ = hold(event, holdA, time.Time{})
	assertCounts(t, "the first hold", event.Trade, 1, 1)
	if !event.Trade.Logs[0].CreatedAt.IsZero() || !event.Trade.StrategyEvents[0].CreatedAt.IsZero() {
		t.Fatalf("with no clock the row and the event are unstamped, got %v and %v", event.Trade.Logs[0].CreatedAt, event.Trade.StrategyEvents[0].CreatedAt)
	}
	assertPaired(t, event.Trade.Logs[0], event.Trade.StrategyEvents[0])

	collapsed, err := hold(event, holdA, time.Time{})
	if !errors.Is(err, events.ErrHoldNotPersisted) {
		t.Fatalf("the same reason again with no clock is never written again, got %v", err)
	}
	assertCounts(t, "the repeat", collapsed.Trade, 1, 1)

	event, _ = hold(event, holdB, time.Time{})
	assertCounts(t, "another reason", event.Trade, 2, 2)
	if !event.Trade.StrategyEvents[1].CreatedAt.IsZero() {
		t.Fatalf("the second event is unstamped too, got %v", event.Trade.StrategyEvents[1].CreatedAt)
	}

	// A stamped row met by a tick with no clock is never expired.
	var stamped updates
	dated := holdTick(eventsDay, &stamped)
	dated, _ = hold(dated, holdA, eventsDay)
	repeat, err := hold(dated, holdA, time.Time{})
	if !errors.Is(err, events.ErrHoldNotPersisted) {
		t.Fatalf("an unknown clock must not expire a stamped row, got %v", err)
	}
	assertCounts(t, "an unknown clock", repeat.Trade, 1, 1)
}

// An engine that gives no tick clock but keeps a simulated time on the trade
// (sisyphus keeps it in UpdatedAt) stamps the row and the event with that.
func TestTheTradesStampIsTheClockWhenTheEngineGivesNone(t *testing.T) {
	var record updates
	event := holdTick(time.Time{}, &record)
	event.Trade.UpdatedAt = eventsDay

	written, _ := hold(event, holdA, time.Time{})

	assertCounts(t, "the hold", written.Trade, 1, 1)
	if !written.Trade.Logs[0].CreatedAt.Equal(eventsDay) || !written.Trade.StrategyEvents[0].CreatedAt.Equal(eventsDay) {
		t.Fatalf("row %v and event %v, want the trade's stamp %v on both", written.Trade.Logs[0].CreatedAt, written.Trade.StrategyEvents[0].CreatedAt, eventsDay)
	}
}

// The hold's data never costs the hold its row: a document that cannot be
// marshalled leaves an inert event (no kind, so no fold acts on it) beside the
// row, and the chain stops and persists as it does for any other hold.
func TestAHoldWhoseDataCannotBeMarshalledStillWritesItsPair(t *testing.T) {
	var record updates
	unmarshalable := gates.Hold{
		Reason: "cooldown: reason with a NaN",
		Param:  aggragates.StrategyParamCooldown,
		Gate:   "depthSpacing",
		Data:   struct{ Price float64 }{math.NaN()},
	}

	written, err := hold(holdTick(eventsDay, &record), unmarshalable, eventsDay)

	if !errors.Is(err, events.ErrTradeHeld) || errors.Is(err, events.ErrHoldNotPersisted) {
		t.Fatalf("got %v", err)
	}
	assertCounts(t, "the hold", written.Trade, 1, 1)
	stored := written.Trade.StrategyEvents[0]
	if stored.Data == nil || stored.Kind() != "" || stored.Param != unmarshalable.Param || stored.Gate != unmarshalable.Gate {
		t.Fatalf("event = %+v, want an inert one filed under the hold's param and gate", stored)
	}
	assertPaired(t, written.Trade.Logs[0], stored)
	if record.calls != 1 {
		t.Fatalf("updateTrade ran %d times, want the once", record.calls)
	}
}

// The param decides the event and nothing else: the same hold with and without
// one returns the same error, runs updateTrade the same number of times,
// writes the same row and leaves the proposal reset the same way — on the
// first write, on the collapsed repeat and on the re-log a window later.
func TestAHoldWithAndWithoutAParamIsOtherwiseIdentical(t *testing.T) {
	with := gates.Hold{Reason: holdA.Reason, Param: holdA.Param, Gate: holdA.Gate, Data: holdA.Data}
	without := gates.Hold{Reason: holdA.Reason}

	var withRecord, withoutRecord updates
	withEvent := holdTick(eventsDay, &withRecord)
	withoutEvent := holdTick(eventsDay, &withoutRecord)

	for i, offset := range []time.Duration{0, time.Minute, window + time.Minute} {
		var withErr, withoutErr error
		withEvent, withErr = hold(withEvent, with, eventsDay.Add(offset))
		withoutEvent, withoutErr = hold(withoutEvent, without, eventsDay.Add(offset))

		if withErr == nil || withoutErr == nil || withErr.Error() != withoutErr.Error() {
			t.Fatalf("step %d: errors %v and %v, want the same", i, withErr, withoutErr)
		}
		for _, target := range []error{events.ErrTradeHeld, events.ErrHoldNotPersisted} {
			if errors.Is(withErr, target) != errors.Is(withoutErr, target) {
				t.Fatalf("step %d: the two disagree on %v", i, target)
			}
		}
		if withRecord.calls != withoutRecord.calls {
			t.Fatalf("step %d: updateTrade ran %d and %d times, want the same", i, withRecord.calls, withoutRecord.calls)
		}

		bare := withEvent.Trade
		bare.StrategyEvents = nil
		if !reflect.DeepEqual(bare, withoutEvent.Trade) || !reflect.DeepEqual(withEvent.Params, withoutEvent.Params) {
			t.Fatalf("step %d: the trades differ beyond the events:\n%+v\n%+v", i, bare, withoutEvent.Trade)
		}
	}
	assertCounts(t, "with a param", withEvent.Trade, 2, 2)
	assertCounts(t, "without a param", withoutEvent.Trade, 2, 0)
}

// The rule, over any sequence of holds: a random walk over two reasons that
// name a param and one that does not, on tick offsets straddling the window.
// Against a model of the dedupe that knows nothing of SaveHoldLog — a reason
// is written unless its row is under a window old — the trade ends with the
// rows the model expects, exactly as many events as param-bearing rows, and
// the i-th event paired with the i-th such row and carrying that hold's
// document; updateTrade ran once per hold written and never for a collapse.
func TestEventsStayPairedWithTheirRowsOverARandomWalk(t *testing.T) {
	holds := []gates.Hold{holdA, holdB, holdC}
	offsets := []time.Duration{time.Minute, 15 * time.Minute, window - time.Minute, window + time.Minute}

	type written struct {
		hold gates.Hold
		at   time.Time
	}
	for seed := uint64(1); seed <= 60; seed++ {
		r := rand.New(rand.NewPCG(seed, seed*7919))
		var record updates
		event := holdTick(eventsDay, &record)
		var model []written
		at := eventsDay

		for step := 0; step < 80; step++ {
			at = at.Add(offsets[r.IntN(len(offsets))])
			next := holds[r.IntN(len(holds))]

			collapses := false
			for _, row := range model {
				if row.hold.Reason == next.Reason && at.Sub(row.at) < window {
					collapses = true
				}
			}

			var err error
			event, err = hold(event, next, at)
			if !errors.Is(err, events.ErrTradeHeld) || errors.Is(err, events.ErrHoldNotPersisted) != collapses {
				t.Fatalf("seed %d step %d: %q at %v, model says collapsed=%v, got %v", seed, step, next.Reason, at, collapses, err)
			}
			if !collapses {
				model = append(model, written{next, at})
			}

			bearing := 0
			for _, row := range model {
				if row.hold.Param != "" {
					bearing++
				}
			}
			if len(event.Trade.Logs) != len(model) || len(event.Trade.StrategyEvents) != bearing {
				t.Fatalf("seed %d step %d: %d rows and %d events, model has %d rows and %d param-bearing", seed, step, len(event.Trade.Logs), len(event.Trade.StrategyEvents), len(model), bearing)
			}
			if record.calls != len(model) {
				t.Fatalf("seed %d step %d: updateTrade ran %d times, want %d", seed, step, record.calls, len(model))
			}
		}

		paired := 0
		for i, row := range model {
			if event.Trade.Logs[i].Message != "Hold stopLoss: "+row.hold.Reason || !event.Trade.Logs[i].CreatedAt.Equal(row.at) {
				t.Fatalf("seed %d: row %d = %q at %v, want %q at %v", seed, i, event.Trade.Logs[i].Message, event.Trade.Logs[i].CreatedAt, row.hold.Reason, row.at)
			}
			if row.hold.Param == "" {
				continue
			}
			stored := event.Trade.StrategyEvents[paired]
			paired++
			assertPaired(t, event.Trade.Logs[i], stored)
			if !sameEvent(stored, wantEvent(row.hold, row.at)) {
				t.Fatalf("seed %d: the event paired with row %d is %+v, want %+v", seed, i, stored, wantEvent(row.hold, row.at))
			}
		}
	}
}
