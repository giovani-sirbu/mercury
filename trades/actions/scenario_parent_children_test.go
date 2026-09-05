package actions_test

import (
	"testing"

	"github.com/adshao/go-binance/v2/common"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/exchange"
	exchangeAggregates "github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// TestParentChildren_ParentTradeHasProfitCombinesAllChildren walks the
// realistic impasse flow: the parent trade plus two children, each priced
// by the exchange. ParentTradeHasProfit must aggregate children profits
// into the parent's profit and accept the close when the combined profit
// is positive.
func TestParentChildren_ParentTradeHasProfitCombinesAllChildren(t *testing.T) {
	parent := scenarioBuildTrade("sellParent", 101000, false)
	scenarioAppendHistory(&parent, "BUY", 0.001, 100000, "", 0)

	child1 := scenarioBuildTrade("takeProfit", 0, false)
	child1.Symbol = "ETH/USDC"
	scenarioAppendHistory(&child1, "BUY", 0.05, 3000, "", 0)

	child2 := scenarioBuildTrade("takeProfit", 0, false)
	child2.Symbol = "SOL/USDC"
	scenarioAppendHistory(&child2, "BUY", 0.5, 200, "", 0)

	customActions := exchangeAggregates.Actions{
		GetPrice: func(symbol string) (float64, *common.APIError) {
			switch symbol {
			case "ETH/USDC":
				return 3100, nil
			case "SOL/USDC":
				return 210, nil
			}
			return 0, &common.APIError{Message: "unknown symbol"}
		},
	}

	event := events.Events{
		Exchange:       exchange.Exchange{IsCustom: true, CustomActions: customActions},
		Trade:          parent,
		ChildrenTrades: []aggragates.Trades{child1, child2},
		Events: map[string]func(events.Events) (events.Events, error){
			"hasProfit": actions.HasProfit,
		},
	}

	if _, err := actions.ParentTradeHasProfit(event); err != nil {
		t.Fatalf("ParentTradeHasProfit rejected a profitable impasse-with-children: %v", err)
	}
}

// TestParentChildren_ParentTradeHasProfitFailsOnZeroChildPrice pins the
// guard at the top of the children loop: if any child price comes back as
// zero (exchange hiccup), the function bails out with an error rather than
// posting a phantom profit.
func TestParentChildren_ParentTradeHasProfitFailsOnZeroChildPrice(t *testing.T) {
	parent := scenarioBuildTrade("sellParent", 101000, false)
	scenarioAppendHistory(&parent, "BUY", 0.001, 100000, "", 0)

	child := scenarioBuildTrade("takeProfit", 0, false)
	child.Symbol = "DOGE/USDC"

	customActions := exchangeAggregates.Actions{
		GetPrice: func(symbol string) (float64, *common.APIError) { return 0, nil },
	}

	event := events.Events{
		Exchange:       exchange.Exchange{IsCustom: true, CustomActions: customActions},
		Trade:          parent,
		ChildrenTrades: []aggragates.Trades{child},
		Events: map[string]func(events.Events) (events.Events, error){
			"hasProfit": actions.HasProfit,
		},
	}

	if _, err := actions.ParentTradeHasProfit(event); err == nil {
		t.Fatal("expected error when child price lookup returns zero")
	}
}

// TestParentChildren_SellAllPropagatesToEachChild proves SellAll iterates
// every child, runs the sell chain on each, and finally flips the parent
// to sellParent + invokes updateTrade.
func TestParentChildren_SellAllPropagatesToEachChild(t *testing.T) {
	parent := scenarioBuildTrade("impasse", 95000, false)

	child1 := scenarioBuildTrade("takeProfit", 3100, false)
	child1.Symbol = "ETH/USDC"
	scenarioAppendHistory(&child1, "BUY", 0.05, 3000, "", 0)

	child2 := scenarioBuildTrade("takeProfit", 210, false)
	child2.Symbol = "SOL/USDC"
	// child2 has empty history -> close branch (no sell call).

	var sellCalls, updateCalls int
	stubSell := func(e events.Events) (events.Events, error) {
		sellCalls++
		return e, nil
	}
	stubUpdate := func(e events.Events) (events.Events, error) {
		updateCalls++
		return e, nil
	}

	event := events.Events{
		Trade:          parent,
		ChildrenTrades: []aggragates.Trades{child1, child2},
		Events: map[string]func(events.Events) (events.Events, error){
			"sell":        stubSell,
			"updateTrade": stubUpdate,
		},
	}

	got, err := actions.SellAll(event)
	if err != nil {
		t.Fatalf("SellAll returned error: %v", err)
	}
	if got.Trade.PositionType != "sellParent" {
		t.Errorf("parent PositionType = %q, want sellParent", got.Trade.PositionType)
	}
	if sellCalls != 1 {
		t.Errorf("expected sell called once for child with history, got %d", sellCalls)
	}
	// updateTrade fires once per child chain plus once for the parent.
	if updateCalls != 3 {
		t.Errorf("expected updateTrade called three times, got %d", updateCalls)
	}
}
