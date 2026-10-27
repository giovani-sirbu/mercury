package smarttakeloss

// ActivationActions is the chain the engines run when Result.Activated (an activation from buy or
// a re-anchor of a committed stopLoss): it only moves the anchor to the tick price and the state to stopLoss.
func ActivationActions() []string {
	return []string{"updateTrade"}
}
