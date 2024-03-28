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
	exchangeInit := exchange.Exchange{Name: event.Exchange.Name, ApiKey: event.Exchange.ApiKey, ApiSecret: event.Exchange.ApiSecret, TestNet: event.Exchange.TestNet}
	client, _ := exchangeInit.Client()
	assets, assetsErr := client.GetUserAssets() // Get user balance

	if assetsErr != nil {
		return events.Events{}, fmt.Errorf("could not fetch user assets")
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

	buyQuantity, _ := trades.GetQuantities(event.Trade.History)
	if remainedQuantity < buyQuantity*event.Trade.Position.Price {
		// If nou enough funds log and return
		msg := fmt.Sprintf("Not enough funds to buy, available qty: %f, necessary qty: %f", remainedQuantity, buyQuantity*event.Trade.Position.Price)
		return events.Events{}, fmt.Errorf(msg)
	}

	return event, nil
}

// TODO: action to to verify if trade is on profit
func HasProfit(event events.Events) (events.Events, error) {
	simulateHistory := event.Trade.History
	_, feeInQuote := CalculateFees(event.Trade.History, event.Trade.Symbol)
	buyQuantity, _ := trades.GetQuantities(event.Trade.History)
	simulateHistory = append(simulateHistory, aggragates.History{Type: "sell", Quantity: buyQuantity})
	profit := GetProfit(simulateHistory)
	if profit-feeInQuote < 0 {
		return events.Events{}, fmt.Errorf("profit is smaller then min profit")
	}
	return event, nil
}

func Buy(event events.Events) (events.Events, error) {
	exchangeInit := exchange.Exchange{Name: event.Exchange.Name, ApiKey: event.Exchange.ApiKey, ApiSecret: event.Exchange.ApiSecret, TestNet: event.Exchange.TestNet}
	client, clientError := exchangeInit.Client()
	if clientError != nil {
		return events.Events{}, clientError
	}
	buyQuantity, _ := trades.GetQuantities(event.Trade.History)
	priceInString := strconv.FormatFloat(event.Trade.Position.Price, 'f', -1, 64)

	var response aggregates.CreateOrderResponse
	var err error

	if len(event.Trade.History) > 0 {
		response, err = client.Buy(event.Trade.Symbol, buyQuantity, priceInString)
	} else {
		response, err = client.MarketBuy(event.Trade.Symbol, buyQuantity)
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
	buyQuantity, _ := trades.GetQuantities(event.Trade.History)
	feeInBase, _ := CalculateFees(event.Trade.History, event.Trade.Symbol)
	priceInString := strconv.FormatFloat(event.Trade.Position.Price, 'f', -1, 64)

	response, err := client.Sell(event.Trade.Symbol, buyQuantity-feeInBase, priceInString)

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
