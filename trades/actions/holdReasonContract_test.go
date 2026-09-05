package actions

import (
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/crashguard"
	"github.com/giovani-sirbu/mercury/trades/gates/regime"
	"github.com/giovani-sirbu/mercury/trades/gates/smarttakeloss"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// The capitulation override recognises the holds of other families by their
// text (crashguard.keepCapitulationHold / capitulationEligibleHold). Those
// families live in their own packages now, and crashguard cannot import
// smarttakeloss, so this test — in the one package that imports all of them
// — pins the texts together: a rename on either side fails here instead of
// silently un-coupling capitulation.
func TestHoldReasonContractAcrossFamilies(t *testing.T) {
	if !strings.HasPrefix(smarttakeloss.HTFFreezeReason, "smart-take-loss: HTF") {
		t.Fatalf("smarttakeloss.HTFFreezeReason = %q, capitulation keeps holds by the prefix \"smart-take-loss: HTF\"", smarttakeloss.HTFFreezeReason)
	}
	if !strings.HasPrefix(crashguard.DeepHoldReason, "crash-guard: deep") {
		t.Fatalf("crashguard.DeepHoldReason = %q, capitulation keeps holds by the prefix \"crash-guard: deep\"", crashguard.DeepHoldReason)
	}

	// regime.HoldReason must produce the three prefixes capitulation bypasses.
	deep := testutil.DeepTrade(false) // four filled entries, stopLoss
	shock := regime.HoldReason(events.Events{Trade: deep}, "stopLoss", aggragates.AIIndicators{
		HasRegimeVerdict: true, AddAllowed: true,
		Regimes: map[string]string{"15m": regime.ShockDown},
	})
	if !strings.HasPrefix(shock, regime.ShockHoldPrefix) {
		t.Errorf("shock hold = %q, want the %q prefix", shock, regime.ShockHoldPrefix)
	}

	addVeto := regime.HoldReason(events.Events{Trade: deep}, "stopLoss", aggragates.AIIndicators{
		HasRegimeVerdict: true, AddAllowed: false,
		Regimes: map[string]string{"4h": regime.DownPersist, "15m": "mixed"},
	})
	if !strings.HasPrefix(addVeto, regime.AddVetoPrefix) {
		t.Errorf("add veto = %q, want the %q prefix", addVeto, regime.AddVetoPrefix)
	}

	inverse := deep
	inverse.Inverse = true
	inverseVeto := regime.HoldReason(events.Events{Trade: inverse}, "stopLoss", aggragates.AIIndicators{
		HasRegimeVerdict: true, AddAllowed: true,
		Regimes: map[string]string{"4h": regime.UpPersist, "15m": "mixed"},
	})
	if !strings.HasPrefix(inverseVeto, regime.InverseAddVetoPrefix) {
		t.Errorf("inverse add veto = %q, want the %q prefix", inverseVeto, regime.InverseAddVetoPrefix)
	}
}
