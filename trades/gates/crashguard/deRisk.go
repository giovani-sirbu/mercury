package crashguard

import (
	"strings"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// DeRiskMinDepth is how many filled entries make a trade "deep".
// Deep trades are the ones a slow decline traps: the crash guard holds their
// further capital. It does not flatten — sellLoss is Smart Take Loss only.
// The threshold sits just below the depth where a trapped ladder starts
// doubling its quantity, so the shallow, cheap fills still trade while the
// fills that carry most of the quantity are the ones held.
const DeRiskMinDepth = 4

const (
	// ArmedPrefix / ClearedPrefix open the trade-log rows the engines write
	// on the edges of the slow-decline verdict, so an operator can read on
	// the trade when the crash guard's holds applied and when they stopped.
	ArmedPrefix   = "Crash guard ARMED"
	ClearedPrefix = "Crash guard CLEARED"
)

// TransitionMessage is the ARMED/CLEARED row body for an edge of the
// slow-decline verdict. Every engine writes this one text, so live and
// replayed trades read the same.
func TransitionMessage(ai aggragates.AIIndicators) string {
	if !ai.SlowDecline {
		return ClearedPrefix + ": slow decline over, back to normal flow (deep-trade holds off)"
	}
	message := ArmedPrefix + ": slow decline (deep-trade holds on)"
	if len(ai.SlowDeclineReasons) > 0 {
		message += " | " + strings.Join(ai.SlowDeclineReasons, "; ")
	}
	return message
}
