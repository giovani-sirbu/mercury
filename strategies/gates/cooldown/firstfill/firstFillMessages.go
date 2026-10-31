package firstfill

import (
	"fmt"
	"strconv"

	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/strategies/gates/cooldown"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The rows' texts, formatted from the event beside each row: byte-stable for cp and the
// notification filter, never read back (firstFillState reads the events).
const (
	// firstFillInverseSuffix marks a hold on a spot inverse ladder.
	firstFillInverseSuffix = " (inverse)"
)

// firstFillWaitingMessage names the reference and both ways out of the hold; fixed while it stands.
func firstFillWaitingMessage(trade aggragates.Trades, levels firstFillLevels, data cooldown.FirstFillEvent) string {
	up := helpers.FormatPriceLevel(trade, levels.up(data.Reference))
	arm := helpers.FormatPriceLevel(trade, levels.arm(data.Reference))
	reference := helpers.FormatPriceLevel(trade, data.Reference)
	if levels.inverse {
		return fmt.Sprintf("%s%s, enters below %s or above %s after a bounce%s",
			cooldown.FirstFillWaitingPrefix, reference, up, arm, firstFillInverseSuffix)
	}
	return fmt.Sprintf("%s%s, enters above %s or below %s after a bounce",
		cooldown.FirstFillWaitingPrefix, reference, up, arm)
}

// firstFillArmedMessage names the arm level, the anchor (a new anchor is a new row) and the bounce.
func firstFillArmedMessage(trade aggragates.Trades, levels firstFillLevels, data cooldown.FirstFillEvent) string {
	arm := helpers.FormatPriceLevel(trade, levels.arm(data.Reference))
	anchor := helpers.FormatPriceLevel(trade, data.Anchor)
	tolerance := strconv.FormatFloat(levels.tolerance, 'f', -1, 64)
	if levels.inverse {
		return fmt.Sprintf("%sabove %s: high %s, enters on a %s%% bounce%s",
			cooldown.FirstFillArmedPrefix, arm, anchor, tolerance, firstFillInverseSuffix)
	}
	return fmt.Sprintf("%sbelow %s: low %s, enters on a %s%% bounce",
		cooldown.FirstFillArmedPrefix, arm, anchor, tolerance)
}

// firstFillEnteredMessage is the row of the release through the reference (NextDepthDoubled).
func firstFillEnteredMessage(trade aggragates.Trades, levels firstFillLevels, data cooldown.FirstFillEvent) string {
	direction := "above"
	if levels.inverse {
		direction = "below"
	}
	return fmt.Sprintf("%s%s the reference %s, next depth arms at double percentage",
		cooldown.FirstFillEnteredPrefix, direction, helpers.FormatPriceLevel(trade, data.Reference))
}

// firstFillVerdictMessage is the futures hold's row: the verdict alone.
func firstFillVerdictMessage(side string) string {
	if side == aggragates.SideShort {
		return "cooldown: trying to get a better entry price" + firstFillInverseSuffix
	}
	return "cooldown: trying to get a better entry price"
}
