package cooldown

import (
	"strconv"
	"strings"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// depthSpacingMarker is the head of the row DepthSpacingHoldReason writes; the
// depth follows it. Rows are matched by it anywhere in the message, because
// gates.SaveHoldLog frames the reason in front of it.
const depthSpacingMarker = "cooldown: depths too close (depth "

// depthSpacingActivations rebuilds, from the trade's own log rows, when the
// depth-spacing gate activated for each depth: the earliest stamp of a row
// carrying that depth. The rows are the only state, the way firstFillState
// rebuilds the first-fill gate, and they reach the gate on every engine.
//
// Later rows for the same depth are gates.SaveHoldLog re-logging the standing
// hold once its row ages out, so they are the same activation and never a new
// one. The step the row printed is not read: it is derived, and trusting it
// would let an old row overrule the rule as it stands. A row with no stamp or
// an unreadable depth carries nothing to measure and is skipped.
func depthSpacingActivations(logs []aggragates.TradesLogs) map[int]time.Time {
	activations := make(map[int]time.Time)
	for _, row := range logs {
		if row.CreatedAt.IsZero() {
			continue
		}
		_, tail, found := strings.Cut(row.Message, depthSpacingMarker)
		if !found {
			continue
		}
		digits, _, found := strings.Cut(tail, ",")
		if !found {
			continue
		}
		depth, err := strconv.Atoi(digits)
		if err != nil || depth < 1 {
			continue
		}
		at := row.CreatedAt.UTC()
		if first, seen := activations[depth]; !seen || at.Before(first) {
			activations[depth] = at
		}
	}
	return activations
}
