package actions

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/strategies/gates/dynamicparams"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// minProfitCase is one close put to the gate: the price it closes at and
// whether the gate refuses it. A refusal must name the floor the gate read, or
// a close refused for any other reason would pass for the one under test.
type minProfitCase struct {
	name    string
	price   float64
	floor   float64
	refused bool
}

// assertGate builds the trade each case closes at its price and holds HasProfit
// to the case. The fixtures carry no fee, so the floor alone decides.
func assertGate(t *testing.T, build func(price float64) aggragates.Trades, cases []minProfitCase) {
	t.Helper()

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := HasProfit(events.Events{Trade: build(c.price)})

			if (err != nil) != c.refused {
				t.Fatalf("closing at %v: error = %v, want a refusal: %t", c.price, err, c.refused)
			}
			if want := fmt.Sprintf("min profit(%f)", c.floor); c.refused && !strings.Contains(err.Error(), want) {
				t.Errorf("closing at %v: refusal %q does not name the floor, want %q", c.price, err, want)
			}
		})
	}
}

// inverseHasProfitTrade is hasProfitTrade turned around: an inverse ladder of 6
// depths at 2.2 whose one entry SOLD 10 base at 1000, closing at price by buying
// the base back with the quote it holds.
func inverseHasProfitTrade(price float64) aggragates.Trades {
	trade := hasProfitTrade(price)
	trade.Inverse = true
	trade.History = []aggragates.TradesHistory{{Type: "SELL", Quantity: 10, Price: 1000, OrderId: 1}}

	return trade
}

// An inverse ladder counts its profit in base, so its floor is the share of the
// base its entries sold, never of the quote they fetched: the user's example
// turned around must buy back at least 10 * 2.2 / 6 / 100 = 0.0366... base more
// than it sold. The close buys back 10000 / price and its lot-size remainder is
// credited as dust, so the net profit is 10000 / price - 10, and the highest
// price on the two-decimal grid that still books the floor is 996.34.
func TestHasProfitHoldsAnInverseCloseToTheBaseShareFloor(t *testing.T) {
	const floor = 10 * 2.2 / 6 / 100

	assertGate(t, inverseHasProfitTrade, []minProfitCase{
		{"the first price under the floor is refused", 996.35, floor, true},
		{"the first price over it is taken", 996.34, floor, false},
		{"a price well over it is taken", 950, floor, false},
		{"the entry price books nothing and is refused", 1000, floor, true},
	})

	got, _ := HasProfit(events.Events{Trade: inverseHasProfitTrade(996.34)})
	if want := 10000/996.34 - 10; math.Abs(got.Trade.Profit-want) > 1e-9 {
		t.Errorf("net profit = %v, want %v: an inverse close books its profit in base", got.Trade.Profit, want)
	}
}

// raisedHasProfitTrade is hasProfitTrade as the engine leaves it when its ladder
// opened raised: the flag on for a long spot parent and the opened pair of the
// given amounts appended with the engine's own writer. The rows the trade stores
// stay 2.2 over 6. The amounts are this file's own, never the shipped constants.
func raisedHasProfitTrade(points float64, depths int) func(price float64) aggragates.Trades {
	return func(price float64) aggragates.Trades {
		trade := hasProfitTrade(price)
		trade.Strategy.Params.DynamicParams = true
		trade.Strategy.TradeType = aggragates.Spot
		row, event := dynamicparams.Opened{Points: points, Depths: depths}.Rows(trade, 0, time.Time{})

		return aggragates.AppendStrategyRow(trade, row, event)
	}
}

