package crashguard_test

import (
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/crashguard"
)

// The ARMED row carries the verdict's own reasons, so the row explains itself
// without a second lookup.
func TestTransitionMessageArmedCarriesTheSlowDeclineReasons(t *testing.T) {
	message := crashguard.TransitionMessage(aggragates.AIIndicators{
		SlowDecline:        true,
		FreeFall:           true,
		SlowDeclineReasons: []string{"leg down 14.2% from its high close", "leg 61h long", "no support below the 30-day low"},
	})

	if !strings.HasPrefix(message, crashguard.ArmedPrefix) {
		t.Fatalf("armed row must open with %q: %s", crashguard.ArmedPrefix, message)
	}
	for _, want := range []string{"slow decline", "leg down 14.2%", "leg 61h long", "no support below"} {
		if !strings.Contains(message, want) {
			t.Errorf("armed row missing %q: %s", want, message)
		}
	}
}

func TestTransitionMessageArmedWithoutReasonsHasNoSeparator(t *testing.T) {
	message := crashguard.TransitionMessage(aggragates.AIIndicators{SlowDecline: true})
	if !strings.HasPrefix(message, crashguard.ArmedPrefix) || strings.Contains(message, "|") {
		t.Fatalf("an armed row without reasons must not end in an empty list: %s", message)
	}
}

// The crash score has no say in the row: the edge is the slow-decline
// verdict's, whatever the score reads.
func TestTransitionMessageClearedFollowsTheSlowDeclineVerdict(t *testing.T) {
	message := crashguard.TransitionMessage(aggragates.AIIndicators{CrashActive: true, CrashScore: 90})

	if !strings.HasPrefix(message, crashguard.ClearedPrefix) || !strings.Contains(message, "normal flow") {
		t.Fatalf("cleared row must state the return to normal flow: %s", message)
	}
	if strings.Contains(message, crashguard.ArmedPrefix) {
		t.Fatalf("cleared row must not read as armed: %s", message)
	}
}

// The row follows the slow-decline flag alone: free fall without a slow
// decline holds nothing and reads CLEARED, and a CLEARED row never carries
// reasons a payload may still send along.
func TestTransitionMessageFreeFallAloneReadsCleared(t *testing.T) {
	message := crashguard.TransitionMessage(aggragates.AIIndicators{
		FreeFall:           true,
		SlowDeclineReasons: []string{"leg 61h long"},
	})
	if !strings.HasPrefix(message, crashguard.ClearedPrefix) {
		t.Fatalf("free fall alone must read CLEARED: %s", message)
	}
	if strings.Contains(message, "leg 61h long") || strings.Contains(message, "|") {
		t.Fatalf("a CLEARED row must not carry reasons: %s", message)
	}
}
