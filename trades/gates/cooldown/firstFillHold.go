// Package cooldown is the Cooldown flag's two gates, one on each side of the
// first fill: the first-fill gate served by sophos /markers (FirstFillHold,
// capped by Expired) and depth spacing, the gate that keeps a ladder from
// cascading through every depth in one drop (DepthSpacingHoldReason).
package cooldown

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// FirstFillHold is the Cooldown flag's whole first-fill gate: the first fill
// waits for a cheap 1h location plus a 15m turn, served by sophos /markers.
// A missing verdict fails open. After the first fill this gate is inert.
func FirstFillHold(inverse bool, cool aggragates.CoolDownIndicators) string {
	if !cool.HasFirstFillVerdict {
		return ""
	}
	if inverse {
		if !cool.AllowShortEntry {
			return "cooldown: first fill expensive (inverse)"
		}
		return ""
	}
	if !cool.AllowLongEntry {
		return "cooldown: first fill expensive"
	}
	return ""
}
