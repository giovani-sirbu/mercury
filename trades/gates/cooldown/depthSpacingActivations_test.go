package cooldown

import (
	"math"
	"slices"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// sameSpacingState compares two folds field by field: the stamp by instant.
func sameSpacingState(a, b depthSpacingState) bool {
	return a.step == b.step && a.hold == b.hold && a.eligibleFrom.Equal(b.eligibleFrom)
}

// A depth activates once, at its earliest event, in whatever order the events
// arrive: agora loads them by id and sisyphus appends them, and a rehydrated
// trade is not trusted to keep either. A re-logged depth and two events of a
// depth at one stamp are the one activation, and depths never disturb each other.
func TestDepthSpacingActivationsAreTheEarliestEventOfEachDepth(t *testing.T) {
	start := testutil.At("09:00:00")
	written := []aggragates.TradesStrategyEvents{
		spacingEvent(1, start),
		spacingEvent(2, start.Add(time.Hour)),
		spacingEvent(1, start.Add(26*time.Hour)),
		spacingEvent(3, start.Add(2*time.Hour)),
		spacingEvent(3, start.Add(2*time.Hour)),
		spacingEvent(2, start.Add(25*time.Hour)),
	}
	want := map[int]time.Time{1: start, 2: start.Add(time.Hour), 3: start.Add(2 * time.Hour)}

	reversed := slices.Clone(written)
	slices.Reverse(reversed)
	newestFirst := slices.Clone(written)
	slices.SortFunc(newestFirst, func(a, b aggragates.TradesStrategyEvents) int {
		return b.CreatedAt.Compare(a.CreatedAt)
	})

	for name, order := range map[string][]aggragates.TradesStrategyEvents{"as written": written, "reversed": reversed, "newest first": newestFirst} {
		got := depthSpacingActivations(order)
		if len(got) != len(want) {
			t.Fatalf("%s: activations = %v, want %v", name, got, want)
		}
		for depth, at := range want {
			if !got[depth].Equal(at) {
				t.Errorf("%s: depth %d activated at %s, want its earliest event %s", name, depth, got[depth], at)
			}
		}
	}
}

// Only a stamped, readable event of the depth spacing gate of the cooldown flag,
// of the kind it writes and naming a depth, is an activation. Every other event
// is skipped on its own, never read as a depth of zero or guessed at; the
// depth guards also keep the fold off fills[depth-1] for a depth below the first.
func TestDepthSpacingActivationsSkipWhatCarriesNothingToMeasure(t *testing.T) {
	at := testutil.At("09:00:00")
	hold := DepthSpacingEvent{Event: gates.EventHeld, Depth: 3, Step: 2, Hold: time.Hour}
	filed := func(data []byte) aggragates.TradesStrategyEvents {
		return aggragates.TradesStrategyEvents{Param: aggragates.StrategyParamCooldown, Gate: GateDepthSpacing, Data: data, CreatedAt: at}
	}
	priority := DepthPriorityEvent{Event: gates.EventHeld, PrioritySymbol: "LINK/USDT", PriorityDepth: 5, PriorityMaxDepth: 8, Depth: 3, MaxDepth: 8}

	skipped := map[string]aggragates.TradesStrategyEvents{
		"no stamp":          spacingEvent(3, time.Time{}),
		"a torn document":   filed([]byte("{not json")),
		"no document":       filed(nil),
		"an empty document": aggragates.NewStrategyEvent(0, aggragates.StrategyParamCooldown, GateDepthSpacing, math.NaN(), at),
		"no depth":          NewDepthSpacingEvent(0, DepthSpacingEvent{Event: gates.EventHeld}, at),
		"a negative depth":  spacingEvent(-2, at),
		"no kind":           NewDepthSpacingEvent(0, DepthSpacingEvent{Depth: 3}, at),
		"an unknown kind":   NewDepthSpacingEvent(0, DepthSpacingEvent{Event: "released", Depth: 3}, at),
		"another param":     aggragates.NewStrategyEvent(0, aggragates.StrategyParamSmartTakeLoss, GateDepthSpacing, hold, at),
		"another gate":      aggragates.NewStrategyEvent(0, aggragates.StrategyParamCooldown, GateDepthPriority, hold, at),
		"a priority event":  NewDepthPriorityEvent(0, priority, at),
		"a first fill":      NewFirstFillEvent(0, FirstFillEvent{Event: FirstFillArmed, Price: 101}, at),
	}
	for name, event := range skipped {
		if got := depthSpacingActivations([]aggragates.TradesStrategyEvents{event}); len(got) != 0 {
			t.Errorf("%s: activations = %v, want none", name, got)
		}
	}
	if got := depthSpacingActivations([]aggragates.TradesStrategyEvents{spacingEvent(3, at)}); len(got) != 1 || !got[3].Equal(at) {
		t.Fatalf("control: activations = %v, want depth 3 at %s", got, at)
	}
}

// The fold answers a map it may be written into, the tick's own activation
// included, even for a trade with no events at all.
func TestDepthSpacingActivationsOfNoEventsIsAMapToWriteInto(t *testing.T) {
	for name, none := range map[string][]aggragates.TradesStrategyEvents{"nil": nil, "empty": {}} {
		got := depthSpacingActivations(none)
		if got == nil || len(got) != 0 {
			t.Fatalf("%s: activations = %#v, want an empty map", name, got)
		}
		got[1] = testutil.At("09:00:00")
	}
}

// The newest depth is activated at its first event, else at the tick: with its
// event the state stands still as the clock moves on; without it the tick is the
// activation, so the state counts one more inside the window and starts over a
// full window past the previous expiry.
func TestDepthSpacingTheNewestDepthActivatesAtItsFirstEventElseTheTick(t *testing.T) {
	start := testutil.At("09:00:00")
	second := start.Add(10 * time.Minute)
	placements := []time.Time{start, second}
	previous := spacingEvent(1, start.Add(time.Minute))
	tick := second.Add(time.Minute)

	both := []aggragates.TradesStrategyEvents{previous, spacingEvent(2, tick)}
	standing := spacingState(placements, both, tick)
	for _, later := range []time.Duration{time.Minute, time.Hour, 3 * DepthSpacingWindow} {
		if got := spacingState(placements, both, tick.Add(later)); !sameSpacingState(got, standing) {
			t.Errorf("%s later: state = %+v, want %+v: the activation is the event's stamp", later, got, standing)
		}
	}

	alone := []aggragates.TradesStrategyEvents{previous}
	if got := spacingState(placements, alone, tick); got.step != 2 {
		t.Errorf("the tick inside the window: step = %d, want 2", got.step)
	}
	pause := start.Add(depthSpacingHoldFor(1)).Add(DepthSpacingWindow)
	if got := spacingState(placements, alone, pause); got.step != 1 {
		t.Errorf("the tick a full window past the expiry: step = %d, want 1", got.step)
	}
}

// An event naming a depth the ladder has not filled is never counted: the fold
// reads the fill of each activation off the ladder, and a depth past it has none.
func TestDepthSpacingNeverCountsADepthTheLadderHasNotFilled(t *testing.T) {
	start := testutil.At("09:00:00")
	placements := []time.Time{start, start.Add(time.Hour)}
	now := start.Add(time.Hour + time.Minute)
	ahead := []aggragates.TradesStrategyEvents{spacingEvent(3, now), spacingEvent(7, now)}

	if got, want := spacingState(placements, ahead, now), spacingState(placements, nil, now); !sameSpacingState(got, want) {
		t.Fatalf("state = %+v, want %+v: a depth past the ladder counts for nothing", got, want)
	}
}

// The events are the only state: the row beside an event is the text an
// operator reads. The hold is written by the gate's own path, row and event at
// each tick; with the events dropped the rows it leaves — the exact text, stamped
// inside the window — are no activation, and the next depth holds as on a ladder
// that was never held. With the event kept it escalates.
func TestDepthSpacingHoldRowsAloneAreNoActivation(t *testing.T) {
	first := testutil.At("09:00:00")
	expiry := first.Add(depthSpacingHoldFor(1))
	trade := hbarTrade(1)
	trade.History = testutil.DepthTrade(first).History

	held, _ := heldAt(t, trade, first.Add(time.Minute))
	held.History = testutil.DepthTrade(first, expiry).History
	rowsOnly := held
	rowsOnly.StrategyEvents = nil
	never := rowsOnly
	never.Logs = nil
	if len(rowsOnly.Logs) != 1 {
		t.Fatalf("fixture drifted: the trade carries %d rows, want the hold's row", len(rowsOnly.Logs))
	}

	stepOf := func(trade aggragates.Trades) int {
		data, ok := DepthSpacingHold(tickOf(trade, expiry.Add(time.Minute)), "stopLoss").Data.(DepthSpacingEvent)
		if !ok {
			t.Fatal("the gate must hold the second depth")
		}
		return data.Step
	}
	if got, want := stepOf(rowsOnly), stepOf(never); got != want || got != 1 {
		t.Errorf("hold rows alone: step = %d, want %d, the first activation of a ladder never held", got, want)
	}
	if got := stepOf(held); got != 2 {
		t.Errorf("the row with its event: step = %d, want 2", got)
	}
}
