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

// bearishRead is the wire value of a bearish read (sophos' smctypes.Bearish); no other value counts.
const bearishRead = -1

// TierOf counts the bearish reads of a block: both, one or none; a block that is not Valid is TierBase.
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
