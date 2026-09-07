package smarttakeloss

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// max(2, Depths − 3), never at or past the last configured depth.
func TestArmDepth(t *testing.T) {
	cases := []struct{ maxDepths, want int }{
		{9, 6}, {8, 5}, {7, 4}, {6, 3}, {5, 2}, {4, 2}, {3, 2}, {2, 2}, {1, 2}, {0, 2},
	}
	for _, c := range cases {
		if got := ArmDepth(c.maxDepths); got != c.want {
			t.Errorf("ArmDepth(%d) = %d, want %d", c.maxDepths, got, c.want)
		}
	}
}

// The w3s row (Depths 8) arms at the fifth fill, not the fourth; a
// fractional Depths is floored before the subtraction.
func TestArmedAtArmDepth(t *testing.T) {
	if armed(testutil.LadderTrade(false, fills(4, "17:38:00")...)) {
		t.Fatal("4 of 8 filled entries must not arm")
	}
	five := testutil.LadderTrade(false, fills(5, "17:38:00")...)
	if !armed(five) {
		t.Fatal("5 of 8 filled entries must arm")
	}

	five.StrategyPair.StrategySettings[0].Depths = 8.9
	if !armed(five) {
		t.Fatal("Depths 8.9 floors to 8 and arms at 5")
	}
	five.StrategyPair.StrategySettings[0].Depths = 9
	if armed(five) {
		t.Fatal("Depths 9 arms at 6, not 5")
	}
}

func TestArmedNeedsSettingsAndDepths(t *testing.T) {
	trade := testutil.LadderTrade(false, fills(5, "17:38:00")...)
	trade.StrategyPair.StrategySettings[0].Depths = 0
	if armed(trade) {
		t.Fatal("Depths 0 must not arm")
	}
	trade.StrategyPair.StrategySettings = nil
	if armed(trade) {
		t.Fatal("no settings must not arm")
	}
}

// A fund block used to arm unconditionally. It no longer does: hermes ticks
// no blocked trade and the rule reads only the fills.
func TestArmedIgnoresBlockedStatus(t *testing.T) {
	trade := testutil.LadderTrade(false, fills(2, "17:38:00")...)
	trade.Status = aggragates.Blocked
	if armed(trade) {
		t.Fatal("a blocked trade below ArmDepth must not arm")
	}
}
