package cooldown

import "time"

// firstFillExpired reports whether the first-fill hold has stood longer than
// FirstFillMaxHold and must let the entry through.
//
// It fails CLOSED — an unknown clock on either side keeps the hold — which is
// the opposite of the fail-open posture the rest of this package takes, and
// deliberately so. Everywhere else a missing clock means "do not hold", and
// the worst case is a gate that does nothing. Here the missing clock would
// mean "expire", and the worst case is the whole first-fill gate switching
// itself off on any engine that does not stamp its hold rows — a silent
// behaviour change rather than a silent no-op. The conservative direction is
// the one that keeps the gate doing what it was configured to do.
//
// It is measured from the FIRST waiting row, not from the trade's creation:
// the row is stamped with the tick the hold actually activated, it lives on
// the trade like every other fact this gate reads, and trade.CreatedAt means
// different things on different engines.
//
// A zero FirstFillMaxHold disables the cap: the gate is then released by
// price alone, as it was between 2026-09-05 and 2026-09-07.
func firstFillExpired(state firstFillRecord, now time.Time) bool {
	if FirstFillMaxHold <= 0 || !state.activated {
		return false
	}
	if state.activatedAt.IsZero() || now.IsZero() {
		return false
	}
	return now.UTC().Sub(state.activatedAt.UTC()) >= FirstFillMaxHold
}
