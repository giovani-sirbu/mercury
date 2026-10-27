package smarttakeloss

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// SaleGapTolerances is the gap floor of the committed sale: a print under the anchor by more than
// that many of the held depth's tolerances opens a gap, once per gap episode with its own row, and
// is held; the trail goes on from the same anchor and SaleGapWait bounds the hold: a gapped print
// that long after the row sells at the print, a print back over the floor closes the gap with its
// own row; a move under minus the tolerance but not under the floor sells at once.
// The knobs are pinned for the package (the floor off); each test switches them on to gapFor
// tolerances and gapWait.

// gapFor is the floor the tests switch on, gapWait the wait counted from the stamp of the gap row.
const (
	gapFor  = 5.0
	gapWait = 5 * time.Second
)

// gapScene is a sale gate's committed ladder, its stopLoss anchored at the print its trigger was
// reached at with no gap open, and the prints the gap tests ask it at, each by its move against the anchor.
type gapScene struct {
	fixture  saleFixture
	anchored aggragates.Trades
	row      aggragates.StrategySettings
	floor    float64
	// gapped, deeper and collapse are past the floor, inside under the tolerance within it; holdOver
	// and holdUnder fall between the bounds on either side of the anchor, rise is past the trail.
	gapped, deeper, collapse, inside, holdOver, holdUnder, rise float64
	gapRow                                                      Row
}

// gapSceneOf is the scene of the fixture's gate, the gap floor read off the held depth's row.
func gapSceneOf(c saleFixture) gapScene {
	anchored := restingAt(committedBy(c.opened(), c.held(c.over), c.activated(c.within)), "stopLoss", c.within)
	row := trailRow(anchored)
	floor := gapFor * row.Tolerance
	at := func(move float64) float64 { return priceAtMove(anchored, c.within, move) }
	return gapScene{
		fixture: c, anchored: anchored, row: row, floor: floor,
		gapped: at(-(floor + 1)), deeper: at(-(floor + 10)), collapse: at(-50), inside: at(-(row.Tolerance + floor) / 2),
		holdOver: at(row.TrailingTakeProfit / 2), holdUnder: at(-0.4 * row.Tolerance), rise: at(row.TrailingTakeProfit + 1),
		gapRow: SaleHeldByGapRow(c.gate, c.fill, floor+1, floor, gapFor, c.features...),
	}
}

// closedRow is the gapClosed row of the scene's gate for a print at move against the anchor.
func (s gapScene) closedRow(move float64) Row {
	return GapClosedRow(s.fixture.gate, s.fixture.fill, move, s.fixture.features...)
}

// opened is the ladder after the gapped print at confirmStart: the gap row written and stamped there.
func (s gapScene) opened(t *testing.T) aggragates.Trades {
	t.Helper()
	opened, got := engineStepAt(s.anchored, "", s.gapped, confirmStart, s.fixture.ai)
	assertTrailHeld(t, got, s.gapRow)
	return opened
}

// pastTheFloor are the scene's prints past the gap floor.
func (s gapScene) pastTheFloor() []float64 {
	return []float64{s.gapped, s.deeper, s.collapse}
}

// gapped is exactly the comparison the floor makes: strictly under minus the tolerances, the knob on.
func TestGappedIsStrictlyUnderMinusTheToleranceTimesTheKnob(t *testing.T) {
	withSaleGapTolerances(t, gapFor)
	for _, c := range []struct {
		move, tolerance float64
		want            bool
	}{
		{-1.25, 0.25, false}, {-1.2500001, 0.25, true}, {-1.0, 0.25, false}, {-100, 0.25, true},
		{0, 0.25, false}, {0.5, 0.25, false}, {-0.5, 0.1, false}, {-0.51, 0.1, true},
		{-5, 0, false}, {-5, -0.25, false},
	} {
		if got := gapped(c.move, c.tolerance); got != c.want {
			t.Errorf("gapped(%v, %v) at %v tolerances = %v, want %v", c.move, c.tolerance, gapFor, got, c.want)
		}
	}

	withSaleGapTolerances(t, 2.5)
	if gapped(-0.62, 0.25) || !gapped(-0.63, 0.25) {
		t.Errorf("a fractional floor of 2.5 tolerances must sit at 0.625 %% of a 0.25 tolerance")
	}
	for _, off := range []float64{0, -1} {
		withSaleGapTolerances(t, off)
		if gapped(-100, 0.25) {
			t.Errorf("a floor of %v tolerances is off: no move is a gap", off)
		}
	}
}

