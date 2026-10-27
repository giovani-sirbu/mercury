package dynamicparams

import (
	"strconv"
	"strings"
)

// RowPrefix opens the opened row; agora keeps rows carrying it off the notifications, so it is
// byte-stable text, never a schema.
const RowPrefix = "dynamicParams:"

// OpenedMarker follows RowPrefix on the opened row; byte-stable text no reader matches.
const OpenedMarker = "volatility expected, strategy params adjusted"

// openedHead is how the opened row begins.
const openedHead = RowPrefix + " " + OpenedMarker

// The labels of the opened row's amounts.
const (
	percentageLabel = "percentage +"
	depthsLabel     = "depths +"
)

// OpenedMessage is the opened row's text: openedHead, a colon, then the amounts (AmountsOf);
// byte-stable, and the ladder's amounts are read from the event, never from it.
func OpenedMessage(points float64, depths int) string {
	return openedHead + ": " + AmountsOf(points, depths)
}

// AmountsOf names each part of a raise that raises something under its label (shortest float form),
// joined with ", "; the opened row and the early take profit latch's row print it.
func AmountsOf(points float64, depths int) string {
	var parts []string
	if points != 0 {
		parts = append(parts, percentageLabel+strconv.FormatFloat(points, 'f', -1, 64))
	}
	if depths != 0 {
		parts = append(parts, depthsLabel+strconv.Itoa(depths))
	}

	return strings.Join(parts, ", ")
}
