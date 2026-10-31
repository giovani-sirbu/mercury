package dynamicparams

// Raise is the increase a tier asks for: both bearish raises both, mixed raises what MixedIncrease
// names, the base tier nothing.
func Raise(tier Tier) Increase {
	return raiseFor(tier, MixedIncrease)
}

// raiseFor is Raise with the mixed tier's increase handed in, for the tests.
func raiseFor(tier Tier, mixed Increase) Increase {
	switch tier {
	case TierBothBearish:
		return IncreaseBoth
	case TierMixed:
		return mixed
	default:
		return IncreaseNone
	}
}
