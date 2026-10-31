package tradelog

import (
	"errors"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// statusHoldEvent is holdLogEvent for a trade found in status, the engines' chain copy reset to its
// open status (SettledStatus not held) with OldStatus naming what the tick found; published counts
// updateTrade's runs and published the status each one carried.
func statusHoldEvent(status aggragates.Status, at time.Time, published *[]aggragates.Status) events.Events {
	trade := testutil.NewHoldTrade("trailingBuy", false)
	trade.Status = aggragates.SettledStatus(status, false)
	event := holdLogEvent(trade, at)
	event.Params.OldStatus = status
	event.Events = map[string]func(events.Events) (events.Events, error){
		"updateTrade": func(e events.Events) (events.Events, error) {
			*published = append(*published, e.Trade.Status)
			return testutil.NopUpdateTrade(e)
		},
	}
	return event
}

// The hold row carries the hold's key, its position word is the state's name, and the trade goes
// out as hold from an open status, the blocked and paused ones kept; without OldStatus the status
// is untouched.
func TestSaveHoldLogKeysTheRowAndSettlesTheHoldStatus(t *testing.T) {
	at := time.Date(2026, time.October, 27, 12, 0, 0, 0, time.UTC)
	hold := gates.Hold{Reason: holdReason, Key: aggragates.LogKeyHoldDepthSpacing}
	for _, c := range []struct {
		found aggragates.Status
		want  aggragates.Status
	}{
		{aggragates.Active, aggragates.Hold},
		{aggragates.Hold, aggragates.Hold},
		{aggragates.Blocked, aggragates.Blocked},
		{aggragates.Paused, aggragates.Paused},
	} {
		var published []aggragates.Status
		held, err := SaveHoldLog(statusHoldEvent(c.found, at, &published), "trailingBuy", hold)
		if !errors.Is(err, events.ErrTradeHeld) || errors.Is(err, events.ErrHoldNotPersisted) {
			t.Fatalf("from %s: a written hold stops the chain with ErrTradeHeld alone, got %v", c.found, err)
		}
		if len(published) != 1 || published[0] != c.want || held.Trade.Status != c.want {
			t.Errorf("from %s: published %v, status %s, want %s once", c.found, published, held.Trade.Status, c.want)
		}
		row := held.Trade.Logs[len(held.Trade.Logs)-1]
		if row.MessageKey != aggragates.LogKeyHoldDepthSpacing || row.Message != "Hold trailingBuy: "+holdReason {
			t.Errorf("from %s: the row is %q keyed %q", c.found, row.Message, row.MessageKey)
		}
	}

	var published []aggragates.Status
	event := statusHoldEvent(aggragates.Active, at, &published)
	event.Params.OldStatus = ""
	if held, _ := SaveHoldLog(event, "trailingBuy", hold); held.Trade.Status != aggragates.Active || published[0] != aggragates.Active {
		t.Errorf("without OldStatus the status is untouched, got %s", held.Trade.Status)
	}
}

// A collapsed hold writes no row: found in hold it publishes nothing and says so
// (ErrHoldNotPersisted), the caller releasing its lock; found active it publishes the hold status
// alone, the position and price restored and no transition row, and stops the chain with plain
// ErrTradeHeld, the publish releasing the lock.
func TestACollapsedHoldPublishesTheHoldStatusOnlyFromActive(t *testing.T) {
	at := time.Date(2026, time.October, 27, 12, 0, 0, 0, time.UTC)
	hold := gates.Hold{Reason: holdReason, Key: aggragates.LogKeyHoldDepthSpacing}
	var first []aggragates.Status
	written, _ := SaveHoldLog(statusHoldEvent(aggragates.Active, at, &first), "trailingBuy", hold)
	logs := written.Trade.Logs

	var fromHold []aggragates.Status
	again := statusHoldEvent(aggragates.Hold, at.Add(time.Minute), &fromHold)
	again.Trade.Logs = logs
	if _, err := SaveHoldLog(again, "trailingBuy", hold); !errors.Is(err, events.ErrHoldNotPersisted) || len(fromHold) != 0 {
		t.Fatalf("collapsed from hold: published %v, err %v, want nothing and ErrHoldNotPersisted", fromHold, err)
	}

	var fromActive []aggragates.Status
	again = statusHoldEvent(aggragates.Active, at.Add(time.Minute), &fromActive)
	again.Trade.Logs = logs
	again.Trade.PositionType, again.Trade.PositionPrice = "trailingBuy", 91
	held, err := SaveHoldLog(again, "trailingBuy", hold)
	if !errors.Is(err, events.ErrTradeHeld) || errors.Is(err, events.ErrHoldNotPersisted) {
		t.Fatalf("collapsed from active: err %v, want plain ErrTradeHeld", err)
	}
	if len(fromActive) != 1 || fromActive[0] != aggragates.Hold || len(held.Trade.Logs) != len(logs) || !held.Params.PreventInfoLog {
		t.Fatalf("collapsed from active: published %v with %d rows, want hold alone and no row", fromActive, len(held.Trade.Logs))
	}
	if held.Trade.PositionType != again.Params.OldPosition || held.Trade.PositionPrice != again.Params.OldPositionPrice {
		t.Fatalf("collapsed from active: the position and price are restored, got %s at %v", held.Trade.PositionType, held.Trade.PositionPrice)
	}
}

// The dedupe reads the key and the message together: the same text under another key is a new row.
func TestSaveHoldLogDedupesOnTheKeyAndTheMessage(t *testing.T) {
	at := time.Date(2026, time.October, 27, 12, 0, 0, 0, time.UTC)
	var published []aggragates.Status
	event, _ := SaveHoldLog(statusHoldEvent(aggragates.Active, at, &published), "trailingBuy", gates.Hold{Reason: holdReason, Key: aggragates.LogKeyHoldDepthSpacing})
	event.Trade.PositionType = "trailingBuy"
	event.Timestamp = at.Add(time.Minute).UnixMilli()
	event, _ = SaveHoldLog(event, "trailingBuy", gates.Hold{Reason: holdReason, Key: aggragates.LogKeyHoldPattern})
	if got := holdMessages(event.Trade); len(got) != 2 {
		t.Fatalf("the same text under another key must write its own row, got %v", got)
	}
}
