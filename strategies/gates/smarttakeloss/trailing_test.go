package smarttakeloss

import (
	"fmt"
	"slices"
	"testing"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/dynamicparams"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// The trail: from stopLoss, once a gate activated it, the shared action answers one of three things
// on every tick from the anchor (trade.PositionPrice) and the held depth's row, the move negated for
// an inverse ladder: under minus the tolerance it sells, over the trailing take profit it re-anchors,
// in between it holds; the cap is not asked and a hold or a re-anchor writes no row. The expectations
// are stated by hand off the fixture row (tolerance 0.25, trailing take profit 1).

// trailRow is the settings row the trail reads for the trade: the newest fill's depth, or the base row.
func trailRow(trade aggragates.Trades) aggragates.StrategySettings {
	rows := trade.StrategyPair.StrategySettings
	return rows[ladder.SettingsIndexOrBase(rows, ladder.CountFilledEntries(trade)-1)]
}

// anchoredFor is the trade anchored so that a tick at price stands move percent from the anchor in
// the trail's metric (a share of the tick price), positive in the ladder's favour.
func anchoredFor(trade aggragates.Trades, price, move float64) aggragates.Trades {
	if trade.Inverse {
		trade.PositionPrice = price * (1 + move/100)
	} else {
		trade.PositionPrice = price * (1 - move/100)
	}
	return trade
}

// priceAtMove is the tick price that stands move percent from the anchor, the inverse
// of anchoredFor for a price asked of a given anchor.
func priceAtMove(trade aggragates.Trades, anchor, move float64) float64 {
	if trade.Inverse {
		return anchor / (1 + move/100)
	}
	return anchor / (1 - move/100)
}

// anchoredToHold, anchoredToSell and anchoredToReanchor anchor the trade so a tick at price is held,
// sold and re-anchored, none near a bound.
func anchoredToHold(trade aggragates.Trades, price float64) aggragates.Trades {
	row := trailRow(trade)
	return anchoredFor(trade, price, (row.TrailingTakeProfit-row.Tolerance)/2)
}

func anchoredToSell(trade aggragates.Trades, price float64) aggragates.Trades {
	return anchoredFor(trade, price, -(trailRow(trade).Tolerance + 1))
}

func anchoredToReanchor(trade aggragates.Trades, price float64) aggragates.Trades {
	return anchoredFor(trade, price, trailRow(trade).TrailingTakeProfit+1)
}

// restingAt is the trade resting in state with its anchor at anchor.
func restingAt(trade aggragates.Trades, state string, anchor float64) aggragates.Trades {
	trade = restingIn(trade, state)
	trade.PositionPrice = anchor
	return trade
}

// reanchoring and holdingTheTrail are the cases answering the trail: the re-anchor and
// the hold of a committed ladder, each the step alone, with no reason and no row.
func (c sellLossCase) reanchoring() sellLossCase {
	c.step = stepReanchor
	return c
}

func (c sellLossCase) holdingTheTrail() sellLossCase {
	c.step = stepHold
	return c
}

// trailCases are the three answers of the trail at price for a ladder resting in state
// with these triggers, each on the ladder anchored to give it: held, re-anchored, sold.
func trailCases(name string, trade aggragates.Trades, state string, price float64, triggers ...saleTrigger) []sellLossCase {
	return []sellLossCase{
		onLadder(name+", held between the bounds", anchoredToHold(trade, price), state, price, triggers...).holdingTheTrail(),
		onLadder(name+", re-anchored over the trail", anchoredToReanchor(trade, price), state, price, triggers...).reanchoring(),
		onLadder(name+", sold under the tolerance", anchoredToSell(trade, price), state, price, triggers...).selling(),
	}
}

// assertTrailHeld fails unless Apply held the committed ladder: nothing proposed, no reason, no activation, exactly these rows.
func assertTrailHeld(t *testing.T, got Result, rows ...Row) {
	t.Helper()
	if got.Position != "" || got.Reason != "" || got.Activated {
		t.Fatalf("expected the committed ladder held with no sale and no activation, got %+v", got)
	}
	assertRows(t, got, rows...)
}

// assertReanchored fails unless Apply re-anchored: Position "stopLoss" and Activated, no reason, exactly these rows.
func assertReanchored(t *testing.T, got Result, rows ...Row) {
	t.Helper()
	assertActivated(t, got, rows...)
}

// The three answers on both sides: the same move gives the same answer for a long and an inverse ladder.
func TestTrailingStepAnswersSellReanchorOrHoldOnBothSides(t *testing.T) {
	long, inverse := restingAt(flatMarketLadder(false), "stopLoss", 100), restingAt(flatMarketLadder(true), "stopLoss", 100)
	if row := trailRow(long); row.Tolerance != 0.25 || row.TrailingTakeProfit != 1 {
		t.Fatalf("fixture drifted: the row must carry a tolerance of 0.25 and a trailing take profit of 1, got %+v", row)
	}

	for _, c := range []struct {
		name  string
		trade aggragates.Trades
		price float64
		want  saleStep
	}{
		{"long, flat", long, 100, stepHold},
		{"long, a hair up", long, 100.1, stepHold},
		{"long, a hair down", long, 99.9, stepHold},
		{"long, a fall just inside the tolerance", long, 99.8, stepHold},
		{"long, a fall past the tolerance", long, 99.7, stepSell},
		{"long, a collapse", long, 50, stepSell},
		{"long, a rise just inside the trail", long, 100.9, stepHold},
		{"long, a rise past the trail", long, 101.2, stepReanchor},
		{"long, a spike", long, 200, stepReanchor},
		{"inverse, flat", inverse, 100, stepHold},
		{"inverse, a hair down", inverse, 99.9, stepHold},
		{"inverse, a hair up", inverse, 100.1, stepHold},
		{"inverse, a rise just inside the tolerance", inverse, 100.2, stepHold},
		{"inverse, a rise past the tolerance", inverse, 100.3, stepSell},
		{"inverse, a spike", inverse, 200, stepSell},
		{"inverse, a fall just inside the trail", inverse, 99.1, stepHold},
		{"inverse, a fall past the trail", inverse, 98.8, stepReanchor},
		{"inverse, a collapse", inverse, 50, stepReanchor},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := trailingStep(c.trade, c.price); got != c.want {
				t.Errorf("trailingStep at %v from an anchor of %v = %s, want %s", c.price, c.trade.PositionPrice, stepNames[got], stepNames[c.want])
			}
		})
	}

	for _, move := range []struct {
		percent float64
		want    saleStep
	}{{-20, stepSell}, {-1, stepSell}, {-0.3, stepSell}, {-0.1, stepHold}, {0, stepHold}, {0.5, stepHold}, {1.5, stepReanchor}, {20, stepReanchor}} {
		for _, side := range []aggragates.Trades{long, inverse} {
			trade := anchoredFor(side, 250, move.percent)
			if got := trailingStep(trade, 250); got != move.want {
				t.Errorf("inverse %v, a move of %v%% in the ladder's favour: trailingStep = %s, want %s", side.Inverse, move.percent, stepNames[got], stepNames[move.want])
			}
		}
	}
}