// trailingStep answers the gap hold before the sale, on both sides: under minus the tolerance but not
// under the floor sells, one further under holds, a rise is never a gap; knob off, every fall sells.
func TestTrailingStepHoldsAGapAndSellsInsideTheFloor(t *testing.T) {
	long, inverse := restingAt(flatMarketLadder(false), "stopLoss", 100), restingAt(flatMarketLadder(true), "stopLoss", 100)
	if row := trailRow(long); row.Tolerance != 0.25 || row.TrailingTakeProfit != 1 {
		t.Fatalf("fixture drifted: the row must carry a tolerance of 0.25 and a trailing take profit of 1, got %+v", row)
	}
	type answer struct {
		trade aggragates.Trades
		price float64
		on    saleStep
		off   saleStep
	}
	for name, c := range map[string]answer{
		"long, a fall past the tolerance":           {long, 99.7, stepSell, stepSell},
		"long, a fall to just over the floor":       {long, 98.8, stepSell, stepSell},
		"long, a fall just under the floor":         {long, 98.7, stepGapHold, stepSell},
		"long, a collapse":                          {long, 50, stepGapHold, stepSell},
		"long, a fall just inside the tolerance":    {long, 99.8, stepHold, stepHold},
		"long, a rise past the trail":               {long, 101.2, stepReanchor, stepReanchor},
		"long, a spike up":                          {long, 200, stepReanchor, stepReanchor},
		"inverse, a rise past the tolerance":        {inverse, 100.3, stepSell, stepSell},
		"inverse, a rise to just over the floor":    {inverse, 101.2, stepSell, stepSell},
		"inverse, a rise just under the floor":      {inverse, 101.3, stepGapHold, stepSell},
		"inverse, a spike":                          {inverse, 200, stepGapHold, stepSell},
		"inverse, a rise just inside the tolerance": {inverse, 100.2, stepHold, stepHold},
		"inverse, a fall past the trail":            {inverse, 98.8, stepReanchor, stepReanchor},
		"inverse, a collapse down is a re-anchor":   {inverse, 50, stepReanchor, stepReanchor},
	} {
		withSaleGapTolerances(t, gapFor)
		if got := trailingStep(c.trade, c.price); got != c.on {
			t.Errorf("%s, floor on: trailingStep at %v = %s, want %s", name, c.price, stepNames[got], stepNames[c.on])
		}
		withSaleGapTolerances(t, 0)
		if got := trailingStep(c.trade, c.price); got != c.off {
			t.Errorf("%s, floor off: trailingStep at %v = %s, want %s", name, c.price, stepNames[got], stepNames[c.off])
		}
	}
}

// A settings row whose tolerance is zero or below has no floor (every fall is already past the
// tolerance): the gapped print sells, long and inverse, and no gap row is written.
func TestARowWithNoToleranceHasNoGapFloor(t *testing.T) {
	flatMarketAlone(t)
	withSellLossIgnoreLastDepths(t, SellLossIgnoreLastDepths)
	withSaleGapTolerances(t, gapFor)
	for _, tolerance := range []float64{0, -0.25} {
		for _, c := range []struct {
			inverse bool
			gapped  float64
		}{{false, 98.7}, {true, 101.3}} {
			trade := flatMarketLadder(c.inverse)
			trade.StrategyPair.StrategySettings[0].Tolerance = tolerance
			ai := flatMarketMarket(trade)
			committed := restingAt(committedBy(withRows(trade, triggeredBy(trade, ai)), FlatMarketStopLossActivatedRow(trade.PositionPrice, -0.1)), "stopLoss", 100)
			if got := trailingStep(committed, c.gapped); got != stepSell {
				t.Errorf("tolerance %v, inverse %v: trailingStep at %v = %s, want sell", tolerance, c.inverse, c.gapped, stepNames[got])
			}
			assertSold(t, Apply(committed, "", c.gapped, ai), reasonFlatMarket)
		}
	}
}

// The floor is counted in the tolerance of the held depth's row, as the sale's own bound is.
func TestTheGapFloorReadsTheToleranceOfTheHeldDepth(t *testing.T) {
	withSaleGapTolerances(t, gapFor)
	rows := []aggragates.StrategySettings{
		{Percentage: 2, Tolerance: 0.1, TrailingTakeProfit: 0.2, Multiplier: 2, Depths: 8},
		{Percentage: 2, Tolerance: 0.3, TrailingTakeProfit: 0.4, Multiplier: 2, Depths: 8},
		{Percentage: 2, Tolerance: 0.5, TrailingTakeProfit: 0.6, Multiplier: 2, Depths: 8},
	}
	for filled, tolerance := range map[int]float64{1: 0.1, 2: 0.3, 3: 0.5} {
		trade := restingAt(depthLadder(filled), "stopLoss", 100)
		trade.StrategyPair.StrategySettings = rows
		for _, c := range []struct {
			name string
			move float64
			want saleStep
		}{
			{"a fall inside the floor", -0.9 * gapFor * tolerance, stepSell},
			{"a fall past the floor", -1.1 * gapFor * tolerance, stepGapHold},
		} {
			if got := trailingStep(anchoredFor(trade, 100, c.move), 100); got != c.want {
				t.Errorf("%d fills (tolerance %v), %s: trailingStep = %s, want %s", filled, tolerance, c.name, stepNames[got], stepNames[c.want])
			}
		}
	}
}

