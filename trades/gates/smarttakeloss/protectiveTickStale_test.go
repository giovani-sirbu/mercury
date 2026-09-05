package smarttakeloss

import (
	"github.com/giovani-sirbu/mercury/trades/gates/crashguard"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The stale-bag cut is a forced exit with a reason of its own: run 97 closed
// four trades through it with nothing on record but BUY_TO_SELLLOSS.
func TestApplyProtectiveTickStaleCutIsForcedAndExplained(t *testing.T) {
	trade := testutil.DeepLadderTrade(crashguard.DeRiskMinDepth, false)
	trade.Strategy.Params.SmartTakeLoss = true
	trade.CreatedAt = time.Unix(1_700_000_000, 0).UTC()
	now := trade.CreatedAt.Add(StaleAfter)

	got := ApplyProtectiveTick(trade, "stopLoss", 90, now, aggragates.AIIndicators{}, false)
	if got.Position != "sellLoss" || !got.STLForced {
		t.Fatalf("stale bag must be a forced sellLoss, got %+v", got)
	}
	if !got.Eval.StaleCut || got.Eval.AgeDays < 20.9 || got.Eval.AgeDays > 21.1 {
		t.Fatalf("the evaluation must carry the stale cut and its age, got %+v", got.Eval)
	}
	if got.Eval.EstProfit >= 0 {
		t.Fatalf("a stale cut is underwater by definition, got est. profit %v", got.Eval.EstProfit)
	}

	message := TriggeredMessage(got.Eval)
	if !strings.HasPrefix(message, StaleCutPrefix) || strings.HasPrefix(message, TriggeredPrefix) {
		t.Fatalf("a stale cut must log under its own prefix, got %q", message)
	}
	if !strings.Contains(message, "age 21 days") || !strings.Contains(message, "depth 4/") {
		t.Fatalf("the row must name the age and the depth, got %q", message)
	}

	// Its row must not start the TRIGGERED-keyed emergency clock.
	trade.Logs = append(trade.Logs, aggragates.TradesLogs{Message: message, CreatedAt: now})
	if !TriggeredAt(trade).IsZero() {
		t.Fatal("a STALE-CUT row must not count as a TRIGGERED row")
	}
}

// Under the horizon, or with the crash guard armed, nothing is forced and
// nothing is claimed.
func TestApplyProtectiveTickStaleCutWaits(t *testing.T) {
	trade := testutil.DeepLadderTrade(crashguard.DeRiskMinDepth, false)
	trade.Strategy.Params.SmartTakeLoss = true
	trade.CreatedAt = time.Unix(1_700_000_000, 0).UTC()
	due := trade.CreatedAt.Add(StaleAfter)

	early := ApplyProtectiveTick(trade, "stopLoss", 90, due.Add(-time.Hour), aggragates.AIIndicators{}, false)
	if early.Position != "stopLoss" || early.STLForced || early.Eval.StaleCut {
		t.Fatalf("under 21 days nothing may be forced, got %+v", early)
	}
	armed := ApplyProtectiveTick(trade, "stopLoss", 90, due, aggragates.AIIndicators{CrashActive: true}, true)
	if armed.Position != "stopLoss" || armed.STLForced || armed.Eval.StaleCut {
		t.Fatalf("a crash-active tick must not cut, got %+v", armed)
	}
}
