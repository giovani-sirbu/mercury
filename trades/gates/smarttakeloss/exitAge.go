package smarttakeloss

import "time"

// minAgeReached reports whether the newest entry fill has stood for
// MinAgeAfterLastFill, the distance every forced exit waits for. The
// reference is the fill's own stamp — a persisted fact — against the tick
// clock the engine passes in.
//
// An unknown clock on either side holds the exit. This gate ACTS by
// replacing the ladder's proposal with a close, so "a gate that cannot
// measure does not act" reads the other way round here than in cooldown's
// depth spacing, which fails open by declining to restrict. And the fill it
// reads is not the one activation vetted: after the activation the newest
// fill is the permitted depth, which never passes an age check of its own,
// so a row that lost its stamp would otherwise sell on the tick it appears —
// the one-second exit this wait exists to prevent. A trade whose newest fill
// carries no stamp simply never takes the forced exit; every ladder close
// still works.
func minAgeReached(st state, now time.Time) bool {
	at := st.lastFill().At
	if at.IsZero() || now.IsZero() {
		return false
	}
	return now.UTC().Sub(at.UTC()) >= MinAgeAfterLastFill
}
