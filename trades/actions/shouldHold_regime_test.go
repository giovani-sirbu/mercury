package actions

import (
	"github.com/giovani-sirbu/mercury/trades/gates/regime"
	"github.com/giovani-sirbu/mercury/trades/gates/smarttakeloss"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func regimeHoldEvent(positionType string, inverse bool, ai aggragates.AIIndicators) events.Events {
	trade := testutil.NewHoldTrade(positionType, inverse)
	trade.Strategy.Params.RegimeHold = true
	return events.Events{
		Trade: trade,
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": testutil.NopUpdateTrade,
		},
		Params: aggragates.Params{
			OldPosition:  "active",
			AIIndicators: ai,
		},
	}
}

// The regime lens never touches the first fill: a 4h downtrend, a refused
// verdict, a shock — none of it holds a new trade. The first fill is the
// cooldown's (the 4h entry veto measured one right call in four on run 97).
func TestShouldHoldRegimeHoldNeverVetoesEntry(t *testing.T) {
	for _, inverse := range []bool{false, true} {
		for _, label := range []string{regime.DownPersist, regime.UpPersist, regime.Shock, regime.ShockDown, regime.ShockUp} {
			event := regimeHoldEvent("buy", inverse, aggragates.AIIndicators{
				HasRegimeVerdict: true,
				EnterAllowed:     false,
				AddAllowed:       false,
				Regimes:          map[string]string{"4h": label, "1h": label, "15m": label},
				Regime:           label,
			})
			event.Params.OldPosition = "new"

			held, err := ShouldHold(event)
			if err != nil {
				t.Fatalf("inverse=%v %s: the regime lens must not veto a first fill, got %v", inverse, label, err)
			}
			if len(held.Trade.Logs) != 0 {
				t.Fatalf("inverse=%v %s: no row may be written, got %v", inverse, label, held.Trade.Logs)
			}
		}
	}
}

// The legacy bearish/bullish read belongs to UseAI: with RegimeHold on and
// UseAI off it does nothing; with UseAI on it is the entry veto.
func TestShouldHoldLegacyBearishIsOwnedByUseAI(t *testing.T) {
	ai := aggragates.AIIndicators{
		HasRegimeVerdict: true,
		EnterAllowed:     true,
		AIMarketBearish:  true,
		Regimes:          map[string]string{"4h": "mixed", "1h": "mixed", "15m": "flat"},
	}
	event := regimeHoldEvent("buy", false, ai)
	event.Params.OldPosition = "new"
	if _, err := ShouldHold(event); err != nil {
		t.Fatalf("RegimeHold alone must not read the legacy bearish flag, got %v", err)
	}

	event = regimeHoldEvent("buy", false, ai)
	event.Trade.Strategy.Params.UseAI = true
	event.Params.OldPosition = "new"
	held, err := ShouldHold(event)
	if err == nil {
		t.Fatal("UseAI must veto the entry on the legacy bearish flag")
	}
	if held.Trade.Logs[0].Message != "Hold entry: AI market is bearish" {
		t.Errorf("unexpected hold message %q", held.Trade.Logs[0].Message)
	}
}

func TestShouldHoldRegimeStopLossBlockedWhenAddNotAllowed(t *testing.T) {
	event := regimeHoldEvent("stopLoss", false, aggragates.AIIndicators{
		HasRegimeVerdict: true,
		EnterAllowed:     false,
		AddAllowed:       false,
		Regimes:          map[string]string{"4h": "mixed", "1h": regime.DownPersist, "15m": "flat"},
	})

	held, err := ShouldHold(event)
	if err == nil {
		t.Fatal("expected rebuy hold when the regime refuses an add")
	}
	if !strings.Contains(held.Trade.Logs[0].Message, "add not allowed") {
		t.Errorf("unexpected hold message %q", held.Trade.Logs[0].Message)
	}
}

