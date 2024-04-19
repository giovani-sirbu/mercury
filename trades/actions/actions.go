package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/exchange"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/trades"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"math"
	"strconv"
	"strings"
)

func UpdateTrade(event events.Events) (events.Events, error) {
	tradeInBytes, _ := json.Marshal(event.Trade)
	topic := "update-trade"
	event.Broker.Producer(topic, context.Background(), nil, tradeInBytes)
	return event, nil
}

func CancelPendingOrder(event events.Events) (events.Events, error) {
	if event.Trade.PendingOrder != 0 {
		exchangeInit := exchange.Exchange{Name: event.Exchange.Name, ApiKey: event.Exchange.ApiKey, ApiSecret: event.Exchange.ApiSecret, TestNet: event.Exchange.TestNet}
		client, clientError := exchangeInit.Client()
		if clientError != nil {
			return events.Events{}, clientError
		}
		_, err := client.CancelOrder(event.Trade.PendingOrder, event.Trade.Symbol)

		if err != nil {
			return events.Events{}, err
		}
		return event, nil
	} else {
		event.Trade.PendingOrder = 0
		return event, nil
	}
}

func HasFunds(event events.Events) (events.Events, error) {
	if event.Exchange.TestNet {
		return event, nil
	}
	exchangeInit := exchange.Exchange{Name: event.Exchange.Name, ApiKey: event.Exchange.ApiKey, ApiSecret: event.Exchange.ApiSecret, TestNet: event.Exchange.TestNet}
	client, _ := exchangeInit.Client()
	assets, assetsErr := client.GetUserAssets() // Get user balance

	if assetsErr != nil {
		return events.Events{}, assetsErr
	}

	var remainedQuantity float64 // Init needed quantity

	// Check if account has remaining balance for pair
	for _, balance := range assets {
		pairSymbols := strings.Split(event.Trade.Symbol, "/")
		if balance.Asset == pairSymbols[1] {
			floatQuantity, _ := strconv.ParseFloat(balance.Free, 64)
			remainedQuantity = floatQuantity
		}
	}

	quantity := trades.GetQuantityByHistory(event.Trade.History, event.Trade.Inverse)
	if remainedQuantity < quantity*event.Trade.PositionPrice {
		// If nou enough funds log and return
		msg := fmt.Sprintf("Not enough funds to buy, available qty: %f, necessary qty: %f", remainedQuantity, quantity*event.Trade.PositionPrice)
		return events.Events{}, fmt.Errorf(msg)
	}

	return event, nil
}

func HasProfit(event events.Events) (events.Events, error) {
	simulateHistory := event.Trade.History
	feeInBase, feeInQuote := CalculateFees(event.Trade.History, event.Trade.Symbol)
	buyQty, _ := trades.GetQuantities(event.Trade.History)
	quantity := buyQty
	historyType := "sell"

	if event.Trade.Inverse {
		quantity = trades.GetQuantityInQuote(event.Trade.History)
		quantity = quantity / event.Trade.PositionPrice
		quantity = ToFixed(quantity, event.TradeSettings.LotSize)
		historyType = "buy"
	}

	simulateHistory = append(simulateHistory, aggragates.History{Type: historyType, Quantity: quantity, Price: event.Trade.PositionPrice})
	sellTotal, buyTotal := GetProfit(simulateHistory)
	profit := sellTotal - buyTotal
	fee := feeInQuote
	if event.Trade.Inverse {
		fee = feeInBase
		_, profit = GetProfitInBase(simulateHistory)
	}
	if profit-fee < 0 {
		msg := fmt.Sprintf("profit: %f is smaller then min profit", profit-fee)
		return events.Events{}, fmt.Errorf(msg)
	}
	return event, nil
}

