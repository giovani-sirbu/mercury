package actions_test

import (
	"reflect"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/profit"
)

// acceptLossLadder is an underwater two-fill BTC/USDC ladder about to sell at
// the loss, its fills paying commissions in feeAsset (none when empty).
func acceptLossLadder(feeAsset string, fee float64) events.Events {
	trade := scenarioBuildTrade("sellLoss", 93000, false)
	scenarioAppendHistory(&trade, "BUY", 0.00006, dustEntryPrice, feeAsset, fee)
	scenarioAppendHistory(&trade, "BUY", 0.00012, 94500.25, feeAsset, fee*2)
	return events.Events{Trade: trade}
}

// AcceptLoss books the figure profit.SimulatedCloseProfit reads — the one the
// smart take loss's loss cap measures a band sale by — on the trade and in the
// params the chain hands on, whatever commission the fills paid: none, base or
// quote. The figure is a loss, so a cap that read another number than the one
// booked would hold or release a sale on a loss the sale never takes.
func TestAcceptLossBooksTheSimulatedCloseProfit(t *testing.T) {
	cases := []struct {
		name     string
		feeAsset string
		fee      float64
	}{
		{"no fee", "", 0},
		{"a base fee", "BTC", 0.00000006},
		{"a quote fee", "USDC", 0.0057},
	}

	for _, c := range cases {
		event := acceptLossLadder(c.feeAsset, c.fee)
		want := profit.SimulatedCloseProfit(event)
		if want >= 0 {
			t.Fatalf("%s: fixture drifted, the close books %v and no longer a loss", c.name, want)
		}

		got, err := actions.AcceptLoss(event)
		if err != nil {
			t.Fatalf("%s: AcceptLoss returned error: %v", c.name, err)
		}
		if got.Params.Profit != want || got.Trade.Profit != want {
			t.Errorf("%s: AcceptLoss booked params %v and trade %v, want SimulatedCloseProfit %v", c.name, got.Params.Profit, got.Trade.Profit, want)
		}
	}
}

// AcceptLoss hands the trade back as it took it but for the profit: the history
// and the dust the close was simulated on are not left on it, and the figure
// is the one thing it writes.
func TestAcceptLossChangesNothingButTheProfit(t *testing.T) {
	event := acceptLossLadder("BTC", 0.00000006)
	want := event.Trade

	got, err := actions.AcceptLoss(event)
	if err != nil {
		t.Fatalf("AcceptLoss returned error: %v", err)
	}

	want.Profit = got.Trade.Profit
	if !reflect.DeepEqual(got.Trade, want) {
		t.Fatalf("AcceptLoss moved more than the profit:\n got %+v\nwant %+v", got.Trade, want)
	}
}
