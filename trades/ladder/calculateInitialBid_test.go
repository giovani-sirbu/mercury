package ladder

import (
	"math"
	"reflect"
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

// deeperLadderSettings is ladderSettings a depth deeper and a step wider:
// the shape of the rows an engine names for a first entry while it raises
// them.
func deeperLadderSettings() aggragates.StrategySettings {
	settings := ladderSettings()
	settings.Depths++
	settings.Percentage += 0.5
	return settings
}

// A first entry the engine named rows for is sized from them: handed
// Params.SizingTrade's copy, CalculateInitialBid sizes the named row — a
// depth deeper, so a smaller first entry — while the trade the copy was taken
// from keeps its own rows, the very array.
func TestCalculateInitialBidSizesAFirstEntryOnTheEntrySettings(t *testing.T) {
	const wallet = 50000.0

	trade := sizingTrade(false, 118000, ladderSettings())
	stored := trade.StrategyPair.StrategySettings
	storedRow := stored[0]
	deeper := deeperLadderSettings()

	sizing := aggragates.Params{EntrySettings: []aggragates.StrategySettings{deeper}}.SizingTrade(trade)
	bid, err := CalculateInitialBid(wallet, sizing, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := GetInitialBidByDepth(wallet*(1-InitialBidReservePercent/100), deeper.Depths, deeper.Multiplier, deeper.Percentage)
	if math.Abs(bid-want) > 0.001 {
		t.Fatalf("bid on the named rows = %f, want %f", bid, want)
	}

	configured, err := CalculateInitialBid(wallet, trade, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bid >= configured {
		t.Fatalf("a first entry sized a depth deeper = %f, want less than the configured %f", bid, configured)
	}

	if &trade.StrategyPair.StrategySettings[0] != &stored[0] || !reflect.DeepEqual(trade.StrategyPair.StrategySettings[0], storedRow) {
		t.Fatal("the trade must keep its own rows")
	}
}

// With no rows named — EntrySettings nil or empty — and on a ladder that has
// a fill whatever is named, the sizing copy carries the trade's own rows, the
// very slice, so CalculateInitialBid answers exactly what it answers without
// one.
func TestCalculateInitialBidWithoutEntrySettingsIsUnchanged(t *testing.T) {
	const wallet = 50000.0

	trade := sizingTrade(false, 118000, ladderSettings())
	want, wantErr := CalculateInitialBid(wallet, trade, 0)
	if wantErr != nil {
		t.Fatalf("unexpected error: %v", wantErr)
	}

	filled := trade
	filled.History = []aggragates.TradesHistory{{Type: "BUY", Quantity: 0.001, Price: 118000, OrderId: 1}}
	named := []aggragates.StrategySettings{deeperLadderSettings()}

	for name, sized := range map[string]aggragates.Trades{
		"no rows named":        aggragates.Params{}.SizingTrade(trade),
		"an empty set of rows": aggragates.Params{EntrySettings: []aggragates.StrategySettings{}}.SizingTrade(trade),
		"a ladder with a fill": aggragates.Params{EntrySettings: named}.SizingTrade(filled),
	} {
		if &sized.StrategyPair.StrategySettings[0] != &trade.StrategyPair.StrategySettings[0] {
			t.Errorf("%s: the sizing copy must carry the trade's own rows", name)
		}

		got, err := CalculateInitialBid(wallet, sized, 0)
		if err != nil || got != want {
			t.Errorf("%s: bid = %v (%v), want exactly %v", name, got, err, want)
		}
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
