package actions

import (
	"testing"

	"github.com/adshao/go-binance/v2/common"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/exchange"
	exchangeAggregates "github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func TestParentTradeHasProfitSucceedsWhenSimulatedProfitPositive(t *testing.T) {
	trade := aggragates.Trades{Symbol: "BTC/USDT", PositionPrice: 200}
	trade.History = []aggragates.TradesHistory{{Type: "buy", Quantity: 1, Price: 100}}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{LotSize: 2, MinNotional: 10, PriceFilter: 2}
	trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{{Percentage: 2, Tolerance: 0}}

	event := events.Events{Trade: trade}

	_, err := ParentTradeHasProfit(event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestParentTradeHasProfitAddsChildrenProfitFromExchangePrice(t *testing.T) {
	parent := aggragates.Trades{Symbol: "BTC/USDT", PositionPrice: 200}
	parent.History = []aggragates.TradesHistory{{Type: "buy", Quantity: 1, Price: 100}}
	parent.StrategyPair.TradeFilters = aggragates.TradeFilters{LotSize: 2, MinNotional: 10, PriceFilter: 2}
	parent.StrategyPair.StrategySettings = []aggragates.StrategySettings{{Percentage: 2, Tolerance: 0}}

	child := aggragates.Trades{Symbol: "ETH/USDT", PositionPrice: 50}
	child.History = []aggragates.TradesHistory{{Type: "buy", Quantity: 1, Price: 25}}
	child.StrategyPair.TradeFilters = aggragates.TradeFilters{LotSize: 2, MinNotional: 5, PriceFilter: 2}
	child.StrategyPair.StrategySettings = []aggragates.StrategySettings{{Percentage: 2, Tolerance: 0}}

	customActions := exchangeAggregates.Actions{
		GetPrice: func(symbol string) (float64, *common.APIError) {
			return 60, nil
		},
	}

	hasProfit := func(event events.Events) (events.Events, error) {
		return HasProfit(event)
	}

	event := events.Events{
		Exchange:       exchange.Exchange{IsCustom: true, CustomActions: customActions},
		Trade:          parent,
		ChildrenTrades: []aggragates.Trades{child},
		Events: map[string]func(events.Events) (events.Events, error){
			"hasProfit": hasProfit,
		},
	}

	_, err := ParentTradeHasProfit(event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestParentTradeHasProfitFailsWhenChildPriceLookupFails(t *testing.T) {
	parent := aggragates.Trades{Symbol: "BTC/USDT", PositionPrice: 200}
	parent.History = []aggragates.TradesHistory{{Type: "buy", Quantity: 1, Price: 100}}
	parent.StrategyPair.TradeFilters = aggragates.TradeFilters{LotSize: 2, MinNotional: 10, PriceFilter: 2}
	parent.StrategyPair.StrategySettings = []aggragates.StrategySettings{{Percentage: 2, Tolerance: 0}}

	child := aggragates.Trades{Symbol: "ETH/USDT"}

	customActions := exchangeAggregates.Actions{
		GetPrice: func(symbol string) (float64, *common.APIError) {
			return 0, nil
		},
	}

	event := events.Events{
		Exchange:       exchange.Exchange{IsCustom: true, CustomActions: customActions},
		Trade:          parent,
		ChildrenTrades: []aggragates.Trades{child},
		Events: map[string]func(events.Events) (events.Events, error){
			"hasProfit": HasProfit,
		},
	}

	_, err := ParentTradeHasProfit(event)
	if err == nil {
		t.Fatal("expected error when child price lookup returns 0")
	}
}
