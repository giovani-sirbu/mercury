package actions

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/strategies/gates"
	"github.com/giovani-sirbu/mercury/strategies/gates/ai"
	"github.com/giovani-sirbu/mercury/strategies/gates/cooldown/depthpriority"
	"github.com/giovani-sirbu/mercury/strategies/gates/cooldown/depthspacing"
	"github.com/giovani-sirbu/mercury/strategies/gates/cooldown/firstfill"
	"github.com/giovani-sirbu/mercury/strategies/gates/patterns"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/tradelog"
)

// ShouldHold blocks the action chain when a strategy flag advises against
// acting; a hold is recorded by tradelog.SaveHoldLog, with the gate's strategy
// event beside the row where the gate owns one. Every gate answers to exactly
// one flag, flag first and payload second: a payload fetched for another
// flag's sake switches no gate on (StrategyParams.NeedsSophos). The smart
// take loss holds only the first fill here; on an open ladder it acts through
// smarttakeloss.Apply, after the ladder decides. With every flag off nothing
// holds and the ladder is stopped by funds alone.
func ShouldHold(event events.Events) (events.Events, error) {
	if event.Params.OldPosition == "new" {
		return shouldHoldEntry(event)
	}
	return shouldHoldPosition(event)
}

// shouldHoldEntry is the first fill, judged on the side aggragates.EntrySide
// resolves (Inverse on spot, the ML verdict on futures). In order: the wallet
// reserve (depth priority), the smart take loss entry holds (quiet decline,
// flat market, then free fall prevention), the first-fill gate, then the
// legacy AI veto. The wallet
// reserve and the smart take loss run before the first-fill gate so a held
// entry neither consumes nor records a first-fill verdict.
func shouldHoldEntry(event events.Events) (events.Events, error) {
	params := event.Trade.Strategy.Params
	side := aggragates.EntrySide(event.Trade, event.Params.AIIndicators)

	if params.Cooldown {
		// Before the first-fill gate so a held entry records no verdict.
		if hold := depthpriority.DepthPriorityHold(event, event.Trade.PositionType); hold.Held() {
			return tradelog.SaveHoldLog(event, "entry", hold)
		}
	}
	if params.SmartTakeLoss {
		// No new ladder while a quiet decline, a flat market or a free fall reading stands.
		if hold := smarttakeloss.EntryHold(event.Trade, side, event.Params.AIIndicators); hold.Held() {
			return tradelog.SaveHoldLog(event, "entry", hold)
		}
	}
	if params.Cooldown {
		// The gate hands the event back with the entered row firstfill.NextDepthDoubled reads.
		var hold gates.Hold
		event, hold = firstfill.FirstFillHold(event, side)
		if hold.Held() {
			return tradelog.SaveHoldLog(event, "entry", hold)
		}
	}
	if params.UseAI {
		if reason := ai.EntryHold(side, event.Params.AIIndicators); reason != "" {
			return tradelog.SaveHoldLog(event, "entry", gates.Hold{Reason: reason})
		}
	}
	return event, nil
}

// shouldHoldPosition is every transition after the first fill: arming a depth
// (stopLoss), arming the exit (takeProfit) and the force-trailing re-anchors
// of either. The first hold that speaks wins, in this order: the chart
// patterns (UsePatterns), the legacy AI hold (UseAI), then under Cooldown the
// wallet reserve (depth priority) and depth spacing, last because it has no
// view of the market.
func shouldHoldPosition(event events.Events) (events.Events, error) {
	params := event.Trade.Strategy.Params
	indicators := event.Params.AIIndicators
	position := gates.PositionType(event.Trade.PositionType)

	// The market families first; both write the row alone.
	var hold gates.Hold
	if params.UsePatterns {
		hold.Reason = patterns.HoldReason(event, position, indicators)
	}
	if !hold.Held() && params.UseAI {
		hold.Reason = ai.LegacyHoldReason(event, position, indicators)
	}

	if !hold.Held() && params.Cooldown {
		hold = depthpriority.DepthPriorityHold(event, position)
	}

	if !hold.Held() && params.Cooldown {
		// Last: depth spacing has no view of the market.
		hold = depthspacing.DepthSpacingHold(event, position)
	}

	if hold.Held() {
		return tradelog.SaveHoldLog(event, event.Trade.PositionType, hold)
	}
	return event, nil
}
