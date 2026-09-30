package ladder

import (
	"reflect"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/dynamicparams"
)

// The amounts the opened rows below carry. They are this suite's own, apart
// from the shipped constants, so a retune of BearPercentagePoints or
// BearDepths moves no expectation here: the ladder trades whatever its own
// row says.
const (
	raisePoints = 0.5
	raiseDepths = 2
)

// withOpenedRow is the trade as the engine leaves it once its ladder opened
// raised: the DynamicParams flag on for a long spot parent, and the one
// opened row in its logs carrying these amounts. The rows the trade stores
// are untouched.
func withOpenedRow(trade aggragates.Trades, points float64, depths int) aggragates.Trades {
	trade.Strategy.Params.DynamicParams = true
	trade.Strategy.TradeType = aggragates.Spot
	trade.Logs = append(append([]aggragates.TradesLogs(nil), trade.Logs...), aggragates.TradesLogs{
		Message: dynamicparams.OpenedMessage(points, depths),
		Type:    aggragates.LOG_INFO,
	})

	return trade
}

// Every amount form the opened row writes raises a COPY of every stored row by
// exactly the amounts it carries — the rows the ladder trades — and the
// stored rows stay as they were.
func TestTradedSettingsRaisesEveryRowByTheOpenedRowsAmounts(t *testing.T) {
	cases := []struct {
		name   string
		points float64
		depths int
	}{
		{"percentage and depths", raisePoints, raiseDepths},
		{"depths only", 0, raiseDepths},
		{"percentage only", raisePoints, 0},
	}

	for _, c := range cases {
		trade := withOpenedRow(rowsLadder(1, 16, 4, rowsGrid()...), c.points, c.depths)
		stored := trade.StrategyPair.StrategySettings
		before := append([]aggragates.StrategySettings(nil), stored...)

		got := tradedSettings(trade)

		if want := dynamicparams.RaiseBy(before, c.points, c.depths); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: tradedSettings = %+v, want the stored rows raised %+v", c.name, got, want)
		}
		for index := range before {
			if got[index].Percentage != before[index].Percentage+c.points {
				t.Errorf("%s row %d: percentage %v, want the stored %v plus %v", c.name, index, got[index].Percentage, before[index].Percentage, c.points)
			}
			if got[index].Depths != before[index].Depths+float64(c.depths) {
				t.Errorf("%s row %d: depths %v, want the stored %v plus %d", c.name, index, got[index].Depths, before[index].Depths, c.depths)
			}
		}
		if &got[0] == &stored[0] {
			t.Errorf("%s: the traded rows must be a copy of the stored ones", c.name)
		}
		if !reflect.DeepEqual(stored, before) {
			t.Errorf("%s: the stored rows moved: %+v, want %+v", c.name, stored, before)
		}
	}
}

// A trade without an opened row, one the flag does not shape and one whose
// ladder the flag leaves alone trade the stored rows — the very slice, so the
// readings on top of it are byte-identical to what they were before the flag
// existed.
func TestTradedSettingsKeepsTheStoredRowsWhenNothingRaisesThem(t *testing.T) {
	cases := []struct {
		name   string
		change func(*aggragates.Trades)
	}{
		{"no opened row", func(trade *aggragates.Trades) { trade.Logs = nil }},
		{"the flag off", func(trade *aggragates.Trades) { trade.Strategy.Params.DynamicParams = false }},
		{"an inverse ladder", func(trade *aggragates.Trades) { trade.Inverse = true }},
		{"a futures strategy", func(trade *aggragates.Trades) { trade.Strategy.TradeType = aggragates.Futures }},
		{"an impasse child", func(trade *aggragates.Trades) { trade.ParentID = 7 }},
	}

	for _, c := range cases {
		trade := withOpenedRow(rowsLadder(1, 16, 4, rowsGrid()...), raisePoints, raiseDepths)
		c.change(&trade)
		stored := trade.StrategyPair.StrategySettings

		got := tradedSettings(trade)

		if len(got) != len(stored) || &got[0] != &stored[0] {
			t.Errorf("%s: tradedSettings must hand back the stored slice itself", c.name)
		}
	}
}

// A trade whose pair carries no rows has nothing to raise: the answer is
// empty, and every reading on top of it reads that as an unknown ceiling.
func TestTradedSettingsOfATradeWithoutRowsIsEmpty(t *testing.T) {
	trade := withOpenedRow(depthTrade(2), raisePoints, raiseDepths)

	if got := tradedSettings(trade); len(got) != 0 {
		t.Errorf("tradedSettings = %+v, want no rows", got)
	}
}
