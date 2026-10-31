package actions_test

import (
	"github.com/giovani-sirbu/mercury/trades/fees"
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/actions"
)

// The three tests below exercise both fee primitives on the same BTC/USDC
// trade shape with three fee compositions so the difference between
// trade-execution math and profit-accounting math is observable.
//
//   CalculateFees: used by Sell + HasFunds. Only the literal base/quote
//   fees count — third-asset fees (BNB) are excluded because they come from a
//   separate BNB wallet and do not reduce base or quote holdings.
//
//   GetFeesBaseQuote: used by GetFees / profit accounting. ALL fees are
//   cross-converted into base and quote denominations so BNB cost is
//   reflected in P&L.
//
// Common history (used by all three):
//   BUY  0.001 BTC @ 65000 USDC
//   BUY  0.002 BTC @ 64000 USDC
//   SELL 0.001 BTC @ 66000 USDC
// PositionPrice = 65000 USDC.

const (
	btcusdcBuy1Qty   = 0.001
	btcusdcBuy1Price = 65000.0
	btcusdcBuy2Qty   = 0.002
	btcusdcBuy2Price = 64000.0
	btcusdcSellQty   = 0.001
	btcusdcSellPrice = 66000.0
	btcusdcPosPrice  = 65000.0
)

// Scenario A: fees paid only in USDC (quote asset).
// Both functions agree on feeInQuote. GetFeesBaseQuote cross-converts USDC
// into base for profit accounting; CalculateFees keeps feeInBase at
// zero because USDC fees do not touch the BTC wallet (so Sell must not
// over-deduct).
func TestFees_BTCUSDC_OnlyUSDC(t *testing.T) {
	trade := scenarioBuildTrade("closed", btcusdcPosPrice, false)
	scenarioAppendHistory(&trade, "BUY", btcusdcBuy1Qty, btcusdcBuy1Price, "USDC", 0.065)
	scenarioAppendHistory(&trade, "BUY", btcusdcBuy2Qty, btcusdcBuy2Price, "USDC", 0.128)
	scenarioAppendHistory(&trade, "SELL", btcusdcSellQty, btcusdcSellPrice, "USDC", 0.066)

	event := events.Events{Trade: trade}
	totalBase, totalQuote := fees.GetFeesBaseQuote(event)
	walletBase, walletQuote := fees.CalculateFees(event)

	t.Logf("=== BTC/USDC, fees only in USDC ===")
	t.Logf("GetFeesBaseQuote   (profit math):    feeInBase = %.12f BTC | feeInQuote = %.6f USDC", totalBase, totalQuote)
	t.Logf("CalculateFees (Sell/HasFunds): feeInBase = %.12f BTC | feeInQuote = %.6f USDC", walletBase, walletQuote)

	// Sanity: Sell must not over-deduct from BTC wallet.
	if walletBase != 0 {
		t.Errorf("CalculateFees feeInBase = %v, want 0 — USDC fees must not touch BTC wallet", walletBase)
	}
}

// Scenario B: fees paid only in BNB (third asset).
// GetFeesBaseQuote converts BNB into BOTH denominations (profit math sees
// the real BNB cost). CalculateFees returns (0, 0) because BNB fees
// do not touch the BTC or USDC wallets — Sell must not subtract them.
func TestFees_BTCUSDC_OnlyBNB(t *testing.T) {
	trade := scenarioBuildTrade("closed", btcusdcPosPrice, false)
	scenarioAppendHistory(&trade, "BUY", btcusdcBuy1Qty, btcusdcBuy1Price, "BNB", 0.0002)
	scenarioAppendHistory(&trade, "BUY", btcusdcBuy2Qty, btcusdcBuy2Price, "BNB", 0.0004)
	scenarioAppendHistory(&trade, "SELL", btcusdcSellQty, btcusdcSellPrice, "BNB", 0.00021)

	event := events.Events{
		Trade:    trade,
		WsPrices: map[string]float64{"BNB/USDC": 600}, // 600 USDC per BNB
	}
	totalBase, totalQuote := fees.GetFeesBaseQuote(event)
	walletBase, walletQuote := fees.CalculateFees(event)

	t.Logf("=== BTC/USDC, fees only in BNB (priced at 600 USDC) ===")
	t.Logf("GetFeesBaseQuote   (profit math):    feeInBase = %.12f BTC | feeInQuote = %.6f USDC", totalBase, totalQuote)
	t.Logf("CalculateFees (Sell/HasFunds): feeInBase = %.12f BTC | feeInQuote = %.6f USDC", walletBase, walletQuote)

	if walletBase != 0 || walletQuote != 0 {
		t.Errorf("CalculateFees = (%v, %v), want (0, 0) — BNB fees never touch base/quote wallets", walletBase, walletQuote)
	}
}

