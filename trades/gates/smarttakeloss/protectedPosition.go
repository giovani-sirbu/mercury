package smarttakeloss

// protectedPosition reports whether the ladder has already decided something
// this overlay must never replace.
//
// Smart take loss forces an exit INSTEAD OF committing more capital, so the
// only positions it may override are the ones that would add or re-arm an add:
// buy, stopLoss and their re-arms. Everything below is already a decision the
// ladder made about closing, and overriding it changes what the chain does:
//
//   - "sell", "takeProfit", "update_takeProfit" and "sellParent" run hasProfit,
//     which refuses to close below the minimum profit. Forcing "sellLoss" (or
//     an STL trail) over them swaps that gate for acceptLoss, which accepts a
//     negative result — so a close the ladder priced as profitable is either
//     sold below break even or deferred into a trail that can no longer sell
//     on weakness at all.
//   - "impasse" is not a close but a chain: createChildrenTrades, then
//     parentTradeHasProfit, then sellAll. Replacing it leaves the parent with
//     no children to sell.
//   - "sellLoss" is already this overlay's own exit; re-forcing it is a no-op
//     at best.
//
// Both override branches ask this, so the trigger and the stale cut protect
// the same set. The stale cut used to exempt "takeProfit" alone, which let it
// turn the other four into a below-break-even close.
func protectedPosition(position string) bool {
	switch position {
	case "sell", "takeProfit", "update_takeProfit", "sellParent", "impasse", "sellLoss":
		return true
	}

	return false
}
