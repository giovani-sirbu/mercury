package actions

import (
	"github.com/giovani-sirbu/mercury/trades/gates/crashguard"
	"github.com/giovani-sirbu/mercury/trades/gates/regime"
	"github.com/giovani-sirbu/mercury/trades/gates/smarttakeloss"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func capShockAI(inverse bool) aggragates.AIIndicators {
	label15m := regime.ShockDown
	if inverse {
		label15m = regime.ShockUp
	}
	return aggragates.AIIndicators{
		HasRegimeVerdict: true,
		AddAllowed:       true,
		Regimes:          map[string]string{"4h": "mixed", "1h": "mixed", "15m": label15m},
	}
}

func capReclaimBucket(inverse bool) events.FiveMinOHLC {
	if inverse {
		return events.FiveMinOHLC{Open: 121, High: 122, Low: 110, Last: 114}
	}
	return events.FiveMinOHLC{Open: 80, High: 90, Low: 78, Last: 86}
}

func capFallingBucket() events.FiveMinOHLC {
	return events.FiveMinOHLC{Open: 90, High: 90, Low: 78, Last: 80}
}

func capTrade(inverse bool, fills int, lastFill, price float64) aggragates.Trades {
	side := "BUY"
	if inverse {
		side = "SELL"
	}
	trade := testutil.NewHoldTrade("stopLoss", inverse)
	// Capitulation is the crash guard's and can only bypass a regime hold:
	// both flags on is the configuration the episode machinery lives in.
	trade.Strategy.Params.RegimeHold = true
	trade.Strategy.Params.CrashGuard = true
	trade.PositionPrice = price
	trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{
		{Percentage: 2, Tolerance: 0.5, Depths: 8},
	}
	for i := 0; i < fills; i++ {
		fillPrice := lastFill
		if i < fills-1 {
			delta := float64(fills - 1 - i)
			if inverse {
				fillPrice = lastFill - delta
			} else {
				fillPrice = lastFill + delta
			}
		}
		trade.History = append(trade.History, aggragates.TradesHistory{
			Type: side, Quantity: 1, Price: fillPrice, OrderId: int64(i + 1),
		})
	}
	return trade
}

func capEvent(trade aggragates.Trades, ai aggragates.AIIndicators, bucket events.FiveMinOHLC) events.Events {
	return events.Events{
		Trade: trade,
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": testutil.NopUpdateTrade,
		},
		Params:      aggragates.Params{OldPosition: "active", AIIndicators: ai},
		FiveMinOHLC: bucket,
	}
}

func TestShouldHoldCapitulationLongOverrideOnReclaim(t *testing.T) {
	event := capEvent(capTrade(false, 3, 100, 79), capShockAI(false), capReclaimBucket(false))
	got, err := ShouldHold(event)
	if err != nil {
		t.Fatalf("8x dump + 5m upper-half reclaim must bypass the shock hold, got %v", err)
	}
	if !hasCapitulationPrefix(got.Trade.Logs, crashguard.CapitulationAllowedPrefix) {
		t.Fatalf("expected add-allowed log, got %#v", messages(got.Trade.Logs))
	}
}

func TestShouldHoldCapitulationLongNoOverrideWhileFalling(t *testing.T) {
	event := capEvent(capTrade(false, 3, 100, 79), capShockAI(false), capFallingBucket())
	_, err := ShouldHold(event)
	if err == nil {
		t.Fatal("8x dump with 5m close below midpoint must keep the shock hold")
	}
}

func TestShouldHoldCapitulationNoOverrideWhenCrashDeep(t *testing.T) {
	ai := capShockAI(false)
	ai.CrashActive = true
	event := capEvent(capTrade(false, crashguard.DeRiskMinDepth, 100, 79), ai, capReclaimBucket(false))
	event.Trade.Strategy.Params.CrashGuard = true
	held, err := ShouldHold(event)
	if err == nil {
		t.Fatal("crash-deep must not be overridden")
	}
	if !strings.Contains(held.Trade.Logs[len(held.Trade.Logs)-1].Message, "crash-guard: deep") {
		t.Errorf("expected crash-deep hold, got %q", held.Trade.Logs[len(held.Trade.Logs)-1].Message)
	}
}