func Buy(event events.Events) (events.Events, error) {
	exchangeInit := exchange.Exchange{Name: event.Exchange.Name, ApiKey: event.Exchange.ApiKey, ApiSecret: event.Exchange.ApiSecret, TestNet: event.Exchange.TestNet}
	client, clientError := exchangeInit.Client()
	if clientError != nil {
		return events.Events{}, clientError
	}
	quantity := trades.GetQuantityByHistory(event.Trade.History, event.Trade.Inverse)

	historyCount := len(event.Trade.History)
	settings := []byte(event.Trade.Strategy.Params)
	var StrategySettings []aggragates.StrategyParams
	var settingsIndex int

	json.Unmarshal(settings, &StrategySettings)

	if historyCount > len(StrategySettings) {
		settingsIndex = len(StrategySettings) - 1
	} else {
		settingsIndex = historyCount - 1
	}

	if historyCount == 0 {
		settingsIndex = 0
	}

	initialBid := StrategySettings[settingsIndex].InitialBid
	multiplier := StrategySettings[settingsIndex].Multiplier

	if quantity == 0 {
		quantity = (event.TradeSettings.MinNotion / event.Trade.PositionPrice) * initialBid
	}

	priceInString := strconv.FormatFloat(event.Trade.PositionPrice, 'f', -1, 64)
	quantity = ToFixed(quantity*multiplier, event.TradeSettings.LotSize)

	var response aggregates.CreateOrderResponse
	var err error

	if historyCount > 0 {
		if event.Trade.Inverse {
			response, err = client.Sell(event.Trade.Symbol, quantity, priceInString)
		} else {
			response, err = client.Buy(event.Trade.Symbol, quantity, priceInString)
		}
	} else {
		if event.Trade.Inverse {
			response, err = client.MarketSell(event.Trade.Symbol, quantity)
		} else {
			response, err = client.MarketBuy(event.Trade.Symbol, quantity)
		}
	}

	event.Trade.PendingOrder = response.OrderID

	if err != nil {
		return events.Events{}, err
	}
	return event, nil
}

func Sell(event events.Events) (events.Events, error) {
	exchangeInit := exchange.Exchange{Name: event.Exchange.Name, ApiKey: event.Exchange.ApiKey, ApiSecret: event.Exchange.ApiSecret, TestNet: event.Exchange.TestNet}
	client, clientError := exchangeInit.Client()
	if clientError != nil {
		return events.Events{}, clientError
	}
	buyQuantity, sellQuantity := trades.GetQuantities(event.Trade.History)
	feeInBase, _ := CalculateFees(event.Trade.History, event.Trade.Symbol)
	quantity := buyQuantity - sellQuantity - feeInBase

	if event.Trade.Inverse {
		quantity = trades.GetQuantityInQuote(event.Trade.History)
		quantity = quantity / event.Trade.PositionPrice
	}

	quantity = ToFixed(quantity, event.TradeSettings.LotSize)
	priceInString := strconv.FormatFloat(event.Trade.PositionPrice, 'f', -1, 64)

	var response aggregates.CreateOrderResponse
	var err error

	if event.Trade.Inverse {
		response, err = client.Buy(event.Trade.Symbol, quantity, priceInString)
	} else {
		response, err = client.Sell(event.Trade.Symbol, quantity, priceInString)
	}

	event.Trade.PendingOrder = response.OrderID

	if err != nil {
		return events.Events{}, err
	}
	return event, nil
}

func DuplicateTrade(event events.Events) (events.Events, error) {
	tradeInBytes, _ := json.Marshal(event.Trade)
	topic := "duplicate-trade"
	event.Broker.Producer(topic, context.Background(), nil, tradeInBytes)
	return event, nil
}

func CalculateFees(history []aggragates.History, symbol string) (float64, float64) {
	var feeInBase = 0.0
	var feeInQuote = 0.0

	for _, historyData := range history {
		baseSymbol := strings.Split(symbol, "/")[0]

		if len(historyData.FeeDetails) > 0 {
			for _, feeDetail := range historyData.FeeDetails {
				feeInQuote = feeInQuote + feeDetail.FeeInQuote
				if baseSymbol == feeDetail.Asset {
					feeInBase = feeInBase + feeDetail.Fee
				}
			}
		}
	}

	return feeInBase, feeInQuote
}

func GetProfit(history []aggragates.History) (float64, float64) {
	var buyTotal float64
	var sellTotal float64
	for _, historyData := range history {
		if strings.ToLower(historyData.Type) == "buy" {
			buyPerHistory := historyData.Price * historyData.Quantity
			buyTotal += buyPerHistory
		} else {
			sellPerHistory := historyData.Price * historyData.Quantity
			sellTotal += sellPerHistory
		}
	}
	return sellTotal, buyTotal
}

func GetProfitInBase(history []aggragates.History) (float64, float64) {
	var buyTotal float64
	var sellTotal float64
	for _, historyData := range history {
		if strings.ToLower(historyData.Type) == "buy" {
			buyTotal += historyData.Quantity
		} else {
			sellTotal += historyData.Quantity
		}
	}
	return sellTotal, buyTotal
}

func ToFixed(num float64, precision int) float64 {
	output := math.Pow(10, float64(precision))
	return math.Round(num*output) / output
}
