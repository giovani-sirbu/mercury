package smarttakeloss

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// The row is INFO, on the trade, at the row's own price and the engine's
// clock — and rebuildState reads it back as the activation.
func TestActivationLogIsAnInfoRowAtTheFillPrice(t *testing.T) {
	trade := armedTrade()
	trade.PositionPrice = 179.90
	at := testutil.At("17:45:00")
	row := ActivationLog(trade, Row{Message: ActivationMessage(trade.PositionType), Price: 179.78}, at)

	if row.TradeID != 45211 || row.Type != aggragates.LOG_INFO {
		t.Fatalf("expected an INFO row on trade 45211, got %+v", row)
	}
	if row.Message != "Hold buy: smartTakeLoss: Potential trend reversal" || row.Price != 179.78 {
		t.Fatalf("the row carries the marker and the fill, got %+v", row)
	}
	if !row.CreatedAt.Equal(at) || !row.UpdatedAt.Equal(at) {
		t.Fatalf("the row is stamped with the engine's clock, got %+v", row)
	}

	trade.Logs = append(trade.Logs, row)
	if st := rebuildState(trade); !st.active || st.activationPrice != 179.78 {
		t.Fatalf("the written row must read back as the activation, got %+v", st)
	}
}

func TestExitMessagePrintsTheReasonAndTheLevel(t *testing.T) {
	trade := armedTrade() // PriceFilter 2
	if got := ExitMessage(trade, "resistance line", 192.6543); got != "smartTakeLoss: sell at resistance line 192.65" {
		t.Fatalf("unexpected exit message %q", got)
	}
	trade.StrategyPair.TradeFilters.PriceFilter = 0
	if got := ExitMessage(trade, "tolerance under the last fill", 175.39); got != "smartTakeLoss: sell at tolerance under the last fill 175.39" {
		t.Fatalf("without a price filter the shortest exact form is printed, got %q", got)
	}
}
