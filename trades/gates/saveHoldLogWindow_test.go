package gates

import (
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"testing"
	"time"
)

// Two reasons alternating tick by tick collapse to one row per reason per
// day: comparing with the last row only, run 98 wrote 18 rows a day on one
// trade that flipped between the cooldown and the regime reason.
func TestSaveHoldLogCollapsesAlternatingReasonsWithinWindow(t *testing.T) {
	at := time.Date(2025, 10, 10, 21, 0, 0, 0, time.UTC)
	const reasonA = "regime: add not allowed (4h downtrend-persist)"
	const reasonB = "capitulation: freeze, one add already taken"
	event := holdLogEvent(testutil.NewHoldTrade("stopLoss", false), at)

	step := func(reason string, offset time.Duration) {
		event.Trade.PositionType = "stopLoss"
		event.Timestamp = at.Add(offset).UnixMilli()
		event, _ = SaveHoldLog(event, "stopLoss", reason)
	}

	step(reasonA, 0)
	step(reasonB, 15*time.Minute)
	step(reasonA, 30*time.Minute)
	step(reasonB, 45*time.Minute)
	if got := holdMessages(event.Trade); len(got) != 2 {
		t.Fatalf("A-B-A-B within the window must be two rows, got %v", got)
	}

	step(reasonA, holdRelogAfter+time.Minute)
	if got := holdMessages(event.Trade); len(got) != 3 {
		t.Fatalf("A a day later must be written again, got %v", got)
	}
	step(reasonB, holdRelogAfter+2*time.Minute)
	if got := holdMessages(event.Trade); len(got) != 3 {
		t.Fatalf("B's row is 23h47m old and must still collapse, got %v", got)
	}
	step(reasonB, holdRelogAfter+16*time.Minute)
	if got := holdMessages(event.Trade); len(got) != 4 {
		t.Fatalf("B a day after its row must be written again, got %v", got)
	}
}
