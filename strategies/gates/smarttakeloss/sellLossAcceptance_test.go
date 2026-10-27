package smarttakeloss

import (
	"fmt"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/dynamicparams"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// The order's decision table of the shared action, asked of Apply itself: the real
// triggers of both gates, the rows its folds read back and the engines' write-back.
// sellLoss_test.go asks the action directly with hand-made triggers; here every answer is
// stated by hand from the table — the entries and the ceiling of the ladder as the fixture
// builds it, the loss worked out from its fills — and never read off a function of the
// package. The table is read by the ladder's STATE: in buy the stopLoss is activated on a
// real loss within the cap, in stopLoss that the ladder armed itself the ladder is sold at
// no loss and within the cap, in stopLoss that a gate activated it TRAILS from its anchor
// whatever the loss (sold under the tolerance, re-anchored over the trailing take profit,
// held in between, no row), and in any other state nothing is done; the depth bound is
// read first in the three states that act, and over either bound the action waits with its
// held rows. The ladders
// are the round ladder of whole units, whose loss at a price is exact, with the low
// interest episode already open so that no triggered row stands between a case and its
// rows. Where the shipped constants leave a row nothing to measure the test skips, naming
// the constant.

// tableStand is which gates' triggers stand on a case's ladder: the slow decline's while it
// is pending from its judged newest fill, the flat market gate's while its market reads.
// The action is named by the first that stands, the slow decline's before the low
// interest gate's.
type tableStand struct {
	name      string
	slow, low bool
	reason    string
	written   [][]string
}

var tableStands = []tableStand{
	{"the slow decline", true, false, reasonSlowDecline, [][]string{{GateSlowDecline}}},
	{"flat market", false, true, reasonFlatMarket, [][]string{{GateFlatMarket}}},
	{"both gates", true, true, reasonSlowDecline, [][]string{{GateSlowDecline}, {GateFlatMarket}, {GateSlowDecline, GateFlatMarket}}},
}

// tableCase is one decision asked of Apply: what it must answer, the reason of the sale or
// none, whether the stopLoss is activated (an activation, or the re-anchor of a committed
// ladder), whether a committed ladder is held (the proposal replaced by nothing), and the
// rows in order.
type tableCase struct {
	name      string
	trade     aggragates.Trades
	ai        aggragates.AIIndicators
	position  string
	price     float64
	reason    string
	activated bool
	held      bool
	rows      []Row
}

// tableLadder is the round ladder at the ceiling given, resting in buy, its gates as the
// stand says (pending on the quiet decline from the newest fill, the flat market episode
// open) and the held row of each gate named in written already on the log.
func tableLadder(stand tableStand, ceiling float64, written ...string) (aggragates.Trades, aggragates.AIIndicators) {
	trade := flatMarketLadder(false)
	trade.StrategyPair.StrategySettings[0].Depths = ceiling
	fill, ai := trade.PositionPrice, withBlock(withEveryRowTrend(aggragates.SmartTakeLossIndicators{}))
	if stand.slow {
		trade = withRows(trade, quietPending(fill))
	}
	if stand.low {
		ai = flatMarketMarket(trade)
		trade = withRows(trade, triggeredBy(trade, ai))
	}
	if slices.Contains(written, GateSlowDecline) {
		trade = withRows(trade, SaleHeldRow(fill, -9, FeatureQuietDecline))
	}
	if slices.Contains(written, GateFlatMarket) {
		trade = withRows(trade, FlatMarketSaleHeldRow(fill, -9))
	}
	return trade, ai
}

// activatedOn is the trade as the tick that activated the stopLoss on the stand's triggers
// leaves it: the activation row of each of their gates written, and the ladder resting in
// stopLoss.
func (s tableStand) activatedOn(trade aggragates.Trades) aggragates.Trades {
	fill := trade.PositionPrice
	var rows []Row
	if s.slow {
		rows = append(rows, StopLossActivatedRow(fill, -0.1, FeatureQuietDecline))
	}
	if s.low {
		rows = append(rows, FlatMarketStopLossActivatedRow(fill, -0.1))
	}
	return committedBy(trade, rows...)
}

// owed are the held rows a refusal by a depth bound writes for the gates of the stand whose
// row is not written yet, in trigger order, worded by why and naming no trigger.
func (s tableStand) owed(fill float64, why string, written ...string) []Row {
	return s.owedBy(func(trigger saleTrigger) Row { return heldRowOf(trigger, fill, why) }, written)
}

// owedByLoss are the held rows a refusal by the profit writes for the gates of the stand whose
// row is not written yet, in trigger order: the loss (lossPct) under the cap (capPct), each row
// naming the trigger it is held under (heldByLossRowOf).
func (s tableStand) owedByLoss(fill, lossPct, capPct float64, written ...string) []Row {
	return s.owedBy(func(trigger saleTrigger) Row { return heldByLossRowOf(trigger, fill, lossPct, capPct) }, written)
}

// owedBy is the row of each gate of the stand not in written, in trigger order.
func (s tableStand) owedBy(row func(saleTrigger) Row, written []string) []Row {
	var rows []Row
	if s.slow && !slices.Contains(written, GateSlowDecline) {
		rows = append(rows, row(saleTrigger{gate: GateSlowDecline, features: []string{FeatureQuietDecline}}))
	}
	if s.low && !slices.Contains(written, GateFlatMarket) {
		rows = append(rows, row(saleTrigger{gate: GateFlatMarket}))
	}
	return rows
}

// activates are the activation rows the stand's gates are owed on an activation, in trigger
// order: one for each gate whose trigger stands, whatever was written before, naming the
// profit beside the trigger in force (capPct, the loss it takes).
func (s tableStand) activates(fill, lossPct, capPct float64) []Row {
	var rows []Row
	if s.slow {
		rows = append(rows, activationRowOf(saleTrigger{gate: GateSlowDecline, features: []string{FeatureQuietDecline}}, fill, activatedText(lossPct, capPct)))
	}
	if s.low {
		rows = append(rows, activationRowOf(saleTrigger{gate: GateFlatMarket}, fill, activatedText(lossPct, capPct)))
	}
	return rows
}

func runTable(t *testing.T, cases []tableCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Apply(c.trade, c.position, c.price, c.ai)
			wantPosition := c.position
			switch {
			case c.reason != "":
				wantPosition = "sellLoss"
			case c.activated:
				wantPosition = "stopLoss"
			case c.held:
				wantPosition = ""
			}
			if got.Position != wantPosition || got.Reason != c.reason || got.Activated != c.activated {
				t.Errorf("answered %q (reason %q, activated %v), want %q (reason %q, activated %v)", got.Position, got.Reason, got.Activated, wantPosition, c.reason, c.activated)
			}
			if len(got.Rows) != len(c.rows) || (len(c.rows) > 0 && !sameRows(got.Rows, c.rows)) {
				t.Errorf("rows = %+v, want exactly %+v", got.Rows, c.rows)
			}
		})
	}
}

