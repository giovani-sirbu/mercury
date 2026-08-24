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
	quantity, dust := SellableQuantity(event)

	// if no bought quantity, update event status and close it
	if quantity <= 0 {
		event.Trade.Status = aggragates.Closed
		return event, nil
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

// SellableQuantity returns the LotSize-floored base quantity a full close
// must sell (inverse: buy back) plus the dust left below the lot step.
// Computed from executed history and literal asset fees: only base-paid fees
// reduce the base wallet balance, only quote-paid fees reduce the quote
// wallet balance — BNB/third-asset fees come from a separate wallet and must
// not be subtracted here (profit accounting uses GetFeesBaseQuote / GetFees).
// Shared by Sell, the pending-order close rule and manual full-close sizing
// so "how much can this trade sell" has exactly one definition.
func SellableQuantity(event events.Events) (float64, float64) {
	buyQty, sellQty := GetGrossQuantities(event)
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
	quantity = ToFixed(quantity, int(event.Trade.StrategyPair.TradeFilters.LotSize))

	var dust float64
	if quantityBeforeLotSize > quantity {
		dust = quantityBeforeLotSize - quantity
	}

	return quantity, dust
}
