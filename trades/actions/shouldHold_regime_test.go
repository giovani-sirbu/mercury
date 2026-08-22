package actions

import (
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func regimeHoldEvent(positionType string, inverse bool, ai aggragates.AIIndicators) events.Events {
	return events.Events{
		Trade: newHoldTrade(positionType, inverse),
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": nopUpdateTradeForHold,
		},
		Params: aggragates.Params{
			OldPosition:  "active",
			AIIndicators: ai,
		},
	}
}

func TestShouldHoldRegimeEntryBlockedWhenEnterNotAllowed(t *testing.T) {
	event := regimeHoldEvent("buy", false, aggragates.AIIndicators{
		UseAI:            true,
		HasRegimeVerdict: true,
		EnterAllowed:     false,
		Regimes:          map[string]string{"4h": RegimeDownPersist, "1h": "mixed", "15m": "flat"},
		Regime:           RegimeDownPersist,
	})
	event.Params.OldPosition = "new"

	held, err := ShouldHold(event)
	if err == nil {
		t.Fatal("expected entry hold when the regime refuses new capital")
	}
	if !strings.Contains(held.Trade.Logs[0].Message, "4h downtrend-persist") {
		t.Errorf("hold log must name the vetoing timeframe, got %q", held.Trade.Logs[0].Message)
	}
}

// With a verdict present the legacy bearish flag must stop mattering: the
// per-transition answer is the whole gate.
func TestShouldHoldRegimeVerdictSupersedesLegacyBearish(t *testing.T) {
	event := regimeHoldEvent("buy", false, aggragates.AIIndicators{
		UseAI:            true,
		HasRegimeVerdict: true,
		EnterAllowed:     true,
		AIMarketBearish:  true,
		Regimes:          map[string]string{"4h": "mixed", "1h": "mixed", "15m": "flat"},
	})
	event.Params.OldPosition = "new"

	if _, err := ShouldHold(event); err != nil {
		t.Fatalf("verdict EnterAllowed must supersede legacy bearish, got %v", err)
	}
}

func TestShouldHoldRegimeInverseEntryBlockedOnUptrend(t *testing.T) {
	event := regimeHoldEvent("buy", true, aggragates.AIIndicators{
		UseAI:            true,
		HasRegimeVerdict: true,
		EnterAllowed:     true, // long-only answer says yes; the mirror must say no
		Regimes:          map[string]string{"4h": RegimeUpPersist, "1h": "mixed", "15m": "flat"},
	})
	event.Params.OldPosition = "new"

	if _, err := ShouldHold(event); err == nil {
		t.Fatal("expected inverse entry hold while the market rises persistently")
	}
}

func TestShouldHoldRegimeStopLossBlockedWhenAddNotAllowed(t *testing.T) {
	event := regimeHoldEvent("stopLoss", false, aggragates.AIIndicators{
		UseAI:            true,
		HasRegimeVerdict: true,
		EnterAllowed:     false,
		AddAllowed:       false,
		Regimes:          map[string]string{"4h": "mixed", "1h": RegimeDownPersist, "15m": "flat"},
	})

	held, err := ShouldHold(event)
	if err == nil {
		t.Fatal("expected rebuy hold when the regime refuses an add")
	}
	if !strings.Contains(held.Trade.Logs[0].Message, "add not allowed") {
		t.Errorf("unexpected hold message %q", held.Trade.Logs[0].Message)
	}
}

// A profitable close is never gated by the regime, whatever the verdict says
// and whichever direction the trade runs. The old "keep trailing" deferral was
// measured against its absence and lost; this pins that it stays gone.
func TestShouldHoldRegimeNeverDefersProfitExit(t *testing.T) {
	verdicts := []aggragates.AIIndicators{
		{UseAI: true, HasRegimeVerdict: true, Regimes: map[string]string{"15m": RegimeUpPersist, "1h": RegimeUpPersist, "4h": RegimeUpPersist}},
		{UseAI: true, HasRegimeVerdict: true, Regimes: map[string]string{"15m": RegimeDownPersist, "1h": RegimeDownPersist, "4h": RegimeDownPersist}},
		{UseAI: true, HasRegimeVerdict: true, Regime: RegimeShock, ExitPreferred: true},
		{UseAI: true, HasRegimeVerdict: true, CrashActive: true, CrashScore: 90},
		{UseAI: true, HasRegimeVerdict: true, EnterAllowed: false, AddAllowed: false},
	}

	for _, inverse := range []bool{false, true} {
		for i, ai := range verdicts {
			event := regimeHoldEvent("takeProfit", inverse, ai)
			if _, err := ShouldHold(event); err != nil {
				t.Fatalf("verdict %d (inverse=%v) blocked a profitable close: %v", i, inverse, err)
			}
		}
	}
}

