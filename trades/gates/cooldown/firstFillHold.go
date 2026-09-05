// Package cooldown is the Cooldown flag's two gates, one on each side of the
// first fill: the first-fill gate served by sophos /markers (FirstFillHold,
// capped by Expired) and depth spacing, the gate that keeps a ladder from
// cascading through every depth in one drop (DepthSpacingHoldReason).
package cooldown

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// FirstFillHold is the Cooldown flag's whole first-fill gate: the first fill
// waits for a cheap 1h location plus a 15m turn, served by sophos /markers.
// A missing verdict fails open. After the first fill this gate is inert.
//
// `side` is aggragates.EntrySide, not the Inverse flag. sophos scores the two
// directions separately, and taking Inverse here judged every futures entry —
// short ones included — against AllowLongEntry, because a futures trade is
// never marked inverse.
//
// An empty side is no direction to judge, so nothing is held.
func FirstFillHold(side string, cool aggragates.CoolDownIndicators) string {
	if !cool.HasFirstFillVerdict {
		return ""
	}

	switch side {
	case aggragates.SideLong:
		if !cool.AllowLongEntry {
			return "cooldown: first fill expensive"
		}
	case aggragates.SideShort:
		if !cool.AllowShortEntry {
			return "cooldown: first fill expensive (inverse)"
		}
	}

	return ""
}
