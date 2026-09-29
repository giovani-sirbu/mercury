package actions

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/cooldown"
	"github.com/giovani-sirbu/mercury/trades/gates/smarttakeloss"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// The smart take loss's one hold: no new ladder on a pair while sophos reads
// a quiet slow decline on it. Flag first, long parents only, verdict only —
// and before the cooldown's first-fill gate, after its wallet reserve.

const stlEntryHoldRow = "Hold entry: " + smarttakeloss.SlowDeclineEntryHoldReason

// stlEntryEvent is a new long parent's first-fill tick under the given flags,
// carrying the sophos payload the engine fetched.
func stlEntryEvent(params aggragates.StrategyParams, ai aggragates.AIIndicators) events.Events {
	trade := testutil.NewHoldTrade("buy", false)
	trade.Strategy.Params = params
	trade.PositionPrice = 100
	return events.Events{
		Trade: trade,
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": testutil.NopUpdateTrade,
		},
		Params:    aggragates.Params{OldPosition: "new", AIIndicators: ai},
		Timestamp: testutil.At("09:00:00").UnixMilli(),
	}
}

// slowDeclinePayload is the block sophos serves with the verdict on or off;
// the sell band rides either way.
func slowDeclinePayload(on bool) aggragates.AIIndicators {
	block := aggragates.SmartTakeLossIndicators{SlowDeclineExit: on, SlowDeclineSellBand: 101}
	if on {
		block.SlowDeclineExitReasons = []string{"leg down 7.0% from its high close"}
	}
	return aggragates.AIIndicators{SmartTakeLoss: block}
}

func assertNotHeld(t *testing.T, name string, event events.Events) {
	t.Helper()
	held, err := ShouldHold(event)
	if err != nil || len(held.Trade.Logs) != 0 {
		t.Fatalf("%s: the first fill must go through, got %v %v", name, err, messages(held.Trade.Logs))
	}
}

// A long parent's first fill is held while the verdict stands: one INFO row
// at the tick price, the trade restored to "new", and the chain stopped with
// the held error. The next tick collapses onto the same row.
func TestShouldHoldEntrySmartTakeLossHoldsALongFirstFill(t *testing.T) {
	held, err := ShouldHold(stlEntryEvent(aggragates.StrategyParams{SmartTakeLoss: true}, slowDeclinePayload(true)))
	if !errors.Is(err, events.ErrTradeHeld) {
		t.Fatalf("the first fill must be held, got %v", err)
	}
	if len(held.Trade.Logs) != 1 || held.Trade.Logs[0].Message != stlEntryHoldRow {
		t.Fatalf("rows = %v, want %q", messages(held.Trade.Logs), stlEntryHoldRow)
	}
	if row := held.Trade.Logs[0]; row.Price != 100 || row.Type != aggragates.LOG_INFO {
		t.Fatalf("the row carries the tick as an INFO row, got %+v", row)
	}
	if held.Trade.PositionType != "new" {
		t.Fatalf("a held first fill stays new, got %q", held.Trade.PositionType)
	}

	again, err := ShouldHold(held)
	if !errors.Is(err, events.ErrTradeHeld) || len(again.Trade.Logs) != 1 {
		t.Fatalf("the next tick must hold onto the same row, got %v %v", err, messages(again.Trade.Logs))
	}
}

// FETCH IS NOT GATE: the verdict rides /patterns for DynamicParams too, and
// holds nothing without SmartTakeLoss.
func TestShouldHoldEntrySmartTakeLossNeedsItsFlag(t *testing.T) {
	assertNotHeld(t, "dynamicParams", stlEntryEvent(aggragates.StrategyParams{DynamicParams: true}, slowDeclinePayload(true)))
	assertNotHeld(t, "no flag", stlEntryEvent(aggragates.StrategyParams{}, slowDeclinePayload(true)))
}