// On every sale gate a committed ladder gapped under its anchor: the proposal is dropped (Position
// ""), nothing is sold, and ONE gap row is written under the committed gate with its features; the
// prints after it hold with no row; a print inside the floor sells. A print over the floor closes
// the gap: a hold between the bounds or a re-anchor writes the gapClosed row beside the answer (none
// with no gap open), and a gap after it opens a new one with a new gap row, where the first one's
// wait would have passed already; knob off, the gapped print sells.
func TestAGappedPrintHoldsTheCommittedSaleOncePerGapEpisode(t *testing.T) {
	for _, gate := range saleGates {
		t.Run(gate.name, func(t *testing.T) {
			c := gate.build(t)
			withSaleGapTolerances(t, gapFor)
			withSaleGapWait(t, gapWait)
			s := gapSceneOf(c)

			assertSold(t, Apply(s.anchored, "", s.inside, c.ai), c.reason)
			for _, position := range addSideProposals {
				assertTrailHeld(t, Apply(s.anchored, position, s.gapped, c.ai), s.gapRow)
			}
			assertTrailHeld(t, Apply(s.anchored, "", s.holdOver, c.ai))
			assertTrailHeld(t, Apply(s.anchored, "", s.holdUnder, c.ai))
			assertReanchored(t, Apply(s.anchored, "", s.rise, c.ai))

			first := s.opened(t)
			memory := memoryOf(rebuildState(first), c.gate)
			if !memory.gapHeldAt.Equal(confirmStart) || !memory.activated {
				t.Fatalf("the gap row must fold into the gate's gap stamp at its own clock on its activation, got %+v", memory)
			}
			for _, price := range s.pastTheFloor() {
				assertTrailHeld(t, Apply(first, "", price, c.ai))
				assertTrailHeld(t, ApplyAt(first, "", price, c.ai, confirmStart.Add(time.Second)))
			}
			assertSold(t, Apply(first, "", s.inside, c.ai), c.reason)

			for _, closing := range []struct {
				name      string
				price     float64
				move      float64
				reanchors bool
			}{
				{"a hold over the anchor", s.holdOver, s.row.TrailingTakeProfit / 2, false},
				{"a hold under the anchor", s.holdUnder, -0.4 * s.row.Tolerance, false},
				{"a re-anchor", s.rise, s.row.TrailingTakeProfit + 1, true},
			} {
				row := s.closedRow(closing.move)
				answers := func(got Result) {
					t.Helper()
					if closing.reanchors {
						assertReanchored(t, got, row)
						return
					}
					assertTrailHeld(t, got, row)
				}
				if !closing.reanchors {
					for _, position := range addSideProposals {
						answers(Apply(first, position, closing.price, c.ai))
					}
				}
				answers(Apply(first, "", closing.price, c.ai))
				answers(ApplyAt(first, "", closing.price, c.ai, confirmStart.Add(time.Hour)))

				closed, got := engineStepAt(first, "", closing.price, confirmStart.Add(2*time.Second), c.ai)
				answers(got)
				if memory := memoryOf(rebuildState(closed), c.gate); memory.gapHeld || !memory.activated {
					t.Fatalf("%s: the gapClosed row must close the gap and leave the gate activated, got %+v", closing.name, memory)
				}

				anchor := closed.PositionPrice
				again, later := priceAtMove(closed, anchor, -(s.floor+1)), confirmStart.Add(gapWait+time.Second)
				reopened, got := engineStepAt(closed, "", again, later, c.ai)
				assertTrailHeld(t, got, s.gapRow)
				if stamp := memoryOf(rebuildState(reopened), c.gate).gapHeldAt; !stamp.Equal(later) {
					t.Fatalf("%s: a gap after the close must stamp a new episode at %v, got %v", closing.name, later, stamp)
				}
				assertTrailHeld(t, ApplyAt(reopened, "", again, c.ai, later.Add(gapWait-time.Nanosecond)))
				assertSold(t, ApplyAt(reopened, "", again, c.ai, later.Add(gapWait)), c.reason)
				assertSold(t, Apply(closed, "", priceAtMove(closed, anchor, -(s.row.Tolerance+s.floor)/2), c.ai), c.reason)
			}

			withSaleGapTolerances(t, 0)
			assertSold(t, Apply(s.anchored, "", s.gapped, c.ai), c.reason)
			assertSold(t, Apply(first, "", s.gapped, c.ai), c.reason)
		})
	}
}

// A second gapped print before SaleGapWait has passed since the gap row holds with no row, whatever
// the print and the proposal, even on a clock behind the row; the hold writes nothing, so the wait
// stays counted from the row and not from the held prints.
func TestASecondGappedPrintBeforeTheWaitHoldsWithNoRow(t *testing.T) {
	for _, gate := range saleGates {
		t.Run(gate.name, func(t *testing.T) {
			c := gate.build(t)
			withSaleGapTolerances(t, gapFor)
			withSaleGapWait(t, gapWait)
			s := gapSceneOf(c)
			first := s.opened(t)

			for _, offset := range []time.Duration{-time.Second, 0, time.Nanosecond, time.Second, gapWait - time.Second, gapWait - time.Nanosecond} {
				now := confirmStart.Add(offset)
				for _, price := range s.pastTheFloor() {
					assertTrailHeld(t, ApplyAt(first, "", price, c.ai, now))
				}
				for _, position := range addSideProposals {
					assertTrailHeld(t, ApplyAt(first, position, s.gapped, c.ai, now))
				}
			}

			held, got := engineStepAt(first, "", s.deeper, confirmStart.Add(gapWait-time.Nanosecond), c.ai)
			assertTrailHeld(t, got)
			if stamp := memoryOf(rebuildState(held), c.gate).gapHeldAt; !stamp.Equal(confirmStart) {
				t.Fatalf("a held print must leave the row's stamp standing, got %v", stamp)
			}
			assertSold(t, ApplyAt(held, "", s.gapped, c.ai, confirmStart.Add(gapWait)), c.reason)
		})
	}
}

// A gapped print at or after SaleGapWait since the gap row sells under the committed trigger's
// reason (Position sellLoss, no row), whatever the print and the proposal: the gap was a move. The
// wait is the knob's, to the nanosecond.
func TestAGappedPrintAtOrAfterTheWaitSells(t *testing.T) {
	for _, gate := range saleGates {
		t.Run(gate.name, func(t *testing.T) {
			c := gate.build(t)
			withSaleGapTolerances(t, gapFor)
			withSaleGapWait(t, gapWait)
			s := gapSceneOf(c)
			first := s.opened(t)

			for _, offset := range []time.Duration{gapWait, gapWait + time.Nanosecond, gapWait + time.Second, time.Hour, 48 * time.Hour} {
				now := confirmStart.Add(offset)
				for _, price := range s.pastTheFloor() {
					assertSold(t, ApplyAt(first, "", price, c.ai, now), c.reason)
				}
				for _, position := range addSideProposals {
					assertSold(t, ApplyAt(first, position, s.gapped, c.ai, now), c.reason)
				}
			}

			for _, wait := range []time.Duration{time.Nanosecond, time.Second, time.Minute} {
				withSaleGapWait(t, wait)
				assertTrailHeld(t, ApplyAt(first, "", s.gapped, c.ai, confirmStart.Add(wait-time.Nanosecond)))
				assertSold(t, ApplyAt(first, "", s.gapped, c.ai, confirmStart.Add(wait)), c.reason)
			}
		})
	}
}

