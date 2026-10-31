package actions

import (
	"testing"

	"github.com/adshao/go-binance/v2/common"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/exchange"
	exchangeAggregates "github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// denominationTrade is a one-fill trade priced so its profit is easy to read.
func denominationTrade(symbol string, inverse bool, quantity, price float64) aggragates.Trades {
	side := "buy"
	if inverse {
		side = "sell"
	}
	trade := aggragates.Trades{Symbol: symbol, Inverse: inverse}
	trade.History = []aggragates.TradesHistory{{Type: side, Quantity: quantity, Price: price}}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{LotSize: 8, MinNotional: 1, PriceFilter: 2}
	trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{{Percentage: 2, Tolerance: 0}}
	return trade
}

func parentEventWithChild(parent, child aggragates.Trades, childPrice float64) events.Events {
	return events.Events{
		Exchange: exchange.Exchange{IsCustom: true, CustomActions: exchangeAggregates.Actions{
			GetPrice: func(string) (float64, *common.APIError) { return childPrice, nil },
		}},
		Trade:          parent,
		ChildrenTrades: []aggragates.Trades{child},
		Events: map[string]func(events.Events) (events.Events, error){
			"hasProfit": HasProfit,
		},
	}
}

// A child's profit is an amount of quote, the same unit the parent's own leg
// is measured in, so it is added. Scaling it by the parent's position price
// treated it as an amount of the parent's BASE asset: a BTC parent at 100000
// turned a child's few dollars into a few hundred thousand, and every impasse
// close passed on children profit alone.
//
// Here the parent is 1000 USDT underwater and the spot child is 1 USDT up, so
// the combined result must still be a refusal. Under the old scaling the child
// contributed 100000 and the close was accepted.
func TestParentTradeHasProfitAddsChildrenProfitWithoutScalingByParentPrice(t *testing.T) {
	parent := denominationTrade("BTC/USDT", false, 1, 100000)
	parent.PositionPrice = 99000

	child := denominationTrade("ETH/USDT", false, 1, 100)

	_, err := ParentTradeHasProfit(parentEventWithChild(parent, child, 101))

	if err == nil {
		t.Fatal("a 1 USDT child must not cover a 1000 USDT parent loss")
	}
}

// The same parent, with a child whose profit genuinely does cover the loss,
// must still close — otherwise the fix above would have disabled the feature.
func TestParentTradeHasProfitStillClosesWhenChildrenCoverTheLoss(t *testing.T) {
	parent := denominationTrade("BTC/USDT", false, 1, 100000)
	parent.PositionPrice = 99000

	child := denominationTrade("ETH/USDT", false, 100, 100)

	if _, err := ParentTradeHasProfit(parentEventWithChild(parent, child, 121)); err != nil {
		t.Fatalf("a 2100 USDT child profit must cover a 1000 USDT parent loss, got %v", err)
	}
}

// An inverse child answers in its own BASE asset, so its own price converts it
// — the parent's price is unrelated to the child's symbol.
func TestParentTradeHasProfitConvertsAnInverseChildWithItsOwnPrice(t *testing.T) {
	parent := denominationTrade("BTC/USDT", false, 1, 100000)
	parent.PositionPrice = 99000

	// Sold 100 ETH at 100 USDT, buying back at 50: ~50 ETH of base profit,
	// which its own price turns into ~2500 USDT.
	child := denominationTrade("ETH/USDT", true, 100, 100)

	if _, err := ParentTradeHasProfit(parentEventWithChild(parent, child, 50)); err != nil {
		t.Fatalf("the inverse child's base profit must convert at its own price, got %v", err)
	}
}
