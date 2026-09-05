package regime

// ShockBlocks reports whether a shock label vetoes new capital for the given
// trade direction. Shock carries its direction since trade 13421 sat out a
// -16% BTC crash behind a direction-blind "4h shock" and then entered at the
// bottom: a FALLING shock is the long side's knife and the inverse side's
// harvest, a RISING one the reverse. The directionless legacy "shock" (older
// sophos, stale cache, or the sophos knob turned off) blocks both — exactly
// the pre-directional behavior.
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
