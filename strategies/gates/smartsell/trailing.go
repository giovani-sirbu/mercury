package smartsell

import (
	"slices"

	"github.com/giovani-sirbu/mercury/strategies/gates/dynamicparams"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// trailingStep trails a committed ladder (trailingBuy or takeProfit) on the tick's move against the
// anchor (trailingMove): a gap holds (gapped), under minus the tolerance sells, over
// trailingTakeProfit re-anchors; no anchor sells.
func trailingStep(trade aggragates.Trades, price float64) saleStep {
	move, row, ok := trailingMove(trade, price)
	if !ok {
		return stepSell
	}

	switch {
	case gapped(move, row.Tolerance):
		return stepGapHold
	case move < -row.Tolerance:
		return stepSell
	case move > row.TrailingTakeProfit:
		return stepReanchor
	}
	return stepHold
}

// trailingMove is the tick's move in percent against the anchor, negated inverse, and the held depth's settings row.
func trailingMove(trade aggragates.Trades, price float64) (move float64, row aggragates.StrategySettings, ok bool) {
	rows, _ := dynamicparams.RaisedSettings(trade)
	anchor := trade.PositionPrice
	if anchor <= 0 || len(rows) == 0 {
		return 0, aggragates.StrategySettings{}, false
	}

	row = rows[ladder.SettingsIndexOrBase(rows, ladder.CountFilledEntries(trade)-1)]
	move = (price - anchor) / price * 100
	if trade.Inverse {
		move = -move
	}
	return move, row, true
}

// committedIn is whether any trigger activated the sale into position, which commits it there.
func committedIn(triggers []saleTrigger, position string) bool {
	return slices.ContainsFunc(triggers, func(trigger saleTrigger) bool {
		return trigger.committedIn(position)
	})
}

// committedIn is whether the trigger activated the sale into position.
func (trigger saleTrigger) committedIn(position string) bool {
	return trigger.activated && trigger.activatedIn == position
}

// committedReason names a sale committed in position by its trigger (committedTrigger).
func committedReason(live []saleTrigger, position string) string {
	return committedTrigger(live, position).reason
}

// committedTrigger is the first live trigger that activated the sale into position, else the first live one.
func committedTrigger(live []saleTrigger, position string) saleTrigger {
	for _, trigger := range live {
		if trigger.committedIn(position) {
			return trigger
		}
	}
	return live[0]
}
