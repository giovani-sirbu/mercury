package smarttakeloss

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
	"time"

	"github.com/giovani-sirbu/mercury/strategies/gates"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// Row is a decision Apply asks the engine to write: the log row (Message at Price) and the strategy
// event beside it, written under one stamp; built only by the row builders (PendingRow, ExitRow and
// the rest), as a Row without a Gate files its event under none. Price is the newest fill's price.
type Row struct {
	Message  string
	Price    float64
	Gate     string
	Event    string
	Features []string
	Reasons  []string
	// Trigger is the trigger a row held by the profit was held under (profitRowDue) or a reached row
	// reached (TriggerReachedRow); nil on every other row.
	Trigger *float64
}

// Result is the overlay's answer: Position is the proposal, "sellLoss" on a forced exit (Reason
// names the rule for ExitRow), "stopLoss" when the shared action activated or re-anchored, or empty
// on a hold; Rows are the rows to write in order (the gate's, the episodes', the action's).
type Result struct {
	Position string
	Reason   string
	// Activated is whether the engines run ActivationActions on this tick: an activation or a
	// re-anchor (Position "stopLoss", Reason empty) in place of the ladder's proposal; nothing is
	// sold, bought or placed then.
	Activated bool
	Rows      []Row
}

// Apply is ApplyAt with no clock: the waits after a trigger (WaitAfterTrigger and capital
// protection's) then wait nothing.
func Apply(trade aggragates.Trades, position string, price float64, ai aggragates.AIIndicators) Result {
	return ApplyAt(trade, position, price, ai, time.Time{})
}

// ApplyAt overlays the smart take loss on the ladder's proposal at now, the stamp its rows get: the slow
// decline tick, the protected positions, then the episodes' triggers fed to the shared action (sellLossAt).
func ApplyAt(trade aggragates.Trades, position string, price float64, ai aggragates.AIIndicators, now time.Time) Result {
	result := Result{Position: position}
	if !trade.Strategy.Params.SmartTakeLoss || trade.ParentID != 0 || price <= 0 {
		return result
	}
	if len(trade.StrategyPair.StrategySettings) == 0 {
		return result
	}

	st := rebuildState(trade)
	if !st.slowDeclineWatched && !st.capitalProtectionWatched && !st.earlyTakeProfitWatched &&
		!st.flatMarketWatched && !st.freeFallPreventionWatched {
		return result
	}
	st, row := slowDeclineTick(trade, st, ai.SmartTakeLoss)
	if row != nil {
		st.slowDeclineTriggeredAt = triggerStamp([]Row{*row}, EventPending, st.slowDeclineTriggeredAt, now)
		result.Rows = append(result.Rows, *row)
	}
	if protectedPosition(gates.PositionType(trade.PositionType)) {
		return result
	}
	if protectedPosition(gates.PositionType(position)) && !yieldsToActivation(position, trade.PositionType) {
		return result
	}

	var triggers []saleTrigger
	if !st.depthPriorityHeld {
		var rows []Row
		st, rows, triggers = pausableTriggers(trade, st, ai, now)
		result.Rows = append(result.Rows, rows...)
	}
	if st.capitalProtectionWatched {
		next, rows, holds := capitalProtectionTick(trade, st, ai.SmartTakeLoss, now)
		st = next
		result.Rows = append(result.Rows, rows...)
		if holds {
			triggers = append(triggers, capitalProtectionTrigger(st))
		}
	}

	trigger := sharedTriggerOf(series.TrendOf(ai.SmartTakeLoss), series.ReadsOf(ai.SmartTakeLoss), aggragates.EntrySide(trade, ai))
	shared, step, reason := sellLossAt(trade, st, price, triggers, trigger, now)
	if step == stepSell {
		return forced(result, reason)
	}
	result.Rows = append(result.Rows, shared...)
	switch step {
	case stepActivate, stepReanchor:
		return activated(result)
	case stepHold:
		return held(result)
	}
	return result
}

// pausableTriggers are the slow decline, flat market and free fall prevention gates' triggers and their
// episodes' rows at now, none from a gate still waiting after its trigger (saleWaiting).
func pausableTriggers(trade aggragates.Trades, st state, ai aggragates.AIIndicators, now time.Time) (state, []Row, []saleTrigger) {
	var rows []Row
	var triggers []saleTrigger
	if slowDeclineTriggered(trade, st) && !saleWaiting(st.slowDeclineTriggeredAt, st.slowDeclineActivated, now) {
		triggers = append(triggers, saleTrigger{
			reason:    reasonSlowDecline,
			gate:      GateSlowDecline,
			features:  st.pendingFeatures,
			rowDue:    st.saleHeldRowDue(),
			heldUnder: st.saleHeldUnder,
			reachedAt: st.saleReachedAt,
			gapRowDue: st.slowDeclineGapHeldAt.IsZero(),
			gapHeldAt: st.slowDeclineGapHeldAt,
			activated: st.slowDeclineActivated,
		})
	}
	if st.flatMarketWatched {
		next, episode, holds := flatMarketTick(trade, st, ai.SmartTakeLoss, ai)
		st = next
		st.flatMarketTriggeredAt = triggerStamp(episode, EventTriggered, st.flatMarketTriggeredAt, now)
		rows = append(rows, episode...)
		if holds && !saleWaiting(st.flatMarketTriggeredAt, st.flatMarketActivated, now) {
			triggers = append(triggers, saleTrigger{
				reason:    reasonFlatMarket,
				gate:      GateFlatMarket,
				rowDue:    !st.flatMarketSaleHeld,
				heldUnder: st.flatMarketHeldUnder,
				reachedAt: st.flatMarketReachedAt,
				gapRowDue: st.flatMarketGapHeldAt.IsZero(),
				gapHeldAt: st.flatMarketGapHeldAt,
				activated: st.flatMarketActivated,
			})
		}
	}
	if st.freeFallPreventionWatched {
		next, episode, holds := freeFallPreventionTick(trade, st, ai.SmartTakeLoss, ai)
		st = next
		st.freeFallPreventionTriggeredAt = triggerStamp(episode, EventTriggered, st.freeFallPreventionTriggeredAt, now)
		rows = append(rows, episode...)
		if holds && !saleWaiting(st.freeFallPreventionTriggeredAt, st.freeFallPreventionActivated, now) {
			triggers = append(triggers, saleTrigger{
				reason:    reasonFreeFallPrevention,
				gate:      GateFreeFallPrevention,
				rowDue:    !st.freeFallPreventionSaleHeld,
				heldUnder: st.freeFallPreventionHeldUnder,
				reachedAt: st.freeFallPreventionReachedAt,
				gapRowDue: st.freeFallPreventionGapHeldAt.IsZero(),
				gapHeldAt: st.freeFallPreventionGapHeldAt,
				activated: st.freeFallPreventionActivated,
			})
		}
	}
	return st, rows, triggers
}
