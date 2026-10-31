package smartsell

import (
	"fmt"
	"math"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates"
	"github.com/giovani-sirbu/mercury/strategies/gates/dynamicparams"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// The shared action (takeLoss) is asked on its own, off the state rebuildState reads, by the
// ladder's state and never the proposal: buy activates, trailingBuy sells or trails, any other state
// gets nothing. Tests set their bounds through the helpers (saleOn) and state the rows by hand.

const (
	// slowReason and lowReason are the opaque reasons the triggers carry; takeLoss hands the first back untouched.
	slowReason = "slow decline reason"
	lowReason  = "flat market reason"
	// gainFactor scales a break even into a tick price that books a gain, lossFactor into one far over any cap.
	gainFactor = 1.02
	lossFactor = 0.5
	// positionMovedBy is how far the fixtures move the position price off the newest fill, as a re-anchor does.
	positionMovedBy = 2.5
	// raisedDepths is what the opened event of a raised ladder adds to its ceiling.
	raisedDepths = 2
)

var (
	// armingProposals are the proposals that arm or re-arm a depth, as the engines hand them to Apply;
	// the action replaces them all the same.
	armingProposals = []string{"trailingBuy", "update_trailingBuy", "forceTrailingBuy"}
	// closingProposals are the ladder's own closes, which protectedPosition passes
	// through: the action never replaces one, in any state.
	closingProposals = []string{"sell", "update_takeProfit", "takeLoss", "sellParent", "impasse"}
	// takeProfitProposals are the take profits the ladder proposes, protected in every state but
	// buy, where the activation replaces them (yieldsToActivation).
	takeProfitProposals = []string{"takeProfit", "forceTrailingTakeProfit"}
	// addSideProposals are the proposals the action may replace, with an activation
	// or with a sale: none, the fill on a bounce and the three that arm a depth.
	addSideProposals = slices.Concat(flatMarketOrdinaryProposals, armingProposals)
	// actingStates are the states the action acts on (buy activates, trailingBuy and force-trailing sell);
	// inertStates every other state, where it writes no row.
	actingStates = []string{"buy", "trailingBuy", "forceTrailingBuy"}
	inertStates  = []string{"new", "takeProfit", "forceTrailingTakeProfit", "sell", "takeLoss", "impasse", "sellParent", ""}
)

// stepNames words a saleStep in a failure.
var stepNames = map[saleStep]string{stepNone: "none", stepActivate: "activate", stepActivateTakeProfit: "activate take profit", stepActivateTrailingBuy: "activate trailing buy", stepSell: "sell", stepReanchor: "reanchor", stepHold: "hold", stepGapHold: "gap hold"}

// saleCase is one decision the tables ask takeLoss for, and what it must hand back; shared is
// the tick's trigger, every row reading when nil (everyRowShared).
type saleCase struct {
	name     string
	trade    aggragates.Trades
	price    float64
	triggers []saleTrigger
	shared   *sharedTrigger
	step     saleStep
	reason   string
	rows     []Row
}

// onLadder is a case on the ladder resting in state at the price, answering nothing until a verb says what it must.
func onLadder(name string, trade aggragates.Trades, state string, price float64, triggers ...saleTrigger) saleCase {
	return saleCase{name: name, trade: restingIn(trade, state), price: price, triggers: triggers}
}

// selling is the case answering the sale with no row: the reason of the first trigger that
// activated it (committedReason), else of the first trigger.
func (c saleCase) selling() saleCase {
	c.step, c.reason = stepSell, c.triggers[0].reason
	if index := slices.IndexFunc(c.triggers, func(trigger saleTrigger) bool { return trigger.activated }); index >= 0 {
		c.reason = c.triggers[index].reason
	}
	return c
}

// activating is the case answering the activation: one row per trigger, in order, at the newest fill,
// naming the profit beside the trigger (lossPct and capPct, negated); activatingAtItsFill puts them at the position price.
func (c saleCase) activating(fill, lossPct, capPct float64) saleCase {
	c.step = stepActivateTrailingBuy
	if state, _ := activationPathOf(lossPct); state == "takeProfit" {
		c.step = stepActivateTakeProfit
	}
	for _, trigger := range c.triggers {
		c.rows = append(c.rows, activationRowOf(trigger, fill, lossPct, capPct))
	}
	return c
}

func (c saleCase) activatingAtItsFill(lossPct, capPct float64) saleCase {
	return c.activating(c.trade.PositionPrice, lossPct, capPct)
}

// goingAhead is the case where both bounds let the action through: buy is activated, a self-armed
// trailingBuy is sold; a committed one trails instead (trailCases).
func (c saleCase) goingAhead(fill, lossPct, capPct float64) saleCase {
	if gates.PositionType(c.trade.PositionType) == "trailingBuy" {
		return c.selling()
	}
	return c.activating(fill, lossPct, capPct)
}

// holding is the case answering a held action for why: nothing activated or sold, the held row of
// every trigger whose row is due, in order, at the fill; holdingAtItsFill puts it at the position price.
func (c saleCase) holding(fill float64, why string) saleCase {
	for _, trigger := range c.triggers {
		if trigger.rowDue {
			c.rows = append(c.rows, heldRowOf(trigger, fill, why))
		}
	}
	return c
}

func (c saleCase) holdingAtItsFill(why string) saleCase {
	return c.holding(c.trade.PositionPrice, why)
}

// holdingByLoss is holding for a sale the profit holds: the held row of every trigger not yet held
// under the trigger (capPct, negated), naming it; holdingByLossAtItsFill puts it at the position price.
func (c saleCase) holdingByLoss(fill, lossPct, capPct float64) saleCase {
	for _, trigger := range c.triggers {
		if !trigger.heldUnder.named || trigger.heldUnder.pct != 0-capPct {
			c.rows = append(c.rows, heldByLossRowOf(trigger, fill, lossPct, capPct))
		}
	}
	return c
}

func (c saleCase) holdingByLossAtItsFill(lossPct, capPct float64) saleCase {
	return c.holdingByLoss(c.trade.PositionPrice, lossPct, capPct)
}

// runSale asks takeLoss each case's decision on the rebuilt state and fails on any difference.
func runSale(t *testing.T, cases []saleCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			shared := everyRowShared()
			if c.shared != nil {
				shared = *c.shared
			}
			rows, step, reason, _ := sale(c.trade, rebuildState(c.trade), c.price, c.triggers, shared)
			if step != c.step {
				t.Errorf("step = %s, want %s", stepNames[step], stepNames[c.step])
			}
			if reason != c.reason {
				t.Errorf("reason = %q, want %q", reason, c.reason)
			}
			if len(rows) != len(c.rows) || (len(c.rows) > 0 && !reflect.DeepEqual(rows, c.rows)) {
				t.Errorf("rows = %+v, want exactly %+v", rows, c.rows)
			}
		})
	}
}

