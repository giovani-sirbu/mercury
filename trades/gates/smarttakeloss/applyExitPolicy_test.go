package smarttakeloss

import (
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/crashguard"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

func TestApplySmartTakeLossExitPolicyUnderwaterAndEmergency(t *testing.T) {
	trade := testutil.DeepLadderTrade(8, false)
	trade.PositionType = "smartTakeLossTrail"
	triggered := time.Unix(1_700_000_000, 0).UTC()
	trade.Logs = []aggragates.TradesLogs{{
		Message:   TriggeredPrefix + ": risk 80",
		CreatedAt: triggered,
	}}

	if got := ApplyExitPolicy(trade, "sellLoss", 90, triggered, false); got != "" {
		t.Fatalf("underwater trail sellLoss must stay empty, got %q", got)
	}
	if got := ApplyExitPolicy(trade, "sellLoss", 97, triggered, false); got != "sellLoss" {
		t.Fatalf("profitable trail sellLoss must execute, got %q", got)
	}

	fourteen := triggered.Add(EmergencyAfter)
	if got := ApplyExitPolicy(trade, "sellLoss", 90, fourteen, true); got != "" {
		t.Fatalf("crash-active emergency must wait, got %q", got)
	}
	if got := ApplyExitPolicy(trade, "", 90, fourteen, false); got != "sellLoss" {
		t.Fatalf("14d underwater + crash clear must allow sellLoss, got %q", got)
	}
}

func TestStaleBagDueCutsUnderwaterDeepTrade(t *testing.T) {
	trade := testutil.DeepLadderTrade(crashguard.DeRiskMinDepth, false)
	trade.Strategy.Params.SmartTakeLoss = true
	trade.CreatedAt = time.Unix(1_700_000_000, 0).UTC()
	now := trade.CreatedAt.Add(StaleAfter)

	if staleBagDue(trade, 90, trade.CreatedAt.Add(20*24*time.Hour)) {
		t.Fatal("before 21d the bag must stay")
	}
	if !staleBagDue(trade, 90, now) {
		t.Fatal("21d underwater deep bag must be due")
	}
	got := ApplyProtectiveTick(trade, "stopLoss", 90, now, aggragates.AIIndicators{}, false)
	if got.Position != "sellLoss" {
		t.Fatalf("stale bag must flatten, got %+v", got)
	}
	if ApplyProtectiveTick(trade, "takeProfit", 90, now, aggragates.AIIndicators{}, false).Position != "takeProfit" {
		t.Fatal("stale bag must not steal a takeProfit tick")
	}
}
