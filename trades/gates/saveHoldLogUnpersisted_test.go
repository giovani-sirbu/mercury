package gates

import (
	"errors"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// recordingHoldEvent is holdLogEvent with an updateTrade that records whether
// it ran, which is what tells the persisting hold path from the collapsed one.
func recordingHoldEvent(trade aggragates.Trades, at time.Time, ran *bool) events.Events {
	event := holdLogEvent(trade, at)
	event.Events = map[string]func(events.Events) (events.Events, error){
		"updateTrade": func(e events.Events) (events.Events, error) {
			*ran = true
			return testutil.NopUpdateTrade(e)
		},
	}
	return event
}

const holdReason = "cooldown: depths too close (depth 3, step 2), next add parked for 3h0m0s"

// The first hold of a reason writes its row and runs updateTrade, so whatever
// the caller acquired for this run is released downstream.
func TestSaveHoldLogPersistsTheFirstHold(t *testing.T) {
	ran := false
	at := time.Date(2026, time.September, 5, 12, 0, 0, 0, time.UTC)

	_, err := SaveHoldLog(recordingHoldEvent(testutil.NewHoldTrade("stopLoss", false), at, &ran), "stopLoss", holdReason)

	if !ran {
		t.Fatal("a hold that writes its row must run updateTrade")
	}
	if !errors.Is(err, events.ErrTradeHeld) {
		t.Fatalf("a hold stops the chain with ErrTradeHeld, got %v", err)
	}
	if errors.Is(err, events.ErrHoldNotPersisted) {
		t.Fatal("a hold that persisted must not be flagged as unpersisted")
	}
}

// The regression. A repeat of the same reason writes nothing and never reaches
// updateTrade. hermes holds a 24h trade lock whose only release is agora's
// update-trade consumer, so without this flag the second identical hold wedged
// the trade for the whole TTL: no take profit, no cut, no crash reaction.
func TestSaveHoldLogFlagsTheCollapsedHoldAsUnpersisted(t *testing.T) {
	at := time.Date(2026, time.September, 5, 12, 0, 0, 0, time.UTC)
	first := false
	event := recordingHoldEvent(testutil.NewHoldTrade("stopLoss", false), at, &first)
	event, _ = SaveHoldLog(event, "stopLoss", holdReason)
	if !first {
		t.Fatal("setup: the first hold must persist")
	}

	second := false
	event.Trade.PositionType = "stopLoss"
	repeat := recordingHoldEvent(event.Trade, at.Add(time.Minute), &second)

	_, err := SaveHoldLog(repeat, "stopLoss", holdReason)

	if second {
		t.Fatal("a collapsed hold must not run updateTrade")
	}
	if !errors.Is(err, events.ErrTradeHeld) {
		t.Fatalf("it is still a hold, got %v", err)
	}
	if !errors.Is(err, events.ErrHoldNotPersisted) {
		t.Fatalf("a hold that published nothing must say so, got %v", err)
	}
}

// Past the relog window the reason is written again, so that tick persists and
// the caller is owed nothing.
func TestSaveHoldLogPastTheRelogWindowIsPersistedAgain(t *testing.T) {
	at := time.Date(2026, time.September, 5, 12, 0, 0, 0, time.UTC)
	first := false
	event := recordingHoldEvent(testutil.NewHoldTrade("stopLoss", false), at, &first)
	event, _ = SaveHoldLog(event, "stopLoss", holdReason)

	later := false
	event.Trade.PositionType = "stopLoss"
	repeat := recordingHoldEvent(event.Trade, at.Add(holdRelogAfter+time.Minute), &later)

	_, err := SaveHoldLog(repeat, "stopLoss", holdReason)

	if !later {
		t.Fatal("past the relog window the reason is written again")
	}
	if errors.Is(err, events.ErrHoldNotPersisted) {
		t.Fatalf("that write persists, got %v", err)
	}
}
