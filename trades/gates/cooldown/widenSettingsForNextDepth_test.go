package cooldown

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The doubled step lives on a copy of the base row for one tick; the row the
// trade carries, every other row and every other field stay as configured.
func TestWidenSettingsForNextDepthDoublesTheBaseRowOnACopy(t *testing.T) {
	settings := []aggragates.StrategySettings{
		{Percentage: 2.5, Tolerance: 0.15, TrailingTakeProfit: 0.75, Multiplier: 2.2},
		{Percentage: 3, Tolerance: 0.2},
	}

	widened := WidenSettingsForNextDepth(settings)

	if widened[0].Percentage != 5 {
		t.Fatalf("base row percentage = %v, want 5", widened[0].Percentage)
	}
	if widened[0].Tolerance != 0.15 || widened[0].TrailingTakeProfit != 0.75 || widened[0].Multiplier != 2.2 {
		t.Fatalf("only the percentage may change on the base row, got %+v", widened[0])
	}
	if widened[1].Percentage != 3 || widened[1].Tolerance != 0.2 {
		t.Fatalf("row 1 must stay untouched, got %+v", widened[1])
	}
	if settings[0].Percentage != 2.5 {
		t.Fatalf("input slice mutated: %v", settings[0].Percentage)
	}
	if &widened[0] == &settings[0] {
		t.Fatal("the widened settings must be a copy, never the trade's own slice")
	}
}

func TestWidenSettingsForNextDepthLeavesEmptySettingsAlone(t *testing.T) {
	if got := WidenSettingsForNextDepth(nil); got != nil {
		t.Fatalf("nil settings must come back nil, got %v", got)
	}
	empty := []aggragates.StrategySettings{}
	if got := WidenSettingsForNextDepth(empty); len(got) != 0 {
		t.Fatalf("empty settings must come back empty, got %v", got)
	}
}
