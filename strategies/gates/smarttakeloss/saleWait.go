package smarttakeloss

import (
	"slices"
	"time"
)

// saleWaiting is whether the wait after a sale gate's last trigger (WaitAfterTrigger) still holds
// its trigger back from the shared action: the clock known, the trigger stamped at, the gate's sale
// not yet activated (a committed sale trails whatever), and less than the wait passed since at.
func saleWaiting(at time.Time, activated bool, now time.Time) bool {
	if now.IsZero() || at.IsZero() || activated || waitAfterTrigger <= 0 {
		return false
	}
	return now.Sub(at) < waitAfterTrigger
}

// triggerStamp is a sale gate's trigger stamp after its rows of a tick at now: now when they hold
// its trigger event (kind), else stamp.
func triggerStamp(rows []Row, kind string, stamp, now time.Time) time.Time {
	triggered := slices.ContainsFunc(rows, func(row Row) bool {
		return row.Event == kind
	})
	if triggered {
		return now
	}
	return stamp
}

// saleWaitReasons is a copy of reasons with the wait the sale keeps from the trigger
// (WaitAfterTrigger) last, while one stands.
func saleWaitReasons(reasons []string) []string {
	reasons = slices.Clone(reasons)
	if waitAfterTrigger > 0 {
		reasons = append(reasons, "the sale waits "+waitAfterTrigger.String()+" from the trigger")
	}
	return reasons
}
