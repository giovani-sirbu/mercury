package smarttakeloss

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// A take profit the ladder proposes in buy runs the triggers like any proposal and gives way to the
// activation alone (yieldsToActivation): the tests below hold each sale gate to it on its own
// fixture ladder (saleGates), and the ladder's other closes and every other state to the contrary.

// saleFixture is one sale gate on its fixture ladder, resting in buy with the gate on and every other
// gate off: the reading its episode opens and is held on, the rows its tick writes ahead of the
// shared action's, a print its trigger is reached at (within) and one it falls short of (over), and
// the rows it is owed at a print.
type saleFixture struct {
	gate, reason string
	features     []string
	trade        aggragates.Trades
	ai           aggragates.AIIndicators
	reads        func(down []int) aggragates.AIIndicators
	opening      []Row
	fill         float64
	within, over float64
	activated    func(price float64) Row
	held         func(price float64) Row
	reached      func(price float64) Row
	off          func(*testing.T)
	gone         func() (aggragates.Trades, aggragates.AIIndicators)
}

// saleGates are the four sale gates' fixtures, each built under the test's own t (the switches it sets are put back after).
var saleGates = []struct {
	name string
	// bounded is whether the depth bounds and the trend hold the gate's trigger (capital protection's is its own).
	bounded bool
	build   func(*testing.T) saleFixture
}{
	{"flat market", true, func(t *testing.T) saleFixture {
		flatMarketAlone(t)
		trade := flatMarketLadder(false)
		ai, fill := flatMarketMarket(trade), trade.PositionPrice
		within, over := flatMarketPrices(t, trade)
		return saleFixture{
			gate: GateFlatMarket, reason: reasonFlatMarket,
			trade: trade, ai: ai, opening: []Row{triggeredBy(trade, ai)}, fill: fill, within: within, over: over,
			reads: func(down []int) aggragates.AIIndicators {
				return withBlock(withTrendReads(ai.SmartTakeLoss, down, nil))
			},
			activated: func(price float64) Row {
				return FlatMarketStopLossActivatedRow(fill, -handLossPct(trade, price))
			},
			held: func(price float64) Row { return FlatMarketSaleHeldRow(fill, -handLossPct(trade, price)) },
			reached: func(price float64) Row {
				return TriggerReachedRow(GateFlatMarket, fill, -handLossPct(trade, price), FullTrendTrigger())
			},
			off:  func(t *testing.T) { withFlatMarket(t, false) },
			gone: func() (aggragates.Trades, aggragates.AIIndicators) { return trade, barsGone(trade) },
		}
	}},
	{"free fall prevention", true, func(t *testing.T) saleFixture {
		freeFallPreventionAlone(t)
		trade := flatMarketLadder(false)
		ai, fill := freeFallMarket(trade), trade.PositionPrice
		within, over := flatMarketPrices(t, trade)
		return saleFixture{
			gate: GateFreeFallPrevention, reason: reasonFreeFallPrevention,
			trade: trade, ai: ai, opening: []Row{freeFallTriggeredBy(trade)}, fill: fill, within: within, over: over,
			reads: func(down []int) aggragates.AIIndicators {
				return withBlock(withTrendReads(ai.SmartTakeLoss, down, nil))
			},
			activated: func(price float64) Row {
				return FreeFallPreventionStopLossActivatedRow(fill, -handLossPct(trade, price))
			},
			held: func(price float64) Row { return FreeFallPreventionSaleHeldRow(fill, -handLossPct(trade, price)) },
			reached: func(price float64) Row {
				return TriggerReachedRow(GateFreeFallPrevention, fill, -handLossPct(trade, price), FullTrendTrigger())
			},
			off: func(t *testing.T) { withFreeFallPrevention(t, false) },
			gone: func() (aggragates.Trades, aggragates.AIIndicators) {
				return trade, freeFallMisses(aggragates.SideLong)["no row in free fall"]
			},
		}
	}},
	{"slow decline", true, func(t *testing.T) saleFixture {
		underTheShippedCap(t)
		trade := pendingTrade()
		armed, _ := armingOverTheCap(t, trade)
		within, _ := firstPriceWithinTheCap(t, trade, armed)
		return saleFixture{
			gate: GateSlowDecline, reason: reasonSlowDecline, features: []string{FeatureQuietDecline},
			trade: trade, ai: nothingServed(), fill: slowDeclineLastFill, within: within, over: armed,
			reads: func(down []int) aggragates.AIIndicators {
				return withBlock(withTrendReads(aggragates.SmartTakeLossIndicators{}, down, nil))
			},
			activated: func(price float64) Row {
				return StopLossActivatedRow(slowDeclineLastFill, -handLossPct(trade, price), FeatureQuietDecline)
			},
			held: func(price float64) Row {
				return SaleHeldRow(slowDeclineLastFill, -handLossPct(trade, price), FeatureQuietDecline)
			},
			reached: func(price float64) Row {
				return TriggerReachedRow(GateSlowDecline, slowDeclineLastFill, -handLossPct(trade, price), FullTrendTrigger(), FeatureQuietDecline)
			},
			off: func(t *testing.T) {
				withQuietDecline(t, false)
				withSwingDecline(t, false)
			},
			gone: func() (aggragates.Trades, aggragates.AIIndicators) { return watchedTrade(), slowDeclineBlock(false) },
		}
	}},
	{"capital protection", false, func(t *testing.T) saleFixture {
		capitalProtectionAlone(t)
		trade := lastDepthLadder()
		within, _ := firstPriceWithinTheDrawdown(t, trade)
		fill := trade.PositionPrice
		return saleFixture{
			gate: GateCapitalProtection, reason: reasonCapitalProtection,
			trade: trade, ai: withBlock(solBlock()), opening: []Row{capitalProtectionTriggered(trade)}, fill: fill, within: within, over: lastDepthPrint,
			reads: func(down []int) aggragates.AIIndicators {
				return withBlock(withTrendReads(solBlock(), down, nil))
			},
			activated: func(price float64) Row {
				return CapitalProtectionStopLossActivatedRow(fill, -handLossPct(trade, price))
			},
			held: func(price float64) Row { return CapitalProtectionSaleHeldRow(fill, -handLossPct(trade, price)) },
			reached: func(price float64) Row {
				return TriggerReachedRow(GateCapitalProtection, fill, -handLossPct(trade, price), 0-capitalProtectionMaxDrawdown)
			},
			off:  func(t *testing.T) { withCapitalProtectionExit(t, false) },
			gone: func() (aggragates.Trades, aggragates.AIIndicators) { return trade, nothingServed() },
		}
	}},
}