// slowTrigger is the slow decline gate's trigger on a pending ladder, lowTrigger the flat market
// gate's; due is whether the held row is due, one not due held under the trigger in force
// (heldUnlessDue), eachGate each on its own, activatedOf a trigger whose gate activated.
func slowTrigger(due bool) saleTrigger {
	return heldUnlessDue(saleTrigger{
		reason:   slowReason,
		gate:     GateSlowDecline,
		features: []string{FeatureQuietDecline, FeatureSwingDecline},
		rowDue:   due,
	})
}

func lowTrigger(due bool) saleTrigger {
	return heldUnlessDue(saleTrigger{reason: lowReason, gate: GateFlatMarket, rowDue: due})
}

// heldUnlessDue is the trigger with its episode's held row standing under the trigger in force
// (FullTrendTrigger) unless its row is due.
func heldUnlessDue(trigger saleTrigger) saleTrigger {
	if !trigger.rowDue {
		trigger.heldUnder = heldTrigger{pct: FullTrendTrigger(), named: true}
	}
	return trigger
}

func eachGate() []saleTrigger {
	return []saleTrigger{slowTrigger(true), lowTrigger(true)}
}

func activatedOf(trigger saleTrigger) saleTrigger {
	trigger.activated, trigger.activatedIn = true, "trailingBuy"
	return trigger
}

// standing is one way the triggers can stand on a tick: the slow decline gate's alone,
// the flat market gate's alone, or both, the slow decline gate's first.
type standing struct {
	name     string
	triggers []saleTrigger
}

func eachStanding() []standing {
	return []standing{
		{"the slow decline", []saleTrigger{slowTrigger(true)}},
		{"flat market", []saleTrigger{lowTrigger(true)}},
		{"both gates", []saleTrigger{slowTrigger(true), lowTrigger(true)}},
	}
}

// gateLabel is how a row words the gate whose trigger the action acts on.
func gateLabel(trigger saleTrigger) string {
	switch trigger.gate {
	case GateFlatMarket:
		return "flat market"
	case GateCapitalProtection:
		return "capital protection"
	}
	return "slow decline"
}

// heldRowOf is the row the trigger's gate is owed when its action is held for why, stated by hand.
func heldRowOf(trigger saleTrigger, fill float64, why string) Row {
	return Row{
		Message:  "smartSell: " + gateLabel(trigger) + " sale held, " + why,
		Price:    fill,
		Gate:     trigger.gate,
		Event:    EventSaleHeld,
		Features: trigger.features,
	}
}

// heldByLossRowOf is the row the trigger's gate is owed when the profit (lossPct, negated) holds its
// action under the trigger (capPct, negated), stated by hand.
func heldByLossRowOf(trigger saleTrigger, fill, lossPct, capPct float64) Row {
	row := heldRowOf(trigger, fill, byLossText(lossPct, capPct))
	triggerPct := 0 - capPct
	row.Trigger = &triggerPct
	return row
}

// activationRowOf is the row the trigger's gate is owed when the action activates on it at lossPct
// against capPct, stated by hand: the path by the profit's sign (activationPathOf).
func activationRowOf(trigger saleTrigger, fill, lossPct, capPct float64) Row {
	state, words := activationPathOf(lossPct)
	return Row{
		Message:  "smartSell: " + gateLabel(trigger) + " " + words + ", " + activatedText(lossPct, capPct),
		Price:    fill,
		Gate:     trigger.gate,
		Event:    EventActivated,
		Features: trigger.features,
		State:    state,
	}
}

// activatedWordsOf is the words an activation row into state names it by, stated by hand.
func activatedWordsOf(state string) string {
	if state == "takeProfit" {
		return "take profit activated"
	}
	return "take loss activated"
}

// activationPathOf is the state an activation at lossPct moves to and the words its row names it
// by, stated by hand: takeProfit at no loss, trailingBuy at a loss.
func activationPathOf(lossPct float64) (state, words string) {
	if 0-lossPct >= 0 {
		return "takeProfit", "take profit activated"
	}
	return "trailingBuy", "take loss activated"
}

// atDepthText and byLossText are the two held reasons as the rows word them; activatedText the
// activation's. Both take the loss and the allowed loss (capPct) and print them as the signed profit and trigger.
func atDepthText(filled, ceiling, bound int) string {
	return fmt.Sprintf("depth %d of %d over %d", filled, ceiling, bound)
}

func byLossText(lossPct, capPct float64) string {
	return fmt.Sprintf("profit %.2f%% of the used amount under %.2f%%", 0-lossPct, 0-capPct)
}

func activatedText(lossPct, capPct float64) string {
	return fmt.Sprintf("profit %.2f%% of the used amount at or over %.2f%%", 0-lossPct, 0-capPct)
}

// depthLadder is the w3s ladder n deep with one row configuring lastDepthFills depths; withDepths
// and withDepthRows reconfigure the depths (one row, or a row per depth) on a ladder built for the call.
func depthLadder(n int) aggragates.Trades {
	return testutil.LadderTrade(false, fills(n, "21:30:00")...)
}

func withDepths(trade aggragates.Trades, depths float64) aggragates.Trades {
	trade.StrategyPair.StrategySettings[0].Depths = depths
	return trade
}

func withDepthRows(trade aggragates.Trades, depths ...float64) aggragates.Trades {
	base := trade.StrategyPair.StrategySettings[0]
	rows := make([]aggragates.StrategySettings, len(depths))
	for index, allowed := range depths {
		rows[index] = base
		rows[index].Depths = allowed
	}
	trade.StrategyPair.StrategySettings = rows
	return trade
}

