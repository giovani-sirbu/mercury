package smarttakeloss

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// ExitRow is the row the engines write beside a forced sellLoss: ExitMessage at the level and the
// EventSold event under the gate of the rule that sold, with the features the tick's gate row names.
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
