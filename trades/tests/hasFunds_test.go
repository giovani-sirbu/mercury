package tests

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/virtualExchange"
	"testing"
)

type hasFundsTests struct {
	Event func() events.Events
	Name  string
}

func TakeProfitSNCaseHasFunds() events.Events {
	var tradesHistory []aggragates.TradesHistory
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 3.29, Price: 14.17, Type: "BUY"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 6.58, Price: 13.96, Type: "BUY"})

	trade := aggragates.Trades{}
	trade.History = tradesHistory
	trade.PositionPrice = 14.46
	trade.Inverse = false
	trade.Symbol = "LINK/USDT"
	trade.PositionType = "takeProfit"
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{LotSize: 2, MinNotional: 5, PriceFilter: 2}
	trade.StrategyPair.StrategySettings = append(trade.StrategyPair.StrategySettings,
		aggragates.StrategySettings{MinDepths: 6, Depths: 8, Percentage: 2, Multiplier: 2, Tolerance: 0.25, InitialBid: 0.5},
	)
	defaultActions := actions.GetDefaultActions()
	virtualExchange.ResetWallet()
	exchangeInit := GetVirtualExchange("LINK", "9.87")
	defaultActions["updateTrade"] = EmptyUpdateTrade
	newEvent := events.Events{Trade: trade, Exchange: exchangeInit, Events: defaultActions, EventsNames: []string{"hasFunds"}}
	return newEvent
}

func TakeProfitCaseHasFunds() events.Events {
	var tradesHistory []aggragates.TradesHistory
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{
		Quantity: 25.38, Price: 5.056, Type: "BUY",
		Fees: []aggragates.TradesFees{{Asset: "DOT", Fee: 0.02538}},
	})

	tradesHistory = append(tradesHistory, aggragates.TradesHistory{
		Quantity: 50.76, Price: 4.944, Type: "BUY",
		Fees: []aggragates.TradesFees{{Asset: "DOT", Fee: 0.05076}},
	})

	tradesHistory = append(tradesHistory, aggragates.TradesHistory{
		Quantity: 101.52, Price: 4.799, Type: "BUY",
		Fees: []aggragates.TradesFees{{Asset: "DOT", Fee: 0.10152}},
	})

	tradesHistory = append(tradesHistory, aggragates.TradesHistory{
		Quantity: 203.04, Price: 4.693, Type: "BUY",
		Fees: []aggragates.TradesFees{{Asset: "DOT", Fee: 0.20304}},
	})

	tradesHistory = append(tradesHistory, aggragates.TradesHistory{
		Quantity: 406.08, Price: 4.471, Type: "BUY",
		Fees: []aggragates.TradesFees{{Asset: "DOT", Fee: 0.40608}},
	})

	tradesHistory = append(tradesHistory, aggragates.TradesHistory{
		Quantity: 812.16, Price: 4.373, Type: "BUY",
		Fees: []aggragates.TradesFees{{Asset: "DOT", Fee: 0.81216}},
	})

	trade := aggragates.Trades{}
	trade.History = tradesHistory
	trade.PositionPrice = 4.272
	trade.Inverse = false
	trade.Symbol = "DOT/USDT"
	trade.PositionType = "takeProfit"
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{LotSize: 2, MinNotional: 5, PriceFilter: 3}
	trade.StrategyPair.StrategySettings = append(trade.StrategyPair.StrategySettings,
		aggragates.StrategySettings{MinDepths: 6, Depths: 8, Percentage: 2, Multiplier: 2, Tolerance: 0.25, InitialBid: 0.5},
	)
	defaultActions := actions.GetDefaultActions()
	virtualExchange.ResetWallet()
	exchangeInit := GetVirtualExchange("DOT", "1597.645002")
	defaultActions["updateTrade"] = EmptyUpdateTrade
	newEvent := events.Events{Trade: trade, Exchange: exchangeInit, Events: defaultActions, EventsNames: []string{"hasFunds"}}
	return newEvent
}

func TakeProfitInverseCaseHasFunds() events.Events {
	var tradesHistory []aggragates.TradesHistory

	tradesHistory = append(tradesHistory, aggragates.TradesHistory{
		Quantity: 1.38, Price: 0.0001624, Type: "SELL",
		Fees: []aggragates.TradesFees{{Asset: "BTC", Fee: 2.2e-7}},
	})

	tradesHistory = append(tradesHistory, aggragates.TradesHistory{
		Quantity: 2.76, Price: 0.0001675, Type: "SELL",
		Fees: []aggragates.TradesFees{{Asset: "BTC", Fee: 4.6e-7}},
	})

	trade := aggragates.Trades{}
	trade.History = tradesHistory
	trade.PositionPrice = 0.0001624
	trade.Inverse = true
	trade.Symbol = "LINK/BTC"
	trade.PositionType = "takeProfit"
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{LotSize: 2, MinNotional: 0.0001, PriceFilter: 7}
	trade.StrategyPair.StrategySettings = append(trade.StrategyPair.StrategySettings,
		aggragates.StrategySettings{MinDepths: 6, Depths: 8, Percentage: 2, Multiplier: 2, Tolerance: 0.25, InitialBid: 0.5},
	)
	defaultActions := actions.GetDefaultActions()
	virtualExchange.ResetWallet()
	exchangeInit := GetVirtualExchange("BTC", "0.000685732")
	defaultActions["updateTrade"] = EmptyUpdateTrade
	newEvent := events.Events{Trade: trade, Exchange: exchangeInit, Events: defaultActions, EventsNames: []string{"hasFunds"}}
	return newEvent
}

func FeeQtyHasFunds() events.Events {
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
	virtualExchange.ResetWallet()
	exchangeInit := GetVirtualExchange("BTC", "0.00813")
	defaultActions["updateTrade"] = EmptyUpdateTrade
	newEvent := events.Events{Trade: trade, Exchange: exchangeInit, Events: defaultActions, EventsNames: []string{"hasFunds"}}
	return newEvent
}

func TestHasFunds(t *testing.T) {
	var tests []hasFundsTests

	tests = append(tests, hasFundsTests{TakeProfitSNCaseHasFunds, "Next buy, no history"})
	tests = append(tests, hasFundsTests{TakeProfitCaseHasFunds, "Next buy, no history"})
	tests = append(tests, hasFundsTests{TakeProfitInverseCaseHasFunds, "Next buy, no history"})
	tests = append(tests, hasFundsTests{FeeQtyHasFunds, "Next buy, no history"})

	for _, test := range tests {
		event := test.Event()
		nextEvent, err := actions.HasFunds(event)

		if err != nil {
			t.Fatalf("Failed with error: %s", err)
		}

		t.Logf("PASS: Correct quantity %f when testing %s", nextEvent.Params.Quantity, test.Name)

	}

}
