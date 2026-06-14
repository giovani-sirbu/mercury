package messagebroker

import "sync"

// knownTopics tracks every topic this process has seen at runtime, fed by
// runConsumer (subscriber side) and produceWithCorrelation (producer side).
// Used by the pending sampler to emit messagebroker_pending_messages=0 for
// topics that have no rows in message_queue yet — without this, a brand-new
// or fully-drained topic never appears in /metrics and dashboards show
// "No data" instead of a healthy 0.
//
// The set is process-local and best-effort: a restart wipes it. That's fine
// because the same Consumer/Produce calls re-populate it within seconds of
// boot. Sized for the realistic ~10 topics across HERMATIC; the
// sync.RWMutex is hot only at startup and on producer/consumer registration.
var (
	knownTopicsMu sync.RWMutex
	knownTopics   = make(map[string]struct{})
)

// registerTopic marks a topic as known. Idempotent. The topic should already
// carry any TOPIC_PREFIX (i.e. callers pass prefixedTopic) so the sampler
// can match it 1:1 against the value stored in message_queue.topic.
func registerTopic(prefixedTopic string) {
	knownTopicsMu.Lock()
	knownTopics[prefixedTopic] = struct{}{}
	knownTopicsMu.Unlock()
}

// snapshotKnownTopics returns a copy of the current known-topics set. Used
// by the pending sampler under a read lock so it can iterate without
// blocking new registrations.
func snapshotKnownTopics() []string {
	knownTopicsMu.RLock()
	defer knownTopicsMu.RUnlock()
	out := make([]string, 0, len(knownTopics))
	for t := range knownTopics {
		out = append(out, t)
	}
	return out
}