// raisedLadder is the w3s ladder n deep as the engine leaves it once opened raised: the DynamicParams
// flag on and the opened pair appended, the stored rows untouched.
func raisedLadder(n, depths int) aggragates.Trades {
	trade := depthLadder(n)
	trade.Strategy.Params.DynamicParams = true
	trade.Strategy.TradeType = aggragates.Spot
	row, event := dynamicparams.Opened{Depths: depths}.Rows(trade, trade.PositionPrice, time.Time{})
	return aggragates.AppendStrategyRow(trade, row, event)
}

// saleOn runs the test with both bounds of the shared action on at their shipped values.
func saleOn(t *testing.T) {
	t.Helper()
	withSaleIgnoreLastDepths(t, SaleIgnoreLastDepths)
	withAllowedLoss(t, shippedLoss)
}

// sellingDepth is the deepest the w3s ladder is acted on under the bound in force (the last depth
// where it is off); it skips the test when SaleIgnoreLastDepths leaves no depth.
func sellingDepth(t *testing.T) int {
	t.Helper()
	depth := min(lastDepthFills, lastDepthFills-saleIgnoreLastDepths)
	if depth < 1 {
		t.Skip("SaleIgnoreLastDepths leaves the w3s ladder no depth to be acted on at")
	}
	return depth
}

// salePrices are the prices at which the round ladder loses exactly the cap in force and one
// cent more, exact to the bit; it skips the test when the trigger takes no loss or names no price above zero.
func salePrices(t *testing.T, trade aggragates.Trades) (atTheCap, oneStepLower float64) {
	t.Helper()
	used, units := ladder.EntryNotional(trade), ladder.EntryQuantity(trade)
	cents := math.Round((used - used*allowedLoss()/100) / units * 100)
	atTheCap, oneStepLower = cents/100, (cents-1)/100
	loss, lowerLoss := used-float64(units*atTheCap), used-float64(units*oneStepLower)
	if allowedLoss() <= 0 || oneStepLower <= 0 || loss*100 != allowedLoss()*used || lowerLoss*100 <= allowedLoss()*used {
		t.Skip("SaleTriggerPercentage takes no loss or names no price above zero on the cent grid the round ladder loses exactly that much at")
	}
	return atTheCap, oneStepLower
}

// capPrices are the prices a ladder is read at, stated from its totals: a gain, break even, a loss
// within the cap, exactly at it, one step over and far over.
type capPrices struct {
	gain, breakEven, within, atTheCap, over, far float64
}

// roundLadderPrices is capPrices of the round ladder under the cap in force, on the
// grid where its loss is exact (salePrices).
func roundLadderPrices(t *testing.T, trade aggragates.Trades) capPrices {
	t.Helper()
	atTheCap, oneStepLower := salePrices(t, trade)
	average := ladder.AverageEntryPrice(trade)
	return capPrices{
		gain: average * gainFactor, breakEven: average, within: (atTheCap + average) / 2,
		atTheCap: atTheCap, over: oneStepLower, far: average * lossFactor,
	}
}

// farOverTheCap skips the test when capPct is off or so wide that lossFactor's price is not over it.
func farOverTheCap(t *testing.T, capPct float64) {
	t.Helper()
	if capPct <= 0 || capPct >= 100*(1-lossFactor) {
		t.Skip("SaleTriggerPercentage takes no loss or wide enough to let the far-over price be acted on")
	}
}

// aLossWithinTheCap is the first cent under the ladder's break even, a real loss within the cap;
// it skips the test when the cap is narrower than that.
func aLossWithinTheCap(t *testing.T, trade aggragates.Trades) float64 {
	t.Helper()
	price := math.Floor(ladder.AverageEntryPrice(trade)*100) / 100
	for handLossPct(trade, price) <= 0 {
		price = math.Round((price-0.01)*100) / 100
	}
	if loss := handLossPct(trade, price); allowedLoss() > 0 && loss > allowedLoss() {
		t.Skip("SaleTriggerPercentage is narrower than the loss of the first cent under the break even")
	}
	return price
}

// lossPctOf is the loss, as a share of what a long ladder used, of selling all it holds at price on
// the cent grid, stated from the totals; inverseLossPctOf the same for an inverse ladder, a share of its base.
func lossPctOf(trade aggragates.Trades, price float64) float64 {
	used := ladder.EntryNotional(trade)
	return (used - ladder.EntryQuantity(trade)*helpers.ToFixed(price, 2)) / used * 100
}

func inverseLossPctOf(trade aggragates.Trades, price float64) float64 {
	return (1 - ladder.AverageEntryPrice(trade)/helpers.ToFixed(price, 2)) * 100
}

// With no trigger standing the action answers nothing in any state, and in a state it does not act
// on it answers nothing with the triggers standing too, no row included; the acting states are the control.
func TestSaleAnswersNothingWithoutATriggerOrInAStateItDoesNotActOn(t *testing.T) {
	withSaleIgnoreLastDepths(t, 1)
	withAllowedLoss(t, 0.5)
	trade := lastDepthLadder()
	average := ladder.AverageEntryPrice(trade)
	prices := []float64{average * gainFactor, average * lossFactor}

	var cases []saleCase
	for _, state := range slices.Concat(actingStates, inertStates) {
		for _, price := range prices {
			cases = append(cases, onLadder(fmt.Sprintf("no trigger, %q at %v", state, price), trade, state, price))
		}
	}
	for _, state := range inertStates {
		for _, price := range prices {
			cases = append(cases, onLadder(fmt.Sprintf("both triggers, %q at %v", state, price), trade, state, price, eachGate()...))
		}
	}
	for _, state := range actingStates {
		for _, price := range prices {
			cases = append(cases, onLadder(fmt.Sprintf("both triggers, %q at %v: control", state, price), trade, state, price, eachGate()...).
				holdingAtItsFill(atDepthText(lastDepthFills, lastDepthFills, lastDepthFills-1)))
		}
	}
	runSale(t, cases)
}