// opened is the fixture's ladder with its episode's opening rows written, so that a tick writes none of them again.
func (c saleFixture) opened() aggragates.Trades {
	return withRows(c.trade, c.opening...)
}

// A take profit proposed in buy, the trailing one the backtest forces included, gives way to the
// activation of each sale gate when its trigger is reached: the answer is Apply's on the empty
// proposal (the stopLoss activated, the same rows), the next tick anchors the stopLoss at the
// print, and a fall past the tolerance sells under that gate's reason, filed under that gate.
func TestATakeProfitProposedInBuyGivesWayToTheActivation(t *testing.T) {
	for _, gate := range saleGates {
		t.Run(gate.name, func(t *testing.T) {
			c := gate.build(t)
			want := append(slices.Clone(c.opening), c.activated(c.within))
			bare := Apply(c.trade, "", c.within, c.ai)
			assertActivated(t, bare, want...)

			for _, position := range takeProfitProposals {
				got := Apply(c.trade, position, c.within, c.ai)
				assertActivated(t, got, want...)
				if !reflect.DeepEqual(got, bare) {
					t.Errorf("%s: the answer is %+v, want the empty proposal's %+v", position, got, bare)
				}

				next, _ := engineStep(c.trade, position, c.within, testutil.At("13:00:00"), c.ai)
				if next.PositionType != "stopLoss" || next.PositionPrice != c.within {
					t.Fatalf("%s: the tick must leave the stopLoss anchored at the print, got %q at %v", position, next.PositionType, next.PositionPrice)
				}
				assertTrailHeld(t, Apply(next, "", c.within, c.ai))
				fall := priceAtMove(next, c.within, -(trailRow(next).Tolerance + 1))
				sold := Apply(next, "", fall, c.ai)
				assertSold(t, sold, c.reason)
				if filed := ExitRow(next, sold.Reason, fall).Gate; filed != c.gate {
					t.Errorf("%s: the sold event is filed under %q, want %q", position, filed, c.gate)
				}
			}
		})
	}
}

