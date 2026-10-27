package smarttakeloss

import (
	"testing"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// A committed ladder's sale is named by the first live trigger whose gate activated the stopLoss
// (committedReason), else by the first live one, the activated bounded trigger a bound refused
// being no longer live.

// A sale from the trail is named by the trigger that activated it, whichever stands first.
func TestTheTrailsSaleIsNamedByTheTriggerThatActivated(t *testing.T) {
	withSellLossIgnoreLastDepths(t, 0)
	trade := flatMarketLadder(false)
	price := ladder.AverageEntryPrice(trade) * lossFactor
	slow, low := slowTrigger(true), lowTrigger(true)
	freeFall := saleTrigger{reason: reasonFreeFallPrevention, gate: GateFreeFallPrevention, rowDue: true}
	capital := saleTrigger{reason: reasonCapitalProtection, gate: GateCapitalProtection, rowDue: true, terms: capitalProtectionTerms}

	for _, c := range []struct {
		name     string
		triggers []saleTrigger
		reason   string
	}{
		{"the free fall prevention first, capital protection activated", []saleTrigger{freeFall, activatedOf(capital)}, reasonCapitalProtection},
		{"capital protection first, the free fall prevention activated", []saleTrigger{capital, activatedOf(freeFall)}, reasonFreeFallPrevention},
		{"the slow decline first, the flat market activated", []saleTrigger{slow, activatedOf(low)}, lowReason},
		{"the flat market first, the slow decline activated", []saleTrigger{low, activatedOf(slow)}, slowReason},
		{"the first of two activated", []saleTrigger{activatedOf(freeFall), activatedOf(capital)}, reasonFreeFallPrevention},
		{"the second of two activated named for the first", []saleTrigger{activatedOf(capital), activatedOf(freeFall)}, reasonCapitalProtection},
		{"the third activated of three", []saleTrigger{slow, low, activatedOf(capital)}, reasonCapitalProtection},
	} {
		var cases []sellLossCase
		for _, state := range []string{"stopLoss", "forceTrailingStopLoss"} {
			sale := onLadder(c.name+", "+state, anchoredToSell(trade, price), state, price, c.triggers...)
			sale.step, sale.reason = stepSell, c.reason
			cases = append(cases, sale)
		}
		runSellLoss(t, cases)
	}

	// An activated trigger a depth bound refuses is not live: the sale is named by the live one
	// that is first, though the refused trigger committed it.
	withSellLossIgnoreLastDepths(t, ladder.ConfiguredDepths(trade))
	var cases []sellLossCase
	for _, c := range []struct {
		name     string
		triggers []saleTrigger
		reason   string
	}{
		{"an activated bounded trigger refused, capital protection live", []saleTrigger{activatedOf(slow), capital}, reasonCapitalProtection},
		{"an activated bounded trigger refused after the live one", []saleTrigger{capital, activatedOf(slow)}, reasonCapitalProtection},
		{"two bounded refused, capital protection activated", []saleTrigger{activatedOf(slow), low, activatedOf(capital)}, reasonCapitalProtection},
	} {
		sale := onLadder(c.name, anchoredToSell(trade, price), "stopLoss", price, c.triggers...)
		sale.step, sale.reason = stepSell, c.reason
		cases = append(cases, sale)
	}
	runSellLoss(t, cases)

	for _, c := range []struct {
		name string
		live []saleTrigger
		want string
	}{
		{"the first activated", []saleTrigger{freeFall, activatedOf(capital), activatedOf(low)}, reasonCapitalProtection},
		{"none activated, the first live", []saleTrigger{freeFall, capital}, reasonFreeFallPrevention},
		{"the only one", []saleTrigger{capital}, reasonCapitalProtection},
	} {
		if got := committedReason(c.live); got != c.want {
			t.Errorf("committedReason, %s: %q, want %q", c.name, got, c.want)
		}
	}
}

// A free fall prevention episode open beside a capital protection activation, the ladder at its
// last depth: the tick that activates capital protection's stopLoss leaves the free fall
// prevention's held, and the trail's sale is capital protection's, filed under its gate and
// acted on for it (CapitalProtectionActs), not "free fall prevention" for being first in the order
// the triggers are handed in. Where the free fall prevention activated alone the sale is its own.
func TestAFreeFallEpisodeBesideACapitalProtectionActivationSellsForCapitalProtection(t *testing.T) {
	capitalProtectionAlone(t)
	withFreeFallPrevention(t, true)
	withSellLossIgnoreLastDepths(t, 0)
	trade := capitalProtectionLadder(false)
	block := freeFallBlockOf(aggragates.SideLong, true, false, false)
	block.CapitalProtectionSmcBearish = true
	ai := withBlock(block)
	const price = 99.50

	for _, c := range []struct {
		name         string
		rule         float64
		drawdown     float64
		reason, gate string
		capital      bool
	}{
		{"capital protection reached, the free fall prevention short", 0.7, 1.0, reasonCapitalProtection, GateCapitalProtection, true},
		{"the free fall prevention reached, capital protection short", -0.6, 0.3, reasonFreeFallPrevention, GateFreeFallPrevention, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			withTable(t, []SellLossTrigger{{MinTrendDirection: 1, Percentage: c.rule}})
			withCapitalProtectionMaxDrawdown(t, c.drawdown)
			if shared := sharedTriggerOf(series.TrendOf(block), series.ReadsOf(block), aggragates.SideLong); !shared.on || shared.pct != c.rule {
				t.Fatalf("fixture drifted: the dashboard must meet the rule at %v, got %+v", c.rule, shared)
			}

			committed, got := engineStep(trade, "", price, testutil.At("13:00:00"), ai)
			st := rebuildState(committed)
			if !got.Activated || !st.freeFallPreventionActive || !st.capitalProtectionActive || st.freeFallPreventionActivated != !c.capital || st.capitalProtectionActivated != c.capital {
				t.Fatalf("fixture drifted: the tick must activate only the gate it reaches, with both episodes open, got %+v / %+v", got, st)
			}

			fall := priceAtMove(committed, price, -(trailRow(committed).Tolerance + 1))
			sold := Apply(committed, "", fall, ai)
			assertForced(t, sold, c.reason)
			if filed := ExitRow(committed, sold.Reason, fall).Gate; filed != c.gate {
				t.Errorf("the sold event is filed under %q, want %q", filed, c.gate)
			}
			if acts := CapitalProtectionActs(committed, fall, block, testutil.At("13:05:00")); acts != c.capital {
				t.Errorf("CapitalProtectionActs = %v, want %v", acts, c.capital)
			}
			if !ExitReached(committed, fall, block) {
				t.Errorf("ExitReached must read the sale")
			}
		})
	}
}
