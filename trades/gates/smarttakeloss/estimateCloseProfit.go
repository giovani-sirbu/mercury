package smarttakeloss

import (
	"math"
	"strings"

	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/profit"
)

// EstimateCloseProfit is the close P&L used by Evaluate, net of the fees a
// real close would settle. Engines must use this for the STL underwater-sell
// gate so long and inverse stay on the same estimate; do not substitute
// HasProfit.
func EstimateCloseProfit(trade aggragates.Trades, price float64) (pnl, invested float64) {
	return estimateCloseProfit(trade, price)
}

// estimateCloseProfit simulates selling the trade's remaining base at price
// and returns the quote P&L next to the invested quote. The remaining base
// is what Sell would actually submit (gross minus the base-asset commissions
// the exchange already took) and the closing leg's fee is charged at the
// rate observed on the entries, so a chain a hair above gross break-even is
// not "banked at a profit" only to settle negative. Without fee rows on the
// history (unit fixtures, exchanges that bill elsewhere) the estimate is the
// fee-free one. Inverse trades take the mirrored branch.
func estimateCloseProfit(trade aggragates.Trades, price float64) (pnl, invested float64) {
	if trade.Inverse {
		return estimateInverseCloseProfit(trade, price)
	}

	baseSymbol, quoteSymbol := helpers.SplitSymbol(trade.Symbol)
	var boughtBase, soldBase, feeInBase, feeInQuote float64
	for _, history := range trade.History {
		if strings.ToLower(history.Type) == "buy" {
			boughtBase += history.Quantity
			invested += history.Quantity * history.Price
		} else {
			soldBase += history.Quantity
		}
		for _, fee := range history.Fees {
			switch {
			case fee.Fee <= 0:
			case fee.Asset == baseSymbol:
				feeInBase += fee.Fee
			case fee.Asset == quoteSymbol:
				feeInQuote += fee.Fee
			}
		}
	}
	remaining := boughtBase - soldBase - feeInBase
	if remaining <= 0 {
		return 0, invested
	}

	simulated := trade
	simulated.History = append(append([]aggragates.TradesHistory(nil), trade.History...), aggragates.TradesHistory{
		Type:     "SELL",
		Quantity: remaining,
		Price:    price,
	})
	closingFee := remaining * price * observedEntryFeeRate(feeInBase, boughtBase)
	return profit.GetProfit(simulated) - feeInQuote - closingFee, invested
}

// observedEntryFeeRate is the commission rate the entries actually paid
// (fee taken in the received asset over the received quantity); zero when
// the history carries no fee rows.
func observedEntryFeeRate(feePaid, received float64) float64 {
	if feePaid <= 0 || received <= 0 {
		return 0
	}
	return feePaid / received
}

// requiredRecoveryPct is the favorable price move, as a percent of the
// current print, needed to close fee-free at break even — the numerator of
// the projected-block estimate. Uses the same history aggregation and dust
// handling as GetProfit so the break-even it solves is the profit the
// engine would actually report. Returns 0 when already at or past break
// even, or when the ledger has nothing left to close.
func requiredRecoveryPct(trade aggragates.Trades, price float64) float64 {
	var buyQty, sellQty, buyQuote, sellQuote float64
	for _, history := range trade.History {
		if strings.ToLower(history.Type) == "buy" {
			buyQty += history.Quantity
			buyQuote += history.Quantity * history.Price
		} else {
			sellQty += history.Quantity
			sellQuote += history.Quantity * history.Price
		}
	}

	if trade.Inverse {
		// Entries sold base for quote; the close buys base back, so break
		// even is the price at which the held quote repurchases the base
		// deficit (dust is base-denominated for inverse trades).
		baseDeficit := sellQty - buyQty - trade.Dust
		remainingQuote := sellQuote - buyQuote
		if baseDeficit <= 0 || remainingQuote <= 0 {
			return 0
		}
		breakEven := remainingQuote / baseDeficit
		return math.Max(0, (price-breakEven)/price*100)
	}

	remainingBase := buyQty - sellQty
	if remainingBase <= 0 {
		return 0
	}
	breakEven := (buyQuote - sellQuote - trade.Dust*trade.PositionPrice) / remainingBase
	return math.Max(0, (breakEven-price)/price*100)
}

// estimateInverseCloseProfit mirrors estimateCloseProfit for inverse trades:
// entries are SELLs of base for quote, so closing buys the base back with
// the quote still held. Profit follows GetProfit's inverse contract
// (base-denominated); invested is the base committed by the entries — a
// different unit than the long branch, fine for the positivity guard and
// internal ratios this feeds.
func estimateInverseCloseProfit(trade aggragates.Trades, price float64) (pnl, invested float64) {
	baseSymbol, quoteSymbol := helpers.SplitSymbol(trade.Symbol)
	var soldBase, quoteIn, quoteOut, feeInBase, feeInQuote float64
	for _, history := range trade.History {
		if strings.ToLower(history.Type) == "buy" {
			quoteOut += history.Quantity * history.Price
		} else {
			soldBase += history.Quantity
			quoteIn += history.Quantity * history.Price
		}
		for _, fee := range history.Fees {
			switch {
			case fee.Fee <= 0:
			case fee.Asset == baseSymbol:
				feeInBase += fee.Fee
			case fee.Asset == quoteSymbol:
				feeInQuote += fee.Fee
			}
		}
	}
	invested = soldBase
	// The quote the close can spend is net of the quote commissions the
	// entries already paid (mirrors Sell's inverse branch).
	remainingQuote := quoteIn - quoteOut - feeInQuote
	if remainingQuote <= 0 || price <= 0 {
		return 0, invested
	}

	simulated := trade
	simulated.History = append(append([]aggragates.TradesHistory(nil), trade.History...), aggragates.TradesHistory{
		Type:     "BUY",
		Quantity: remainingQuote / price,
		Price:    price,
	})
	closingFee := (remainingQuote / price) * observedEntryFeeRate(feeInQuote, quoteIn)
	return profit.GetProfit(simulated) - feeInBase - closingFee, invested
}
