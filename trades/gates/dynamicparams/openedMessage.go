package dynamicparams

import (
	"strconv"
	"strings"
)

// RowPrefix opens the one row the engines write for the flag: the opened
// row. agora keeps the rows carrying it off the users' notifications and
// OpenedRaise finds the row by it, so it must stay byte-stable.
const RowPrefix = "dynamic params:"

// OpenedMarker is the words the opened row carries right after RowPrefix:
// Opening writes them, and OpenedRaise matches the row by them.
const OpenedMarker = "opened raised"

// openedHead is how the opened row begins: RowPrefix, then OpenedMarker.
const openedHead = RowPrefix + " " + OpenedMarker

// The labels the opened row names its amounts under, each followed by the
// amount: the points added to every row's percentage, and the depths added
// to every row's depths.
const (
	percentageLabel = "percentage +"
	depthsLabel     = "depths +"
)

// OpenedMessage is the opened row's text for a ladder that opens with points
// added to every row's percentage and depths added to every row's depths:
// openedHead, then the parts that raise something — the percentage and the
// depths, each under its label with its amount, joined with " and " — and
// " on every row". A part whose amount is zero is left out; Opening never
// writes a row whose amounts are both zero. The amounts are written with no
// trailing zeros (strconv 'f' at the shortest precision), the text
// OpenedRaise parses back to the very amount written.
//
// Every engine writes this one text, and OpenedRaise reads the ladder's rows
// back from it on every later tick, so it must stay byte-stable.
func OpenedMessage(points float64, depths int) string {
	var parts []string
	if points != 0 {
		parts = append(parts, percentageLabel+strconv.FormatFloat(points, 'f', -1, 64))
	}
	if depths != 0 {
		parts = append(parts, depthsLabel+strconv.Itoa(depths))
	}

	return openedHead + ", " + strings.Join(parts, " and ") + " on every row"
}
