package cooldown

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// The rows the gate leaves behind, as firstFillState reads them: the marker
// and the Price column matter, the rest of the text does not.
func waitingRowAt(price float64) aggragates.TradesLogs {
	return aggragates.TradesLogs{Message: "Hold entry: " + FirstFillWaitingPrefix + "…", Price: price, Type: aggragates.LOG_INFO}
}

func armedRowAt(price float64) aggragates.TradesLogs {
	return aggragates.TradesLogs{Message: "Hold entry: " + FirstFillArmedPrefix + "…", Price: price, Type: aggragates.LOG_INFO}
}

func enteredRowAt(price float64) aggragates.TradesLogs {
	return aggragates.TradesLogs{Message: FirstFillEnteredPrefix + "above the reference …", Price: price, Type: aggragates.LOG_INFO}
}

// On spot the verdict is fetched for a new trade until the hold stands, and
// never again once it does or once the entry was released above the
// reference.
func TestFirstFillVerdictNeededOnlyBeforeTheHoldStands(t *testing.T) {
	trade := testutil.NewHoldTrade("buy", false)
	if !FirstFillVerdictNeeded(trade, "new") {
		t.Fatal("a new trade with no rows needs the verdict")
	}
	for _, position := range []string{"buy", "stopLoss", "takeProfit", ""} {
		if FirstFillVerdictNeeded(trade, position) {
			t.Fatalf("position %q has no first fill to judge", position)
		}
	}

	activated := trade
	activated.Logs = []aggragates.TradesLogs{waitingRowAt(100)}
	if FirstFillVerdictNeeded(activated, "new") {
		t.Fatal("a standing hold is priced, not judged again")
	}

	entered := trade
	entered.Logs = []aggragates.TradesLogs{waitingRowAt(100), enteredRowAt(102.6)}
	if FirstFillVerdictNeeded(entered, "new") {
		t.Fatal("an entry released above the reference is not judged again")
	}

	// A waiting row without a price carries no reference: not activated.
	unpriced := trade
	unpriced.Logs = []aggragates.TradesLogs{waitingRowAt(0)}
	if !FirstFillVerdictNeeded(unpriced, "new") {
		t.Fatal("a row without a price is no activation")
	}
}

// Futures keep the verdict-only gate: every tick of a new trade fetches,
// whatever the rows say.
func TestFirstFillVerdictNeededOnFuturesFollowsThePositionAlone(t *testing.T) {
	trade := testutil.NewHoldTrade("buy", false)
	trade.Strategy.TradeType = aggragates.Futures
	trade.Logs = []aggragates.TradesLogs{waitingRowAt(100), enteredRowAt(102.6)}

	if !FirstFillVerdictNeeded(trade, "new") {
		t.Fatal("a new futures trade fetches the verdict on every tick")
	}
	if FirstFillVerdictNeeded(trade, "stopLoss") {
		t.Fatal("an open futures position has no first fill to judge")
	}
}