func TestShouldHoldCapitulationNoOverrideOnSTLFreeze(t *testing.T) {
	event := stlFreezeEvent(false, stlFreezeAI(smarttakeloss.RiskThreshold, 0, regime.DownPersist))
	event.FiveMinOHLC = capReclaimBucket(false)
	last := event.Trade.History[len(event.Trade.History)-1].Price
	event.Trade.PositionPrice = last * 0.75
	held, err := ShouldHold(event)
	if err == nil {
		t.Fatal("STL HTF freeze must not be overridden")
	}
	if !strings.Contains(held.Trade.Logs[0].Message, "smart-take-loss: HTF continuation, no add") {
		t.Errorf("unexpected hold message %q", held.Trade.Logs[0].Message)
	}
}

func TestShouldHoldCapitulationInverseOverrideOnSqueeze(t *testing.T) {
	event := capEvent(capTrade(true, 3, 100, 121), capShockAI(true), capReclaimBucket(true))
	got, err := ShouldHold(event)
	if err != nil {
		t.Fatalf("8x squeeze + 5m lower-half reclaim must bypass the inverse hold, got %v", err)
	}
	if !hasCapitulationPrefix(got.Trade.Logs, crashguard.CapitulationAllowedPrefix) {
		t.Fatalf("expected add-allowed log, got %#v", messages(got.Trade.Logs))
	}
}

func TestShouldHoldCapitulationInverseDumpDoesNotOverride(t *testing.T) {
	ai := aggragates.AIIndicators{
		HasRegimeVerdict: true,
		AddAllowed:       true,
		Regimes:          map[string]string{"4h": regime.UpPersist, "1h": "mixed", "15m": "mixed"},
	}
	event := capEvent(capTrade(true, 3, 100, 80), ai, capReclaimBucket(true))
	held, err := ShouldHold(event)
	if err == nil {
		t.Fatal("dump below last sell must not fire the inverse capitulation override")
	}
	if !strings.Contains(held.Trade.Logs[len(held.Trade.Logs)-1].Message, "inverse add not allowed") {
		t.Errorf("expected inverse add veto, got %q", held.Trade.Logs[len(held.Trade.Logs)-1].Message)
	}
}

func TestShouldHoldCapitulationFreezeAfterOneFill(t *testing.T) {
	first := capEvent(capTrade(false, 3, 100, 79), capShockAI(false), capReclaimBucket(false))
	allowed, err := ShouldHold(first)
	if err != nil {
		t.Fatalf("first 8x reclaim add must pass, got %v", err)
	}

	secondTrade := capTrade(false, 4, 79, 78)
	secondTrade.Logs = allowed.Trade.Logs
	second := capEvent(secondTrade, capShockAI(false), capReclaimBucket(false))
	held, err := ShouldHold(second)
	if err == nil {
		t.Fatal("second stopLoss in the same episode must freeze")
	}
	if !strings.Contains(held.Trade.Logs[len(held.Trade.Logs)-1].Message, crashguard.CapitulationFreezeHold) {
		t.Errorf("expected freeze message, got %q", held.Trade.Logs[len(held.Trade.Logs)-1].Message)
	}
}

func hasCapitulationPrefix(logs []aggragates.TradesLogs, prefix string) bool {
	for _, entry := range logs {
		if strings.HasPrefix(entry.Message, prefix) {
			return true
		}
	}
	return false
}

func messages(logs []aggragates.TradesLogs) []string {
	out := make([]string, 0, len(logs))
	for _, entry := range logs {
		out = append(out, entry.Message)
	}
	return out
}
