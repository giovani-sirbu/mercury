package smarttakeloss

import (
	"fmt"

	"github.com/giovani-sirbu/mercury/strategies/gates/cooldown"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// depthPriorityHeld is whether a depth priority pauses the gates on the ladder:
// PauseWhenActiveDepthPriority on and a hold standing (depthPriorityHold).
func depthPriorityHeld(trade aggragates.Trades, newest entryFill) bool {
	if !pauseWhenActiveDepthPriority {
		return false
	}
	_, holds := depthPriorityHold(trade, newest)
	return holds
}

// depthPriorityHold is the newest cooldown.GateDepthPriority event stamped strictly after the newest
// fill's history stamp; only the next fill ends the hold. An unstamped event or fill holds nothing.
func depthPriorityHold(trade aggragates.Trades, newest entryFill) (aggragates.TradesStrategyEvents, bool) {
	if newest.At.IsZero() {
		return aggragates.TradesStrategyEvents{}, false
	}
	for index := len(trade.StrategyEvents) - 1; index >= 0; index-- {
		event := trade.StrategyEvents[index]
		if event.Param == aggragates.StrategyParamCooldown && event.Gate == cooldown.GateDepthPriority && event.CreatedAt.After(newest.At) {
			return event, true
		}
	}
	return aggragates.TradesStrategyEvents{}, false
}

// depthPriorityLatch is the finding a standing depth priority hold latches the early take profit
// on, paused or not: the hold's label with the ladder in front and this one.
func depthPriorityLatch(trade aggragates.Trades, st state) (Finding, bool) {
	if !earlyTakeProfitDepthPriority {
		return Finding{}, false
	}
	hold, holds := depthPriorityHold(trade, st.lastFill())
	if !holds {
		return Finding{}, false
	}
	return Finding{Feature: FeatureDepthPriority, Reasons: depthPriorityReasons(hold)}, true
}

// depthPriorityReasons names the ladder in front and this one, each at its depth of its ceiling;
// nothing for a document that does not read.
func depthPriorityReasons(hold aggragates.TradesStrategyEvents) []string {
	var data cooldown.DepthPriorityEvent
	if err := hold.DecodeData(&data); err != nil || data.PrioritySymbol == "" {
		return nil
	}
	return []string{fmt.Sprintf("%s at depth %d of %d holds this ladder at depth %d of %d",
		data.PrioritySymbol, data.PriorityDepth, data.PriorityMaxDepth, data.Depth, data.MaxDepth)}
}

// slowDeclineReset is what a held tick does to the gate: a pending ladder is reset, the reset row
// naming the features it was pending on, and left watched and not pending; one not pending gets nothing.
func slowDeclineReset(st state) (state, tick) {
	if !st.slowDeclinePending {
		return st, tick{}
	}
	found := tick{kind: rowReset}
	for _, feature := range st.pendingFeatures {
		found.findings = append(found.findings, Finding{Feature: feature})
	}
	return st.notPending(), found
}
