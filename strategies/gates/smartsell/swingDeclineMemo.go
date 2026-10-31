package smartsell

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
	"math"
	"sync"
	"time"

	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/swingdecline"
)

// swingDeclineMemoBound is the most entries the memo holds, one per ladder, depth and closed bar
// (ladderKey), far over an hour's asks; the oldest go first and a forgotten entry is worked out again.
const swingDeclineMemoBound = 1024

// swingMemo is the memo swingDeclineReads answers through, one for the process.
var swingMemo = newSwingDeclineMemo(swingDeclineMemoBound, swingdecline.TriggerAtLastBar)

// swingReadFunc is the reading whose answers the memo keeps:
// swingdecline.TriggerAtLastBar in the process, a counting wrapper of it in a test.
type swingReadFunc func(fills []series.Fill, bars series.Series, trend series.Trend) (swingdecline.Reading, bool)

// swingDeclineMemo remembers what the swing decline read for a ladder, so the prints of an hour
// cost a lookup: exact (an entry answers only inputs equal by value; ladderKey only finds it),
// immutable entries handed out as copies, a hard bound forgetting the oldest first, safe for concurrent use.
type swingDeclineMemo struct {
	read  swingReadFunc
	bound int

	mu      sync.RWMutex
	entries map[ladderKey]*swingDeclineEntry
	// order holds the keys in the order they were first remembered, the oldest at next once full.
	order []ladderKey
	next  int
}

// newSwingDeclineMemo is an empty memo of at most bound entries (at least one) answering with read.
func newSwingDeclineMemo(bound int, read swingReadFunc) *swingDeclineMemo {
	return &swingDeclineMemo{
		read:    read,
		bound:   max(bound, 1),
		entries: make(map[ladderKey]*swingDeclineEntry),
	}
}

// fillStamp is a fill's stamp to the nanosecond as a map key (a time.Time would split equal instants).
type fillStamp struct {
	sec  int64
	nsec int
}

func stampOf(at time.Time) fillStamp {
	return fillStamp{sec: at.Unix(), nsec: at.Nanosecond()}
}

// ladderKey finds an entry by the first fill's stamp and price, the newest fill's stamp, the number
// of fills and the series' last closed bar; whether the entry answers is swingDeclineEntry.answers'.
type ladderKey struct {
	firstAt    fillStamp
	firstPrice uint64
	newestAt   fillStamp
	fills      int
	lastBar    int64
}

func ladderKeyOf(fills []entryFill, bars series.Series) ladderKey {
	key := ladderKey{
		fills:   len(fills),
		lastBar: bars.LastBarOpen(),
	}
	if len(fills) == 0 {
		return key
	}
	first, newest := fills[0], fills[len(fills)-1]
	key.firstAt = stampOf(first.At)
	key.firstPrice = math.Float64bits(first.Price)
	key.newestAt = stampOf(newest.At)
	return key
}

// answer is the remembered reading when an entry answers the call, else the one read works out,
// which replaces the key's entry; worked out outside the lock.
func (memo *swingDeclineMemo) answer(fills []entryFill, bars series.Series, trend series.Trend) (swingdecline.Reading, bool) {
	key := ladderKeyOf(fills, bars)
	if entry := memo.recall(key); entry != nil && entry.answers(fills, bars, trend) {
		return cloneReading(entry.reading), entry.holds
	}
	reading, holds := memo.read(swingFills(fills), bars, trend)
	memo.remember(key, newSwingDeclineEntry(fills, bars, trend, reading, holds))
	return reading, holds
}

func (memo *swingDeclineMemo) recall(key ladderKey) *swingDeclineEntry {
	memo.mu.RLock()
	defer memo.mu.RUnlock()
	return memo.entries[key]
}

// remember keeps the entry under the key, replacing its old one or admitting the key as the newest.
func (memo *swingDeclineMemo) remember(key ladderKey, entry *swingDeclineEntry) {
	memo.mu.Lock()
	defer memo.mu.Unlock()
	if _, known := memo.entries[key]; !known {
		memo.admit(key)
	}
	memo.entries[key] = entry
}

// admit makes room for a key's first entry, forgetting the oldest at the bound; the caller holds the lock.
func (memo *swingDeclineMemo) admit(key ladderKey) {
	if len(memo.order) < memo.bound {
		memo.order = append(memo.order, key)
		return
	}
	delete(memo.entries, memo.order[memo.next])
	memo.order[memo.next] = key
	memo.next = (memo.next + 1) % memo.bound
}
