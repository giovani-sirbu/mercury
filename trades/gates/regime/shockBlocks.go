package regime

// ShockBlocks reports whether a shock label vetoes new capital for the given
// trade direction. Shock carries its direction because a direction-blind
// label parks a trade through the whole fall it should have been adding into
// and releases it only once the move is over: a FALLING shock is the long
// side's knife and the inverse side's harvest, a RISING one the reverse. The
// directionless legacy "shock" (older sophos, stale cache, or the sophos knob
// turned off) blocks both — exactly the pre-directional behavior.
func ShockBlocks(label string, inverse bool) bool {
	switch label {
	case Shock:
		return true
	case ShockDown:
		return !inverse
	case ShockUp:
		return inverse
	}
	return false
}
