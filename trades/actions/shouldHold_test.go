package actions

import (
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func nopUpdateTradeForHold(event events.Events) (events.Events, error) {
	return event, nil
}

func newHoldTrade(positionType string, inverse bool) aggragates.Trades {
	return aggragates.Trades{
		Symbol:       "BTC/USDT",
		PositionType: positionType,
		Inverse:      inverse,
	}
}

func TestShouldHoldReturnsEventUnchangedWhenNoSignals(t *testing.T) {
	event := events.Events{
		Trade:  newHoldTrade("stopLoss", false),
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

func TestShouldHoldHoldsStopLossOnBearishClassicMarket(t *testing.T) {
	event := events.Events{
		Trade: newHoldTrade("stopLoss", false),
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": nopUpdateTradeForHold,
		},
		Params: aggragates.Params{
			OldPosition: "active",
			CoolDownIndicators: aggragates.CoolDownIndicators{
				MarketBearish: true,
			},
		},
	}

	_, err := ShouldHold(event)
	if err == nil {
		t.Fatal("expected hold error when classic indicator is bearish for stopLoss spot position")
	}
}

func TestShouldHoldHoldsOnExplicitAIHoldAction(t *testing.T) {
	event := events.Events{
		Trade: newHoldTrade("stopLoss", false),
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": nopUpdateTradeForHold,
		},
		Params: aggragates.Params{
			OldPosition:  "active",
			AIIndicators: aggragates.AIIndicators{UseAI: true, AIAction: ActionHold},
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
			Trade: newHoldTrade("takeProfit", inverse),
			Params: aggragates.Params{
				OldPosition:  "active",
				AIIndicators: aggragates.AIIndicators{UseAI: true, AIAction: ActionHold},
			},
		}

		if _, err := ShouldHold(event); err != nil {
			t.Fatalf("expected explicit HOLD to allow takeProfit (inverse=%v), got %v", inverse, err)
		}
	}
}

func TestShouldHoldWritesInfoLogAndCollapsesRepeats(t *testing.T) {
	event := events.Events{
		Trade: newHoldTrade("stopLoss", false),
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": nopUpdateTradeForHold,
		},
		Params: aggragates.Params{
			OldPosition:  "active",
			AIIndicators: aggragates.AIIndicators{UseAI: true, AIMarketBearish: true},
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

	// Next tick holds the same position for a different reason: the log is
	// collapsed instead of appended again.
	held.Trade.PositionType = "stopLoss"
	held.Params.AIIndicators = aggragates.AIIndicators{UseAI: true, AIAction: ActionHold}
	again, err := ShouldHold(held)
	if err == nil {
		t.Fatal("expected hold error on second tick")
	}
	if len(again.Trade.Logs) != 1 {
		t.Fatalf("expected repeated hold to collapse, got %d logs", len(again.Trade.Logs))
	}
}

func TestShouldHoldNewStatusBlocksOpposingAISignal(t *testing.T) {
	event := events.Events{
		Trade: newHoldTrade("buy", false),
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": nopUpdateTradeForHold,
		},
		Params: aggragates.Params{
			OldPosition:  "new",
			AIIndicators: aggragates.AIIndicators{UseAI: true, AIAction: ActionShort},
		},
	}

	_, err := ShouldHold(event)
	if err == nil {
		t.Fatal("expected hold error when entering new trade against an AI short signal")
	}
}

func TestShouldHoldNewStatusAllowsAlignedAISignal(t *testing.T) {
	event := events.Events{
		Trade: newHoldTrade("buy", false),
		Params: aggragates.Params{
			OldPosition:  "new",
			AIIndicators: aggragates.AIIndicators{UseAI: true, AIAction: ActionLong},
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