// A move exactly at a bound is inside it, compared strictly as the take profit row is: exactly the
// tolerance holds, a hair past sells, exactly the trailing take profit holds, a hair past re-anchors; the
// row is wide and the prices exact in floating point.
func TestTrailingStepAtTheBoundsHoldsAndJustPastThemActs(t *testing.T) {
	wide := func(inverse bool, anchor float64) aggragates.Trades {
		trade := restingAt(flatMarketLadder(inverse), "stopLoss", anchor)
		trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{{Percentage: 2.25, Tolerance: 25, TrailingTakeProfit: 50, Multiplier: 2, Depths: 8}}
		return trade
	}
	for _, c := range []struct {
		name    string
		inverse bool
		anchor  float64
		price   float64
		want    saleStep
	}{
		{"long, exactly the tolerance under the anchor", false, 100, 80, stepHold},
		{"long, one hair past the tolerance", false, 100, 79.9, stepSell},
		{"long, exactly the trail over the anchor", false, 100, 200, stepHold},
		{"long, one hair past the trail", false, 100, 200.5, stepReanchor},
		{"inverse, exactly the tolerance against the anchor", true, 150, 200, stepHold},
		{"inverse, one hair past the tolerance", true, 150, 200.5, stepSell},
		{"inverse, exactly the trail in favour", true, 300, 200, stepHold},
		{"inverse, one hair past the trail", true, 300, 199, stepReanchor},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := trailingStep(wide(c.inverse, c.anchor), c.price); got != c.want {
				t.Errorf("trailingStep at %v from an anchor of %v = %s, want %s", c.price, c.anchor, stepNames[got], stepNames[c.want])
			}
		})
	}
}

