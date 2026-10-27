package smarttakeloss

import (
	"slices"

	"github.com/giovani-sirbu/mercury/strategies/gates/dynamicparams"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// trailingStep trails a committed stopLoss on the tick's move against the anchor (trailingMove): a gap
// holds (gapped), under minus the tolerance sells, over trailingTakeProfit re-anchors; no anchor sells.
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

// committed is whether any trigger activated the stopLoss, which commits the sale.
func committed(triggers []saleTrigger) bool {
	return slices.ContainsFunc(triggers, func(trigger saleTrigger) bool {
		return trigger.activated
	})
}

// committedReason names a committed sale by its trigger (committedTrigger).
func committedReason(live []saleTrigger) string {
	return committedTrigger(live).reason
}

// committedTrigger is the first live trigger that activated, else the first live one.
func committedTrigger(live []saleTrigger) saleTrigger {
	for _, trigger := range live {
		if trigger.activated {
			return trigger
		}
	}
	return live[0]
}
