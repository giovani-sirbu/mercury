package actions

import (
	"fmt"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"math"
	"sort"
	"strconv"
	"time"
)

func GetLatestIncome(event events.Events, timeWindow time.Duration) (float64, error) {
	// Init futures client
	client, clientError := event.Exchange.FuturesClient()
	if clientError != nil {
		return 0, clientError
	}

	// Fetch income history for the symbol
	income, incomeErr := client.GetIncomeHistory(event.Trade.Symbol)
	if incomeErr != nil {
		return 0, incomeErr
	}

	if len(income) == 0 {
		return 0, fmt.Errorf("couldn't fetch income")
	}

	// Sort income records by timestamp (descending, most recent first)
	sort.Slice(income, func(i, j int) bool {
		return income[i].Time > income[j].Time
	})

	// Group records within the same time window for the latest closed position
	var totalPNL float64
	latestTime := time.UnixMilli(income[0].Time) // Convert int64 to time.Time
	for _, record := range income {
		recordTime := time.UnixMilli(record.Time) // Convert int64 to time.Time
		// Only include records within the time window of the latest record
		if latestTime.Sub(recordTime) <= timeWindow {
			if record.IncomeType == "REALIZED_PNL" {
				pnl, err := strconv.ParseFloat(record.Income, 64)
				if err != nil {
					return 0, fmt.Errorf("failed to parse PNL: %v", err)
				}
				totalPNL += pnl
			}
		} else {
			break // Exit once we go beyond the time window
		}
	}

	if totalPNL == 0 {
		return 0, fmt.Errorf("no realized PNL found in the time window")
	}

	return totalPNL, nil
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
		closeSide := futures.SideTypeSell
		if posAmt < 0 {
			closeSide = futures.SideTypeBuy
		}

		absQty := math.Abs(posAmt)
		qtyStr := fmt.Sprintf("%.*f", event.Trade.StrategyPair.TradeFilters.LotSize, absQty)

		// Step 3: Close the position
		_, closeThePositionErr = client.CreateFuturesOrder(string(closeSide), string(futures.OrderTypeMarket), event.Trade.Symbol, qtyStr, "", true)
		fmt.Println("CloseFuturesTrade, close position", string(closeSide), string(futures.OrderTypeMarket), event.Trade.Symbol, qtyStr)
	}

	pnl, incomeErr := GetLatestIncome(event, 2*time.Second)

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