// A take profit stands when no activation is due: with the sale held (the print short of the
// trigger, a depth bound, the trend not read) the held row of the tick is written beside it, once;
// with the wait after the trigger running, an order resting, no trigger standing or the gate off,
// it comes back untouched. The next tick's take profit is untouched, the row being once.
func TestATakeProfitStandsWhileTheSaleIsHeldOrNoTriggerStands(t *testing.T) {
	for _, gate := range saleGates {
		t.Run(gate.name, func(t *testing.T) {
			c := gate.build(t)
			opened := c.opened()

			for _, position := range takeProfitProposals {
				got := Apply(c.trade, position, c.over, c.ai)
				assertWaits(t, got, position, append(slices.Clone(c.opening), c.held(c.over))...)
				written := withRows(c.trade, got.Rows...)
				for _, price := range []float64{c.over, c.over - 1} {
					assertUntouched(t, Apply(written, position, price, c.ai), position)
				}
				assertActivated(t, Apply(written, position, c.within, c.ai), c.activated(c.within))

				if gate.bounded {
					unread := c.reads([]int{0, 0, 0})
					byTrend := saleHeldRow(c.gate, c.fill, heldByTrendOf(unread.SmartTakeLoss, false), c.features)
					got := Apply(opened, position, c.within, unread)
					assertWaits(t, got, position, byTrend)
					assertUntouched(t, Apply(withRows(opened, got.Rows...), position, c.within, unread), position)

					filled := len(rebuildState(opened).fills)
					withSellLossStartAtDepth(t, filled+1)
					byFloor := saleHeldRow(c.gate, c.fill, heldUnderDepth(filled, filled+1), c.features)
					got = Apply(opened, position, c.within, c.ai)
					assertWaits(t, got, position, byFloor)
					assertUntouched(t, Apply(withRows(opened, got.Rows...), position, c.within, c.ai), position)
					withSellLossStartAtDepth(t, 0)
				}

				assertWaits(t, Apply(withRestingOrder(c.trade), position, c.within, c.ai), position, c.opening...)
				for _, price := range []float64{c.within, c.over} {
					assertUntouched(t, Apply(withRestingOrder(opened), position, price, c.ai), position)
				}

				goneTrade, goneAI := c.gone()
				assertUntouched(t, Apply(goneTrade, position, c.within, goneAI), position)
			}

			c.off(t)
			for _, position := range takeProfitProposals {
				assertUntouched(t, Apply(c.trade, position, c.within, c.ai), position)
			}
		})
	}

	t.Run("inside the wait after the trigger, an episode gate", func(t *testing.T) {
		for _, gate := range episodeGates {
			t.Run(gate.name, func(t *testing.T) {
				gate.alone(t)
				withWaitAfterTrigger(t, saleWait)
				start := flatMarketLadder(false)
				ai, fill := gate.market(start), start.PositionPrice
				atTheCap, oneStepLower := flatMarketPrices(t, start)
				opened := testutil.At("13:00:00")
				triggered := gate.triggered(start, ai)
				held := gate.held(fill, -handLossPct(start, oneStepLower))
				activation := gate.activated(fill, -handLossPct(start, atTheCap))
				trade := withRow(start, triggered, opened)

				for _, position := range takeProfitProposals {
					for _, price := range []float64{atTheCap, oneStepLower} {
						assertWaits(t, ApplyAt(start, position, price, ai, opened), position, triggered)
						assertUntouched(t, ApplyAt(trade, position, price, ai, opened.Add(saleWait-time.Second)), position)
					}
					past := opened.Add(saleWait)
					assertWaits(t, ApplyAt(trade, position, oneStepLower, ai, past), position, held)
					assertActivated(t, ApplyAt(trade, position, atTheCap, ai, past), activation)
				}
			})
		}
	})

	t.Run("inside the wait after the trigger, the slow decline", func(t *testing.T) {
		slowDeclineWaitAlone(t)
		trade, ai := watchedTrade(), slowDeclineBlock(true)
		within, over := handLossPct(trade, slowDeclineBand), handLossPct(trade, underTheBand)
		withAllowedLoss(t, (within+over)/2)
		opened := testutil.At("18:00:00")
		pending := quietPending(slowDeclineLastFill)
		waiting := withRow(trade, pending, opened)

		for _, position := range takeProfitProposals {
			assertWaits(t, ApplyAt(trade, position, slowDeclineBand, ai, opened), position, pending)
			assertUntouched(t, ApplyAt(waiting, position, slowDeclineBand, ai, opened.Add(saleWait-time.Second)), position)
			past := opened.Add(saleWait)
			assertWaits(t, ApplyAt(waiting, position, underTheBand, ai, past), position, SaleHeldRow(slowDeclineLastFill, -over, FeatureQuietDecline))
			assertActivated(t, ApplyAt(waiting, position, slowDeclineBand, ai, past), quietActivation(trade, slowDeclineBand))
		}
	})
}