// A profitable close still sells when 15m is not a favorable persist, and
// crash neither clears a trend hold nor adds a take-profit hold of its own.
//
// The regime DOES defer a profitable close in one case — regime.HoldReason
// dispatches takeProfit to profitHoldReason — so the old name here
// ("NeverDefersProfitExit") contradicted the code it guards. The deferral
// itself is pinned further down this file by the profit-hold cases; what this
// one pins is the release: none of the three verdicts below earns it.
func TestShouldHoldRegimeReleasesProfitExitWithoutAFavorablePersist(t *testing.T) {
	verdicts := []aggragates.AIIndicators{
		{HasRegimeVerdict: true, Regime: regime.Shock},
		{HasRegimeVerdict: true, CrashActive: true, CrashScore: 90, Regimes: map[string]string{"15m": "mixed", "4h": "mixed"}},
		{HasRegimeVerdict: true, EnterAllowed: false, AddAllowed: false},
	}

	for _, inverse := range []bool{false, true} {
		for i, ai := range verdicts {
			event := regimeHoldEvent("takeProfit", inverse, ai)
			if ai.CrashActive {
				event.Trade.Strategy.Params.CrashGuard = true
			}
			if _, err := ShouldHold(event); err != nil {
				t.Fatalf("verdict %d (inverse=%v) blocked a profitable close: %v", i, inverse, err)
			}
		}
	}
}

// A 15m shock parks a deep-enough rebuy (via AddAllowed=false in this
// fixture) but never the first buy and never a profitable close.
func TestShouldHoldRegimeShockBlocksCapitalNotProfitExit(t *testing.T) {
	shock := aggragates.AIIndicators{
		HasRegimeVerdict: true,
		EnterAllowed:     false,
		AddAllowed:       false,
		Regime:           regime.Shock,
		Regimes:          map[string]string{"4h": "mixed", "1h": "mixed", "15m": regime.Shock},
	}

	entry := regimeHoldEvent("buy", false, shock)
	entry.Params.OldPosition = "new"
	if _, err := ShouldHold(entry); err != nil {
		t.Fatalf("a 15m shock must not block the first buy, got %v", err)
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
		HasRegimeVerdict: true,
		AddAllowed:       true,
		Regime:           regime.Shock,
		Regimes:          map[string]string{"4h": "mixed", "1h": "mixed", "15m": regime.Shock},
	}

	shallow := regimeHoldEvent("stopLoss", false, shock15m)
	if _, err := ShouldHold(shallow); err != nil {
		t.Fatalf("a shallow rebuy must trade through a 15m shock, got %v", err)
	}

	deep := regimeHoldEvent("stopLoss", false, shock15m)
	deep.Trade = testutil.DeepTrade(false)
	deep.Trade.Strategy.Params.RegimeHold = true
	deep.Trade.Inverse = false
	// deepTrade builds BUY entries for non-inverse counting.
	deep.Trade.History = nil
	for i := 0; i < regime.ShockHoldMinDepth; i++ {
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
		HasRegimeVerdict: true,
		AddAllowed:       true,
		Regime:           regime.Shock,
		Regimes:          map[string]string{"4h": regime.Shock, "1h": "mixed", "15m": "mixed"},
	}

	event := regimeHoldEvent("stopLoss", false, shock4h)
	if _, err := ShouldHold(event); err != nil {
		t.Fatalf("a 4h shock alone must not park a shallow rebuy, got %v", err)
	}
}

// Directional shock on deep rebuys: the veto follows the direction the
// capital would buy into. A falling shock blocks the long side and frees the
// inverse side; a rising shock the reverse; the directionless legacy "shock"
// blocks both.
func TestShouldHoldShockDirectionOnDeepRebuys(t *testing.T) {
	deepFor := func(inverse bool) events.Events {
		side := "BUY"
		if inverse {
			side = "SELL"
		}
		trade := testutil.NewHoldTrade("stopLoss", inverse)
		trade.Strategy.Params.RegimeHold = true
		for i := 0; i < regime.ShockHoldMinDepth; i++ {
			trade.History = append(trade.History, aggragates.TradesHistory{
				Type: side, Quantity: 1, Price: 100, OrderId: int64(i + 1),
			})
		}
		return events.Events{
			Trade: trade,
			Events: map[string]func(events.Events) (events.Events, error){
				"updateTrade": testutil.NopUpdateTrade,
			},
			Params: aggragates.Params{OldPosition: "active"},
		}
	}

	cases := []struct {
		label       string
		inverse     bool
		wantBlocked bool
	}{
		{regime.ShockDown, false, true}, // long adding into the falling knife: parks
		{regime.ShockUp, false, false},  // long rebuy during a rising shock: passes
		{regime.ShockUp, true, true},    // inverse adding into a rising knife: parks
		{regime.ShockDown, true, false}, // inverse during a fall: its harvest, passes
		{regime.Shock, false, true},     // legacy directionless: parks both
	}

	for _, c := range cases {
		event := deepFor(c.inverse)
		event.Params.AIIndicators = aggragates.AIIndicators{
			HasRegimeVerdict: true,
			AddAllowed:       true,
			Regimes:          map[string]string{"4h": "mixed", "1h": "mixed", "15m": c.label},
		}
		_, err := ShouldHold(event)
		if c.wantBlocked && err == nil {
			t.Fatalf("%s inverse=%v: expected the deep rebuy parked", c.label, c.inverse)
		}
		if !c.wantBlocked && err != nil {
			t.Fatalf("%s inverse=%v: rebuy must pass, got %v", c.label, c.inverse, err)
		}
	}
}

