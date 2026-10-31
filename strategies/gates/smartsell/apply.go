package smartsell

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
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
	// State is the state an activation row moves the ladder to (activationState); empty on every other row.
	State string
}

// Result is the overlay's answer: the proposal, the sale's terminal, an activation's state or none on a hold, and the rows to write.
type Result struct {
	Position string
	Reason   string
	// Activated is whether the engines run ActivationActions on this tick: an activation or a
	// re-anchor (Reason empty) in place of the ladder's proposal; nothing is sold, bought or placed then.
	Activated bool
	// Held is whether the overlay holds the sale on this tick (a bound or trend refusal, a profit
	// short of the trigger, a confirmation wait, a gap); the engines show the trade on hold.
	Held bool
	Rows []Row
}

// Apply is ApplyAt with no clock: the waits after a trigger (WaitAfterTrigger and capital
// protection's) then wait nothing.
func Apply(trade aggragates.Trades, position string, price float64, ai aggragates.AIIndicators) Result {
	return ApplyAt(trade, position, price, ai, time.Time{})
}

// ApplyAt overlays the smart sell on the ladder's proposal at now, the stamp its rows get.
func ApplyAt(trade aggragates.Trades, position string, price float64, ai aggragates.AIIndicators, now time.Time) Result {
	result := Result{Position: position}
	if !trade.Strategy.Params.SmartSell || trade.ParentID != 0 || price <= 0 {
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
	st, row := slowDeclineTick(trade, st, ai.SmartSell)
	if row != nil {
		st.slowDeclineTriggeredAt = triggerStamp([]Row{*row}, EventPending, st.slowDeclineTriggeredAt, now)
		result.Rows = append(result.Rows, *row)
	}
	state := gates.PositionType(trade.PositionType)
	// A takeProfit an activation committed is the overlay's whatever the ladder proposes from it.
	committed := state == aggragates.PositionTakeProfit && st.committedIn(state)
	if protectedPosition(state) && !committed {
		return result
	}
	if protectedPosition(gates.PositionType(position)) && !yieldsToActivation(position, trade.PositionType) && !committed {
		return result
	}

	var triggers []saleTrigger
	if !st.depthPriorityHeld {
		var rows []Row
		st, rows, triggers = pausableTriggers(trade, st, ai, now)
		result.Rows = append(result.Rows, rows...)
	}
	if st.capitalProtectionWatched {
		next, rows, holds := capitalProtectionTick(trade, st, ai.SmartSell, now)
		st = next
		result.Rows = append(result.Rows, rows...)
		if holds {
			triggers = append(triggers, capitalProtectionTrigger(st))
		}
	}

	trigger := sharedTriggerOf(series.TrendOf(ai.SmartSell), series.ReadsOf(ai.SmartSell), aggragates.EntrySide(trade, ai))
	shared, step, reason, holds := saleAt(trade, st, price, triggers, trigger, now)
	result.Held = holds
	if step == stepSell {
		return forced(result, reason, saleTerminal(trade, price))
	}
	result.Rows = append(result.Rows, shared...)
	switch step {
	case stepActivateTakeProfit:
		return activated(result, aggragates.PositionTakeProfit)
	case stepActivateTrailingBuy:
		return activated(result, aggragates.PositionTrailingBuy)
	case stepReanchor:
		return activated(result, state)
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
			reason:      reasonSlowDecline,
			gate:        GateSlowDecline,
			features:    st.pendingFeatures,
			rowDue:      st.saleHeldRowDue(),
			heldUnder:   st.saleHeldUnder,
			reachedAt:   st.saleReachedAt,
			gapRowDue:   st.slowDeclineGapHeldAt.IsZero(),
			gapHeldAt:   st.slowDeclineGapHeldAt,
			activated:   st.slowDeclineActivated,
			activatedIn: st.slowDeclineActivatedIn,
		})
	}
	if st.flatMarketWatched {
		next, episode, holds := flatMarketTick(trade, st, ai.SmartSell, ai)
		st = next
		st.flatMarketTriggeredAt = triggerStamp(episode, EventTriggered, st.flatMarketTriggeredAt, now)
		rows = append(rows, episode...)
		if holds && !saleWaiting(st.flatMarketTriggeredAt, st.flatMarketActivated, now) {
			triggers = append(triggers, saleTrigger{
				reason:      reasonFlatMarket,
				gate:        GateFlatMarket,
				rowDue:      !st.flatMarketSaleHeld,
				heldUnder:   st.flatMarketHeldUnder,
				reachedAt:   st.flatMarketReachedAt,
				gapRowDue:   st.flatMarketGapHeldAt.IsZero(),
				gapHeldAt:   st.flatMarketGapHeldAt,
				activated:   st.flatMarketActivated,
				activatedIn: st.flatMarketActivatedIn,
			})
		}
	}
	if st.freeFallPreventionWatched {
		next, episode, holds := freeFallPreventionTick(trade, st, ai.SmartSell, ai)
		st = next
		st.freeFallPreventionTriggeredAt = triggerStamp(episode, EventTriggered, st.freeFallPreventionTriggeredAt, now)
		rows = append(rows, episode...)
		if holds && !saleWaiting(st.freeFallPreventionTriggeredAt, st.freeFallPreventionActivated, now) {
			triggers = append(triggers, saleTrigger{
				reason:      reasonFreeFallPrevention,
				gate:        GateFreeFallPrevention,
				rowDue:      !st.freeFallPreventionSaleHeld,
				heldUnder:   st.freeFallPreventionHeldUnder,
				reachedAt:   st.freeFallPreventionReachedAt,
				gapRowDue:   st.freeFallPreventionGapHeldAt.IsZero(),
				gapHeldAt:   st.freeFallPreventionGapHeldAt,
				activated:   st.freeFallPreventionActivated,
				activatedIn: st.freeFallPreventionActivatedIn,
			})
		}
	}
	return st, rows, triggers
}
