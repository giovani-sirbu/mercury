package aggragates

import "testing"

// The first-buy gate injection is a product rule: cooldown, UseAI and
// UsePatterns own it; CrashGuard, SmartTakeLoss and RegimeHold fetch the
// verdict for adds and exits only, and PowerLawQuantiles does nothing yet.
// Pinned so the difference between "fetches the verdict" and "gates the
// first buy" stays deliberate rather than accidental.
func TestInjectsEntryHoldIsOwnedByEntryFlags(t *testing.T) {
	cases := []struct {
		name   string
		params StrategyParams
		want   bool
	}{
		{"nothing on", StrategyParams{}, false},
		{"cooldown only", StrategyParams{Cooldown: true}, true},
		{"useAI only", StrategyParams{UseAI: true}, true},
		{"usePatterns only", StrategyParams{UsePatterns: true}, true},
		{"crashGuard only", StrategyParams{CrashGuard: true}, false},
		{"smartTakeLoss only", StrategyParams{SmartTakeLoss: true}, false},
		{"regimeHold only", StrategyParams{RegimeHold: true}, false},
		{"powerLawQuantiles only", StrategyParams{PowerLawQuantiles: true}, false},
		{"crashGuard with usePatterns", StrategyParams{CrashGuard: true, UsePatterns: true}, true},
	}
	for _, tc := range cases {
		if got := tc.params.InjectsEntryHold(); got != tc.want {
			t.Errorf("%s: InjectsEntryHold = %v, want %v", tc.name, got, tc.want)
		}
	}
}
