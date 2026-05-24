package tests

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/virtualExchange"
)

// scenarioPair is the trading pair used across every scenario test.
const scenarioPair = "BTC/USDC"

// scenarioFilters mirror Binance's real BTC/USDC market filters at the time
// of writing: 5 decimals on quantity (step 0.00001 BTC), 2 decimals on price
// (tick 0.01 USDC), 5 USDC minimum notional.
func scenarioFilters() aggragates.TradeFilters {
	return aggragates.TradeFilters{
		LotSize:     5,
		PriceFilter: 2,
		MinNotional: 5,
	}
}

// scenarioSettings returns a single-row strategy settings slice sized to make
// the DCA math straightforward in tests: percentage 2% per depth, multiplier
// x2 per buy, tolerance 0.25 (used by HasProfit to simulate unrealised PnL),
// depths 6..8, no initial bid (forces budget-driven first buy).
func scenarioSettings() []aggragates.StrategySettings {
	return []aggragates.StrategySettings{
		{
			MinDepths:    6,
			Depths:       8,
			Percentage:   2,
			Multiplier:   2,
			Tolerance:    0.25,
			InitialBid:   0,
			ImpasseDepth: 4,
		},
	}
}

// scenarioBuildTrade returns a baseline BTC/USDC trade with the standard
// scenario filters and settings applied. Callers mutate History,
// PositionPrice, Inverse, PositionType, etc., before passing into actions.
func scenarioBuildTrade(positionType string, positionPrice float64, inverse bool) aggragates.Trades {
	trade := aggragates.Trades{
		Symbol:        scenarioPair,
		PositionPrice: positionPrice,
		PositionType:  positionType,
		Inverse:       inverse,
		ProfitAsset:   "USDC",
	}
	if inverse {
		trade.ProfitAsset = "BTC"
	}
	trade.StrategyPair.TradeFilters = scenarioFilters()
	trade.StrategyPair.StrategySettings = scenarioSettings()
	return trade
}

// scenarioBuildEvent wires the trade into an event with the virtual exchange
// pre-funded with `asset` / `freeAmount` and registers the default action
// map plus an EmptyUpdateTrade stub so SaveError / UpdateTrade do not try to
// reach a real broker.
func scenarioBuildEvent(trade aggragates.Trades, asset string, freeAmount string) events.Events {
	virtualExchange.ResetWallet()
	exchangeInit := GetVirtualExchange(asset, freeAmount)
	defaultActions := actions.GetDefaultActions()
	defaultActions["updateTrade"] = EmptyUpdateTrade
	return events.Events{
		Trade:    trade,
		Exchange: exchangeInit,
		Events:   defaultActions,
	}
}

// scenarioBuyHistory appends a buy/sell row to the trade history. Side is
// the literal "BUY" or "SELL" string the action code uses for matching.
func scenarioAppendHistory(trade *aggragates.Trades, side string, qty, price float64, feeAsset string, fee float64) {
	entry := aggragates.TradesHistory{Type: side, Quantity: qty, Price: price}
	if feeAsset != "" {
		entry.Fees = []aggragates.TradesFees{{Asset: feeAsset, Fee: fee}}
	}
	trade.History = append(trade.History, entry)
}
