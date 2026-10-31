package gates

import "testing"

// PositionType reads a force-trailing re-anchor as the state it re-arms; every other state, the
// legacy names included, comes back as it is.
func TestPositionTypeReadsTheForceTrailingStatesAsTheirOwn(t *testing.T) {
	for positionType, want := range map[string]string{
		"forceTrailingBuy":        "trailingBuy",
		"forceTrailingTakeProfit": "takeProfit",
		"trailingBuy":             "trailingBuy",
		"takeProfit":              "takeProfit",
		"takeLoss":                "takeLoss",
		"buy":                     "buy",
		"":                        "",
	} {
		if got := PositionType(positionType); got != want {
			t.Errorf("PositionType(%q) = %q, want %q", positionType, got, want)
		}
	}
}
