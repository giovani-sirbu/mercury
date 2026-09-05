package crashguard

import (
	"fmt"
	"strings"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// capitulationEpisode is the durable state of one capitulation episode,
// rebuilt from the trade logs on every tick.
type capitulationEpisode struct {
	tagged      bool
	allowed     bool
	freezing    bool
	consumed    bool
	hadCrash    bool
	taggedDepth int
}

func taggedMessage(lastFill float64, depth int, crash bool) string {
	msg := fmt.Sprintf("%s: last-fill %g depth %d", CapitulationTaggedPrefix, lastFill, depth)
	if crash {
		msg += " crash-active"
	}
	return msg
}

func allowedMessage(lastFill float64, depth int) string {
	return fmt.Sprintf("%s: last-fill %g depth %d", CapitulationAllowedPrefix, lastFill, depth)
}

func rebuildCapitulationEpisode(trade aggragates.Trades) capitulationEpisode {
	var ep capitulationEpisode
	for _, entry := range trade.Logs {
		msg := entry.Message
		switch {
		case strings.HasPrefix(msg, CapitulationFreezeOffPrefix):
			ep = capitulationEpisode{}
		case strings.HasPrefix(msg, CapitulationTaggedPrefix):
			ep.tagged = true
			ep.taggedDepth, ep.hadCrash = parseCapitulationTagged(msg)
			if liveHadCrash(trade.ID) {
				ep.hadCrash = true
			}
		case strings.HasPrefix(msg, CapitulationAllowedPrefix):
			ep.allowed = true
		case strings.HasPrefix(msg, CapitulationFreezeOnPrefix):
			ep.freezing = true
			ep.consumed = true
		}
	}
	// The one capitulation add is consumed only once it was GRANTED and a
	// fill landed past the tagged depth. A tag whose reclaim never fired
	// granted nothing, so a later gate-approved fill must not freeze the
	// ladder behind a log line claiming "one add already taken".
	if ep.tagged && ep.allowed && !ep.consumed && ladder.CountFilledEntries(trade) > ep.taggedDepth {
		ep.consumed = true
	}
	return ep
}

func parseCapitulationTagged(msg string) (depth int, crash bool) {
	crash = strings.Contains(msg, "crash-active")
	if i := strings.LastIndex(msg, "depth "); i >= 0 {
		_, _ = fmt.Sscanf(msg[i:], "depth %d", &depth)
	}
	return depth, crash
}
