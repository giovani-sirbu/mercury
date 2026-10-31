package smartsell

import "testing"

// The trend's side of the every-tick read (swingDeclineReads): a Trend Direction
// that turns down far past the newest fill's bar triggers on its own tick, and a
// trend that misses the need is asked first and reads no window. The staircases
// and the helpers (lastBarBlock, readsAsOf, firstHeldBar, needsAScenario) are those
// of swingDeclineEveryTick_test.go.

// farBars is how many bars past the newest fill's bar the late reads below are
// taken: far beyond any bound the read was once held to.
const farBars = 30

// A trend that turns down late triggers on its own tick, however far past the
// newest fill: the staircase holds from the third fill's bar on, the Trend
// Direction reads a row short of the need until farBars past that bar and meets it
// then, and the tick of that bar writes the pending row, no tick before. The ticks
// run at a print over the ladder's break even (gainPrint) with a buy order resting, where a ladder in buy is
// activated nothing and the pending row stands alone; the swing decline and the
// early take profit (the pending row names its latch) are switched on for the
// test, so that it reads the same at every tuning.
func TestSwingDeclineATrendThatTurnsDownFarPastTheNewestFillStillTriggers(t *testing.T) {
	needsTheTrend(t)
	withLinearDecline(t, false)
	withSwingDecline(t, true)
	withEarlyTakeProfit(t, true)
	trade, fillBar := stairLadder(0, 9, 15), 15
	turn := fillBar + farBars
	needsAScenario(t, rebuildState(trade).swingDeclineWatched && firstHeldBar(trade, stairDecline, fillBar, turn-1, patternTrendDown) >= 0, "a staircase that holds before the trend turns")

	for bar := fillBar; bar < turn; bar++ {
		assertUntouched(t, applyAtAGain(trade, "", withBlock(lastBarBlock(stairDecline, bar, shortTrendDown))), "")
	}
	block := lastBarBlock(stairDecline, turn, patternTrendDown)
	reading, holds := readsAsOf(trade, block)
	needsAScenario(t, holds, "a staircase that holds when the trend turns")
	assertRow(t, applyAtAGain(trade, "", withBlock(block)), PendingRow(stairPrices[2], indecisionFinding(nil), swingFinding(reading.Reasons)))
}

// A trend that misses the need cannot trigger, so the gate asks the trend first and
// reads no window for it: under every trend the answer is TriggerAtLastBar's, and
// one short of the need allocates nothing — not even the fills the detector reads —
// where reading the windows does.
func TestSwingDeclineAsksTheTrendFirstAndAnswersAsTriggerAtLastBarDoes(t *testing.T) {
	trade := stairFourth()
	st := rebuildState(trade)
	for _, last := range []int{23, 24, 30, 36, 48} {
		for _, valid := range []int{0, patternTrendValid} {
			for down := 0; down <= patternTrendValid; down++ {
				block := withSmcTrend(stairBlock(stairDecline, last), down, valid)
				_, want := readsAsOf(trade, block)
				if _, got := swingDeclineReads(st, block); got != want {
					t.Errorf("series to bar %d, trend %d/%d: the gate reads holds=%v, TriggerAtLastBar %v", last, down, valid, got, want)
				}
			}
		}
	}
	needsTheTrend(t)
	short, met := lastBarBlock(stairDecline, 36, shortTrendDown), lastBarBlock(stairDecline, 36, patternTrendDown)
	if allocs := testing.AllocsPerRun(50, func() { swingDeclineReads(st, short) }); allocs != 0 {
		t.Errorf("a trend short of the need reads no window and builds nothing, got %v allocations", allocs)
	}
	if allocs := testing.AllocsPerRun(50, func() { readsAsOf(trade, met) }); allocs == 0 {
		t.Error("fixture drifted: reading the windows allocates, which a trend short of the need avoids")
	}
}
