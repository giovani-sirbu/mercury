package smartsell

// ActivationActions is the chain the engines run when Result.Activated (an activation from buy or a
// re-anchor of a committed takeProfit or trailingBuy): it only moves the anchor to the tick price and
// the state to Result.Position.
func ActivationActions() []string {
	return []string{"updateTrade"}
}

// SaleActions is the chain the engines run on a committed sale (Result.Reason set), whichever its
// terminal (sell or takeLoss): acceptLoss books the result, with no minimum profit to meet.
func SaleActions() []string {
	return []string{"cancelPendingOrder", "acceptLoss", "sell", "updateTrade"}
}
