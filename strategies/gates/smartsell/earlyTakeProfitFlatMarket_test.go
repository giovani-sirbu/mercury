package smartsell

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// latchedFlatMarketRows counts the rows of the trade that latched on the bars.
func latchedFlatMarketRows(trade aggragates.Trades) int {
	return carriedRows(trade, "early take profit latched (flat market:")
}

// The bars alone latch a watched ladder on one row of their own, at every depth and whatever the trend reads.
func TestTheFlatMarketBarsLatchAWatchedLadderOnTheirOwnRow(t *testing.T) {
	onlyTheFlatMarketLatch(t)
	proposals := slices.Concat([]string{"", "buy", "takeProfit", "sell"}, armingProposals)
	for depth := latchDepth; depth < latchDepth+3 && depth < lastDepthFills; depth++ {
		trade := latchLadder(depth)
		for _, trend := range latchTrends {
			ai := latchMarket(t, trade, trend.down, trend.valid)
			want := latchRowOf(t, trade, ai.SmartSell)
			message := "smartSell: early take profit latched (flat market: " + strings.Join(want.Reasons, ", ") + ")"
			if want.Message != message || want.Gate != GateSlowDecline || want.Event != EventLatched || want.Price != trade.PositionPrice {
				t.Fatalf("depth %d, %s: the gate's latched kind naming the bars at the newest fill, got %+v", depth, trend.name, want)
			}
			if !slices.Equal(want.Features, []string{FeatureEarlyTakeProfit}) || len(want.Reasons) != 5 {
				t.Fatalf("depth %d, %s: the indecision alone and five reasons, got %+v", depth, trend.name, want)
			}
			for slot, opening := range []string{"range on 1h bars ", "quiet bodies ", "fall ", "closes within ", "closes span "} {
				if !strings.HasPrefix(want.Reasons[slot], opening) {
					t.Errorf("depth %d, %s: reason %d is %q, want it opening %q", depth, trend.name, slot, want.Reasons[slot], opening)
				}
			}
			reanchored := trade
			reanchored.PositionPrice += 5
			assertRow(t, Apply(reanchored, "", trade.PositionPrice, ai), want)
			for _, price := range []float64{trade.PositionPrice, trade.PositionPrice / 2} {
				for _, position := range proposals {
					got := Apply(trade, position, price, ai)
					assertRow(t, got, want)
					assertNoSale(t, got, position)
				}
			}
		}
	}
}

// A resting close and an impasse strategy's parent are read like any other ladder.
func TestTheFlatMarketBarsLatchARestingAndAnImpasseLadderToo(t *testing.T) {
	onlyTheFlatMarketLatch(t)
	trade := latchLadder(latchDepth)
	ai := latchMarket(t, trade, 0, 0)
	want := latchRowOf(t, trade, ai.SmartSell)
	resting := trade
	resting.PositionType = "takeProfit"
	assertRow(t, Apply(resting, "", trade.PositionPrice, ai), want)
	impasse := trade
	impasse.Strategy.Params.Impasse = true
	assertRow(t, Apply(impasse, "", trade.PositionPrice, ai), want)
}

// The row goes out once; the fold reads the ladder latched and the take profit reads its position price.
func TestTheFlatMarketLatchGoesOutOnceAndTheFoldAndTheTakeProfitReadIt(t *testing.T) {
	onlyTheFlatMarketLatch(t)
	trade := latchLadder(max(latchDepth, 2))
	ai := latchMarket(t, trade, 0, 0)
	price, fromAverage, fromPosition := overBreakEven(trade)
	if !(fromAverage < fromPosition) {
		t.Fatal("fixture drifted: the position price's move must exceed the average entry price's")
	}
	if got := TakeProfitPercentage(trade, price, fromAverage); got != fromAverage {
		t.Fatalf("control: an unlatched ladder's take profit reads the input %v, got %v", fromAverage, got)
	}

	latched, got := engineTick(trade, "", price, testutil.At("18:00:00"), ai)
	assertRow(t, got, latchRowOf(t, trade, ai.SmartSell))
	if st := rebuildState(latched); !st.earlyTakeProfit || !st.earlyTakeProfitWatched || st.slowDeclinePending {
		t.Fatalf("the tick after: latched, watched and not pending, got %+v", st)
	}
	var data EventData
	events := latched.StrategyEventsOf(aggragates.StrategyParamSmartSell, GateSlowDecline)
	if len(events) != 1 || events[0].DecodeData(&data) != nil || data.Event != EventLatched || !slices.Equal(data.Features, []string{FeatureEarlyTakeProfit}) {
		t.Fatalf("one latched event carrying the indecision alone, got %+v", events)
	}
	if want := latchRowOf(t, trade, ai.SmartSell); data.Price != want.Price || !slices.Equal(data.Reasons, want.Reasons) {
		t.Fatalf("the event carries the newest fill's price and the bars' reasons, got %+v", data)
	}
	if got := TakeProfitPercentage(latched, price, fromAverage); got != fromPosition {
		t.Fatalf("a latched ladder's take profit reads its position price %v, got %v", fromPosition, got)
	}

	later := []aggragates.AIIndicators{ai, latchMarket(t, trade, 3, 3), withBlock(aggragates.SmartSellIndicators{})}
	for index, reading := range later {
		latched, got = engineTick(latched, "", price, testutil.At("18:15:00").Add(time.Duration(index)*time.Minute), reading)
		assertUntouched(t, got, "")
	}
	withEarlyTakeProfitFlatMarket(t, false)
	if st := rebuildState(latched); !st.earlyTakeProfit || TakeProfitPercentage(latched, price, fromAverage) != fromPosition {
		t.Fatalf("nothing takes the latch away, the switch off included, got %+v", st)
	}
	if rows, latches := latchedFlatMarketRows(latched), carriedLatches(latched); rows != 1 || latches != 1 || carriedEvents(latched, GateSlowDecline, EventLatched) != 1 {
		t.Fatalf("one row and one latched event over every tick, got %d and %d in %+v", rows, latches, latched.Logs)
	}
	assertUntouched(t, Apply(latchedBy(trade), "", price, ai), "")
}

