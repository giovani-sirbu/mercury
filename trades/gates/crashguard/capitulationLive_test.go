package crashguard

import (
	"github.com/giovani-sirbu/mercury/trades/gates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func capitulationOverrideEvent(tradeID uint, position string) events.Events {
	trade := testutil.NewHoldTrade(position, false)
	trade.ID = tradeID
	trade.ParentID = 0
	return events.Events{
		Trade:  trade,
		Params: aggragates.Params{OldPosition: "buy", OldPositionPrice: 100},
	}
}

// A force-trailing ratchet only re-anchors the rung; the live episode
// (hadCrash, quiet windows) must survive it. The gates see the ratchet as
// the rung it re-arms (gates.PositionType), the episode sees the raw name.
func TestCapitulationLiveSurvivesForceTrailingRatchet(t *testing.T) {
	const tradeID = uint(970001)
	clearCapitulationLive(tradeID)
	markCapitulationCrash(tradeID)

	for _, position := range []string{"forceTrailingStopLoss", "forceTrailingTakeProfit"} {
		event := capitulationOverrideEvent(tradeID, position)
		ApplyCapitulationOverride(event, gates.PositionType(position), aggragates.AIIndicators{}, "")
		if !liveHadCrash(tradeID) {
			t.Fatalf("%s must not clear the live capitulation episode", position)
		}
	}
	clearCapitulationLive(tradeID)
}

// Leaving the ladder for real still ends the live bookkeeping.
func TestCapitulationLiveClearsOnExit(t *testing.T) {
	const tradeID = uint(970002)
	for _, position := range []string{"takeProfit", "sell", "sellLoss"} {
		clearCapitulationLive(tradeID)
		markCapitulationCrash(tradeID)
		event := capitulationOverrideEvent(tradeID, position)
		ApplyCapitulationOverride(event, gates.PositionType(position), aggragates.AIIndicators{}, "")
		if liveHadCrash(tradeID) {
			t.Fatalf("%s must clear the live capitulation episode", position)
		}
	}
}
