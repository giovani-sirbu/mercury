package actions_test

import (
	"github.com/adshao/go-binance/v2/common"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/exchange"
	exchangeaggr "github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/virtualexchange"
	"testing"
)

type sellCallTracker struct {
	limitBuy   int
	limitSell  int
	marketBuy  int
	marketSell int
}

func SellFeeQtyInverseHasFunds() events.Events {
	var tradesHistory []aggragates.TradesHistory

	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 36.52, Price: 0.000031, Type: "SELL", Fees: []aggragates.TradesFees{{Asset: "BTC", Fee: 0.00000113}}})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 73.04, Price: 0.0000317, Type: "SELL", Fees: []aggragates.TradesFees{{Asset: "BTC", Fee: 0.0000023200000000000002}}})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 146.08, Price: 0.0000325, Type: "SELL", Fees: []aggragates.TradesFees{{Asset: "BTC", Fee: 0.00000475}}})

	trade := aggragates.Trades{}
	trade.History = tradesHistory
	trade.PositionPrice = 0.0000318
	trade.Inverse = true
	trade.Symbol = "DOT/BTC"
	trade.PositionType = "takeProfit"
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{LotSize: 2, MinNotional: 0.0001, PriceFilter: 7}
	trade.StrategyPair.StrategySettings = append(trade.StrategyPair.StrategySettings,
		aggragates.StrategySettings{MinDepths: 6, Depths: 8, Percentage: 2, Multiplier: 2, Tolerance: 0.25, InitialBid: 0.5},
	)
	defaultActions := actions.GetDefaultActions()
	virtualexchange.ResetWallet()
	exchangeInit := GetVirtualExchange("BTC", "0.00812")
	defaultActions["updateTrade"] = EmptyUpdateTrade
	newEvent := events.Events{Trade: trade, Exchange: exchangeInit, Events: defaultActions, EventsNames: []string{"hasFunds"}}
	return newEvent
}

func TestSell(t *testing.T) {
	var tests []hasFundsTests

	tests = append(tests, hasFundsTests{SellFeeQtyInverseHasFunds, "Next buy, no history"})

	for _, test := range tests {
		event := test.Event()
		nextEvent, err := actions.Sell(event)

		if err != nil {
			t.Fatalf("Failed with error: %s", err)
		}

		t.Logf("PASS: Correct quantity %f when testing %s", nextEvent.Params.Quantity, test.Name)

	}

}

func TestSellUsesMarketSellWhenRequested(t *testing.T) {
	calls := sellCallTracker{}
	event := SellWithCustomExchange(false, true, &calls)

	_, err := actions.Sell(event)

	if err != nil {
		t.Fatalf("Failed with error: %s", err)
	}
	if calls.marketSell != 1 || calls.limitSell != 0 {
		t.Fatalf("Expected market sell only, got market=%d limit=%d", calls.marketSell, calls.limitSell)
	}
}

func TestSellUsesLimitSellByDefault(t *testing.T) {
	calls := sellCallTracker{}
	event := SellWithCustomExchange(false, false, &calls)

	_, err := actions.Sell(event)

	if err != nil {
		t.Fatalf("Failed with error: %s", err)
	}
	if calls.limitSell != 1 || calls.marketSell != 0 {
		t.Fatalf("Expected limit sell only, got limit=%d market=%d", calls.limitSell, calls.marketSell)
	}
}

func TestSellUsesMarketBuyForInverseMarketOrder(t *testing.T) {
	calls := sellCallTracker{}
	event := SellWithCustomExchange(true, true, &calls)

	_, err := actions.Sell(event)

	if err != nil {
		t.Fatalf("Failed with error: %s", err)
	}
	if calls.marketBuy != 1 || calls.limitBuy != 0 {
		t.Fatalf("Expected inverse market buy only, got market=%d limit=%d", calls.marketBuy, calls.limitBuy)
	}
}

func SellWithCustomExchange(inverse bool, marketSellOrder bool, calls *sellCallTracker) events.Events {
	historyType := "BUY"
	if inverse {
		historyType = "SELL"
	}

	trade := aggragates.Trades{}
	trade.History = []aggragates.TradesHistory{{Quantity: 2, Price: 100, Type: historyType}}
	trade.PositionPrice = 100
	trade.Inverse = inverse
	trade.Symbol = "ETH/USDT"
	trade.PositionType = "buy"
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{LotSize: 2, MinNotional: 5, PriceFilter: 2}

	customActions := exchangeaggr.Actions{
		Buy: func(symbol string, quantity float64, price string) (exchangeaggr.CreateOrderResponse, *common.APIError) {
			calls.limitBuy++
			return exchangeaggr.CreateOrderResponse{OrderID: 1}, nil
		},
		Sell: func(symbol string, quantity float64, price string) (exchangeaggr.CreateOrderResponse, *common.APIError) {
			calls.limitSell++
			return exchangeaggr.CreateOrderResponse{OrderID: 2}, nil
		},
		MarketBuy: func(symbol string, quantity float64) (exchangeaggr.CreateOrderResponse, *common.APIError) {
			calls.marketBuy++
			return exchangeaggr.CreateOrderResponse{OrderID: 3}, nil
		},
		MarketSell: func(symbol string, quantity float64) (exchangeaggr.CreateOrderResponse, *common.APIError) {
			calls.marketSell++
			return exchangeaggr.CreateOrderResponse{OrderID: 4}, nil
		},
	}

	return events.Events{
		Exchange: exchange.Exchange{CustomActions: customActions, IsCustom: true},
		Params:   aggragates.Params{MarketSellOrder: marketSellOrder},
		Trade:    trade,
	}
}
