package actions

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/exchange"
	"github.com/giovani-sirbu/mercury/trades"
	"strconv"
)

// TODO: action to update trade
func UpdateTrade(event events.Events) (events.Events, error) {
	//tradeInBytes, _ := json.Marshal(event.Trade)
	//event.Broker.Producer(tradeInBytes) // Move message broker into Mercury
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

// TODO: action to to verify if account still have funds
func HasFunds(event events.Events) (events.Events, error) {
	fmt.Println("has funds", event.Trade.Symbol)
	return event, nil
}

// TODO: action to to verify if trade is on profit
func HasProfit(event events.Events) (events.Events, error) {
	fmt.Println("has funds", event.Trade.Symbol)
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

	response, err := client.Buy(event.Trade.Symbol, buyQuantity, priceInString)

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
	priceInString := strconv.FormatFloat(event.Trade.Position.Price, 'f', -1, 64)

	response, err := client.Buy(event.Trade.Symbol, buyQuantity, priceInString)

	event.Trade.PendingOrder = response.OrderID

	if err != nil {
		return events.Events{}, err
	}
	return event, nil
}

// TODO: action to create a new trade with same settings
func DuplicateTrade(event events.Events) (events.Events, error) {
	//tradeInBytes, _ := json.Marshal(event.Trade)
	//event.Broker.Producer(tradeInBytes) // Move message broker into Mercury
	return event, nil
}
