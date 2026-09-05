package actions

import (
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/patterns"
)

// capPatternAI is the shock verdict capitulation bypasses, carrying a scored
// chart pattern in the trade's own direction — the verdict patterns.HoldReason
// turns into "do not average down into a bounce".
func capPatternAI() aggragates.AIIndicators {
	ai := capShockAI(false)
	ai.PatternName = "double-bottom"
	ai.PatternDirection = "long"
	ai.PatternScore = patterns.HoldMinScore
	return ai
}

// Capitulation may bypass a REGIME hold only; patterns.HoldReason's own doc
// says a pattern is never bypassed by it, because "price should bounce" and
// "take one extra fill on the reclaim" contradict.
//
// Asking the families in a first-non-empty chain broke that silently: the
// regime shock answered first, so the pattern verdict was never computed, and
// the bypass released a trade nobody had asked the pattern gate about. The
// families are evaluated independently now, so the bypass releases the regime
// reason and the pattern still holds.
func TestShouldHoldCapitulationDoesNotReleaseAPatternHold(t *testing.T) {
	trade := capTrade(false, 3, 100, 79)
	trade.Strategy.Params.UsePatterns = true

	_, err := ShouldHold(capEvent(trade, capPatternAI(), capReclaimBucket(false)))

	if err == nil {
		t.Fatal("the capitulation bypass must not release a trade the pattern gate holds")
	}
	if !strings.Contains(err.Error(), "double-bottom") {
		t.Fatalf("the standing hold must be the pattern's, got %v", err)
	}
}

// The same tick without the flag is the case the bypass was written for, so
// the guard above must not have disabled capitulation itself.
func TestShouldHoldCapitulationStillReleasesWithoutAPatternVerdict(t *testing.T) {
	trade := capTrade(false, 3, 100, 79)
	trade.Strategy.Params.UsePatterns = true

	// UsePatterns on, but sophos served no pattern this tick.
	if _, err := ShouldHold(capEvent(trade, capShockAI(false), capReclaimBucket(false))); err != nil {
		t.Fatalf("with no pattern to hold, the reclaim must still take its fill, got %v", err)
	}
}
