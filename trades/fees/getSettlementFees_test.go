package fees

import (
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// TestGetSettlementFeesSpotCountsOnlyQuoteFees pins the embodiment rule for
// spot trades: the buy leg's base-asset fee already shrank the base the sell
// could move, so only the sell leg's quote fee still reduces profit.
func TestGetSettlementFeesSpotCountsOnlyQuoteFees(t *testing.T) {
	trade := aggragates.Trades{
		Symbol: "BTC/USDT",
		History: []aggragates.TradesHistory{
			{Type: "buy", Quantity: 1, Price: 64000, Fees: []aggragates.TradesFees{{Asset: "BTC", Fee: 0.001}}},
			{Type: "sell", Quantity: 0.999, Price: 65000, Fees: []aggragates.TradesFees{{Asset: "USDT", Fee: 64.9}}},
		},
	}

	got := GetSettlementFees(events.Events{Trade: trade})

	const want = 64.9 // the base fee is embodied, never re-charged
	if got != want {
		t.Fatalf("settlement fees = %v, want %v", got, want)
	}
}

// TestGetSettlementFeesInverseCountsOnlyBaseFees is the mirror: the sell
// legs' quote fees already shrank the quote the buy-back could spend, so only
// the buy-back's base fee still reduces the base-denominated profit.
func TestGetSettlementFeesInverseCountsOnlyBaseFees(t *testing.T) {
	trade := aggragates.Trades{
		Symbol:  "BTC/USDT",
		Inverse: true,
		History: []aggragates.TradesHistory{
			{Type: "sell", Quantity: 1, Price: 65000, Fees: []aggragates.TradesFees{{Asset: "USDT", Fee: 65}}},
			{Type: "buy", Quantity: 1.001, Price: 64800, Fees: []aggragates.TradesFees{{Asset: "BTC", Fee: 0.001001}}},
		},
	}

	got := GetSettlementFees(events.Events{Trade: trade})

	const want = 0.001001 // the quote fee is embodied in the buy-back sizing
	if got != want {
		t.Fatalf("settlement fees = %v, want %v", got, want)
	}
}

// TestGetSettlementFeesPricesBNBFeePerDirection: a fee paid from the separate
// BNB wallet is embodied in nothing, so it must always be charged — in quote
// for spot, in base (via ProfitAsset) for inverse.
func TestGetSettlementFeesPricesBNBFeePerDirection(t *testing.T) {
	event := buildTradeWithBNBFee(0.1)
	event.WsPrices = map[string]float64{
		"BNB/USDC": 500,
		"BTC/USDC": 65000,
	}

	if got := GetSettlementFees(event); got != 50.0 {
		t.Fatalf("spot BNB settlement fee = %v, want 50 (0.1 BNB * 500)", got)
	}

	event.Trade.Inverse = true
	wantBase := 0.1 * 500 / 65000.0
	if got := GetSettlementFees(event); math.Abs(got-wantBase) > 1e-12 {
		t.Fatalf("inverse BNB settlement fee = %v, want %v", got, wantBase)
	}
}
