package cooldown

import (
	"errors"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// The expectations below are the firstFillLevels formulas (firstFillLevels.go)
// evaluated on the fixture ladder row from a reference of 100, printed at the
// fixture pair's price precision:
//
//	up      enters at market from it up
//	arm     arms from it down
//	bounce  fills strictly above it
//	trail   a new low strictly below it
//
// On the inverse ladder every level mirrors, with the anchor trailing the
// high.
const (
	waitingReason = "cooldown: trying to get a better entry price: reference 100.0000, enters above 102.5641 or below 97.4184 after a bounce"
	waitingRow    = "Hold entry: " + waitingReason
	enteredRow    = "cooldown: entered above the reference 100.0000, next depth arms at double percentage"
)

func armedReason(low string) string {
	return "cooldown: trying to get a better entry price, armed below 97.4184: low " + low + ", enters on a 0.15% bounce"
}

// refused is the sophos read of a local top: neither side may enter.
func refused() aggragates.CoolDownIndicators {
	return aggragates.CoolDownIndicators{HasFirstFillVerdict: true}
}

func allowed() aggragates.CoolDownIndicators {
	return aggragates.CoolDownIndicators{HasFirstFillVerdict: true, AllowLongEntry: true, AllowShortEntry: true}
}

// firstFillEvent is a new spot trade under the Cooldown flag on a first-buy
// tick, before the engine has set the tick price.
func firstFillEvent(inverse bool, verdict aggragates.CoolDownIndicators) events.Events {
	trade := testutil.NewHoldTrade("buy", inverse)
	trade.ID = 7
	trade.Strategy.Params.Cooldown = true
	return events.Events{
		Trade: trade,
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": testutil.NopUpdateTrade,
		},
		Params: aggragates.Params{OldPosition: "new", CoolDownIndicators: verdict},
	}
}

// tick runs one print through the gate the way shouldHoldEntry does: the
// engine sets PositionPrice to the print, the gate answers, and a hold goes
// through gates.SaveHoldLog, which writes or collapses the row. The event
// that comes back is what the next tick starts from.
func tick(t *testing.T, event events.Events, price float64, at time.Time) (events.Events, string) {
	t.Helper()
	event.Trade.PositionPrice = price
	event.Timestamp = at.UnixMilli()
	side := aggragates.EntrySide(event.Trade, event.Params.AIIndicators)
	event, reason := FirstFillHold(event, side)
	if reason == "" {
		return event, ""
	}
	held, err := gates.SaveHoldLog(event, "entry", reason)
	if !errors.Is(err, events.ErrTradeHeld) {
		t.Fatalf("a hold must stop the chain, got %v", err)
	}
	return held, reason
}

// ticks runs a sequence of prints a minute apart from the given clock and
// returns the event the last one left behind.
func ticks(t *testing.T, event events.Events, from time.Time, prices ...float64) events.Events {
	t.Helper()
	for i, price := range prices {
		event, _ = tick(t, event, price, from.Add(time.Duration(i)*time.Minute))
	}
	return event
}

func rows(event events.Events) []string {
	out := make([]string, 0, len(event.Trade.Logs))
	for _, row := range event.Trade.Logs {
		out = append(out, row.Message)
	}
	return out
}

// A refused verdict activates the hold at the tick price: one row, its Price
// the reference, and PositionPrice back at 0 — a positive one reads as
// "entered" to the engines.
func TestFirstFillHoldActivatesAtTheTickPrice(t *testing.T) {
	held, reason := tick(t, firstFillEvent(false, refused()), 100, testutil.At("09:00:00"))
	if reason != waitingReason {
		t.Fatalf("reason = %q, want %q", reason, waitingReason)
	}
	if len(held.Trade.Logs) != 1 || held.Trade.Logs[0].Message != waitingRow {
		t.Fatalf("rows = %q, want the one waiting row", rows(held))
	}
	row := held.Trade.Logs[0]
	if row.Price != 100 || row.Type != aggragates.LOG_INFO || row.TradeID != 7 {
		t.Fatalf("row = %+v, want Price 100, INFO, trade 7", row)
	}
	if held.Trade.PositionPrice != 0 {
		t.Fatalf("PositionPrice = %v, must stay 0 on a held new trade", held.Trade.PositionPrice)
	}
	if FirstFillVerdictNeeded(held.Trade, "new") {
		t.Fatal("once the hold stands the verdict is not fetched again")
	}
}

