package ladder

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