// With SaleGapWait at zero or below, or on a clockless Apply, the gap holds without end: the first
// gapped print still writes its row, no later gapped print sells however long after, a print inside
// the floor sells and a print over the floor still closes the gap with its row.
func TestAGapHoldsWithoutEndWithTheWaitOffOrNoClock(t *testing.T) {
	for _, gate := range saleGates {
		t.Run(gate.name, func(t *testing.T) {
			c := gate.build(t)
			withSaleGapTolerances(t, gapFor)
			s := gapSceneOf(c)
			long := []time.Duration{gapWait, time.Hour, 365 * 24 * time.Hour}

			for _, wait := range []time.Duration{0, -time.Nanosecond, -gapWait} {
				withSaleGapWait(t, wait)
				assertTrailHeld(t, ApplyAt(s.anchored, "", s.gapped, c.ai, confirmStart), s.gapRow)
				first := s.opened(t)
				for _, offset := range long {
					now := confirmStart.Add(offset)
					for _, price := range s.pastTheFloor() {
						assertTrailHeld(t, ApplyAt(first, "", price, c.ai, now))
					}
					assertSold(t, ApplyAt(first, "", s.inside, c.ai, now), c.reason)
					assertTrailHeld(t, ApplyAt(first, "", s.holdOver, c.ai, now), s.closedRow(s.row.TrailingTakeProfit/2))
				}
			}

			withSaleGapWait(t, gapWait)
			first := s.opened(t)
			for _, price := range s.pastTheFloor() {
				assertTrailHeld(t, Apply(first, "", price, c.ai))
				assertTrailHeld(t, ApplyAt(first, "", price, c.ai, time.Time{}))
			}
			assertTrailHeld(t, Apply(s.anchored, "", s.gapped, c.ai), s.gapRow)
			assertSold(t, Apply(first, "", s.inside, c.ai), c.reason)
			assertTrailHeld(t, Apply(first, "", s.holdUnder, c.ai), s.closedRow(-0.4*s.row.Tolerance))
		})
	}
}

// The inverse ladder gaps upward: the same sequence mirrored through Apply, one row, then none until
// the wait has passed, then the sale; a print back down is the close, "over" the anchor in the
// ladder's own metric, a print a hair up the hold "under" it.
func TestAnInverseLaddersGapIsAPrintAboveTheAnchor(t *testing.T) {
	flatMarketAlone(t)
	withSellLossIgnoreLastDepths(t, SellLossIgnoreLastDepths)
	withSaleGapTolerances(t, gapFor)
	withSaleGapWait(t, gapWait)
	trade := flatMarketLadder(true)
	ai, fill := flatMarketMarket(trade), trade.PositionPrice
	committed := restingAt(committedBy(withRows(trade, triggeredBy(trade, ai)), FlatMarketStopLossActivatedRow(fill, -0.1)), "stopLoss", 100)
	gapRow := SaleHeldByGapRow(GateFlatMarket, fill, 1.28, 1.25, gapFor)
	closed := func(move float64) Row { return GapClosedRow(GateFlatMarket, fill, move) }

	for _, position := range addSideProposals {
		assertTrailHeld(t, Apply(committed, position, 101.3, ai), gapRow)
		assertSold(t, Apply(committed, position, 101.0, ai), reasonFlatMarket)
		assertReanchored(t, Apply(committed, position, 98.8, ai))
	}
	first, got := engineStepAt(committed, "", 101.3, confirmStart, ai)
	assertTrailHeld(t, got, gapRow)
	assertTrailHeld(t, Apply(first, "", 150, ai))
	assertTrailHeld(t, ApplyAt(first, "", 150, ai, confirmStart.Add(gapWait-time.Nanosecond)))
	assertSold(t, ApplyAt(first, "", 150, ai, confirmStart.Add(gapWait)), reasonFlatMarket)
	assertSold(t, Apply(first, "", 101.0, ai), reasonFlatMarket)

	fall, rise := priceAtMove(committed, 100, 2), priceAtMove(committed, 100, -0.1)
	assertReanchored(t, Apply(first, "", fall, ai), closed(2))
	assertTrailHeld(t, Apply(first, "", rise, ai), closed(-0.1))
	assertReanchored(t, Apply(committed, "", fall, ai))

	withSaleGapTolerances(t, 0)
	assertSold(t, Apply(committed, "", 101.3, ai), reasonFlatMarket)
}

