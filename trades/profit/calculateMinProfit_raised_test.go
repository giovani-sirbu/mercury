// The minimum profit of a ladder that opened raised. The raise reaches
// CalculateMinProfit only through the trade's own opened event, so these tests
// build that event with the writer the engine uses. They share the entryFill,
// entryFills and ladderTrade fixtures of calculateMinProfit_blackbox_test.go.

package profit_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/dynamicparams"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/profit"
)

// The amounts the opened events below carry. They are this suite's own, apart
// from the shipped constants, so a retune of dynamicparams.BearPercentagePoints
// or dynamicparams.BearDepths moves no expectation here: the ladder trades
// whatever its own event says.
const (
	openedPoints = 0.5
	openedDepths = 2
)

// withOpenedEvent is the trade as the engine leaves it once its ladder opened
// raised: the DynamicParams flag on for a long spot parent, and the opened pair
// appended with the writer the engine uses (dynamicparams.Opened.Rows), the
// event every reader takes the amounts from. The rows the trade stores are
// untouched.
func withOpenedEvent(trade aggragates.Trades, points float64, depths int) aggragates.Trades {
	trade.Strategy.Params.DynamicParams = true
	trade.Strategy.TradeType = aggragates.Spot
	row, event := dynamicparams.Opened{Points: points, Depths: depths}.Rows(trade, 0, time.Time{})

	return aggragates.AppendStrategyRow(trade, row, event)
}

// A ladder that opened raised trades the raised rows, and so does its floor: the
// held depth's row raised by the amounts the opened event carries, each part
// only when the event names it. The trade the chain hands over carries the
// STORED rows, so the floor reads the raise off the event itself.
func TestCalculateMinProfitReadsTheRowsTheLadderTrades(t *testing.T) {
	plain := ladderTrade(
		[]aggragates.StrategySettings{{Percentage: 2.2, Depths: 6}},
		entryFill("BUY", 1, 10, 1000),
	)

	cases := []struct {
		name   string
		points float64
		depths int
		want   float64
	}{
		{"percentage and depths", openedPoints, openedDepths, 10000 * (2.2 + openedPoints) / (6 + openedDepths) / 100},
		{"percentage only", openedPoints, 0, 10000 * (2.2 + openedPoints) / 6 / 100},
		{"depths only", 0, openedDepths, 10000 * 2.2 / (6 + openedDepths) / 100},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := profit.CalculateMinProfit(withOpenedEvent(plain, c.points, c.depths))
			testutil.AssertFloatEqual(t, got, c.want, 1e-10, "raised ladder floor")
		})
	}

	t.Run("the held depth's raised row", func(t *testing.T) {
		second := ladderTrade(
			[]aggragates.StrategySettings{{Percentage: 2, Depths: 8}, {Percentage: 3, Depths: 8}},
			entryFills("BUY", 2)...,
		)

		got := profit.CalculateMinProfit(withOpenedEvent(second, openedPoints, openedDepths))
		testutil.AssertFloatEqual(t, got, 2000*(3+openedPoints)/(8+openedDepths)/100, 1e-10, "raised held row floor")
	})
}

// The raise is the ladder's own opened event and nothing else: without the
// event, or with the flag off, the ladder trades the rows it stores.
func TestCalculateMinProfitKeepsTheStoredRowsWithoutARaise(t *testing.T) {
	plain := ladderTrade(
		[]aggragates.StrategySettings{{Percentage: 2.2, Depths: 6}},
		entryFill("BUY", 1, 10, 1000),
	)
	want := 10000 * 2.2 / 6 / 100

	flagWithoutEvent := plain
	flagWithoutEvent.Strategy.Params.DynamicParams = true
	flagWithoutEvent.Strategy.TradeType = aggragates.Spot

	eventWithoutFlag := withOpenedEvent(plain, openedPoints, openedDepths)
	eventWithoutFlag.Strategy.Params.DynamicParams = false

	testutil.AssertFloatEqual(t, profit.CalculateMinProfit(flagWithoutEvent), want, 1e-10, "flag on, no opened event")
	testutil.AssertFloatEqual(t, profit.CalculateMinProfit(eventWithoutFlag), want, 1e-10, "opened event, flag off")
}

// The raise is read afresh on every call and written nowhere: the trade keeps
// the rows it stores, and asking again answers the same floor instead of the
// raise raised twice.
func TestCalculateMinProfitNeverRaisesTheStoredRows(t *testing.T) {
	stored := []aggragates.StrategySettings{{Percentage: 2.2, Depths: 6}}
	snapshot := append([]aggragates.StrategySettings(nil), stored...)
	raised := withOpenedEvent(ladderTrade(stored, entryFill("BUY", 1, 10, 1000)), openedPoints, openedDepths)

	first := profit.CalculateMinProfit(raised)
	second := profit.CalculateMinProfit(raised)

	if first != second {
		t.Errorf("the floor moved from %v to %v between two reads of the same trade", first, second)
	}
	if !reflect.DeepEqual(stored, snapshot) {
		t.Errorf("the stored rows were written: %+v, want %+v", stored, snapshot)
	}
}
