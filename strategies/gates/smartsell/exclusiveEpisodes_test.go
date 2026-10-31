package smartsell

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/freefallprevention"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/swingdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// bothReadingsMarket is the payload on which the flat market's range and line both hold for the
// trade: the range's quiet window, with its wicks, read by the line as a flat one.
func bothReadingsMarket(t *testing.T, trade aggragates.Trades) aggragates.AIIndicators {
	t.Helper()
	side := aggragates.EntrySide(trade, aggragates.AIIndicators{})
	block := flatMarketBlock(basePercentage(trade), side)
	ai := withBlock(block)
	if reading := flatMarketReading(trade, block, ai); !reading.Range.Holds || !reading.Line.Holds {
		t.Fatal("fixture drifted: both readings must hold on the market")
	}
	return ai
}

// One sale gate holds an episode at a time: on a market both readings hold on, the flat market
// opens one episode, named by the range, first in order, and reads nothing on that tick or after;
// an episode already open, the flat market's or a pending slow decline, keeps the other gate from
// opening; capital protection opens beside any of them.
func TestASaleGateOpensNoEpisodeWhileAnotherIsOpen(t *testing.T) {
	flatMarketLineAlone(t)
	trade := flatMarketLadder(false)
	ai := bothReadingsMarket(t, trade)
	resting := withRestingOrder(trade)

	got := Apply(resting, "buy", trade.PositionPrice, ai)
	assertWaits(t, got, "buy", triggeredBy(trade, ai))
	if st := rebuildState(withRows(trade, got.Rows...)); !st.flatMarketActive {
		t.Fatalf("the flat market opens one episode, got %+v", st)
	}
	if reasons := flatMarketReading(trade, ai.SmartSell, ai).Reasons; len(reasons) == 0 || !strings.HasPrefix(reasons[0], "range on ") {
		t.Fatalf("the range, first in order, names the episode, got %q", reasons)
	}
	opened := withRows(trade, got.Rows...)
	assertUntouched(t, Apply(withRestingOrder(opened), "buy", trade.PositionPrice, ai), "buy")

	withQuietDecline(t, true)
	pending := withRows(trade, quietPending(trade.PositionPrice))
	if !rebuildState(pending).slowDeclinePending {
		t.Fatal("fixture drifted: the quiet decline must watch the ladder")
	}
	assertUntouched(t, Apply(withRestingOrder(pending), "buy", trade.PositionPrice, ai), "buy")
	withQuietDecline(t, false)

	withCapitalProtectionExit(t, true)
	last := capitalProtectionLadder(false)
	block := bothReadingsMarket(t, last).SmartSell
	block.CapitalProtectionSmcBearish = true
	beside := withRows(last, flatMarketLineTriggeredBy(last, withBlock(block)))
	assertWaits(t, Apply(withRestingOrder(beside), "buy", last.PositionPrice, withBlock(block)), "buy", capitalProtectionTriggered(last))
}

// The slow decline gate reads for going pending only while no flat market episode is open, and
// the flat market opens nothing on the tick the slow decline goes pending: the first trigger owns the sale.
func TestTheSlowDeclineAndTheAreaGatesTakeTheSaleOneAtATime(t *testing.T) {
	onlyTheLinearDecline(t)
	withEarlyTakeProfit(t, false)
	withLinearDecline(t, true)
	withFlatMarket(t, true)
	trade, block := linearScene(linearTrendMet)
	newest := trade.PositionPrice
	line := Finding{Feature: FeatureLinearDecline, Reasons: linearReading(t, trade, block).Reasons}
	area := flatMarketLineBlockOf(basePercentage(trade), aggragates.SideLong, 0)
	block.FlatMarketBarOpens, block.FlatMarketOpenPrices, block.FlatMarketCloses = area.FlatMarketBarOpens, area.FlatMarketOpenPrices, area.FlatMarketCloses
	ai := withBlock(block)
	if !flatMarketReading(trade, block, ai).Holds {
		t.Skip("the flat market reading does not hold beside the line under the constants in force")
	}

	got := Apply(withRestingOrder(trade), "", gainPrint(trade), ai)
	assertWaits(t, got, "", PendingRow(newest, line))
	if st := rebuildState(withRows(trade, got.Rows...)); !st.slowDeclinePending || st.flatMarketActive {
		t.Fatalf("the line goes pending and the flat market opens nothing, got %+v", st)
	}

	open := withRows(trade, FlatMarketTriggeredRow(newest, flatMarketReading(trade, block, ai).Reasons))
	assertUntouched(t, Apply(withRestingOrder(open), "", gainPrint(trade), ai), "")
	if rebuildState(open).slowDeclinePending {
		t.Fatal("an open flat market episode keeps the slow decline from going pending")
	}
}