// Profit-exit deferral from ProfitHoldMinDepth up while 15m still moves in
// the trade's favor. A non-persist 15m label still releases the close, and
// so does a shallow ladder: below four fills the deferral measured as a
// negative coin flip that tied capital (runs 90/94). Crash does not release.
func TestShouldHoldDeepProfitExitRidesTheTrend(t *testing.T) {
	profitEvent := func(inverse bool, fills int, depths float64, ai aggragates.AIIndicators) events.Events {
		side := "BUY"
		if inverse {
			side = "SELL"
		}
		trade := testutil.NewHoldTrade("takeProfit", inverse)
		trade.Strategy.Params.RegimeHold = true
		trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{{Depths: depths, Percentage: 2}}
		for i := 0; i < fills; i++ {
			trade.History = append(trade.History, aggragates.TradesHistory{
				Type: side, Quantity: 1, Price: 100 - float64(i), OrderId: int64(i + 1),
			})
		}
		return events.Events{
			Trade: trade,
			Events: map[string]func(events.Events) (events.Events, error){
				"updateTrade": testutil.NopUpdateTrade,
			},
			Params: aggragates.Params{OldPosition: "active", AIIndicators: ai},
		}
	}
	verdict := func(label15m string, crash bool) aggragates.AIIndicators {
		return aggragates.AIIndicators{
			HasRegimeVerdict: true,
			CrashActive:      crash,
			Regimes:          map[string]string{"4h": "mixed", "1h": "mixed", "15m": label15m},
		}
	}

	cases := []struct {
		name     string
		inverse  bool
		fills    int
		depths   float64
		label15m string
		crash    bool
		wantHold bool
	}{
		{"long rides uptrend at depth 5", false, 5, 8, regime.UpPersist, false, true},
		{"long rides uptrend at the depth floor", false, regime.ProfitHoldMinDepth, 8, regime.UpPersist, false, true},
		{"long rides uptrend at last rung", false, 8, 8, regime.UpPersist, false, true},
		{"shallow long sells into an uptrend", false, 1, 8, regime.UpPersist, false, false},
		{"just under the floor sells", false, regime.ProfitHoldMinDepth - 1, 8, regime.UpPersist, false, false},
		{"inverse rides downtrend", true, 5, 8, regime.DownPersist, false, true},
		{"inverse never rides an uptrend", true, 5, 8, regime.UpPersist, false, false},
		{"long never rides a downtrend", false, 5, 8, regime.DownPersist, false, false},
		{"shock-up releases the close", false, 5, 8, regime.ShockUp, false, false},
		{"crash guard does not release a trend hold", false, 5, 8, regime.UpPersist, true, true},
		{"tiny ladder at its floor still rides", false, regime.ProfitHoldMinDepth, 4, regime.UpPersist, false, true},
	}

	for _, c := range cases {
		event := profitEvent(c.inverse, c.fills, c.depths, verdict(c.label15m, c.crash))
		if c.crash {
			event.Trade.Strategy.Params.CrashGuard = true
		}
		held, err := ShouldHold(event)
		if c.wantHold && err == nil {
			t.Fatalf("%s: expected the profit exit deferred", c.name)
		}
		if !c.wantHold && err != nil {
			t.Fatalf("%s: profit exit must execute, got %v", c.name, err)
		}
		if c.wantHold && !strings.Contains(held.Trade.Logs[0].Message, "rides the trend") {
			t.Errorf("%s: unexpected hold message %q", c.name, held.Trade.Logs[0].Message)
		}
	}

	blocked := profitEvent(false, 5, 8, verdict(regime.UpPersist, false))
	blocked.Trade.Status = aggragates.Blocked
	if _, err := ShouldHold(blocked); err == nil {
		t.Fatal("a blocked trade still rides a favorable 15m")
	}

	exhausted := profitEvent(false, 7, 8, verdict(regime.UpPersist, false))
	if _, err := ShouldHold(exhausted); err == nil {
		t.Fatal("the last rungs still ride a favorable 15m")
	}

	// Another trade of the wallet is funds-blocked: the close is the capital
	// the ladder waits for, so the deferral stands down.
	starved := profitEvent(false, 5, 8, verdict(regime.UpPersist, false))
	starved.Params.PortfolioBlocked = true
	if _, err := ShouldHold(starved); err != nil {
		t.Fatalf("a blocked portfolio must release the profit exit, got %v", err)
	}
}

