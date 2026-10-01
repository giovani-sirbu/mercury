package cooldown

import (
	"encoding/json"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// The first-fill fold on typed events, the state the gate rebuilds on every
// tick from the trade's cooldown/firstFill events and from nothing else. What
// the verdict, the hold and the doubling of the second depth read is this one
// record, so every case below is stated on the fold and, where the readers are
// exported, on them too.

// stampedFirstFill is a first-fill event of the kind at the price, stamped at,
// built with the constructor the gate writes it with. Reference and Anchor are
// set apart from the price on purpose: the fold reads the Price of the tick a
// row was written on and neither of them, so a fold that read one instead
// would land somewhere else.
func stampedFirstFill(kind string, price float64, at time.Time) aggragates.TradesStrategyEvents {
	return NewFirstFillEvent(7, FirstFillEvent{Event: kind, Price: price, Reference: price + 1000, Anchor: price + 2000}, at)
}

// rawFirstFill is a first-fill event whose document is exactly this text, for
// the documents the writers never produce, or that a database renders anew.
func rawFirstFill(document json.RawMessage, at time.Time) aggragates.TradesStrategyEvents {
	return aggragates.TradesStrategyEvents{
		TradeID:   7,
		Param:     aggragates.StrategyParamCooldown,
		Gate:      GateFirstFill,
		Data:      document,
		CreatedAt: at,
	}
}

// foldTrade is a spot trade under the Cooldown flag carrying exactly these
// events, in order and no log row.
func foldTrade(inverse bool, stored []aggragates.TradesStrategyEvents) aggragates.Trades {
	return withEvents(testutil.NewHoldTrade("buy", inverse), stored...)
}

// withOneFill is the trade with one executed entry at the price, on the side
// its ladder enters on.
func withOneFill(trade aggragates.Trades, price float64) aggragates.Trades {
	side := "BUY"
	if trade.Inverse {
		side = "SELL"
	}
	trade.History = []aggragates.TradesHistory{{Type: side, Quantity: 1, Price: price, OrderId: 1}}
	return trade
}

// sameRecord compares two records, the stamp as an instant.
func sameRecord(got, want firstFillRecord) bool {
	return got.activated == want.activated && got.reference == want.reference &&
		got.activatedAt.Equal(want.activatedAt) && got.activatedAt.IsZero() == want.activatedAt.IsZero() &&
		got.armed == want.armed && got.anchor == want.anchor && got.enteredAbove == want.enteredAbove
}

type foldCase struct {
	name    string
	inverse bool
	events  []aggragates.TradesStrategyEvents
	want    firstFillRecord
}

// foldCases are the fold's cases the gate's own tests do not state: what a
// long and an inverse ladder anchor on, the armed re-log that deepens the
// anchor, where an entered event may stand, and what carries no level.
func foldCases() []foldCase {
	start := testutil.At("09:00:00")
	hour := func(n int) time.Time { return start.Add(time.Duration(n) * time.Hour) }
	ff := stampedFirstFill
	in := func(stored ...aggragates.TradesStrategyEvents) []aggragates.TradesStrategyEvents { return stored }
	activated, armed, entered := FirstFillActivated, FirstFillArmed, FirstFillEntered

	return []foldCase{
		{"no events", false, nil, firstFillRecord{}},

		{"the first activated event in slice order is the reference, whatever the stamps", false,
			in(ff(activated, 100, hour(2)), ff(activated, 90, hour(0))),
			firstFillRecord{activated: true, reference: 100, activatedAt: hour(2)}},
		{"an activated event with no level is skipped and the next one activates", false,
			in(ff(activated, 0, hour(0)), ff(activated, -3, hour(1)), ff(activated, 100, hour(2))),
			firstFillRecord{activated: true, reference: 100, activatedAt: hour(2)}},
		{"an unstamped activation has no start", false,
			in(ff(activated, 100, time.Time{})),
			firstFillRecord{activated: true, reference: 100}},
		{"a stamped re-log after an unstamped activation gives it no start either", false,
			in(ff(activated, 100, time.Time{}), ff(activated, 101, hour(1))),
			firstFillRecord{activated: true, reference: 100}},

		{"an armed event before any activation still trails an anchor", false,
			in(ff(armed, 97.4, hour(1))),
			firstFillRecord{armed: true, anchor: 97.4}},
		{"the anchor is the lowest armed price on a long", false,
			in(ff(activated, 100, hour(0)), ff(armed, 97.4, hour(1)), ff(armed, 96.5, hour(2)), ff(armed, 96.9, hour(3))),
			firstFillRecord{activated: true, reference: 100, activatedAt: hour(0), armed: true, anchor: 96.5}},
		{"the anchor is the highest armed price on an inverse ladder", true,
			in(ff(activated, 100, hour(0)), ff(armed, 102.8, hour(1)), ff(armed, 103.8, hour(2)), ff(armed, 103.0, hour(3))),
			firstFillRecord{activated: true, reference: 100, activatedAt: hour(0), armed: true, anchor: 103.8}},

		{"an armed re-log deeper than the anchor moves it, inside the step too (long)", false,
			in(ff(activated, 100, hour(0)), ff(armed, 97.40, hour(1)), ff(armed, 97.10, hour(26))),
			firstFillRecord{activated: true, reference: 100, activatedAt: hour(0), armed: true, anchor: 97.10}},
		{"an armed re-log deeper than the anchor moves it, whichever came first (long)", false,
			in(ff(armed, 97.10, hour(1)), ff(armed, 97.40, hour(26))),
			firstFillRecord{armed: true, anchor: 97.10}},
		{"an armed re-log deeper than the anchor moves it, inside the step too (inverse)", true,
			in(ff(activated, 100, hour(0)), ff(armed, 102.8, hour(1)), ff(armed, 103.0, hour(26))),
			firstFillRecord{activated: true, reference: 100, activatedAt: hour(0), armed: true, anchor: 103.0}},
		{"an armed re-log shallower than the anchor does not move it (long)", false,
			in(ff(armed, 96.50, hour(1)), ff(armed, 96.60, hour(26))),
			firstFillRecord{armed: true, anchor: 96.50}},
		{"an armed re-log shallower than the anchor does not move it (inverse)", true,
			in(ff(armed, 103.8, hour(1)), ff(armed, 103.6, hour(26))),
			firstFillRecord{armed: true, anchor: 103.8}},
		{"an armed event with no level trails nothing", false,
			in(ff(armed, 0, hour(1)), ff(armed, -1, hour(2))),
			firstFillRecord{}},

		{"an entered event with no level still finishes the gate", false,
			in(ff(entered, 0, hour(1))),
			firstFillRecord{enteredAbove: true}},
		{"an entered event before the activation counts, and sets no reference", false,
			in(ff(entered, 102.6, hour(0)), ff(activated, 100, hour(1))),
			firstFillRecord{activated: true, reference: 100, activatedAt: hour(1), enteredAbove: true}},
		{"an entered event after the anchor counts", false,
			in(ff(activated, 100, hour(0)), ff(armed, 97.4, hour(1)), ff(entered, 102.6, hour(2))),
			firstFillRecord{activated: true, reference: 100, activatedAt: hour(0), armed: true, anchor: 97.4, enteredAbove: true}},

		{"a verdictHeld event is no activation and no anchor, whatever level it carries", false,
			in(ff(FirstFillVerdictHeld, 0, hour(0)), ff(FirstFillVerdictHeld, 100, hour(1))),
			firstFillRecord{}},
		{"an event of a kind the fold does not know, or of none, is inert", false,
			in(ff("mystery", 100, hour(0)), ff("", 100, hour(1))),
			firstFillRecord{}},
	}
}

// The fold's cases, on the record the gate rebuilds from them.
func TestFirstFillStateFoldsTheEvents(t *testing.T) {
	for _, tc := range foldCases() {
		if got := firstFillState(foldTrade(tc.inverse, tc.events)); !sameRecord(got, tc.want) {
			t.Errorf("%s: state = %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

// Only cooldown/firstFill events are the fold's. Events shaped exactly like its
// own, filed under another gate or another param, change nothing — alone or
// standing among the real ones; nor does a document the fold cannot read, an
// empty one, one of no kind, and the log rows the gate writes beside its
// events, whatever levels they carry.
func TestFirstFillStateIgnoresWhatIsNotItsEvents(t *testing.T) {
	start := testutil.At("09:00:00")
	real := []aggragates.TradesStrategyEvents{
		stampedFirstFill(FirstFillActivated, 100, start),
		stampedFirstFill(FirstFillArmed, 97.4, start.Add(time.Hour)),
	}
	wantReal := firstFillRecord{activated: true, reference: 100, activatedAt: start, armed: true, anchor: 97.4}
	around := func(noise ...aggragates.TradesStrategyEvents) []aggragates.TradesStrategyEvents {
		var stored []aggragates.TradesStrategyEvents
		stored = append(stored, noise...)
		stored = append(stored, real...)
		return append(stored, noise...)
	}

	shaped := []FirstFillEvent{
		{Event: FirstFillActivated, Price: 90},
		{Event: FirstFillArmed, Price: 80},
		{Event: FirstFillEntered, Price: 110},
	}
	for _, filed := range [][2]string{
		{aggragates.StrategyParamCooldown, GateDepthSpacing},
		{aggragates.StrategyParamCooldown, GateDepthPriority},
		{aggragates.StrategyParamCooldown, ""},
		{aggragates.StrategyParamSmartTakeLoss, GateFirstFill},
		{aggragates.StrategyParamDynamicParams, GateFirstFill},
		{"", GateFirstFill},
		{"", ""},
	} {
		var foreign []aggragates.TradesStrategyEvents
		for _, data := range shaped {
			foreign = append(foreign, aggragates.NewStrategyEvent(7, filed[0], filed[1], data, start))
		}
		if got := firstFillState(foldTrade(false, foreign)); !sameRecord(got, firstFillRecord{}) {
			t.Errorf("events of %q/%q: state = %+v, want none", filed[0], filed[1], got)
		}
		if got := firstFillState(foldTrade(false, around(foreign...))); !sameRecord(got, wantReal) {
			t.Errorf("events of %q/%q among the real ones: state = %+v, want %+v", filed[0], filed[1], got, wantReal)
		}
	}

	unreadable := map[string]json.RawMessage{
		"no document":                        nil,
		"an empty document":                  json.RawMessage(""),
		"text that is not JSON":              json.RawMessage(`not json`),
		"a truncated document":               json.RawMessage(`{"event":"entered",`),
		"an array":                           json.RawMessage(`[]`),
		"a string":                           json.RawMessage(`"entered"`),
		"null":                               json.RawMessage(`null`),
		"an empty object":                    json.RawMessage(`{}`),
		"a level and no kind":                json.RawMessage(`{"price":100}`),
		"an event key that is no string":     json.RawMessage(`{"event":5,"price":100}`),
		"a level that is no number":          json.RawMessage(`{"event":"activated","price":"100"}`),
		"an entered event with a bad level":  json.RawMessage(`{"event":"entered","price":"x"}`),
		"another gate's kind":                json.RawMessage(`{"event":"held","price":100}`),
		"the smart take loss's pending kind": json.RawMessage(`{"event":"pending","price":100}`),
	}
	for name, document := range unreadable {
		bad := rawFirstFill(document, start)
		if got := firstFillState(foldTrade(false, []aggragates.TradesStrategyEvents{bad})); !sameRecord(got, firstFillRecord{}) {
			t.Errorf("%s: state = %+v, want none", name, got)
		}
		if !FirstFillVerdictNeeded(foldTrade(false, []aggragates.TradesStrategyEvents{bad}), "new") {
			t.Errorf("%s: an event the fold cannot read is no activation, the verdict is still needed", name)
		}
		if got := firstFillState(foldTrade(false, around(bad))); !sameRecord(got, wantReal) {
			t.Errorf("%s among the real events: state = %+v, want %+v — the fold must go on past it", name, got, wantReal)
		}
	}

	// The rows are the operator's text: even carrying every level the events do,
	// at other levels, they are never read.
	trade := foldTrade(false, real)
	trade.Logs = []aggragates.TradesLogs{
		{Message: waitingRow, Price: 90, Type: aggragates.LOG_INFO},
		{Message: "Hold entry: " + armedReason("80.0000"), Price: 80, Type: aggragates.LOG_INFO},
		{Message: enteredRow, Price: 110, Type: aggragates.LOG_INFO},
	}
	if got := firstFillState(trade); !sameRecord(got, wantReal) {
		t.Errorf("rows beside the events: state = %+v, want the events' %+v", got, wantReal)
	}
}

// The documents a Postgres jsonb column hands back are the writers' documents
// rendered anew — keys in another order, spaces after the separators, numbers
// in another form. Each folds as the document the writer marshalled.
func TestFirstFillStateFoldsThePostgresRenderingOfItsDocuments(t *testing.T) {
	start := testutil.At("09:00:00")
	written := []aggragates.TradesStrategyEvents{
		NewFirstFillEvent(7, FirstFillEvent{Event: FirstFillActivated, Price: 100, Reference: 100}, start),
		NewFirstFillEvent(7, FirstFillEvent{Event: FirstFillArmed, Price: 97.4, Reference: 100, Anchor: 97.4}, start.Add(time.Hour)),
		NewFirstFillEvent(7, FirstFillEvent{Event: FirstFillEntered, Price: 102.64, Reference: 100}, start.Add(2*time.Hour)),
	}
	want := firstFillRecord{activated: true, reference: 100, activatedAt: start, armed: true, anchor: 97.4, enteredAbove: true}
	if got := firstFillState(foldTrade(false, written)); !sameRecord(got, want) {
		t.Fatalf("fixture drifted: the written events fold to %+v, want %+v", got, want)
	}

	for name, documents := range map[string][3]string{
		"jsonb's key order and spacing": {
			`{"event": "activated", "price": 100, "reference": 100}`,
			`{"price": 97.4, "anchor": 97.4, "event": "armed", "reference": 100}`,
			`{"price": 102.64, "event": "entered", "reference": 100}`,
		},
		"numbers in other forms": {
			`{"event":"activated","price":100.0}`,
			`{"event":"armed","price":9.74E+1,"anchor":97.40}`,
			`{"event":"entered","price":1.0264e2}`,
		},
		"whitespace everywhere": {
			"{\n  \"event\" : \"activated\" ,\n  \"price\" : 100\n}",
			"\t{ \"event\":\"armed\",\"price\":97.4 }\n",
			"{ \"event\" :\t\"entered\" }",
		},
	} {
		rendered := []aggragates.TradesStrategyEvents{
			rawFirstFill(json.RawMessage(documents[0]), written[0].CreatedAt),
			rawFirstFill(json.RawMessage(documents[1]), written[1].CreatedAt),
			rawFirstFill(json.RawMessage(documents[2]), written[2].CreatedAt),
		}
		if got := firstFillState(foldTrade(false, rendered)); !sameRecord(got, want) {
			t.Errorf("%s: state = %+v, want %+v", name, got, want)
		}
	}
}

// The armed re-log is the parity case: gates.SaveHoldLog writes a standing hold
// again once its row is a window old, at the tick of that later time, and that
// tick may sit below the anchor the row's text names while still inside the
// trail step — a price the gate itself would not have trailed to. The event
// carries that tick, the fold takes it as the anchor, and the gate then holds
// and releases from it: the hold names the deeper anchor, and a print that is a
// bounce off it — and not off the anchor the text named — fills the entry. A
// re-log above the anchor moves nothing.
func TestTheGateHoldsAndReleasesFromTheAnchorAnArmedReLogDeepened(t *testing.T) {
	start := testutil.At("09:00:00")
	activated := NewFirstFillEvent(7, FirstFillEvent{Event: FirstFillActivated, Price: 100, Reference: 100}, start)
	armed := func(price, anchor float64, minute int) aggragates.TradesStrategyEvents {
		return NewFirstFillEvent(7, FirstFillEvent{Event: FirstFillArmed, Price: price, Reference: 100, Anchor: anchor}, start.Add(time.Duration(minute)*time.Minute))
	}
	holding := func(inverse bool, stored ...aggragates.TradesStrategyEvents) events.Events {
		event := firstFillEvent(inverse, refused())
		event.Trade.StrategyEvents = stored
		return event
	}

	t.Run("long", func(t *testing.T) {
		levels, _ := firstFillLevelsFrom(testutil.NewHoldTrade("buy", false), aggragates.SideLong)
		if levels.below(97.10, levels.trail(97.40)) {
			t.Fatal("fixture drifted: 97.10 must sit inside the trail step of 97.40")
		}
		deepened := holding(false, activated, armed(97.40, 97.40, 1), armed(97.10, 97.40, 2))
		if _, reason := tick(t, deepened, 97.20, start.Add(3*time.Minute)); reason != armedReason("97.1000") {
			t.Fatalf("the hold must name the anchor the re-log deepened, got %q, want %q", reason, armedReason("97.1000"))
		}
		if _, reason := tick(t, deepened, 97.50, start.Add(3*time.Minute)); reason != "" {
			t.Fatalf("97.50 is a bounce off 97.10 and must fill the entry, got %q", reason)
		}

		shallower := holding(false, activated, armed(97.40, 97.40, 1), armed(97.60, 97.40, 2))
		if _, reason := tick(t, shallower, 97.30, start.Add(3*time.Minute)); reason != armedReason("97.4000") {
			t.Fatalf("a re-log above the anchor must leave it, got %q, want %q", reason, armedReason("97.4000"))
		}
		if _, reason := tick(t, shallower, 97.50, start.Add(3*time.Minute)); reason != armedReason("97.4000") {
			t.Fatalf("97.50 is no bounce off 97.40 yet, got %q", reason)
		}
	})

	t.Run("inverse", func(t *testing.T) {
		levels, _ := firstFillLevelsFrom(testutil.NewHoldTrade("buy", true), aggragates.SideShort)
		if levels.below(103.0, levels.trail(102.8)) {
			t.Fatal("fixture drifted: 103.0 must sit inside the trail step of 102.8")
		}
		deepened := holding(true, activated, armed(102.80, 102.80, 1), armed(103.00, 102.80, 2))
		if _, reason := tick(t, deepened, 102.90, start.Add(3*time.Minute)); !strings.Contains(reason, "high 103.0000") {
			t.Fatalf("the hold must name the high the re-log deepened, got %q", reason)
		}
		if _, reason := tick(t, deepened, 102.70, start.Add(3*time.Minute)); reason != "" {
			t.Fatalf("102.70 is a bounce off 103.0 and must fill the entry, got %q", reason)
		}

		shallower := holding(true, activated, armed(102.80, 102.80, 1), armed(102.60, 102.80, 2))
		if _, reason := tick(t, shallower, 102.70, start.Add(3*time.Minute)); !strings.Contains(reason, "high 102.8000") {
			t.Fatalf("a re-log below the high must leave it, got %q", reason)
		}
	})
}

// The cap is measured from the FIRST activated event's stamp: a stamp at all is
// needed (an unstamped activation, or an unknown clock, never expires — the
// gate fails closed, on a missing clock, on the cap), a re-log of the wait
// written later does not restart it, and a trade with no activation has no
// start to measure from.
func TestFirstFillExpiryIsMeasuredFromTheFirstActivationsStamp(t *testing.T) {
	start := testutil.At("09:00:00")
	// A cap of zero disables it, and then nothing ever expires.
	capOn := FirstFillMaxHold > 0
	never := time.Duration(1000) * time.Hour

	stamped := firstFillState(foldTrade(false, []aggragates.TradesStrategyEvents{stampedFirstFill(FirstFillActivated, 100, start)}))
	if !stamped.activatedAt.Equal(start) {
		t.Fatalf("activatedAt = %v, want the activation's stamp %v", stamped.activatedAt, start)
	}
	if firstFillExpired(stamped, start.Add(FirstFillMaxHold-time.Second)) {
		t.Error("a hold under the cap must not expire")
	}
	if got := firstFillExpired(stamped, start.Add(FirstFillMaxHold)); got != capOn {
		t.Errorf("at the cap expired = %v, want %v", got, capOn)
	}
	if firstFillExpired(stamped, time.Time{}) {
		t.Error("an unknown clock must never expire a hold")
	}

	relogged := firstFillState(foldTrade(false, []aggragates.TradesStrategyEvents{
		stampedFirstFill(FirstFillActivated, 100, start),
		stampedFirstFill(FirstFillActivated, 101.5, start.Add(FirstFillMaxHold-time.Minute)),
	}))
	if got := firstFillExpired(relogged, start.Add(FirstFillMaxHold)); got != capOn {
		t.Errorf("a re-log must not restart the cap: expired = %v, want %v", got, capOn)
	}

	// An activation with no stamp — and a stamped re-log after it — has no start.
	for name, stored := range map[string][]aggragates.TradesStrategyEvents{
		"an unstamped activation": {stampedFirstFill(FirstFillActivated, 100, time.Time{})},
		"an unstamped activation and a stamped re-log": {
			stampedFirstFill(FirstFillActivated, 100, time.Time{}),
			stampedFirstFill(FirstFillActivated, 101, start),
		},
	} {
		state := firstFillState(foldTrade(false, stored))
		if !state.activated || !state.activatedAt.IsZero() {
			t.Fatalf("%s: state = %+v, want activated with no start", name, state)
		}
		if firstFillExpired(state, start.Add(never)) {
			t.Errorf("%s: a hold with no start must never expire", name)
		}
	}

	// Armed and entered events alone are no activation to measure from.
	armedOnly := firstFillState(foldTrade(false, []aggragates.TradesStrategyEvents{
		stampedFirstFill(FirstFillArmed, 97.4, start),
		stampedFirstFill(FirstFillEntered, 102.6, start),
	}))
	if firstFillExpired(armedOnly, start.Add(never)) {
		t.Error("a trade that never activated has no hold to expire")
	}
}

// The verdict fetch and the doubling of the second depth read the fold the
// cases above state: the verdict is needed while the gate has neither activated
// nor entered, and the second depth doubles when it activated, entered above
// its reference and the one fill it holds landed at or beyond it (below it on
// an inverse ladder).
func TestTheVerdictAndTheDoublingReadTheFoldsRecord(t *testing.T) {
	for _, tc := range foldCases() {
		trade := foldTrade(tc.inverse, tc.events)

		if got, want := FirstFillVerdictNeeded(trade, "new"), !tc.want.activated && !tc.want.enteredAbove; got != want {
			t.Errorf("%s: FirstFillVerdictNeeded = %v, want %v", tc.name, got, want)
		}
		for _, fill := range []float64{99, 100, 101} {
			want := tc.want.activated && tc.want.enteredAbove && fill >= tc.want.reference
			if tc.inverse {
				want = tc.want.activated && tc.want.enteredAbove && fill <= tc.want.reference
			}
			if got := NextDepthDoubled(withOneFill(trade, fill)); got != want {
				t.Errorf("%s: NextDepthDoubled with one fill at %v = %v, want %v", tc.name, fill, got, want)
			}
		}
	}
}

// modelEvent is one generated event as the generator knows it: what it is filed
// under, the kind and level it carries, whether its document is unreadable.
type modelEvent struct {
	param, gate string
	garbage     bool
	kind        string
	price       float64
	at          time.Time
}

// modelOfFirstFill is the fold stated apart from its implementation, on the
// generator's own records rather than on documents: the first activated event
// with a level is the reference and its stamp the start, the anchor is the
// extreme armed level, the entered event anywhere is the release.
func modelOfFirstFill(inverse bool, generated []modelEvent) firstFillRecord {
	var want firstFillRecord
	var armedLevels []float64
	for _, event := range generated {
		if event.param != aggragates.StrategyParamCooldown || event.gate != GateFirstFill || event.garbage {
			continue
		}
		switch event.kind {
		case FirstFillEntered:
			want.enteredAbove = true
		case FirstFillActivated:
			if event.price > 0 && !want.activated {
				want.activated, want.reference, want.activatedAt = true, event.price, event.at
			}
		case FirstFillArmed:
			if event.price > 0 {
				armedLevels = append(armedLevels, event.price)
			}
		}
	}
	if len(armedLevels) > 0 {
		want.armed, want.anchor = true, armedLevels[0]
		for _, level := range armedLevels[1:] {
			if (!inverse && level < want.anchor) || (inverse && level > want.anchor) {
				want.anchor = level
			}
		}
	}
	return want
}

// Over any log of events — every kind, levels above and at and below zero,
// stamps present and absent and out of order, other gates' events, unreadable
// documents — on a long ladder and an inverse one, the fold equals the model,
// and the verdict fetch and the doubling agree with the model's record.
func TestFirstFillStateMatchesAnIndependentModelOnRandomEventLogs(t *testing.T) {
	start := testutil.At("09:00:00")
	kinds := []string{FirstFillActivated, FirstFillActivated, FirstFillArmed, FirstFillArmed, FirstFillArmed, FirstFillEntered, FirstFillVerdictHeld, "mystery", ""}
	foreign := [][2]string{
		{aggragates.StrategyParamCooldown, GateDepthSpacing},
		{aggragates.StrategyParamCooldown, GateDepthPriority},
		{aggragates.StrategyParamSmartTakeLoss, GateFirstFill},
		{aggragates.StrategyParamDynamicParams, GateFirstFill},
		{"", ""},
	}
	garbage := []string{`not json`, `[]`, `null`, `{}`, `"activated"`, `{"event":`, `{"event":"entered","price":"x"}`, `{"event":5}`}

	for seed := uint64(1); seed <= 300; seed++ {
		r := rand.New(rand.NewPCG(seed, seed*104729))
		inverse := r.IntN(2) == 0
		var generated []modelEvent
		var stored []aggragates.TradesStrategyEvents

		for i, count := 0, r.IntN(24); i < count; i++ {
			event := modelEvent{param: aggragates.StrategyParamCooldown, gate: GateFirstFill, kind: kinds[r.IntN(len(kinds))]}
			switch r.IntN(10) {
			case 0:
				other := foreign[r.IntN(len(foreign))]
				event.param, event.gate = other[0], other[1]
			case 1:
				event.garbage = true
			}
			switch r.IntN(8) {
			case 0:
				event.price = 0
			case 1:
				event.price = -2.5
			default:
				event.price = 90 + float64(r.IntN(2001))/100
			}
			if r.IntN(5) > 0 {
				event.at = start.Add(time.Duration(r.IntN(3000)) * time.Minute)
			}

			row := NewFirstFillEvent(7, FirstFillEvent{Event: event.kind, Price: event.price}, event.at)
			row.Param, row.Gate = event.param, event.gate
			if event.garbage {
				row.Data = json.RawMessage(garbage[r.IntN(len(garbage))])
			}
			generated = append(generated, event)
			stored = append(stored, row)
		}

		want := modelOfFirstFill(inverse, generated)
		trade := foldTrade(inverse, stored)
		if got := firstFillState(trade); !sameRecord(got, want) {
			t.Fatalf("seed %d (inverse %v): state = %+v, want %+v from %+v", seed, inverse, got, want, generated)
		}
		if got, need := FirstFillVerdictNeeded(trade, "new"), !want.activated && !want.enteredAbove; got != need {
			t.Fatalf("seed %d: FirstFillVerdictNeeded = %v, want %v", seed, got, need)
		}
		for _, fill := range []float64{want.reference - 0.5, want.reference, want.reference + 0.5, 99, 101} {
			need := want.activated && want.enteredAbove && fill >= want.reference
			if inverse {
				need = want.activated && want.enteredAbove && fill <= want.reference
			}
			if got := NextDepthDoubled(withOneFill(trade, fill)); got != need {
				t.Fatalf("seed %d (inverse %v): NextDepthDoubled with one fill at %v = %v, want %v (reference %v)", seed, inverse, fill, got, need, want.reference)
			}
		}
	}
}