// An anchor at or under zero cannot be read as a move: the ladder is sold, whatever the price and side.
func TestTrailingStepWithTheAnchorAtOrUnderZeroSells(t *testing.T) {
	for _, inverse := range []bool{false, true} {
		for _, anchor := range []float64{0, -1, -1e9} {
			trade := restingAt(flatMarketLadder(inverse), "stopLoss", anchor)
			for _, price := range []float64{0.01, 1, 98, 100, 250, 1e9} {
				if got := trailingStep(trade, price); got != stepSell {
					t.Errorf("inverse %v, an anchor of %v, a print of %v: trailingStep = %s, want sell", inverse, anchor, price, stepNames[got])
				}
			}
			for _, state := range []string{"stopLoss", "forceTrailingStopLoss"} {
				runSellLoss(t, []sellLossCase{
					onLadder(fmt.Sprintf("inverse %v, %s, an anchor of %v", inverse, state, anchor), restingAt(trade, state, anchor), state, 100, activatedOf(slowTrigger(true)), activatedOf(lowTrigger(true))).selling(),
				})
			}
		}
	}
}

// A trade with no settings row has no bounds to read: the ladder is sold, with no index out of range;
// asked of the decision and the shared action directly, since Apply never reaches it.
func TestTrailingStepWithNoRowToReadSells(t *testing.T) {
	withSellLossIgnoreLastDepths(t, 0)
	for _, inverse := range []bool{false, true} {
		trade := restingAt(flatMarketLadder(inverse), "stopLoss", 100)
		trade.StrategyPair.StrategySettings = nil
		for _, price := range []float64{50, 100, 150} {
			if got := trailingStep(trade, price); got != stepSell {
				t.Errorf("inverse %v, no row, a print of %v: trailingStep = %s, want sell", inverse, price, stepNames[got])
			}
		}
		runSellLoss(t, []sellLossCase{
			onLadder(fmt.Sprintf("inverse %v, no row", inverse), trade, "stopLoss", 100, activatedOf(lowTrigger(true))).selling(),
		})
	}
}

// The bounds at the edge mirror the ladder's own row: a trailing take profit at or under zero
// re-anchors on any rise, a tolerance at or under zero sells on any fall, no clamping.
func TestTrailingStepBoundsAtOrUnderZeroActOnAnyMove(t *testing.T) {
	for _, inverse := range []bool{false, true} {
		for _, c := range []struct {
			name             string
			tolerance, trail float64
			move             float64
			want             saleStep
		}{
			{"both zero, flat", 0, 0, 0, stepHold},
			{"both zero, a hair in favour", 0, 0, 0.001, stepReanchor},
			{"both zero, a hair against", 0, 0, -0.001, stepSell},
			{"trail zero, a hair in favour", 0.25, 0, 0.001, stepReanchor},
			{"trail zero, flat", 0.25, 0, 0, stepHold},
			{"trail zero, a hair against, inside the tolerance", 0.25, 0, -0.001, stepHold},
			{"tolerance zero, a hair against", 0, 1, -0.001, stepSell},
			{"tolerance zero, a hair in favour, inside the trail", 0, 1, 0.001, stepHold},
			{"tolerance under zero, a hair against", -0.1, 1, -0.001, stepSell},
			{"trail under zero, a hair in favour", 0.25, -0.1, 0.001, stepReanchor},
		} {
			trade := restingAt(flatMarketLadder(inverse), "stopLoss", 100)
			trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{{Percentage: 2.25, Tolerance: c.tolerance, TrailingTakeProfit: c.trail, Multiplier: 2, Depths: 8}}
			trade = anchoredFor(trade, 100, c.move)
			if got := trailingStep(trade, 100); got != c.want {
				t.Errorf("inverse %v, %s: trailingStep = %s, want %s", inverse, c.name, stepNames[got], stepNames[c.want])
			}
		}
	}
}