// A tick a depth priority holds reads nothing, the sophos flag aside; the ladder's next fill ends the hold and the bars latch it.
func TestTheFlatMarketLatchReadsNoTickADepthPriorityHolds(t *testing.T) {
	onlyTheFlatMarketLatch(t)
	withDepthPriorityPause(t, true)
	trade := latchLadder(latchDepth)
	ai := latchMarket(t, trade, 0, 0)
	newest := trade.PositionPrice
	held := heldBy(trade, trade.History[len(trade.History)-1].CreatedAt.Add(time.Minute))
	if !rebuildState(held).depthPriorityHeld {
		t.Fatal("fixture drifted: the ladder must read held")
	}
	for _, position := range []string{"", "trailingBuy", "takeProfit"} {
		assertUntouched(t, Apply(held, position, newest, ai), position)
	}
	flag := ai
	flag.SmartSell.Indecision, flag.SmartSell.QuietDeclineBreakReasons = true, indecisionReasons
	assertRow(t, Apply(held, "", newest, flag), LatchedRow(newest, indecisionFinding(indecisionReasons)))

	withDepthPriorityPause(t, false)
	assertRow(t, Apply(held, "", newest, ai), latchRowOf(t, held, ai.SmartSell))
	withDepthPriorityPause(t, true)

	next := fills(latchDepth+1, "19:00:00")[latchDepth]
	resumed := withFill(held, next.Price, next.At)
	if rebuildState(resumed).depthPriorityHeld {
		t.Fatal("fixture drifted: the next fill ends the hold")
	}
	assertRow(t, Apply(resumed, "", next.Price, ai), latchRowOf(t, resumed, ai.SmartSell))
}

// A tick another reading wrote the row on leaves the latch to the next tick: the quiet decline goes pending first.
func TestAnotherRowOnTheTickLeavesTheFlatMarketLatchToTheNextTick(t *testing.T) {
	onlyTheFlatMarketLatch(t)
	withQuietDecline(t, true)
	trade := watchedTrade()
	if !quietDeclineWatched(trade) || !earlyTakeProfitWatched(trade) || capitalProtectionWatched(trade) {
		t.Skip("the quiet decline or the early take profit does not watch the fixture ladder, or it stands at its last depth")
	}
	ai := withBlock(withVerdict(latchMarket(t, trade, 0, 0).SmartSell))
	gain := gainPrint(trade)

	pending, got := tickResting(trade, "", gain, testutil.At("18:00:00"), ai)
	assertRow(t, got, quietPending(slowDeclineLastFill))
	if st := rebuildState(pending); !st.slowDeclinePending || st.earlyTakeProfit {
		t.Fatalf("the tick that went pending latches nothing, got %+v", st)
	}
	latched, got := tickResting(pending, "", gain, testutil.At("18:15:00"), ai)
	assertRow(t, got, latchRowOf(t, pending, ai.SmartSell))
	assertNoSale(t, got, "")
	if st := rebuildState(latched); !st.earlyTakeProfit || !st.slowDeclinePending {
		t.Fatalf("the next tick latches the ladder, which stays pending, got %+v", st)
	}
	assertUntouched(t, Apply(withRestingOrder(latched), "", gain, ai), "")
}
