package smartsell

import (
	"fmt"
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// The floor of the activation (SaleStartAtDepth) is decided here on the shared
// action (takeLoss) itself, on the w3s ladders n deep, the way sale_test.go decides
// the other two bounds. A ladder in buy needs at least that many filled entries for its
// trailingBuy to be ACTIVATED: under it the action WAITS, activates nothing and writes the
// held row of each gate that is due, and the ceiling and the loss are not asked. It is the
// activation's alone: a ladder in trailingBuy, committed or armed by itself, is no activation
// and is not asked the floor. The rows are stated by hand, whole.

// underDepthText is the reason an activation is held for by the floor, as the row words
// it: the entries the ladder has filled and the least number the activation needs.
func underDepthText(filled, need int) string {
	return fmt.Sprintf("depth %d under the %d depths the activation needs", filled, need)
}

// floorOn runs the calling test with the three bounds of the shared action ON at their
// shipped values, the floor, the depth bound and the cap. It skips the test, naming the
// constant, when the floor is off or leaves no entry count under it, or when the depth
// bound leaves the ladder no depth the activation can be made at from the floor on.
func floorOn(t *testing.T) {
	t.Helper()
	if SaleStartAtDepth <= 1 {
		t.Skip("SaleStartAtDepth is off or leaves no entry count under it")
	}
	saleOn(t)
	withSaleStartAtDepth(t, SaleStartAtDepth)
	if sellingDepth(t) < SaleStartAtDepth {
		t.Skip("SaleIgnoreLastDepths leaves the w3s ladder no depth to be acted on at from SaleStartAtDepth")
	}
}

// farPrice is a price on the pair's cent grid at which a sale of the whole position loses
// far more than the cap: lossFactor of the break even, floored to the cent.
func farPrice(trade aggragates.Trades) float64 {
	return math.Floor(ladder.AverageEntryPrice(trade)*lossFactor*100) / 100
}

// A ladder in buy that has filled fewer entries than the floor needs is held at any print:
// a real loss within the cap, a gain and a loss far over the cap all name the floor, so the
// loss is not asked under it. From the floor on the activation goes as before: on a loss
// the trigger takes and at a gain, with the loss row far under the trigger, up to the
// deepest depth the depth bound lets the action act at, and past it the depth bound holds.
func TestSaleHoldsTheActivationUnderTheFloorAndGoesAheadFromIt(t *testing.T) {
	floorOn(t)
	farOverTheCap(t, shippedLoss)
	bound := sellingDepth(t)

	var cases []saleCase
	for _, stand := range eachStanding() {
		for filled := 1; filled <= lastDepthFills; filled++ {
			trade := depthLadder(filled)
			within, far, gain := aLossWithinTheCap(t, trade), farPrice(trade), math.Ceil(ladder.AverageEntryPrice(trade)*gainFactor*100)/100
			at := func(what string, price float64) saleCase {
				return onLadder(fmt.Sprintf("%s, %d fills, %s", stand.name, filled, what), trade, "buy", price, stand.triggers...)
			}
			switch {
			case filled < SaleStartAtDepth:
				why := underDepthText(filled, SaleStartAtDepth)
				cases = append(cases, at("within the cap", within).holdingAtItsFill(why), at("a gain", gain).holdingAtItsFill(why), at("far over the cap", far).holdingAtItsFill(why))
			case filled > bound:
				cases = append(cases, at("within the cap", within).holdingAtItsFill(atDepthText(filled, lastDepthFills, bound)))
			default:
				cases = append(cases,
					at("within the cap", within).goingAhead(trade.PositionPrice, lossPctOf(trade, within), shippedLoss),
					at("a gain", gain).goingAhead(trade.PositionPrice, lossPctOf(trade, gain), shippedLoss),
					at("far over the cap", far).holdingByLossAtItsFill(lossPctOf(trade, far), shippedLoss),
				)
			}
		}
	}
	runSale(t, cases)
}

// The floor is read before the ceiling and before the loss: a ladder in buy under the floor
// whose ceiling is no deeper than the depth bound, or is not known, and whose loss is far
// over the cap names the floor, where the same ladder in trailingBuy, which is no activation,
// is held by the depth bound as it always was.
func TestSaleNamesTheFloorBeforeTheCeilingAndTheLoss(t *testing.T) {
	floorOn(t)
	if SaleIgnoreLastDepths <= 0 {
		t.Skip("SaleIgnoreLastDepths is off: no ceiling holds a ladder to be read after the floor")
	}
	farOverTheCap(t, shippedLoss)
	filled := SaleStartAtDepth - 1
	noRow := depthLadder(filled)
	noRow.StrategyPair.StrategySettings = nil
	ladders := []struct {
		name  string
		trade aggragates.Trades
		depth string
	}{
		{"a ceiling at the depth bound", withDepths(depthLadder(filled), SaleIgnoreLastDepths), atDepthText(filled, SaleIgnoreLastDepths, 0)},
		{"a row under one depth", withDepths(depthLadder(filled), 0.5), fmt.Sprintf("depth %d of no known ceiling", filled)},
		{"no settings row", noRow, fmt.Sprintf("depth %d of no known ceiling", filled)},
	}

	var cases []saleCase
	for _, c := range ladders {
		for _, trigger := range eachGate() {
			for state, why := range map[string]string{"buy": underDepthText(filled, SaleStartAtDepth), "trailingBuy": c.depth} {
				cases = append(cases, onLadder(c.name+", "+trigger.gate+", "+state, c.trade, state, farPrice(c.trade), trigger).holdingAtItsFill(why))
			}
		}
	}
	runSale(t, cases)
}

// At zero or below the floor is off and a ladder in buy is activated at any depth, the first
// fill included; the same ladder is held with the floor on, and the row names the floor in
// force, not the shipped one.
func TestSaleFloorAtZeroOrBelowIsOffAndTheRowNamesTheFloorInForce(t *testing.T) {
	floorOn(t)
	filled := SaleStartAtDepth - 1
	trade := depthLadder(filled)
	price := aLossWithinTheCap(t, trade)

	var held, ahead, stricter []saleCase
	for _, stand := range eachStanding() {
		c := onLadder(stand.name+", under the floor", trade, "buy", price, stand.triggers...)
		held = append(held, c.holdingAtItsFill(underDepthText(filled, SaleStartAtDepth)))
		ahead = append(ahead, c.goingAhead(trade.PositionPrice, lossPctOf(trade, price), shippedLoss))
		stricter = append(stricter, c.holdingAtItsFill(underDepthText(filled, SaleStartAtDepth+2)))
	}
	runSale(t, held)
	withSaleStartAtDepth(t, SaleStartAtDepth+2)
	runSale(t, stricter)
	for _, off := range []int{0, -SaleStartAtDepth} {
		withSaleStartAtDepth(t, off)
		runSale(t, ahead)
	}
}

// The floor binds neither the sale of a committed ladder, which trails from its anchor (held,
// re-anchored or sold) whatever its depth, nor a ladder that armed itself in trailingBuy, which
// is sold on a real loss within the cap and held over it: both rest in trailingBuy, the
// force-trailing state included, and neither is an activation. Their fixtures hold fewer
// entries than the floor needs.
func TestSaleFloorBindsNeitherTheCommittedTrailNorALadderThatArmedItself(t *testing.T) {
	floorOn(t)
	farOverTheCap(t, shippedLoss)

	var cases []saleCase
	for filled := 1; filled < SaleStartAtDepth; filled++ {
		trade := depthLadder(filled)
		newest := trade.PositionPrice
		trade.PositionPrice = newest + positionMovedBy
		within, far := aLossWithinTheCap(t, trade), farPrice(trade)
		for _, state := range []string{"trailingBuy", "forceTrailingBuy"} {
			for _, trigger := range eachGate() {
				name := fmt.Sprintf("%d fills, %s, %s", filled, state, trigger.gate)
				cases = append(cases,
					onLadder(name+", armed itself, within the cap", trade, state, within, trigger).selling(),
					onLadder(name+", armed itself, far over the cap", trade, state, far, trigger).holdingByLoss(newest, lossPctOf(trade, far), shippedLoss),
				)
				cases = append(cases, trailCases(name+", committed", trade, state, within, activatedOf(trigger))...)
			}
		}
	}
	runSale(t, cases)
}