// Inside the slow decline gate one decline at a time: with the swing decline and the line both
// reading, the ladder goes pending on the swing decline alone, and a ladder pending on the swing
// decline is judged at its new fill on the swing decline alone, the line holding there or not.
func TestTheDeclinesTakeTheSlowDeclineGateOneAtATime(t *testing.T) {
	onlyTheLinearDecline(t)
	withEarlyTakeProfit(t, false)
	withLinearDecline(t, true)
	withSwingDecline(t, true)
	depth := max(linearDepth, swingdecline.ArmDepth)
	if depth >= lastDepthFills {
		t.Skip("swingdecline.ArmDepth reaches the last depth, where a new fill is never judged")
	}
	trade := linearLadder(linearSlope, depth)
	down := max(swingdecline.MinSmcTrendDown, linearTrendMet)
	block := withSmcTrend(linearBlock(linearSlope, depth, linearTail, down), down, max(linearTrendValid, down))
	linearReading(t, trade, block)
	withSwingMemo(t, newSwingDeclineMemo(4, func([]series.Fill, series.Series, series.Trend) (swingdecline.Reading, bool) {
		return swingdecline.Reading{Holds: true, Reasons: linearSwingReasons}, true
	}))

	got := Apply(withRestingOrder(trade), "", gainPrint(trade), withBlock(block))
	assertWaits(t, got, "", PendingRow(trade.PositionPrice, swingFinding(linearSwingReasons)))
	if st := rebuildState(withRows(trade, got.Rows...)); !reflect.DeepEqual(st.pendingFeatures, []string{FeatureSwingDecline}) {
		t.Fatalf("pending on the swing decline alone, got %v", st.pendingFeatures)
	}

	before := linearLadder(linearSlope, depth-1).PositionPrice
	judged := withRow(trade, PendingRow(before, swingFinding(nil)), testutil.At("00:00:00"))
	got = Apply(withRestingOrder(judged), "", gainPrint(trade), withBlock(block))
	for _, row := range got.Rows {
		if row.Gate == GateSlowDecline && slices.Contains(row.Features, FeatureLinearDecline) {
			t.Fatalf("the new fill of a swing pending ladder reads no line, got %+v", got.Rows)
		}
	}
	if st := rebuildState(withRows(judged, got.Rows...)); slices.Contains(st.pendingFeatures, FeatureLinearDecline) {
		t.Fatalf("the line never joins the swing decline, got %v", st.pendingFeatures)
	}
}

// Episodes stored under the legacy low interest and non interest area gates side by side fold into
// the one flat market episode: its trigger stands and the shared action writes one row.
func TestEpisodesStoredSideBySideStillAct(t *testing.T) {
	flatMarketLineAlone(t)
	trade := flatMarketLadder(false)
	ai := bothReadingsMarket(t, trade)
	reasons := flatMarketReading(trade, ai.SmartSell, ai).Reasons
	legacyRange, legacyLine := FlatMarketTriggeredRow(trade.PositionPrice, reasons), FlatMarketTriggeredRow(trade.PositionPrice, reasons)
	legacyRange.Gate, legacyLine.Gate = gateLowInterest, gateNonInterestArea
	both := withRows(trade, legacyRange, legacyLine)
	if !rebuildState(both).flatMarketActive {
		t.Fatal("the legacy episodes fold into the flat market's")
	}
	_, oneStepLower := flatMarketPrices(t, trade)
	loss := handLossPct(trade, oneStepLower)
	assertWaits(t, Apply(both, "buy", oneStepLower, ai), "buy", FlatMarketSaleHeldRow(trade.PositionPrice, -loss))
}