// A ladder in buy is activated on a profit at or over the trigger — a gain, break even, a loss the
// trigger takes, exactly at it included — and held one step under it, the row naming the profit and
// the trigger. The activation writes a row per trigger, the held row only for the triggers whose row is due.
func TestSaleActivatesALadderInBuyAtOrOverTheTriggerAndHoldsItUnder(t *testing.T) {
	trade := flatMarketLadder(false)
	fill := trade.PositionPrice
	for _, capPct := range []float64{shippedLoss, 2 * shippedLoss} {
		t.Run(fmt.Sprintf("cap %v", capPct), func(t *testing.T) {
			withSaleIgnoreLastDepths(t, 0)
			withAllowedLoss(t, capPct)
			farOverTheCap(t, capPct)
			p := roundLadderPrices(t, trade)

			var cases []saleCase
			for _, stand := range eachStanding() {
				name := func(what string) string { return stand.name + ", " + what }
				cases = append(cases,
					onLadder(name("a gain"), trade, "buy", p.gain, stand.triggers...).activating(fill, lossPctOf(trade, p.gain), capPct),
					onLadder(name("the break even"), trade, "buy", p.breakEven, stand.triggers...).activating(fill, lossPctOf(trade, p.breakEven), capPct),
					onLadder(name("within the cap"), trade, "buy", p.within, stand.triggers...).activating(fill, lossPctOf(trade, p.within), capPct),
					onLadder(name("exactly at the cap"), trade, "buy", p.atTheCap, stand.triggers...).activating(fill, lossPctOf(trade, p.atTheCap), capPct),
					onLadder(name("one price step over the cap"), trade, "buy", p.over, stand.triggers...).holdingByLoss(fill, lossPctOf(trade, p.over), capPct),
					onLadder(name("far over the cap"), trade, "buy", p.far, stand.triggers...).holdingByLoss(fill, lossPctOf(trade, p.far), capPct),
				)
			}
			runSale(t, cases)
		})
	}
}

// A self-armed trailingBuy is sold at a gain, at break even and on a loss the trigger takes, exactly at
// it included, and held under the trigger with the gates' held rows; the force-trailing state reads as trailingBuy.
func TestSaleSellsALadderThatArmedItselfOnNoLossAndWithinTheCapAndHoldsItOverIt(t *testing.T) {
	trade := flatMarketLadder(false)
	fill := trade.PositionPrice
	for _, capPct := range []float64{shippedLoss, 2 * shippedLoss} {
		t.Run(fmt.Sprintf("cap %v", capPct), func(t *testing.T) {
			withSaleIgnoreLastDepths(t, 0)
			withAllowedLoss(t, capPct)
			farOverTheCap(t, capPct)
			p := roundLadderPrices(t, trade)

			var cases []saleCase
			for _, state := range []string{"trailingBuy", "forceTrailingBuy"} {
				for _, stand := range eachStanding() {
					name := func(what string) string { return state + ", " + stand.name + ", " + what }
					cases = append(cases,
						onLadder(name("a gain"), trade, state, p.gain, stand.triggers...).selling(),
						onLadder(name("the break even"), trade, state, p.breakEven, stand.triggers...).selling(),
						onLadder(name("within the cap"), trade, state, p.within, stand.triggers...).selling(),
						onLadder(name("exactly at the cap"), trade, state, p.atTheCap, stand.triggers...).selling(),
						onLadder(name("one price step over the cap"), trade, state, p.over, stand.triggers...).holdingByLoss(fill, lossPctOf(trade, p.over), capPct),
						onLadder(name("far over the cap"), trade, state, p.far, stand.triggers...).holdingByLoss(fill, lossPctOf(trade, p.far), capPct),
					)
				}
			}
			runSale(t, cases)
		})
	}
}

// Once a gate activated the trailingBuy the sale is committed and the action trails from the anchor by
// the held depth's row: held between the tolerance and the trailing take profit, re-anchored over,
// sold under, with no row and whatever the loss or gain; break even and the cap are not asked.
func TestSaleTrailsACommittedLadderFromItsAnchorWhateverTheLoss(t *testing.T) {
	withSaleIgnoreLastDepths(t, 0)
	withAllowedLoss(t, shippedLoss)
	farOverTheCap(t, shippedLoss)
	trade := flatMarketLadder(false)
	p := roundLadderPrices(t, trade)

	var cases []saleCase
	for _, state := range []string{"trailingBuy", "forceTrailingBuy"} {
		for _, trigger := range eachGate() {
			committed := activatedOf(trigger)
			for name, price := range map[string]float64{
				"a gain": p.gain, "the break even": p.breakEven, "within the cap": p.within, "exactly at the cap": p.atTheCap,
				"one price step over the cap": p.over, "far over the cap": p.far,
			} {
				cases = append(cases, trailCases(state+", "+trigger.gate+" activated, "+name, trade, state, price, committed)...)
			}
		}
	}
	runSale(t, cases)
}

// Any standing trigger that activated commits the sale, named by the first trigger that activated
// it; with neither activated the same price is held with a row per gate whose row is due.
func TestSaleASaleIsCommittedByAnyActivatedTriggerAndNamedByTheTriggerThatActivated(t *testing.T) {
	withSaleIgnoreLastDepths(t, 0)
	withAllowedLoss(t, shippedLoss)
	farOverTheCap(t, shippedLoss)
	trade := flatMarketLadder(false)
	price := ladder.AverageEntryPrice(trade) * lossFactor
	loss := lossPctOf(trade, price)

	slow, low := slowTrigger(true), lowTrigger(true)
	for _, c := range []struct {
		name     string
		triggers []saleTrigger
		sells    bool
		reason   string
	}{
		{"the slow decline gate activated", []saleTrigger{activatedOf(slow), low}, true, slowReason},
		{"the flat market gate activated", []saleTrigger{slow, activatedOf(low)}, true, lowReason},
		{"both activated", []saleTrigger{activatedOf(slow), activatedOf(low)}, true, slowReason},
		{"the flat market gate first, the slow decline gate activated", []saleTrigger{low, activatedOf(slow)}, true, slowReason},
		{"neither activated", []saleTrigger{slow, low}, false, ""},
	} {
		sale := onLadder(c.name, trade, "trailingBuy", price, c.triggers...)
		if c.sells {
			sale = onLadder(c.name, anchoredToSell(trade, price), "trailingBuy", price, c.triggers...)
			sale.step, sale.reason = stepSell, c.reason
		} else {
			sale = sale.holdingByLossAtItsFill(loss, shippedLoss)
		}
		runSale(t, []saleCase{sale})
	}
}

