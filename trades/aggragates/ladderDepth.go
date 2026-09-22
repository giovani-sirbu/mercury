package aggragates

// LadderDepth is one parent trade of a wallet as the depth-priority gate sees
// it: how many entries its ladder has already filled, and how many entries
// the settings row of its next fill allows it. Nothing else about the trade
// takes part in that decision.
//
// The engines own the wallet view — sisyphus builds it from its own memory,
// hermes fetches it from agora, which reads it out of the database — so the
// json tags below are a cross-service contract and move together.
type LadderDepth struct {
	TradeID  uint   `json:"tradeId"`
	Symbol   string `json:"symbol"`
	Depth    int    `json:"depth"`
	MaxDepth int    `json:"maxDepth"`
}