// sellLoss on its own: the row is due while no gap is open (gapRowDue), filed under the committed
// trigger (the first live one that activated, else the first) with its features; the hold sells nothing.
func TestSellLossHoldsAGappedCommittedSaleWithTheCommittedTriggersRow(t *testing.T) {
	withSellLossIgnoreLastDepths(t, 0)
	withSaleGapTolerances(t, gapFor)
	trade := restingAt(flatMarketLadder(false), "stopLoss", 100)
	fill := roundPrices[len(roundPrices)-1]
	due := func(trigger saleTrigger) saleTrigger {
		trigger.gapRowDue = true
		return trigger
	}
	slow, low := activatedOf(slowTrigger(false)), activatedOf(lowTrigger(false))
	slowFeatures := slowTrigger(false).features
	const gapped, inside = 98.7, 99.0
	movePct := (100 - gapped) / gapped * 100

	for _, c := range []struct {
		name     string
		price    float64
		triggers []saleTrigger
		step     saleStep
		reason   string
		rows     []Row
	}{
		{"one row due", gapped, []saleTrigger{due(low)}, stepHold, "", []Row{SaleHeldByGapRow(GateFlatMarket, fill, movePct, 1.25, gapFor)}},
		{"the row already written", gapped, []saleTrigger{low}, stepHold, "", nil},
		{"the slow decline's, with its features", gapped, []saleTrigger{due(slow)}, stepHold, "", []Row{SaleHeldByGapRow(GateSlowDecline, fill, movePct, 1.25, gapFor, slowFeatures...)}},
		{"both activated, the slow decline first", gapped, []saleTrigger{due(slow), due(low)}, stepHold, "", []Row{SaleHeldByGapRow(GateSlowDecline, fill, movePct, 1.25, gapFor, slowFeatures...)}},
		{"both activated, the flat market first", gapped, []saleTrigger{due(low), due(slow)}, stepHold, "", []Row{SaleHeldByGapRow(GateFlatMarket, fill, movePct, 1.25, gapFor)}},
		{"only the second activated", gapped, []saleTrigger{due(slowTrigger(false)), due(low)}, stepHold, "", []Row{SaleHeldByGapRow(GateFlatMarket, fill, movePct, 1.25, gapFor)}},
		{"inside the floor sells under the committed reason", inside, []saleTrigger{due(slowTrigger(false)), due(low)}, stepSell, lowReason, nil},
		{"only the first trigger's row due, the second activated too", gapped, []saleTrigger{due(low), slow}, stepHold, "", []Row{SaleHeldByGapRow(GateFlatMarket, fill, movePct, 1.25, gapFor)}},
	} {
		t.Run(c.name, func(t *testing.T) {
			rows, step, reason := sellLoss(trade, rebuildState(trade), c.price, c.triggers, everyRowShared())
			if step != c.step || reason != c.reason {
				t.Errorf("step = %s, reason = %q, want %s, %q", stepNames[step], reason, stepNames[c.step], c.reason)
			}
			if len(rows) != len(c.rows) || (len(c.rows) > 0 && !reflect.DeepEqual(rows, c.rows)) {
				t.Errorf("rows = %+v, want exactly %+v", rows, c.rows)
			}
		})
	}
}

// sellLossAt on its own, the committed trigger's gap stamp read at the clock: an open gap sells a
// gapped print once SaleGapWait has passed since the stamp and holds it before, with the wait off,
// on no clock or with no stamp; a print inside the floor sells; a hold or a re-anchor over the floor
// writes the gapClosed row under the committed trigger's gate and features, none with no gap open.
func TestSellLossAtReadsTheCommittedTriggersGapStamp(t *testing.T) {
	withSellLossIgnoreLastDepths(t, 0)
	withSaleGapTolerances(t, gapFor)
	trade := restingAt(flatMarketLadder(false), "stopLoss", 100)
	fill := roundPrices[len(roundPrices)-1]
	stamp := confirmStart
	open := func(trigger saleTrigger) saleTrigger {
		trigger.gapHeldAt = stamp
		return trigger
	}
	due := func(trigger saleTrigger) saleTrigger {
		trigger.gapRowDue = true
		return trigger
	}
	slow, low := activatedOf(slowTrigger(false)), activatedOf(lowTrigger(false))
	const gapped, inside, holdUnder, holdOver, rise = 98.7, 99.0, 99.9, 100.5, 101.2
	move := func(price float64) float64 { return (price - 100) / price * 100 }
	closed := func(gate string, price float64, features ...string) []Row {
		return []Row{GapClosedRow(gate, fill, move(price), features...)}
	}
	movePct := -move(gapped)

	for _, c := range []struct {
		name     string
		price    float64
		triggers []saleTrigger
		now      time.Time
		wait     time.Duration
		step     saleStep
		reason   string
		rows     []Row
	}{
		{"a gap open, before the wait", gapped, []saleTrigger{open(low)}, stamp.Add(gapWait - time.Nanosecond), gapWait, stepHold, "", nil},
		{"a gap open, at the wait", gapped, []saleTrigger{open(low)}, stamp.Add(gapWait), gapWait, stepSell, lowReason, nil},
		{"a gap open, long after the wait", gapped, []saleTrigger{open(low)}, stamp.Add(100 * time.Hour), gapWait, stepSell, lowReason, nil},
		{"a gap open, the wait off", gapped, []saleTrigger{open(low)}, stamp.Add(100 * time.Hour), 0, stepHold, "", nil},
		{"a gap open, the wait below zero", gapped, []saleTrigger{open(low)}, stamp.Add(100 * time.Hour), -gapWait, stepHold, "", nil},
		{"a gap open, no clock", gapped, []saleTrigger{open(low)}, time.Time{}, gapWait, stepHold, "", nil},
		{"a clock behind the stamp", gapped, []saleTrigger{open(low)}, stamp.Add(-time.Hour), gapWait, stepHold, "", nil},
		{"no stamp and no row due: no gap to have waited", gapped, []saleTrigger{low}, stamp.Add(100 * time.Hour), gapWait, stepHold, "", nil},
		{"no gap open: the row, whatever the clock", gapped, []saleTrigger{due(low)}, stamp.Add(100 * time.Hour), gapWait, stepHold, "",
			[]Row{SaleHeldByGapRow(GateFlatMarket, fill, movePct, 1.25, gapFor)}},
		{"a gap open, a print inside the floor", inside, []saleTrigger{open(low)}, stamp.Add(time.Second), gapWait, stepSell, lowReason, nil},
		{"a gap open, a hold under the anchor", holdUnder, []saleTrigger{open(low)}, stamp.Add(time.Second), gapWait, stepHold, "", closed(GateFlatMarket, holdUnder)},
		{"a gap open, a hold over the anchor", holdOver, []saleTrigger{open(low)}, stamp.Add(time.Second), gapWait, stepHold, "", closed(GateFlatMarket, holdOver)},
		{"a gap open, a re-anchor", rise, []saleTrigger{open(low)}, stamp.Add(time.Second), gapWait, stepReanchor, "", closed(GateFlatMarket, rise)},
		{"a gap open, a hold on no clock", holdOver, []saleTrigger{open(low)}, time.Time{}, gapWait, stepHold, "", closed(GateFlatMarket, holdOver)},
		{"a gap open, a hold, the wait off", holdOver, []saleTrigger{open(low)}, stamp.Add(time.Second), 0, stepHold, "", closed(GateFlatMarket, holdOver)},
		{"the slow decline's, with its features", holdOver, []saleTrigger{open(slow), low}, stamp.Add(time.Second), gapWait, stepHold, "", closed(GateSlowDecline, holdOver, slowTrigger(false).features...)},
		{"no gap open: a hold writes nothing", holdOver, []saleTrigger{low}, stamp.Add(time.Second), gapWait, stepHold, "", nil},
		{"no gap open: a re-anchor writes nothing", rise, []saleTrigger{low}, stamp.Add(time.Second), gapWait, stepReanchor, "", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			withSaleGapWait(t, c.wait)
			rows, step, reason := sellLossAt(trade, rebuildState(trade), c.price, c.triggers, everyRowShared(), c.now)
			if step != c.step || reason != c.reason {
				t.Errorf("step = %s, reason = %q, want %s, %q", stepNames[step], reason, stepNames[c.step], c.reason)
			}
			if len(rows) != len(c.rows) || (len(c.rows) > 0 && !reflect.DeepEqual(rows, c.rows)) {
				t.Errorf("rows = %+v, want exactly %+v", rows, c.rows)
			}
		})
	}
}

