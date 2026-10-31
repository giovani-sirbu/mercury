package earlytakeprofit

var (
	// Enabled switches the early take profit: its watch (earlyTakeProfitWatched), its latch and
	// the take profit a latched ladder reads from its position price (TakeProfitPercentage).
	Enabled = true
	// ArmDepth is the fewest filled entries a long spot ladder holds before the latch watches it
	// (earlyTakeProfitWatched).
	ArmDepth = 1
	// QuietDecline switches the latch on the quiet decline's reading, on the tick's row and on a
	// tick with none (latchEarlyTakeProfit), whatever quietdecline.Enabled says.
	QuietDecline = true
	// SwingDecline switches the latch on the swing decline's reading, on the tick's row and on a
	// tick with none (latchEarlyTakeProfit), whatever swingdecline.Enabled says.
	SwingDecline = false
	// LinearDecline switches the latch on the linear decline's line (linearDeclineReads), whatever
	// lineardecline.Enabled says.
	LinearDecline = true
	// FlatMarket switches the latch on the flat market's latch reading, the range on its bars alone
	// and the line under the trend (earlyTakeProfitFlatMarketReads), whatever flatmarket.Enabled says.
	FlatMarket = true
	// FreeFallPrevention switches the latch on the free fall prevention's reading, the last closed
	// bar or the window's share (earlyTakeProfitFreeFallPreventionReads), whatever
	// freefallprevention.Enabled says.
	FreeFallPrevention = true
	// DepthPriority switches the latch on a cooldown depth priority hold of the ladder
	// (depthPriorityLatch), whatever PauseWhenActiveDepthPriority says.
	DepthPriority = true
	// DynamicParams switches the latch on a ladder that opened raised under the DynamicParams flag
	// (dynamicParamsLatch), held or not; the raise itself stays with package dynamicparams.
	DynamicParams = false
)
