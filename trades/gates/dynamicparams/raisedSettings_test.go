package dynamicparams_test

import (
	"math"
	"reflect"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/dynamicparams"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// openedRaised is the flagged ladder carrying the opened row the shipped
// constants write for both reads bearish.
func openedRaised() aggragates.Trades {
	return withRows(flaggedTrade(), dynamicparams.OpenedMessage(dynamicparams.BearPercentagePoints, dynamicparams.BearDepths))
}

// raisedByTheConstants is the three-row ladder raised by both amounts the
// constants name, each row once.
func raisedByTheConstants() []aggragates.StrategySettings {
	rows := threeRowLadder()
	for index := range rows {
		rows[index].Percentage += dynamicparams.BearPercentagePoints
		rows[index].Depths += float64(dynamicparams.BearDepths)
	}
	return rows
}

// A trade without an opened row, a trade the flag does not shape whatever
// row it carries, and a row whose amounts raise nothing hand back the stored
// rows themselves and false: the engine then changes nothing.
func TestRaisedSettingsLeavesTheStoredRowsWhenNothingIsRaised(t *testing.T) {
	cases := []struct {
		name   string
		change func(*aggragates.Trades)
	}{
		{"no opened row", func(trade *aggragates.Trades) { trade.Logs = nil }},
		{"only the per-tick rows an earlier release wrote", func(trade *aggragates.Trades) {
			*trade = withRows(*trade, "dynamic params: 1D Super Guppy bearish, BMSB bearish: both bearish, percentage +0.5 and depths +1 on every row")
		}},
		{"an opened row raising nothing", func(trade *aggragates.Trades) {
			*trade = withRows(*trade, "dynamic params: opened raised, on every row")
		}},
		{"the flag off", func(trade *aggragates.Trades) { trade.Strategy.Params.DynamicParams = false }},
		{"an inverse ladder", func(trade *aggragates.Trades) { trade.Inverse = true }},
		{"a futures strategy", func(trade *aggragates.Trades) { trade.Strategy.TradeType = aggragates.Futures }},
		{"an impasse child", func(trade *aggragates.Trades) { trade.ParentID = 7 }},
	}

	for _, c := range cases {
		trade := openedRaised()
		c.change(&trade)
		stored := trade.StrategyPair.StrategySettings
		before := cloneRows(stored)

		got, raised := dynamicparams.RaisedSettings(trade)
		if raised {
			t.Errorf("%s: RaisedSettings reported a raise", c.name)
		}
		if len(got) != len(stored) || &got[0] != &stored[0] {
			t.Errorf("%s: RaisedSettings must hand back the stored slice itself", c.name)
		}
		if !reflect.DeepEqual(stored, before) {
			t.Errorf("%s: the stored rows moved", c.name)
		}
	}

	noRows := openedRaised()
	noRows.StrategyPair.StrategySettings = nil
	if got, raised := dynamicparams.RaisedSettings(noRows); raised || got != nil {
		t.Errorf("a trade without rows: RaisedSettings = %v, %v, want nil and false", got, raised)
	}
}

// An opened row raises a copy of every row by the amounts it carries, only
// the parts it names, and the stored rows stay exactly as they were.
func TestRaisedSettingsRaisesACopyOfEveryRowByTheRowsAmounts(t *testing.T) {
	for name, amounts := range raiseAmounts {
		trade := withRows(flaggedTrade(), dynamicparams.OpenedMessage(amounts.points, amounts.depths))
		stored := trade.StrategyPair.StrategySettings
		before := cloneRows(stored)

		got, raised := dynamicparams.RaisedSettings(trade)
		if !raised {
			t.Fatalf("%s: RaisedSettings must report the raise", name)
		}
		if &got[0] == &stored[0] {
			t.Fatalf("%s: the raised rows must be a copy", name)
		}
		if !reflect.DeepEqual(stored, before) {
			t.Fatalf("%s: the stored rows moved: %+v, want %+v", name, stored, before)
		}

		for index := range before {
			wantPercentage := before[index].Percentage + amounts.points
			wantDepths := before[index].Depths + float64(amounts.depths)
			if got[index].Percentage != wantPercentage || got[index].Depths != wantDepths {
				t.Errorf("%s row %d: %v%% over %v depths, want %v%% over %v", name, index,
					got[index].Percentage, got[index].Depths, wantPercentage, wantDepths)
			}
		}
	}
}

// Every tick raises the configured rows afresh: two ticks off the one stored
// trade read the configured rows plus the row's amounts both times — never
// twice the amounts — because the raise is never written back for a later
// tick to raise again.
func TestRaisedSettingsNeverCompounds(t *testing.T) {
	trade := openedRaised()
	want := raisedByTheConstants()

	first, _ := dynamicparams.RaisedSettings(trade)
	second, _ := dynamicparams.RaisedSettings(trade)

	for tick, rows := range [][]aggragates.StrategySettings{first, second} {
		if !reflect.DeepEqual(rows, want) {
			t.Errorf("tick %d: %+v, want the configured rows raised once %+v", tick+1, rows, want)
		}
	}
}

// The first entry of a ladder that opens raised is sized for the extra
// depth: ladder.CalculateInitialBid on the sizing copy the chain's
// EntrySettings make sizes the raised row — its depths plus BearDepths at its
// percentage plus BearPercentagePoints — a smaller first entry than the
// configured row sizes, while the trade keeps its own rows.
func TestRaisedSettingsSizeTheFirstEntryForTheExtraDepth(t *testing.T) {
	const budget = 10000.0

	trade := openedRaised()
	stored := trade.StrategyPair.StrategySettings
	configured := cloneRows(stored)

	rows, raised := dynamicparams.RaisedSettings(trade)
	if !raised {
		t.Fatal("the opened row must raise the rows")
	}

	sizing := aggragates.Params{EntrySettings: rows}.SizingTrade(trade)
	bid, err := ladder.CalculateInitialBid(budget, sizing, 0)
	if err != nil {
		t.Fatalf("the raised rows must size a first entry from this budget: %v", err)
	}

	haircut := budget * (1 - ladder.InitialBidReservePercent/100)
	want := ladder.GetInitialBidByDepth(
		haircut,
		configured[0].Depths+float64(dynamicparams.BearDepths),
		configured[0].Multiplier,
		configured[0].Percentage+dynamicparams.BearPercentagePoints,
	)
	if math.Abs(bid-want) > 1e-9 {
		t.Fatalf("first entry on the raised rows = %v, want %v", bid, want)
	}

	configuredBid, err := ladder.CalculateInitialBid(budget, trade, 0)
	if err != nil {
		t.Fatalf("the configured rows must size a first entry from this budget: %v", err)
	}
	if bid >= configuredBid {
		t.Fatalf("a ladder sized for an extra depth must open smaller: %v, configured %v", bid, configuredBid)
	}

	if &trade.StrategyPair.StrategySettings[0] != &stored[0] || !reflect.DeepEqual(stored, configured) {
		t.Fatal("the trade must keep its own rows")
	}
}