// ExitReached reads a gap hold as no exit and the print inside the floor as one, the floor on.
func TestExitReachedIsFalseOnAGapHold(t *testing.T) {
	flatMarketAlone(t)
	withSellLossIgnoreLastDepths(t, SellLossIgnoreLastDepths)
	withSaleGapTolerances(t, gapFor)
	for _, inverse := range []bool{false, true} {
		trade := flatMarketLadder(inverse)
		ai := flatMarketMarket(trade)
		committed := restingAt(committedBy(withRows(trade, triggeredBy(trade, ai)), FlatMarketStopLossActivatedRow(trade.PositionPrice, -0.1)), "stopLoss", 100)
		gappedPrint, inside := priceAtMove(committed, 100, -2), priceAtMove(committed, 100, -0.5)
		logs, events := len(committed.Logs), len(committed.StrategyEvents)
		if ExitReached(committed, gappedPrint, ai.SmartTakeLoss) {
			t.Errorf("inverse %v: a gap hold is no exit", inverse)
		}
		if !ExitReached(committed, inside, ai.SmartTakeLoss) {
			t.Errorf("inverse %v: a print inside the floor is an exit", inverse)
		}
		if len(committed.Logs) != logs || len(committed.StrategyEvents) != events {
			t.Errorf("inverse %v: ExitReached must write nothing", inverse)
		}
	}
}

