package cooldown

import (
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The first-fill lens may hold a new trade for at most MaxHold; past
// that the engines skip the /markers fetch and the entry proceeds. Unknown
// clocks never expire, so a trade without CreatedAt (live-testing memory
// trades) keeps the lens exactly as before.
func TestCooldownExpiredHonoursTheEightHourHorizon(t *testing.T) {
	created := time.Date(2025, time.October, 10, 12, 0, 0, 0, time.UTC)
	trade := aggragates.Trades{CreatedAt: created}

	if Expired(trade, created) {
		t.Fatal("a trade created this tick has not expired")
	}
	if Expired(trade, created.Add(MaxHold-time.Minute)) {
		t.Fatal("one minute short of the horizon must still hold")
	}
	if !Expired(trade, created.Add(MaxHold)) {
		t.Fatal("exactly the horizon must expire")
	}
	if !Expired(trade, created.Add(3*24*time.Hour)) {
		t.Fatal("days past the horizon must expire")
	}

	local := time.FixedZone("UTC+3", 3*60*60)
	if !Expired(aggragates.Trades{CreatedAt: created.In(local)}, created.Add(MaxHold).In(local)) {
		t.Fatal("the horizon must be compared on the UTC instant, whatever zone the clocks carry")
	}
}

func TestCooldownExpiredNeverFiresOnUnknownClocks(t *testing.T) {
	now := time.Date(2025, time.October, 11, 12, 0, 0, 0, time.UTC)
	if Expired(aggragates.Trades{}, now) {
		t.Fatal("a trade without CreatedAt must never expire")
	}
	if Expired(aggragates.Trades{CreatedAt: now.Add(-48 * time.Hour)}, time.Time{}) {
		t.Fatal("a zero tick clock must never expire the lens")
	}
}
