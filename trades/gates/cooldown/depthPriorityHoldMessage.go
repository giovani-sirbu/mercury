package cooldown

import "fmt"

// depthPriorityHoldMessage says which ladder the wallet is in front of, and
// what it is waiting on: a ladder with depths left is keeping what it still
// needs, while a full one is keeping nothing and is simply not done with the
// wallet until it closes. An operator reading the second row knows no further
// entry will free the funds — only the close will. It is formatted from the
// event that goes beside the row, so the two cannot disagree.
//
// The message must stay byte-identical for as long as the hold stands:
// gates.SaveHoldLog deduplicates on the full string, so anything that moves
// tick by tick — the balance above all — would write a row per tick. Both
// depths are frozen while the hold stands: a held entry is precisely one that
// has not filled, and a priority that is keeping the wallet for its own next
// entry has not placed it either.
func depthPriorityHoldMessage(data DepthPriorityEvent) string {
	if data.PriorityDepth >= data.PriorityMaxDepth {
		return fmt.Sprintf(
			DepthPriorityHoldMarker+", %s at depth %d of %d holds the wallet until it closes, this ladder waits at depth %d of %d",
			data.PrioritySymbol, data.PriorityDepth, data.PriorityMaxDepth, data.Depth, data.MaxDepth,
		)
	}

	return fmt.Sprintf(
		DepthPriorityHoldMarker+", %s at depth %d of %d keeps the wallet for its remaining depths, this ladder waits at depth %d of %d",
		data.PrioritySymbol, data.PriorityDepth, data.PriorityMaxDepth, data.Depth, data.MaxDepth,
	)
}
