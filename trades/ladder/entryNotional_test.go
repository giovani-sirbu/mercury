package ladder

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The amount a trade has used is the quantity times the price of every entry
// fill, summed: a partial fill of one order adds up with the rest, and the exits
// a ladder has already booked are not part of it.
func TestEntryNotionalIsWhatEveryEntryFillCost(t *testing.T) {
	trade := aggragates.Trades{History: []aggragates.TradesHistory{
		{Type: "BUY", Quantity: 1, Price: 100, OrderId: 10},
		{Type: "BUY", Quantity: 0.5, Price: 96, OrderId: 11},
		{Type: "BUY", Quantity: 0.5, Price: 96, OrderId: 11},
		{Type: "SELL", Quantity: 1, Price: 130, OrderId: 12},
	}}

	if got, want := EntryNotional(trade), 100.0+0.5*96+0.5*96; got != want {
		t.Fatalf("entry notional = %f, want %f", got, want)
	}
}

// An inverse ladder enters with SELLs, so those rows are what it has used and
// its BUYs are the exits.
func TestEntryNotionalUsesSellsForInverseTrades(t *testing.T) {
	trade := aggragates.Trades{Inverse: true, History: []aggragates.TradesHistory{
		{Type: "SELL", Quantity: 1, Price: 100, OrderId: 20},
		{Type: "SELL", Quantity: 3, Price: 104, OrderId: 21},
		{Type: "BUY", Quantity: 1, Price: 90, OrderId: 22},
	}}

	if got, want := EntryNotional(trade), 100.0+3*104; got != want {
		t.Fatalf("inverse entry notional = %f, want %f", got, want)
	}
}

// The child-profit rows an impasse parent carries are bookkeeping at the
// sentinel price, and a row with no quantity filled nothing: neither adds to
// what the ladder has used, and a ladder with no fills has used nothing.
func TestEntryNotionalSkipsAccountingRowsAndEmptyLadders(t *testing.T) {
	ledgerOnly := aggragates.Trades{History: []aggragates.TradesHistory{
		{Type: "BUY", Quantity: 12.5, Price: AccountingPriceCeiling, OrderId: 30},
		{Type: "BUY", Quantity: 12.5, Price: 0.0000000000001, OrderId: 31},
	}}
	if got := EntryNotional(ledgerOnly); got != 0 {
		t.Fatalf("a child-profit ledger row is not an entry, got %f", got)
	}

	beside := aggragates.Trades{History: []aggragates.TradesHistory{
		{Type: "BUY", Quantity: 12.5, Price: AccountingPriceCeiling, OrderId: 30},
		{Type: "BUY", Quantity: 2, Price: 50, OrderId: 32},
		{Type: "BUY", Quantity: 0, Price: 80, OrderId: 33},
	}}
	if got := EntryNotional(beside); got != 100 {
		t.Fatalf("only the real fill is an entry, got %f", got)
	}

	if got := EntryNotional(aggragates.Trades{}); got != 0 {
		t.Fatalf("no fills must have used nothing, got %f", got)
	}
}

// AverageEntryPrice is built on the same fold: the notional over the quantity
// of the very rows EntryNotional counts, in every shape above, so a trade's used
// amount and its break even can never disagree on which rows are entries.
func TestAverageEntryPriceIsTheEntryNotionalOverTheEntryQuantity(t *testing.T) {
	cases := map[string]struct {
		trade    aggragates.Trades
		quantity float64
	}{
		"a ladder with partial fills and an exit": {
			trade: aggragates.Trades{History: []aggragates.TradesHistory{
				{Type: "BUY", Quantity: 1, Price: 100, OrderId: 10},
				{Type: "BUY", Quantity: 0.5, Price: 96, OrderId: 11},
				{Type: "BUY", Quantity: 0.5, Price: 96, OrderId: 11},
				{Type: "SELL", Quantity: 1, Price: 130, OrderId: 12},
			}},
			quantity: 2,
		},
		"an inverse ladder": {
			trade: aggragates.Trades{Inverse: true, History: []aggragates.TradesHistory{
				{Type: "SELL", Quantity: 1, Price: 100, OrderId: 20},
				{Type: "SELL", Quantity: 3, Price: 104, OrderId: 21},
				{Type: "BUY", Quantity: 1, Price: 90, OrderId: 22},
			}},
			quantity: 4,
		},
		"an accounting row beside a fill": {
			trade: aggragates.Trades{History: []aggragates.TradesHistory{
				{Type: "BUY", Quantity: 12.5, Price: AccountingPriceCeiling, OrderId: 30},
				{Type: "BUY", Quantity: 2, Price: 50, OrderId: 32},
			}},
			quantity: 2,
		},
	}

	for name, c := range cases {
		if got, want := AverageEntryPrice(c.trade), EntryNotional(c.trade)/c.quantity; got != want {
			t.Errorf("%s: average entry price = %f, want the entry notional over %v units, %f", name, got, c.quantity, want)
		}
	}
}
