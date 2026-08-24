package trades

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func TestCountFilledEntriesCountsDistinctEntryOrders(t *testing.T) {
	trade := aggragates.Trades{History: []aggragates.TradesHistory{
		{Type: "BUY", Quantity: 1, Price: 100, OrderId: 10},
		// Partial fills of the same order id must not advance the ladder row.
		{Type: "BUY", Quantity: 0.5, Price: 99, OrderId: 11},
		{Type: "BUY", Quantity: 0.5, Price: 99, OrderId: 11},
		// Sells never count as entries.
		{Type: "SELL", Quantity: 1, Price: 101, OrderId: 12},
	}}

	if got := CountFilledEntries(trade); got != 2 {
		t.Fatalf("filled entries = %d, want 2", got)
	}
}

func TestCountFilledEntriesUsesSellForInverseTrades(t *testing.T) {
	trade := aggragates.Trades{Inverse: true, History: []aggragates.TradesHistory{
		{Type: "SELL", Quantity: 1, Price: 100, OrderId: 20},
		{Type: "BUY", Quantity: 1, Price: 99, OrderId: 21},
	}}

	if got := CountFilledEntries(trade); got != 1 {
		t.Fatalf("inverse filled entries = %d, want 1", got)
	}
}

func TestCountFilledEntriesGivesLegacyRowsWithoutOrderIdOwnIdentity(t *testing.T) {
	trade := aggragates.Trades{History: []aggragates.TradesHistory{
		{Type: "BUY", Quantity: 1, Price: 100},
		{Type: "BUY", Quantity: 1, Price: 100},
	}}

	if got := CountFilledEntries(trade); got != 2 {
		t.Fatalf("legacy filled entries = %d, want 2", got)
	}
}

func TestSettingsIndexOrBaseFallsBackToBaseRow(t *testing.T) {
	single := []aggragates.StrategySettings{{Depths: 7}}
	three := []aggragates.StrategySettings{{Depths: 7}, {Depths: 7}, {Depths: 7}}

	// A single row applies to every depth.
	for depth := 0; depth < 8; depth++ {
		if got := SettingsIndexOrBase(single, depth); got != 0 {
			t.Fatalf("single-row index for depth %d = %d, want 0", depth, got)
		}
	}
	// An existing row is used exactly.
	if got := SettingsIndexOrBase(three, 1); got != 1 {
		t.Fatalf("existing row index = %d, want 1", got)
	}
	// A missing row falls back to the BASE row, never the last one.
	if got := SettingsIndexOrBase(three, 5); got != 0 {
		t.Fatalf("missing row index = %d, want 0 (base row fallback)", got)
	}
	if got := SettingsIndexOrBase(three, -1); got != 0 {
		t.Fatalf("negative index = %d, want 0", got)
	}
}

func TestCountFilledEntriesSkipsAccountingRows(t *testing.T) {
	// An impasse parent receives its children's profits as BUY rows priced at
	// the 1e-13 sentinel — ledger entries, not market fills.
	trade := aggragates.Trades{History: []aggragates.TradesHistory{
		{Type: "BUY", Quantity: 1, Price: 100, OrderId: 30},
		{Type: "BUY", Quantity: 2, Price: 98, OrderId: 31},
		{Type: "BUY", Quantity: 5.5, Price: 0.0000000000001, OrderId: 32},
	}}

	if got := CountFilledEntries(trade); got != 2 {
		t.Fatalf("filled entries = %d, want 2 (accounting row must not count)", got)
	}
}

func overrideFixtureTrade() aggragates.Trades {
	return aggragates.Trades{
		ID: 7,
		Strategy: aggragates.Strategies{
			Params: aggragates.StrategyParams{Pairs: 3, Impasse: true},
		},
		StrategyPair: aggragates.StrategiesPairs{
			StrategySettings: []aggragates.StrategySettings{
				{Percentage: 1, Depths: 5, MinDepths: 2, Multiplier: 2},
				{Percentage: 2, Depths: 5, MinDepths: 2, Multiplier: 2},
			},
		},
	}
}

func TestApplySettingsOverride_NilOverrideReturnsTradeUntouched(t *testing.T) {
	trade := overrideFixtureTrade()

	got := ApplySettingsOverride(trade)

	if len(got.StrategyPair.StrategySettings) != 2 || got.StrategyPair.StrategySettings[0].Percentage != 1 {
		t.Fatalf("ladder changed without an override: %+v", got.StrategyPair.StrategySettings)
	}
	if got.Strategy.Params.Pairs != 3 || !got.Strategy.Params.Impasse {
		t.Fatalf("params changed without an override: %+v", got.Strategy.Params)
	}
}