// The bounds are the row of the held depth (the newest fill's, or the base row), never the last row:
// four rows and a ladder at every depth, just inside and just past each bound; a partial fill leaves the depth.
func TestTrailingStepReadsTheRowOfTheHeldDepth(t *testing.T) {
	rows := []aggragates.StrategySettings{
		{Percentage: 2, Tolerance: 0.1, TrailingTakeProfit: 0.2, Multiplier: 2, Depths: 8},
		{Percentage: 2, Tolerance: 0.3, TrailingTakeProfit: 0.4, Multiplier: 2, Depths: 8},
		{Percentage: 2, Tolerance: 0.5, TrailingTakeProfit: 0.6, Multiplier: 2, Depths: 8},
		{Percentage: 2, Tolerance: 0.7, TrailingTakeProfit: 0.8, Multiplier: 2, Depths: 8},
	}
	governing := map[int]int{1: 0, 2: 1, 3: 2, 4: 3, 5: 0, 6: 0, 7: 0}

	check := func(t *testing.T, trade aggragates.Trades, row int) {
		t.Helper()
		tolerance, trail := rows[row].Tolerance, rows[row].TrailingTakeProfit
		for _, c := range []struct {
			name string
			move float64
			want saleStep
		}{
			{"a fall just inside the tolerance", -0.9 * tolerance, stepHold},
			{"a fall just past the tolerance", -1.1 * tolerance, stepSell},
			{"a rise just inside the trail", 0.9 * trail, stepHold},
			{"a rise just past the trail", 1.1 * trail, stepReanchor},
		} {
			if got := trailingStep(anchoredFor(trade, 100, c.move), 100); got != c.want {
				t.Errorf("row %d (tolerance %v, trail %v), %s: trailingStep = %s, want %s", row, tolerance, trail, c.name, stepNames[got], stepNames[c.want])
			}
		}
	}

	for filled, row := range governing {
		trade := restingAt(depthLadder(filled), "stopLoss", 100)
		trade.StrategyPair.StrategySettings = rows
		t.Run(fmt.Sprintf("%d fills", filled), func(t *testing.T) { check(t, trade, row) })
	}

	t.Run("a partial fill of an order already counted", func(t *testing.T) {
		trade := restingAt(depthLadder(2), "stopLoss", 100)
		trade.StrategyPair.StrategySettings = rows
		trade.History = append(trade.History, aggragates.TradesHistory{Type: "BUY", Quantity: 0.5, Price: trade.History[1].Price, OrderId: trade.History[1].OrderId, CreatedAt: testutil.At("22:00:00")})
		if filled := ladder.CountFilledEntries(trade); filled != 2 {
			t.Fatalf("fixture drifted: the partial fill is the same order, got %d entries", filled)
		}
		check(t, trade, 1)
	})

	t.Run("a single row governs every depth", func(t *testing.T) {
		for filled := 1; filled <= lastDepthFills; filled++ {
			trade := restingAt(depthLadder(filled), "stopLoss", 100)
			trade.StrategyPair.StrategySettings = rows[2:3]
			check(t, trade, 2)
		}
	})
}

// The row is read from the rows the ladder trades, the raise included; a raise moves neither knob, so
// a raised ladder trails exactly as the unraised one, and the stored rows are never written.
func TestARaisedLadderTrailsByTheSameBoundsAsTheUnraisedOne(t *testing.T) {
	rows := []aggragates.StrategySettings{
		{Percentage: 2, Tolerance: 0.1, TrailingTakeProfit: 0.2, Multiplier: 2, Depths: 8},
		{Percentage: 2, Tolerance: 0.3, TrailingTakeProfit: 0.4, Multiplier: 2, Depths: 8},
		{Percentage: 2, Tolerance: 0.5, TrailingTakeProfit: 0.6, Multiplier: 2, Depths: 8},
	}
	plain := restingAt(depthLadder(3), "stopLoss", 100)
	plain.StrategyPair.StrategySettings = rows

	raised := plain
	raised.Strategy.Params.DynamicParams, raised.Strategy.TradeType = true, aggragates.Spot
	row, event := dynamicparams.Opened{Points: 0.7, Depths: raisedDepths}.Rows(raised, raised.PositionPrice, testutil.At("08:00:00"))
	raised = aggragates.AppendStrategyRow(raised, row, event)
	if traded, isRaised := dynamicparams.RaisedSettings(raised); !isRaised || traded[2].Percentage != rows[2].Percentage+0.7 || traded[2].Tolerance != rows[2].Tolerance {
		t.Fatalf("fixture drifted: the ladder must trade rows raised in their percentage and depths alone, got %+v (raised %v)", traded, isRaised)
	}

	for _, c := range []struct {
		name string
		move float64
		want saleStep
	}{
		{"a fall inside the held row's tolerance", -0.45, stepHold},
		{"a fall past the held row's tolerance", -0.55, stepSell},
		{"a rise inside the held row's trail", 0.55, stepHold},
		{"a rise past the held row's trail", 0.65, stepReanchor},
	} {
		for name, trade := range map[string]aggragates.Trades{"unraised": plain, "raised": raised} {
			if got := trailingStep(anchoredFor(trade, 100, c.move), 100); got != c.want {
				t.Errorf("%s, %s: trailingStep = %s, want %s", name, c.name, stepNames[got], stepNames[c.want])
			}
		}
	}
	if stored := raised.StrategyPair.StrategySettings; stored[2].Percentage != 2 || stored[2].Depths != 8 {
		t.Errorf("the trade keeps its own rows, got %+v", stored[2])
	}
}

