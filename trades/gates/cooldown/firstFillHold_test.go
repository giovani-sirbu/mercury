package cooldown

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// sophos scores the two directions separately. Taking the Inverse flag here
// judged every futures entry against the cheap-LONG read, because a futures
// trade is never marked inverse.
func TestFirstFillHoldJudgesTheSideNotTheFlag(t *testing.T) {
	cheapShort := aggragates.CoolDownIndicators{
		HasFirstFillVerdict: true,
		AllowLongEntry:      false,
		AllowShortEntry:     true,
	}

	if reason := FirstFillHold(aggragates.SideShort, cheapShort); reason != "" {
		t.Errorf("a short entry at a cheap short location must open, got %q", reason)
	}
	if FirstFillHold(aggragates.SideLong, cheapShort) == "" {
		t.Error("a long entry at an expensive long location must be held")
	}
}

// A missing verdict fails open: sophos is allowed to be unreachable.
func TestFirstFillHoldFailsOpenWithoutAVerdict(t *testing.T) {
	none := aggragates.CoolDownIndicators{AllowLongEntry: false, AllowShortEntry: false}

	for _, side := range []string{aggragates.SideLong, aggragates.SideShort} {
		if reason := FirstFillHold(side, none); reason != "" {
			t.Errorf("no verdict must never hold %q, got %q", side, reason)
		}
	}
}

// No direction is nothing to judge.
func TestFirstFillHoldIsInertWithoutASide(t *testing.T) {
	expensive := aggragates.CoolDownIndicators{HasFirstFillVerdict: true}

	if reason := FirstFillHold("", expensive); reason != "" {
		t.Errorf("an unresolved side must not be held here, got %q", reason)
	}
}
