package actions

import (
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func withAI(trade aggragates.Trades) aggragates.Trades {
	trade.Strategy.Params.UseAI = true
	return trade
}

func withCooldown(trade aggragates.Trades) aggragates.Trades {
	trade.Strategy.Params.Cooldown = true
	return trade
}

func TestShouldHoldReturnsEventUnchangedWhenNoSignals(t *testing.T) {
	event := events.Events{
		Trade:  testutil.NewHoldTrade("stopLoss", false),
		Params: aggragates.Params{OldPosition: "active"},
	}

	got, err := ShouldHold(event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Trade.Symbol != event.Trade.Symbol {
		t.Errorf("expected trade returned unchanged, symbol = %q", got.Trade.Symbol)
	}
}

func TestShouldHoldIgnoresCooldownAfterFirstFill(t *testing.T) {
	event := events.Events{
		Trade: withCooldown(testutil.NewHoldTrade("stopLoss", false)),
		Params: aggragates.Params{
			OldPosition: "active",
			CoolDownIndicators: aggragates.CoolDownIndicators{
				HasFirstFillVerdict: true,
				AllowLongEntry:      false,
				MarketBearish:       true,
			},
		},
	}

	if _, err := ShouldHold(event); err != nil {
		t.Fatalf("cooldown must not hold a rebuy, got %v", err)
	}
}

func TestShouldHoldFirstFillExpensiveLong(t *testing.T) {
	event := events.Events{
		Trade: withCooldown(testutil.NewHoldTrade("buy", false)),
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": testutil.NopUpdateTrade,
		},
		Params: aggragates.Params{
			OldPosition: "new",
			CoolDownIndicators: aggragates.CoolDownIndicators{
				HasFirstFillVerdict: true,
				AllowLongEntry:      false,
			},
		},
	}

	if _, err := ShouldHold(event); err == nil {
		t.Fatal("expected hold when the first long fill is expensive")
	}
}

func TestShouldHoldFirstFillCheapLongPasses(t *testing.T) {
	event := events.Events{
		Trade: withCooldown(testutil.NewHoldTrade("buy", false)),
		Params: aggragates.Params{
			OldPosition: "new",
			CoolDownIndicators: aggragates.CoolDownIndicators{
				HasFirstFillVerdict: true,
				AllowLongEntry:      true,
			},
		},
	}

	if _, err := ShouldHold(event); err != nil {
		t.Fatalf("cheap first long fill must pass, got %v", err)
	}
}

func TestShouldHoldFirstFillExpensiveInverse(t *testing.T) {
	event := events.Events{
		Trade: withCooldown(testutil.NewHoldTrade("buy", true)),
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": testutil.NopUpdateTrade,
		},
		Params: aggragates.Params{
			OldPosition: "new",
			CoolDownIndicators: aggragates.CoolDownIndicators{
				HasFirstFillVerdict: true,
				AllowShortEntry:     false,
			},
		},
	}

	if _, err := ShouldHold(event); err == nil {
		t.Fatal("expected hold when the first inverse fill is expensive")
	}
}

func TestShouldHoldFirstFillWithoutVerdictFailsOpen(t *testing.T) {
	event := events.Events{
		Trade: withCooldown(testutil.NewHoldTrade("buy", false)),
		Params: aggragates.Params{
			OldPosition:        "new",
			CoolDownIndicators: aggragates.CoolDownIndicators{},
		},
	}

	if _, err := ShouldHold(event); err != nil {
		t.Fatalf("missing first-fill verdict must fail open, got %v", err)
	}
}

func TestShouldHoldHoldsOnExplicitAIHoldAction(t *testing.T) {
	event := events.Events{
		Trade: withAI(testutil.NewHoldTrade("stopLoss", false)),
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": testutil.NopUpdateTrade,
		},
		Params: aggragates.Params{
			OldPosition:  "active",
			AIIndicators: aggragates.AIIndicators{AIAction: aggragates.ActionHold},
		},
	}

	_, err := ShouldHold(event)
	if err == nil {
		t.Fatal("expected hold error when AI explicitly recommends HOLD for a risk-adding position")
	}
}

func TestShouldHoldAllowsTakeProfitOnExplicitAIHold(t *testing.T) {
	for _, inverse := range []bool{false, true} {
		event := events.Events{
			Trade: withAI(testutil.NewHoldTrade("takeProfit", inverse)),
			Params: aggragates.Params{
				OldPosition:  "active",
				AIIndicators: aggragates.AIIndicators{AIAction: aggragates.ActionHold},
			},
		}

		if _, err := ShouldHold(event); err != nil {
			t.Fatalf("expected explicit HOLD to allow takeProfit (inverse=%v), got %v", inverse, err)
		}
	}
}