// Through the shared action the trail names no row and no reason but the sale's (the first trigger
// that activated it); a hold and a re-anchor have none even where a held row is due, whatever the
// cap. The same in the force-trailing state.
func TestTheTrailNamesTheTriggerThatActivatedOnASaleAndWritesNoRow(t *testing.T) {
	withSellLossIgnoreLastDepths(t, 0)
	trade := flatMarketLadder(false)
	far := ladder.AverageEntryPrice(trade) * lossFactor
	slow, low := slowTrigger(true), lowTrigger(true)

	for _, capPct := range []float64{0, shippedLoss} {
		withAllowedLoss(t, capPct)
		var cases []sellLossCase
		for _, state := range []string{"stopLoss", "forceTrailingStopLoss"} {
			for _, set := range []struct {
				name     string
				triggers []saleTrigger
			}{
				{"both activated, the slow decline first", []saleTrigger{activatedOf(slow), activatedOf(low)}},
				{"both activated, the flat market first", []saleTrigger{activatedOf(low), activatedOf(slow)}},
				{"the slow decline activated alone", []saleTrigger{activatedOf(slow)}},
				{"the flat market activated alone", []saleTrigger{activatedOf(low)}},
				{"the second activated, the first not", []saleTrigger{slow, activatedOf(low)}},
				{"the first activated, the second not", []saleTrigger{activatedOf(slow), low}},
			} {
				for _, price := range []float64{far, ladder.AverageEntryPrice(trade), ladder.AverageEntryPrice(trade) * gainFactor} {
					name := fmt.Sprintf("cap %v, %s, %s, at %v", capPct, state, set.name, price)
					cases = append(cases, trailCases(name, trade, state, price, set.triggers...)...)
				}
			}
		}
		runSellLoss(t, cases)
	}
}

// A self-armed stopLoss is no committed ladder and is not trailed: sold on a bounce and within the cap,
// held over it; the same ladder a gate activated is held between its bounds.
func TestTheNaturalStopLossIsNotTrailed(t *testing.T) {
	withSellLossIgnoreLastDepths(t, 0)
	withAllowedLoss(t, shippedLoss)
	farOverTheCap(t, shippedLoss)
	trade := flatMarketLadder(false)
	fill := trade.PositionPrice
	p := roundLadderPrices(t, trade)

	var cases []sellLossCase
	for _, state := range []string{"stopLoss", "forceTrailingStopLoss"} {
		for _, stand := range eachStanding() {
			committed := make([]saleTrigger, len(stand.triggers))
			for index, trigger := range stand.triggers {
				committed[index] = activatedOf(trigger)
			}
			for name, price := range map[string]float64{"a gain": p.gain, "the break even": p.breakEven, "within the cap": p.within, "exactly at the cap": p.atTheCap} {
				anchored := anchoredToHold(trade, price)
				label := state + ", " + stand.name + ", " + name
				cases = append(cases,
					onLadder(label+", natural", anchored, state, price, stand.triggers...).selling(),
					onLadder(label+", committed", anchored, state, price, committed...).holdingTheTrail(),
				)
			}
			for name, price := range map[string]float64{"one price step over the cap": p.over, "far over the cap": p.far} {
				anchored := anchoredToHold(trade, price)
				label := state + ", " + stand.name + ", " + name
				cases = append(cases,
					onLadder(label+", natural", anchored, state, price, stand.triggers...).holdingByLoss(fill, lossPctOf(trade, price), shippedLoss),
					onLadder(label+", committed", anchored, state, price, committed...).holdingTheTrail(),
				)
			}
		}
	}
	runSellLoss(t, cases)
}

