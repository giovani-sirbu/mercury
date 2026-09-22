package aggragates

// LadderDepth is one parent trade of a wallet as the depth-priority gate sees
// it: how many entries its ladder has already filled, how many entries the
// settings row of its next fill allows it, and what the entries it has left
// still cost. Nothing else about the trade takes part in that decision.
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
	TradeID  uint   `json:"tradeId"`
	Symbol   string `json:"symbol"`
	Depth    int    `json:"depth"`
	MaxDepth int    `json:"maxDepth"`
	// Asset is what this ladder's next entry SPENDS: the quote side of the
	// pair for a long ladder, the base side for an inverse one. Ladders are
	// only ever compared against the ones spending the same asset, since two
	// wallets' worth of different currencies never compete.
	Asset string `json:"asset"`
	// RemainingCost is what the entries from this ladder's next depth through
	// its last configured one cost, counted in Asset at its own last position
	// price. Zero for a ladder that has filled nothing, carries no settings
	// row or has no configured ceiling — none of those can name an amount the
	// wallet would have to be kept for.
	RemainingCost float64 `json:"remainingCost"`
}