// Shock blocks every form of new capital but never a profitable close.
func TestShouldHoldRegimeShockBlocksCapitalNotProfitExit(t *testing.T) {
	shock := aggragates.AIIndicators{
		UseAI:            true,
		HasRegimeVerdict: true,
		EnterAllowed:     false,
		AddAllowed:       false,
		ExitPreferred:    true,
		Regime:           RegimeShock,
		Regimes:          map[string]string{"4h": "mixed", "1h": "mixed", "15m": RegimeShock},
	}

	entry := regimeHoldEvent("buy", false, shock)
	entry.Params.OldPosition = "new"
	if _, err := ShouldHold(entry); err == nil {
		t.Fatal("expected shock to block the entry")
	}

	rebuy := regimeHoldEvent("stopLoss", false, shock)
	if _, err := ShouldHold(rebuy); err == nil {
		t.Fatal("expected shock to block the rebuy")
	}

	profitExit := regimeHoldEvent("takeProfit", false, shock)
	if _, err := ShouldHold(profitExit); err != nil {
		t.Fatalf("shock must never block a profitable close, got %v", err)
	}
}


// The recalibrated shock policy: a 15m shock parks a rebuy only from
// ShockHoldMinDepth entries up. Shallow rungs trade straight through the
// spike — 26 of run 73's 29 shock holds had landed on 0-1 entry trades.
func TestShouldHoldRegimeShockHoldIsDepthAware(t *testing.T) {
	shock15m := aggragates.AIIndicators{
		UseAI:            true,
		HasRegimeVerdict: true,
		AddAllowed:       true,
		Regime:           RegimeShock,
		Regimes:          map[string]string{"4h": "mixed", "1h": "mixed", "15m": RegimeShock},
	}

	shallow := regimeHoldEvent("stopLoss", false, shock15m)
	if _, err := ShouldHold(shallow); err != nil {
		t.Fatalf("a shallow rebuy must trade through a 15m shock, got %v", err)
	}

	deep := regimeHoldEvent("stopLoss", false, shock15m)
	deep.Trade = deepTrade(false, 0)
	deep.Trade.Inverse = false
	// deepTrade builds BUY entries for non-inverse counting.
	deep.Trade.History = nil
	for i := 0; i < ShockHoldMinDepth; i++ {
		deep.Trade.History = append(deep.Trade.History, aggragates.TradesHistory{
			Type: "BUY", Quantity: 1, Price: 100 - float64(i), OrderId: int64(i + 1),
		})
	}
	deep.Trade.PositionType = "stopLoss"
	if _, err := ShouldHold(deep); err == nil {
		t.Fatal("a rebuy at ShockHoldMinDepth entries must park during a 15m shock")
	}
}

// A shock on a higher timeframe alone no longer parks rebuys — only the
// trigger timeframe's shock does, and only at depth.
func TestShouldHoldRegimeHigherTimeframeShockDoesNotParkRebuy(t *testing.T) {
	shock4h := aggragates.AIIndicators{
		UseAI:            true,
		HasRegimeVerdict: true,
		AddAllowed:       true,
		Regime:           RegimeShock,
		Regimes:          map[string]string{"4h": RegimeShock, "1h": "mixed", "15m": "mixed"},
	}

	event := regimeHoldEvent("stopLoss", false, shock4h)
	if _, err := ShouldHold(event); err != nil {
		t.Fatalf("a 4h shock alone must not park a shallow rebuy, got %v", err)
	}
}