// Through Apply the trail mirrors on an inverse ladder: sold on a fall for a long, on a rise for an
// inverse, every add-side proposal answered the same.
func TestTheTrailMirrorsOnAnInverseLadderThroughApply(t *testing.T) {
	flatMarketAlone(t)
	withSellLossIgnoreLastDepths(t, SellLossIgnoreLastDepths)
	for _, c := range []struct {
		inverse bool
		sells   float64
		holds   []float64
		rises   float64
	}{
		{false, 99.7, []float64{99.9, 100, 100.5}, 101.2},
		{true, 100.3, []float64{100.1, 100, 99.5}, 98.8},
	} {
		trade := flatMarketLadder(c.inverse)
		ai := flatMarketMarket(trade)
		committed := committedBy(withRows(trade, triggeredBy(trade, ai)), FlatMarketStopLossActivatedRow(trade.PositionPrice, -0.1))
		committed.PositionPrice = 100
		for _, position := range addSideProposals {
			assertSold(t, Apply(committed, position, c.sells, ai), reasonFlatMarket)
			assertReanchored(t, Apply(committed, position, c.rises, ai))
			for _, price := range c.holds {
				assertTrailHeld(t, Apply(committed, position, price, ai))
			}
		}
	}
}

// A hold replaces whatever the ladder proposes with nothing and keeps the tick's rows (a latch is
// written beside it); a re-anchor replaces them with the stopLoss activated and writes no row.
func TestAHoldReplacesWhateverTheLadderProposesAndKeepsTheRowsOfTheTick(t *testing.T) {
	flatMarketAlone(t)
	withEarlyTakeProfit(t, true)
	withSellLossIgnoreLastDepths(t, SellLossIgnoreLastDepths)
	trade := orderLadder()
	fill, price := trade.PositionPrice, 99.9
	latching := withBlock(flatMarketOver(indecisionReading().SmartTakeLoss, trade))
	latched := LatchedRow(fill, indecisionFinding(indecisionReasons))
	committed := committedBy(withRows(trade, triggeredBy(trade, latching)), FlatMarketStopLossActivatedRow(fill, -0.1))

	for _, position := range addSideProposals {
		assertTrailHeld(t, Apply(anchoredToHold(committed, price), position, price, latching), latched)
		assertReanchored(t, Apply(anchoredToReanchor(committed, price), position, price, latching), latched)
		assertSoldWith(t, Apply(anchoredToSell(committed, price), position, price, latching), reasonFlatMarket, latched)

		latchedAlready := withRow(committed, latched, testutil.At("13:00:00"))
		assertTrailHeld(t, Apply(anchoredToHold(latchedAlready, price), position, price, latching))
	}
}

// The ladder's own closes are never replaced, a hold and a re-anchor included: a committed ladder
// rests in stopLoss, where a take profit is protected too.
func TestAHoldAndAReanchorReplaceNoCloseTheLadderMakes(t *testing.T) {
	flatMarketAlone(t)
	withSellLossIgnoreLastDepths(t, SellLossIgnoreLastDepths)
	trade := flatMarketLadder(false)
	ai := flatMarketMarket(trade)
	committed := committedBy(withRows(trade, triggeredBy(trade, ai)), FlatMarketStopLossActivatedRow(trade.PositionPrice, -0.1))
	const price = 99.9

	for _, answer := range []func(aggragates.Trades, float64) aggragates.Trades{anchoredToHold, anchoredToReanchor, anchoredToSell} {
		for _, position := range slices.Concat(closingProposals, takeProfitProposals) {
			assertUntouched(t, Apply(answer(committed, price), position, price, ai), position)
		}
		for _, state := range []string{"takeProfit", "forceTrailingTakeProfit", "sellLoss", "sell", "sellParent", "impasse"} {
			assertUntouched(t, Apply(restingIn(answer(committed, price), state), "", price, ai), "")
		}
	}
}

