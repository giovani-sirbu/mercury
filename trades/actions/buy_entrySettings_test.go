package actions_test

import (
	"reflect"
	"testing"

	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// entryWallet is the quote balance every first entry below is sized off, as
// the string the virtual exchange serves and as the budget it parses to.
const (
	entryWallet       = "100000"
	entryWalletBudget = 100000.0
)

// deeperEntrySettings is the scenario rows a depth deeper and a step wider:
// the shape of the rows an engine names for a first entry while it raises
// them.
func deeperEntrySettings() []aggragates.StrategySettings {
	rows := scenarioSettings()
	for index := range rows {
		rows[index].Depths++
		rows[index].Percentage += 0.5
	}
	return rows
}

// firstEntryQuantity is the quantity Buy places for a first entry sized off
// entryWalletBudget from the rows of the trade it is handed: the initial bid
// in base units, fitted to the lot size.
func firstEntryQuantity(t *testing.T, trade aggragates.Trades) float64 {
	t.Helper()

	bid, err := ladder.CalculateInitialBid(entryWalletBudget, trade, 0)
	if err != nil {
		t.Fatalf("the fixture must size a first entry off this wallet: %v", err)
	}

	return helpers.ToFixed(bid/trade.PositionPrice, int(trade.StrategyPair.TradeFilters.LotSize))
}

// A first entry is sized from the rows the engine named for it: a ladder a
// depth deeper opens with a smaller first entry, exactly the one those rows
// size — and the trade the chain carries on keeps its own rows, the very
// array, untouched.
func TestBuySizesAFirstEntryOnTheEntrySettings(t *testing.T) {
	trade := scenarioBuildTrade("buy", 100000, false)
	stored := trade.StrategyPair.StrategySettings
	storedBefore := append([]aggragates.StrategySettings(nil), stored...)
	named := deeperEntrySettings()

	event := scenarioBuildEvent(trade, "USDC", entryWallet)
	event.Params.EntrySettings = named

	got, err := actions.Buy(event)
	if err != nil {
		t.Fatalf("Buy returned error: %v", err)
	}

	want := firstEntryQuantity(t, aggragates.Params{EntrySettings: named}.SizingTrade(trade))
	if got.Params.Quantity != want {
		t.Fatalf("first entry on the named rows = %v, want %v", got.Params.Quantity, want)
	}
	if configured := firstEntryQuantity(t, trade); got.Params.Quantity >= configured {
		t.Fatalf("a first entry sized a depth deeper = %v, want less than the configured %v", got.Params.Quantity, configured)
	}

	if &got.Trade.StrategyPair.StrategySettings[0] != &stored[0] || !reflect.DeepEqual(stored, storedBefore) {
		t.Fatal("the trade must keep its own rows")
	}
}

// With no rows named — EntrySettings nil or empty — a first entry is sized
// exactly as before: from the trade's own rows.
func TestBuyWithoutEntrySettingsSizesFromTheTradesRows(t *testing.T) {
	for name, named := range map[string][]aggragates.StrategySettings{
		"no rows named":        nil,
		"an empty set of rows": {},
	} {
		trade := scenarioBuildTrade("buy", 100000, false)

		event := scenarioBuildEvent(trade, "USDC", entryWallet)
		event.Params.EntrySettings = named

		got, err := actions.Buy(event)
		if err != nil {
			t.Fatalf("%s: Buy returned error: %v", name, err)
		}
		if want := firstEntryQuantity(t, trade); got.Params.Quantity != want {
			t.Errorf("%s: first entry = %v, want exactly %v", name, got.Params.Quantity, want)
		}
	}
}

// An add multiplies the last fill by the trade's own row, whatever rows are
// named for a first entry — even a named multiplier far off the configured
// one moves nothing.
func TestBuyAddsIgnoreTheEntrySettings(t *testing.T) {
	named := scenarioSettings()
	named[0].Multiplier *= 10

	for name, entrySettings := range map[string][]aggragates.StrategySettings{
		"no rows named": nil,
		"rows named":    named,
	} {
		trade := scenarioBuildTrade("buy", 98000, false)
		scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)

		event := scenarioBuildEvent(trade, "USDC", "1000")
		event.Params.EntrySettings = entrySettings

		got, err := actions.Buy(event)
		if err != nil {
			t.Fatalf("%s: Buy returned error: %v", name, err)
		}
		AssertFloatEqual(t, got.Params.Quantity, 0.002, 1e-9, name+": the add on the trade's own multiplier")
	}
}

// A pair with a configured initial bid never consults the depth ladder for
// its first entry, so rows a depth deeper change nothing for it: the extra
// depth is inert there.
func TestBuyFirstEntryWithAnInitialBidIgnoresTheExtraDepth(t *testing.T) {
	withInitialBid := func() []aggragates.StrategySettings {
		rows := scenarioSettings()
		rows[0].InitialBid = 2
		return rows
	}

	named := withInitialBid()
	named[0].Depths++
	named[0].Percentage += 0.5

	for name, entrySettings := range map[string][]aggragates.StrategySettings{
		"no rows named": nil,
		"rows named":    named,
	} {
		trade := scenarioBuildTrade("buy", 100000, false)
		trade.StrategyPair.StrategySettings = withInitialBid()

		event := scenarioBuildEvent(trade, "USDC", "0")
		event.Params.EntrySettings = entrySettings

		got, err := actions.Buy(event)
		if err != nil {
			t.Fatalf("%s: Buy returned error: %v", name, err)
		}
		AssertFloatEqual(t, got.Params.Quantity, 0.0001, 1e-9, name+": the configured initial bid")
	}
}
