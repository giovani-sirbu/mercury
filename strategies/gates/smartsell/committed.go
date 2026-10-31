package smartsell

// committedIn is whether a gate's activation moved the ladder to position (its <gate>ActivatedIn) in
// an episode still open, which commits the sale there: the overlay then trails it, a takeProfit included.
func (st state) committedIn(position string) bool {
	return (st.slowDeclinePending && st.slowDeclineActivated && st.slowDeclineActivatedIn == position) ||
		(st.flatMarketActivated && st.flatMarketActivatedIn == position) ||
		(st.freeFallPreventionActivated && st.freeFallPreventionActivatedIn == position) ||
		(st.capitalProtectionActivated && st.capitalProtectionActivatedIn == position)
}