// Scenario C: mixed BNB + USDC fees across the same history.
// GetFeesBaseQuote rolls BNB into both denominations on top of the USDC fee.
// CalculateFees keeps only the literal USDC row; BNB stays out.
func TestFees_BTCUSDC_MixedBNBAndUSDC(t *testing.T) {
	trade := scenarioBuildTrade("closed", btcusdcPosPrice, false)
	scenarioAppendHistory(&trade, "BUY", btcusdcBuy1Qty, btcusdcBuy1Price, "BNB", 0.0002)
	scenarioAppendHistory(&trade, "BUY", btcusdcBuy2Qty, btcusdcBuy2Price, "USDC", 0.128)
	scenarioAppendHistory(&trade, "SELL", btcusdcSellQty, btcusdcSellPrice, "BNB", 0.00021)

	event := events.Events{
		Trade:    trade,
		WsPrices: map[string]float64{"BNB/USDC": 600},
	}
	totalBase, totalQuote := fees.GetFeesBaseQuote(event)
	walletBase, walletQuote := fees.CalculateFees(event)

	t.Logf("=== BTC/USDC, mixed BNB + USDC fees ===")
	t.Logf("GetFeesBaseQuote   (profit math):    feeInBase = %.12f BTC | feeInQuote = %.6f USDC", totalBase, totalQuote)
	t.Logf("CalculateFees (Sell/HasFunds): feeInBase = %.12f BTC | feeInQuote = %.6f USDC", walletBase, walletQuote)

	if walletBase != 0 {
		t.Errorf("CalculateFees feeInBase = %v, want 0 — neither BNB nor USDC fees should touch BTC wallet", walletBase)
	}
	if math.Abs(walletQuote-0.128) > 1e-9 {
		t.Errorf("CalculateFees feeInQuote = %v, want 0.128 (USDC row only, BNB excluded)", walletQuote)
	}
}

// TestSell_BTCUSDC_USDCFeesDoNotReduceBaseQuantity pins the dust regression
// fix end-to-end. Two BUYs with USDC-paid fees and no sells: the BTC wallet
// holds the full 0.003 BTC because Binance took the fees from the USDC wallet
// instead. Sell must target the full 0.003 BTC — not deduct the USDC fees
// converted into a base equivalent.
//
// Pre-fix (using GetFeesBaseQuote in sell.go), this would have computed:
//
//	feeInBase = (0.065 + 0.128) / fill_price ≈ 3e-6 BTC
//	quantity  = 0.003 - 0 - 3e-6 = 0.002997 BTC
//	ToFixed(0.002997, 5) = 0.00299 BTC, leaving 0.000007 BTC as artificial dust.
//
// Post-fix (CalculateFees), feeInBase stays at zero and the full
// 0.003 BTC is sold cleanly with no dust.
func TestSell_BTCUSDC_USDCFeesDoNotReduceBaseQuantity(t *testing.T) {
	trade := scenarioBuildTrade("sell", btcusdcPosPrice, false)
	scenarioAppendHistory(&trade, "BUY", btcusdcBuy1Qty, btcusdcBuy1Price, "USDC", 0.065)
	scenarioAppendHistory(&trade, "BUY", btcusdcBuy2Qty, btcusdcBuy2Price, "USDC", 0.128)

	event := scenarioBuildEvent(trade, "BTC", "0.003")
	got, err := actions.Sell(event)
	if err != nil {
		t.Fatalf("Sell returned error: %v", err)
	}

	const wantQty = 0.003
	if math.Abs(got.Params.Quantity-wantQty) > 1e-9 {
		t.Errorf("Sell quantity = %v, want %v (USDC fees must not over-deduct from BTC wallet)", got.Params.Quantity, wantQty)
	}
	if got.Trade.Dust != 0 {
		t.Errorf("Trade.Dust = %v, want 0 — no truncation needed and no artificial over-deduction", got.Trade.Dust)
	}

	t.Logf("Sell quantity = %.8f BTC (full position, USDC fees correctly ignored from base math)", got.Params.Quantity)
	t.Logf("Trade.Dust    = %.8f BTC", got.Trade.Dust)
}

// TestSell_BTCUSDC_BNBFeesDoNotReduceBaseQuantity is the same fix pinned for
// BNB-discount users. Three BUYs paying BNB-only fees: BTC wallet holds the
// full position because BNB came from a separate wallet. Sell must target the
// full base quantity.
func TestSell_BTCUSDC_BNBFeesDoNotReduceBaseQuantity(t *testing.T) {
	trade := scenarioBuildTrade("sell", btcusdcPosPrice, false)
	scenarioAppendHistory(&trade, "BUY", btcusdcBuy1Qty, btcusdcBuy1Price, "BNB", 0.0002)
	scenarioAppendHistory(&trade, "BUY", btcusdcBuy2Qty, btcusdcBuy2Price, "BNB", 0.0004)

	event := scenarioBuildEvent(trade, "BTC", "0.003")
	event.WsPrices = map[string]float64{"BNB/USDC": 600}

	got, err := actions.Sell(event)
	if err != nil {
		t.Fatalf("Sell returned error: %v", err)
	}

	const wantQty = 0.003
	if math.Abs(got.Params.Quantity-wantQty) > 1e-9 {
		t.Errorf("Sell quantity = %v, want %v (BNB fees must not over-deduct from BTC wallet)", got.Params.Quantity, wantQty)
	}
	if got.Trade.Dust != 0 {
		t.Errorf("Trade.Dust = %v, want 0 — no artificial dust from BNB cross-conversion", got.Trade.Dust)
	}

	t.Logf("Sell quantity = %.8f BTC (full position, BNB fees correctly ignored from base math)", got.Params.Quantity)
	t.Logf("Trade.Dust    = %.8f BTC", got.Trade.Dust)
}
