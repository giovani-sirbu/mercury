package actions

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/trades"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
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

	quantityBeforeLotSize := quantity
	var dust float64
	quantity = ToFixed(quantity, event.TradeSettings.LotSize)

	if quantityBeforeLotSize > quantity {
		dust = quantityBeforeLotSize - quantity
	}

	var response aggregates.CreateOrderResponse
	var err error

	if dust > 0 {
		if event.Trade.Inverse {
			response, err = client.MarketBuy(event.Trade.Symbol, quantity)
		} else {
			response, err = client.MarketSell(event.Trade.Symbol, quantity)
		}
		if err == nil {
			priceInFloat, _ := strconv.ParseFloat(response.Price, 64)
			qtyInFloat, _ := strconv.ParseFloat(response.ExecutedQuantity, 64)
			history := aggragates.TradesHistory{Type: "sell", Price: priceInFloat, Quantity: qtyInFloat, OrderId: response.OrderID}

			if event.Trade.Inverse {
				history.Type = "buy"
			}

			event.Trade.History = append(event.Trade.History, history)
		}
	}

	priceInString := strconv.FormatFloat(event.Trade.PositionPrice, 'f', -1, 64)
	event.Params.Quantity = quantity

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