// Long parents only, the side read the way every entry gate reads it: an
// inverse spot entry, a futures SHORT or a directionless futures verdict and
// an impasse child go through; a futures LONG is held like a spot long.
func TestShouldHoldEntrySmartTakeLossHoldsLongParentsOnly(t *testing.T) {
	params := aggragates.StrategyParams{SmartTakeLoss: true}

	inverse := stlEntryEvent(params, slowDeclinePayload(true))
	inverse.Trade.Inverse = true
	assertNotHeld(t, "inverse spot", inverse)

	for _, action := range []string{aggragates.ActionShort, ""} {
		futures := stlEntryEvent(params, slowDeclinePayload(true))
		futures.Trade.Strategy.TradeType = aggragates.Futures
		futures.Params.AIIndicators.AIAction = action
		assertNotHeld(t, "futures "+action, futures)
	}

	long := stlEntryEvent(params, slowDeclinePayload(true))
	long.Trade.Strategy.TradeType = aggragates.Futures
	long.Params.AIIndicators.AIAction = aggragates.ActionLong
	if _, err := ShouldHold(long); !errors.Is(err, events.ErrTradeHeld) {
		t.Fatalf("a futures LONG entry is a long entry, got %v", err)
	}

	child := stlEntryEvent(params, slowDeclinePayload(true))
	child.Trade.ParentID = 7
	assertNotHeld(t, "impasse child", child)
}

// No verdict, no hold: the band sophos serves while the verdict is off, and
// an empty payload, hold nothing.
func TestShouldHoldEntrySmartTakeLossWithoutTheVerdict(t *testing.T) {
	params := aggragates.StrategyParams{SmartTakeLoss: true}
	assertNotHeld(t, "verdict off", stlEntryEvent(params, slowDeclinePayload(false)))
	assertNotHeld(t, "no payload", stlEntryEvent(params, aggragates.AIIndicators{}))
}

// The wallet reserve still speaks first: an entry the wallet cannot spare is
// the reserve's row, verdict or not.
func TestShouldHoldEntryDepthPriorityOutranksTheSlowDeclineHold(t *testing.T) {
	trade := testutil.LadderDepthTrade(21, "ADA/USDT", 0, walletDepths)
	trade.PositionType = "buy"
	trade.Strategy.Params.SmartTakeLoss = true

	event := priorityEvent(trade, "new", priorityWallet(), walletShortOf(t, trade, priorityReserve()))
	event.Params.AIIndicators = slowDeclinePayload(true)
	held, err := ShouldHold(event)
	if err == nil || len(held.Trade.Logs) != 1 {
		t.Fatalf("expected one held row, got %v %v", err, messages(held.Trade.Logs))
	}
	if row := held.Trade.Logs[0].Message; !strings.HasPrefix(row, "Hold entry: cooldown: depth priority,") {
		t.Fatalf("row = %q, want the wallet reserve's", row)
	}
}

// Before the first-fill gate: while the slow-decline hold stands the gate is
// never asked, so it writes no row and the verdict it would have activated on
// is still there for the tick the hold lifts. Without the slow decline the
// same tick is the gate's.
func TestShouldHoldEntrySlowDeclineHoldsBeforeTheFirstFillGate(t *testing.T) {
	params := aggragates.StrategyParams{Cooldown: true, SmartTakeLoss: true}
	refused := aggragates.CoolDownIndicators{HasFirstFillVerdict: true}

	event := stlEntryEvent(params, slowDeclinePayload(true))
	event.Params.CoolDownIndicators = refused
	held, err := ShouldHold(event)
	if !errors.Is(err, events.ErrTradeHeld) || len(held.Trade.Logs) != 1 || held.Trade.Logs[0].Message != stlEntryHoldRow {
		t.Fatalf("the slow-decline hold must be the only row, got %v %v", err, messages(held.Trade.Logs))
	}
	if strings.Contains(held.Trade.Logs[0].Message, cooldown.FirstFillWaitingPrefix) {
		t.Fatal("the first-fill gate must not have been consulted")
	}
	if !cooldown.FirstFillVerdictNeeded(held.Trade, "new") {
		t.Fatal("a first fill the gate never judged must still need its verdict")
	}

	control := stlEntryEvent(params, slowDeclinePayload(false))
	control.Params.CoolDownIndicators = refused
	held, err = ShouldHold(control)
	if err == nil || len(held.Trade.Logs) != 1 || !strings.Contains(held.Trade.Logs[0].Message, cooldown.FirstFillWaitingPrefix) {
		t.Fatalf("control: without the slow decline the first-fill gate holds, got %v %v", err, messages(held.Trade.Logs))
	}
}

