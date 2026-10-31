package smartsell

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// ExitRow is the row the engines write beside a committed sale: ExitMessage at the level, worded by
// the sale's terminal there (saleTerminal), and the EventSold event under the gate of the rule that
// sold, with the features the tick's gate row names.
func ExitRow(trade aggragates.Trades, reason string, level float64) Row {
	features := exitFeatures(trade, reason)
	row := Row{
		Message:  exitMessage(trade, reason, level, features),
		Price:    level,
		Gate:     GateSlowDecline,
		Event:    EventSold,
		Features: features,
	}
	switch reason {
	case reasonCapitalProtection:
		row.Gate = GateCapitalProtection
	case reasonFlatMarket:
		row.Gate = GateFlatMarket
	case reasonFreeFallPrevention:
		row.Gate = GateFreeFallPrevention
	}
	return row
}
