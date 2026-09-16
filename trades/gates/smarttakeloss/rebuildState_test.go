package smarttakeloss

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// activationRow is the row the engines write on the activation tick, framed
// "Hold buy: …" and carrying the activating fill's price.
func activationRow(price float64) aggragates.TradesLogs {
	return aggragates.TradesLogs{Message: ActivationMessage("buy"), Price: price, Type: aggragates.LOG_INFO}
}

func TestRebuildStateWithoutRows(t *testing.T) {
	trade := sizedLadder(false, fills(5, "17:38:00")...)
	// The newest fill is the last in slice order even when its stamp is
	// older than the one before it (live-testing stamps by hand).
	trade.History[4].CreatedAt = testutil.At("07:00:00")

	st := rebuildState(trade)
	if st.active || st.activationPrice != 0 || st.depthsAfterActivation != 0 {
		t.Fatalf("no row must mean no activation, got %+v", st)
	}
	if !st.armed || len(st.fills) != 5 || st.lastFill().Price != 179.78 {
		t.Fatalf("5 fills arm and the last fill is 179.78, got %+v", st)
	}
}

func TestRebuildStateFindsTheFramedRowAndTheFirstOneWins(t *testing.T) {
	trade := sizedLadder(false, fills(5, "17:38:00")...)
	trade.Logs = []aggragates.TradesLogs{
		{Message: "Hold stopLoss: cooldown: depth 5 held for 30m0s", Price: 180},
		activationRow(179.78),
		activationRow(175.83), // a duplicate written under a lost lock
	}
	st := rebuildState(trade)
	if !st.active || st.activationPrice != 179.78 {
		t.Fatalf("the first activation row is the activation, got %+v", st)
	}
}

func TestRebuildStateIgnoresARowWithoutAPrice(t *testing.T) {
	trade := sizedLadder(false, fills(5, "17:38:00")...)
	trade.Logs = []aggragates.TradesLogs{activationRow(0)}
	if st := rebuildState(trade); st.active {
		t.Fatalf("a marker without a price carries no fill, got %+v", st)
	}
}

// The counter is distinct entry orders after the activating fill in slice
// order: a partial fill of the sixth order is one depth, and an accounting
// row or a SELL is no depth at all.
func TestRebuildStateCountsDepthsAfterTheActivatingFill(t *testing.T) {
	trade := sizedLadder(false, fills(5, "17:38:00")...)
	trade.Logs = []aggragates.TradesLogs{activationRow(179.78)}
	if st := rebuildState(trade); st.depthsAfterActivation != 0 {
		t.Fatalf("nothing filled after the activation, got %+v", st)
	}

	trade.History = append(trade.History,
		aggragates.TradesHistory{Type: "BUY", Quantity: 0.5, Price: 175.83, OrderId: 6, CreatedAt: testutil.At("18:41:00")},
		aggragates.TradesHistory{Type: "BUY", Quantity: 0.5, Price: 175.83, OrderId: 6, CreatedAt: testutil.At("18:42:00")},
		aggragates.TradesHistory{Type: "BUY", Quantity: 1, Price: 1e-13},
		aggragates.TradesHistory{Type: "SELL", Quantity: 1, Price: 190, OrderId: 7},
	)
	st := rebuildState(trade)
	if st.depthsAfterActivation != 1 || len(st.fills) != 6 || st.lastFill().Price != 175.83 {
		t.Fatalf("one partially filled order after the activation is one depth, got %+v", st)
	}
}

// After the activation the ladder can take profit, sell and re-anchor
// (update_buy) without a fill; the next depth then fills ABOVE the
// activating price and is still the one permitted depth.
func TestRebuildStateCountsAFillAboveTheActivationPrice(t *testing.T) {
	trade := sizedLadder(false, fills(5, "17:38:00")...)
	trade.Logs = []aggragates.TradesLogs{activationRow(179.78)}
	trade.History = append(trade.History, aggragates.TradesHistory{Type: "BUY", Quantity: 1, Price: 183.10, OrderId: 6})
	if st := rebuildState(trade); st.depthsAfterActivation != 1 {
		t.Fatalf("a fill after the activation counts whatever its price, got %+v", st)
	}
}

// With no fill at the row's price the count falls back to the fills strictly
// beyond it.
func TestRebuildStateFallsBackToFillsBeyondThePrice(t *testing.T) {
	trade := sizedLadder(false, fills(6, "18:41:00")...) // …, 179.78, 175.83
	trade.Logs = []aggragates.TradesLogs{activationRow(178)}
	if st := rebuildState(trade); !st.active || st.depthsAfterActivation != 1 {
		t.Fatalf("only 175.83 is under 178, got %+v", st)
	}
}

func TestRebuildStateInverseCountsHigherFills(t *testing.T) {
	trade := sizedLadder(true, risingFills(5, "17:38:00")...) // 100 … 108
	trade.Logs = []aggragates.TradesLogs{activationRow(106)}
	if st := rebuildState(trade); st.depthsAfterActivation != 1 || st.lastFill().Price != 108 {
		t.Fatalf("108 filled after the activation at 106, got %+v", st)
	}

	trade.Logs = []aggragates.TradesLogs{activationRow(105)}
	if st := rebuildState(trade); st.depthsAfterActivation != 2 {
		t.Fatalf("the fallback counts the fills above 105, got %+v", st)
	}
}
