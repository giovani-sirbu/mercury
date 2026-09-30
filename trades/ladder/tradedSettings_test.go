package ladder

import (
	"reflect"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/dynamicparams"
)

// Every amount form the opened event carries raises a COPY of every stored row
// by exactly the amounts it carries — the rows the ladder trades — and the
// stored rows stay as they were. The raise is withOpenedEvent's: the opened
// pair the engine's own writer appends, with the amounts of this suite.
func TestTradedSettingsRaisesEveryRowByTheOpenedEventsAmounts(t *testing.T) {
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
		trade := withOpenedEvent(rowsLadder(1, 16, 4, rowsGrid()...), c.points, c.depths)
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

// A trade without an opened event, one the flag does not shape and one whose
// ladder the flag leaves alone trade the stored rows — the very slice, so the
// readings on top of it are byte-identical to what they were before the flag
// existed. The shaped cases still carry their opened event: the event is not
// enough where the flag does not apply.
func TestTradedSettingsKeepsTheStoredRowsWhenNothingRaisesThem(t *testing.T) {
	cases := []struct {
		name   string
		change func(*aggragates.Trades)
	}{
		{"no opened event", func(trade *aggragates.Trades) {
			trade.Logs = nil
			trade.StrategyEvents = nil
		}},
		{"the flag off", func(trade *aggragates.Trades) {
			trade.Strategy.Params.DynamicParams = false
		}},
		{"an inverse ladder", func(trade *aggragates.Trades) {
			trade.Inverse = true
		}},
		{"a futures strategy", func(trade *aggragates.Trades) {
			trade.Strategy.TradeType = aggragates.Futures
		}},
		{"an impasse child", func(trade *aggragates.Trades) {
			trade.ParentID = 7
		}},
	}

	for _, c := range cases {
		trade := withOpenedEvent(rowsLadder(1, 16, 4, rowsGrid()...), raisePoints, raiseDepths)
		c.change(&trade)
		stored := trade.StrategyPair.StrategySettings

		got := tradedSettings(trade)

		if len(got) != len(stored) || &got[0] != &stored[0] {
			t.Errorf("%s: tradedSettings must hand back the stored slice itself", c.name)
		}
	}
}

// The opened event is the raise and the opened row beside it is only the text
// an operator reads: a trade whose logs carry the row but whose strategy events
// carry no opened event trades its stored rows, the very slice, while the same
// trade with its event beside the row trades the raised copy.
func TestTradedSettingsReadsTheOpenedEventNeverTheRow(t *testing.T) {
	paired := withOpenedEvent(rowsLadder(1, 16, 4, rowsGrid()...), raisePoints, raiseDepths)
	textOnly := paired
	textOnly.StrategyEvents = nil
	stored := textOnly.StrategyPair.StrategySettings

	if len(textOnly.Logs) != 1 {
		t.Fatalf("fixture drifted: the trade carries %d rows, want the opened row", len(textOnly.Logs))
	}
	if got := tradedSettings(textOnly); len(got) != len(stored) || &got[0] != &stored[0] {
		t.Errorf("tradedSettings = %+v, want the stored slice itself: a row without its event opens nothing", got)
	}
	if got := tradedSettings(paired); &got[0] == &stored[0] || got[0].Depths != stored[0].Depths+raiseDepths {
		t.Errorf("tradedSettings = %+v, want the raised copy when the event stands beside the row", got)
	}
}

// A trade whose pair carries no rows has nothing to raise: the answer is
// empty, and every reading on top of it reads that as an unknown ceiling.
func TestTradedSettingsOfATradeWithoutRowsIsEmpty(t *testing.T) {
	trade := withOpenedEvent(depthTrade(2), raisePoints, raiseDepths)

	if got := tradedSettings(trade); len(got) != 0 {
		t.Errorf("tradedSettings = %+v, want no rows", got)
	}
}