// The fold reads a gap event into its gate's gap stamp, at the event's own clock, joining the episode
// the way a held event does and touching nothing else; a gapClosed event, an activation, a sale, a
// reset, the episode's opening, the slow decline's pending or cancelled and the legacy ended zero it,
// and a gap after a close stamps a new episode; an activation leaves the held sale and its trigger as
// they were, a gapClosed leaves the sale activated.
func TestRebuildStateFoldsTheGapStampOfEachSaleGate(t *testing.T) {
	for _, gate := range saleGates {
		t.Run(gate.name, func(t *testing.T) {
			c := gate.build(t)
			memoryAfter := func(trade aggragates.Trades) gateMemory { return memoryOf(rebuildState(trade), c.gate) }
			gapRow := SaleHeldByGapRow(c.gate, c.fill, 2.25, 1.25, gapFor, c.features...)
			closedRow := GapClosedRow(c.gate, c.fill, 0.5, c.features...)
			held := Append(c.opened(), c.held(c.over), confirmStart)
			heldMemory := memoryAfter(held)
			if !heldMemory.saleHeld || !heldMemory.heldUnder.named || heldMemory.gapHeld {
				t.Fatalf("fixture drifted: the held row must leave the sale held under its trigger and no gap row, got %+v", heldMemory)
			}

			activated := Append(held, c.activated(c.within), confirmStart.Add(time.Second))
			afterActivation := memoryAfter(activated)
			if !afterActivation.activated || afterActivation.saleHeld != heldMemory.saleHeld || afterActivation.heldUnder != heldMemory.heldUnder {
				t.Errorf("an activation must leave the held sale and its trigger as they were, got %+v from %+v", afterActivation, heldMemory)
			}

			gapAt := confirmStart.Add(2 * time.Second)
			gappedTrade := Append(activated, gapRow, gapAt)
			if got := memoryAfter(gappedTrade); !got.gapHeldAt.Equal(gapAt) || !got.activated || got.saleHeld != heldMemory.saleHeld || got.heldUnder != heldMemory.heldUnder {
				t.Fatalf("a gap event must stamp the gap at its own clock and nothing else, got %+v", got)
			}
			if got := memoryAfter(Append(gappedTrade, c.held(c.over), confirmStart.Add(3*time.Second))); !got.gapHeldAt.Equal(gapAt) {
				t.Errorf("a held row does not move the gap stamp, got %+v", got)
			}

			closedAt := confirmStart.Add(4 * time.Second)
			closedTrade := Append(gappedTrade, closedRow, closedAt)
			if got := memoryAfter(closedTrade); got.gapHeld || !got.gapHeldAt.IsZero() || !got.activated || got.saleHeld != heldMemory.saleHeld || got.heldUnder != heldMemory.heldUnder {
				t.Errorf("a gapClosed event must zero the stamp and leave the sale activated and held as it was, got %+v", got)
			}
			reopenedAt := confirmStart.Add(time.Minute)
			if got := memoryAfter(Append(closedTrade, gapRow, reopenedAt)); !got.gapHeldAt.Equal(reopenedAt) || !got.activated {
				t.Errorf("a gap after a close must stamp a new episode at its own clock, got %+v", got)
			}
			if got := memoryAfter(Append(activated, closedRow, closedAt)); got.gapHeld || !got.activated {
				t.Errorf("a gapClosed event with no gap open must leave the sale activated and no gap, got %+v", got)
			}

			opens := quietPending(c.fill)
			if len(c.opening) > 0 {
				opens = c.opening[0]
			}
			resets := map[string]Row{
				GateFlatMarket:         FlatMarketResetRow(c.fill),
				GateFreeFallPrevention: FreeFallPreventionResetRow(c.fill),
				GateCapitalProtection:  CapitalProtectionResetRow(c.fill),
				GateSlowDecline:        ResetRow(c.fill, quietFinding(nil)),
			}
			clears := map[string]Row{
				"activated":         c.activated(c.within),
				"sold":              ExitRow(c.trade, c.reason, c.within),
				"reset":             resets[c.gate],
				"the opening event": opens,
			}
			if c.gate == GateSlowDecline {
				clears["cancelled"] = CancelledRow(c.fill, quietFinding(nil))
			}
			for name, row := range clears {
				if got := memoryAfter(Append(gappedTrade, row, confirmStart.Add(time.Minute))); got.gapHeld {
					t.Errorf("%s must clear the gap stamp, got %+v", name, got)
				}
			}
			if c.gate == GateFlatMarket {
				ended := eventOf(gateLowInterest, legacyDocument(EventEnded, c.fill))
				if got := memoryAfter(withEvents(gappedTrade, ended)); got.gapHeld {
					t.Errorf("the legacy ended event must clear the gap stamp, got %+v", got)
				}
			}

			if c.gate != GateSlowDecline {
				if got := memoryAfter(Append(c.trade, gapRow, confirmStart)); !got.open || !got.gapHeldAt.Equal(confirmStart) || got.saleHeld {
					t.Errorf("a gap event must join the episode without holding the sale, got %+v", got)
				}
			}
			if got := memoryAfter(Append(c.opened(), gapRow, confirmStart)); got.saleHeld || got.heldUnder.named {
				t.Errorf("a gap event must not hold the sale, got %+v", got)
			}

			other := GateFlatMarket
			if c.gate == other {
				other = GateCapitalProtection
			}
			if got := memoryAfter(Append(activated, SaleHeldByGapRow(other, c.fill, 2.25, 1.25, gapFor), confirmStart)); got.gapHeld {
				t.Errorf("a gap event of %s must not set %s's stamp, got %+v", other, c.gate, got)
			}
			if got := memoryAfter(Append(gappedTrade, GapClosedRow(other, c.fill, 0.5), closedAt)); !got.gapHeldAt.Equal(gapAt) {
				t.Errorf("a gapClosed event of %s must not zero %s's stamp, got %+v", other, c.gate, got)
			}
			if c.gate == GateSlowDecline {
				bare := eventOf(GateSlowDecline, `{"event":"gapHeld","price":`+jsonNumber(c.fill)+`}`)
				bare.CreatedAt = gapAt
				if got := memoryAfter(withEvents(activated, bare)); got.gapHeld {
					t.Errorf("a slow decline gap event without features is never folded, got %+v", got)
				}
				bareClose := eventOf(GateSlowDecline, `{"event":"gapClosed","price":`+jsonNumber(c.fill)+`}`)
				bareClose.CreatedAt = closedAt
				if got := memoryAfter(withEvents(gappedTrade, bareClose)); !got.gapHeldAt.Equal(gapAt) {
					t.Errorf("a slow decline gapClosed event without features is never folded, got %+v", got)
				}
			}
		})
	}
}

