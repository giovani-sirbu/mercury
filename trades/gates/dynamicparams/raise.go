package dynamicparams

// Raise is the increase a tier asks for: both bearish raises the percentage
// and the depths, mixed raises what MixedIncrease names, and the base tier
// raises nothing.
func Raise(tier Tier) Increase {
	return raiseFor(tier, MixedIncrease)
}

// raiseFor is Raise with the mixed tier's increase handed in, so every option
// MixedIncrease can take is testable without changing the constant.
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
