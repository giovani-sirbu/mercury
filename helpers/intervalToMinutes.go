package helpers

import (
	"strconv"
	"strings"
)

// IntervalToMinutes converts a Binance interval string (e.g. "15m", "1h") into minutes.
// Returns -1 if the format is invalid.
func IntervalToMinutes(interval string) int {
	if len(interval) < 2 {
		return -1
	}

	// Split numeric part and unit part
	numPart := interval[:len(interval)-1]
	unit := interval[len(interval)-1:]

	value, err := strconv.Atoi(numPart)
	if err != nil || value <= 0 {
		return -1
	}

	switch strings.ToLower(unit) {
	case "m": // minutes
		return value
	case "h": // hours → minutes
		return value * 60
	case "d": // days → minutes
		return value * 60 * 24
	case "w": // weeks → minutes
		return value * 60 * 24 * 7
	default:
		return -1
	}
}
