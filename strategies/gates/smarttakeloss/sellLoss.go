package smarttakeloss

import (
	"time"

	"github.com/giovani-sirbu/mercury/strategies/gates"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// saleTerms are the terms the shared action judges a trigger on, read at decision time: the zero
// value is the shared terms, the depth bounds and the trigger the SMC trend dashboard gives from
// SellLossTriggers (sharedTrigger); capital protection's own skip the bounds and the trend
// and take capitalprotection.MaxDrawdownPercentage as a loss.
type saleTerms int

const (
	sharedTerms saleTerms = iota
	capitalProtectionTerms
)

// bounded is whether the depth bounds and the trend hold the trigger (sellLossStartAtDepth,
// sellLossIgnoreLastDepths, sharedTrigger.on).
func (terms saleTerms) bounded() bool {
	return terms == sharedTerms
}

// triggerPct is the profit the trigger activates buy at and sells a self-armed stopLoss at: the
// tick's shared trigger, or capital protection's drawdown.
func (terms saleTerms) triggerPct(shared sharedTrigger) float64 {
	if terms == capitalProtectionTerms {
		return 0 - capitalProtectionMaxDrawdown
	}
	return shared.pct
}

// saleTrigger is one gate whose trigger stands on a tick, as Apply hands it to sellLoss.
type saleTrigger struct {
	// reason names the gate's sale in Result.Reason when this trigger sells (committedReason, profitStep).
	reason string
	// gate files and words the gate's rows (GateSlowDecline, GateFlatMarket,
	// GateFreeFallPrevention, GateCapitalProtection).
	gate string
	// features are the decline features the ladder is pending on; none for the other gates.
	features []string
	// rowDue is whether no held row stands in the gate's episode yet, when a bound's or the trend's
	// is due; read for the bounded triggers only (heldRows), a held row by the profit reads heldUnder.
	rowDue bool
	// heldUnder is the trigger the episode's newest held row was held under (profitRowDue).
	heldUnder heldTrigger
	// reachedAt is the stamp of the gate's reached event the activation confirms from; zero when none stands.
	reachedAt time.Time
	// gapRowDue is whether a gapped print opens a gap with its row (none open), gapHeldAt the open gap's
	// stamp SaleGapWait and the close read; the committed branch's only (committedStep).
	gapRowDue bool
	gapHeldAt time.Time
	// activated is whether the gate activated the stopLoss in its episode, committing the sale.
	activated bool
	// terms are the terms the trigger is judged on (saleTerms).
	terms saleTerms
}

// saleStep is what the shared action does to the ladder on a tick.
type saleStep int

const (
	// stepNone does nothing to the ladder, the held rows of a refusal aside.
	stepNone saleStep = iota
	// stepActivate moves the ladder from buy to stopLoss (activated).
	stepActivate
	// stepSell sells the ladder in stopLoss at market (forced).
	stepSell
	// stepReanchor keeps the committed ladder in stopLoss and moves its anchor up to the tick (activated).
	stepReanchor
	// stepHold holds the ladder, its proposal replaced by nothing (held): a committed one between its
	// bounds, or one whose reached trigger waits for its confirmation.
	stepHold
	// stepGapHold is trailingStep's hold of a committed ladder on a print gapping under the floor (gapped).
	stepGapHold
)

// sellLoss is sellLossAt with no clock: a reached trigger acts on its first print.
func sellLoss(trade aggragates.Trades, st state, price float64, triggers []saleTrigger, shared sharedTrigger) (rows []Row, step saleStep, reason string) {
	return sellLossAt(trade, st, price, triggers, shared, time.Time{})
}

// sellLossAt decides the shared action at now from the ladder's state: refusals, the committed trail (committedStep), else the profit (profitStep).
func sellLossAt(trade aggragates.Trades, st state, price float64, triggers []saleTrigger, shared sharedTrigger, now time.Time) (rows []Row, step saleStep, reason string) {
	if len(triggers) == 0 {
		return nil, stepNone, ""
	}
	switch gates.PositionType(trade.PositionType) {
	case "buy":
		if trade.PendingOrder != 0 {
			return nil, stepNone, ""
		}
		step = stepActivate
	case "stopLoss":
		step = stepSell
	default:
		return nil, stepNone, ""
	}

	fill := st.lastFill().Price
	refusals, live := boundRefusal(trade, st, step, triggers, fill)
	if len(live) == 0 {
		return refusals, stepNone, ""
	}
	if step == stepSell && committed(triggers) {
		return committedStep(trade, committedTrigger(live), price, fill, now)
	}
	trendRefusals, live := trendRefusal(live, shared, fill)
	refusals = append(refusals, trendRefusals...)
	if len(live) == 0 {
		return refusals, stepNone, ""
	}
	rows, step, reason = profitStep(trade, live, fill, price, step, shared, now)
	if step == stepSell {
		return nil, stepSell, reason
	}
	return append(refusals, rows...), step, ""
}

// boundRefusal splits the triggers on the depth bounds (boundRefused): the held rows of the bounded
// triggers a bound refuses, and the live triggers, the unbounded ones and every trigger when no
// bound refuses.
func boundRefusal(trade aggragates.Trades, st state, step saleStep, triggers []saleTrigger, fill float64) (refusals []Row, live []saleTrigger) {
	why, refused := boundRefused(trade, st, step)
	if !refused {
		return nil, triggers
	}
	return splitBounded(triggers, fill, why)
}

// trendRefusal is boundRefusal for the trend: while the shared trigger is off (no row reading the
// ladder's way, or the trend not read) the bounded triggers are held, worded by heldByTrend.
func trendRefusal(triggers []saleTrigger, shared sharedTrigger, fill float64) (refusals []Row, live []saleTrigger) {
	if shared.on {
		return nil, triggers
	}
	return splitBounded(triggers, fill, shared.heldByTrend())
}

// splitBounded is the held rows of the bounded triggers, worded by why, and the unbounded ones live.
func splitBounded(triggers []saleTrigger, fill float64, why string) (refusals []Row, live []saleTrigger) {
	var held []saleTrigger
	for _, trigger := range triggers {
		if trigger.terms.bounded() {
			held = append(held, trigger)
		} else {
			live = append(live, trigger)
		}
	}
	return heldRows(held, fill, why), live
}

// boundRefused is whether a depth bound refuses the bounded triggers on this tick, and why: the
// floor of an activation first (sellLossStartAtDepth), then the ceiling (sellLossIgnoreLastDepths).
func boundRefused(trade aggragates.Trades, st state, step saleStep) (why string, refused bool) {
	filled := len(st.fills)
	if step == stepActivate && sellLossStartAtDepth > 0 && filled < sellLossStartAtDepth {
		return heldUnderDepth(filled, sellLossStartAtDepth), true
	}
	if sellLossIgnoreLastDepths > 0 {
		if ceiling := ladder.ConfiguredDepths(trade); filled > ceiling-sellLossIgnoreLastDepths {
			return heldAtDepth(filled, ceiling), true
		}
	}
	return "", false
}
