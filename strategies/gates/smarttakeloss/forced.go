package smarttakeloss

// forced is the answer when a rule sells: the proposal replaced by sellLoss, Reason naming the rule.
func forced(result Result, reason string) Result {
	result.Position = "sellLoss"
	result.Reason = reason
	return result
}

// activated is the answer when the shared action activates or re-anchors the stopLoss: the proposal
// replaced by stopLoss, Activated telling the engines to run ActivationActions; nothing is sold.
func activated(result Result) Result {
	result.Position = "stopLoss"
	result.Activated = true
	return result
}

// held is the answer when the shared action holds a committed ladder between its bounds or gapped
// under the anchor, or a reached trigger awaiting confirmation: no proposal, no chain, rows kept.
func held(result Result) Result {
	result.Position = ""
	return result
}
