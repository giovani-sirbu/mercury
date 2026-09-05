package actions

import (
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/regime"
)

// ridingUptrend is the verdict the profit hold exists for: 15m persisting up
// under a 4h that agrees.
func ridingUptrend() aggragates.AIIndicators {
	return aggragates.AIIndicators{
		HasRegimeVerdict: true,
		Regimes:          map[string]string{"15m": regime.UpPersist, "4h": regime.UpPersist},
	}
}

// The hold belongs to the tick that ARMS the exit. There the trade is still in
// position, so deferring keeps it in the trend — which is what the reason says.
func TestShouldHoldProfitHoldDefersTheArming(t *testing.T) {
	event := profitHoldDeepEvent(regime.ProfitHoldMinDepth, 8, ridingUptrend())
	event.Params.OldPosition = "buy"

	held, err := ShouldHold(event)
	if err == nil {
		t.Fatal("arming a profitable exit under a persisting uptrend must be deferred")
	}
	if !strings.Contains(held.Trade.Logs[0].Message, "rides the trend") {
		t.Fatalf("unexpected hold reason %q", held.Trade.Logs[0].Message)
	}
}

// The trailing re-anchor is the opposite. The exit is already armed; holding
// only freezes its anchor while the trend runs, so the eventual sell lands
// lower than the trail would have given. The engine's own ratchet
// (`percentage > trailingTakeProfit ? 'update_takeProfit'`) IS how a trade
// rides a trend, and this gate must not block it.
func TestShouldHoldProfitHoldNeverFreezesTheRatchet(t *testing.T) {
	for _, from := range []string{"takeProfit", "forceTrailingTakeProfit"} {
		event := profitHoldDeepEvent(regime.ProfitHoldMinDepth, 8, ridingUptrend())
		event.Params.OldPosition = from

		if _, err := ShouldHold(event); err != nil {
			t.Errorf("a re-anchor from %q must not be held, got %v", from, err)
		}
	}
}
