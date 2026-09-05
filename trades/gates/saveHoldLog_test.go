package gates

import (
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func holdLogEvent(trade aggragates.Trades, at time.Time) events.Events {
	return events.Events{
		Trade: trade,
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": testutil.NopUpdateTrade,
		},
		Params:    aggragates.Params{OldPosition: "buy", OldPositionPrice: 100},
		Timestamp: at.UnixMilli(),
	}
}

func holdMessages(trade aggragates.Trades) []string {
	var out []string
	for _, entry := range trade.Logs {
		if strings.HasPrefix(entry.Message, "Hold ") {
			out = append(out, entry.Message)
		}
	}
	return out
}

// The same reason on consecutive ticks is one row.
func TestSaveHoldLogCollapsesSameReason(t *testing.T) {
	at := time.Date(2025, 10, 10, 21, 0, 0, 0, time.UTC)
	event := holdLogEvent(testutil.NewHoldTrade("stopLoss", false), at)

	event, _ = SaveHoldLog(event, "stopLoss", "regime: add not allowed (4h downtrend-persist)")
	event.Trade.PositionType = "stopLoss"
	event.Timestamp = at.Add(15 * time.Minute).UnixMilli()
	event, _ = SaveHoldLog(event, "stopLoss", "regime: add not allowed (4h downtrend-persist)")

	if got := holdMessages(event.Trade); len(got) != 1 {
		t.Fatalf("same reason 15 minutes apart must stay one row, got %v", got)
	}
}

// A different reason on the same position is a new row: the prefix dedup
// used to swallow a capitulation freeze behind a regime veto.
func TestSaveHoldLogWritesReasonChange(t *testing.T) {
	at := time.Date(2025, 10, 10, 21, 0, 0, 0, time.UTC)
	event := holdLogEvent(testutil.NewHoldTrade("stopLoss", false), at)

	event, _ = SaveHoldLog(event, "stopLoss", "regime: add not allowed (4h downtrend-persist)")
	event.Trade.PositionType = "stopLoss"
	event.Timestamp = at.Add(time.Minute).UnixMilli()
	event, _ = SaveHoldLog(event, "stopLoss", "capitulation: freeze, one add already taken")

	got := holdMessages(event.Trade)
	if len(got) != 2 || !strings.Contains(got[1], "capitulation") {
		t.Fatalf("a reason change must write its own row, got %v", got)
	}
}

// The same reason still in force a day later is written again, so a long
// blockade is readable as a sequence of dated rows.
func TestSaveHoldLogRelogsAfterADay(t *testing.T) {
	at := time.Date(2025, 10, 10, 21, 0, 0, 0, time.UTC)
	event := holdLogEvent(testutil.NewHoldTrade("stopLoss", false), at)

	event, _ = SaveHoldLog(event, "stopLoss", "regime: add not allowed (4h downtrend-persist)")
	event.Trade.PositionType = "stopLoss"
	event.Timestamp = at.Add(holdRelogAfter - time.Minute).UnixMilli()
	event, _ = SaveHoldLog(event, "stopLoss", "regime: add not allowed (4h downtrend-persist)")
	if got := holdMessages(event.Trade); len(got) != 1 {
		t.Fatalf("under a day the row must not repeat, got %v", got)
	}

	event.Trade.PositionType = "stopLoss"
	event.Timestamp = at.Add(holdRelogAfter + time.Minute).UnixMilli()
	event, _ = SaveHoldLog(event, "stopLoss", "regime: add not allowed (4h downtrend-persist)")
	got := holdMessages(event.Trade)
	if len(got) != 2 {
		t.Fatalf("after a day the same reason must be written again, got %v", got)
	}
	if !event.Trade.Logs[len(event.Trade.Logs)-1].CreatedAt.Equal(at.Add(holdRelogAfter + time.Minute)) {
		t.Fatalf("the row must carry the tick clock, got %v", event.Trade.Logs[len(event.Trade.Logs)-1].CreatedAt)
	}
}

// Without any clock the rule is the plain same-message dedup.
func TestSaveHoldLogWithoutClockNeverRelogs(t *testing.T) {
	event := holdLogEvent(testutil.NewHoldTrade("stopLoss", false), time.Time{})
	event.Timestamp = 0
	event.Trade.UpdatedAt = time.Time{}

	event, _ = SaveHoldLog(event, "stopLoss", "regime: add not allowed (4h downtrend-persist)")
	event.Trade.PositionType = "stopLoss"
	event, _ = SaveHoldLog(event, "stopLoss", "regime: add not allowed (4h downtrend-persist)")

	if got := holdMessages(event.Trade); len(got) != 1 {
		t.Fatalf("no clock means no re-log, got %v", got)
	}
}
