package smarttakeloss

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/cooldown"
)

// depthPriorityHeld is whether a depth priority holds the ladder, which
// pauses the slow decline gate and capital protection on it (Apply,
// TakeProfitPercentage): DepthPriorityHoldPausesSmartTakeLoss on, and the
// trade carries an event of the cooldown depth priority gate
// (cooldown.GateDepthPriority) stamped strictly after its newest entry fill —
// newest in slice order, as rebuildState folds the fills. The gate has one
// kind, its hold, so the event is not decoded. The sophos reading of the
// indecision direction is not paused: a held ladder is latched on it, and a
// latched ladder's take profit reads its position price, all the same.
//
// The comparison is the event's CreatedAt against the fill's history stamp
// (entryFill.At). gates.SaveHoldLog stamps the event with the tick clock, as
// it does the row beside it, and the gate holds only an entry that is not
// placed, so the next fill is stamped after the event: the hold lasts until
// the ladder's next fill, and nothing else ends it — not the gate letting the
// entry through, nor the ladder the wallet was kept for closing. The fill's
// stamp is the order's placement tick in backtesting, its reconciliation
// time in production and the wall clock in live-testing; in backtesting a
// fill of an order placed before the event therefore reads as older than the
// event, the accepted limit. An event or a newest fill without a stamp holds
// nothing, and neither does a ladder with no fill.
//
// The rule is literal: a ladder that has filled its last depth and is held
// on the next entry it proposes is held too, so capital protection sells
// nothing on it until it fills again or closes.
func depthPriorityHeld(trade aggragates.Trades, newest entryFill) bool {
	if !depthPriorityHoldPauses || newest.At.IsZero() {
		return false
	}
	for _, event := range trade.StrategyEvents {
		if event.Param != aggragates.StrategyParamCooldown || event.Gate != cooldown.GateDepthPriority {
			continue
		}
		if event.CreatedAt.After(newest.At) {
			return true
		}
	}
	return false
}

// slowDeclineReset is what a held tick finds of the gate, if anything, and the
// state as it leaves it: a pending ladder is reset — the reset row, naming the
// decline features it was pending on and carrying its newest fill's price, and
// the ladder watched and not pending — exactly as a cancelled row leaves it, so
// only goesPending makes it pending again once the hold has ended. It resets
// whichever feature made the ladder pending, and a ladder the events of the
// gate it replaced left pending too (rebuildState). A ladder not pending gets
// nothing, so the reset is written once.
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
