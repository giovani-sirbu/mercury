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
		Trade: newHoldTrade("takeProfit", false),
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": nopUpdateTradeForHold,
		},
		Params: aggragates.Params{
			OldPosition: "active",
			AIIndicators: aggragates.AIIndicators{UseAI: true, AIAction: ActionHold},
		},
	}

	_, err := ShouldHold(event)
	if err == nil {
		t.Fatal("expected hold error when AI explicitly recommends HOLD")
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