// sameRows is whether two lists of rows are the same rows in the same order, the trigger a held
// row names included: none on both, or the same value on both (heldByLossRowOf).
func sameRows(got, want []Row) bool {
	return slices.EqualFunc(got, want, func(a, b Row) bool {
		return a.Message == b.Message && a.Price == b.Price && a.Gate == b.Gate && a.Event == b.Event &&
			slices.Equal(a.Features, b.Features) && slices.Equal(a.Reasons, b.Reasons) && sameTrigger(a.Trigger, b.Trigger)
	})
}

// sameTrigger is whether two triggers a row names are both none, or both named and equal.
func sameTrigger(a, b *float64) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

// Every row of the order's table, for each gate alone and both together. The ladders are the
// round ladder's entries; the ceilings are built from the bound in force: a ceiling exactly
// at it (the action goes ahead), one a depth under it (held by depth), none known, and a
// roomy one for the cap to decide.
func TestSellLossAcceptanceTheDecisionTableThroughApply(t *testing.T) {
	flatMarketAlone(t)
	withQuietDecline(t, true)
	withSellLossIgnoreLastDepths(t, SellLossIgnoreLastDepths)
	if SellLossIgnoreLastDepths <= 0 || shippedLoss <= 0 {
		t.Skip("SellLossIgnoreLastDepths is off or SellLossTriggerPercentage takes no loss: the table has no bound to read")
	}
	farOverTheCap(t, shippedLoss)
	probe := flatMarketLadder(false)
	atTheCap, oneCentOver := sellLossPrices(t, probe)
	average, filled, bound, capPct := ladder.AverageEntryPrice(probe), len(roundPrices), SellLossIgnoreLastDepths, float64(shippedLoss)
	gain, within, farOver := average*gainFactor, (atTheCap+average)/2, average*lossFactor
	atBound, pastBound, roomy, unknown := float64(filled+bound), float64(filled+bound-1), float64(filled+bound+4), 0.5
	fill, pastText := probe.PositionPrice, atDepthText(filled, int(pastBound), int(pastBound)-bound)
	noCeiling := fmt.Sprintf("depth %d of no known ceiling", filled)
	named := func(stand tableStand, state, position string, price float64) string {
		return fmt.Sprintf("%s, %s, %s at %v", stand.name, state, position, price)
	}
	heldByCap := func(stand tableStand, price float64, written ...string) []Row {
		return stand.owedByLoss(fill, lossPctOf(probe, price), capPct, written...)
	}

	var none, inert, buyActivates, buyHeldByCap, buyHeldByDepth []tableCase
	var naturalSells, naturalHeldByCap, naturalHeldByDepth, committedSells, committedReanchors, committedHolds, committedHeldByDepth, once []tableCase

	empty, emptyAI := tableLadder(tableStand{}, roomy)
	for _, state := range actingStates {
		for _, position := range addSideProposals {
			for _, price := range []float64{gain, farOver} {
				none = append(none, tableCase{name: fmt.Sprintf("no trigger, %s, %q at %v", state, position, price), trade: restingIn(empty, state), ai: emptyAI, position: position, price: price})
			}
		}
	}

	for _, stand := range tableStands {
		roomyTrade, ai := tableLadder(stand, roomy)
		boundTrade, _ := tableLadder(stand, atBound)
		unknownTrade, _ := tableLadder(stand, unknown)
		pastTrade, _ := tableLadder(stand, pastBound)
		for _, state := range inertStates {
			for _, position := range addSideProposals {
				for _, price := range []float64{gain, farOver} {
					inert = append(inert, tableCase{name: named(stand, state, position, price), trade: restingIn(roomyTrade, state), ai: ai, position: position, price: price})
				}
			}
		}

		for _, position := range addSideProposals {
			for _, price := range []float64{gain, average, within, atTheCap} {
				for _, ceilingTrade := range []aggragates.Trades{boundTrade, roomyTrade} {
					buyActivates = append(buyActivates, tableCase{
						name: named(stand, "buy", position, price), trade: ceilingTrade, ai: ai, position: position, price: price,
						activated: true, rows: stand.activates(fill, lossPctOf(probe, price), capPct),
					})
				}
			}
			for _, price := range []float64{oneCentOver, farOver} {
				buyHeldByCap = append(buyHeldByCap, tableCase{name: named(stand, "buy", position, price), trade: roomyTrade, ai: ai, position: position, price: price, rows: heldByCap(stand, price)})
				naturalHeldByCap = append(naturalHeldByCap, tableCase{name: named(stand, "stopLoss", position, price), trade: restingIn(roomyTrade, "stopLoss"), ai: ai, position: position, price: price, rows: heldByCap(stand, price)})
			}
			for _, price := range []float64{gain, within, farOver} {
				for _, past := range []struct {
					trade aggragates.Trades
					why   string
				}{{unknownTrade, noCeiling}, {pastTrade, pastText}} {
					buyHeldByDepth = append(buyHeldByDepth, tableCase{name: named(stand, "buy", position, price), trade: past.trade, ai: ai, position: position, price: price, rows: stand.owed(fill, past.why)})
					naturalHeldByDepth = append(naturalHeldByDepth, tableCase{name: named(stand, "stopLoss", position, price), trade: restingIn(past.trade, "stopLoss"), ai: ai, position: position, price: price, rows: stand.owed(fill, past.why)})
					committedHeldByDepth = append(committedHeldByDepth, tableCase{name: named(stand, "committed", position, price), trade: stand.activatedOn(past.trade), ai: ai, position: position, price: price, rows: stand.owed(fill, past.why)})
				}
			}
			for _, price := range []float64{gain, average, within, atTheCap} {
				naturalSells = append(naturalSells, tableCase{name: named(stand, "stopLoss", position, price), trade: restingIn(boundTrade, "stopLoss"), ai: ai, position: position, price: price, reason: stand.reason})
			}
			for _, price := range []float64{gain, average, within, atTheCap, oneCentOver, farOver} {
				committed := stand.activatedOn(boundTrade)
				committedSells = append(committedSells, tableCase{name: named(stand, "committed", position, price), trade: anchoredToSell(committed, price), ai: ai, position: position, price: price, reason: stand.reason})
				committedReanchors = append(committedReanchors, tableCase{name: named(stand, "committed", position, price), trade: anchoredToReanchor(committed, price), ai: ai, position: position, price: price, activated: true})
				committedHolds = append(committedHolds, tableCase{name: named(stand, "committed", position, price), trade: anchoredToHold(committed, price), ai: ai, position: position, price: price, held: true})
			}
		}

		for _, written := range stand.written {
			for _, refusal := range []struct {
				ceiling, price float64
				owed           func(written ...string) []Row
			}{
				{roomy, farOver, func(written ...string) []Row { return heldByCap(stand, farOver, written...) }},
				{pastBound, gain, func(written ...string) []Row { return stand.owed(fill, pastText, written...) }},
			} {
				trade, _ := tableLadder(stand, refusal.ceiling, written...)
				for _, state := range []string{"buy", "stopLoss"} {
					once = append(once, tableCase{
						name:  fmt.Sprintf("%s, %s, written for %v, held at ceiling %v", stand.name, state, written, refusal.ceiling),
						trade: restingIn(trade, state), ai: ai, position: "stopLoss", price: refusal.price, rows: refusal.owed(written...),
					})
				}
			}
		}
	}
	t.Run("no trigger: no rows and no sale", func(t *testing.T) { runTable(t, none) })
	t.Run("a state the action does not act on: no rows and no sale, whatever the bounds", func(t *testing.T) { runTable(t, inert) })
	t.Run("a ladder in buy within the depth bound, at or over the trigger, a gain included: the stopLoss is activated, a row for each gate", func(t *testing.T) { runTable(t, buyActivates) })
	t.Run("a ladder in buy within the bound, over the cap: held by the cap", func(t *testing.T) { runTable(t, buyHeldByCap) })
	t.Run("a ladder in buy, the ceiling unknown or past the depth bound: held by depth, the loss not read", func(t *testing.T) { runTable(t, buyHeldByDepth) })
	t.Run("a ladder in stopLoss that armed itself, no loss or within the cap: sold, the first trigger's reason and no row", func(t *testing.T) { runTable(t, naturalSells) })
	t.Run("a ladder in stopLoss that armed itself, over the cap: held by the cap", func(t *testing.T) { runTable(t, naturalHeldByCap) })
	t.Run("a ladder in stopLoss that armed itself, past the depth bound: held by depth", func(t *testing.T) { runTable(t, naturalHeldByDepth) })
	t.Run("a ladder in stopLoss that a gate activated, a fall under the tolerance of its anchor: sold at any loss, the reason of the first trigger that activated and no row", func(t *testing.T) { runTable(t, committedSells) })
	t.Run("a ladder in stopLoss that a gate activated, a rise over the trailing take profit: re-anchored at any loss, no row", func(t *testing.T) { runTable(t, committedReanchors) })
	t.Run("a ladder in stopLoss that a gate activated, between the bounds: held at any loss, the proposal replaced by nothing and no row", func(t *testing.T) { runTable(t, committedHolds) })
	t.Run("a ladder in stopLoss that a gate activated, past the depth bound: still held by depth", func(t *testing.T) { runTable(t, committedHeldByDepth) })
	t.Run("a held row is written once per episode and gate, whichever bound held it, in both states", func(t *testing.T) { runTable(t, once) })

	t.Run("the activation row is written on every activation, whether the gate's held row was written before or not", func(t *testing.T) {
		var activations []tableCase
		for _, stand := range tableStands {
			for _, written := range stand.written {
				trade, ai := tableLadder(stand, roomy, written...)
				activations = append(activations, tableCase{
					name: fmt.Sprintf("%s, written for %v", stand.name, written), trade: trade, ai: ai, position: "", price: within,
					activated: true, rows: stand.activates(fill, lossPctOf(probe, within), capPct),
				})
			}
		}
		runTable(t, activations)
	})
	t.Run("any activated gate commits the sale of both: the reason is the trigger's that activated it", func(t *testing.T) {
		both := tableStands[2]
		trade, ai := tableLadder(both, roomy)
		onlySlow := committedBy(trade, StopLossActivatedRow(fill, -0.1, FeatureQuietDecline))
		onlyLow := committedBy(trade, FlatMarketStopLossActivatedRow(fill, -0.1))
		natural := restingIn(trade, "stopLoss")
		runTable(t, []tableCase{
			{name: "only the slow decline activated, far over the cap", trade: anchoredToSell(onlySlow, farOver), ai: ai, position: "update_stopLoss", price: farOver, reason: reasonSlowDecline},
			{name: "only the flat market activated, far over the cap", trade: anchoredToSell(onlyLow, farOver), ai: ai, position: "update_stopLoss", price: farOver, reason: reasonFlatMarket},
			{name: "only the slow decline activated, held far over the cap", trade: anchoredToHold(onlySlow, farOver), ai: ai, position: "update_stopLoss", price: farOver, held: true},
			{name: "only the flat market activated, re-anchored far over the cap", trade: anchoredToReanchor(onlyLow, farOver), ai: ai, position: "update_stopLoss", price: farOver, activated: true},
			{name: "neither activated, far over the cap", trade: natural, ai: ai, position: "update_stopLoss", price: farOver, rows: heldByCap(both, farOver)},
		})
	})

	t.Run("the bound off acts where it holds", func(t *testing.T) {
		var off []tableCase
		for _, stand := range tableStands {
			for _, ceiling := range []float64{unknown, pastBound, 1} {
				trade, ai := tableLadder(stand, ceiling)
				off = append(off,
					tableCase{name: fmt.Sprintf("%s, buy, ceiling %v", stand.name, ceiling), trade: trade, ai: ai, position: "stopLoss", price: within, activated: true, rows: stand.activates(fill, lossPctOf(probe, within), capPct)},
					tableCase{name: fmt.Sprintf("%s, stopLoss, ceiling %v", stand.name, ceiling), trade: restingIn(trade, "stopLoss"), ai: ai, position: "stopLoss", price: gain, reason: stand.reason},
					tableCase{name: fmt.Sprintf("%s, committed, ceiling %v", stand.name, ceiling), trade: anchoredToSell(stand.activatedOn(trade), gain), ai: ai, position: "stopLoss", price: gain, reason: stand.reason},
				)
			}
		}
		withSellLossIgnoreLastDepths(t, 0)
		runTable(t, off)
	})
	t.Run("the ceiling is the raised one: held by the stored rows, acted on with the raise", func(t *testing.T) {
		var raised []tableCase
		for _, stand := range tableStands {
			trade, ai := tableLadder(stand, pastBound)
			trade.Strategy.Params.DynamicParams, trade.Strategy.TradeType = true, aggragates.Spot
			row, event := dynamicparams.Opened{Depths: 2}.Rows(trade, trade.PositionPrice, time.Time{})
			trade = aggragates.AppendStrategyRow(trade, row, event)
			raised = append(raised,
				tableCase{name: stand.name + ", buy", trade: trade, ai: ai, position: "stopLoss", price: within, activated: true, rows: stand.activates(fill, lossPctOf(probe, within), capPct)},
				tableCase{name: stand.name + ", stopLoss", trade: restingIn(trade, "stopLoss"), ai: ai, position: "stopLoss", price: gain, reason: stand.reason},
			)
		}
		runTable(t, raised)
	})
	t.Run("a trigger taking any loss acts where the shipped one holds", func(t *testing.T) {
		var capless []tableCase
		for _, stand := range tableStands {
			trade, ai := tableLadder(stand, roomy)
			capless = append(capless,
				tableCase{name: stand.name + ", buy", trade: trade, ai: ai, position: "update_stopLoss", price: farOver, activated: true, rows: stand.activates(fill, lossPctOf(probe, farOver), anyLoss)},
				tableCase{name: stand.name + ", buy at a gain", trade: trade, ai: ai, position: "update_stopLoss", price: gain, activated: true, rows: stand.activates(fill, lossPctOf(probe, gain), anyLoss)},
				tableCase{name: stand.name + ", stopLoss", trade: restingIn(trade, "stopLoss"), ai: ai, position: "update_stopLoss", price: farOver, reason: stand.reason},
			)
		}
		withAllowedLoss(t, anyLoss)
		runTable(t, capless)
	})

	for _, stand := range tableStands {
		t.Run("a refusal retried on a later tick, then the two ticks end to end, "+stand.name, func(t *testing.T) {
			trade, ai := tableLadder(stand, roomy)
			held, got := engineStep(trade, "stopLoss", oneCentOver, testutil.At("13:00:00"), ai)
			assertWaits(t, got, "stopLoss", heldByCap(stand, oneCentOver)...)
			for _, position := range addSideProposals {
				assertUntouched(t, Apply(held, position, oneCentOver, ai), position)
			}

			activated, got := engineStep(held, "update_stopLoss", atTheCap, testutil.At("13:05:00"), ai)
			assertActivated(t, got, stand.activates(fill, lossPctOf(probe, atTheCap), capPct)...)
			if activated.PositionType != "stopLoss" {
				t.Fatalf("the activation chain leaves the ladder in stopLoss, got %q", activated.PositionType)
			}
			// The ticks after the activation, whatever the ladder proposes, trail from the anchor
			// the activation wrote (the print it was made at): that very print holds, so does a
			// print in between, a rise over the trailing take profit re-anchors and a fall under
			// the tolerance sells, a print far over the cap included.
			row := trailRow(activated)
			rise, fall := priceAtMove(activated, atTheCap, 1.5*row.TrailingTakeProfit), priceAtMove(activated, atTheCap, -1.5*row.Tolerance)
			for _, tick := range []struct {
				position string
				price    float64
				answer   saleStep
			}{
				{"buy", atTheCap, stepHold}, {"update_buy", oneCentOver, stepHold}, {"update_stopLoss", within, stepHold}, {"", atTheCap, stepHold},
				{"buy", rise, stepReanchor}, {"update_stopLoss", gain, stepReanchor},
				{"", fall, stepSell}, {"update_stopLoss", farOver, stepSell},
			} {
				got := Apply(activated, tick.position, tick.price, ai)
				switch tick.answer {
				case stepHold:
					assertTrailHeld(t, got)
				case stepReanchor:
					assertReanchored(t, got)
				default:
					assertSold(t, got, stand.reason)
				}
			}
			withAllowedLoss(t, 2*capPct)
			assertActivated(t, Apply(held, "stopLoss", oneCentOver, ai), stand.activates(fill, lossPctOf(probe, oneCentOver), 2*capPct)...)
		})
	}

	t.Run("an inverse ladder counts its SELL entries and loses on the base its entries sold", func(t *testing.T) {
		if capPct >= 100 {
			t.Skip("SellLossTriggerPercentage leaves the inverse ladder no loss under it to read")
		}
		inverse := withDepths(flatMarketLadder(true), roomy)
		inverseAI := flatMarketMarket(inverse)
		inverse = withRows(inverse, triggeredBy(inverse, inverseAI))
		inverseAverage := ladder.AverageEntryPrice(inverse)
		inverseWithin := math.Floor(inverseAverage/(1-capPct/100)*100) / 100
		inverseOver, inverseGain := inverseWithin+0.01, math.Floor(inverseAverage/gainFactor*100)/100
		past := withDepths(flatMarketLadder(true), pastBound)
		past = withRows(past, triggeredBy(past, inverseAI))
		low := tableStand{low: true, reason: reasonFlatMarket}
		runTable(t, []tableCase{
			{name: "buy, a gain", trade: inverse, ai: inverseAI, position: "stopLoss", price: inverseGain, activated: true, rows: low.activates(fill, inverseLossPctOf(inverse, inverseGain), capPct)},
			{name: "buy, within the cap", trade: inverse, ai: inverseAI, position: "stopLoss", price: inverseWithin, activated: true, rows: low.activates(fill, inverseLossPctOf(inverse, inverseWithin), capPct)},
			{name: "buy, one cent over the cap", trade: inverse, ai: inverseAI, position: "stopLoss", price: inverseOver, rows: low.owedByLoss(fill, inverseLossPctOf(inverse, inverseOver), capPct)},
			{name: "buy, past the depth bound", trade: past, ai: inverseAI, position: "update_stopLoss", price: inverseAverage / gainFactor, rows: low.owed(fill, pastText)},
			{name: "stopLoss, a gain", trade: restingIn(inverse, "stopLoss"), ai: inverseAI, position: "stopLoss", price: inverseAverage / gainFactor, reason: reasonFlatMarket},
			{name: "stopLoss, within the cap", trade: restingIn(inverse, "stopLoss"), ai: inverseAI, position: "stopLoss", price: inverseWithin, reason: reasonFlatMarket},
			{name: "stopLoss, one cent over the cap", trade: restingIn(inverse, "stopLoss"), ai: inverseAI, position: "stopLoss", price: inverseOver, rows: low.owedByLoss(fill, inverseLossPctOf(inverse, inverseOver), capPct)},
			{name: "stopLoss, past the depth bound", trade: restingIn(past, "stopLoss"), ai: inverseAI, position: "update_stopLoss", price: inverseAverage / gainFactor, rows: low.owed(fill, pastText)},
		})
	})
}

