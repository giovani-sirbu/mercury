package testutil

import (
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// LadderFill is one executed entry order of a LadderTrade: its price and the
// stamp of its history row.
type LadderFill struct {
	Price float64
	At    time.Time
}

// LadderTrade is a trade under SmartTakeLoss on one ladder row, so that row's
// settings govern every depth: one distinct entry order per fill (OrderId
// i+1, one unit, CreatedAt = the fill's stamp), PositionType "buy" and
// PositionPrice at the last fill. An inverse ladder holds the same fills as
// SELLs.
func LadderTrade(inverse bool, fills ...LadderFill) aggragates.Trades {
	trade := aggragates.Trades{
		ID:           45211,
		Symbol:       "SOL/USDT",
		PositionType: "buy",
		Inverse:      inverse,
		Status:       aggragates.Active,
		Strategy: aggragates.Strategies{
			Params: aggragates.StrategyParams{SmartTakeLoss: true},
		},
		StrategyPair: aggragates.StrategiesPairs{
			TradeFilters: aggragates.TradeFilters{PriceFilter: 2},
			StrategySettings: []aggragates.StrategySettings{
				{Percentage: 2.25, Tolerance: 0.25, TrailingTakeProfit: 1, Multiplier: 2, MinDepths: 6, Depths: 8},
			},
		},
	}
	side := "BUY"
	if inverse {
		side = "SELL"
	}
	for index, fill := range fills {
		trade.History = append(trade.History, aggragates.TradesHistory{
			Type: side, Quantity: 1, Price: fill.Price, OrderId: int64(index + 1), CreatedAt: fill.At,
		})
		trade.PositionPrice = fill.Price
	}
	return trade
}

// W3sFills is the fixture ladder's entry prices — a ladder filled to its
// last depth — one per clock given, stamped with At. Passing two clocks
// yields the two-deep prefix of it; clocks past the last price are ignored.
func W3sFills(clocks ...string) []LadderFill {
	prices := []float64{201.46, 192.81, 188.58, 184.45, 179.78, 175.83, 171.97, 168.2}
	fills := make([]LadderFill, 0, len(clocks))
	for index, clock := range clocks {
		if index >= len(prices) {
			break
		}
		fills = append(fills, LadderFill{Price: prices[index], At: At(clock)})
	}
	return fills
}
