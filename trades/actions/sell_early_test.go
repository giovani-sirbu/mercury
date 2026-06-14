package actions

import (
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func TestSellReturnsErrorWhenPendingOrderExists(t *testing.T) {
	event := events.Events{
		Trade: aggragates.Trades{PendingOrder: 99, Symbol: "BTC/USDT"},
	}

	_, err := Sell(event)
	if err == nil {
		t.Fatal("expected error when trade already has pending order id")
	}
}

func TestSellClosesNewStatusTradeWithoutCallingExchange(t *testing.T) {
	event := events.Events{
		Trade: aggragates.Trades{Symbol: "BTC/USDT", Status: "new"},
	}

	got, err := Sell(event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Trade.Status != aggragates.Closed {
		t.Errorf("expected status closed, got %q", got.Trade.Status)
	}
}

func TestSellClosesTradeWhenOldPositionIsNew(t *testing.T) {
	event := events.Events{
		Trade:  aggragates.Trades{Symbol: "BTC/USDT"},
		Params: aggragates.Params{OldPosition: "new"},
	}

	got, err := Sell(event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Trade.Status != aggragates.Closed {
		t.Errorf("expected status closed for new old position, got %q", got.Trade.Status)
	}
}
