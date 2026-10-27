package aggragates

import (
	"sync"
	"sync/atomic"
)

// eventMemoLimit bounds one memo: past it the memo starts over, so a process
// that decodes events for months never grows without end.
const eventMemoLimit = 1 << 17

// memoizeEvents is whether the memos remember at all. Off, Decode is
// DecodeData: a live engine reloads its trades, so every document it reads is
// fresh, and fixtures reuse ids across tests. A replay turns it on at start.
var memoizeEvents atomic.Bool

// MemoizeEvents turns the event memos on or off for the process.
func MemoizeEvents(on bool) {
	memoizeEvents.Store(on)
}

// EventMemo remembers one gate's decoded event documents by stored event, so a
// tape of millions of prints decodes each event once instead of on every print.
// Only stored events (a non-zero id) are remembered, and only while
// MemoizeEvents is on: a stored document never changes. The decoded value is
// shared between readers and is read-only.
type EventMemo[T any] struct {
	entries sync.Map
	size    atomic.Int64
}

// memoKey is the stored id with the document's size, so an id reused with
// another document (a fixture) never answers for the first one.
type memoKey struct {
	id   uint
	size int
}

type memoEntry[T any] struct {
	value T
	err   error
}

// Decode is DecodeData into a fresh T, remembered for a stored event.
func (memo *EventMemo[T]) Decode(event TradesStrategyEvents) (T, error) {
	var value T

	if event.ID == 0 || !memoizeEvents.Load() {
		return value, event.DecodeData(&value)
	}

	key := memoKey{id: event.ID, size: len(event.Data)}

	if entry, ok := memo.entries.Load(key); ok {
		cached := entry.(memoEntry[T])

		return cached.value, cached.err
	}

	err := event.DecodeData(&value)

	if memo.size.Add(1) > eventMemoLimit {
		memo.entries.Clear()
		memo.size.Store(1)
	}

	memo.entries.Store(key, memoEntry[T]{value: value, err: err})

	return value, err
}
