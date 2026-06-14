package tests

import (
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The tests in this file pin the regression fix end-to-end through Sell and
// HasFunds. They demonstrate the difference between profit-accounting fees
// (which include BNB cost via cross-conversion) and trade-execution fees
// (which must NOT deduct BNB or quote-asset fees from the base wallet).
//
// Pair throughout: BTC/USDC. PositionPrice 65000 unless noted.

// TestSell_BTCUSDC_MixedAllThreeAssetsFees pins the most realistic Binance
// pattern: three BUYs with the three fee assets (BNB discount on the first,
// quote on the second when BNB ran out, base on the third dust-fee row).
// Only the literal BTC fee should reduce the base quantity to sell.
//
// History:
//
//	BUY 0.001 BTC @ 65000 USDC, fee 0.0002 BNB
//	BUY 0.002 BTC @ 64000 USDC, fee 0.128 USDC
//	BUY 0.004 BTC @ 63000 USDC, fee 0.000004 BTC
//
// Expected sell quantity = 0.007 - 0 - 0.000004 = 0.006996 BTC.
// ToFixed(0.006996, 5) = 0.00699, dust = 0.000006 BTC (truncation residue
// only — no over-deduction from BNB/USDC cross-conversion).
func TestSell_BTCUSDC_MixedAllThreeAssetsFees(t *testing.T) {
	trade := scenarioBuildTrade("sell", 65000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 65000, "BNB", 0.0002)
	scenarioAppendHistory(&trade, "BUY", 0.002, 64000, "USDC", 0.128)
	scenarioAppendHistory(&trade, "BUY", 0.004, 63000, "BTC", 0.000004)

	event := scenarioBuildEvent(trade, "BTC", "0.007")
	event.WsPrices = map[string]float64{"BNB/USDC": 600}

	got, err := actions.Sell(event)
	if err != nil {
		t.Fatalf("Sell returned error: %v", err)
	}

	// Only the BTC fee deducts. The buyTotal = 0.007 BTC, sellTotal = 0,
	// feeInBase (literal) = 0.000004 BTC.
	// quantityBeforeLotSize = 0.007 - 0 - 0.000004 = 0.006996 BTC
	// ToFixed(0.006996, 5) = 0.00699 BTC, dust = 0.000006 BTC.
	const wantQty = 0.00699
	const wantDust = 0.000006
	if math.Abs(got.Params.Quantity-wantQty) > 1e-7 {
		t.Errorf("Sell quantity = %v, want %v", got.Params.Quantity, wantQty)
	}
	if math.Abs(got.Trade.Dust-wantDust) > 1e-9 {
		t.Errorf("Trade.Dust = %v, want %v (truncation residue only)", got.Trade.Dust, wantDust)
	}

	t.Logf("Sell quantity = %.8f BTC (only BTC fee deducted from base wallet)", got.Params.Quantity)
	t.Logf("Trade.Dust    = %.8f BTC (truncation residue, no artificial over-deduction)", got.Trade.Dust)
}

// TestSell_Inverse_BNBFeesPreservedFullQuantity is the inverse-mode pin for
// the BNB fix. Three inverse SELLs (= open shorts) with BNB fees. The
// inverse-branch math in sell.go must NOT subtract a cross-converted BNB
// equivalent — CalculateFees returns 0 in both buckets for BNB so the math
// reduces to "no fee adjustment", matching the pre-migration behavior.
func TestSell_Inverse_BNBFeesPreservedFullQuantity(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 100000, true)
	scenarioAppendHistory(&trade, "SELL", 0.001, 100000, "BNB", 0.0002)
	scenarioAppendHistory(&trade, "SELL", 0.002, 102000, "BNB", 0.0004)

	event := scenarioBuildEvent(trade, "USDC", "1000")
	event.WsPrices = map[string]float64{"BNB/USDC": 600}

	got, err := actions.Sell(event)
	if err != nil {
		t.Fatalf("Sell returned error: %v", err)
	}

	// sellInQuote = 0.001*100000 + 0.002*102000 = 100 + 204 = 304 USDC
	// buyInQuote  = 0 (no inverse "buys" / closes yet)
	// feeInBase (BNB) = 0, feeInQuote (BNB) = 0 — literal-only.
	// quantity = (304 - 0 - 0) / 100000 = 0.00304 BTC
	// ToFixed(0.00304, 5) = 0.00304, dust = 0.
	const wantQty = 0.00304
	if math.Abs(got.Params.Quantity-wantQty) > 1e-7 {
		t.Errorf("Inverse Sell quantity = %v, want %v (BNB must not affect inverse math)", got.Params.Quantity, wantQty)
	}

	t.Logf("Inverse Sell quantity = %.8f BTC, dust = %.8f BTC", got.Params.Quantity, got.Trade.Dust)
}

