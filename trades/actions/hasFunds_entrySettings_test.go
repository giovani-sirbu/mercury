package actions_test

import (
	"reflect"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// A first entry needs nothing from the funds gate — there is no fill to
// multiply — so rows named for it change nothing here: the entry passes, no
// shortfall is waived, and the trade keeps its own rows.
func TestHasFundsPassesAFirstEntryWhateverRowsAreNamed(t *testing.T) {
	for name, named := range map[string][]aggragates.StrategySettings{
		"no rows named": nil,
		"rows named":    deeperEntrySettings(),
	} {
		trade := scenarioBuildTrade("buy", 100000, false)
		stored := trade.StrategyPair.StrategySettings
		storedBefore := append([]aggragates.StrategySettings(nil), stored...)

		event := scenarioBuildEvent(trade, "USDC", entryWallet)
		event.Params.EntrySettings = named

		got, err := actions.HasFunds(event)
		if err != nil {
			t.Fatalf("%s: HasFunds refused a first entry: %v", name, err)
		}
		if got.Params.AvailableQuantity != 0 {
			t.Errorf("%s: a first entry needs no waiver, got %v", name, got.Params.AvailableQuantity)
		}
		if &got.Trade.StrategyPair.StrategySettings[0] != &stored[0] || !reflect.DeepEqual(stored, storedBefore) {
			t.Errorf("%s: the trade must keep its own rows", name)
		}
	}
}

// impasseTrade is a long parent four depths deep on an impasse strategy,
// deep enough that the capital it holds can size a ladder of its own rows.
func impasseTrade() aggragates.Trades {
	trade := scenarioBuildTrade("buy", 92000, false)
	trade.Strategy.Params.Impasse = true
	scenarioAppendHistory(&trade, "BUY", 0.01, 100000, "", 0)
	scenarioAppendHistory(&trade, "BUY", 0.02, 98000, "", 0)
	scenarioAppendHistory(&trade, "BUY", 0.04, 96040, "", 0)
	scenarioAppendHistory(&trade, "BUY", 0.08, 94120, "", 0)
	return trade
}

// unfundableRows is a ladder so deep that no capital this trade holds can
// size its first entry over the pair's minimum.
func unfundableRows() []aggragates.StrategySettings {
	rows := scenarioSettings()
	rows[0].MinDepths = 60
	rows[0].Depths = 60
	return rows
}

// The impasse check sizes a ladder from the trade's own rows, never from rows
// named for a first entry — a ladder that reaches it has fills. Named rows no
// capital could fund leave the check exactly where the trade's own rows put
// it, although handed to the trade as its own they would refuse it.
func TestHasFundsImpasseCheckReadsTheTradesOwnRows(t *testing.T) {
	trade := impasseTrade()
	stored := trade.StrategyPair.StrategySettings
	storedBefore := append([]aggragates.StrategySettings(nil), stored...)

	event := scenarioBuildEvent(trade, "USDC", "5")
	event.Params.EntrySettings = unfundableRows()

	got, err := actions.HasFunds(event)
	if err == nil {
		t.Fatal("expected HasFunds to refuse the short wallet")
	}
	if got.Trade.PositionType != "impasse" {
		t.Fatalf("PositionType = %q, want impasse on the trade's own rows", got.Trade.PositionType)
	}
	if !reflect.DeepEqual(stored, storedBefore) {
		t.Fatal("the trade must keep its own rows")
	}

	// The probe means something: as the trade's own rows, the unfundable
	// ladder keeps the trade out of impasse.
	probe := impasseTrade()
	probe.StrategyPair.StrategySettings = unfundableRows()

	refused, err := actions.HasFunds(scenarioBuildEvent(probe, "USDC", "5"))
	if err == nil {
		t.Fatal("expected HasFunds to refuse the short wallet")
	}
	if refused.Trade.PositionType == "impasse" {
		t.Fatal("rows no capital can fund must keep the trade out of impasse")
	}
}
