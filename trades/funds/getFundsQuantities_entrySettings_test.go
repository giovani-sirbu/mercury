package funds_test

import (
	"reflect"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/funds"
	"github.com/giovani-sirbu/mercury/trades/internal/virtualexchange"
)

// fundsTrade is a long BTC/USDC ladder on one row, with the fills given.
func fundsTrade(history ...aggragates.TradesHistory) aggragates.Trades {
	trade := aggragates.Trades{Symbol: "BTC/USDC", PositionType: "buy", PositionPrice: 100000, History: history}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{LotSize: 5, PriceFilter: 2, MinNotional: 5}
	trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{
		{MinDepths: 6, Depths: 8, Percentage: 2, Multiplier: 2, Tolerance: 0.25},
	}
	return trade
}

// namedRows is the trade's row a depth deeper and a step wider, with a
// multiplier far off the configured one, so any read of it shows.
func namedRows() []aggragates.StrategySettings {
	return []aggragates.StrategySettings{
		{MinDepths: 6, Depths: 9, Percentage: 2.5, Multiplier: 20, Tolerance: 0.25},
	}
}

// fundsEvent carries the trade on a virtual exchange funded with quote, and
// the rows named for a first entry.
func fundsEvent(trade aggragates.Trades, entrySettings []aggragates.StrategySettings) events.Events {
	virtualexchange.ResetWallet()

	return events.Events{
		Trade:    trade,
		Exchange: virtualexchange.InitVirtualExchange([]aggregates.UserAssetRecord{{Asset: "USDC", Free: "1000"}}),
		Params:   aggragates.Params{EntrySettings: entrySettings},
	}
}

// fundsAnswer is everything GetFundsQuantities answers.
type fundsAnswer struct {
	remained float64
	needed   float64
	asset    string
}

// answerFor runs GetFundsQuantities on the trade with the rows named.
func answerFor(t *testing.T, trade aggragates.Trades, entrySettings []aggragates.StrategySettings) fundsAnswer {
	t.Helper()

	remained, needed, asset, err := funds.GetFundsQuantities(fundsEvent(trade, entrySettings))
	if err != nil {
		t.Fatalf("GetFundsQuantities: %v", err)
	}

	return fundsAnswer{remained: remained, needed: needed, asset: asset}
}

// The funds gate reads the trade's own rows whatever rows are named for a
// first entry. A first entry needs nothing here — no fill to multiply — and
// an add multiplies its last fill by the trade's own row, so named rows move
// neither answer, and the trade keeps its rows.
func TestGetFundsQuantitiesReadsTheTradesOwnRows(t *testing.T) {
	add := aggragates.TradesHistory{Type: "BUY", Quantity: 0.001, Price: 100000, OrderId: 1}

	cases := []struct {
		name       string
		trade      aggragates.Trades
		wantNeeded float64
	}{
		{"a first entry", fundsTrade(), 0},
		{"an add", fundsTrade(add), 200},
	}

	for _, c := range cases {
		stored := c.trade.StrategyPair.StrategySettings
		storedBefore := append([]aggragates.StrategySettings(nil), stored...)

		unnamed := answerFor(t, c.trade, nil)
		named := answerFor(t, c.trade, namedRows())

		if named != unnamed {
			t.Errorf("%s: named rows moved the answer: %+v, want %+v", c.name, named, unnamed)
		}
		if unnamed.needed != c.wantNeeded || unnamed.asset != "USDC" {
			t.Errorf("%s: needed %v %s, want %v USDC", c.name, unnamed.needed, unnamed.asset, c.wantNeeded)
		}
		if !reflect.DeepEqual(stored, storedBefore) {
			t.Errorf("%s: the trade must keep its own rows", c.name)
		}
	}
}
