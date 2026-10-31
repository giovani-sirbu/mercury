package smartsell

// forced is the answer when a rule sells: the proposal replaced by the sale's terminal (sell or
// takeLoss, saleTerminal), Reason naming the rule.
func forced(result Result, reason, terminal string) Result {
	result.Position = terminal
	result.Reason = reason
	return result
}

// activated is the answer when the shared action activates the sale or re-anchors it: the proposal
// replaced by state (takeProfit or trailingBuy), Activated telling the engines to run
// ActivationActions; nothing is sold.
func activated(result Result, state string) Result {
	result.Position = state
	result.Activated = true
	return result
}

// held is the answer when the shared action holds a committed ladder between its bounds or gapped
// under the anchor, or a reached trigger awaiting confirmation: no proposal, no chain, rows kept.
func held(result Result) Result {
	result.Position = ""
	return result
}
