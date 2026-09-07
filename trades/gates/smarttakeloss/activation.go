package smarttakeloss

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// activates is the "the price is at the bottom of the window" test, read on
// EVERY tick of an armed trade. Sophos counts, over its own window of closed
// daily bars, how many printed under a price — how many bars are to its LEFT
// — and serves the level where that count is the most the gate allows. A tick
// price at or under it activates the trade (at or over the mirror level, for
// an inverse ladder); the trade then keeps being watched, and the second
// reading — no bar to the left at all — is what later arms the tolerance
// exit.
//
// It is not tied to a fill: a ladder sitting in the dead zone between its next
// depth and its take profit activates on the tick the price arrives, where the
// old reading needed a fill inside the freshest bars to look at all. What it
// still cannot reach is a trade the exchange refused for funds — that one is
// Blocked, and no engine ticks it (agora serves Active trades only, and
// backtesting skips a blocked trade before this overlay), so the rule resumes
// from the trade's own rows once the block is released.
//
// The activation row is anchored to the newest fill all the same (Apply),
// because that is what the one permitted depth is counted from.
//
// A trade with no fill price, or a block without a verdict (older sophos, a
// failed pattern leg, a window shorter than sophos can count over), never
// activates: the cooldown's fail-open posture.
func activates(trade aggragates.Trades, st state, price float64, ai aggragates.SmartTakeLossIndicators) bool {
	if st.lastFill().Price <= 0 || price <= 0 || !ai.HasVerdict {
		return false
	}
	if trade.Inverse {
		return ai.HighWithBarsLeft > 0 && price >= ai.HighWithBarsLeft
	}
	return ai.LowWithBarsLeft > 0 && price <= ai.LowWithBarsLeft
}

// noBarsLeft is the second reading of the same window: NO bar of it printed
// under the tick price (over it, on an inverse ladder) — the price is the
// lowest the window has seen. It is what arms the tolerance exit on the last
// permitted depth; a block without a verdict or without the level arms
// nothing.
func noBarsLeft(trade aggragates.Trades, price float64, ai aggragates.SmartTakeLossIndicators) bool {
	if price <= 0 || !ai.HasVerdict {
		return false
	}
	if trade.Inverse {
		return ai.HighestHigh > 0 && price >= ai.HighestHigh
	}
	return ai.LowestLow > 0 && price <= ai.LowestLow
}