// The inverse ADD veto reads labels, and a violent rally labels as shock-up,
// not uptrend-persist — without shockBlocks in the veto loop a vertical
// squeeze slipped exactly the veto that saved run 74's rally blow-ups. The
// veto follows shock direction at ANY depth (this is the uptrend veto, not
// the depth-aware shock rule): shock-up parks the add, shock-down is the
// inverse trade's harvest and passes.
func TestShouldHoldInverseAddVetoCatchesShockShadowedUptrend(t *testing.T) {
	cases := []struct {
		timeframe   string
		label       string
		wantBlocked bool
	}{
		{"4h", regime.ShockUp, true},
		{"1h", regime.ShockUp, true},
		{"4h", regime.ShockDown, false},
		{"4h", regime.Shock, true}, // legacy directionless: conservative, parks
	}

	for _, c := range cases {
		regimes := map[string]string{"4h": "mixed", "1h": "mixed", "15m": "mixed"}
		regimes[c.timeframe] = c.label

		event := regimeHoldEvent("stopLoss", true, aggragates.AIIndicators{
			HasRegimeVerdict: true,
			AddAllowed:       true,
			Regimes:          regimes,
		})
		_, err := ShouldHold(event)
		if c.wantBlocked && err == nil {
			t.Fatalf("%s %s: expected the shallow inverse add vetoed", c.timeframe, c.label)
		}
		if !c.wantBlocked && err != nil {
			t.Fatalf("%s %s: inverse add must pass, got %v", c.timeframe, c.label, err)
		}
	}
}

func profitHoldDeepEvent(fills int, depths float64, ai aggragates.AIIndicators) events.Events {
	trade := testutil.NewHoldTrade("takeProfit", false)
	trade.Strategy.Params.RegimeHold = true
	trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{{Depths: depths, Percentage: 2}}
	for i := 0; i < fills; i++ {
		trade.History = append(trade.History, aggragates.TradesHistory{
			Type: "BUY", Quantity: 1, Price: 100 - float64(i), OrderId: int64(i + 1),
		})
	}
	return events.Events{
		Trade: trade,
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": testutil.NopUpdateTrade,
		},
		Params: aggragates.Params{OldPosition: "active", AIIndicators: ai},
	}
}

// C.4: 15m up vs 4h down is disagreement — a deep takeProfit must execute,
// not ride. 4h still agreeing up keeps the existing deferral.
func TestShouldHoldProfitHoldRequiresFourHourAgreement(t *testing.T) {
	ai := func(label15m, label4h string) aggragates.AIIndicators {
		return aggragates.AIIndicators{
			HasRegimeVerdict: true,
			Regimes:          map[string]string{"4h": label4h, "1h": "mixed", "15m": label15m},
		}
	}

	disagreement := profitHoldDeepEvent(5, 8, ai(regime.UpPersist, regime.DownPersist))
	if _, err := ShouldHold(disagreement); err != nil {
		t.Fatalf("4h downtrend + 15m up must not defer takeProfit, got %v", err)
	}

	shockDown := profitHoldDeepEvent(5, 8, ai(regime.UpPersist, regime.ShockDown))
	if _, err := ShouldHold(shockDown); err != nil {
		t.Fatalf("4h shock-down + 15m up must not defer takeProfit, got %v", err)
	}

	agreement := profitHoldDeepEvent(5, 8, ai(regime.UpPersist, regime.UpPersist))
	held, err := ShouldHold(agreement)
	if err == nil {
		t.Fatal("4h uptrend-persist + 15m up + deep must still defer")
	}
	if !strings.Contains(held.Trade.Logs[0].Message, "rides the trend") {
		t.Errorf("unexpected hold message %q", held.Trade.Logs[0].Message)
	}
}

