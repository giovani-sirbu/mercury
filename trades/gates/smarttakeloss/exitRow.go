package smarttakeloss

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// ExitRow is the row the engines write beside a forced sellLoss: reason is
// Result.Reason, the rule that sold, and level the price the sellLoss chain
// places its limit at. The message is ExitMessage, the price the level itself
// — unrounded, where the message prints the pair's decimals — and the event is
// EventSold filed under the gate of the rule that sold: the slow decline's for
// its sell band, the slow pattern's for the same band under its own reason,
// capital protection's for its upper band. The engines build
// the row of a sale Apply forced with it, and Rows writes it like every
// other.
func ExitRow(trade aggragates.Trades, reason string, level float64) Row {
	gate := GateSlowDecline
	switch reason {
	case reasonCapitalProtection:
		gate = GateCapitalProtection
	case reasonSlowPatternBand:
		gate = GateSlowPattern
	}

	return Row{
		Message: ExitMessage(trade, reason, level),
		Price:   level,
		Gate:    gate,
		Event:   EventSold,
	}
}
