package smartsell

import (
	"slices"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// reachConfirmation is where a reached trigger stands in the activation's confirmation (confirmationOf).
type reachConfirmation int

const (
	// reachConfirmed acts on the reached trigger.
	reachConfirmed reachConfirmation = iota
	// reachFresh is a reached print with no reached row, or a stale one: its reached row stamps it.
	reachFresh
	// reachPending is a reached print younger than ActivationConfirmation after the reached row.
	reachPending
)

// confirmationOf is where a reached print at now stands against the trigger's reached stamp, the trade's newest fill and the window.
func confirmationOf(trade aggragates.Trades, trigger saleTrigger, now time.Time) reachConfirmation {
	if activationConfirmation <= 0 || now.IsZero() {
		return reachConfirmed
	}
	if trigger.reachedAt.IsZero() || staleReach(trigger.reachedAt, newestFillMoment(trade), now) {
		return reachFresh
	}
	if now.Sub(trigger.reachedAt) >= activationConfirmation {
		return reachConfirmed
	}
	return reachPending
}

// staleReach is whether a reached stamp is older than a known fill time or than a window above zero.
func staleReach(reachedAt, fillAt, now time.Time) bool {
	if !fillAt.IsZero() && reachedAt.Before(fillAt) {
		return true
	}
	return activationConfirmationWindow > 0 && now.Sub(reachedAt) > activationConfirmationWindow
}

// newestFillMoment is the FilledMoment of the newest executed entry order, picked as entryFills picks them; zero when none.
func newestFillMoment(trade aggragates.Trades) time.Time {
	entrySide := "BUY"
	if trade.Inverse {
		entrySide = "SELL"
	}
	seenOrders := make(map[int64]struct{}, len(trade.History))
	var moment time.Time
	for index, history := range trade.History {
		if history.Type != entrySide || history.Quantity <= 0 || history.Price <= ladder.AccountingPriceCeiling {
			continue
		}
		orderID := history.OrderId
		if orderID == 0 {
			orderID = -int64(index + 1)
		}
		if _, exists := seenOrders[orderID]; exists {
			continue
		}
		seenOrders[orderID] = struct{}{}
		moment = history.FilledMoment()
	}
	return moment
}

// profitStep judges the live triggers at now on the profit a sale books against each one's trigger: a
// confirmed reach (confirmationOf) sells or activates on the path the profit's sign takes
// (activationStep), a fresh or pending one holds; held is whether a miss or a confirmation holds the sale.
func profitStep(trade aggragates.Trades, live []saleTrigger, fill, price float64, step saleStep, shared sharedTrigger, now time.Time) (rows []Row, next saleStep, reason string, held bool) {
	var hits, misses, fresh []saleTrigger
	var profitPct float64
	confirming := false
	for _, trigger := range live {
		pct, standing := saleProfit(trade, price, trigger.terms.triggerPct(shared))
		profitPct = pct
		switch {
		case standing == nothingUsed:
			if step == stepSell {
				return nil, stepSell, live[0].reason, false
			}
			return nil, stepNone, "", false
		case standing == short:
			misses = append(misses, trigger)
			continue
		}
		switch confirmationOf(trade, trigger, now) {
		case reachConfirmed:
			hits = append(hits, trigger)
		case reachFresh:
			fresh, confirming = append(fresh, trigger), true
		default:
			confirming = true
		}
	}
	if len(hits) > 0 {
		if step == stepSell {
			return nil, stepSell, hits[0].reason, false
		}
		return activationRows(hits, fill, profitPct, shared), activationStep(profitPct), "", false
	}
	rows = profitHeldRows(misses, fill, profitPct, shared)
	if !confirming {
		return rows, stepNone, "", true
	}
	return append(rows, reachedRows(fresh, fill, profitPct, shared)...), stepHold, "", true
}

// reachedRows is the reached row of every trigger, in trigger order, naming the profit against its own trigger.
func reachedRows(triggers []saleTrigger, fill, profitPct float64, shared sharedTrigger) []Row {
	var rows []Row
	for _, trigger := range triggers {
		rows = append(rows, triggerReachedRow(trigger.gate, fill, profitPct, trigger.terms.triggerPct(shared), trigger.features))
	}
	return rows
}

// triggerReachedRow is the one grammar of a reached trigger's row, at the newest fill's price under gate, carrying the trigger.
func triggerReachedRow(gate string, fill, profitPct, triggerPct float64, features []string) Row {
	return Row{
		Message:  RowPrefix + saleHeldLabel(gate) + " trigger reached, " + activatedByProfit(profitPct, triggerPct) + ", the activation confirms after " + activationConfirmation.String(),
		Price:    fill,
		Gate:     gate,
		Event:    EventReached,
		Features: slices.Clone(features),
		Trigger:  &triggerPct,
	}
}

// TriggerReachedRow is the row of a trigger's first reached print under gate (triggerReachedRow), its event EventReached.
func TriggerReachedRow(gate string, fill, profitPct, triggerPct float64, features ...string) Row {
	return triggerReachedRow(gate, fill, profitPct, triggerPct, features)
}
