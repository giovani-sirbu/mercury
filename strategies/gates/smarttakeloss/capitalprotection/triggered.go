package capitalprotection

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// Triggered is whether the trend reading opens an episode on a tick: the SMC trend dashboard reads
// every timeframe against the ladder, bearish for a long one (CapitalProtectionSmcBearish) and
// bullish for an inverse one (CapitalProtectionSmcBullish); the parent asks it on a ladder at its
// last depth while no episode is open.
func Triggered(inverse bool, block aggragates.SmartTakeLossIndicators) bool {
	if inverse {
		return block.CapitalProtectionSmcBullish
	}
	return block.CapitalProtectionSmcBearish
}