// Only a take profit proposed in buy meets the action: in a self-armed stopLoss, a committed one
// (held, re-anchored or sold by the trail), a new trade or any state that decided a close, the take
// profit comes back untouched with no row, the trigger reached or not.
func TestATakeProfitIsReplacedInBuyAlone(t *testing.T) {
	for _, gate := range saleGates {
		t.Run(gate.name, func(t *testing.T) {
			c := gate.build(t)
			opened := c.opened()
			committed := committedBy(opened, c.activated(c.within))

			for _, position := range takeProfitProposals {
				for _, price := range []float64{c.within, c.over} {
					for _, state := range []string{"stopLoss", "forceTrailingStopLoss"} {
						assertUntouched(t, Apply(restingIn(opened, state), position, price, c.ai), position)
					}
					for _, state := range []string{"new", "", "takeProfit", "forceTrailingTakeProfit", "sell", "sellLoss", "impasse", "sellParent"} {
						assertUntouched(t, Apply(restingIn(opened, state), position, price, c.ai), position)
					}
				}
				for name, resting := range map[string]aggragates.Trades{
					"held":       anchoredToHold(committed, c.within),
					"re-anchor":  anchoredToReanchor(committed, c.within),
					"sold under": anchoredToSell(committed, c.within),
				} {
					for _, state := range []string{"stopLoss", "forceTrailingStopLoss"} {
						if got := Apply(restingIn(resting, state), position, c.within, c.ai); got.Position != position || got.Reason != "" || got.Activated || len(got.Rows) != 0 {
							t.Errorf("%s, %s committed in %s: the take profit must come back untouched, got %+v", position, name, state, got)
						}
					}
				}
			}
		})
	}
}

// The ladder's other closes stay protected in buy: a sell, the trailing take profit's update, a
// sellLoss, a sellParent and an impasse come back untouched with no row at a print the trigger is
// reached at and one it falls short of, where the take profit proposed beside them gives way.
func TestTheLaddersOtherClosesStayProtectedInBuy(t *testing.T) {
	for _, gate := range saleGates {
		t.Run(gate.name, func(t *testing.T) {
			c := gate.build(t)
			for _, price := range []float64{c.within, c.over} {
				for _, position := range closingProposals {
					assertUntouched(t, Apply(c.trade, position, price, c.ai), position)
					assertUntouched(t, Apply(c.opened(), position, price, c.ai), position)
				}
			}
			assertActivated(t, Apply(c.trade, "takeProfit", c.within, c.ai), append(slices.Clone(c.opening), c.activated(c.within))...)
		})
	}
}

