package depthpriority

import (
	"fmt"

	"github.com/giovani-sirbu/mercury/strategies/gates/cooldown"
)

// depthPriorityHoldMessage is the hold row's text, formatted from the event beside it: a priority
// at or past its costed ceiling (DepthOffset) holds the wallet until it closes, one with costed
// depths left keeps it for them. It stays byte-stable while the hold stands (tradelog.SaveHoldLog
// deduplicates on it), so it names no balance.
func depthPriorityHoldMessage(data cooldown.DepthPriorityEvent) string {
	if data.PriorityDepth >= data.PriorityMaxDepth-depthOffset {
		return fmt.Sprintf(
			cooldown.DepthPriorityHoldMarker+", %s at depth %d of %d holds the wallet until it closes, this ladder waits at depth %d of %d",
			data.PrioritySymbol, data.PriorityDepth, data.PriorityMaxDepth, data.Depth, data.MaxDepth,
		)
	}

	return fmt.Sprintf(
		cooldown.DepthPriorityHoldMarker+", %s at depth %d of %d keeps the wallet for its remaining depths, this ladder waits at depth %d of %d",
		data.PrioritySymbol, data.PriorityDepth, data.PriorityMaxDepth, data.Depth, data.MaxDepth,
	)
}
