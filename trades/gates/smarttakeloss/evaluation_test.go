package smarttakeloss

import (
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// highRiskAI is a verdict that endangers the given direction with no
// favorable reversal evidence and plenty of projected block time.
func highRiskAI(inverse bool) aggragates.AIIndicators {
	ai := aggragates.AIIndicators{HasContinuationVerdict: true, DailyNatrPct: 0.05}
	if inverse {
		ai.UpContinuationRisk = 80
	} else {
		ai.DownContinuationRisk = 80
	}
	return ai
}

func TestSmartTakeLossArmedEdges(t *testing.T) {
	ai := highRiskAI(false)
	armAt := ArmDepth(9)
	if _, high := Evaluate(testutil.DeepLadderTrade(armAt-1, false), 90, ai); high {
		t.Fatalf("depth %d of 9 must not arm", armAt-1)
	}
	if _, high := Evaluate(testutil.DeepLadderTrade(armAt, false), 90, ai); !high {
		t.Fatalf("depth %d of 9 must arm and trigger", armAt)
	}
	if _, high := Evaluate(testutil.DeepLadderTrade(9, false), 90, ai); !high {
		t.Fatal("depth 9 of 9 must stay armed")
	}

	fractional := testutil.DeepLadderTrade(ArmDepth(8), false)
	fractional.StrategyPair.StrategySettings[0].Depths = 8.5
	if _, high := Evaluate(fractional, 90, ai); !high {
		t.Fatalf("Depths 8.5 floors to 8, so %d entries must arm", ArmDepth(8))
	}

	// Short ladders clamp to the floor: Depths 4 with offset 3 would arm at 1,
	// but the first rungs are where a grid trades through volatility.
	shortLadder := testutil.DeepLadderTrade(minArmDepth-1, false)
	shortLadder.StrategyPair.StrategySettings[0].Depths = 4
	if _, high := Evaluate(shortLadder, 90, ai); high {
		t.Fatal("below the min arm depth a short ladder must not arm")
	}
	shortLadder = testutil.DeepLadderTrade(minArmDepth, false)
	shortLadder.StrategyPair.StrategySettings[0].Depths = 4
	if _, high := Evaluate(shortLadder, 90, ai); !high {
		t.Fatal("a short ladder must arm at the min arm depth floor")
	}

	zeroDepths := testutil.DeepLadderTrade(8, false)
	zeroDepths.StrategyPair.StrategySettings[0].Depths = 0
	if _, high := Evaluate(zeroDepths, 90, ai); high {
		t.Fatal("a ladder without a Depths sizing must never depth-arm")
	}

	blocked := testutil.DeepLadderTrade(1, false)
	blocked.Status = aggragates.Blocked
	if _, high := Evaluate(blocked, 90, ai); !high {
		t.Fatal("an already-blocked trade must arm at any depth")
	}

	child := testutil.DeepLadderTrade(8, false)
	child.ParentID = 7
	if _, high := Evaluate(child, 90, ai); high {
		t.Fatal("impasse children stay with their parent")
	}
}

// A fund-blocked ladder that can close in profit banks it regardless of the
// continuation risk: the risk gate reads the market, not the wallet. Run 79's
// 14632 recovered to +2.2% in a rising market (down-risk naturally low), the
// gated profit branch stayed silent, and ~21k USDT sat captive for months.
func TestSmartTakeLossBlockedProfitBanksUnconditionally(t *testing.T) {
	lowRisk := aggragates.AIIndicators{HasContinuationVerdict: true, DownContinuationRisk: 10, DailyNatrPct: 0.05}

	blocked := testutil.DeepLadderTrade(6, false)
	blocked.Status = aggragates.Blocked
	eval, high := Evaluate(blocked, 110, lowRisk)
	if !high || !eval.ProfitNow {
		t.Fatalf("a blocked trade in profit must bank it at ANY risk, got high=%v eval=%+v", high, eval)
	}

	if _, high := Evaluate(blocked, 60, lowRisk); high {
		t.Fatal("a blocked trade at a LOSS keeps the risk-gated semantics: low risk must not force-sell")
	}

	activeArmed := testutil.DeepLadderTrade(8, false)
	if _, high := Evaluate(activeArmed, 110, lowRisk); high {
		t.Fatal("an armed but UNBLOCKED trade keeps the old gate: low risk + profit must not force-sell (the normal takeProfit path owns it)")
	}
}

func TestSmartTakeLossRiskAndReversalGates(t *testing.T) {
	trade := testutil.DeepLadderTrade(8, false)

	ai := highRiskAI(false)
	ai.DownContinuationRisk = RiskThreshold - 0.1
	if _, high := Evaluate(trade, 90, ai); high {
		t.Fatal("risk under the threshold must not trigger")
	}

	ai = highRiskAI(false)
	ai.DownContinuationRisk = RiskThreshold
	if _, high := Evaluate(trade, 90, ai); !high {
		t.Fatal("risk at the threshold must trigger")
	}

	ai = highRiskAI(false)
	ai.ReversalUpEvidence = MinReversalEvidence
	if _, high := Evaluate(trade, 90, ai); high {
		t.Fatal("favorable reversal evidence must veto the exit")
	}

	wrongDirection := aggragates.AIIndicators{HasContinuationVerdict: true, UpContinuationRisk: 90, DailyNatrPct: 0.05}
	if _, high := Evaluate(trade, 90, wrongDirection); high {
		t.Fatal("a long trade must read the DOWN risk, not the up risk")
	}
	inverse := testutil.DeepLadderTrade(8, true)
	if _, high := Evaluate(inverse, 110, wrongDirection); !high {
		t.Fatal("an inverse trade must read the UP risk")
	}
	vetoedInverse := highRiskAI(true)
	vetoedInverse.ReversalDownEvidence = MinReversalEvidence
	if _, high := Evaluate(inverse, 110, vetoedInverse); high {
		t.Fatal("an inverse trade must be vetoed by DOWN reversal evidence")
	}

	if _, high := Evaluate(trade, 90, aggragates.AIIndicators{}); high {
		t.Fatal("a zero verdict must be inert")
	}
}

func TestSmartTakeLossBlockedDaysGate(t *testing.T) {
	trade := testutil.DeepLadderTrade(8, false)
	required := requiredRecoveryPct(trade, 90) // (93-90)/90*100 ≈ 3.333
	if math.Abs(required-3.0/90*100) > 1e-9 {
		t.Fatalf("long required recovery = %f, want %f", required, 3.0/90*100)
	}

	ai := highRiskAI(false)
	ai.DailyNatrPct = required / MinBlockedDays // exactly 30 days
	eval, high := Evaluate(trade, 90, ai)
	if !high || eval.ProfitNow {
		t.Fatalf("30 projected days must trigger a loss exit, got %v (%+v)", high, eval)
	}
	ai.DailyNatrPct = required / (MinBlockedDays - 0.1)
	if _, high := Evaluate(trade, 90, ai); high {
		t.Fatal("29.9 projected days must not trigger")
	}
	ai.DailyNatrPct = 0
	if _, high := Evaluate(trade, 90, ai); high {
		t.Fatal("a zero NATR yardstick must be inert on the loss path")
	}
}

func TestSmartTakeLossProfitNowShortCircuit(t *testing.T) {
	trade := testutil.DeepLadderTrade(8, false)
	ai := highRiskAI(false)
	ai.DailyNatrPct = 0 // must not matter on the profit path
	eval, high := Evaluate(trade, 95, ai)
	if !high || !eval.ProfitNow {
		t.Fatalf("profitable close under HIGH risk must trigger, got %v (%+v)", high, eval)
	}
}

func TestSmartTakeLossPositionMatchesVariantSwitch(t *testing.T) {
	want := "sellLoss"
	if TrailingExit {
		want = "update_smartTakeLoss"
	}
	position, ok := Position(testutil.DeepLadderTrade(8, false), 90, highRiskAI(false))
	if !ok || position != want {
		t.Fatalf("got %q/%v, want %q", position, ok, want)
	}
	if _, ok := Position(testutil.DeepLadderTrade(ArmDepth(9)-1, false), 90, highRiskAI(false)); ok {
		t.Fatal("an unarmed trade must not force a position")
	}
}
