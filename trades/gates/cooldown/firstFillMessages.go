package cooldown

import (
	"fmt"
	"strconv"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
)

// The messages below are the human-readable half of the first-fill gate: the
// text of the rows an operator reads in the trade's log, which cp and the
// notification filter show and match. The gate's state is not in them.
// firstFillState rebuilds the reference, the anchor and the release from the
// strategy events written beside the rows (FirstFillEvent), so nothing reads
// these texts back — they stay byte-stable for cp, the notification filter and
// every analysis of a recorded log, never as a schema.
//
// Each message is formatted from the event that goes beside its row: the
// floats it prints are the event's, through gates.FormatPriceLevel, and the
// levels are the ones the hold was priced with, so a row and its event cannot
// disagree.
const (
	// FirstFillWaitingPrefix opens the message written while the entry waits
	// at the reference. gates.SaveHoldLog frames it as "Hold entry: …" and
	// writes it again while the hold stands.
	FirstFillWaitingPrefix = "cooldown: trying to get a better entry price: reference "
	// FirstFillArmedPrefix opens every armed message. The anchor is in the
	// text on purpose: gates.SaveHoldLog deduplicates on the whole message, so
	// a new low is a new row and a standing low collapses to one.
	FirstFillArmedPrefix = "cooldown: trying to get a better entry price, armed "
	// FirstFillEnteredPrefix opens the message of the INFO row the gate writes
	// itself, beside its entered event, on the tick the price ran through the
	// reference and the entry went to market. The next word is the direction —
	// "above" on a long, "below" on an inverse ladder.
	FirstFillEnteredPrefix = "cooldown: entered "
	// firstFillInverseSuffix marks a hold on a spot inverse ladder, the
	// convention every entry hold keeps.
	firstFillInverseSuffix = " (inverse)"
)

// firstFillWaitingMessage names the reference and both ways out of the
// hold. Every value in it is fixed while the hold stands, so the text is
// byte-identical tick to tick and gates.SaveHoldLog collapses it.
func firstFillWaitingMessage(trade aggragates.Trades, levels firstFillLevels, data FirstFillEvent) string {
	up := gates.FormatPriceLevel(trade, levels.up(data.Reference))
	arm := gates.FormatPriceLevel(trade, levels.arm(data.Reference))
	reference := gates.FormatPriceLevel(trade, data.Reference)
	if levels.inverse {
		return fmt.Sprintf("%s%s, enters below %s or above %s after a bounce%s",
			FirstFillWaitingPrefix, reference, up, arm, firstFillInverseSuffix)
	}
	return fmt.Sprintf("%s%s, enters above %s or below %s after a bounce",
		FirstFillWaitingPrefix, reference, up, arm)
}

// firstFillArmedMessage names the arm level and the anchor the hold trails.
// The anchor changes the text, which is what makes each trailing step a new
// row (see FirstFillArmedPrefix); the bounce that fills the entry is the
// tolerance, quoted so the operator knows what the row is waiting for.
func firstFillArmedMessage(trade aggragates.Trades, levels firstFillLevels, data FirstFillEvent) string {
	arm := gates.FormatPriceLevel(trade, levels.arm(data.Reference))
	anchor := gates.FormatPriceLevel(trade, data.Anchor)
	tolerance := strconv.FormatFloat(levels.tolerance, 'f', -1, 64)
	if levels.inverse {
		return fmt.Sprintf("%sabove %s: high %s, enters on a %s%% bounce%s",
			FirstFillArmedPrefix, arm, anchor, tolerance, firstFillInverseSuffix)
	}
	return fmt.Sprintf("%sbelow %s: low %s, enters on a %s%% bounce",
		FirstFillArmedPrefix, arm, anchor, tolerance)
}

// firstFillEnteredMessage is the row of the release through the reference:
// the hold called the wrong direction, the entry goes to market and the
// depth after it arms at double the step (NextDepthDoubled).
func firstFillEnteredMessage(trade aggragates.Trades, levels firstFillLevels, data FirstFillEvent) string {
	direction := "above"
	if levels.inverse {
		direction = "below"
	}
	return fmt.Sprintf("%s%s the reference %s, next depth arms at double percentage",
		FirstFillEnteredPrefix, direction, gates.FormatPriceLevel(trade, data.Reference))
}

// firstFillVerdictMessage is the futures hold: the verdict alone, with no
// ladder to price a release from. The short side keeps the inverse suffix
// the entry holds have always carried.
func firstFillVerdictMessage(side string) string {
	if side == aggragates.SideShort {
		return "cooldown: trying to get a better entry price" + firstFillInverseSuffix
	}
	return "cooldown: trying to get a better entry price"
}
