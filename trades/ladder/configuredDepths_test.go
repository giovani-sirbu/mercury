package ladder

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// depthTrade is a long whose ladder has filled the given number of entries,
// one distinct exchange order each, on the given settings rows.
func depthTrade(filled int, settings ...aggragates.StrategySettings) aggragates.Trades {
	trade := aggragates.Trades{
		ID:           7412,
		Symbol:       "LINK/USDT",
		StrategyPair: aggragates.StrategiesPairs{StrategySettings: settings},
	}

	for index := 0; index < filled; index++ {
		trade.History = append(trade.History, aggragates.TradesHistory{
			Type:     "BUY",
			Quantity: 1,
			Price:    100 - float64(index),
			OrderId:  int64(index + 1),
		})
	}

	return trade
}

// A single configured row governs every depth, so the ceiling it carries is
// the answer however far down the ladder already is.
func TestConfiguredDepthsReadsTheOnlyRow(t *testing.T) {
	settings := aggragates.StrategySettings{Percentage: 2.5, Depths: 8}

	for _, filled := range []int{0, 1, 5, 8, 20} {
		if got := ConfiguredDepths(depthTrade(filled, settings)); got != 8 {
			t.Errorf("%d entries filled: ConfiguredDepths = %d, want 8", filled, got)
		}
	}
}

// With a row per depth the answer is the row the NEXT fill would use — the
// row that governs the entry being decided, not the one that placed the last.
func TestConfiguredDepthsReadsTheRowOfTheNextFill(t *testing.T) {
	settings := []aggragates.StrategySettings{
		{Depths: 4},
		{Depths: 5},
		{Depths: 6},
		{Depths: 7},
	}

	if got := ConfiguredDepths(depthTrade(2, settings...)); got != 6 {
		t.Errorf("ConfiguredDepths = %d, want the row of the third entry", got)
	}
	// Past the configured rows SettingsIndexOrBase falls back to the base
	// row, never to the last one.
	if got := ConfiguredDepths(depthTrade(9, settings...)); got != 4 {
		t.Errorf("ConfiguredDepths = %d, want the base row", got)
	}
}

// Depths is configured as a float and a fraction of an entry cannot be
// placed, so the ceiling is floored — the same read the smart take loss arms
// on.
func TestConfiguredDepthsFloorsAFractionalRow(t *testing.T) {
	if got := ConfiguredDepths(depthTrade(1, aggragates.StrategySettings{Depths: 8.7})); got != 8 {
		t.Errorf("ConfiguredDepths = %d, want the fractional ceiling floored", got)
	}
}

// No settings row means no known ceiling. The callers read 0 as unknown, so
// answering anything else would invent a ladder the pair never configured.
func TestConfiguredDepthsIsZeroWithoutSettings(t *testing.T) {
	if got := ConfiguredDepths(depthTrade(3)); got != 0 {
		t.Errorf("ConfiguredDepths = %d, want 0 without a settings row", got)
	}
}
