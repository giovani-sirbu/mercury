package smarttakeloss

import (
	"fmt"
	"strings"
)

// ArmedMessage is the once-per-trade ARMED log body.
func ArmedMessage(eval Evaluation, blocked bool) string {
	if blocked {
		return fmt.Sprintf("%s: fund-blocked at depth %d, watching continuation risk",
			ArmedPrefix, eval.FilledEntries)
	}
	return fmt.Sprintf("%s: depth %d/%d, watching continuation risk",
		ArmedPrefix, eval.FilledEntries, eval.MaxDepths)
}

// TriggeredMessage is the forced-exit log body.
func TriggeredMessage(eval Evaluation) string {
	// A stale-bag cut is a forced exit too, but it read no lens: it gets
	// its own prefix so it can neither pose as a risk verdict nor start the
	// TRIGGERED-keyed emergency clock.
	if eval.StaleCut {
		return staleCutMessage(eval)
	}
	var message string
	if eval.ProfitNow {
		message = fmt.Sprintf("%s: closing at profit under block risk %.0f",
			TriggeredPrefix, eval.Risk)
	} else {
		message = fmt.Sprintf("%s: risk %.0f, est blocked %.0f days, required recovery %.2f%%",
			TriggeredPrefix, eval.Risk, eval.EstBlockedDays, eval.RequiredRecoveryPct)
	}
	if len(eval.Reasons) > 0 {
		message += " | " + strings.Join(eval.Reasons, "; ")
	}
	return message
}