// A re-anchor moves the anchor to the tick and keeps the state, so the next bounds are measured
// from the higher price: the activation print holds, a rise re-anchors, a fall the old anchor would
// re-anchor on holds, a fall under the new anchor's tolerance sells; the flat market episode stands
// through it and every tick is rebuilt from the trade alone.
func TestAReanchorMovesTheAnchorAndTheNextBoundsAreMeasuredFromIt(t *testing.T) {
	flatMarketAlone(t)
	withSellLossIgnoreLastDepths(t, SellLossIgnoreLastDepths)
	trade := orderLadder()
	ai, gone := flatMarketMarket(trade), withBlock(aggragates.SmartTakeLossIndicators{})
	fill := trade.PositionPrice
	atTheCap, _ := sellLossPrices(t, trade)
	row := trailRow(trade)
	now := testutil.At("13:00:00")

	trade, got := engineStep(trade, "", atTheCap, now, ai)
	assertActivated(t, got, triggeredBy(orderLadder(), ai), FlatMarketStopLossActivatedRow(fill, -lossPctOf(orderLadder(), atTheCap)))
	if trade.PositionType != "stopLoss" || trade.PositionPrice != atTheCap {
		t.Fatalf("the activation leaves the ladder in stopLoss anchored at the print, got %q at %v", trade.PositionType, trade.PositionPrice)
	}
	rows := len(trade.Logs)

	for _, position := range addSideProposals {
		assertTrailHeld(t, Apply(trade, position, atTheCap, gone))
	}

	first := priceAtMove(trade, atTheCap, 1.5*row.TrailingTakeProfit)
	trade, got = engineStep(trade, "", first, now, gone)
	assertReanchored(t, got)
	if trade.PositionType != "stopLoss" || trade.PositionPrice != first || len(trade.Logs) != rows {
		t.Fatalf("a re-anchor moves the anchor to the print, keeps the state and writes no row, got %q at %v with %d rows", trade.PositionType, trade.PositionPrice, len(trade.Logs))
	}

	back := priceAtMove(trade, first, -0.5*row.Tolerance)
	if old := moveAgainst(back, atTheCap); old <= row.TrailingTakeProfit {
		t.Fatalf("fixture drifted: the old anchor would re-anchor on a print of %v, got a move of %v", back, old)
	}
	for _, position := range addSideProposals {
		assertTrailHeld(t, Apply(trade, position, back, gone))
	}

	second := priceAtMove(trade, first, 1.5*row.TrailingTakeProfit)
	trade, got = engineStep(trade, "", second, now, gone)
	assertReanchored(t, got)
	if trade.PositionPrice != second {
		t.Fatalf("a second re-anchor moves the anchor again, got %v", trade.PositionPrice)
	}

	sale := priceAtMove(trade, second, -1.5*row.Tolerance)
	if sale <= atTheCap || sale <= ladder.AverageEntryPrice(trade) {
		t.Fatalf("fixture drifted: the sale must sit over the first anchor and over break even, got %v", sale)
	}
	for _, position := range addSideProposals {
		assertSold(t, Apply(trade, position, sale, gone), reasonFlatMarket)
	}

	state := rebuildState(trade)
	if !state.flatMarketActive || !state.flatMarketActivated || state.flatMarketFrom != fill {
		t.Fatalf("the episode stands activated at the newest fill through the holds and the re-anchors, got %+v", state)
	}
}

// Capital protection's episode trails like the others once it activated: the committed ladder is
// held, re-anchored or sold by its anchor alone, the sale under capital protection's reason and the
// trend flag no longer read. With the flat market trigger standing beside it the ladder still
// trails — any activated trigger commits the sale — and the sale is named by the trigger that
// activated it, capital protection's, with the depth bound off and once the bound holds the flat
// market gate at the last depth alike.
func TestCapitalProtectionTrailsOnceItActivated(t *testing.T) {
	capitalProtectionAlone(t)
	withSellLossIgnoreLastDepths(t, 0)
	trade := capitalProtectionLadder(false)
	committed := committedBy(withRows(trade, capitalProtectionTriggered(trade)), CapitalProtectionStopLossActivatedRow(trade.PositionPrice, -0.5))
	const price = 99.9
	if st := rebuildState(committed); !st.capitalProtectionWatched || !st.capitalProtectionActivated {
		t.Fatalf("fixture drifted: the ladder must be at its last depth with capital protection's stopLoss activated, got %+v", st)
	}

	for _, reading := range []aggragates.AIIndicators{withBlock(solBlock()), nothingServed()} {
		assertTrailHeld(t, Apply(anchoredToHold(committed, price), "", price, reading))
		assertReanchored(t, Apply(anchoredToReanchor(committed, price), "", price, reading))
		assertSold(t, Apply(anchoredToSell(committed, price), "", price, reading), reasonCapitalProtection)
	}

	withFlatMarket(t, true)
	market := flatMarketMarket(trade)
	if !flatMarketReading(trade, market.SmartTakeLoss, market).Holds {
		t.Fatal("fixture drifted: the flat market reading must hold")
	}
	assertTrailHeld(t, Apply(anchoredToHold(committed, price), "", price, market), triggeredBy(trade, market))
	assertReanchored(t, Apply(anchoredToReanchor(committed, price), "", price, market), triggeredBy(trade, market))
	assertSoldWith(t, Apply(anchoredToSell(committed, price), "", price, market), reasonCapitalProtection, triggeredBy(trade, market))
	withSellLossIgnoreLastDepths(t, 1)
	assertSoldWith(t, Apply(anchoredToSell(committed, price), "", price, market), reasonCapitalProtection, triggeredBy(trade, market))
}

