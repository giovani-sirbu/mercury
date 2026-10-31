package smartsell

import (
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The round ladder (102, 101, 99 and 98, a hundred units each) breaks even at exactly 100, and a
// cent under books a loss: the two prints the path is read at.
const (
	pathBreakEven = 100.0
	pathUnder     = 99.99
	pathGain      = 101.0
	pathLoss      = 99.0
)

// pathGate is one sale gate the activation's path is asked of: its switches, its ladder resting in
// buy, the reading that triggers it and the rows of the tick that activates at profitPct.
type pathGate struct {
	name   string
	reason string
	setup  func(t *testing.T)
	trade  func() aggragates.Trades
	ai     func(trade aggragates.Trades) aggragates.AIIndicators
	rows   func(trade aggragates.Trades, ai aggragates.AIIndicators, profitPct float64) []Row
}

// pathGates are the four gates on the round ladder, every loss reaching the trigger.
func pathGates() []pathGate {
	return []pathGate{
		{
			name:   "slow decline",
			reason: reasonSlowDecline,
			setup: func(t *testing.T) {
				withQuietDecline(t, true)
				withFlatMarket(t, false)
				withAllowedLoss(t, anyLoss)
			},
			trade: exactLadder,
			ai: func(aggragates.Trades) aggragates.AIIndicators {
				return nothingServed()
			},
			rows: func(trade aggragates.Trades, _ aggragates.AIIndicators, profitPct float64) []Row {
				return []Row{ActivatedRow(trade.PositionPrice, profitPct, FeatureQuietDecline)}
			},
		},
		{
			name:   "flat market",
			reason: reasonFlatMarket,
			setup: func(t *testing.T) {
				flatMarketAlone(t)
				withAllowedLoss(t, anyLoss)
			},
			trade: func() aggragates.Trades {
				return flatMarketLadder(false)
			},
			ai: flatMarketMarket,
			rows: func(trade aggragates.Trades, ai aggragates.AIIndicators, profitPct float64) []Row {
				return []Row{triggeredBy(trade, ai), FlatMarketActivatedRow(trade.PositionPrice, profitPct)}
			},
		},
		{
			name:   "free fall prevention",
			reason: reasonFreeFallPrevention,
			setup: func(t *testing.T) {
				freeFallPreventionAlone(t)
				withAllowedLoss(t, anyLoss)
			},
			trade: func() aggragates.Trades {
				return flatMarketLadder(false)
			},
			ai: freeFallMarket,
			rows: func(trade aggragates.Trades, _ aggragates.AIIndicators, profitPct float64) []Row {
				return []Row{freeFallTriggeredBy(trade), FreeFallPreventionActivatedRow(trade.PositionPrice, profitPct)}
			},
		},
		{
			name:   "capital protection",
			reason: reasonCapitalProtection,
			setup:  capitalProtectionAlone,
			trade: func() aggragates.Trades {
				return capitalProtectionLadder(false)
			},
			ai: func(aggragates.Trades) aggragates.AIIndicators {
				return withBlock(solBlock())
			},
			rows: func(trade aggragates.Trades, _ aggragates.AIIndicators, profitPct float64) []Row {
				return []Row{capitalProtectionTriggered(trade), CapitalProtectionActivatedRow(trade.PositionPrice, profitPct)}
			},
		},
	}
}

// committedAt is the gate's ladder after the tick that activated it at price, as the engines leave it.
func (gate pathGate) committedAt(t *testing.T, price float64) aggragates.Trades {
	t.Helper()
	trade := gate.trade()
	committed, got := engineStep(trade, "", price, pathClock, gate.ai(trade))
	if !got.Activated || got.Position != committed.PositionType {
		t.Fatalf("%s: fixture drifted: the tick at %v must activate the sale, got %+v", gate.name, price, got)
	}
	return committed
}

// pathClock stamps the rows of the path's ticks; Apply runs with no clock, so no wait holds them.
var pathClock = testutil.At("13:00:00")

// The path is the sign of the profit the activation measures: at break even and over, the ladder
// moves to takeProfit with the take profit row; a cent under break even, to trailingBuy with the
// take loss row; alike for every gate, capital protection at break even included. The event beside
// the row names the state, the log row's key the path.
func TestTheActivationTakesThePathOfTheProfitsSign(t *testing.T) {
	for _, gate := range pathGates() {
		t.Run(gate.name, func(t *testing.T) {
			gate.setup(t)
			trade := gate.trade()
			if profitPct, _ := saleProfit(trade, pathBreakEven, 0); profitPct != 0 {
				t.Fatalf("fixture drifted: the round ladder must break even at %v, got %v%%", pathBreakEven, profitPct)
			}
			ai := gate.ai(trade)
			for _, c := range []struct {
				price float64
				state string
				words string
				key   string
			}{
				{pathBreakEven, "takeProfit", "take profit activated", "_TAKE_PROFIT_ACTIVATED"},
				{pathGain, "takeProfit", "take profit activated", "_TAKE_PROFIT_ACTIVATED"},
				{pathUnder, "trailingBuy", "take loss activated", "_TAKE_LOSS_ACTIVATED"},
			} {
				profitPct := 0 - handLossPct(trade, c.price)
				got := Apply(trade, "", c.price, ai)
				assertActivatedIn(t, got, c.state, gate.rows(trade, ai, profitPct)...)
				activation := got.Rows[len(got.Rows)-1]
				if activation.State != c.state || !strings.Contains(activation.Message, c.words) {
					t.Errorf("at %v: the activation row must name %q and carry %s, got %+v", c.price, c.words, c.state, activation)
				}
				logs, events := TickRows(trade, got.Rows, pathClock)
				if key := logs[len(logs)-1].MessageKey; !strings.HasSuffix(key, c.key) {
					t.Errorf("at %v: the activation's log row is keyed %q, want the suffix %q", c.price, key, c.key)
				}
				if data := string(events[len(events)-1].Data); !strings.Contains(data, `"state":"`+c.state+`"`) {
					t.Errorf("at %v: the activated event must name the state, got %s", c.price, data)
				}
			}
		})
	}
}

// A take profit the ladder proposes in buy gives way to the activation on either path.
func TestATakeProfitProposedInBuyYieldsToEitherPath(t *testing.T) {
	for _, gate := range pathGates() {
		t.Run(gate.name, func(t *testing.T) {
			gate.setup(t)
			trade := gate.trade()
			ai := gate.ai(trade)
			for _, position := range takeProfitProposals {
				for price, state := range map[float64]string{pathGain: "takeProfit", pathUnder: "trailingBuy"} {
					assertActivatedIn(t, Apply(trade, position, price, ai), state, gate.rows(trade, ai, 0-handLossPct(trade, price))...)
				}
			}
		})
	}
}

// Once committed the path is the activation's: a trigger read on a later tick at the other sign (a
// profit to wait for, or a loss to take) moves nothing, the trail holds, re-anchors and sells from
// the state it is in, and a re-anchor stays in that state.
func TestACommittedSaleTrailsFromItsStateWhateverTheTriggerReadsLater(t *testing.T) {
	for _, gate := range pathGates() {
		t.Run(gate.name, func(t *testing.T) {
			gate.setup(t)
			for activatedAt, state := range map[float64]string{pathGain: "takeProfit", pathUnder: "trailingBuy"} {
				committed := gate.committedAt(t, activatedAt)
				if committed.PositionType != state {
					t.Fatalf("fixture drifted: the activation at %v must move to %s, got %s", activatedAt, state, committed.PositionType)
				}
				ai := gate.ai(committed)
				for _, allowed := range []float64{-5, anyLoss} {
					withAllowedLoss(t, allowed)
					assertTrailHeld(t, Apply(anchoredToHold(committed, activatedAt), "", activatedAt, ai))
					assertActivatedIn(t, Apply(anchoredToReanchor(committed, activatedAt), "", activatedAt, ai), state)
					assertSoldAt(t, Apply(anchoredToSell(committed, activatedAt), "", activatedAt, ai), gate.reason, committed, activatedAt)
				}
			}
		})
	}
}

// A takeProfit an activation committed is the overlay's whatever the ladder proposes from it (its
// sale, its raise, its release, nothing): the trail answers every proposal alike. With no trigger
// standing (the gate switched off) the proposal comes back as the ladder made it.
func TestACommittedTakeProfitReplacesEveryLadderProposal(t *testing.T) {
	flatMarketAlone(t)
	withAllowedLoss(t, anyLoss)
	gate := pathGates()[1]
	committed := gate.committedAt(t, pathGain)
	ai := gate.ai(committed)
	for _, position := range []string{"", "sell", "update_takeProfit", "update_buy", "takeProfit"} {
		assertTrailHeld(t, Apply(anchoredToHold(committed, pathGain), position, pathGain, ai))
		assertActivatedIn(t, Apply(anchoredToReanchor(committed, pathGain), position, pathGain, ai), "takeProfit")
		assertSoldAt(t, Apply(anchoredToSell(committed, pathLoss), position, pathLoss, ai), reasonFlatMarket, committed, pathLoss)
	}

	withFlatMarket(t, false)
	for _, position := range []string{"", "sell", "update_takeProfit", "update_buy"} {
		assertUntouched(t, Apply(anchoredToSell(committed, pathLoss), position, pathLoss, ai), position)
	}
}

// A takeProfit the ladder armed itself is its own: with the episode open and no activation, or with
// an activation that moved the ladder to trailingBuy (stale, the ladder bounced since), every
// proposal of the take profit comes back untouched.
func TestANaturalTakeProfitIsTheLaddersOwn(t *testing.T) {
	flatMarketAlone(t)
	withAllowedLoss(t, anyLoss)
	trade := flatMarketLadder(false)
	ai, fill := flatMarketMarket(trade), trade.PositionPrice
	open := withRows(trade, triggeredBy(trade, ai))
	for name, natural := range map[string]aggragates.Trades{
		"no activation":                  open,
		"an activation into trailingBuy": withRows(open, FlatMarketActivatedRow(fill, -0.5)),
	} {
		resting := restingIn(natural, "takeProfit")
		for _, position := range []string{"", "sell", "update_takeProfit", "update_buy"} {
			for _, price := range []float64{pathGain, pathLoss} {
				if got := Apply(anchoredToSell(resting, price), position, price, ai); got.Position != position || got.Reason != "" || got.Activated {
					t.Errorf("%s, %q at %v: a natural takeProfit is the ladder's own, got %+v", name, position, price, got)
				}
			}
		}
	}
}

// A gate's activation commits the ladder only in the state it moved it to: an activation into
// takeProfit on a ladder resting in trailingBuy commits nothing, the trailingBuy is self-armed and
// sold on the trigger reached (here any loss), where a committed one would trail.
func TestAGateFlagStandsOnlyInItsTargetedState(t *testing.T) {
	flatMarketAlone(t)
	withAllowedLoss(t, anyLoss)
	trade := flatMarketLadder(false)
	ai, fill := flatMarketMarket(trade), trade.PositionPrice
	intoTakeProfit := withRows(trade, triggeredBy(trade, ai), FlatMarketActivatedRow(fill, 0.5))
	resting := anchoredToHold(restingIn(intoTakeProfit, "trailingBuy"), pathLoss)
	assertSoldAt(t, Apply(resting, "", pathLoss, ai), reasonFlatMarket, trade, pathLoss)
	assertTrailHeld(t, Apply(restingIn(resting, "takeProfit"), "", pathLoss, ai))
}

// Documented, not wished: an activated event into takeProfit whose chain never ran (the trade lock
// taken by another tick) leaves the ladder in buy, where the activation is retried; should the
// ladder instead arm its own take profit within the same episode, the stale activation commits it
// and the overlay trails it.
func TestAStaleTakeProfitActivationCommitsANaturalTakeProfit(t *testing.T) {
	flatMarketAlone(t)
	withAllowedLoss(t, anyLoss)
	trade := flatMarketLadder(false)
	ai, fill := flatMarketMarket(trade), trade.PositionPrice
	stale := withRows(trade, triggeredBy(trade, ai), FlatMarketActivatedRow(fill, 0.5))
	if got := Apply(stale, "", pathGain, ai); !got.Activated || got.Position != "takeProfit" {
		t.Fatalf("a ladder still in buy beside a stale activation has the activation retried, got %+v", got)
	}
	natural := restingIn(stale, "takeProfit")
	assertTrailHeld(t, Apply(anchoredToHold(natural, pathGain), "sell", pathGain, ai))
}
