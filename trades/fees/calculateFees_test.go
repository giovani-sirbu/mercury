package fees

import (
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func buildEventWithFees(symbol string, fees []aggragates.TradesFees, fillPrice float64) events.Events {
	trade := aggragates.Trades{
		Symbol: symbol,
		History: []aggragates.TradesHistory{
			{Type: "BUY", Quantity: 1, Price: fillPrice, Fees: fees},
		},
	}
	return events.Events{Trade: trade}
}

// TestCalculateFees_BaseFeeOnly pins the trade-execution semantic for
// fees paid in the base asset (the Binance default for BUY orders without BNB
// discount): the fee reduces the base wallet, so feeInBase is populated and
// feeInQuote stays zero. No cross-conversion.
func TestCalculateFees_BaseFeeOnly(t *testing.T) {
	event := buildEventWithFees("BTC/USDC", []aggragates.TradesFees{
		{Asset: "BTC", Fee: 0.001},
	}, 50000)

	feeInBase, feeInQuote := CalculateFees(event)
	if math.Abs(feeInBase-0.001) > 1e-12 {
		t.Errorf("feeInBase = %v, want 0.001", feeInBase)
	}
	if feeInQuote != 0 {
		t.Errorf("feeInQuote = %v, want 0 (no cross-conversion)", feeInQuote)
	}
}

// TestCalculateFees_QuoteFeeOnly pins the trade-execution semantic for
// fees paid in the quote asset (Binance default for SELL orders without BNB
// discount): the fee reduces the quote wallet, so feeInQuote is populated and
// feeInBase stays zero. Critically: feeInBase MUST stay zero even though a
// cross-converted value would be non-zero — sell.go subtracts feeInBase from
// the base wallet balance and quote fees do not touch base.
func TestCalculateFees_QuoteFeeOnly(t *testing.T) {
	event := buildEventWithFees("BTC/USDC", []aggragates.TradesFees{
		{Asset: "USDC", Fee: 26},
	}, 52000)

	feeInBase, feeInQuote := CalculateFees(event)
	if feeInBase != 0 {
		t.Errorf("feeInBase = %v, want 0 (USDC fee must not bleed into base)", feeInBase)
	}
	if math.Abs(feeInQuote-26) > 1e-9 {
		t.Errorf("feeInQuote = %v, want 26", feeInQuote)
	}
}

// TestCalculateFees_BNBFeeIsExcluded is the regression pin for the bug
// fix that motivated this helper. Fees paid in BNB (the Binance discount
// option) come from a separate BNB wallet and do not reduce base or quote
// holdings. They MUST be zero in both buckets.
//
// GetFeesBaseQuote (the cross-converting cousin) deliberately includes BNB in
// both buckets — that semantic is correct for profit accounting. The mistake
// of using GetFeesBaseQuote here would over-deduct from the wallet balance,
// causing sell.go to under-sell and leave artificial dust on every BNB-using
// trade.
func TestCalculateFees_BNBFeeIsExcluded(t *testing.T) {
	event := buildEventWithFees("BTC/USDC", []aggragates.TradesFees{
		{Asset: "BNB", Fee: 0.005},
	}, 50000)

	feeInBase, feeInQuote := CalculateFees(event)
	if feeInBase != 0 {
		t.Errorf("feeInBase = %v, want 0 (BNB fees do not touch base wallet)", feeInBase)
	}
	if feeInQuote != 0 {
		t.Errorf("feeInQuote = %v, want 0 (BNB fees do not touch quote wallet)", feeInQuote)
	}
}

// TestCalculateFees_MixedKeepsBNBOut pins the realistic case where the
// trade history holds a mix: a BUY with a base-asset fee, a SELL with a quote-
// asset fee, and either side carrying a BNB row. Only the base and quote rows
// contribute; BNB is silently excluded by design.
func TestCalculateFees_MixedKeepsBNBOut(t *testing.T) {
	trade := aggragates.Trades{
		Symbol: "BTC/USDC",
		History: []aggragates.TradesHistory{
			{Type: "BUY", Quantity: 1, Price: 50000, Fees: []aggragates.TradesFees{
				{Asset: "BTC", Fee: 0.001},
				{Asset: "BNB", Fee: 0.002}, // must be ignored
			}},
			{Type: "SELL", Quantity: 0.5, Price: 52000, Fees: []aggragates.TradesFees{
				{Asset: "USDC", Fee: 26},
				{Asset: "BNB", Fee: 0.001}, // must be ignored
			}},
		},
	}

	feeInBase, feeInQuote := CalculateFees(events.Events{Trade: trade})

	if math.Abs(feeInBase-0.001) > 1e-12 {
		t.Errorf("feeInBase = %v, want 0.001 (only the literal BTC row)", feeInBase)
	}
	if math.Abs(feeInQuote-26) > 1e-9 {
		t.Errorf("feeInQuote = %v, want 26 (only the literal USDC row)", feeInQuote)
	}
}

// TestCalculateFees_EmptyHistory covers the no-fills edge case.
func TestCalculateFees_EmptyHistory(t *testing.T) {
	feeInBase, feeInQuote := CalculateFees(events.Events{Trade: aggragates.Trades{Symbol: "BTC/USDC"}})
	if feeInBase != 0 || feeInQuote != 0 {
		t.Errorf("empty history = (%v, %v), want (0, 0)", feeInBase, feeInQuote)
	}
}