// The gap row is "smartTakeLoss: <gate> sale held, print <m>% under the anchor, over the gap of <n>
// tolerances (<g>%)" at the newest fill under its gate, its event gapHeld with the fill as price and
// no trigger, features cloned; a held row by the profit keeps its own kind.
func TestTheSaleHeldByGapRowIsTheGrammarOfEachGate(t *testing.T) {
	features := []string{FeatureQuietDecline}
	for gate, label := range map[string]string{
		GateSlowDecline: "slow decline", GateFlatMarket: "flat market",
		GateFreeFallPrevention: "free fall prevention", GateCapitalProtection: "capital protection",
	} {
		row := SaleHeldByGapRow(gate, 12.5, 10.149, 0.5, 5, features...)
		want := Row{
			Message:  "smartTakeLoss: " + label + " sale held, print 10.15% under the anchor, over the gap of 5 tolerances (0.50%)",
			Price:    12.5,
			Gate:     gate,
			Event:    EventGapHeld,
			Features: []string{FeatureQuietDecline},
		}
		if !reflect.DeepEqual(row, want) {
			t.Errorf("%s: row = %+v, want %+v", gate, row, want)
		}
	}
	if got := SaleHeldByGapRow(GateFlatMarket, 12.5, 2.5, 1, 2.5).Message; got != "smartTakeLoss: flat market sale held, print 2.50% under the anchor, over the gap of 2.5 tolerances (1.00%)" {
		t.Errorf("a fractional floor must read as written, got %q", got)
	}

	row := SaleHeldByGapRow(GateSlowDecline, 12.5, 2.25, 1.25, 5, features...)
	features[0] = "changed"
	if row.Features[0] != FeatureQuietDecline {
		t.Errorf("the row must clone its features, got %v", row.Features)
	}
	_, event := Rows(flatMarketLadder(false), SaleHeldByGapRow(GateFlatMarket, 12.5, 2.25, 1.25, 5), testutil.At("13:00:00"))
	if data, ok := readEvent(event); !ok || data.Event != EventGapHeld || data.Price != 12.5 || data.Trigger != nil || !slices.Equal(data.Features, nil) {
		t.Errorf("the gap event must carry the fill as price and no trigger, got %+v", data)
	}
}

// The gapClosed row is "smartTakeLoss: <gate> gap closed, print <m>% under|over the anchor", m the
// absolute move at two decimals, "over" for a move above the anchor in the ladder's metric, under for the rest;
// at the newest fill under its gate, its event gapClosed with the fill as price, no trigger, features cloned.
func TestTheGapClosedRowIsTheGrammarOfEachGate(t *testing.T) {
	features := []string{FeatureQuietDecline}
	for gate, label := range map[string]string{
		GateSlowDecline: "slow decline", GateFlatMarket: "flat market",
		GateFreeFallPrevention: "free fall prevention", GateCapitalProtection: "capital protection",
	} {
		for _, c := range []struct {
			move float64
			text string
		}{
			{0.5, "print 0.50% over the anchor"}, {-0.1, "print 0.10% under the anchor"},
			{1.234, "print 1.23% over the anchor"}, {-1.236, "print 1.24% under the anchor"},
			{12, "print 12.00% over the anchor"}, {0.004, "print 0.00% over the anchor"}, {-0.004, "print 0.00% under the anchor"},
		} {
			row := GapClosedRow(gate, 12.5, c.move, features...)
			want := Row{
				Message:  "smartTakeLoss: " + label + " gap closed, " + c.text,
				Price:    12.5,
				Gate:     gate,
				Event:    EventGapClosed,
				Features: []string{FeatureQuietDecline},
			}
			if !reflect.DeepEqual(row, want) {
				t.Errorf("%s, move %v: row = %+v, want %+v", gate, c.move, row, want)
			}
		}
	}

	row := GapClosedRow(GateSlowDecline, 12.5, 0.5, features...)
	features[0] = "changed"
	if row.Features[0] != FeatureQuietDecline {
		t.Errorf("the row must clone its features, got %v", row.Features)
	}
	if got := GapClosedRow(GateFlatMarket, 12.5, 0.5); got.Features != nil {
		t.Errorf("a row with no features must carry none, got %v", got.Features)
	}
	logRow, event := Rows(flatMarketLadder(false), GapClosedRow(GateFlatMarket, 12.5, 0.5), testutil.At("13:00:00"))
	if string(event.Data) != `{"event":"gapClosed","price":12.5}` || event.Gate != GateFlatMarket || event.Kind() != EventGapClosed {
		t.Errorf("the gapClosed event must be the kind and the fill as price under its gate, got %+v with %s", event, event.Data)
	}
	if data, ok := readEvent(event); !ok || data.Event != EventGapClosed || data.Price != 12.5 || data.Trigger != nil || !slices.Equal(data.Features, nil) {
		t.Errorf("the gapClosed event must read back with the fill as price and no trigger, got %+v", data)
	}
	if logRow.Message != "smartTakeLoss: flat market gap closed, print 0.50% over the anchor" || logRow.Price != 12.5 {
		t.Errorf("the log row is the text at the fill, got %+v", logRow)
	}
}

// A print exactly at the anchor closes the gap with the move zero, as the tick computes it: the row
// reads "0.00% under the anchor", never a signed zero.
func TestAPrintAtTheAnchorClosesTheGapWithNoSignedZero(t *testing.T) {
	for _, gate := range saleGates {
		t.Run(gate.name, func(t *testing.T) {
			c := gate.build(t)
			withSaleGapTolerances(t, gapFor)
			withSaleGapWait(t, gapWait)
			first := gapSceneOf(c).opened(t)

			got := ApplyAt(first, "", first.PositionPrice, c.ai, confirmStart.Add(time.Second))
			if got.Position != "" || got.Reason != "" || got.Activated || len(got.Rows) != 1 {
				t.Fatalf("a print at the anchor holds and closes the gap with one row, got %+v", got)
			}
			if message := got.Rows[0].Message; !strings.HasSuffix(message, "gap closed, print 0.00% under the anchor") {
				t.Errorf("a move of zero must read without a sign, got %q", message)
			}
		})
	}
}
