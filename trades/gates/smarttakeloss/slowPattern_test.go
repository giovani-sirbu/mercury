package smarttakeloss

import (
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/slowpattern"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// stairTurns is the staircase of gates/slowpattern's own tests, three bars a
// leg — bars 0 to 30 — whose reading at each fill the tests below state: four
// down legs from lower highs by bar 24, five by bar 30.
var stairTurns = []float64{94, 100, 96, 98, 93, 95, 90, 92, 88, 89, 85}

// stairBounce is the same staircase with its last leg a rally instead, which
// reads no decline at bar 30.
var stairBounce = []float64{94, 100, 96, 98, 93, 95, 90, 92, 88, 110, 112}

// stairPrices are the w3s ladder's entry prices, the fills of stairLadder.
var stairPrices = []float64{201.46, 192.81, 188.58, 184.45, 179.78, 175.83, 171.97, 168.2}

// stairOpen is the open time of the staircase's bar: the fixture day's hours.
func stairOpen(bar int) time.Time {
	return testutil.At("00:00:00").Add(time.Duration(bar) * time.Hour)
}

// stairBlock serves bars 0 to lastBar of the staircase of turns as the block's
// series, with the sell band beside them.
func stairBlock(turns []float64, lastBar int) aggragates.SmartTakeLossIndicators {
	closes := []float64{turns[0]}
	for index := 1; index < len(turns); index++ {
		from, to := turns[index-1], turns[index]
		closes = append(closes, from+(to-from)/3, from+2*(to-from)/3, to)
	}
	block := aggragates.SmartTakeLossIndicators{SlowDeclineSellBand: slowDeclineBand}
	for bar := 0; bar <= lastBar && bar < len(closes); bar++ {
		block.SlowPatternOpens = append(block.SlowPatternOpens, stairOpen(bar).UnixMilli())
		block.SlowPatternCloses = append(block.SlowPatternCloses, closes[bar])
	}
	return block
}

// stairLadder is the w3s ladder with a fill inside each of these bars of the
// staircase, at the w3s prices, in order.
func stairLadder(bars ...int) aggragates.Trades {
	fills := make([]testutil.LadderFill, len(bars))
	for index, bar := range bars {
		fills[index] = testutil.LadderFill{Price: stairPrices[index], At: stairOpen(bar).Add(17*time.Minute + 9*time.Second)}
	}
	return testutil.LadderTrade(false, fills...)
}

// stairPending is the ladder the staircase reads on at bar 24, its fourth
// fill's bar, with the rows of the tick it went pending on written: pending
// and latched at its newest fill, the slow pattern running as shipped.
func stairPending(t *testing.T) aggragates.Trades {
	t.Helper()
	trade, got := engineTick(stairLadder(0, 9, 15, 24), "", slowDeclineBand-1, testutil.At("00:30:00"), withBlock(stairBlock(stairTurns, 24)))
	if got.SlowDecline == nil || got.SlowDecline.Event != EventPending {
		t.Fatalf("fixture drifted: the staircase must go pending at bar 24, got %+v", got)
	}
	return trade
}

// stairFifthFill is the pending, latched staircase ladder (stairPending) with a
// fifth fill stamped inside bar 30, the bar the staircase's second reading ends at.
func stairFifthFill(t *testing.T) aggragates.Trades {
	t.Helper()
	return withFill(stairPending(t), stairPrices[4], stairOpen(30).Add(17*time.Minute+9*time.Second))
}

// What the staircase reads at its fourth fill's bar (24) and at its fifth's
// (30), and what the rally in place of its last leg (stairBounce) breaks at the
// fifth: the window and each reading, as slowpattern names them.
var (
	patternReasonsAt24 = []string{"depth 1 to 4, 1h bars 2021-07-26 00:00 to 2021-07-27 00:00 UTC", "weight 0.49 over 0.33", "4 legs down of 3 needed", "3 lower highs of 2 needed", "largest leg 28% of the fall under 60%", "down 6.4% past 4.0%"}
	patternReasonsAt30 = []string{"depth 1 to 5, 1h bars 2021-07-26 00:00 to 2021-07-27 06:00 UTC", "weight 0.51 over 0.33", "5 legs down of 3 needed", "4 lower highs of 2 needed", "largest leg 23% of the fall under 60%", "down 9.6% past 4.0%"}
	patternBreaksAt30  = []string{"depth 1 to 5, 1h bars 2021-07-26 00:00 to 2021-07-27 06:00 UTC", "weight -0.29 under 0.33", "up 19.1% short of 4.0% down"}
)

// The read is bound to the bar that holds the newest fill, once that bar has
// closed: the bar before it reads nothing, the bar itself and the
// SlowPatternReadBars after it write the pending row, and a bar later than that
// writes none — a ladder already deep at deploy, or whose read bars got no tick,
// never triggers late.
func TestSlowPatternReadsOnlyInTheBarsAfterTheFill(t *testing.T) {
	trade := stairLadder(0, 9, 15, 24)
	for lastBar, want := range map[int]bool{23: false, 24: true, 24 + slowpattern.SlowPatternReadBars: true, 24 + slowpattern.SlowPatternReadBars + 1: false} {
		got := Apply(trade, "", slowDeclineBand-1, withBlock(stairBlock(append(stairTurns, 84, 83, 82, 81), lastBar)))
		if (got.SlowDecline != nil) != want {
			t.Errorf("series to bar %d: pending row %v, want %v", lastBar, got.SlowDecline != nil, want)
		}
	}
}

// No series, no read: a block without the keys, one that serves none, and one
// whose arrays differ in length leave the ladder as it was — no row, no sale.
func TestSlowPatternWaitsWithoutASeries(t *testing.T) {
	mismatched := stairBlock(stairTurns, 24)
	mismatched.SlowPatternCloses = mismatched.SlowPatternCloses[1:]
	for name, block := range map[string]aggragates.SmartTakeLossIndicators{
		"no keys":    {SlowDeclineSellBand: slowDeclineBand},
		"empty":      {SlowPatternOpens: []int64{}, SlowPatternCloses: []float64{}},
		"mismatched": mismatched,
	} {
		assertUntouched(t, Apply(stairLadder(0, 9, 15, 24), "", slowDeclineBand-1, withBlock(block)), "")
		if got := Apply(stairLadder(0, 9, 15, 24), "", slowDeclineBand+1, withBlock(block)); got.Reason != "" {
			t.Errorf("%s: a ladder not pending sells nothing at the band, got %+v", name, got)
		}
	}
}

// Only a long spot parent ladder from SlowPatternArmDepth fills is read: one
// fill short, an inverse ladder, a futures one, a child, a strategy without
// the flag and the switch off write nothing; an impasse strategy's parent is
// read like any other.
func TestSlowPatternWatchesLongSpotParentsFromTheArmDepth(t *testing.T) {
	block := withBlock(stairBlock(stairTurns, 24))
	for name, tc := range map[string]struct {
		mutate func(*aggragates.Trades)
		reads  bool
	}{
		"a long spot parent":          {func(*aggragates.Trades) {}, true},
		"an impasse strategy":         {func(trade *aggragates.Trades) { trade.Strategy.Params.Impasse = true }, true},
		"one fill short of the arm":   {func(trade *aggragates.Trades) { *trade = stairLadder(0, 9, 24) }, false},
		"a futures ladder":            {func(trade *aggragates.Trades) { trade.Strategy.TradeType = aggragates.Futures }, false},
		"a child":                     {func(trade *aggragates.Trades) { trade.ParentID = 7 }, false},
		"a strategy without the flag": {func(trade *aggragates.Trades) { trade.Strategy.Params.SmartTakeLoss = false }, false},
		"an inverse ladder": {func(trade *aggragates.Trades) {
			trade.Inverse = true
			for index := range trade.History {
				trade.History[index].Type = "SELL"
			}
		}, false},
	} {
		trade := stairLadder(0, 9, 15, 24)
		tc.mutate(&trade)
		if got := Apply(trade, "", slowDeclineBand-1, block); (got.SlowDecline != nil) != tc.reads {
			t.Errorf("%s: pending row %v, want %v", name, got.SlowDecline != nil, tc.reads)
		}
	}
	withSlowPatternDeclineExit(t, false)
	assertUntouched(t, Apply(stairLadder(0, 9, 15, 24), "", slowDeclineBand-1, block), "")
}

// The quiet slow decline's row keeps the slot on a tick that would judge the
// pattern's new fill too: the fill stays unjudged — the pattern writes nothing,
// and sells nothing at the band the quiet rule does not own — and the
// confirmation lands on the next tick, with no second latch.
func TestSlowPatternAJudgementWaitsForTheSlotTheQuietRuleHolds(t *testing.T) {
	block := stairBlock(stairTurns, 30)
	block.SlowDeclineExit, block.SlowDeclineExitReasons, block.SlowDeclineFillFrom = true, slowDeclineReasons, fillWindowFrom.UnixMilli()
	ticked, first := engineTick(stairFifthFill(t), "", underTheBand, stairOpen(31), withBlock(block))
	assertRow(t, first.SlowDecline, SlowDeclineMessage("buy", slowDeclineReasons), stairPrices[4])
	assertNoSale(t, first, "")
	if st := rebuildState(ticked); !st.slowDeclinePending || !st.slowPatternPending || st.slowPatternPendingFrom != stairPrices[3] || !slowPatternFillUnjudged(ticked, st) {
		t.Fatalf("the quiet rule is pending from the fifth fill, the pattern waits unjudged at the fourth, got %+v", st)
	}
	_, second := engineTick(ticked, "", underTheBand, stairOpen(31).Add(15*time.Minute), withBlock(block))
	assertRow(t, second.SlowDecline, SlowPatternMessage("buy", patternReasonsAt30), stairPrices[4])
	if second.Indecision != nil {
		t.Errorf("the latch is taken and never written twice, got %+v", second.Indecision)
	}
}

// A ladder the pattern made pending without a latch — the indecision direction
// was off when it went pending — is latched by the confirmation of its next fill
// once the direction watches it again: one latched row at that fill naming the
// rule and the reasons of the window that held, and no second one after it.
func TestSlowPatternLatchesAConfirmationOnceToo(t *testing.T) {
	withIndecisionDirection(t, false)
	pending := stairPending(t)
	if gridHas(pending, GateIndecision, EventLatched) {
		t.Fatal("fixture drifted: the ladder went pending without a latch")
	}
	withIndecisionDirection(t, true)
	fifth := withFill(pending, stairPrices[4], stairOpen(30).Add(17*time.Minute+9*time.Second))
	confirmed, got := engineTick(fifth, "", underTheBand, stairOpen(31), withBlock(stairBlock(stairTurns, 30)))
	assertRow(t, got.SlowDecline, SlowPatternMessage("buy", patternReasonsAt30), stairPrices[4])
	assertRow(t, got.Indecision, IndecisionMessage("buy", append([]string{"slow pattern decline"}, patternReasonsAt30...)), stairPrices[4])
	sixth := withFill(confirmed, stairPrices[5], stairOpen(36).Add(17*time.Minute))
	if again := Apply(sixth, "", underTheBand, withBlock(stairBlock(stairDecline, 36))); again.SlowDecline == nil || again.Indecision != nil {
		t.Errorf("the sixth fill's confirmation finds the ladder latched, got %+v", again)
	}
}