// The activation is read in the state trailingBuy alone: a ladder back in buy is a ladder in buy,
// activated at a gain and on a loss the trigger takes, held under it.
func TestSaleReadsNoActivationInTheStateBuy(t *testing.T) {
	withSaleIgnoreLastDepths(t, 0)
	withAllowedLoss(t, shippedLoss)
	farOverTheCap(t, shippedLoss)
	trade := flatMarketLadder(false)
	fill := trade.PositionPrice
	p := roundLadderPrices(t, trade)

	var cases []saleCase
	for _, trigger := range eachGate() {
		stale := activatedOf(trigger)
		cases = append(cases,
			onLadder(trigger.gate+", a gain", trade, "buy", p.gain, stale).activating(fill, lossPctOf(trade, p.gain), shippedLoss),
			onLadder(trigger.gate+", within the cap", trade, "buy", p.within, stale).activating(fill, lossPctOf(trade, p.within), shippedLoss),
			onLadder(trigger.gate+", far over the cap", trade, "buy", p.far, stale).holdingByLoss(fill, lossPctOf(trade, p.far), shippedLoss),
		)
	}
	runSale(t, cases)
}

// The depth bound: a ladder is acted on while its filled entries are at most the ceiling less
// SaleIgnoreLastDepths and held past that in every acting state, the bound read before the
// commitment; the held row names the depth, the ceiling and the deepest depth, at the newest fill.
func TestSaleActsUpToTheDepthBoundAndHoldsPastIt(t *testing.T) {
	saleOn(t)
	bound := lastDepthFills - SaleIgnoreLastDepths
	if SaleIgnoreLastDepths <= 0 || bound < 2 {
		t.Skip("SaleIgnoreLastDepths is off or leaves the w3s ladder fewer than two depths to be acted on at")
	}

	var cases []saleCase
	for _, state := range actingStates {
		for _, trigger := range eachGate() {
			for _, committed := range []bool{false, true} {
				if committed && state == "buy" {
					continue
				}
				who := trigger
				if committed {
					who = activatedOf(trigger)
				}
				for _, depth := range []struct {
					name   string
					filled int
				}{
					{"one under the bound", bound - 1},
					{"exactly at the bound", bound},
					{"one over the bound", bound + 1},
					{"the last depth", lastDepthFills},
				} {
					trade := depthLadder(depth.filled)
					newest := trade.PositionPrice
					trade.PositionPrice = newest + positionMovedBy
					price := aLossWithinTheCap(t, trade)
					name := fmt.Sprintf("%s, %s, committed %v, %s", state, trigger.gate, committed, depth.name)
					c := onLadder(name, trade, state, price, who)
					switch {
					case depth.filled > bound:
						cases = append(cases, c.holding(newest, atDepthText(depth.filled, lastDepthFills, bound)))
					case committed:
						cases = append(cases, trailCases(name, trade, state, price, who)...)
					default:
						cases = append(cases, c.goingAhead(newest, lossPctOf(trade, price), shippedLoss))
					}
				}
			}
		}
	}
	runSale(t, cases)
}

// An unknown ceiling, or one no deeper than the bound, holds the action at any depth and the row
// says so; a ceiling one over the bound lets its first fill through only.
func TestSaleHoldsALadderWithNoKnownCeilingOrNoRoomUnderTheBound(t *testing.T) {
	saleOn(t)
	if SaleIgnoreLastDepths <= 0 {
		t.Skip("SaleIgnoreLastDepths is off: no bound holds a ladder")
	}
	unknown := withDepths(depthLadder(3), 0.5)
	noRow := depthLadder(3)
	noRow.StrategyPair.StrategySettings = nil
	atTheBound := withDepths(depthLadder(1), SaleIgnoreLastDepths)
	oneOver := withDepths(depthLadder(1), SaleIgnoreLastDepths+1)
	oneOverDeeper := withDepths(depthLadder(2), SaleIgnoreLastDepths+1)

	var cases []saleCase
	for _, state := range []string{"buy", "trailingBuy"} {
		for _, trigger := range eachGate() {
			ahead := func(name string, trade aggragates.Trades) saleCase {
				price := aLossWithinTheCap(t, trade)
				return onLadder(state+", "+trigger.gate+", "+name, trade, state, price, trigger).goingAhead(trade.PositionPrice, lossPctOf(trade, price), shippedLoss)
			}
			held := func(name string, trade aggragates.Trades, why string) saleCase {
				return onLadder(state+", "+trigger.gate+", "+name, trade, state, aLossWithinTheCap(t, trade), trigger).holdingAtItsFill(why)
			}
			cases = append(cases,
				held("a row under one depth", unknown, "depth 3 of no known ceiling"),
				held("no settings row", noRow, "depth 3 of no known ceiling"),
				held("a ceiling at the bound", atTheBound, atDepthText(1, SaleIgnoreLastDepths, 0)),
				ahead("a ceiling one over the bound, its first fill", oneOver),
				held("a ceiling one over the bound, its second fill", oneOverDeeper, atDepthText(2, SaleIgnoreLastDepths+1, 1)),
			)
		}
	}
	runSale(t, cases)
}

// At zero or below the bound is off and a ladder is acted on at any depth; the same ladders are held with it on.
func TestSaleBoundAtZeroOrBelowIsOff(t *testing.T) {
	saleOn(t)
	if SaleIgnoreLastDepths <= 0 {
		t.Skip("SaleIgnoreLastDepths is off: there is no bound to compare with")
	}
	last := lastDepthLadder()
	unknown := withDepths(depthLadder(3), 0.5)

	var held, ahead []saleCase
	for _, state := range []string{"buy", "trailingBuy"} {
		for _, trigger := range eachGate() {
			lastPrice, unknownPrice := aLossWithinTheCap(t, last), aLossWithinTheCap(t, unknown)
			lastDepth := onLadder(state+", "+trigger.gate+", the last depth", last, state, lastPrice, trigger)
			noCeiling := onLadder(state+", "+trigger.gate+", no known ceiling", unknown, state, unknownPrice, trigger)
			held = append(held,
				lastDepth.holdingAtItsFill(atDepthText(lastDepthFills, lastDepthFills, lastDepthFills-SaleIgnoreLastDepths)),
				noCeiling.holdingAtItsFill("depth 3 of no known ceiling"),
			)
			ahead = append(ahead,
				lastDepth.goingAhead(last.PositionPrice, lossPctOf(last, lastPrice), shippedLoss),
				noCeiling.goingAhead(unknown.PositionPrice, lossPctOf(unknown, unknownPrice), shippedLoss),
			)
		}
	}
	runSale(t, held)
	for _, off := range []int{0, -SaleIgnoreLastDepths} {
		withSaleIgnoreLastDepths(t, off)
		runSale(t, ahead)
	}
}