// trailAnswer reduces what Apply made of a committed ladder to the trail's three answers and the sale's reason.
func trailAnswer(got Result) (saleStep, string) {
	switch {
	case got.Reason != "":
		return stepSell, got.Reason
	case got.Activated:
		return stepReanchor, ""
	case got.Position == "":
		return stepHold, ""
	}
	return stepNone, ""
}

// The trail does not depend on the declines switched on beside the gate that activated: under every
// combination of the five switches a ladder both gates activated is sold, re-anchored or held by its
// anchor alone, and the sale is named by the slow decline gate when the quiet decline is on, else the flat market gate.
func TestTheTrailAnswersTheSameUnderEveryCombinationOfTheDeclines(t *testing.T) {
	flatMarketAlone(t)
	withSellLossIgnoreLastDepths(t, SellLossIgnoreLastDepths)
	both := tableStands[2]
	trade, ai := tableLadder(both, float64(len(roundPrices)+SellLossIgnoreLastDepths+4))
	committed := both.activatedOn(trade)
	const price = 99.9

	for _, switches := range gridSwitches() {
		setGridSwitches(t, switches)
		sale := reasonFlatMarket
		if switches[0] {
			sale = reasonSlowDecline
		}
		for _, c := range []struct {
			name   string
			trade  aggragates.Trades
			answer saleStep
			reason string
		}{
			{"held", anchoredToHold(committed, price), stepHold, ""},
			{"re-anchored", anchoredToReanchor(committed, price), stepReanchor, ""},
			{"sold", anchoredToSell(committed, price), stepSell, sale},
		} {
			for _, position := range addSideProposals {
				got := Apply(c.trade, position, price, ai)
				if answer, reason := trailAnswer(got); answer != c.answer || reason != c.reason {
					t.Fatalf("switches %v, %s, %q: the trail answered %s (reason %q), want %s (reason %q): %+v", switches, c.name, position, stepNames[answer], reason, stepNames[c.answer], c.reason, got)
				}
				for _, row := range got.Rows {
					if isShared(row) {
						t.Fatalf("switches %v, %s, %q: the trail writes no row of the shared action's, got %+v", switches, c.name, position, row)
					}
				}
			}
		}
	}
}

// ExitReached reads a committed ladder as an exit only where the trail sells; a hold and a re-anchor
// are none, long or inverse, served or not, and a self-armed stopLoss is an exit at the bounce the committed one holds at.
func TestExitReachedIsTrueOnlyWhereTheTrailSells(t *testing.T) {
	flatMarketAlone(t)
	withSellLossIgnoreLastDepths(t, SellLossIgnoreLastDepths)
	for _, inverse := range []bool{false, true} {
		trade := flatMarketLadder(inverse)
		ai := flatMarketMarket(trade)
		average := ladder.AverageEntryPrice(trade)
		price, gain := 100.0, average*gainFactor
		if inverse {
			gain = average / gainFactor
		}
		opened := withRows(trade, triggeredBy(trade, ai))
		committed := committedBy(opened, FlatMarketStopLossActivatedRow(trade.PositionPrice, -0.1))

		for _, c := range []struct {
			name  string
			trade aggragates.Trades
			exit  bool
		}{
			{"held between the bounds", anchoredToHold(committed, price), false},
			{"re-anchored over the trail", anchoredToReanchor(committed, price), false},
			{"sold under the tolerance", anchoredToSell(committed, price), true},
		} {
			logs, events := len(c.trade.Logs), len(c.trade.StrategyEvents)
			for name, block := range map[string]aggragates.SmartTakeLossIndicators{"the market served": ai.SmartTakeLoss, "none served": {}} {
				if got := ExitReached(c.trade, price, block); got != c.exit {
					t.Errorf("inverse %v, %s, %s: ExitReached = %v, want %v", inverse, c.name, name, got, c.exit)
				}
			}
			if sold := Apply(c.trade, "", price, ai).Reason != ""; sold != c.exit {
				t.Errorf("inverse %v, %s: Apply sells %v, want %v", inverse, c.name, sold, c.exit)
			}
			if len(c.trade.Logs) != logs || len(c.trade.StrategyEvents) != events {
				t.Errorf("inverse %v, %s: ExitReached must write nothing", inverse, c.name)
			}
		}

		natural := restingAt(opened, "stopLoss", 100)
		if !ExitReached(anchoredToHold(natural, gain), gain, ai.SmartTakeLoss) {
			t.Errorf("inverse %v: a ladder that armed itself in stopLoss is sold at a gain, where the committed ladder is held", inverse)
		}
	}
}