// The same print again, and any print between the levels, is the same hold:
// nothing more is written.
func TestFirstFillHoldCollapsesTheStandingWait(t *testing.T) {
	held := ticks(t, firstFillEvent(false, refused()), testutil.At("09:00:00"), 100, 100, 101.9, 97.5, 100)
	if len(held.Trade.Logs) != 1 {
		t.Fatalf("a standing wait must not write a row per tick, got %q", rows(held))
	}
}

// From up(R) on the hold was wrong: the entry goes to market and the entered
// row is written exactly once — the chain may still stop on funds and bring
// the same release back with the row already on the trade.
func TestFirstFillHoldEntersAboveTheReferenceOnce(t *testing.T) {
	held := ticks(t, firstFillEvent(false, refused()), testutil.At("09:00:00"), 100)

	released, reason := tick(t, held, 102.57, testutil.At("10:00:00"))
	if reason != "" {
		t.Fatalf("above up(R) the entry must proceed, got %q", reason)
	}
	if len(released.Trade.Logs) != 2 || released.Trade.Logs[1].Message != enteredRow {
		t.Fatalf("rows = %q, want the entered row appended", rows(released))
	}
	row := released.Trade.Logs[1]
	if row.Price != 102.57 || row.Type != aggragates.LOG_INFO || row.TradeID != 7 || !row.CreatedAt.Equal(testutil.At("10:00:00")) {
		t.Fatalf("entered row = %+v, want the tick price and the tick clock", row)
	}

	again, reason := tick(t, released, 103, testutil.At("10:01:00"))
	if reason != "" || len(again.Trade.Logs) != 2 {
		t.Fatalf("a release with the row present must add nothing, got %q %q", reason, rows(again))
	}
	if FirstFillVerdictNeeded(again.Trade, "new") {
		t.Fatal("after the release the verdict is not needed")
	}

	// Exactly at up(R) counts: the ladder's own comparison is inclusive. The
	// level is computed in float64 at run time, as the gate computes it — a
	// constant expression is rounded once and can land one ulp away.
	reference, percentage := 100.0, 2.5
	at, reason := tick(t, held, reference/(1-percentage/100), testutil.At("10:00:00"))
	if reason != "" || len(at.Trade.Logs) != 2 {
		t.Fatalf("at up(R) the entry must proceed, got %q %q", reason, rows(at))
	}
}

// At or below arm(R) the entry arms like a depth: a new row at the tick
// price, which is the anchor from here on.
func TestFirstFillHoldArmsAtTheLadderStep(t *testing.T) {
	held := ticks(t, firstFillEvent(false, refused()), testutil.At("09:00:00"), 100)

	armed, reason := tick(t, held, 97.40, testutil.At("10:00:00"))
	if reason != armedReason("97.4000") {
		t.Fatalf("reason = %q, want %q", reason, armedReason("97.4000"))
	}
	if len(armed.Trade.Logs) != 2 || armed.Trade.Logs[1].Price != 97.40 {
		t.Fatalf("rows = %q, want an armed row at the tick price", rows(armed))
	}
	if armed.Trade.PositionPrice != 0 {
		t.Fatalf("PositionPrice = %v, must stay 0 while armed", armed.Trade.PositionPrice)
	}
}

// The anchor follows the low by whole (tr + t) steps and never rises: a
// print inside the step collapses into the standing row, a print a full step
// lower is a new row, and a print back up short of the bounce keeps the low.
func TestFirstFillHoldTrailsTheLowByFullSteps(t *testing.T) {
	armed := ticks(t, firstFillEvent(false, refused()), testutil.At("09:00:00"), 100, 97.40)

	// Both prints sit inside the trail step of the anchor: the same row.
	inside := ticks(t, armed, testutil.At("09:10:00"), 97.0, 96.60)
	if len(inside.Trade.Logs) != 2 {
		t.Fatalf("a print inside the step must not move the anchor, got %q", rows(inside))
	}
	lower, reason := tick(t, inside, 96.50, testutil.At("09:20:00"))
	if reason != armedReason("96.5000") || len(lower.Trade.Logs) != 3 || lower.Trade.Logs[2].Price != 96.50 {
		t.Fatalf("a full step lower must write a new row at the new low, got %q %q", reason, rows(lower))
	}
	// Back up, but short of the bounce off the new low: the low stands.
	back, reason := tick(t, lower, 96.60, testutil.At("09:21:00"))
	if reason != armedReason("96.5000") || len(back.Trade.Logs) != 3 {
		t.Fatalf("a print short of the bounce must keep the low, got %q %q", reason, rows(back))
	}
}