// One sale gate holds an episode at a time. On a market the flat market and the free fall prevention
// both read, the flat market (first in order) opens the one episode and the free fall prevention
// reads nothing on that tick or after; an episode already open, the flat market's or the free fall
// prevention's, keeps the other gate from opening; capital protection opens beside any of them.
func TestFreeFallPreventionAndTheFlatMarketOpenOneEpisodeAtATime(t *testing.T) {
	flatMarketAlone(t)
	withFreeFallPrevention(t, true)
	trade := flatMarketLadder(false)
	fill := trade.PositionPrice
	both := withBlock(inFreeFall(flatMarketBlock(basePercentage(trade), aggragates.SideLong), []bool{true, false, false}, nil))
	if !flatMarketReading(trade, both.SmartSell, both).Holds || !freefallprevention.Trigger(both.SmartSell, aggragates.SideLong).Holds {
		t.Fatal("fixture drifted: both gates must read the market")
	}
	resting := withRestingOrder(trade)

	got := Apply(resting, "buy", fill, both)
	assertWaits(t, got, "buy", triggeredBy(trade, both))
	if st := rebuildState(withRows(trade, got.Rows...)); !st.flatMarketActive || st.freeFallPreventionActive {
		t.Fatalf("the flat market opens the episode and the free fall prevention opens nothing, got %+v", st)
	}

	flatOpen := withRows(trade, triggeredBy(trade, both))
	assertUntouched(t, Apply(withRestingOrder(flatOpen), "buy", fill, freeFallMarket(trade)), "buy")
	if st := rebuildState(flatOpen); st.freeFallPreventionActive {
		t.Fatalf("an open flat market episode keeps the free fall prevention from opening, got %+v", st)
	}

	freeOpen := withRows(trade, freeFallTriggeredBy(trade))
	assertUntouched(t, Apply(withRestingOrder(freeOpen), "buy", fill, both), "buy")
	if st := rebuildState(freeOpen); !st.freeFallPreventionActive || st.flatMarketActive {
		t.Fatalf("an open free fall prevention episode keeps the flat market from opening, got %+v", st)
	}

	withFlatMarket(t, false)
	got = Apply(resting, "buy", fill, both)
	assertWaits(t, got, "buy", freeFallTriggeredBy(trade))
	if st := rebuildState(withRows(trade, got.Rows...)); st.flatMarketActive || !st.freeFallPreventionActive {
		t.Fatalf("with the flat market off the free fall prevention opens, got %+v", st)
	}
}

// The slow decline gate and the free fall prevention take the sale one at a time: on a tick both
// read, the ladder goes pending and the free fall prevention opens nothing, then or while it stays
// pending; an open free fall prevention episode keeps the ladder from going pending.
func TestFreeFallPreventionAndTheSlowDeclineTakeTheSaleOneAtATime(t *testing.T) {
	freeFallPreventionAlone(t)
	withQuietDecline(t, true)
	trade := watchedTrade()
	if !quietDeclineWatched(trade) || !freeFallPreventionWatched(trade) || capitalProtectionWatched(trade) {
		t.Skip("the quiet decline or the free fall prevention does not watch the fixture ladder, or it stands at its last depth")
	}
	fill := slowDeclineLastFill
	both := withBlock(withVerdict(freeFallBlockOf(aggragates.SideLong, true, false, false)))

	pending, got := tickResting(trade, "", gainPrint(trade), testutil.At("18:00:00"), both)
	assertRow(t, got, quietPending(fill))
	if st := rebuildState(pending); !st.slowDeclinePending || st.freeFallPreventionActive {
		t.Fatalf("the ladder goes pending and the free fall prevention opens nothing, got %+v", st)
	}
	_, got = tickResting(pending, "", gainPrint(trade), testutil.At("18:15:00"), withBlock(freeFallBlockOf(aggragates.SideLong, true, false, false)))
	assertUntouched(t, got, "")

	open := withRows(trade, FreeFallPreventionTriggeredRow(fill, freeFallLastBar))
	got = applyAtAGain(open, "", both)
	assertUntouched(t, got, "")
	if st := rebuildState(withRows(open, got.Rows...)); st.slowDeclinePending || !st.freeFallPreventionActive {
		t.Fatalf("an open free fall prevention episode keeps the ladder from going pending, got %+v", st)
	}
}

// Capital protection opens beside an open free fall prevention episode: no pause and no exclusion
// touches it, its triggered row going out while the other episode stands.
func TestCapitalProtectionOpensBesideAFreeFallPreventionEpisode(t *testing.T) {
	freeFallPreventionAlone(t)
	withCapitalProtectionExit(t, true)
	last := capitalProtectionLadder(false)
	block := freeFallMarket(last).SmartSell
	block.CapitalProtectionSmcBearish = true
	beside := withRows(last, FreeFallPreventionTriggeredRow(last.PositionPrice, freeFallLastBar))
	assertWaits(t, Apply(withRestingOrder(beside), "buy", last.PositionPrice, withBlock(block)), "buy", capitalProtectionTriggered(last))
}