// The ceiling is the raised one: a ladder that opened with extra depths is held at the bound its own ceiling sets.
func TestSaleReadsTheCeilingTheLadderWasRaisedTo(t *testing.T) {
	saleOn(t)
	atBound := lastDepthFills - SaleIgnoreLastDepths
	if SaleIgnoreLastDepths <= 0 || atBound < 1 {
		t.Skip("SaleIgnoreLastDepths is off or leaves the w3s ladder no depth to be acted on at")
	}

	var cases []saleCase
	for _, state := range []string{"buy", "trailingBuy"} {
		for _, trigger := range eachGate() {
			ahead := func(name string, trade aggragates.Trades) saleCase {
				price := aLossWithinTheCap(t, trade)
				return onLadder(state+", "+trigger.gate+", "+name, trade, state, price, trigger).goingAhead(trade.PositionPrice, lossPctOf(trade, price), shippedLoss)
			}
			held := func(name string, trade aggragates.Trades, why string) saleCase {
				return onLadder(state+", "+trigger.gate+", "+name, trade, state, aLossWithinTheCap(t, trade), trigger).holdingAtItsFill(why)
			}
			cases = append(cases,
				held("the stored rows hold it", depthLadder(atBound+1), atDepthText(atBound+1, lastDepthFills, atBound)),
				ahead("the raised ceiling lets it through there", raisedLadder(atBound+1, raisedDepths)),
				ahead("the raised ceiling lets it through at its bound", raisedLadder(atBound+raisedDepths, raisedDepths)),
			)
			if atBound+raisedDepths+1 > lastDepthFills {
				continue
			}
			over := raisedLadder(atBound+raisedDepths+1, raisedDepths)
			cases = append(cases, held("the raised ceiling holds it one over its bound", over, atDepthText(atBound+raisedDepths+1, lastDepthFills+raisedDepths, atBound+raisedDepths)))
		}
	}
	runSale(t, cases)
}

// With a row per depth the ceiling is the row of the next fill, not of the newest; past the rows the base row governs.
func TestSaleReadsTheRowOfTheNextFill(t *testing.T) {
	saleOn(t)
	if SaleIgnoreLastDepths <= 0 {
		t.Skip("SaleIgnoreLastDepths is off: no row holds a ladder")
	}
	const filled = 4
	roomy := float64(filled + SaleIgnoreLastDepths)
	tight := roomy - 1

	var cases []saleCase
	for _, state := range []string{"buy", "trailingBuy"} {
		for _, trigger := range eachGate() {
			holds := withDepthRows(depthLadder(filled), 8, 8, 8, roomy+5, tight)
			sells := withDepthRows(depthLadder(filled), 8, 8, 8, tight-1, roomy)
			pastTheRows := withDepthRows(depthLadder(filled), roomy, tight-1, tight-1)
			why := atDepthText(filled, int(tight), int(tight)-SaleIgnoreLastDepths)
			price := aLossWithinTheCap(t, holds)
			loss := lossPctOf(holds, price)
			cases = append(cases,
				onLadder(state+", "+trigger.gate+", the next row holds it", holds, state, price, trigger).holdingAtItsFill(why),
				onLadder(state+", "+trigger.gate+", the next row lets it through", sells, state, price, trigger).goingAhead(holds.PositionPrice, loss, shippedLoss),
				onLadder(state+", "+trigger.gate+", past the rows the base row lets it through", pastTheRows, state, price, trigger).goingAhead(holds.PositionPrice, loss, shippedLoss),
			)
		}
	}
	runSale(t, cases)
}

// The shipped trigger holds a loss far under it; one at zero holds every loss and acts from break
// even, one at a profit holds until that profit: buy is activated and a self-armed trailingBuy sold once
// the profit reaches the trigger, both held under it with the row naming the profit and the trigger.
func TestSaleTriggerAtZeroOrAProfitWaitsForIt(t *testing.T) {
	farOverTheCap(t, shippedLoss)
	withSaleIgnoreLastDepths(t, 0)
	trade := flatMarketLadder(false)
	fill := trade.PositionPrice
	average := ladder.AverageEntryPrice(trade)
	far, gain := average*lossFactor, average*gainFactor
	for _, tc := range []struct {
		allowed     float64
		held, ahead []float64
	}{
		{shippedLoss, []float64{far}, []float64{average, gain}},
		{0, []float64{far, average - 0.01}, []float64{average, gain}},
		{-1, []float64{far, average}, []float64{gain}},
	} {
		withAllowedLoss(t, tc.allowed)
		var cases []saleCase
		for _, state := range []string{"buy", "trailingBuy"} {
			for _, trigger := range eachGate() {
				name := func(what string, price float64) string {
					return fmt.Sprintf("trigger %v, %s, %s, %s at %v", 0-tc.allowed, state, trigger.gate, what, price)
				}
				for _, price := range tc.held {
					cases = append(cases, onLadder(name("held", price), trade, state, price, trigger).holdingByLossAtItsFill(lossPctOf(trade, price), tc.allowed))
				}
				for _, price := range tc.ahead {
					cases = append(cases, onLadder(name("ahead", price), trade, state, price, trigger).goingAhead(fill, lossPctOf(trade, price), tc.allowed))
				}
			}
		}
		runSale(t, cases)
	}
}

// The depth bound is read first and the close simulated only past it: over both bounds the depth
// row is written, within the bound and over the cap the loss row.
func TestSaleNamesTheDepthBoundBeforeTheCap(t *testing.T) {
	if SaleIgnoreLastDepths <= 0 || shippedLoss <= 0 {
		t.Skip("SaleIgnoreLastDepths is off or SaleTriggerPercentage takes no loss: there are not two bounds to be read in order")
	}
	farOverTheCap(t, shippedLoss)
	saleOn(t)
	depth := sellingDepth(t)
	within, past := depthLadder(depth), depthLadder(depth+1)
	farOver := func(trade aggragates.Trades) float64 {
		return math.Floor(ladder.AverageEntryPrice(trade)*lossFactor*100) / 100
	}

	var cases []saleCase
	for _, state := range []string{"buy", "trailingBuy"} {
		for _, trigger := range eachGate() {
			pastTheBound := onLadder(state+", "+trigger.gate+", past the depth bound and over the cap", past, state, farOver(past), trigger)
			overTheCap := onLadder(state+", "+trigger.gate+", within the depth bound and over the cap", within, state, farOver(within), trigger)
			cases = append(cases,
				pastTheBound.holdingAtItsFill(atDepthText(depth+1, lastDepthFills, depth)),
				overTheCap.holdingByLossAtItsFill(lossPctOf(within, farOver(within)), shippedLoss),
			)
		}
	}
	runSale(t, cases)
}