// A t bounce off the low fills the entry, exactly as STOPLOSS_TO_BUY does.
func TestFirstFillHoldFillsOnTheBounce(t *testing.T) {
	lower := ticks(t, firstFillEvent(false, refused()), testutil.At("09:00:00"), 100, 97.40, 96.50)

	released, reason := tick(t, lower, 96.65, testutil.At("09:30:00"))
	if reason != "" || len(released.Trade.Logs) != 3 {
		t.Fatalf("above bounce(A) the entry must proceed with no row, got %q %q", reason, rows(released))
	}
	// Exactly at the bounce is not yet a bounce: the ladder's `>` is strict.
	// Computed in float64 at run time, as the gate computes it.
	anchor, tolerance := 96.50, 0.15
	if _, reason := tick(t, lower, anchor/(1-tolerance/100), testutil.At("09:30:00")); reason == "" {
		t.Fatal("exactly at bounce(A) the entry must still be held")
	}
}

// gates.SaveHoldLog writes a standing row again once it is older than its
// re-log window, at the price of that later tick. The reference is the FIRST
// waiting row and the anchor the lowest armed row, so a re-log moves neither.
//
// Asserted on firstFillState rather than through FirstFillHold: while
// FirstFillMaxHold is shorter than holdRelogAfter no first-fill hold survives
// long enough to BE re-logged, so driving the gate past the window only ever
// proves the cap. The rule the state owns still has to hold — the rows arrive
// from the engines, and a wider cap (or none) puts them back in front of it.
func TestFirstFillStateRelogKeepsTheReferenceAndTheLow(t *testing.T) {
	day := testutil.At("09:00:00")
	trade := aggragates.Trades{Logs: []aggragates.TradesLogs{
		{Message: "Hold entry: " + FirstFillWaitingPrefix + "100", Price: 100, CreatedAt: day},
		// The re-log of that same wait, past the window, at a later price.
		{Message: "Hold entry: " + FirstFillWaitingPrefix + "100", Price: 101.5, CreatedAt: day.Add(25 * time.Hour)},
		{Message: "Hold entry: " + FirstFillArmedPrefix + "97.40", Price: 97.40, CreatedAt: day.Add(26 * time.Hour)},
		{Message: "Hold entry: " + FirstFillArmedPrefix + "96.50", Price: 96.50, CreatedAt: day.Add(27 * time.Hour)},
		// The re-log of the armed row, above the low it trails.
		{Message: "Hold entry: " + FirstFillArmedPrefix + "96.50", Price: 96.60, CreatedAt: day.Add(51 * time.Hour)},
	}}

	state := firstFillState(trade)
	if !state.activated || state.reference != 100 {
		t.Fatalf("reference = %v, want the first waiting row's 100", state.reference)
	}
	if !state.armed || state.anchor != 96.50 {
		t.Fatalf("anchor = %v, want the lowest armed row's 96.50", state.anchor)
	}
	if !state.activatedAt.Equal(day) {
		t.Fatalf("activatedAt = %s, want the first waiting row's stamp %s", state.activatedAt, day)
	}
}

// The cap: past FirstFillMaxHold the entry goes through at the tick price,
// wherever it sits inside the band that was holding it. The shape it answers
// to is a market that drifts sideways inside the band and never leaves it.
func TestFirstFillHoldExpiresAtTheCap(t *testing.T) {
	start := testutil.At("09:00:00")
	held := ticks(t, firstFillEvent(false, refused()), start, 100)

	// A tick inside the band one second under the cap is still held.
	if _, reason := tick(t, held, 100.5, start.Add(FirstFillMaxHold-time.Second)); reason == "" {
		t.Fatal("inside the band and under the cap the entry must still be held")
	}
	// At the cap it is released, and no row is written: in particular not the
	// entered row, which would ask the second depth to arm at 2p.
	released, reason := tick(t, held, 100.5, start.Add(FirstFillMaxHold))
	if reason != "" {
		t.Fatalf("at the cap the entry must proceed, got %q", reason)
	}
	if len(released.Trade.Logs) != 1 {
		t.Fatalf("the release must write no row, got %q", rows(released))
	}
	if NextDepthDoubled(released.Trade) {
		t.Fatal("a hold that ran out of time made no wrong call: the next depth must not double")
	}
}

