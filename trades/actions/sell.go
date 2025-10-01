package actions

import (
	"fmt"
	"github.com/adshao/go-binance/v2/common"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/log"
	"github.com/giovani-sirbu/mercury/trades"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"strconv"
)

func Sell(event events.Events) (events.Events, error) {
	if event.Trade.PendingOrder != 0 {
		msg := fmt.Sprintf("Trade already have pending id %d", event.Trade.PendingOrder)
		return event, fmt.Errorf(msg)
	}
	if event.Trade.Status == "new" || event.Params.OldPosition == "new" {
		event.Trade.Status = "closed"
		return event, nil
	}
	client, clientError := event.Exchange.Client()
	if clientError != nil {
		return SaveError(event, clientError)
	}
	buyQuantity, sellQuantity := trades.GetQuantitiesOld(event.Trade.History)
	feeInBase, feeInQuote := CalculateFeesOld(event)
	quantity := buyQuantity - sellQuantity - feeInBase

	if event.Trade.Inverse {
		sellQuantity = trades.GetQuantityInQuote(event.Trade.History, "BUY")
		buyQuantity = trades.GetQuantityInQuote(event.Trade.History, "SELL")
		quantity = sellQuantity - buyQuantity - feeInQuote
		quantity = quantity / event.Trade.PositionPrice
		quantity = quantity - feeInBase // subtract fee in base for the partial cases
	}

	quantityBeforeLotSize := quantity
	var dust float64
	quantity = ToFixed(quantity, int(event.Trade.StrategyPair.TradeFilters.LotSize))

	// if no bought quantity, update event status and close it
	if quantity <= 0 {
		event.Trade.Status = aggragates.Closed
		return event, nil
	}

	if quantityBeforeLotSize > quantity {
		dust = quantityBeforeLotSize - quantity
	}

	var response aggregates.CreateOrderResponse
	var err *common.APIError

	// TODO - find a solution to sell dust assets
	if dust > 0 && false {
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
	event.Trade.Dust = dust

	if err != nil {
		return SaveError(event, err)
	}

	log.Debug(fmt.Sprintf("Sell(TradeID:#%d): PositionPrice(%f), quantity(%f)", event.Trade.ID, event.Trade.PositionPrice, quantity))

	return event, nil
}