// Every trigger whose row is due is owed its held row, in trigger order, and one not due none; a
// held action activates and sells nothing. The activation row is written for every trigger, due or not.
func TestSaleWritesTheDueRowsInTriggerOrder(t *testing.T) {
	if SaleIgnoreLastDepths <= 0 || shippedLoss <= 0 {
		t.Skip("SaleIgnoreLastDepths is off or SaleTriggerPercentage takes no loss: nothing is held")
	}
	farOverTheCap(t, shippedLoss)
	saleOn(t)
	last := lastDepthLadder()
	round := flatMarketLadder(false)
	roundPrice := ladder.AverageEntryPrice(round) * lossFactor

	slow, low := slowTrigger(true), lowTrigger(true)
	sets := []struct {
		name     string
		triggers []saleTrigger
		owed     []saleTrigger
	}{
		{"both due, the slow decline first", []saleTrigger{slow, low}, []saleTrigger{slow, low}},
		{"both due, the flat market first", []saleTrigger{low, slow}, []saleTrigger{low, slow}},
		{"only the slow decline's row due", []saleTrigger{slowTrigger(true), lowTrigger(false)}, []saleTrigger{slow}},
		{"only the flat market's row due", []saleTrigger{slowTrigger(false), lowTrigger(true)}, []saleTrigger{low}},
		{"no row due", []saleTrigger{slowTrigger(false), lowTrigger(false)}, nil},
	}
	refusals := []struct {
		name         string
		trade        aggragates.Trades
		price        float64
		why          string
		minDepthsOff bool
	}{
		{"the depth bound", last, ladder.AverageEntryPrice(last) * gainFactor, atDepthText(lastDepthFills, lastDepthFills, lastDepthFills-SaleIgnoreLastDepths), false},
		{"the cap", round, roundPrice, byLossText(lossPctOf(round, roundPrice), shippedLoss), true},
	}
	for _, refusal := range refusals {
		if refusal.minDepthsOff {
			withSaleIgnoreLastDepths(t, 0)
		}
		var cases []saleCase
		for _, state := range []string{"buy", "trailingBuy"} {
			for _, set := range sets {
				c := onLadder(refusal.name+", "+state+", "+set.name, refusal.trade, state, refusal.price, set.triggers...)
				for _, trigger := range set.owed {
					row := heldRowOf(trigger, refusal.trade.PositionPrice, refusal.why)
					if refusal.minDepthsOff {
						row = heldByLossRowOf(trigger, refusal.trade.PositionPrice, lossPctOf(round, roundPrice), shippedLoss)
					}
					c.rows = append(c.rows, row)
				}
				cases = append(cases, c)
			}
		}
		runSale(t, cases)
	}

	withSaleIgnoreLastDepths(t, 0)
	price := aLossWithinTheCap(t, round)
	loss := lossPctOf(round, price)
	var cases []saleCase
	for _, set := range sets {
		cases = append(cases, onLadder("an activation, "+set.name, round, "buy", price, set.triggers...).activatingAtItsFill(loss, shippedLoss))
	}
	runSale(t, cases)
}

// An inverse ladder is read on the base its entries sold (ladder.EntryQuantity): a price under its
// break even is a gain, and the depth bound counts its SELL entries.
func TestSaleCountsAnInverseLaddersLossOnTheBaseItsEntriesSold(t *testing.T) {
	if shippedLoss <= 0 || shippedLoss >= 100 || SaleIgnoreLastDepths <= 0 {
		t.Skip("SaleTriggerPercentage takes no loss, or SaleIgnoreLastDepths is off or leaves the inverse ladder nothing to be read at")
	}
	saleOn(t)
	withSaleIgnoreLastDepths(t, 0)
	inverse := flatMarketLadder(true)
	fill := inverse.PositionPrice
	average := ladder.AverageEntryPrice(inverse)
	within := math.Floor(average/(1-allowedLoss()/100)*100) / 100
	over, gain := within+0.01, math.Floor(average/gainFactor*100)/100
	var cases []saleCase
	for _, state := range []string{"buy", "trailingBuy"} {
		for _, trigger := range eachGate() {
			cases = append(cases,
				onLadder(state+", "+trigger.gate+", a gain", inverse, state, gain, trigger).goingAhead(fill, inverseLossPctOf(inverse, gain), shippedLoss),
				onLadder(state+", "+trigger.gate+", within the cap", inverse, state, within, trigger).goingAhead(fill, inverseLossPctOf(inverse, within), shippedLoss),
				onLadder(state+", "+trigger.gate+", one cent over the cap", inverse, state, over, trigger).holdingByLossAtItsFill(inverseLossPctOf(inverse, over), shippedLoss),
			)
		}
	}
	runSale(t, cases)

	withSaleIgnoreLastDepths(t, SaleIgnoreLastDepths)
	last := testutil.LadderTrade(true, fills(lastDepthFills, "21:30:00")...)
	cases = nil
	for _, state := range []string{"buy", "trailingBuy"} {
		for _, trigger := range eachGate() {
			belowBreakEven := onLadder(state+", "+trigger.gate+", the last depth", last, state, ladder.AverageEntryPrice(last)/gainFactor, trigger)
			cases = append(cases, belowBreakEven.holdingAtItsFill(atDepthText(lastDepthFills, lastDepthFills, lastDepthFills-SaleIgnoreLastDepths)))
		}
	}
	runSale(t, cases)
}

