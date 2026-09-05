package cooldown

import (
	"fmt"
	"strconv"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
)

// The first-fill gate keeps its state in the trade's own log rows, so the
// messages below are also its schema: firstFillState rebuilds the reference,
// the anchor and the release from them on every tick. Each phase opens with
// its own marker and the markers are distinct from one another — the armed
// row is keyed on ", armed ", never on the "trying to get a better entry
// price" stem it shares with the waiting row — so a rebuild cannot mistake
// one phase for the other.
//
// The prices in the text are for the operator. The rebuild reads the row's
// Price column and never parses the message.
const (
	// FirstFillWaitingPrefix opens the row written while the entry waits at
	// the reference. gates.SaveHoldLog frames it as "Hold entry: …" and
	// re-logs it once a day, so the FIRST such row is the reference.
	FirstFillWaitingPrefix = "cooldown: trying to get a better entry price: reference "
	// FirstFillArmedPrefix opens every armed row. The anchor is in the text
	// on purpose: gates.SaveHoldLog deduplicates on the whole message, so a
	// new low is a new row and a standing low collapses to one.
	FirstFillArmedPrefix = "cooldown: trying to get a better entry price, armed "
	// FirstFillEnteredPrefix opens the INFO row the gate writes itself on
	// the tick the price ran through the reference and the entry went to
	// market: the fact NextDepthDoubled reads. The next word is the
	// direction — "above" on a long, "below" on an inverse ladder.
	FirstFillEnteredPrefix = "cooldown: entered "
	// firstFillInverseSuffix marks a hold on a spot inverse ladder, the
	// convention every entry hold keeps.
	firstFillInverseSuffix = " (inverse)"
)

// firstFillWaitingMessage names the reference and both ways out of the
// hold. Every value in it is fixed while the hold stands, so the text is
// byte-identical tick to tick and gates.SaveHoldLog collapses it.
func firstFillWaitingMessage(trade aggragates.Trades, levels firstFillLevels, reference float64) string {
	up := gates.FormatPriceLevel(trade, levels.up(reference))
	arm := gates.FormatPriceLevel(trade, levels.arm(reference))
	if levels.inverse {
		return fmt.Sprintf("%s%s, enters below %s or above %s after a bounce%s",
			FirstFillWaitingPrefix, gates.FormatPriceLevel(trade, reference), up, arm, firstFillInverseSuffix)
	}
	return fmt.Sprintf("%s%s, enters above %s or below %s after a bounce",
		FirstFillWaitingPrefix, gates.FormatPriceLevel(trade, reference), up, arm)
}

// firstFillArmedMessage names the arm level and the anchor the hold trails.
// The anchor changes the text, which is what makes each trailing step a new
// row (see FirstFillArmedPrefix); the bounce that fills the entry is the
// tolerance, quoted so the operator knows what the row is waiting for.
func firstFillArmedMessage(trade aggragates.Trades, levels firstFillLevels, reference, anchor float64) string {
	arm := gates.FormatPriceLevel(trade, levels.arm(reference))
	tolerance := strconv.FormatFloat(levels.tolerance, 'f', -1, 64)
	if levels.inverse {
		return fmt.Sprintf("%sabove %s: high %s, enters on a %s%% bounce%s",
			FirstFillArmedPrefix, arm, gates.FormatPriceLevel(trade, anchor), tolerance, firstFillInverseSuffix)
	}
	return fmt.Sprintf("%sbelow %s: low %s, enters on a %s%% bounce",
		FirstFillArmedPrefix, arm, gates.FormatPriceLevel(trade, anchor), tolerance)
}

// firstFillEnteredMessage is the row of the release through the reference:
// the hold called the wrong direction, the entry goes to market and the
// depth after it arms at double the step (NextDepthDoubled).
func firstFillEnteredMessage(trade aggragates.Trades, levels firstFillLevels, reference float64) string {
	direction := "above"
	if levels.inverse {
		direction = "below"
	}
	return fmt.Sprintf("%s%s the reference %s, next depth arms at double percentage",
		FirstFillEnteredPrefix, direction, gates.FormatPriceLevel(trade, reference))
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
