package ladder

import (
	"math"
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func sizingTrade(inverse bool, positionPrice float64, settings aggragates.StrategySettings) aggragates.Trades {
	return aggragates.Trades{
		Inverse:       inverse,
		PositionPrice: positionPrice,
		StrategyPair: aggragates.StrategiesPairs{
			TradeFilters:     aggragates.TradeFilters{MinNotional: 5},
			StrategySettings: []aggragates.StrategySettings{settings},
		},
	}
}

func ladderSettings() aggragates.StrategySettings {
	return aggragates.StrategySettings{
		Depths:       8,
		MinDepths:    6,
		Multiplier:   2,
		Percentage:   2,
		ImpasseDepth: 6,
	}
}

// Regression for backtest #56 trades 8195/8198: an inverse ladder spends base
// units that double exactly, so sizing it with the discounted ratio planned
// only ~88% of the real cost and every max-depth trade blocked on rung 8
// (needed 128×bid, had ~99×bid left).
func TestCalculateInitialBidInverseLadderFitsTheWallet(t *testing.T) {
	wallet := 151532.0 // HBAR free when trade 8195 started
	settings := ladderSettings()

	bid, err := CalculateInitialBid(wallet, sizingTrade(true, 0.0817, settings), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Derived from the haircut budget at max depth, with the inverse percentage-0
	// rule, so retuning InitialBidReservePercent cannot break this expectation.
	want := GetInitialBidByDepth(wallet*(1-InitialBidReservePercent/100), settings.Depths, settings.Multiplier, 0)
	if math.Abs(bid-want) > 0.001 {
		t.Fatalf("inverse bid = %f, want %f", bid, want)
	}

	// The whole doubling ladder, last rung included, fits inside the wallet.
	if total := bid * 255; total > wallet {
		t.Fatalf("full ladder costs %f from a %f wallet", total, wallet)
	}
	if left := wallet - bid*127; left < bid*128 {
		t.Fatalf("rung 8 needs %f but only %f is left", bid*128, left)
	}
}

func TestCalculateInitialBidNormalKeepsDiscountedRatio(t *testing.T) {
	wallet := 50000.0
	settings := ladderSettings()

	bid, err := CalculateInitialBid(wallet, sizingTrade(false, 118000, settings), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Derived from the haircut budget at max depth, keeping the quote ladder's
	// price discount, so retuning InitialBidReservePercent cannot break this.
	want := GetInitialBidByDepth(wallet*(1-InitialBidReservePercent/100), settings.Depths, settings.Multiplier, settings.Percentage)
	if math.Abs(bid-want) > 0.001 {
		t.Fatalf("normal bid = %f, want %f", bid, want)
	}
}

func TestCalculateInitialBidStillRefusesDustWallets(t *testing.T) {
	// Even at minDepths 6 this wallet's bid lands under the 5 minNotional.
	_, err := CalculateInitialBid(200, sizingTrade(false, 118000, ladderSettings()), 0)
	if err == nil {
		t.Fatal("expected an insufficient-funds error, got none")
	}
	if !strings.Contains(err.Error(), "Insufficient funds") {
		t.Fatalf("error = %q, want it to name insufficient funds", err.Error())
	}
}

func TestCalculateInitialBidImpasseSizesOnImpasseDepth(t *testing.T) {
	wallet := 63000.0
	settings := ladderSettings()
	trade := sizingTrade(true, 0.1, settings)
	trade.ParentID = 7

	bid, err := CalculateInitialBid(wallet, trade, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Derived from the haircut budget at the impasse child's depth, with the
	// inverse percentage-0 rule, so a retune of the reserve cannot break this.
	want := GetInitialBidByDepth(wallet*(1-InitialBidReservePercent/100), settings.ImpasseDepth, settings.Multiplier, 0)
	if math.Abs(bid-want) > 0.001 {
		t.Fatalf("impasse bid = %f, want %f", bid, want)
	}
}
