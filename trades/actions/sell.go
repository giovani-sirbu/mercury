package actions

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/trades"
	"strconv"
)

func Sell(event events.Events) (events.Events, error) {
	client, clientError := event.Exchange.Client()
	if clientError != nil {
		return SaveError(event, clientError)
	}
	buyQuantity, sellQuantity := trades.GetQuantities(event.Trade.History)
	feeInBase, _ := CalculateFees(event.Trade.History, event.Trade.Symbol)
	quantity := buyQuantity - sellQuantity - feeInBase

	if event.Trade.Inverse {
		sellQuantity = trades.GetQuantityInQuote(event.Trade.History, "BUY")
		buyQuantity = trades.GetQuantityInQuote(event.Trade.History, "SELL")
		quantity = sellQuantity - buyQuantity
		quantity = quantity / event.Trade.PositionPrice
		quantity = quantity - feeInBase
	}

	quantity = ToFixed(quantity, event.TradeSettings.LotSize)
	priceInString := strconv.FormatFloat(event.Trade.PositionPrice, 'f', -1, 64)
	event.Params.Quantity = quantity

	var response aggregates.CreateOrderResponse
	var err error

	if event.Trade.Inverse {
		response, err = client.Buy(event.Trade.Symbol, quantity, priceInString)
	} else {
		response, err = client.Sell(event.Trade.Symbol, quantity, priceInString)
	}

	event.Trade.PendingOrder = response.OrderID

	if err != nil {
		return SaveError(event, err)
	}
	return event, nil
}
