package fees

import (
	"testing"

	"github.com/adshao/go-binance/v2/common"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/exchange"
	exchangeAggregates "github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// buildTradeWithBNBFee returns a minimal event whose fee asset (BNB) is
// neither base (BTC) nor quote (USDT), forcing getSymbolPrice to resolve
// BNB/USDC from the lookup chain.
func buildTradeWithBNBFee(bnbFee float64) events.Events {
	trade := aggragates.Trades{
		Symbol:        "BTC/USDT",
		PositionPrice: 65000,
		ProfitAsset:   "BTC",
		History: []aggragates.TradesHistory{
			{
				Price: 65000,
				Fees:  []aggragates.TradesFees{{Asset: "BNB", Fee: bnbFee}},
			},
		},
	}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{PriceFilter: 2}
	return events.Events{Trade: trade}
}

// TestGetFeesUsesWsPricesInProcessSnapshot pins the hot-path behaviour
// introduced in the Faza 3 cache-opt: when the event carries a populated
// WsPrices snapshot, getSymbolPrice serves from it without touching Storage
// or the exchange API. Verifies the fee amount lines up with the in-process
// price — if the WsPrices lookup were bypassed, the fee would come out as
// zero (no Storage, no Exchange) and the assertion fails.
func TestGetFeesUsesWsPricesInProcessSnapshot(t *testing.T) {
	event := buildTradeWithBNBFee(0.1)
	event.WsPrices = map[string]float64{
		"BNB/USDC": 500, // 0.1 BNB * 500 USDC = 50 USDC fee
	}

	fees := GetFees(event)

	const want = 50.0
	if fees != want {
		t.Fatalf("fees mismatch: got %f, want %f", fees, want)
	}
}

// TestGetFeesFallsBackToExchangeWhenWsPricesEmpty proves the fallback chain
// does not short-circuit on a nil map: getSymbolPrice must still try Storage
// (nil here) and then the exchange client. Since the event holds no Exchange
// either, getSymbolPrice returns an error path and the fee contribution from
// the third asset is zero. This asserts the absence of WsPrices is not a
// crash path.
func TestGetFeesFallsBackToExchangeWhenWsPricesEmpty(t *testing.T) {
	event := buildTradeWithBNBFee(0.1)
	// event.WsPrices is nil, event.Storage is nil, event.Exchange is zero.

	fees := GetFees(event)
	if fees != 0 {
		t.Fatalf("expected 0 fees when price resolution fails, got %f", fees)
	}
}

func TestGetFeesHandlesNilAPIErrorFromExchangePrice(t *testing.T) {
	event := buildTradeWithBNBFee(0.1)
	event.Exchange = exchange.Exchange{
		IsCustom: true,
		CustomActions: exchangeAggregates.Actions{
			GetPrice: func(symbol string) (float64, *common.APIError) {
				return 500, nil
			},
		},
	}

	got := GetFees(event)
	const want = 50.0
	if got != want {
		t.Fatalf("fees mismatch: got %f, want %f", got, want)
	}
}

// TestGetFeesBaseAssetFee does not hit getSymbolPrice at all — the fee is
// paid in the base asset (BTC), so the calculation uses data.Price directly.
// Keeps the happy path without any price lookup pinned.
func TestGetFeesBaseAssetFee(t *testing.T) {
	trade := aggragates.Trades{
		Symbol:        "BTC/USDT",
		PositionPrice: 65000,
		History: []aggragates.TradesHistory{
			{
				Price: 64000, // fee price != position price
				Fees:  []aggragates.TradesFees{{Asset: "BTC", Fee: 0.001}},
			},
		},
	}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{PriceFilter: 2}

	// feesInQuote = 0.001 BTC * 64000 USDT = 64 USDT. Inverse=false so
	// GetFees returns feesInQuote.
	got := GetFees(events.Events{Trade: trade})
	const want = 64.0
	if got != want {
		t.Fatalf("fees mismatch: got %f, want %f", got, want)
	}
}

// TestGetFeesInverseReturnsBaseCurrencyFees pins the Inverse branch at the
// bottom of GetFees. Inverse trades profit in the base asset, so the
// returned value must be feesInBase rather than feesInQuote.
func TestGetFeesInverseReturnsBaseCurrencyFees(t *testing.T) {
	trade := aggragates.Trades{
		Symbol:        "BTC/USDT",
		PositionPrice: 65000,
		Inverse:       true,
		History: []aggragates.TradesHistory{
			{
				Price: 64000,
				Fees:  []aggragates.TradesFees{{Asset: "BTC", Fee: 0.001}},
			},
		},
	}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{PriceFilter: 2}

	got := GetFees(events.Events{Trade: trade})
	const want = 0.001 // feesInBase, unchanged
	if got != want {
		t.Fatalf("fees mismatch: got %f, want %f", got, want)
	}
}

// TestGetFeesBaseQuoteCrossConvertsBetweenDenominations pins the dual-output
// contract used by Sell + HasFunds. A base-asset fee fills feeInBase directly
// and feeInQuote via fill price; a quote-asset fee fills feeInQuote directly
// and feeInBase via division by fill price. The trade-execution callers rely
// on both buckets being populated regardless of which asset the fee was paid
// in.
func TestGetFeesBaseQuoteCrossConvertsBetweenDenominations(t *testing.T) {
	trade := aggragates.Trades{
		Symbol: "BTC/USDT",
		History: []aggragates.TradesHistory{
			{Price: 64000, Fees: []aggragates.TradesFees{{Asset: "BTC", Fee: 0.001}}},
			{Price: 65000, Fees: []aggragates.TradesFees{{Asset: "USDT", Fee: 1.0}}},
		},
	}

	feeInBase, feeInQuote := GetFeesBaseQuote(events.Events{Trade: trade})

	// feeInBase = 0.001 BTC (direct) + 1.0 USDT / 65000 USDT/BTC = ~1.5385e-5 BTC
	const wantFeeInBase = 0.001 + 1.0/65000.0
	if feeInBase != wantFeeInBase {
		t.Fatalf("feeInBase = %v, want %v", feeInBase, wantFeeInBase)
	}

	// feeInQuote = 1.0 USDT (direct) + 0.001 BTC * 64000 USDT/BTC = 65.0 USDT
	const wantFeeInQuote = 1.0 + 0.001*64000
	if feeInQuote != wantFeeInQuote {
		t.Fatalf("feeInQuote = %v, want %v", feeInQuote, wantFeeInQuote)
	}
}

// TestGetFeesBaseQuotePopulatesBothFromBNBFee confirms the BNB-fee fix
// reaches the trade-execution callers (Sell, HasFunds). The legacy
// CalculateFeesOld returned (0, 0) when the only fee was paid in BNB,
// silently dropping it from quantity calculations. GetFeesBaseQuote uses
// getSymbolPrice (via WsPrices) so a positive BNB fee now contributes to
// both denominations.
func TestGetFeesBaseQuotePopulatesBothFromBNBFee(t *testing.T) {
	event := buildTradeWithBNBFee(0.1)
	event.WsPrices = map[string]float64{
		"BNB/USDC": 500,   // 0.1 BNB priced at 500 USDC each = 50 USDC fee in quote
		"BTC/USDC": 65000, // needed to convert the BNB fee into base (BTC) via ProfitAsset
	}

	feeInBase, feeInQuote := GetFeesBaseQuote(event)

	if feeInQuote <= 0 {
		t.Errorf("feeInQuote = %v, want >0 (BNB fee must reach quote denomination)", feeInQuote)
	}
	if feeInBase <= 0 {
		t.Errorf("feeInBase = %v, want >0 (BNB fee must reach base denomination via ProfitAsset)", feeInBase)
	}
}
