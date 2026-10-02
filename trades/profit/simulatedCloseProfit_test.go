package profit_test

import (
	"math"
	"reflect"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/fees"
	"github.com/giovani-sirbu/mercury/trades/profit"
	"github.com/giovani-sirbu/mercury/trades/quantities"
)

// closingTrade is a BTC/USDC trade about to close at closePrice through the
// sellLoss chain, on the history given: the close the figure is read for is
// priced at the position price itself, with no tolerance haircut.
func closingTrade(closePrice float64, history ...aggragates.TradesHistory) aggragates.Trades {
	return aggragates.Trades{
		Symbol:        "BTC/USDC",
		PositionType:  "sellLoss",
		PositionPrice: closePrice,
		ProfitAsset:   "USDC",
		StrategyPair: aggragates.StrategiesPairs{
			TradeFilters:     aggragates.TradeFilters{LotSize: 5, PriceFilter: 2},
			StrategySettings: []aggragates.StrategySettings{{Tolerance: 0.25}},
		},
		History: history,
	}
}

// entry is one executed entry order, with the commission it paid when feeAsset
// names one.
func entry(side string, quantity, price float64, feeAsset string, fee float64) aggragates.TradesHistory {
	row := aggragates.TradesHistory{Type: side, Quantity: quantity, Price: price}
	if feeAsset != "" {
		row.Fees = []aggragates.TradesFees{{Asset: feeAsset, Fee: fee}}
	}
	return row
}

// closeProfitBeforeTheExtraction is the figure acceptLoss assigned before the
// simulation moved into profit.SimulatedCloseProfit, written out once more
// statement for statement: the control the extraction is measured against.
func closeProfitBeforeTheExtraction(event events.Events) float64 {
	quantity, dust, historyType := quantities.SimulatedClose(event)
	trade := event.Trade
	trade.Dust = dust
	trade.History = append(trade.History, aggragates.TradesHistory{
		Type:     historyType,
		Quantity: quantity,
		Price:    profit.SimulatedClosePrice(event.Trade),
	})
	netProfit := profit.GetProfit(trade)
	closingFees := fees.GetFees(event)
	openingFees := fees.UnembodiedOpeningFees(event)
	netProfit -= closingFees + openingFees

	return netProfit
}

// The figure is what the close books: the position sold at the close price,
// against what the entries cost, less the fees the close is charged — here
// worked out by hand for a commission in no asset, in the base, in the quote
// and in a third asset the wallet prices.
func TestSimulatedCloseProfitIsWhatTheCloseBooks(t *testing.T) {
	cases := []struct {
		name    string
		event   events.Events
		wantNet float64
	}{
		{
			name:    "no fee: the close value over the entry cost",
			event:   events.Events{Trade: closingTrade(90, entry("BUY", 1, 100, "", 0))},
			wantNet: 90 - 100,
		},
		{
			// The commission came out of the base the fill received, so the close
			// sells that much less, and one closing leg is charged on top.
			name:    "a base fee: a smaller close and one closing leg",
			event:   events.Events{Trade: closingTrade(90, entry("BUY", 1, 100, "BTC", 0.001))},
			wantNet: 0.999*90 - 100 - 0.001*100,
		},
		{
			// A spot quote fee leaves the quantity alone and is paid by nothing
			// else, so it is charged for the opening leg and for the closing one.
			name:    "a quote fee: charged for both legs",
			event:   events.Events{Trade: closingTrade(90, entry("BUY", 1, 100, "USDC", 0.1))},
			wantNet: 90 - 100 - 0.1 - 0.1,
		},
		{
			name: "a third-asset fee the wallet prices: charged for both legs",
			event: events.Events{
				Trade:    closingTrade(90, entry("BUY", 1, 100, "BNB", 0.01)),
				WsPrices: map[string]float64{"BNB/USDC": 600},
			},
			wantNet: 90 - 100 - 0.01*600 - 0.01*600,
		},
		{
			name: "two fills close as one position",
			event: events.Events{Trade: closingTrade(90,
				entry("BUY", 1, 100, "", 0),
				entry("BUY", 2, 94, "", 0),
			)},
			wantNet: 3*90 - (100 + 2*94),
		},
	}

	for _, c := range cases {
		got := profit.SimulatedCloseProfit(c.event)
		if math.Abs(got-c.wantNet) > 1e-9 {
			t.Errorf("%s: SimulatedCloseProfit = %v, want %v", c.name, got, c.wantNet)
		}
	}
}

