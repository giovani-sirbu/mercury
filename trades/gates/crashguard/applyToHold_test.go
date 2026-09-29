package crashguard_test

import (
	"math"
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
	"github.com/giovani-sirbu/mercury/trades/gates/crashguard"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// transitionEvent is a ladder leaving oldPosition, anchored at anchor, on a
// tick at `tick`.
func transitionEvent(trade aggragates.Trades, oldPosition string, anchor, tick float64) events.Events {
	trade.PositionPrice = tick
	return events.Events{
		Trade:  trade,
		Params: aggragates.Params{OldPosition: oldPosition, OldPositionPrice: anchor},
	}
}

func freeFallVerdict() aggragates.AIIndicators {
	verdict := slowDecline()
	verdict.FreeFall = true
	return verdict
}

// ladderWithFills is testutil.DeepTrade with `fills` distinct entries, one
// exchange order each, the last of them at deepAnchor.
func ladderWithFills(fills int) aggragates.Trades {
	trade := testutil.DeepTrade(true)
	trade.History = nil
	for i := 0; i < fills; i++ {
		trade.History = append(trade.History, aggragates.TradesHistory{
			Type: "BUY", Quantity: 1, Price: deepAnchor + 2*float64(fills-1-i), OrderId: int64(i + 1),
		})
	}
	return trade
}

func settingsRow(percentage, tolerance float64) aggragates.StrategySettings {
	return aggragates.StrategySettings{Percentage: percentage, Tolerance: tolerance}
}

// ladderArmTick is where testutil.DeepTrade's single row arms the next depth
// on its own: one step, −(p + t), under the anchor — above the widened level.
func ladderArmTick() float64 {
	row := testutil.DeepTrade(true).StrategyPair.StrategySettings[0]
	return deepAnchor / (1 + (row.Percentage+row.Tolerance)/100)
}

// deepTradeWidenedLevel is where testutil.DeepTrade's held depth arms under a
// slow decline alone: SlowDeclineDepthFactor steps under the anchor.
func deepTradeWidenedLevel() float64 {
	row := testutil.DeepTrade(true).StrategyPair.StrategySettings[0]
	return widenedLevel(deepAnchor, row.Percentage, row.Tolerance)
}

// An inverse ladder is never held, whatever the verdict, the transition or
// the tick: the verdict reads a falling market, the side an inverse ladder is
// not trapped on. Whatever held before passes through untouched.
func TestApplyToHoldNeverHoldsAnInverseLadder(t *testing.T) {
	trade := testutil.DeepTrade(true)
	trade.Inverse = true
	for i := range trade.History {
		trade.History[i].Type = "SELL"
	}
	const earlier = "regime: inverse add not allowed (1h upPersist)"

	for _, verdict := range []aggragates.AIIndicators{slowDecline(), freeFallVerdict()} {
		for _, oldPosition := range []string{"buy", "stopLoss", "forceTrailingStopLoss"} {
			for _, tick := range []float64{deepAnchor * 1.3, deepAnchor * 1.02, deepAnchor * 0.7} {
				for _, incoming := range []string{"", earlier} {
					event := transitionEvent(trade, oldPosition, deepAnchor, tick)
					if got := crashguard.ApplyToHold(event, "stopLoss", verdict, incoming); got != incoming {
						t.Fatalf("inverse ladder from %s at %v (free fall %v): got %q, want the incoming %q",
							oldPosition, tick, verdict.FreeFall, got, incoming)
					}
				}
			}
		}
	}
}

// The hold starts at exactly DeRiskMinDepth filled entries, counted the way
// the ladder counts them: one per exchange order, bookkeeping rows left out.
// A ladder showing DeRiskMinDepth rows is therefore not always that deep.
func TestApplyToHoldStartsAtDeRiskMinDepth(t *testing.T) {
	for _, verdict := range []aggragates.AIIndicators{slowDecline(), freeFallVerdict()} {
		shallow := ladderWithFills(crashguard.DeRiskMinDepth - 1)
		if got := crashguard.ApplyToHold(armingEvent(shallow, ladderArmTick()), "stopLoss", verdict, ""); got != "" {
			t.Fatalf("free fall %v: one fill under the depth was held: %q", verdict.FreeFall, got)
		}

		deep := ladderWithFills(crashguard.DeRiskMinDepth)
		if got := crashguard.ApplyToHold(armingEvent(deep, ladderArmTick()), "stopLoss", verdict, ""); got == "" {
			t.Fatalf("free fall %v: a ladder exactly at the depth must be held", verdict.FreeFall)
		}

		sameOrder := ladderWithFills(crashguard.DeRiskMinDepth)
		sameOrder.History[3].OrderId = sameOrder.History[2].OrderId
		if got := crashguard.ApplyToHold(armingEvent(sameOrder, ladderArmTick()), "stopLoss", verdict, ""); got != "" {
			t.Fatalf("free fall %v: two rows of one exchange order counted as two depths: %q", verdict.FreeFall, got)
		}

		bookkeeping := ladderWithFills(crashguard.DeRiskMinDepth - 1)
		bookkeeping.History = append(bookkeeping.History, aggragates.TradesHistory{Type: "BUY", Quantity: 1, Price: 0, OrderId: 99})
		if got := crashguard.ApplyToHold(armingEvent(bookkeeping, ladderArmTick()), "stopLoss", verdict, ""); got != "" {
			t.Fatalf("free fall %v: a bookkeeping row counted as a depth: %q", verdict.FreeFall, got)
		}
	}
}

// Without free fall the held depth arms at exactly the widened level,
// anchor / (1 + (SlowDeclineDepthFactor·p + t)/100): the anchor is the
// transition's OldPositionPrice, p and t the row the ladder binds for the
// held depth — the last filled entry's own row, the base row when the ladder
// has none that deep. The level itself releases, the next representable
// price above it holds, and the reason names the level.
func TestApplyToHoldReleasesExactlyAtTheWidenedLevel(t *testing.T) {
	cases := []struct {
		name     string
		fills    int
		settings []aggragates.StrategySettings
		row      int
		anchor   float64
	}{
		{"one row applies to every depth", 4,
			[]aggragates.StrategySettings{settingsRow(2, 0.5)}, 0, deepAnchor},
		{"the last filled entry's own row", 4,
			[]aggragates.StrategySettings{settingsRow(1, 0.1), settingsRow(1.5, 0.2), settingsRow(2, 0.3), settingsRow(3, 0.5), settingsRow(4, 0.6), settingsRow(5, 0.7)}, 3, deepAnchor},
		{"the base row when no row is that deep", 5,
			[]aggragates.StrategySettings{settingsRow(2.5, 0.25), settingsRow(9, 0.9)}, 0, deepAnchor},
		{"the anchor is the re-anchored price, not the last fill", 4,
			[]aggragates.StrategySettings{settingsRow(2, 0.5)}, 0, 92.5},
	}
	for _, c := range cases {
		trade := ladderWithFills(c.fills)
		trade.StrategyPair.StrategySettings = c.settings
		trade.StrategyPair.TradeFilters.PriceFilter = 2
		row := c.settings[c.row]
		level := c.anchor / (1 + (crashguard.SlowDeclineDepthFactor*row.Percentage+row.Tolerance)/100)
		want := crashguard.SlowDeclineHoldPrefix + ", next depth parked until " + gates.FormatPriceLevel(trade, level)

		above := transitionEvent(trade, "buy", c.anchor, math.Nextafter(level, math.Inf(1)))
		if got := crashguard.ApplyToHold(above, "stopLoss", slowDecline(), ""); got != want {
			t.Errorf("%s: just above the level got %q, want %q", c.name, got, want)
		}
		for _, tick := range []float64{level, math.Nextafter(level, 0), level * 0.95} {
			if got := crashguard.ApplyToHold(transitionEvent(trade, "buy", c.anchor, tick), "stopLoss", slowDecline(), ""); got != "" {
				t.Errorf("%s: at or under the level %v the tick %v must release, got %q", c.name, level, tick, got)
			}
		}
	}
}

// The parked reason is the hold log's dedup key: pin one exactly, the level
// written on the pair's price filter.
func TestApplyToHoldParkedReasonText(t *testing.T) {
	trade := testutil.DeepTrade(true)
	trade.StrategyPair.TradeFilters.PriceFilter = 2
	const want = "crash-guard: slow decline, next depth parked until 86.64"
	if got := crashguard.ApplyToHold(armingEvent(trade, 90), "stopLoss", slowDecline(), ""); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// A re-anchor of a depth already armed — trailing or force-trailing — is not
// the arming, so a slow decline alone lets it move however far above the
// widened level it sits. Under free fall every stopLoss transition holds,
// the arming included at or under the widened level: there is no level to
// buy at when nothing to the left supports the price.
func TestApplyToHoldReAnchorHeldOnlyUnderFreeFall(t *testing.T) {
	trade := testutil.DeepTrade(true)
	for _, oldPosition := range []string{"stopLoss", "forceTrailingStopLoss"} {
		for _, tick := range []float64{93, 86, 70} {
			event := transitionEvent(trade, oldPosition, deepAnchor, tick)
			if got := crashguard.ApplyToHold(event, "stopLoss", slowDecline(), ""); got != "" {
				t.Errorf("re-anchor from %s at %v held on a slow decline alone: %q", oldPosition, tick, got)
			}
			if got := crashguard.ApplyToHold(event, "stopLoss", freeFallVerdict(), ""); got != crashguard.FreeFallHoldReason {
				t.Errorf("re-anchor from %s at %v under free fall: got %q, want %q", oldPosition, tick, got, crashguard.FreeFallHoldReason)
			}
		}
	}

	level := deepTradeWidenedLevel()
	for _, tick := range []float64{level, level * 0.9} {
		if got := crashguard.ApplyToHold(armingEvent(trade, tick), "stopLoss", freeFallVerdict(), ""); got != crashguard.FreeFallHoldReason {
			t.Errorf("arming at %v under free fall: got %q, want %q", tick, got, crashguard.FreeFallHoldReason)
		}
	}
}

// Free fall needs no level: a settings row the level cannot be priced from
// fails the slow-decline hold open, and free fall still holds.
func TestApplyToHoldFreeFallHoldsWithoutAPricedLevel(t *testing.T) {
	unpriceable := []aggragates.StrategySettings{
		settingsRow(0, 0.5),
		settingsRow(-1, 0.5),
		settingsRow(1, -crashguard.SlowDeclineDepthFactor-1), // a tolerance that cancels the whole step
	}
	for _, row := range unpriceable {
		trade := testutil.DeepTrade(true)
		trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{row}
		if got := crashguard.ApplyToHold(armingEvent(trade, 93), "stopLoss", slowDecline(), ""); got != "" {
			t.Errorf("row %+v: an unpriceable level must fail open, got %q", row, got)
		}
		if got := crashguard.ApplyToHold(armingEvent(trade, 93), "stopLoss", freeFallVerdict(), ""); got != crashguard.FreeFallHoldReason {
			t.Errorf("row %+v: free fall must hold without a level, got %q", row, got)
		}
	}
}

// The guard holds capital, never an exit: no transition but stopLoss is
// held, under any verdict, and whatever held before passes through.
func TestApplyToHoldHoldsOnlyStopLossTransitions(t *testing.T) {
	trade := testutil.DeepTrade(true)
	const earlier = "regime: profit hold"
	for _, position := range []string{"takeProfit", "buy", "sell", "sellLoss", "impasse"} {
		for _, oldPosition := range []string{"buy", "stopLoss", "takeProfit"} {
			for _, incoming := range []string{"", earlier} {
				event := transitionEvent(trade, oldPosition, deepAnchor, 105)
				if got := crashguard.ApplyToHold(event, position, freeFallVerdict(), incoming); got != incoming {
					t.Errorf("%s from %s: got %q, want the incoming %q", position, oldPosition, got, incoming)
				}
			}
		}
	}
}

// A depth released by its price hands back whatever held before the crash
// guard was asked: the release is the guard stepping aside, not a pass past
// the families before it. While it holds, its reason replaces theirs.
func TestApplyToHoldReleaseKeepsTheIncomingHold(t *testing.T) {
	trade := testutil.DeepTrade(true)
	const earlier = "regime: add not allowed (4h downPersist)"

	if got := crashguard.ApplyToHold(armingEvent(trade, deepTradeWidenedLevel()), "stopLoss", slowDecline(), earlier); got != earlier {
		t.Fatalf("a depth released at its level dropped the incoming hold: got %q", got)
	}
	if got := crashguard.ApplyToHold(armingEvent(trade, ladderArmTick()), "stopLoss", slowDecline(), earlier); !strings.HasPrefix(got, crashguard.SlowDeclineHoldPrefix+", next depth parked until") {
		t.Fatalf("a held depth must carry the crash guard's reason, got %q", got)
	}
}
