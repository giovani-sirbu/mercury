package actions

import (
	"fmt"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"math"
	"strconv"
)

func GetLatestIncome(event events.Events) (float64, error) {
	// Init futures client
	client, clientError := event.Exchange.FuturesClient()
	if clientError != nil {
		return 0, clientError
	}

	// Fetch realized PnL from income history
	income, incomeErr := client.GetIncomeHistory(event.Trade.Symbol)

	if incomeErr != nil {
		return 0, incomeErr
	}

	// Get the most recent PnL record
	if len(income) > 0 {
		latest := income[len(income)-1]
		pnl, _ := strconv.ParseFloat(latest.Income, 64)

		return pnl, nil
	}

	return 0, fmt.Errorf("couldn't fetch income")
}

func CloseFuturesTrade(event events.Events) (events.Events, error) {
	// Init futures client
	client, clientError := event.Exchange.FuturesClient()
	if clientError != nil {
		return events.Events{}, clientError
	}

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
		if event.Trade.PendingOrder == order.OrderID || event.Trade.History[0].OrderId == order.OrderID {
			_, closeOrdersErr = client.CancelOrders(event.Trade.Symbol, order.OrderID)
		}
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

	pnl, incomeErr := GetLatestIncome(event)

	if incomeErr != nil {
		return events.Events{}, incomeErr
	}

	event.Trade.Profit = pnl
	event.Trade.USDProfit = pnl

	if closeThePositionErr != nil {
		return events.Events{}, closeThePositionErr
	}

	event.Trade.Status = aggragates.Closed

	return event, nil
}
