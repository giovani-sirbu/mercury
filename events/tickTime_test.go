package events

import (
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func TestTickTimeReadsTimestampInAnyUnit(t *testing.T) {
	want := time.UnixMilli(1_700_000_000_000).UTC()
	for _, ts := range []int64{1_700_000_000, 1_700_000_000_000, 1_700_000_000_000_000, 1_700_000_000_000_000_000} {
		event := Events{Timestamp: ts, Trade: aggragates.Trades{UpdatedAt: want.Add(time.Hour)}}
		if got := event.TickTime(); !got.Equal(want) {
			t.Errorf("Timestamp %d: TickTime() = %s, want %s", ts, got, want)
		}
		if got := event.TickMillis(); got != 1_700_000_000_000 {
			t.Errorf("Timestamp %d: TickMillis() = %d, want 1700000000000", ts, got)
		}
	}
}

func TestTickTimeFallsBackToTheTradeStamp(t *testing.T) {
	updatedAt := time.Date(2021, 7, 26, 13, 41, 8, 0, time.UTC)
	event := Events{Trade: aggragates.Trades{UpdatedAt: updatedAt}}

	if got := event.TickTime(); !got.Equal(updatedAt) {
		t.Errorf("TickTime() = %s, want %s", got, updatedAt)
	}
	if got := event.TickMillis(); got != updatedAt.UnixMilli() {
		t.Errorf("TickMillis() = %d, want %d", got, updatedAt.UnixMilli())
	}
}

func TestTickTimeIsZeroWithoutAnyClock(t *testing.T) {
	event := Events{}

	if got := event.TickTime(); !got.IsZero() {
		t.Errorf("TickTime() = %s, want zero", got)
	}
	if got := event.TickMillis(); got != 0 {
		t.Errorf("TickMillis() = %d, want 0", got)
	}
}
