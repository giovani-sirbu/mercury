package smarttakeloss

import "fmt"

// ActivationMarker is the text every activation row carries. It is the
// schema: rebuildState finds the row by this marker anywhere in the message
// (strings.Contains), never by parsing it, and reads the activating fill's
// price from the row's Price column. The text must stay byte-stable across
// releases or the rows already written stop being found.
const ActivationMarker = "smartTakeLoss: Potential trend reversal"

// ActivationMessage frames the marker the way gates.SaveHoldLog frames a
// hold: "Hold <positionType>: …" with the trade's raw PositionType, whatever
// it is — the activation is read on every tick now, not only on the one a
// fill flips to "buy", so a row may well read "Hold stopLoss: …". It is a
// marker row, not a hold: nothing is refused on the activation tick.
func ActivationMessage(positionType string) string {
	return fmt.Sprintf("Hold %s: %s", positionType, ActivationMarker)
}

// WaitMarker is the text of the row written when the wait after a fill
// refuses the ladder's next depth. Like ActivationMarker it is the schema —
// rebuildState finds the row by it and reads the fill it stands for from the
// Price column — so it must stay byte-stable too.
const WaitMarker = "smartTakeLoss: last fill too fresh to sell, no add"

// WaitMessage frames that marker like a hold, because that is what it is:
// the add the ladder proposed is refused while the exit waits out
// MinAgeAfterLastFill. One row per fill, not per tick.
func WaitMessage(positionType string) string {
	return fmt.Sprintf("Hold %s: %s (%s)", positionType, WaitMarker, MinAgeAfterLastFill)
}
