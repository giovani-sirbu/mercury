package firstfill

import "time"

// firstFillExpired is whether the hold has stood FirstFillMaxHold since its first activated event.
// It fails closed: an unknown clock keeps the hold, so an engine that does not stamp its events
// cannot switch the gate off silently. A zero FirstFillMaxHold disables the cap.
func firstFillExpired(state firstFillRecord, now time.Time) bool {
	if FirstFillMaxHold <= 0 || !state.activated {
		return false
	}
	if state.activatedAt.IsZero() || now.IsZero() {
		return false
	}
	return now.UTC().Sub(state.activatedAt.UTC()) >= FirstFillMaxHold
}
