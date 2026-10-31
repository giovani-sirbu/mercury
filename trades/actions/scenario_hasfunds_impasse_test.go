package actions_test

import (
	"strings"
	"testing"

	"github.com/adshao/go-binance/v2/common"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/exchange"
	exchangeAggregates "github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// A trade with no fills holds nothing to average down: a funds shortfall on
// its first entry is a plain block, never an impasse. The live engine used
// to flip such a trade to impasse (sisyphus never did), and the next tick
// then ran createChildrenTrades → sellAll against no position.
func TestHasFundsEdge_FirstEntryNeverEntersImpasse(t *testing.T) {
	customActions := hasFundsCustomActions()
	customActions.GetUserAssets = func() ([]exchangeAggregates.UserAssetRecord, *common.APIError) {
		return []exchangeAggregates.UserAssetRecord{{Asset: "USDC", Free: "100"}}, nil
	}

	trade := scenarioBuildTrade("buy", 100000, false) // no history: a first entry
	trade.Strategy.Params.Impasse = true

	event := events.Events{
		Exchange: exchange.Exchange{IsCustom: true, CustomActions: customActions},
		Trade:    trade,
		Events:   map[string]func(events.Events) (events.Events, error){"updateTrade": EmptyUpdateTrade},
		Params: aggragates.Params{
			OldPosition: "buy",
			// The wallet is overdrawn by inverse trades: the only way a first
			// entry reaches the shortfall branch (its needed quantity is 0).
			InverseUsedAmount: []aggragates.UsedAmountResult{{UsedAmount: 150, QuoteCurrency: "USDC"}},
		},
	}

	got, err := actions.HasFunds(event)
	if err == nil {
		t.Fatal("expected HasFunds to reject the overdrawn first entry")
	}
	if !strings.Contains(err.Error(), "Insufficient funds") {
		t.Errorf("expected insufficient-funds error, got: %v", err)
	}
	if got.Trade.PositionType == "impasse" {
		t.Fatal("a first entry must never enter impasse")
	}
	if got.Trade.Status != aggragates.Blocked {
		t.Errorf("expected the trade blocked on the shortfall, got %q", got.Trade.Status)
	}
}
