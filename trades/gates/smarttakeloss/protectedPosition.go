package smarttakeloss

// protectedPosition reports whether the ladder has already decided something
// this overlay must never replace.
//
// Smart take loss forces an exit INSTEAD OF committing more capital, so the
// only positions it may override are the ones that would add or re-arm an
// add: buy, stopLoss and their re-arms. Everything below is already a
// decision the ladder made about closing, and overriding it changes what the
// chain does:
//
//   - "sell", "takeProfit", "update_takeProfit" and "sellParent" run hasProfit,
//     which refuses to close below the minimum profit. Forcing "sellLoss" over
//     them swaps that gate for acceptLoss, which accepts a negative result —
//     a close the ladder priced as profitable would be sold below break even.
//   - "impasse" is not a close but a chain: createChildrenTrades, then
//     parentTradeHasProfit, then sellAll. Replacing it leaves the parent with
//     no children to sell.
//   - "sellLoss" is already this overlay's own exit; re-forcing it is a no-op
//     at best.
//
// Apply asks it of the proposal AND of the trade's own state (both through
// gates.PositionType, so a force-trailing take profit reads as the take
// profit it re-arms), which keeps a trailing take profit or a resting
// sellLoss limit from being replaced on the ticks where the ladder proposes
// nothing.
func protectedPosition(position string) bool {
	switch position {
	case "sell", "takeProfit", "update_takeProfit", "sellParent", "impasse", "sellLoss":
		return true
	}

	return false
}
