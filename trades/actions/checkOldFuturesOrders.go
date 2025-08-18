package actions

import (
	"fmt"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/giovani-sirbu/mercury/events"
	"math"
	"strconv"
)

func CheckOldFuturesOrders(event events.Events) (events.Events, error) {
	// Init futures client
	client, clientError := event.Exchange.FuturesClient()
	if clientError != nil {
		return events.Events{}, clientError
	}
	// Fetch all order
	orders, listErr := client.ListOrders(event.Trade.Symbol)
	if listErr != nil {
		return events.Events{}, listErr
	}

	// Fetch the active position
	positions, positionsErr := client.GetSymbolPosition(event.Trade.Symbol)
	if positionsErr != nil {
		return events.Events{}, positionsErr
	}

	// Go through all orders by symbol close all old remaining trades, should be a rare case
	var closeOrdersErr error
	for _, order := range orders {
		_, closeOrdersErr = client.CancelOrders(event.Trade.Symbol, order.OrderID)
	}

	if closeOrdersErr != nil {
		return events.Events{}, closeOrdersErr
	}

	var closeThePositionErr error
	for _, p := range positions {
		posAmt, err := strconv.ParseFloat(p.PositionAmt, 64)
		if err != nil || posAmt == 0 {
			continue
		}

		// Determine close side
		closeSide := futures.SideTypeSell
		if posAmt < 0 {
			closeSide = futures.SideTypeBuy
		}

		absQty := math.Abs(posAmt)
		qtyStr := fmt.Sprintf("%.*f", event.Trade.StrategyPair.TradeFilters.LotSize, absQty)

		// Step 3: Close the position
		_, closeThePositionErr = client.CreateFuturesOrder(string(closeSide), string(futures.OrderTypeMarket), event.Trade.Symbol, qtyStr, "", true)
	}
	if closeThePositionErr != nil {
		return events.Events{}, closeThePositionErr
	}
	return event, nil
}