// A ladder that opened raised trades the raised rows, so the gate holds its
// close to the raised floor and never to the one its stored rows would ask: a
// raise that lifts the floor refuses a price the stored rows take, and one that
// lowers it takes a price the stored rows refuse. The stored floor is 10000 *
// 2.2 / 6 / 100 = 36.666..., and a close of 10 at p books 10 * (p - 1000).
func TestHasProfitReadsTheRaisedFloorOfALadderThatOpenedRaised(t *testing.T) {
	const lifted = 10000 * (2.2 + 1) / 6 / 100 // one point more percentage: 3.2 over 6
	assertGate(t, raisedHasProfitTrade(1, 0), []minProfitCase{
		{"a lifted floor: the first price over it is taken", 1005.34, lifted, false},
		{"a lifted floor: the first price under it is refused", 1005.33, lifted, true},
		{"a lifted floor refuses what the stored rows take", 1004, lifted, true},
	})

	const lowered = 10000 * 2.2 / (6 + 2) / 100 // two more depths: 2.2 over 8
	assertGate(t, raisedHasProfitTrade(0, 2), []minProfitCase{
		{"a lowered floor: the first price over it is taken", 1002.76, lowered, false},
		{"a lowered floor: the first price under it is refused", 1002.74, lowered, true},
		{"a lowered floor takes what the stored rows refuse", 1003.5, lowered, false},
	})

	// The same price on the same ladder without its opened event is the stored floor's.
	assertGate(t, hasProfitTrade, []minProfitCase{{"no event: the stored floor refuses it", 1003.5, 10000 * 2.2 / 6 / 100, true}})
}

// A raise belongs to a long spot parent alone: an inverse ladder with the flag
// on and an opened event on its trade still trades its stored rows, so its floor
// is the stored 10 * 2.2 / 6 / 100 base, not the raised one a long ladder reads.
func TestHasProfitNeverRaisesTheFloorOfAnInverseLadder(t *testing.T) {
	const floor = 10 * 2.2 / 6 / 100
	flagged := func(price float64) aggragates.Trades {
		trade := raisedHasProfitTrade(1, 0)(price)
		trade.Inverse = true
		trade.History = inverseHasProfitTrade(price).History

		return trade
	}

	assertGate(t, flagged, []minProfitCase{
		{"the stored floor's first price over it is taken", 996.34, floor, false},
		{"the stored floor's first price under it is refused", 996.35, floor, true},
	})
}

// multiRowHasProfitTrade is a ladder with a row per depth, the shape of the
// "Multi Depths" strategy: a percentage that rises as the ladder goes deeper and
// 8 depths on every row but the last, which carries 6 so the depths the floor
// divides by can only be the held row's own. The given number of entries is
// filled, each its own order of one base at 1000.
func multiRowHasProfitTrade(filled int) func(price float64) aggragates.Trades {
	return func(price float64) aggragates.Trades {
		trade := hasProfitTrade(price)
		trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{
			{Percentage: 2.2, Depths: 8}, {Percentage: 2.6, Depths: 8}, {Percentage: 3.4, Depths: 6},
		}
		trade.History = nil
		for order := 1; order <= filled; order++ {
			trade.History = append(trade.History, aggragates.TradesHistory{Type: "BUY", Quantity: 1, Price: 1000, OrderId: int64(order)})
		}

		return trade
	}
}

// The gate reads the row of the depth the ladder holds, whatever row the next
// fill would use: the floor of N entries of 1000 on that row's percentage over
// that row's depths, held to the price tick. A ladder past its last row falls
// back to the first row, never to the last.
func TestHasProfitReadsTheRowOfTheDepthTheLadderHolds(t *testing.T) {
	depths := []struct {
		filled      int
		floor       float64
		over, under float64
	}{
		{1, 1000 * 2.2 / 8 / 100, 1002.76, 1002.74},
		{2, 2000 * 2.6 / 8 / 100, 1003.26, 1003.24},
		{3, 3000 * 3.4 / 6 / 100, 1005.67, 1005.66},
		{4, 4000 * 2.2 / 8 / 100, 1002.76, 1002.74},
	}

	for _, d := range depths {
		t.Run(fmt.Sprintf("held at depth %d", d.filled), func(t *testing.T) {
			assertGate(t, multiRowHasProfitTrade(d.filled), []minProfitCase{
				{"one tick over its floor is taken", d.over, d.floor, false},
				{"one tick under it is refused", d.under, d.floor, true},
			})
		})
	}
}
