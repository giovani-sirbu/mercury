package actions

import (
	"fmt"
	"strconv"

	"github.com/adshao/go-binance/v2/common"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/log"
	"github.com/giovani-sirbu/mercury/trades"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
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
	buyQty, sellQty := GetGrossQuantities(event)
	// Literal asset fees: only base-paid fees reduce the base wallet balance,
	// only quote-paid fees reduce the quote wallet balance. BNB/third-asset
	// fees come from a separate wallet and must NOT be subtracted from the
	// trade's base or quote totals here. For profit accounting that needs the
	// full cost-in-denomination, use GetFeesBaseQuote / GetFees instead.
	feeInBase, feeInQuote := CalculateFees(event)
	quantity := buyQty - sellQty - feeInBase

	if event.Trade.Inverse {
		// Inverse trades need quote-denominated totals: each fill's quantity is
		// re-aggregated multiplied by its own price. Then we subtract literal
		// quote fees, convert to base by dividing by PositionPrice, and
		// subtract any literal base-side fees (covers partial-fill dust).
		sellInQuote := trades.GetQuantityInQuote(event.Trade.History, "BUY")
		buyInQuote := trades.GetQuantityInQuote(event.Trade.History, "SELL")
		quantity = sellInQuote - buyInQuote - feeInQuote
		quantity = quantity / event.Trade.PositionPrice
		quantity = quantity - feeInBase
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

	if event.Params.MarketSellOrder && event.Trade.Inverse {
		response, err = client.MarketBuy(event.Trade.Symbol, quantity)
	} else if event.Params.MarketSellOrder {
		response, err = client.MarketSell(event.Trade.Symbol, quantity)
	} else if event.Trade.Inverse {
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