// The extraction changed nothing: the figure equals the one the inline
// simulation gave, on every shape of trade the close can meet, the lot-size
// dust, the inverse ladder and the fees of each asset included.
func TestSimulatedCloseProfitEqualsTheSimulationItWasExtractedFrom(t *testing.T) {
	inverse := closingTrade(110, entry("SELL", 1, 100, "", 0), entry("SELL", 2, 104, "BTC", 0.002))
	inverse.Inverse = true
	inverse.ProfitAsset = "BTC"

	dusty := closingTrade(93000, entry("BUY", 0.000063, 95862.54, "BTC", 0.00000006))

	cases := map[string]events.Events{
		"no fee":                  {Trade: closingTrade(90, entry("BUY", 1, 100, "", 0))},
		"a base fee":              {Trade: closingTrade(90, entry("BUY", 1, 100, "BTC", 0.001))},
		"a quote fee":             {Trade: closingTrade(90, entry("BUY", 1, 100, "USDC", 0.1))},
		"a third-asset fee":       {Trade: closingTrade(90, entry("BUY", 1, 100, "BNB", 0.01)), WsPrices: map[string]float64{"BNB/USDC": 600}},
		"every asset on a ladder": {Trade: closingTrade(88, entry("BUY", 1, 100, "BTC", 0.001), entry("BUY", 2, 94, "USDC", 0.2), entry("BUY", 4, 90, "BNB", 0.04)), WsPrices: map[string]float64{"BNB/USDC": 600}},
		"lot-size dust":           {Trade: dusty},
		"an inverse ladder":       {Trade: inverse},
		"no entry at all":         {Trade: closingTrade(90)},
	}

	for name, event := range cases {
		want := closeProfitBeforeTheExtraction(event)
		if got := profit.SimulatedCloseProfit(event); got != want {
			t.Errorf("%s: SimulatedCloseProfit = %v, the inline simulation gave %v", name, got, want)
		}
	}
}

// The close is priced on the pair's price grid, as an order is fitted to the
// exchange's filter: a position price with more decimals than the pair's price
// filter is floored to them (helpers.ToFixed never rounds up) before the close is
// valued, so a tick between two prices of the grid books the figure of the lower
// one — the loss the smart take loss's cap measures a sale by is the loss of a sale
// that never exceeds the price it was read at.
func TestSimulatedCloseProfitPricesTheCloseOnThePairsPriceGrid(t *testing.T) {
	for name, tc := range map[string]struct {
		price float64
		sold  float64
	}{
		"just over a price of the grid":  {90.004, 90.00},
		"just under the next one":        {90.019, 90.01},
		"between two, nearer the higher": {90.006, 90.00},
		"on the grid":                    {90.01, 90.01},
	} {
		got := profit.SimulatedCloseProfit(events.Events{Trade: closingTrade(tc.price, entry("BUY", 1, 100, "", 0))})
		if want := tc.sold - 100; math.Abs(got-want) > 1e-9 {
			t.Errorf("%s: SimulatedCloseProfit at %v = %v, want the close valued at %v: %v", name, tc.price, got, tc.sold, want)
		}
	}
}

// The simulation is on a copy: the history grows by its close row on a clone, so
// a caller whose slice has spare capacity never sees the row land past its
// length, and the event the caller holds — history, dust, profit — is exactly as
// it was handed in.
func TestSimulatedCloseProfitNeverWritesIntoTheCallersHistory(t *testing.T) {
	history := make([]aggragates.TradesHistory, 1, 4)
	history[0] = entry("BUY", 1, 100, "BTC", 0.001)
	event := events.Events{Trade: closingTrade(90)}
	event.Trade.History = history
	event.Trade.Dust = 0.5
	before := event.Trade

	profit.SimulatedCloseProfit(event)

	if spare := history[:cap(history)][1:]; !reflect.DeepEqual(spare, make([]aggragates.TradesHistory, 3)) {
		t.Fatalf("the close row was appended into the caller's backing array: %+v", spare)
	}
	if !reflect.DeepEqual(event.Trade, before) {
		t.Fatalf("the event was written: %+v, want %+v", event.Trade, before)
	}
}
