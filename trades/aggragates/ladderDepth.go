package aggragates

// LadderDepth is one parent trade of a wallet as the depth-priority gate sees
// it: how many entries its ladder has filled, how many the row of its next
// fill allows, and what the entries it has left still cost, in the asset it
// spends, priced from its position and from its last fill. Everything is
// measured on the rows the ladder trades (raised when its opened event
// raises them), built through ladder.DepthOfOffset on every surface. The
// json tags are a cross-service contract: hermes fetches the view from agora.
type LadderDepth struct {
	TradeID uint   `json:"tradeId"`
	Symbol  string `json:"symbol"`
	Depth   int    `json:"depth"`
	// MaxDepth is the ladder's ceiling, the floored depths of the row its next
	// fill reads; zero when no ceiling is known.
	MaxDepth int `json:"maxDepth"`
	// Asset is what the next entry SPENDS: quote for a long ladder, base for
	// an inverse one; only ladders spending the same asset compete.
	Asset string `json:"asset"`
	// RemainingCost is what the entries from the next depth through the costed
	// ceiling (MaxDepth less the gate's DepthOffset) cost in Asset, priced from
	// the position; zero for a ladder that cannot name an amount.
	RemainingCost float64 `json:"remainingCost"`
	// PlannedRemainingCost is the same depths priced from the LAST FILL, the
	// gate's tie-break between two ladders at the same depth: the last fill
	// moves only when a depth fills. Equal to RemainingCost on an inverse
	// ladder, zero wherever RemainingCost is.
	PlannedRemainingCost float64 `json:"plannedRemainingCost"`
}
