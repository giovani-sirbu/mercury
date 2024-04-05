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

	quantity := trades.GetQuantityByHistory(event.Trade.History)
	if remainedQuantity < quantity*event.Trade.Position.Price {
		// If nou enough funds log and return
		msg := fmt.Sprintf("Not enough funds to buy, available qty: %f, necessary qty: %f", remainedQuantity, quantity*event.Trade.Position.Price)
		return events.Events{}, fmt.Errorf(msg)
	}

	return event, nil
}

func HasProfit(event events.Events) (events.Events, error) {
	simulateHistory := event.Trade.History
	_, feeInQuote := CalculateFees(event.Trade.History, event.Trade.Symbol)
	quantity := trades.GetQuantityByHistory(event.Trade.History)
	simulateHistory = append(simulateHistory, aggragates.History{Type: "sell", Quantity: quantity, Price: event.Trade.Position.Price})
	profit := GetProfit(simulateHistory)
	if profit-feeInQuote < 0 {
		msg := fmt.Sprintf("profit: %f is smaller then min profit", profit)
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
	quantity := trades.GetQuantityByHistory(event.Trade.History)

	if quantity == 0 {
		quantity = event.TradeSettings.MinNotion / event.Trade.Position.Price
	}

	priceInString := strconv.FormatFloat(event.Trade.Position.Price, 'f', -1, 64)
	quantity = ToFixed(quantity*event.Trade.Settings.Multiplier, event.TradeSettings.LotSize)

	var response aggregates.CreateOrderResponse
	var err error

	if len(event.Trade.History) > 0 {
		response, err = client.Buy(event.Trade.Symbol, quantity, priceInString)
	} else {
		response, err = client.MarketBuy(event.Trade.Symbol, quantity)
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
	quantity := ToFixed(buyQuantity-sellQuantity-feeInBase, event.TradeSettings.LotSize)
	priceInString := strconv.FormatFloat(event.Trade.Position.Price, 'f', -1, 64)

	response, err := client.Sell(event.Trade.Symbol, quantity, priceInString)

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

func GetProfit(history []aggragates.History) float64 {
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
	return sellTotal - buyTotal
}

func ToFixed(num float64, precision int) float64 {
	output := math.Pow(10, float64(precision))
	return math.Round(num*output) / output
}
