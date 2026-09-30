package aggragates

// LadderDepth is one parent trade of a wallet as the depth-priority gate sees
// it: how many entries its ladder has already filled, how many entries the
// settings row of its next fill allows it, and what the entries it has left
// still cost — priced from where its position stands and from its last fill.
// Nothing else about the trade takes part in that decision.
//
// The row is the one the ladder TRADES: a ladder that opened with a raise
// reads its raised rows, so its ceiling and its costs are the raised ones and
// it is full only there. The raise travels in the trade's own logs, which is
// why every surface that builds the view from a trade — ladder.DepthOf —
// reads it alike.
//
// The cost travels with the depths because the gate is a RESERVE, not a
// ranking: a ladder in the view has to say what it still needs and in which
// asset, or the ladders waiting for it would be waiting on a depth alone and
// would know nothing about the money the wallet actually holds.
//
// The engines own the wallet view — sisyphus builds it from its own memory,
// hermes fetches it from agora, which reads it out of the database — so the
// json tags below are a cross-service contract and move together.
type LadderDepth struct {
	TradeID uint   `json:"tradeId"`
	Symbol  string `json:"symbol"`
	Depth   int    `json:"depth"`
	// MaxDepth is the ladder's own ceiling: the depths of the row its next
	// fill reads, floored, on the rows it trades — raised when its opened row
	// raises them. Zero when no ceiling is known.
	MaxDepth int `json:"maxDepth"`
	// Asset is what this ladder's next entry SPENDS: the quote side of the
	// pair for a long ladder, the base side for an inverse one. Ladders are
	// only ever compared against the ones spending the same asset, since two
	// wallets' worth of different currencies never compete.
	Asset string `json:"asset"`
	// RemainingCost is what the entries from this ladder's next depth through
	// its own ceiling (MaxDepth) cost, counted in Asset at its own last position
	// price. Zero for a ladder that has filled nothing, carries no settings
	// row or has no configured ceiling — none of those can name an amount the
	// wallet would have to be kept for.
	RemainingCost float64 `json:"remainingCost"`
	// PlannedRemainingCost is the same remaining depths priced down the grid
	// from the ladder's LAST FILL instead of its position price. It is the
	// gate's tie-break between two ladders at the same depth, and the anchor
	// is why: the position price moves when a ladder arms and while it trails,
	// and on a blocked ladder sits wherever the block caught it, so
	// RemainingCost can change with nothing about the ladder changed. The last
	// fill moves only when a depth fills. Equal to RemainingCost on an inverse
	// ladder, whose depths are counted in base units, and zero wherever
	// RemainingCost is zero.
	PlannedRemainingCost float64 `json:"plannedRemainingCost"`
}