// historyExtra is rows a ladder's history carries beside its entries — and the entries they
// add, if any — built for a ladder of n entries on the entry side given and its opposite.
type historyExtra struct {
	name    string
	rows    func(n int, entry, exit string) []aggragates.TradesHistory
	entries int
}

// The depth bound counts the entries a ladder has filled, as ladder.CountFilledEntries
// counts them, and never the rows of its history: a partial fill of an entry order is one
// entry with the order, a bookkeeping row, a row that holds nothing and a close are none,
// and a row with no order id is an entry of its own. A ladder at the bound is acted on and
// one entry deeper is held, however many other rows its history carries, on a long ladder
// and an inverse one alike, from buy (the stopLoss activated) and from stopLoss (the ladder
// sold); the held row names the entries. The trigger takes any loss so that only the bound reads, and
// the bound is read through the flat market gate, the one the shipped switches leave
// standing.
func TestSellLossCountsEntriesNotHistoryRows(t *testing.T) {
	flatMarketAlone(t)
	withAllowedLoss(t, anyLoss)
	withSellLossIgnoreLastDepths(t, SellLossIgnoreLastDepths)
	bound := lastDepthFills - SellLossIgnoreLastDepths
	if SellLossIgnoreLastDepths <= 0 || bound < 2 {
		t.Skip("SellLossIgnoreLastDepths is off or leaves the w3s ladder fewer than two depths to be acted on at")
	}
	const legacyPrice = 150.5
	row := func(kind string, quantity, price float64, order int64) aggragates.TradesHistory {
		return aggragates.TradesHistory{Type: kind, Quantity: quantity, Price: price, OrderId: order, CreatedAt: testutil.At("22:00:00")}
	}
	partial := func(n int, entry, _ string) []aggragates.TradesHistory {
		return []aggragates.TradesHistory{row(entry, 0.5, 168.2, int64(n)), row(entry, 0.25, 201.46, 1)}
	}
	bookkeeping := func(_ int, entry, _ string) []aggragates.TradesHistory {
		return []aggragates.TradesHistory{row(entry, 1, ladder.AccountingPriceCeiling, 900)}
	}
	holdsNothing := func(_ int, entry, _ string) []aggragates.TradesHistory {
		return []aggragates.TradesHistory{row(entry, 0, 150, 901)}
	}
	closes := func(_ int, _, exit string) []aggragates.TradesHistory {
		return []aggragates.TradesHistory{row(exit, 0.5, 210, 902)}
	}
	extras := []historyExtra{
		{"a partial fill of an entry order", partial, 0},
		{"a bookkeeping row", bookkeeping, 0},
		{"a row that holds nothing", holdsNothing, 0},
		{"a close", closes, 0},
		{"all of them at once", func(n int, entry, exit string) []aggragates.TradesHistory {
			return slices.Concat(partial(n, entry, exit), bookkeeping(n, entry, exit), holdsNothing(n, entry, exit), closes(n, entry, exit))
		}, 0},
		{"two rows with no order id", func(_ int, entry, _ string) []aggragates.TradesHistory {
			return []aggragates.TradesHistory{row(entry, 1, legacyPrice-1, 0), row(entry, 1, legacyPrice, 0)}
		}, 2},
	}

	for _, inverse := range []bool{false, true} {
		entry, exit := "BUY", "SELL"
		price := 100.0
		if inverse {
			entry, exit = exit, entry
			price = 400.0
		}
		for _, extra := range extras {
			for _, entries := range []int{bound, bound + 1} {
				for _, state := range []string{"buy", "stopLoss"} {
					t.Run(fmt.Sprintf("inverse %v, %s, %d entries, %s", inverse, extra.name, entries, state), func(t *testing.T) {
						n := entries - extra.entries
						if n < 1 {
							t.Skip("SellLossIgnoreLastDepths leaves no ladder under this case's rows")
						}
						trade := testutil.LadderTrade(inverse, fills(n, "21:30:00")...)
						newest := trade.PositionPrice
						trade.History = append(trade.History, extra.rows(n, entry, exit)...)
						if extra.entries > 0 {
							newest = legacyPrice
						}
						ai := flatMarketMarket(trade)
						opened := FlatMarketTriggeredRow(newest, flatMarketReading(trade, ai.SmartTakeLoss, ai).Reasons)
						trade = restingIn(withRows(trade, opened), state)
						got := Apply(trade, "stopLoss", price, ai)
						switch {
						case entries == bound && state == "stopLoss":
							assertSold(t, got, reasonFlatMarket)
						case entries == bound:
							if !got.Activated || got.Position != "stopLoss" || len(got.Rows) != 1 || got.Rows[0].Event != EventActivated || got.Rows[0].Gate != GateFlatMarket {
								t.Fatalf("a ladder in buy at the bound has its stopLoss activated, got %+v", got)
							}
						default:
							assertWaits(t, got, "stopLoss", heldRowOf(lowTrigger(true), newest, atDepthText(entries, lastDepthFills, bound)))
						}
					})
				}
			}
		}
	}
}