func stlFreezeAI(risk, reversal float64, label4h string) aggragates.AIIndicators {
	return aggragates.AIIndicators{
		HasRegimeVerdict:     true,
		AddAllowed:           true,
		DownContinuationRisk: risk,
		ReversalUpEvidence:   reversal,
		Regimes:              map[string]string{"4h": label4h, "1h": "mixed", "15m": "mixed"},
	}
}

func stlFreezeEvent(inverse bool, ai aggragates.AIIndicators) events.Events {
	trade := testutil.DeepLadderTrade(smarttakeloss.ArmDepth(9), inverse)
	trade.PositionType = "stopLoss"
	// The freeze lives next to the regime and crash gates in a real
	// strategy; both on so the precedence tests exercise the real order.
	trade.Strategy.Params.RegimeHold = true
	trade.Strategy.Params.CrashGuard = true
	return events.Events{
		Trade: trade,
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": testutil.NopUpdateTrade,
		},
		Params: aggragates.Params{OldPosition: "active", AIIndicators: ai},
	}
}

// Selective HTF freeze: only an armed long with continuation HIGH, weak
// reversal, and 4h agreeing down parks the add. Not every ARM, not inverse.
func TestShouldHoldSmartTakeLossHTFFreeze(t *testing.T) {
	held, err := ShouldHold(stlFreezeEvent(false, stlFreezeAI(smarttakeloss.RiskThreshold, 0, regime.DownPersist)))
	if err == nil {
		t.Fatal("armed long + risk 70+ + 4h down + low reversal must freeze the add")
	}
	if !strings.Contains(held.Trade.Logs[0].Message, "smart-take-loss: HTF continuation, no add") {
		t.Errorf("unexpected freeze message %q", held.Trade.Logs[0].Message)
	}

	if _, err := ShouldHold(stlFreezeEvent(false, stlFreezeAI(smarttakeloss.RiskThreshold, smarttakeloss.MinReversalEvidence, regime.DownPersist))); err != nil {
		t.Fatalf("reversal >= 60 must not freeze, got %v", err)
	}

	if _, err := ShouldHold(stlFreezeEvent(true, stlFreezeAI(smarttakeloss.RiskThreshold, 0, regime.DownPersist))); err != nil {
		t.Fatalf("inverse + dump must not freeze adds, got %v", err)
	}
}

// The regime gates answer to RegimeHold and to nothing else: a verdict
// fetched for UseAI (or the crash guard) gates no add without the flag.
func TestShouldHoldRegimeGatesRequireRegimeHold(t *testing.T) {
	ai := aggragates.AIIndicators{
		HasRegimeVerdict: true,
		AIAction:         aggragates.ActionLong,
		AIMarketBullish:  true,
		AddAllowed:       false,
		Regimes:          map[string]string{"4h": regime.DownPersist, "1h": "mixed", "15m": "flat"},
	}
	event := regimeHoldEvent("stopLoss", false, ai)
	event.Trade.Strategy.Params.RegimeHold = false
	event.Trade.Strategy.Params.UseAI = true
	if held, err := ShouldHold(event); err != nil {
		t.Fatalf("without RegimeHold a refused add must pass, got %v (%v)", err, held.Trade.Logs)
	}

	event = regimeHoldEvent("stopLoss", false, ai)
	event.Trade.Strategy.Params.UseAI = true
	held, err := ShouldHold(event)
	if err == nil {
		t.Fatal("with RegimeHold the refused add must hold")
	}
	if !strings.Contains(held.Trade.Logs[0].Message, "regime: add not allowed (4h downtrend-persist)") {
		t.Errorf("unexpected hold message %q", held.Trade.Logs[0].Message)
	}
}

// RegimeHold and UseAI are independent: the regime add veto fires even
// when the ML route says LONG.
func TestShouldHoldRegimeAddVetoIndependentOfAIVerdict(t *testing.T) {
	event := regimeHoldEvent("stopLoss", false, aggragates.AIIndicators{
		HasRegimeVerdict: true,
		AIAction:         aggragates.ActionLong,
		AIMarketBullish:  true,
		AddAllowed:       false,
		Regimes:          map[string]string{"4h": regime.DownPersist, "1h": "mixed", "15m": "flat"},
	})
	event.Trade.Strategy.Params.UseAI = true

	if _, err := ShouldHold(event); err == nil {
		t.Fatal("the regime add veto must still fire when ML says LONG")
	}
}
