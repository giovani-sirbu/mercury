package smarttakeloss

import (
	"slices"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// withFlatMarketResetOnNewDepth, withSwingDeclineResetOnNewDepth and withLinearDeclineResetOnNewDepth
// are withIndecision for each gate's ResetOnNewDepth switch.
func withFlatMarketResetOnNewDepth(t *testing.T, on bool) {
	t.Helper()
	previous := flatMarketResetOnNewDepth
	flatMarketResetOnNewDepth = on
	t.Cleanup(func() { flatMarketResetOnNewDepth = previous })
}

func withSwingDeclineResetOnNewDepth(t *testing.T, on bool) {
	t.Helper()
	previous := swingDeclineResetOnNewDepth
	swingDeclineResetOnNewDepth = on
	t.Cleanup(func() { swingDeclineResetOnNewDepth = previous })
}

func withLinearDeclineResetOnNewDepth(t *testing.T, on bool) {
	t.Helper()
	previous := linearDeclineResetOnNewDepth
	linearDeclineResetOnNewDepth = on
	t.Cleanup(func() { linearDeclineResetOnNewDepth = previous })
}

// barsGone is the trade's market with the flat market bars taken away and the trend kept read, so
// the reading misses while the shared sale still reads its trigger.
func barsGone(trade aggragates.Trades) aggragates.AIIndicators {
	block := flatMarketMarket(trade).SmartTakeLoss
	block.FlatMarketBarOpens, block.FlatMarketOpenPrices, block.FlatMarketHighs, block.FlatMarketLows, block.FlatMarketCloses = nil, nil, nil, nil, nil
	if block.SmcTrendValid == 0 {
		block.SmcTrendValid, block.SmcTrendDown = everyRow, everyRow
	}
	return withBlock(block)
}

// With its reset off, a new depth leaves an open flat market episode standing: no reset row, no
// fresh reading, the trigger stands for the cooldown (Triggered) and the shared sale acts on at the
// new fill; with the reset on the same tick writes the reset row.
func TestTheFlatMarketEpisodeStandsAtTheNewDepthWithTheResetOff(t *testing.T) {
	flatMarketAlone(t)
	withFlatMarketResetOnNewDepth(t, false)
	start := flatMarketLadder(false)
	open := withRows(start, triggeredBy(start, flatMarketMarket(start)))
	deeper := deeperBy(open, 97)
	_, oneStepLower := flatMarketPrices(t, deeper)
	gone := barsGone(deeper)

	assertUntouched(t, Apply(withRestingOrder(deeper), "buy", oneStepLower, gone), "buy")
	if st := rebuildState(deeper); !st.flatMarketActive || st.flatMarketFrom != start.PositionPrice || !Triggered(deeper) {
		t.Fatalf("the episode stands at its own fill and the gate reads triggered, got %+v", st)
	}
	assertWaits(t, Apply(deeper, "buy", oneStepLower, gone), "buy", FlatMarketSaleHeldRow(97, -handLossPct(deeper, oneStepLower)))

	t.Run("reset on", func(t *testing.T) {
		withFlatMarketResetOnNewDepth(t, true)
		if Triggered(deeper) {
			t.Fatal("an episode at the old fill is no gate triggered while the reset is due")
		}
		assertWaits(t, Apply(withRestingOrder(deeper), "buy", oneStepLower, gone), "buy", FlatMarketResetRow(97))
	})
}

// With its reset off, the swing decline's pending stands through a new fill: the fill counts as
// judged, no row is written whatever the series reads, the trigger stands and the shared action
// acts; with the reset on the same series cancels the ladder.
func TestTheSwingDeclinePendingStandsAtTheNewFillWithTheResetOff(t *testing.T) {
	withBothDeclines(t)
	withQuietDecline(t, false)
	withSwingDeclineResetOnNewDepth(t, false)
	fifth := stairFifthFill(t)
	st := rebuildState(fifth)
	if !st.slowDeclinePending || !slices.Equal(st.pendingFeatures, []string{FeatureSwingDecline}) || slowDeclineFillUnjudged(fifth, st) || !slowDeclineTriggered(fifth, st) || !Triggered(fifth) {
		t.Fatalf("pending on the swing alone, the fifth fill counting as judged and the trigger standing, got %+v", st)
	}
	broken := withBlock(stairBlock(stairBounce, 30))
	for _, position := range everyProposal() {
		assertUntouched(t, applyAtAGain(fifth, position, broken), position)
	}
	assertActedOn(t, fifth, broken)

	t.Run("reset on", func(t *testing.T) {
		withSwingDeclineResetOnNewDepth(t, true)
		if st := rebuildState(fifth); !slowDeclineFillUnjudged(fifth, st) || slowDeclineTriggered(fifth, st) {
			t.Fatalf("the fifth fill is unjudged again and the trigger waits, got %+v", st)
		}
		if got := applyAtAGain(fifth, "", broken); len(got.Rows) != 1 || got.Rows[0].Event != EventCancelled {
			t.Fatalf("the series cancels the ladder at the new fill, got %+v", got.Rows)
		}
	})
}

// With its reset off, the linear decline's pending stands through a new fill: the fill counts as
// judged and the tick writes no confirmation; with the reset on the fill on the line is confirmed.
func TestTheLinearDeclinePendingStandsAtTheNewFillWithTheResetOff(t *testing.T) {
	onlyTheLinearDecline(t)
	withLinearDecline(t, true)
	withSellLossStartAtDepth(t, 0)
	withLinearDeclineResetOnNewDepth(t, false)
	trade, block := linearScene(linearTrendMet)
	pending := PendingRow(trade.PositionPrice, linearFinding(linearReading(t, trade, block).Reasons), indecisionFinding(nil))
	deeper := linearLadder(linearSlope, linearDepth+1)
	deeperBlock := linearBlock(linearSlope, linearDepth+1, linearTail, linearTrendMet)
	judged := withFill(withRows(linearLadder(linearSlope, linearDepth), pending), deeper.PositionPrice, rebuildState(deeper).lastFill().At)
	if st := rebuildState(judged); !st.slowDeclinePending || slowDeclineFillUnjudged(judged, st) || !Triggered(judged) {
		t.Fatalf("pending on the line, the new fill counting as judged and the trigger standing, got %+v", st)
	}
	assertUntouched(t, applyAtAGain(judged, "", withBlock(deeperBlock)), "")

	t.Run("reset on", func(t *testing.T) {
		withLinearDeclineResetOnNewDepth(t, true)
		confirmed := ConfirmedRow(deeper.PositionPrice, linearFinding(linearReading(t, deeper, deeperBlock).Reasons))
		assertRow(t, applyAtAGain(judged, "", withBlock(deeperBlock)), confirmed)
	})
}