// The hold over time, the way the engines tick it: every tick while the
// verdict stands is held, the row is written once and every later tick
// collapses onto it — no row per tick — and the first-fill gate is never
// asked, so it still needs its verdict. On the first tick the verdict has
// cleared, the first-fill gate speaks for itself.
func TestShouldHoldEntrySlowDeclineHoldCollapsesAndHandsOverToTheFirstFillGate(t *testing.T) {
	params := aggragates.StrategyParams{Cooldown: true, SmartTakeLoss: true}
	refused := aggragates.CoolDownIndicators{HasFirstFillVerdict: true}
	start := testutil.At("09:00:00")

	var logs []aggragates.TradesLogs
	for tick := 0; tick < 8; tick++ {
		event := stlEntryEvent(params, slowDeclinePayload(true))
		event.Params.CoolDownIndicators = refused
		event.Trade.Logs = logs
		event.Timestamp = start.Add(time.Duration(tick) * 15 * time.Minute).UnixMilli()
		held, err := ShouldHold(event)
		if !errors.Is(err, events.ErrTradeHeld) {
			t.Fatalf("tick %d: the first fill must stay held while the verdict stands, got %v", tick, err)
		}
		logs = held.Trade.Logs
	}
	if len(logs) != 1 || logs[0].Message != stlEntryHoldRow {
		t.Fatalf("the hold must write one row over the whole stretch, got %v", messages(logs))
	}
	trade := testutil.NewHoldTrade("buy", false)
	trade.Logs = logs
	if !cooldown.FirstFillVerdictNeeded(trade, "new") {
		t.Fatal("the first-fill gate was never asked, so it must still need its verdict")
	}

	cleared := stlEntryEvent(params, slowDeclinePayload(false))
	cleared.Params.CoolDownIndicators = refused
	cleared.Trade.Logs = logs
	cleared.Timestamp = start.Add(8 * 15 * time.Minute).UnixMilli()
	held, err := ShouldHold(cleared)
	if err == nil || len(held.Trade.Logs) != 2 || !strings.Contains(held.Trade.Logs[1].Message, cooldown.FirstFillWaitingPrefix) {
		t.Fatalf("once the verdict clears the first-fill gate must hold on its own row, got %v %v", err, messages(held.Trade.Logs))
	}
}

// The hold outranks the legacy AI veto after it: an entry both would hold is
// the slow decline's row.
func TestShouldHoldEntrySlowDeclineHoldsBeforeTheAIVeto(t *testing.T) {
	payload := slowDeclinePayload(true)
	payload.AIMarketBearish = true
	held, err := ShouldHold(stlEntryEvent(aggragates.StrategyParams{SmartTakeLoss: true, UseAI: true}, payload))
	if !errors.Is(err, events.ErrTradeHeld) || len(held.Trade.Logs) != 1 || held.Trade.Logs[0].Message != stlEntryHoldRow {
		t.Fatalf("the slow-decline hold must be the one row, got %v %v", err, messages(held.Trade.Logs))
	}

	payload.SmartTakeLoss = aggragates.SmartTakeLossIndicators{}
	held, err = ShouldHold(stlEntryEvent(aggragates.StrategyParams{SmartTakeLoss: true, UseAI: true}, payload))
	if err == nil || len(held.Trade.Logs) != 1 || held.Trade.Logs[0].Message == stlEntryHoldRow {
		t.Fatalf("control: without the verdict the AI veto holds on its own row, got %v %v", err, messages(held.Trade.Logs))
	}
}
