package dynamicparams

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// Tier is how many of the two reads are bearish.
type Tier int

const (
	// TierBase is neither read bearish, or a timeframe sophos did not read.
	TierBase Tier = iota
	// TierMixed is exactly one of the two reads bearish.
	TierMixed
	// TierBothBearish is both reads bearish.
	TierBothBearish
)

// bearishRead is the value a bearish read carries on the wire, sophos'
// smctypes.Bearish. No other value is bearish.
const bearishRead = -1

// TierOf counts the bearish reads of a block: both is TierBothBearish, one is
// TierMixed, none is TierBase. A read counts only when it is exactly
// bearishRead, so a neutral read counts as a bullish one does — not at all. A
// block that is not Valid is TierBase whatever its reads say: sophos did not
// read the timeframe.
func TierOf(reads aggragates.DynamicParamsIndicators) Tier {
	if !reads.Valid {
		return TierBase
	}

	guppyBearish := reads.Guppy == bearishRead
	bmsbBearish := reads.BMSB == bearishRead

	switch {
	case guppyBearish && bmsbBearish:
		return TierBothBearish
	case guppyBearish || bmsbBearish:
		return TierMixed
	default:
		return TierBase
	}
}
