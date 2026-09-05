package crashguard

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func singleRowSettings(percentage float64) []aggragates.StrategySettings {
	return []aggragates.StrategySettings{{Percentage: percentage, Tolerance: 0.5}}
}

func TestWidenedPercentageDoublesPerDepthOnSingleRow(t *testing.T) {
	settings := singleRowSettings(2)

	cases := []struct {
		depthIndex int
		want       float64
	}{
		{0, 2},  // depth 1 keeps the configured gap
		{1, 4},  // depth 2
		{2, 8},  // depth 3
		{3, 16}, // depth 4
		{4, 32}, // depth 5
		{5, 45}, // depth 6 would be 64 — capped
	}
	for _, c := range cases {
		got, _ := WidenedPercentage(settings, c.depthIndex, true)
		if got != c.want {
			t.Errorf("depthIndex %d: got %v, want %v", c.depthIndex, got, c.want)
		}
	}
}

func TestWidenedPercentageInactiveIsIdentity(t *testing.T) {
	settings := singleRowSettings(2)
	for depthIndex := 0; depthIndex < 6; depthIndex++ {
		got, changed := WidenedPercentage(settings, depthIndex, false)
		if got != 2 || changed {
			t.Fatalf("inactive widening changed depth %d: %v", depthIndex, got)
		}
	}
}

// A depth whose own row exists keeps its configured percentage: explicit
// ladders stay authoritative, the doubling covers only the fallback depths.
func TestWidenedPercentageRespectsExplicitRows(t *testing.T) {
	settings := []aggragates.StrategySettings{
		{Percentage: 2}, {Percentage: 3}, {Percentage: 5},
	}

	for depthIndex := 0; depthIndex < 3; depthIndex++ {
		got, changed := WidenedPercentage(settings, depthIndex, true)
		if changed || got != settings[depthIndex].Percentage {
			t.Errorf("explicit row %d overridden: got %v", depthIndex, got)
		}
	}

	// Depth 4 has no row: fallback is the base row, widened by 2^3.
	got, changed := WidenedPercentage(settings, 3, true)
	if !changed || got != 16 {
		t.Errorf("fallback depth: got %v (changed=%v), want 16", got, changed)
	}
}

func TestWidenSettingsForCrashNeverMutatesInput(t *testing.T) {
	settings := singleRowSettings(2)

	widened := WidenSettingsForCrash(settings, 3, true)

	if settings[0].Percentage != 2 {
		t.Fatalf("input slice mutated: %v", settings[0].Percentage)
	}
	if widened[0].Percentage != 16 {
		t.Fatalf("widened copy wrong: %v", widened[0].Percentage)
	}
	if widened[0].Tolerance != settings[0].Tolerance {
		t.Fatalf("tolerance must stay untouched, got %v", widened[0].Tolerance)
	}
}

func TestWidenSettingsForCrashIdentityWhenInactive(t *testing.T) {
	settings := singleRowSettings(2)

	if got := WidenSettingsForCrash(settings, 3, false); &got[0] != &settings[0] {
		t.Fatal("inactive widening must return the input slice itself")
	}
	if got := WidenSettingsForCrash(settings, 0, true); &got[0] != &settings[0] {
		t.Fatal("depth on its own row must return the input slice itself")
	}
}

func TestWidenedPercentageEmptySettings(t *testing.T) {
	if got, changed := WidenedPercentage(nil, 2, true); got != 0 || changed {
		t.Fatalf("empty settings must answer 0, got %v", got)
	}
}
