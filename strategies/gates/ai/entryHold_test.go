package ai

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The veto reads the side the entry would take. A futures short is the case
// that was broken: the verdict that names the direction was also read as the
// reason to refuse it, so futures could only ever open long.
func TestEntryHoldJudgesTheSideNotTheFlag(t *testing.T) {
	short := aggragates.AIIndicators{AIAction: aggragates.ActionShort}
	long := aggragates.AIIndicators{AIAction: aggragates.ActionLong}

	if reason := EntryHold(aggragates.SideShort, short); reason != "" {
		t.Errorf("a short entry on a SHORT verdict must open, got %q", reason)
	}
	if reason := EntryHold(aggragates.SideLong, long); reason != "" {
		t.Errorf("a long entry on a LONG verdict must open, got %q", reason)
	}
	if EntryHold(aggragates.SideLong, short) == "" {
		t.Error("a long entry into a SHORT verdict must be held")
	}
	if EntryHold(aggragates.SideShort, long) == "" {
		t.Error("a short entry into a LONG verdict must be held")
	}
}

// The standalone market flags still veto, independently of the action.
func TestEntryHoldStillReadsTheMarketFlags(t *testing.T) {
	if EntryHold(aggragates.SideLong, aggragates.AIIndicators{AIMarketBearish: true}) == "" {
		t.Error("a long entry must be held on a bearish market")
	}
	if EntryHold(aggragates.SideShort, aggragates.AIIndicators{AIMarketBullish: true}) == "" {
		t.Error("a short entry must be held on a bullish market")
	}
}

// No direction is nothing to judge. The engines refuse such an entry before
// the chain is built, so the gate must not invent a hold for it.
func TestEntryHoldIsInertWithoutASide(t *testing.T) {
	verdict := aggragates.AIIndicators{AIMarketBearish: true, AIMarketBullish: true}
	if reason := EntryHold("", verdict); reason != "" {
		t.Errorf("an unresolved side must not be held here, got %q", reason)
	}
}