func TestApplySettingsOverride_ReplacesLadderAndKeepsBaseParamsWhenNil(t *testing.T) {
	trade := overrideFixtureTrade()
	trade.SettingsOverride = &aggragates.TradeSettingsOverride{
		StrategySettings: []aggragates.StrategySettings{{Percentage: 9, Depths: 3, MinDepths: 1, Multiplier: 1.5}},
	}

	got := ApplySettingsOverride(trade)

	if len(got.StrategyPair.StrategySettings) != 1 || got.StrategyPair.StrategySettings[0].Percentage != 9 {
		t.Fatalf("ladder not replaced: %+v", got.StrategyPair.StrategySettings)
	}
	if got.Strategy.Params.Pairs != 3 || !got.Strategy.Params.Impasse {
		t.Fatalf("nil override params must keep base params, got %+v", got.Strategy.Params)
	}
	if EffectiveStrategySettings(trade)[0].Percentage != 9 {
		t.Fatalf("EffectiveStrategySettings must read the override ladder")
	}
	if EffectiveParams(trade).Pairs != 3 {
		t.Fatalf("EffectiveParams must fall back to the strategy params")
	}
}

func TestApplySettingsOverride_ReplacesParamsWhenSet(t *testing.T) {
	trade := overrideFixtureTrade()
	trade.SettingsOverride = &aggragates.TradeSettingsOverride{
		StrategySettings: []aggragates.StrategySettings{{Percentage: 4, Depths: 2, MinDepths: 1, Multiplier: 1}},
		Params:           &aggragates.StrategyParams{Pairs: 1, UseAI: true},
	}

	got := ApplySettingsOverride(trade)

	if got.Strategy.Params.Pairs != 1 || !got.Strategy.Params.UseAI || got.Strategy.Params.Impasse {
		t.Fatalf("params not replaced: %+v", got.Strategy.Params)
	}
	if EffectiveParams(trade).UseAI != true {
		t.Fatalf("EffectiveParams must read the override params")
	}
}

func TestApplySettingsOverride_DoesNotAliasInputOrOverride(t *testing.T) {
	trade := overrideFixtureTrade()
	trade.SettingsOverride = &aggragates.TradeSettingsOverride{
		StrategySettings: []aggragates.StrategySettings{{Percentage: 9, Depths: 3, MinDepths: 1, Multiplier: 1.5}},
	}

	got := ApplySettingsOverride(trade)
	got.StrategyPair.StrategySettings[0].Percentage = 42

	if trade.SettingsOverride.StrategySettings[0].Percentage != 9 {
		t.Fatalf("overlay aliased the override backing array")
	}
	if trade.StrategyPair.StrategySettings[0].Percentage != 1 {
		t.Fatalf("overlay mutated the input trade ladder")
	}
}

func TestApplySettingsOverride_EmptyLadderKeepsPairLadder(t *testing.T) {
	trade := overrideFixtureTrade()
	trade.SettingsOverride = &aggragates.TradeSettingsOverride{StrategySettings: []aggragates.StrategySettings{}}

	got := ApplySettingsOverride(trade)

	if len(got.StrategyPair.StrategySettings) != 2 {
		t.Fatalf("empty override ladder must keep the pair ladder, got %d rows", len(got.StrategyPair.StrategySettings))
	}
}

func TestApplySettingsOverrides_OverlaysEveryTradeWithoutTouchingInput(t *testing.T) {
	first := overrideFixtureTrade()
	second := overrideFixtureTrade()
	second.ID = 8
	second.SettingsOverride = &aggragates.TradeSettingsOverride{
		StrategySettings: []aggragates.StrategySettings{{Percentage: 5, Depths: 2, MinDepths: 1, Multiplier: 1}},
	}
	input := []aggragates.Trades{first, second}

	got := ApplySettingsOverrides(input)

	if len(got) != 2 || got[0].ID != 7 || got[1].ID != 8 {
		t.Fatalf("order or length changed: %+v", got)
	}
	if got[1].StrategyPair.StrategySettings[0].Percentage != 5 {
		t.Fatalf("second trade not overlaid")
	}
	if input[1].StrategyPair.StrategySettings[0].Percentage != 1 {
		t.Fatalf("input slice mutated")
	}
}
