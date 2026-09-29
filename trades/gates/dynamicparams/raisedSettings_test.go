package dynamicparams_test

import (
	"math"
	"reflect"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/dynamicparams"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// A trade the flag does not apply to, and reads that raise nothing, hand back
// the stored rows themselves and false: the engine then changes nothing.
func TestRaisedSettingsLeavesTheStoredRowsWhenNothingIsRaised(t *testing.T) {
	cases := []struct {
		name   string
		change func(*aggragates.Trades)
		reads  aggragates.DynamicParamsIndicators
	}{
		{"the flag off", func(trade *aggragates.Trades) { trade.Strategy.Params.DynamicParams = false }, bothBearish},
		{"an inverse ladder", func(trade *aggragates.Trades) { trade.Inverse = true }, bothBearish},
		{"a futures strategy", func(trade *aggragates.Trades) { trade.Strategy.TradeType = aggragates.Futures }, bothBearish},
		{"an impasse child", func(trade *aggragates.Trades) { trade.ParentID = 7 }, bothBearish},
		{"reads not bearish", func(*aggragates.Trades) {}, notBearish},
		{"reads not read", func(*aggragates.Trades) {}, notRead},
		{"the zero read", func(*aggragates.Trades) {}, aggragates.DynamicParamsIndicators{}},
	}

	for _, c := range cases {
		trade := flaggedTrade()
		c.change(&trade)
		stored := trade.StrategyPair.StrategySettings
		before := cloneRows(stored)

		got, raised := dynamicparams.RaisedSettings(trade, c.reads)
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

	noRows := flaggedTrade()
	noRows.StrategyPair.StrategySettings = nil
	if got, raised := dynamicparams.RaisedSettings(noRows, bothBearish); raised || got != nil {
		t.Errorf("a trade without rows: RaisedSettings = %v, %v, want nil and false", got, raised)
	}
}

// Bearish reads on a trade the flag applies to raise a copy of every row,
// and the stored rows stay exactly as they were.
func TestRaisedSettingsRaisesACopyOfEveryRow(t *testing.T) {
	mixedPercentage, mixedDepths := mixedRaises()

	cases := []struct {
		name       string
		reads      aggragates.DynamicParamsIndicators
		percentage bool
		depths     bool
	}{
		{"both bearish", bothBearish, true, true},
		{"mixed", mixedRead, mixedPercentage, mixedDepths},
	}

	for _, c := range cases {
		if !c.percentage && !c.depths {
			continue
		}

		trade := flaggedTrade()
		stored := trade.StrategyPair.StrategySettings
		before := cloneRows(stored)

		got, raised := dynamicparams.RaisedSettings(trade, c.reads)
		if !raised {
			t.Fatalf("%s: RaisedSettings must report the raise", c.name)
		}
		if &got[0] == &stored[0] {
			t.Fatalf("%s: the raised rows must be a copy", c.name)
		}
		if !reflect.DeepEqual(stored, before) {
			t.Fatalf("%s: the stored rows moved: %+v, want %+v", c.name, stored, before)
		}

		for index := range before {
			wantPercentage := before[index].Percentage
			if c.percentage {
				wantPercentage += dynamicparams.BearPercentagePoints
			}
			wantDepths := before[index].Depths
			if c.depths {
				wantDepths += float64(dynamicparams.BearDepths)
			}
			if got[index].Percentage != wantPercentage || got[index].Depths != wantDepths {
				t.Errorf("%s row %d: %v%% over %v depths, want %v%% over %v", c.name, index,
					got[index].Percentage, got[index].Depths, wantPercentage, wantDepths)
			}
		}
	}
}

// Every tick raises the configured rows afresh: two ticks under the same
// bearish reads, off the one stored trade, read the configured rows plus the
// amounts both times — never twice the amounts — because the raise is never
// written back for a later tick to raise again.
func TestRaisedSettingsNeverCompounds(t *testing.T) {
	trade := flaggedTrade()
	configured := cloneRows(trade.StrategyPair.StrategySettings)

	first, _ := dynamicparams.RaisedSettings(trade, bothBearish)
	second, _ := dynamicparams.RaisedSettings(trade, bothBearish)

	for tick, rows := range [][]aggragates.StrategySettings{first, second} {
		for index := range configured {
			wantPercentage := configured[index].Percentage + dynamicparams.BearPercentagePoints
			wantDepths := configured[index].Depths + float64(dynamicparams.BearDepths)
			if rows[index].Percentage != wantPercentage || rows[index].Depths != wantDepths {
				t.Errorf("tick %d row %d: %v%% over %v depths, want %v%% over %v", tick+1, index,
					rows[index].Percentage, rows[index].Depths, wantPercentage, wantDepths)
			}
		}
	}
}

// The first entry of a ladder that opens while raised is sized for the extra
// depth: ladder.CalculateInitialBid on the sizing copy the chain's
// EntrySettings make sizes the raised row — its depths plus BearDepths at its
// percentage plus BearPercentagePoints — a smaller first entry than the
// configured row sizes, while the trade keeps its own rows.
func TestRaisedSettingsSizeTheFirstEntryForTheExtraDepth(t *testing.T) {
	const budget = 10000.0

	trade := flaggedTrade()
	stored := trade.StrategyPair.StrategySettings
	configured := cloneRows(stored)

	rows, raised := dynamicparams.RaisedSettings(trade, bothBearish)
	if !raised {
		t.Fatal("both bearish reads must raise the rows")
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