func TestShouldHoldWritesInfoLogAndCollapsesRepeats(t *testing.T) {
	event := events.Events{
		Trade: withAI(testutil.NewHoldTrade("stopLoss", false)),
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": testutil.NopUpdateTrade,
		},
		Params: aggragates.Params{
			OldPosition:  "active",
			AIIndicators: aggragates.AIIndicators{AIMarketBearish: true},
		},
	}

	held, err := ShouldHold(event)
	if err == nil {
		t.Fatal("expected hold error")
	}
	if len(held.Trade.Logs) != 1 {
		t.Fatalf("expected one hold log, got %d", len(held.Trade.Logs))
	}
	logEntry := held.Trade.Logs[0]
	if logEntry.Type != aggragates.LOG_INFO {
		t.Errorf("hold log type = %q, want %q", logEntry.Type, aggragates.LOG_INFO)
	}
	if logEntry.Message != "Hold stopLoss: AI market is bearish" {
		t.Errorf("unexpected hold message %q", logEntry.Message)
	}
	if held.Trade.PositionType != "active" {
		t.Errorf("expected position restored to old position, got %q", held.Trade.PositionType)
	}

	// Next tick holds the same position for a DIFFERENT reason: that is a new
	// row. The old prefix dedup collapsed it and hid every reason change
	// behind the first hold (run 97: capitulation freezes invisible behind a
	// regime veto, the regime entry veto invisible behind cooldown).
	held.Trade.PositionType = "stopLoss"
	held.Params.AIIndicators = aggragates.AIIndicators{AIAction: aggragates.ActionHold}
	again, err := ShouldHold(held)
	if err == nil {
		t.Fatal("expected hold error on second tick")
	}
	if len(again.Trade.Logs) != 2 {
		t.Fatalf("expected a reason change to add a row, got %d logs", len(again.Trade.Logs))
	}
	if again.Trade.Logs[1].Message != "Hold stopLoss: AI recommends HOLD" {
		t.Errorf("unexpected second hold message %q", again.Trade.Logs[1].Message)
	}

	// The same reason again on the next tick still collapses.
	again.Trade.PositionType = "stopLoss"
	third, err := ShouldHold(again)
	if err == nil {
		t.Fatal("expected hold error on third tick")
	}
	if len(third.Trade.Logs) != 2 {
		t.Fatalf("expected a repeated reason to collapse, got %d logs", len(third.Trade.Logs))
	}
}

func TestShouldHoldNewStatusBlocksOpposingAISignal(t *testing.T) {
	event := events.Events{
		Trade: withAI(testutil.NewHoldTrade("buy", false)),
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": testutil.NopUpdateTrade,
		},
		Params: aggragates.Params{
			OldPosition:  "new",
			AIIndicators: aggragates.AIIndicators{AIAction: aggragates.ActionShort},
		},
	}

	_, err := ShouldHold(event)
	if err == nil {
		t.Fatal("expected hold error when entering new trade against an AI short signal")
	}
}

func TestShouldHoldNewStatusAllowsAlignedAISignal(t *testing.T) {
	event := events.Events{
		Trade: withAI(testutil.NewHoldTrade("buy", false)),
		Params: aggragates.Params{
			OldPosition:  "new",
			AIIndicators: aggragates.AIIndicators{AIAction: aggragates.ActionLong},
		},
	}

	got, err := ShouldHold(event)
	if err != nil {
		t.Fatalf("expected aligned AI signal to allow entry, got %v", err)
	}
	if got.Trade.Symbol != event.Trade.Symbol {
		t.Errorf("expected trade returned unchanged on aligned entry")
	}
}

func TestShouldHoldUseAIAloneDoesNotParkCrashDeepRebuy(t *testing.T) {
	crash := aggragates.AIIndicators{
		HasRegimeVerdict: true,
		AddAllowed:       true,
		CrashActive:      true,
		CrashScore:       85,
	}
	event := events.Events{
		Trade: withAI(testutil.DeepTrade(false)),
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": testutil.NopUpdateTrade,
		},
		Params: aggragates.Params{OldPosition: "active", AIIndicators: crash},
	}
	if _, err := ShouldHold(event); err != nil {
		t.Fatalf("UseAI without CrashGuard must not park a deep rebuy, got %v", err)
	}
}
