package smarttakeloss

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// ExitRow is the row the engines write beside a forced sellLoss: reason is
// Result.Reason, the rule that sold, and level the price the sellLoss chain
// places its limit at. The message is ExitMessage, the price the level itself
// — unrounded, where the message prints the pair's decimals — and the event is
// EventSold filed under the gate of the rule that sold: the slow decline
// gate's for its sell band, carrying the decline features the ladder was
// pending on, capital protection's for its upper band, carrying none. The
// engines build the row of a sale Apply forced with it after they have written
// the tick's gate row, which the trade must carry for the features to be read,
// and Rows writes it like every other.
func ExitRow(trade aggragates.Trades, reason string, level float64) Row {
	features := exitFeatures(trade, reason)
	row := Row{
		Message:  exitMessage(trade, reason, level, features),
		Price:    level,
		Gate:     GateSlowDecline,
		Event:    EventSold,
		Features: features,
	}
	if reason == reasonCapitalProtection {
		row.Gate = GateCapitalProtection
	}
	return row
}
