package smarttakeloss

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// The row is INFO, on the trade, at the row's own price and the engine's
// clock — never the position price a re-anchor moved — and rebuildState reads
// a slow-decline marker written through it back as the ladder pending from
// that fill.
func TestLogRowIsAnInfoRowAtTheFillPrice(t *testing.T) {
	trade := watchedTrade()
	trade.PositionPrice = underTheBand
	at := testutil.At("17:45:00")
	row := LogRow(trade, Row{Message: SlowDeclineMessage(trade.PositionType, nil), Price: slowDeclineLastFill}, at)

	if row.TradeID != trade.ID || row.Type != aggragates.LOG_INFO {
		t.Fatalf("expected an INFO row on the trade, got %+v", row)
	}
	if row.Message != "Hold buy: smartTakeLoss: quiet slow decline, sell at the bollinger band" || row.Price != slowDeclineLastFill {
		t.Fatalf("the row carries the marker and the fill, got %+v", row)
	}
	if !row.CreatedAt.Equal(at) || !row.UpdatedAt.Equal(at) {
		t.Fatalf("the row is stamped with the engine's clock, got %+v", row)
	}

	trade.Logs = append(trade.Logs, row)
	if st := rebuildState(trade); !st.slowDeclinePending || st.slowDeclinePendingFrom != slowDeclineLastFill {
		t.Fatalf("the written row must read back as the ladder pending from its fill, got %+v", st)
	}
}

// The exit row names the rule that sold and the level the sellLoss chain
// places its limit at, with the pair's PriceFilter decimals.
func TestExitMessagePrintsTheReasonAndTheLevel(t *testing.T) {
	trade := lastDepthLadder()
	if got := ExitMessage(trade, reasonCapitalProtection, 192.6543); got != "smartTakeLoss: sell at upper bollinger band (capital protection) 192.65" {
		t.Fatalf("unexpected exit message %q", got)
	}
	trade.StrategyPair.TradeFilters.PriceFilter = 0
	if got := ExitMessage(trade, reasonSellBand, 175.39); got != "smartTakeLoss: sell at slow-decline bollinger band 175.39" {
		t.Fatalf("without a price filter the shortest exact form is printed, got %q", got)
	}
}
