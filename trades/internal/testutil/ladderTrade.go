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

// LadderTrade is a SOL/USDT trade under SmartTakeLoss on the w3s row
// {Percentage 2.25, Tolerance 0.25, TrailingTakeProfit 1, Multiplier 2,
// MinDepths 6, Depths 8} — the strategy of backtest 121 — with PriceFilter
// 2, one distinct entry order per fill (OrderId i+1, one unit, CreatedAt =
// the fill's stamp), PositionType "buy" and PositionPrice at the last fill.
// An inverse ladder holds the same fills as SELLs.
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

// W3sFills is trade 45211's ladder (SOL/USDT, December 2021): the eight
// fills it took before the funds ran out, one per clock given, stamped with
// At. W3sFills("13:00:00", "13:15:00") is the two-deep ladder; more than
// eight clocks are ignored.
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