// The two depth rows are the grammar of the held rows for either gate: the entries, the ceiling and
// the deepest depth (or "no known ceiling"), at the newest fill, filed under the gate as a held sale.
func TestSaleDepthRowsAreTheHeldRowGrammarOfEachGate(t *testing.T) {
	withSaleIgnoreLastDepths(t, SaleIgnoreLastDepths)
	const fill = 168.2
	bound := lastDepthFills - SaleIgnoreLastDepths
	features := []string{FeatureQuietDecline, FeatureSwingDecline}

	slow := SaleHeldAtDepthRow(fill, bound+1, lastDepthFills, features...)
	if want := heldRowOf(slowTrigger(true), fill, atDepthText(bound+1, lastDepthFills, bound)); !reflect.DeepEqual(slow, want) {
		t.Errorf("slow decline row = %+v, want %+v", slow, want)
	}
	features[0] = "changed after the row was built"
	if slow.Features[0] != FeatureQuietDecline {
		t.Error("the slow decline row must carry its own copy of the features")
	}
	if got := SaleHeldAtDepthRow(fill, 3, 0); got.Features != nil || got.Message != "smartSell: slow decline sale held, depth 3 of no known ceiling" {
		t.Errorf("slow decline row without a ceiling or features = %+v", got)
	}

	low := FlatMarketSaleHeldAtDepthRow(fill, bound+1, lastDepthFills)
	if want := heldRowOf(lowTrigger(true), fill, atDepthText(bound+1, lastDepthFills, bound)); !reflect.DeepEqual(low, want) {
		t.Errorf("flat market row = %+v, want %+v", low, want)
	}
	if got := FlatMarketSaleHeldAtDepthRow(fill, 3, 0); got.Message != "smartSell: flat market sale held, depth 3 of no known ceiling" {
		t.Errorf("flat market row without a ceiling = %q", got.Message)
	}
}

// The activation rows are the grammar of an activated trailingBuy, one per gate, naming the profit and
// the trigger at the newest fill; the reset row of the flat market gate is filed at the
// new fill with no feature. The documents they write are the kind, the price and the features.
func TestTheActivationAndTheResetRowsAreTheGrammarOfTheirGate(t *testing.T) {
	const fill = 168.2
	features := []string{FeatureQuietDecline, FeatureSwingDecline}
	at := testutil.At("21:45:00")
	trade := depthLadder(4)

	for _, capPct := range []float64{0.5, 1.25, 0, -1} {
		withAllowedLoss(t, capPct)
		slow := ActivatedRow(fill, -0.25, features...)
		if want := activationRowOf(slowTrigger(true), fill, 0.25, capPct); !reflect.DeepEqual(slow, want) {
			t.Errorf("cap %v: slow decline row = %+v, want %+v", capPct, slow, want)
		}
		if low, want := FlatMarketActivatedRow(fill, -0.25), activationRowOf(lowTrigger(true), fill, 0.25, capPct); !reflect.DeepEqual(low, want) {
			t.Errorf("cap %v: flat market row = %+v, want %+v", capPct, low, want)
		}
	}

	withAllowedLoss(t, 0.5)
	slow := ActivatedRow(fill, -0.25, features...)
	features[0] = "changed after the row was built"
	if slow.Features[0] != FeatureQuietDecline {
		t.Error("the slow decline row must carry its own copy of the features")
	}
	if got := ActivatedRow(fill, -0.25); got.Features != nil {
		t.Errorf("a row of no features carries none, got %+v", got.Features)
	}
	if got := FlatMarketActivatedRow(fill, -0.25); got.Features != nil || got.Reasons != nil {
		t.Errorf("the flat market row carries no feature and no reason, got %+v", got)
	}

	reset := FlatMarketResetRow(fill)
	wantReset := Row{Message: "smartSell: flat market reset at the new depth", Price: fill, Gate: GateFlatMarket, Event: EventReset}
	if !reflect.DeepEqual(reset, wantReset) {
		t.Errorf("reset row = %+v, want %+v", reset, wantReset)
	}

	for name, tc := range map[string]struct {
		row      Row
		gate     string
		kind     string
		document string
	}{
		"the slow decline activation": {ActivatedRow(fill, -0.25, FeatureQuietDecline, FeatureSwingDecline), GateSlowDecline, EventActivated, `{"event":"activated","price":168.2,"features":["quietDecline","swingDecline"],"state":"trailingBuy"}`},
		"the flat market activation":  {FlatMarketActivatedRow(fill, -0.25), GateFlatMarket, EventActivated, `{"event":"activated","price":168.2,"state":"trailingBuy"}`},
		"the flat market reset":       {reset, GateFlatMarket, EventReset, `{"event":"reset","price":168.2}`},
	} {
		logRow, event := Rows(trade, tc.row, at)
		if logRow.Message != tc.row.Message || logRow.Price != fill || event.Gate != tc.gate || event.Kind() != tc.kind || string(event.Data) != tc.document {
			t.Errorf("%s: the pair is the row and its event, got %+v and %+v / %s", name, logRow, event, event.Data)
		}
	}
}

// ActivationActions is the chain every engine runs when Result.Activated: updateTrade alone, a fresh slice per call.
func TestActivationActionsAreTheUpdateTradeChainAndAFreshSliceOnEveryCall(t *testing.T) {
	chain := ActivationActions()
	if !slices.Equal(chain, []string{"updateTrade"}) {
		t.Fatalf("the activation chain is updateTrade alone, got %q", chain)
	}
	chain[0] = "changed by a caller"
	_ = append(chain, "extended by a caller")
	if again := ActivationActions(); !slices.Equal(again, []string{"updateTrade"}) {
		t.Fatalf("a caller's write must not reach the next call, got %q", again)
	}
}

// The depth rows fold as the rows of a sale the cap holds, each into its own gate's
// flag: one flag per gate covers both reasons, so the episode's row is written once.
func TestSaleDepthRowsFoldAsTheSaleHeldOfTheirGate(t *testing.T) {
	withQuietDecline(t, true)
	withFlatMarket(t, true)
	trade := depthLadder(4)
	fill := trade.PositionPrice
	at := testutil.At("21:45:00")

	slow := rebuildState(withRow(trade, SaleHeldAtDepthRow(fill, 6, 8, FeatureQuietDecline), at))
	if !slow.saleHeld || slow.saleHeldRowDue() {
		t.Errorf("the slow decline depth row must fold as the sale held, got %+v", slow)
	}
	low := rebuildState(withRow(trade, FlatMarketSaleHeldAtDepthRow(fill, 6, 8), at))
	if !low.flatMarketActive || !low.flatMarketSaleHeld {
		t.Errorf("the flat market depth row must fold as the sale held of an open episode, got %+v", low)
	}
}