// TestSell_Inverse_LiteralUSDCFeeReducesNetQuote pins the legitimate use of
// feeInQuote in the inverse branch. A USDC fee on an inverse SELL is a
// literal quote-side cost — the inverse branch correctly subtracts it from
// the sellInQuote-buyInQuote diff before converting to base.
func TestSell_Inverse_LiteralUSDCFeeReducesNetQuote(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 100000, true)
	scenarioAppendHistory(&trade, "SELL", 0.001, 100000, "USDC", 0.1)
	scenarioAppendHistory(&trade, "SELL", 0.002, 102000, "USDC", 0.204)

	event := scenarioBuildEvent(trade, "USDC", "1000")

	got, err := actions.Sell(event)
	if err != nil {
		t.Fatalf("Sell returned error: %v", err)
	}

	// sellInQuote = 304 USDC
	// buyInQuote  = 0
	// feeInQuote (literal USDC) = 0.304 USDC
	// quantity = (304 - 0 - 0.304) / 100000 = 303.696 / 100000 = 0.00303696 BTC
	// ToFixed(0.00303696, 5) = 0.00303, dust = 0.00000696 BTC.
	const wantQty = 0.00303
	if math.Abs(got.Params.Quantity-wantQty) > 1e-7 {
		t.Errorf("Inverse Sell quantity (USDC fee) = %v, want %v", got.Params.Quantity, wantQty)
	}

	t.Logf("Inverse Sell quantity = %.8f BTC (USDC fee correctly applied), dust = %.8f BTC", got.Params.Quantity, got.Trade.Dust)
}

// TestHasFunds_BTCUSDC_USDCFeesDoNotTriggerFalseInsufficient is the HasFunds
// counterpart to the Sell tests above. The wallet has just enough BTC to
// cover the position (no fee buffer). Pre-fix, GetFundsQuantities would
// have computed neededQuantity = buyQty - sellQty - cross-converted-USDC-fee
// (a positive value slightly less than buyQty), making the check think the
// wallet is short. Post-fix, the literal USDC fee is in feeInQuote (not
// feeInBase), so neededQuantity = buyQty (cleanly).
func TestHasFunds_BTCUSDC_USDCFeesDoNotTriggerFalseInsufficient(t *testing.T) {
	trade := scenarioBuildTrade("sell", 65000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 65000, "USDC", 0.065)
	scenarioAppendHistory(&trade, "BUY", 0.002, 64000, "USDC", 0.128)

	// Wallet has exactly the gross position — no headroom for over-deduction.
	event := scenarioBuildEvent(trade, "BTC", "0.003")

	_, err := actions.HasFunds(event)
	if err != nil {
		t.Errorf("HasFunds rejected the trade despite exact BTC balance: %v (USDC fees must not over-state needed base)", err)
	}
}

// TestHasFunds_BTCUSDC_BNBFeesDoNotTriggerFalseInsufficient mirrors the
// above for BNB-discount users.
func TestHasFunds_BTCUSDC_BNBFeesDoNotTriggerFalseInsufficient(t *testing.T) {
	trade := scenarioBuildTrade("sell", 65000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 65000, "BNB", 0.0002)
	scenarioAppendHistory(&trade, "BUY", 0.002, 64000, "BNB", 0.0004)

	event := scenarioBuildEvent(trade, "BTC", "0.003")
	event.WsPrices = map[string]float64{"BNB/USDC": 600}

	_, err := actions.HasFunds(event)
	if err != nil {
		t.Errorf("HasFunds rejected the trade despite exact BTC balance: %v (BNB fees must not over-state needed base)", err)
	}
}

// TestProfitMath_BNBFeesAreCounted is the symmetric pin for the OTHER side
// of the fix: profit accounting (via GetFees / GetFeesBaseQuote) must STILL
// include BNB cost. CalculateFees would return 0 for a BNB-only history;
// GetFees uses GetFeesBaseQuote which cross-converts BNB to the profit
// denomination so the trade's real cost is reflected in P&L.
func TestProfitMath_BNBFeesAreCounted(t *testing.T) {
	trade := aggragates.Trades{
		Symbol:        "BTC/USDC",
		PositionPrice: 65000,
		ProfitAsset:   "USDC",
		History: []aggragates.TradesHistory{
			{Type: "BUY", Quantity: 0.001, Price: 65000, Fees: []aggragates.TradesFees{
				{Asset: "BNB", Fee: 0.0002},
			}},
		},
	}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{PriceFilter: 2}
	event := events.Events{Trade: trade, WsPrices: map[string]float64{"BNB/USDC": 600}}

	// Trade-execution fee (CalculateFees) must report 0 for BNB.
	feeInBase, feeInQuote := actions.CalculateFees(event)
	if feeInBase != 0 || feeInQuote != 0 {
		t.Errorf("CalculateFees BNB = (%v, %v), want (0, 0)", feeInBase, feeInQuote)
	}

	// Profit-accounting fee (GetFees) must NOT report 0 — BNB is a real cost.
	totalFees := actions.GetFees(event)
	if totalFees <= 0 {
		t.Errorf("GetFees BNB = %v, want >0 (BNB cost must be counted in P&L)", totalFees)
	}
}
