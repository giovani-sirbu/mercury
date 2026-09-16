package smarttakeloss

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// max(minArmDepth, Depths − ArmDepthOffset), never at or past the last
// configured depth. Written against the constants so a recalibration of the
// offset moves the expectations with it.
func TestArmDepth(t *testing.T) {
	for maxDepths := 0; maxDepths <= 12; maxDepths++ {
		got := ArmDepth(maxDepths)
		if got < minArmDepth {
			t.Errorf("ArmDepth(%d) = %d, under minArmDepth %d", maxDepths, got, minArmDepth)
		}
		if maxDepths > minArmDepth && got >= maxDepths {
			t.Errorf("ArmDepth(%d) = %d, at or past the last configured depth", maxDepths, got)
		}
		if want := maxDepths - ArmDepthOffset; want >= minArmDepth && want < maxDepths && got != want {
			t.Errorf("ArmDepth(%d) = %d, want Depths − offset = %d", maxDepths, got, want)
		}
		if maxDepths <= minArmDepth && got != minArmDepth {
			t.Errorf("ArmDepth(%d) = %d, want the floor %d", maxDepths, got, minArmDepth)
		}
	}
}

// The w3s row (Depths 8) arms at ArmDepth fills and not one earlier; a
// fractional Depths is floored before the subtraction.
func TestArmedAtArmDepth(t *testing.T) {
	arm := ArmDepth(8)
	if armed(testutil.LadderTrade(false, fills(arm-1, "17:38:00")...)) {
		t.Fatalf("%d of 8 filled entries must not arm", arm-1)
	}
	at := testutil.LadderTrade(false, fills(arm, "17:38:00")...)
	if !armed(at) {
		t.Fatalf("%d of 8 filled entries must arm", arm)
	}

	at.StrategyPair.StrategySettings[0].Depths = 8.9
	if !armed(at) {
		t.Fatalf("Depths 8.9 floors to 8 and arms at %d", arm)
	}
	at.StrategyPair.StrategySettings[0].Depths = 9
	if armed(at) {
		t.Fatalf("Depths 9 arms at %d, not %d", ArmDepth(9), arm)
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