// The shape of trade 212087: a free fall prevention episode held under the 0.70 % rule (the 1h row
// alone in free fall) with a take profit the ladder proposes in buy. The dashboard then reads the
// 4h in free fall too and the trigger moves to -0.15 %: the held row is written again beside the
// standing take profit while the print is short of it, and a print at a profit in [-0.15, 0)
// activates the stopLoss in its place, the row saying "at or over -0.15%", which trails and sells
// for the gate. Under the 0.70 % rule the same print leaves the take profit alone.
func TestALatchedTakeProfitAtTheTriggerActivatesTheFreeFallPreventionSale(t *testing.T) {
	freeFallPreventionAlone(t)
	withTable(t, []SellLossTrigger{
		{MinTrendDirection: 1, Percentage: 0.7},
		{MinTrendDirection: 1, MinFreeFall: []int{1, 1, 0}, Percentage: -0.15},
	})
	trade := flatMarketLadder(false)
	fill := trade.PositionPrice
	oneRow := withBlock(freeFallBlockOf(aggragates.SideLong, true, false, false))
	twoRows := withBlock(freeFallBlockOf(aggragates.SideLong, true, true, false))
	for _, c := range []struct {
		ai   aggragates.AIIndicators
		want float64
	}{{oneRow, 0.7}, {twoRows, -0.15}} {
		block := c.ai.SmartTakeLoss
		if shared := sharedTriggerOf(series.TrendOf(block), series.ReadsOf(block), aggragates.SideLong); !shared.on || shared.pct != c.want {
			t.Fatalf("fixture drifted: the dashboard must meet the %v rule, got %+v", c.want, shared)
		}
	}
	const short, inBand = 99.60, 99.90
	if loss, band := profitAt(trade, short), profitAt(trade, inBand); !(loss < -0.15 && -0.15 <= band && band < 0) {
		t.Fatalf("fixture drifted: the prints must book a profit under -0.15 and one in [-0.15, 0), got %v and %v", loss, band)
	}

	first, got := engineStep(trade, "", short, testutil.At("17:00:00"), oneRow)
	assertRows(t, got, freeFallTriggeredBy(trade), saleHeldByProfitRow(GateFreeFallPrevention, fill, profitAt(trade, short), 0.7, nil))
	if st := rebuildState(first); !st.freeFallPreventionSaleHeld || st.freeFallPreventionHeldUnder != (heldTrigger{pct: 0.7, named: true}) {
		t.Fatalf("the episode must stand held under 0.70 %%, got %+v", st)
	}

	for _, position := range takeProfitProposals {
		assertUntouched(t, Apply(first, position, inBand, oneRow), position)
	}

	second, got := engineStep(first, "takeProfit", short, testutil.At("18:00:00"), twoRows)
	assertWaits(t, got, "takeProfit", saleHeldByProfitRow(GateFreeFallPrevention, fill, profitAt(trade, short), -0.15, nil))
	if st := rebuildState(second); st.freeFallPreventionHeldUnder != (heldTrigger{pct: -0.15, named: true}) {
		t.Fatalf("the episode must stand held under -0.15 %%, got %+v", st)
	}

	for _, position := range takeProfitProposals {
		got := Apply(second, position, inBand, twoRows)
		assertActivated(t, got, stopLossActivatedRow(GateFreeFallPrevention, fill, profitAt(trade, inBand), -0.15, nil))
		if !strings.Contains(got.Rows[0].Message, "at or over -0.15%") {
			t.Errorf("%s: the activation row must say \"at or over -0.15%%\", got %q", position, got.Rows[0].Message)
		}
	}

	third, _ := engineStep(second, "takeProfit", inBand, testutil.At("19:00:00"), twoRows)
	fall := priceAtMove(third, inBand, -(trailRow(third).Tolerance + 1))
	sold := Apply(third, "", fall, twoRows)
	assertSold(t, sold, reasonFreeFallPrevention)
	if ExitRow(third, sold.Reason, fall).Gate != GateFreeFallPrevention {
		t.Errorf("the sale must be filed under the free fall prevention gate, got %+v", ExitRow(third, sold.Reason, fall))
	}
}