// An unknown clock never expires: the cap must not switch the whole gate off
// on an engine that does not stamp its rows.
func TestFirstFillHoldNeverExpiresOnUnknownClocks(t *testing.T) {
	start := testutil.At("09:00:00")
	held := ticks(t, firstFillEvent(false, refused()), start, 100)

	unstamped := held
	unstamped.Trade.Logs = append([]aggragates.TradesLogs(nil), held.Trade.Logs...)
	unstamped.Trade.Logs[0].CreatedAt = time.Time{}
	if _, reason := tick(t, unstamped, 100.5, start.Add(30*24*time.Hour)); reason == "" {
		t.Fatal("an unstamped hold row must keep holding, not expire")
	}

	noTick := held
	noTick.Trade.PositionPrice = 100.5
	noTick.Timestamp = 0
	side := aggragates.EntrySide(noTick.Trade, noTick.Params.AIIndicators)
	if _, reason := FirstFillHold(noTick, side); reason == "" {
		t.Fatal("a zero tick clock must keep holding, not expire")
	}
}

// The verdict starts the hold and nothing else: missing or allowing, no
// hold; once the hold stands, it is not consulted again.
func TestFirstFillHoldNeedsTheVerdictOnlyToActivate(t *testing.T) {
	for name, verdict := range map[string]aggragates.CoolDownIndicators{"missing": {}, "allowing": allowed()} {
		for _, inverse := range []bool{false, true} {
			open, reason := tick(t, firstFillEvent(inverse, verdict), 100, testutil.At("09:00:00"))
			if reason != "" || len(open.Trade.Logs) != 0 {
				t.Fatalf("a %s verdict must not hold (inverse %v), got %q %q", name, inverse, reason, rows(open))
			}
		}
	}

	held := ticks(t, firstFillEvent(false, refused()), testutil.At("09:00:00"), 100)
	held.Params.CoolDownIndicators = aggragates.CoolDownIndicators{}
	if _, reason := tick(t, held, 100, testutil.At("09:05:00")); reason != waitingReason {
		t.Fatalf("a standing hold needs no verdict, got %q", reason)
	}
}

// sophos scores the two directions separately, and the side is the one the
// entry would take — not the Inverse flag, which is never set on futures.
func TestFirstFillHoldJudgesTheSideNotTheFlag(t *testing.T) {
	cheapShort := aggragates.CoolDownIndicators{HasFirstFillVerdict: true, AllowShortEntry: true}
	event := firstFillEvent(false, cheapShort)
	event.Trade.PositionPrice = 100

	if _, reason := FirstFillHold(event, aggragates.SideShort); reason != "" {
		t.Errorf("a short entry at an allowed short location must open, got %q", reason)
	}
	if _, reason := FirstFillHold(event, aggragates.SideLong); reason == "" {
		t.Error("a long entry at a refused long location must be held")
	}

	// No side is nothing to judge, on spot as on futures: the verdict may
	// refuse both directions and the gate still writes nothing.
	event.Params.CoolDownIndicators = refused()
	if got, reason := FirstFillHold(event, ""); reason != "" || len(got.Trade.Logs) != 0 {
		t.Errorf("an entry with no side must not be held on spot, got %q %q", reason, rows(got))
	}
}

// Fail open: a hold that cannot be priced never activates and never panics.
func TestFirstFillHoldFailsOpenWhenItCannotPriceTheHold(t *testing.T) {
	cases := map[string]func(event *events.Events){
		"no settings row":       func(e *events.Events) { e.Trade.StrategyPair.StrategySettings = nil },
		"zero percentage":       func(e *events.Events) { e.Trade.StrategyPair.StrategySettings[0].Percentage = 0 },
		"step of 100% or more":  func(e *events.Events) { e.Trade.StrategyPair.StrategySettings[0].Percentage = 99.9 },
		"trail of 100% or more": func(e *events.Events) { e.Trade.StrategyPair.StrategySettings[0].TrailingTakeProfit = 99.9 },
		"unknown tick":          func(e *events.Events) {},
	}
	for name, breakIt := range cases {
		event := firstFillEvent(false, refused())
		breakIt(&event)
		price := 100.0
		if name == "unknown tick" {
			price = 0
		}
		open, reason := tick(t, event, price, testutil.At("09:00:00"))
		if reason != "" || len(open.Trade.Logs) != 0 {
			t.Fatalf("%s: must fail open, got %q %q", name, reason, rows(open))
		}
	}
}

