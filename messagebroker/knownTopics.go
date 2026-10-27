package messagebroker

import "sync"

// knownTopics is every topic this process consumed or produced, so the
// pending sampler can report 0 for a topic with no rows.
var (
	knownTopicsMu sync.RWMutex
	knownTopics   = make(map[string]struct{})
)

// registerTopic marks a prefixed topic as known.
func registerTopic(prefixedTopic string) {
	knownTopicsMu.Lock()
	knownTopics[prefixedTopic] = struct{}{}
	knownTopicsMu.Unlock()
}

// snapshotKnownTopics returns a copy of the known-topics set.
func snapshotKnownTopics() []string {
	knownTopicsMu.RLock()
	defer knownTopicsMu.RUnlock()
	out := make([]string, 0, len(knownTopics))
	for t := range knownTopics {
		out = append(out, t)
	}
	return out
}
