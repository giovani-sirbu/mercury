package actions

import (
	"errors"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates"
	"github.com/giovani-sirbu/mercury/strategies/gates/cooldown"
	"github.com/giovani-sirbu/mercury/strategies/gates/cooldown/depthspacing"
	"github.com/giovani-sirbu/mercury/strategies/gates/cooldown/firstfill"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/flatmarket"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/quietdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The smart take loss's quiet decline hold: no new ladder on a pair while sophos
// reads a quiet decline on it. Flag first, long parents only, verdict only —
// and before the cooldown's first-fill gate, after its wallet reserve. Its
// hold is a pair: the INFO row and, beside it under the same stamp, the held
// event of the entry hold gate. The flat market gate's hold follows the tests
// of this one, at the end of the file.

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

// slowDeclineWindowFrom is the QuietDeclineFillFrom the payloads of these tests
// serve, as sophos serves it whenever it read its window: the open of the
// fixture day, before every tick.
var slowDeclineWindowFrom = testutil.At("00:00:00")

// slowDeclinePayload is the block sophos serves with the verdict on or off;
// the fill window rides either way.
func slowDeclinePayload(on bool) aggragates.AIIndicators {
	block := aggragates.SmartTakeLossIndicators{QuietDeclineVerdict: on, QuietDeclineFillFrom: slowDeclineWindowFrom.UnixMilli()}
	if on {
		block.QuietDeclineExitReasons = []string{"leg down 7.0% from its high close"}
	}
	return aggragates.AIIndicators{SmartTakeLoss: block}
}

func assertNotHeld(t *testing.T, name string, event events.Events) {
	t.Helper()
	held, err := ShouldHold(event)
	if err != nil || len(held.Trade.Logs) != 0 || len(held.Trade.StrategyEvents) != 0 {
		t.Fatalf("%s: the first fill must go through, got %v %v and %d events", name, err, messages(held.Trade.Logs), len(held.Trade.StrategyEvents))
	}
}

// assertEntryHoldPair fails unless the trade's newest row and event are the
// smart take loss's entry hold, paired: the one hold event of its gate.
func assertEntryHoldPair(t *testing.T, trade aggragates.Trades) {
	t.Helper()
	assertNewestPair(t, trade, aggragates.StrategyParamSmartTakeLoss, smarttakeloss.GateEntryHold, gates.EventHeld)
	if got := trade.StrategyEventsOf(aggragates.StrategyParamSmartTakeLoss, smarttakeloss.GateEntryHold); len(got) != 1 {
		t.Fatalf("the hold writes one event, got %d", len(got))
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
	assertEntryHoldPair(t, held.Trade)

	again, err := ShouldHold(held)
	if !errors.Is(err, events.ErrTradeHeld) || len(again.Trade.Logs) != 1 || len(again.Trade.StrategyEvents) != 1 {
		t.Fatalf("the next tick must hold onto the same row and write no second event, got %v %v and %d events", err, messages(again.Trade.Logs), len(again.Trade.StrategyEvents))
	}
}

// FETCH IS NOT GATE: a slow-decline verdict holds nothing without
// SmartTakeLoss, whether another sophos flag is on or no flag is.
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

// No verdict, no hold: the fill window sophos serves while the verdict is
// off, and an empty payload, hold nothing.
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
	if len(held.Trade.StrategyEvents) != 1 {
		t.Fatalf("the reserve's row is paired with the reserve's event alone, got %d events", len(held.Trade.StrategyEvents))
	}
	assertNewestPair(t, held.Trade, aggragates.StrategyParamCooldown, cooldown.GateDepthPriority, gates.EventHeld)
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
	assertEntryHoldPair(t, held.Trade)
	if len(held.Trade.StrategyEvents) != 1 {
		t.Fatalf("the slow-decline hold is the only event, got %d", len(held.Trade.StrategyEvents))
	}
	if !firstfill.FirstFillVerdictNeeded(held.Trade, "new") {
		t.Fatal("a first fill the gate never judged must still need its verdict")
	}

	control := stlEntryEvent(params, slowDeclinePayload(false))
	control.Params.CoolDownIndicators = refused
	held, err = ShouldHold(control)
	if err == nil || len(held.Trade.Logs) != 1 || !strings.Contains(held.Trade.Logs[0].Message, cooldown.FirstFillWaitingPrefix) {
		t.Fatalf("control: without the slow decline the first-fill gate holds, got %v %v", err, messages(held.Trade.Logs))
	}
	assertNewestPair(t, held.Trade, aggragates.StrategyParamCooldown, cooldown.GateFirstFill, cooldown.FirstFillActivated)
	if got := held.Trade.StrategyEventsOf(aggragates.StrategyParamSmartTakeLoss, smarttakeloss.GateEntryHold); len(got) != 0 {
		t.Fatalf("control: the slow decline held nothing, so it wrote no event, got %d", len(got))
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
	var stored []aggragates.TradesStrategyEvents
	for tick := 0; tick < 8; tick++ {
		event := stlEntryEvent(params, slowDeclinePayload(true))
		event.Params.CoolDownIndicators = refused
		event.Trade.Logs = logs
		event.Trade.StrategyEvents = stored
		event.Timestamp = start.Add(time.Duration(tick) * 15 * time.Minute).UnixMilli()
		held, err := ShouldHold(event)
		if !errors.Is(err, events.ErrTradeHeld) {
			t.Fatalf("tick %d: the first fill must stay held while the verdict stands, got %v", tick, err)
		}
		logs = held.Trade.Logs
		stored = held.Trade.StrategyEvents
	}
	if len(logs) != 1 || logs[0].Message != stlEntryHoldRow {
		t.Fatalf("the hold must write one row over the whole stretch, got %v", messages(logs))
	}
	trade := testutil.NewHoldTrade("buy", false)
	trade.Logs = logs
	trade.StrategyEvents = stored
	assertEntryHoldPair(t, trade)
	if len(stored) != 1 {
		t.Fatalf("the hold must write one event over the whole stretch, got %d", len(stored))
	}
	if !firstfill.FirstFillVerdictNeeded(trade, "new") {
		t.Fatal("the first-fill gate was never asked, so it must still need its verdict")
	}
	if got := trade.StrategyEventsOf(aggragates.StrategyParamCooldown, cooldown.GateFirstFill); len(got) != 0 {
		t.Fatalf("the first-fill gate was never asked, so it wrote no event, got %d", len(got))
	}

	cleared := stlEntryEvent(params, slowDeclinePayload(false))
	cleared.Params.CoolDownIndicators = refused
	cleared.Trade.Logs = logs
	cleared.Trade.StrategyEvents = stored
	cleared.Timestamp = start.Add(8 * 15 * time.Minute).UnixMilli()
	held, err := ShouldHold(cleared)
	if err == nil || len(held.Trade.Logs) != 2 || !strings.Contains(held.Trade.Logs[1].Message, cooldown.FirstFillWaitingPrefix) {
		t.Fatalf("once the verdict clears the first-fill gate must hold on its own row, got %v %v", err, messages(held.Trade.Logs))
	}
	assertNewestPair(t, held.Trade, aggragates.StrategyParamCooldown, cooldown.GateFirstFill, cooldown.FirstFillActivated)
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
	assertEntryHoldPair(t, held.Trade)

	payload.SmartTakeLoss = aggragates.SmartTakeLossIndicators{}
	held, err = ShouldHold(stlEntryEvent(aggragates.StrategyParams{SmartTakeLoss: true, UseAI: true}, payload))
	if err == nil || len(held.Trade.Logs) != 1 || held.Trade.Logs[0].Message == stlEntryHoldRow {
		t.Fatalf("control: without the verdict the AI veto holds on its own row, got %v %v", err, messages(held.Trade.Logs))
	}
	if len(held.Trade.StrategyEvents) != 0 {
		t.Fatalf("control: the AI veto writes its row alone, got %d events", len(held.Trade.StrategyEvents))
	}
}

// The flat market gate's hold: the first fill of a spot parent, long or inverse,
// for as long as its reading of the last closed bars stands, and nothing on an
// open ladder. These tests serve a flat window, whose bodies are all quiet and
// whose move is none, under a trend that meets flatmarket.MinSmcTrend
// on the entry's side, so that what they assert follows the constants, and they
// skip when a retune leaves no reading that holds. flatmarket.Enabled ships
// on and this package cannot toggle it. The hold is a pair like the quiet
// decline's, its event carrying the reasons of the reading and no price.

const (
	flatMarketEntryHoldRow = "Hold entry: " + smarttakeloss.FlatMarketEntryHoldReason
	// flatPrice is the price every bar of the flat window opens, closes, peaks and
	// troughs at.
	flatPrice = 100.0
)

// flatMarketFirstBar is the open of the oldest bar of the window the tests serve:
// two days before the fixture day, so that every bar is closed at every tick.
var flatMarketFirstBar = testutil.At("00:00:00").Add(-48 * time.Hour)

// flatMarketBlock is the block sophos serves over a flat window of closed 1h bars,
// the oldest opening at first: as many bars as sophos serves and never fewer than
// the reading needs, every price the same, and the SMC trend direction reading the
// side's way on exactly flatmarket.MinSmcTrend rows, all of them served.
func flatMarketBlock(side string, first time.Time) aggragates.SmartTakeLossIndicators {
	var block aggragates.SmartTakeLossIndicators
	for index := range max(24, flatmarket.MinBars) {
		opened := first.Add(time.Duration(index) * time.Hour)
		block.FlatMarketBarOpens = append(block.FlatMarketBarOpens, opened.UnixMilli())
		block.FlatMarketOpenPrices = append(block.FlatMarketOpenPrices, flatPrice)
		block.FlatMarketHighs = append(block.FlatMarketHighs, flatPrice)
		block.FlatMarketLows = append(block.FlatMarketLows, flatPrice)
		block.FlatMarketCloses = append(block.FlatMarketCloses, flatPrice)
	}
	block.SmcTrendValid = flatmarket.MinSmcTrend
	if side == aggragates.SideShort {
		block.SmcTrendUp = flatmarket.MinSmcTrend
	} else {
		block.SmcTrendDown = flatmarket.MinSmcTrend
	}
	return block
}

// flatMarketReading is the reading the gate makes of the block for the trade: the
// bars and the trend of the block against the pair's base percentage and the side
// the entry would take.
func flatMarketReading(trade aggragates.Trades, block aggragates.SmartTakeLossIndicators) flatmarket.RangeReading {
	side := aggragates.EntrySide(trade, aggragates.AIIndicators{})
	return flatmarket.TriggerRange(flatmarket.BarsOf(block), trade.StrategyPair.StrategySettings[0].Percentage, side, series.TrendOf(block))
}

// flatMarketMarket is the payload that holds for the trade, its window opening at
// first. It skips the test when flatmarket.Enabled ships off or the constants
// leave a flat window no reading that holds, which leaves nothing to assert.
func flatMarketMarket(t *testing.T, trade aggragates.Trades, first time.Time) aggragates.AIIndicators {
	t.Helper()
	if !flatmarket.Enabled {
		t.Skip("flatmarket.Enabled ships off: the gate holds nothing")
	}
	side := aggragates.EntrySide(trade, aggragates.AIIndicators{})
	block := flatMarketBlock(side, first)
	if !flatMarketReading(trade, block).Holds {
		t.Skip("the constants of gates/smarttakeloss/lowinterest leave a flat window no reading that holds: flatmarket.BodyShare, flatmarket.QuietShare or flatmarket.MaxMovePct")
	}
	return aggragates.AIIndicators{SmartTakeLoss: block}
}

// assertGoesThrough fails unless the tick went through the holds untouched: no
// error, no row and no event.
func assertGoesThrough(t *testing.T, what string, event events.Events) {
	t.Helper()
	held, err := ShouldHold(event)
	if err != nil || len(held.Trade.Logs) != 0 || len(held.Trade.StrategyEvents) != 0 {
		t.Fatalf("%s: must go through, got %v %v and %d events", what, err, messages(held.Trade.Logs), len(held.Trade.StrategyEvents))
	}
}

// assertFlatMarketHold fails unless the trade's newest row is the hold row and its
// newest event the held event of the flat market gate, paired with it: the kind
// held, exactly the reasons given, no price and no feature.
func assertFlatMarketHold(t *testing.T, trade aggragates.Trades, row string, reasons []string) {
	t.Helper()
	assertNewestPair(t, trade, aggragates.StrategyParamSmartTakeLoss, smarttakeloss.GateFlatMarket, gates.EventHeld)
	if got := trade.Logs[len(trade.Logs)-1].Message; got != row {
		t.Fatalf("row = %q, want %q", got, row)
	}
	var data smarttakeloss.EventData
	if err := trade.StrategyEvents[len(trade.StrategyEvents)-1].DecodeData(&data); err != nil {
		t.Fatalf("the held event does not decode: %v", err)
	}
	if data.Event != gates.EventHeld || data.Price != 0 || len(data.Features) != 0 || len(reasons) == 0 || !reflect.DeepEqual(data.Reasons, reasons) {
		t.Fatalf("the held event is %+v, want the kind %q, the reasons %q and no price or feature", data, gates.EventHeld, reasons)
	}
}

// A spot parent's first fill is held while the reading stands, a long one and an
// inverse one on the mirror: one INFO row at the tick price, the trade restored to
// "new", the chain stopped with the held error, and beside the row the held event
// of the gate naming the reasons of the reading.
func TestShouldHoldEntryFlatMarketHoldsAFirstFillWhileItsReadingStands(t *testing.T) {
	for name, inverse := range map[string]bool{"long": false, "inverse": true} {
		event := stlEntryEvent(aggragates.StrategyParams{SmartTakeLoss: true}, aggragates.AIIndicators{})
		event.Trade.Inverse = inverse
		event.Params.AIIndicators = flatMarketMarket(t, event.Trade, flatMarketFirstBar)
		reasons := flatMarketReading(event.Trade, event.Params.AIIndicators.SmartTakeLoss).Reasons

		held, err := ShouldHold(event)
		if !errors.Is(err, events.ErrTradeHeld) {
			t.Fatalf("%s: the first fill must be held, got %v", name, err)
		}
		if len(held.Trade.Logs) != 1 || len(held.Trade.StrategyEvents) != 1 {
			t.Fatalf("%s: one row and one event, got %v and %d events", name, messages(held.Trade.Logs), len(held.Trade.StrategyEvents))
		}
		if row := held.Trade.Logs[0]; row.Price != 100 || row.Type != aggragates.LOG_INFO {
			t.Fatalf("%s: the row carries the tick as an INFO row, got %+v", name, row)
		}
		if held.Trade.PositionType != "new" {
			t.Fatalf("%s: a held first fill stays new, got %q", name, held.Trade.PositionType)
		}
		assertFlatMarketHold(t, held.Trade, flatMarketEntryHoldRow, reasons)
	}
}

// The first fill goes through when the gate has nothing to say: a flag that is off
// (a reading fetched for another flag's sake gates nothing), a futures entry, an
// impasse child and a pair sophos served no bars for. The reading's own conditions
// are the gate's, tested in its package.
func TestShouldHoldEntryFlatMarketHoldsNothingWithoutItsFlagItsSpotParentOrItsReading(t *testing.T) {
	params := aggragates.StrategyParams{SmartTakeLoss: true}
	market := flatMarketMarket(t, stlEntryEvent(params, aggragates.AIIndicators{}).Trade, flatMarketFirstBar)

	assertGoesThrough(t, "dynamicParams", stlEntryEvent(aggragates.StrategyParams{DynamicParams: true}, market))
	assertGoesThrough(t, "no flag", stlEntryEvent(aggragates.StrategyParams{}, market))
	assertGoesThrough(t, "no payload", stlEntryEvent(params, aggragates.AIIndicators{}))

	futures := stlEntryEvent(params, market)
	futures.Trade.Strategy.TradeType = aggragates.Futures
	futures.Params.AIIndicators.AIAction = aggragates.ActionLong
	assertGoesThrough(t, "a futures long entry", futures)

	child := stlEntryEvent(params, market)
	child.Trade.ParentID = 7
	assertGoesThrough(t, "an impasse child", child)
}

// Before the cooldown's first-fill gate, which is never asked while the hold stands:
// it writes no row, the verdict it would have judged is still wanted for the tick
// the hold lifts, and without the market the same tick is the gate's own. After the
// quiet decline's hold, which speaks first whenever it is on and its verdict stands.
func TestShouldHoldEntryFlatMarketSpeaksAfterTheQuietDeclineAndBeforeTheFirstFillGate(t *testing.T) {
	params := aggragates.StrategyParams{Cooldown: true, SmartTakeLoss: true}
	refused := aggragates.CoolDownIndicators{HasFirstFillVerdict: true}

	event := stlEntryEvent(params, aggragates.AIIndicators{})
	event.Params.AIIndicators = flatMarketMarket(t, event.Trade, flatMarketFirstBar)
	event.Params.CoolDownIndicators = refused
	reasons := flatMarketReading(event.Trade, event.Params.AIIndicators.SmartTakeLoss).Reasons
	held, err := ShouldHold(event)
	if !errors.Is(err, events.ErrTradeHeld) || len(held.Trade.Logs) != 1 || len(held.Trade.StrategyEvents) != 1 {
		t.Fatalf("the flat market hold must be the only row and event, got %v %v and %d events", err, messages(held.Trade.Logs), len(held.Trade.StrategyEvents))
	}
	if strings.Contains(held.Trade.Logs[0].Message, cooldown.FirstFillWaitingPrefix) {
		t.Fatal("the first-fill gate must not have been consulted")
	}
	assertFlatMarketHold(t, held.Trade, flatMarketEntryHoldRow, reasons)
	if !firstfill.FirstFillVerdictNeeded(held.Trade, "new") {
		t.Fatal("a first fill the gate never judged must still need its verdict")
	}

	control := stlEntryEvent(params, aggragates.AIIndicators{})
	control.Params.CoolDownIndicators = refused
	held, err = ShouldHold(control)
	if err == nil || len(held.Trade.Logs) != 1 || !strings.Contains(held.Trade.Logs[0].Message, cooldown.FirstFillWaitingPrefix) {
		t.Fatalf("control: without the market the first-fill gate holds, got %v %v", err, messages(held.Trade.Logs))
	}
	if got := held.Trade.StrategyEventsOf(aggragates.StrategyParamSmartTakeLoss, smarttakeloss.GateFlatMarket); len(got) != 0 {
		t.Fatalf("control: the flat market gate held nothing, so it wrote no event, got %d", len(got))
	}

	both := stlEntryEvent(params, slowDeclinePayload(true))
	both.Params.AIIndicators.SmartTakeLoss = flatMarketMarket(t, both.Trade, flatMarketFirstBar).SmartTakeLoss
	both.Params.AIIndicators.SmartTakeLoss.QuietDeclineVerdict = true
	both.Params.CoolDownIndicators = refused
	want := flatMarketEntryHoldRow
	if quietdecline.Enabled {
		want = stlEntryHoldRow
	}
	held, err = ShouldHold(both)
	if !errors.Is(err, events.ErrTradeHeld) || len(held.Trade.Logs) != 1 || held.Trade.Logs[0].Message != want {
		t.Fatalf("with both readings standing the row is %q, got %v %v", want, err, messages(held.Trade.Logs))
	}
}

// The reason is one text whatever the bars the reading names, which ride in the
// held event, so the hold collapses onto its row: within the re-log window of
// tradelog.SaveHoldLog the next tick writes no second row and no second event, even
// over the next bar's window, and the chain still stops with the held error and the
// note that nothing was persisted.
func TestShouldHoldEntryFlatMarketHoldCollapsesWithinTheRelogWindow(t *testing.T) {
	event := stlEntryEvent(aggragates.StrategyParams{SmartTakeLoss: true}, aggragates.AIIndicators{})
	event.Params.AIIndicators = flatMarketMarket(t, event.Trade, flatMarketFirstBar)
	held, err := ShouldHold(event)
	if !errors.Is(err, events.ErrTradeHeld) || len(held.Trade.Logs) != 1 {
		t.Fatalf("the first tick must write the hold, got %v %v", err, messages(held.Trade.Logs))
	}

	for _, later := range []time.Duration{0, 15 * time.Minute, time.Hour} {
		next := held
		next.Timestamp = event.Timestamp + later.Milliseconds()
		next.Params.AIIndicators = flatMarketMarket(t, next.Trade, flatMarketFirstBar.Add(later))
		again, err := ShouldHold(next)
		if !errors.Is(err, events.ErrTradeHeld) || !errors.Is(err, events.ErrHoldNotPersisted) {
			t.Fatalf("+%s: the next tick must collapse onto the row and persist nothing, got %v", later, err)
		}
		if len(again.Trade.Logs) != 1 || len(again.Trade.StrategyEvents) != 1 {
			t.Fatalf("+%s: no second row and no second event, got %v and %d events", later, messages(again.Trade.Logs), len(again.Trade.StrategyEvents))
		}
	}
}

// flatMarketDepthEvent is the arming of the next depth of a ladder with one fill
// in the position given, under the market, at a tick inside the base hold of the
// cooldown's depth spacing: the tick that gate parks, were it asked.
func flatMarketDepthEvent(t *testing.T, params aggragates.StrategyParams, position string) events.Events {
	t.Helper()
	first := testutil.At("09:00:00")
	trade := testutil.DepthTrade(first)
	trade.Strategy.Params = params
	trade.PositionType = position
	event := depthEvent(trade, first.Add(depthspacing.BaseHold/2))
	event.Params.AIIndicators = flatMarketMarket(t, trade, flatMarketFirstBar)
	return event
}

// The flat market gate holds nothing on an open ladder. Under the very market
// that holds a first fill, the arming of the next depth (the stop loss and its
// force-trailing re-anchor) and the arming of the take profit go through with no
// row and no event: the ladder arms and buys its next depth as its own logic lets
// it, the gate's episode waiting on its bounds (smarttakeloss.Apply). On a tick
// the depth spacing parks, the only row is the depth spacing's and the gate writes
// no event.
func TestShouldHoldPositionFlatMarketHoldsNothingOnAnOpenLadder(t *testing.T) {
	params := aggragates.StrategyParams{SmartTakeLoss: true}
	for _, position := range []string{"stopLoss", "forceTrailingStopLoss", "takeProfit", "forceTrailingTakeProfit"} {
		assertGoesThrough(t, position, flatMarketDepthEvent(t, params, position))
	}

	parked, err := ShouldHold(flatMarketDepthEvent(t, aggragates.StrategyParams{Cooldown: true, SmartTakeLoss: true}, "stopLoss"))
	if err == nil || len(parked.Trade.Logs) != 1 || !strings.Contains(parked.Trade.Logs[0].Message, "cooldown:") {
		t.Fatalf("the depth spacing must park this tick on its own row, got %v %v", err, messages(parked.Trade.Logs))
	}
	assertNewestPair(t, parked.Trade, aggragates.StrategyParamCooldown, cooldown.GateDepthSpacing, gates.EventHeld)
	if got := parked.Trade.StrategyEventsOf(aggragates.StrategyParamSmartTakeLoss, smarttakeloss.GateFlatMarket); len(got) != 0 {
		t.Fatalf("the flat market gate holds no depth, so it wrote no event, got %d", len(got))
	}
}
