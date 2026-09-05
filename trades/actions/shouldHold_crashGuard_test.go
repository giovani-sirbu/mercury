package actions

import (
	"github.com/giovani-sirbu/mercury/trades/gates/crashguard"
	"github.com/giovani-sirbu/mercury/trades/gates/regime"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func TestShouldHoldCrashParksDeepRebuyAndDoesNotForceProfitExit(t *testing.T) {
	crash := aggragates.AIIndicators{
		HasRegimeVerdict: true,
		AddAllowed:       true, // regime alone would allow the add
		CrashActive:      true,
		CrashScore:       85,
	}

	rebuy := events.Events{
		Trade: testutil.DeepTrade(true),
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": testutil.NopUpdateTrade,
		},
		Params: aggragates.Params{OldPosition: "active", AIIndicators: crash},
	}
	if _, err := ShouldHold(rebuy); err == nil {
		t.Fatal("deep rebuy must be parked while the flush signal is armed")
	}

	profitExit := events.Events{
		Trade:  testutil.DeepTrade(true),
		Params: aggragates.Params{OldPosition: "active", AIIndicators: crash},
	}
	profitExit.Trade.PositionType = "takeProfit"
	if _, err := ShouldHold(profitExit); err != nil {
		t.Fatalf("crash must not hold takeProfit when nothing else does, got %v", err)
	}
}

func TestShouldHoldCrashParksAtDeRiskDepth(t *testing.T) {
	crash := aggragates.AIIndicators{
		HasRegimeVerdict: true,
		AddAllowed:       true,
		CrashActive:      true,
		CrashScore:       70,
	}
	event := events.Events{
		Trade: testutil.DeepTrade(true),
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": testutil.NopUpdateTrade,
		},
		Params: aggragates.Params{OldPosition: "active", AIIndicators: crash},
	}
	event.Trade.History = event.Trade.History[:crashguard.DeRiskMinDepth-1]
	if _, err := ShouldHold(event); err != nil {
		t.Fatalf("below de-risk depth crash must not park, got %v", err)
	}

	event.Trade = testutil.DeepTrade(true)
	if _, err := ShouldHold(event); err == nil {
		t.Fatal("at de-risk depth crash must park the add")
	}
}

func TestShouldHoldCrashStickyUntilFourHourReclaim(t *testing.T) {
	ai := aggragates.AIIndicators{
		HasRegimeVerdict: true,
		AddAllowed:       true,
		CrashActive:      false,
		Regimes:          map[string]string{"4h": regime.DownPersist, "1h": "mixed", "15m": "mixed"},
	}
	event := events.Events{
		Trade: testutil.DeepTrade(true),
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": testutil.NopUpdateTrade,
		},
		Params: aggragates.Params{OldPosition: "active", AIIndicators: ai},
	}
	event.Trade.Logs = []aggragates.TradesLogs{{
		Message: crashguard.TransitionMessage(aggragates.AIIndicators{CrashActive: true, CrashScore: 66}),
	}}

	held, err := ShouldHold(event)
	if err == nil {
		t.Fatal("sticky crash must park after CLEAR while 4h is still against")
	}
	if !strings.Contains(held.Trade.Logs[len(held.Trade.Logs)-1].Message, "sticky flush") {
		t.Errorf("expected sticky hold, got %q", held.Trade.Logs[len(held.Trade.Logs)-1].Message)
	}

	event.Params.AIIndicators.Regimes["4h"] = "mixed"
	if _, err := ShouldHold(event); err != nil {
		t.Fatalf("4h reclaim must release sticky crash, got %v", err)
	}
}
