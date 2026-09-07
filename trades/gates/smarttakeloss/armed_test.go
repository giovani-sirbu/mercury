package smarttakeloss

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// Armed is the predicate hermes and live-testing ask before their
// empty-position early return: flag, parent, ArmDepth.
func TestArmed(t *testing.T) {
	if !Armed(testutil.LadderTrade(false, fills(5, "17:38:00")...)) {
		t.Fatal("a 5-deep w3s ladder under the flag is armed")
	}
	if Armed(testutil.LadderTrade(false, fills(4, "17:38:00")...)) {
		t.Fatal("4 fills are below ArmDepth")
	}

	off := testutil.LadderTrade(false, fills(5, "17:38:00")...)
	off.Strategy.Params.SmartTakeLoss = false
	if Armed(off) {
		t.Fatal("without the flag nothing is armed")
	}

	child := testutil.LadderTrade(false, fills(5, "17:38:00")...)
	child.ParentID = 7
	if Armed(child) {
		t.Fatal("an impasse child belongs to the impasse chain")
	}
}