// The inverse ladder sells first, so every level mirrors: up below the
// reference, arm above it, the anchor trailing the high, the bounce down.
func TestFirstFillHoldMirrorsTheInverseLadder(t *testing.T) {
	const (
		waiting = "cooldown: trying to get a better entry price: reference 100.0000, enters below 97.5610 or above 102.7221 after a bounce (inverse)"
		entered = "cooldown: entered below the reference 100.0000, next depth arms at double percentage"
	)
	armedAt := func(high string) string {
		return "cooldown: trying to get a better entry price, armed above 102.7221: high " + high + ", enters on a 0.15% bounce (inverse)"
	}

	held, reason := tick(t, firstFillEvent(true, refused()), 100, testutil.At("09:00:00"))
	if reason != waiting {
		t.Fatalf("reason = %q, want %q", reason, waiting)
	}

	released, reason := tick(t, held, 97.5, testutil.At("09:01:00"))
	if reason != "" || len(released.Trade.Logs) != 2 || released.Trade.Logs[1].Message != entered {
		t.Fatalf("below up(R) the inverse entry must proceed with the entered row, got %q %q", reason, rows(released))
	}

	armed, reason := tick(t, held, 102.8, testutil.At("09:01:00"))
	if reason != armedAt("102.8000") || len(armed.Trade.Logs) != 2 || armed.Trade.Logs[1].Price != 102.8 {
		t.Fatalf("above arm(R) the inverse entry must arm at the tick, got %q %q", reason, rows(armed))
	}
	// Inside the trail step of the anchor the same row, past it a new high.
	inside := ticks(t, armed, testutil.At("09:02:00"), 103.5, 103.0)
	if len(inside.Trade.Logs) != 2 {
		t.Fatalf("a print inside the step must not move the anchor, got %q", rows(inside))
	}
	higher, reason := tick(t, inside, 103.8, testutil.At("09:10:00"))
	if reason != armedAt("103.8000") || len(higher.Trade.Logs) != 3 {
		t.Fatalf("a full step higher must write a new row at the new high, got %q %q", reason, rows(higher))
	}
	// Short of the bounce off the high it stands, past it the entry fills.
	if _, reason := tick(t, higher, 103.7, testutil.At("09:11:00")); reason != armedAt("103.8000") {
		t.Fatalf("a print short of the bounce must keep the high, got %q", reason)
	}
	if _, reason := tick(t, higher, 103.6, testutil.At("09:12:00")); reason != "" {
		t.Fatalf("below bounce(A) the inverse entry must proceed, got %q", reason)
	}
}

// A futures entry keeps the verdict-only gate: no ladder rule, no rows, no
// price read — held while the verdict refuses the side, open otherwise.
func TestFirstFillHoldOnFuturesIsTheVerdictAlone(t *testing.T) {
	event := firstFillEvent(false, refused())
	event.Trade.Strategy.TradeType = aggragates.Futures
	event.Trade.PositionPrice = 0
	event.Trade.Logs = []aggragates.TradesLogs{{Message: waitingRow, Price: 100}}

	got, reason := FirstFillHold(event, aggragates.SideLong)
	if reason != "cooldown: trying to get a better entry price" || len(got.Trade.Logs) != 1 {
		t.Fatalf("a refused futures long must be held by the verdict alone, got %q %q", reason, rows(got))
	}
	if _, reason := FirstFillHold(event, aggragates.SideShort); reason != "cooldown: trying to get a better entry price (inverse)" {
		t.Fatalf("a refused futures short must be held by the verdict alone, got %q", reason)
	}
	if _, reason := FirstFillHold(event, ""); reason != "" {
		t.Fatalf("no side is nothing to judge, got %q", reason)
	}

	event.Params.CoolDownIndicators = allowed()
	for _, side := range []string{aggragates.SideLong, aggragates.SideShort} {
		if _, reason := FirstFillHold(event, side); reason != "" {
			t.Fatalf("an allowed futures %s must open, got %q", side, reason)
		}
	}
	event.Params.CoolDownIndicators = aggragates.CoolDownIndicators{}
	if _, reason := FirstFillHold(event, aggragates.SideLong); reason != "" {
		t.Fatalf("a missing verdict must fail open on futures, got %q", reason)
	}
	if !FirstFillVerdictNeeded(event.Trade, "new") {
		t.Fatal("futures fetch the verdict on every tick of a new trade")
	}
}
