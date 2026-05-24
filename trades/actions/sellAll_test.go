package actions

import (
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func TestSellAllUpdatesParentPositionToSellParent(t *testing.T) {
	var updateCalls int
	updateTrade := func(event events.Events) (events.Events, error) {
		updateCalls++
		return event, nil
	}

	event := events.Events{
		Trade: aggragates.Trades{Symbol: "BTC/USDT"},
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": updateTrade,
		},
	}

	got, err := SellAll(event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Trade.PositionType != "sellParent" {
		t.Errorf("PositionType = %q, want sellParent", got.Trade.PositionType)
	}
	if updateCalls != 1 {
		t.Errorf("expected updateTrade to be called once, got %d", updateCalls)
	}
}

func TestSellAllClosesChildrenWithoutHistoryWithoutSelling(t *testing.T) {
	var sellCalls, updateCalls int
	updateTrade := func(event events.Events) (events.Events, error) {
		updateCalls++
		return event, nil
	}
	sell := func(event events.Events) (events.Events, error) {
		sellCalls++
		return event, nil
	}

	children := []aggragates.Trades{
		{Symbol: "ETH/USDT"}, // no history
	}

	event := events.Events{
		Trade:          aggragates.Trades{Symbol: "BTC/USDT"},
		ChildrenTrades: children,
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": updateTrade,
			"sell":        sell,
		},
	}

	_, err := SellAll(event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if sellCalls != 0 {
		t.Errorf("expected sell not invoked for empty-history child, called %d times", sellCalls)
	}
	// updateTrade is called once per child (the close path) and once for the parent.
	if updateCalls != 2 {
		t.Errorf("expected updateTrade called twice, got %d", updateCalls)
	}
}

func TestSellAllInvokesSellForChildrenWithHistory(t *testing.T) {
	var sellCalls, updateCalls int
	updateTrade := func(event events.Events) (events.Events, error) {
		updateCalls++
		return event, nil
	}
	sell := func(event events.Events) (events.Events, error) {
		sellCalls++
		return event, nil
	}

	children := []aggragates.Trades{
		{
			Symbol:  "ETH/USDT",
			History: []aggragates.TradesHistory{{Type: "buy", Quantity: 1, Price: 100}},
		},
	}

	event := events.Events{
		Trade:          aggragates.Trades{Symbol: "BTC/USDT"},
		ChildrenTrades: children,
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": updateTrade,
			"sell":        sell,
		},
	}

	_, err := SellAll(event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if sellCalls != 1 {
		t.Errorf("expected sell called once for child with history, got %d", sellCalls)
	}
	// updateTrade called once for child sell chain and once for parent close.
	if updateCalls != 2 {
		t.Errorf("expected updateTrade called twice, got %d", updateCalls)
	}
}
