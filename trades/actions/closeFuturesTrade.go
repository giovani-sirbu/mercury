package actions

import (
	"fmt"
	binanceFutures "github.com/adshao/go-binance/v2/futures"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/futures"
	"math"
	"strconv"
	"time"
)

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
			fmt.Println("CloseFuturesTrade, close orders", event.Trade.Symbol, order.OrderID)
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
		closeSide := binanceFutures.SideTypeSell
		if posAmt < 0 {
			closeSide = binanceFutures.SideTypeBuy
		}

		absQty := math.Abs(posAmt)
		qtyStr := fmt.Sprintf("%.*f", event.Trade.StrategyPair.TradeFilters.LotSize, absQty)

		// Step 3: Close the position
		_, closeThePositionErr = client.CreateFuturesOrder(string(closeSide), string(binanceFutures.OrderTypeMarket), event.Trade.Symbol, qtyStr, "", true)
		fmt.Println("CloseFuturesTrade, close position", string(closeSide), string(binanceFutures.OrderTypeMarket), event.Trade.Symbol, qtyStr)
	}

	pnl, incomeErr := futures.GetLatestIncome(event, 2*time.Second)

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
