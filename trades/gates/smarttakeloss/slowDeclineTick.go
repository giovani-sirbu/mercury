package smarttakeloss

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// rowKind is the kind of row the gate writes on a tick, which fixes the head of
// its message (buildRow) and the kind of its event. The zero value is no row.
type rowKind int

const (
	rowNone rowKind = iota
	// rowTriggered: the ladder went pending, the indecision latch among the
	// features when it landed on the same row.
	rowTriggered
	// rowConfirmed: a pending ladder's new fill was judged and confirmed.
	rowConfirmed
	// rowCancelled: a pending ladder's new fill was judged and every reading that
	// applied missed.
	rowCancelled
	// rowReset: a depth priority hold took a pending ladder's exit away.
	rowReset
	// rowLatched: the indecision direction latched the ladder, on a tick that
	// wrote no other row.
	rowLatched
)

// event is the kind of the event beside a row of this kind: the confirmation
// shares the pending kind with the trigger.
func (kind rowKind) event() string {
	switch kind {
	case rowCancelled:
		return EventCancelled
	case rowReset:
		return EventReset
	case rowLatched:
		return EventLatched
	}
	return EventPending
}

// tick is what the gate found on one tick before it becomes the tick's one row:
// the kind of row, rowNone for none, and the findings that fill it.
type tick struct {
	kind     rowKind
	findings []Finding
}

// names is whether the tick's row is a trigger or a confirmation naming the
// feature: the feature is a subject of the row.
func (found tick) names(feature string) bool {
	if found.kind != rowTriggered && found.kind != rowConfirmed {
		return false
	}
	for _, finding := range found.findings {
		if finding.Feature == feature {
			return true
		}
	}
	return false
}

// slowDeclineWatched is whether the slow decline gate watches a trade: the
// quiet decline or the swing decline does. The indecision direction watches on
// its own account (indecisionWatched). The flag and the parent check are
// Armed's and Apply's.
func slowDeclineWatched(trade aggragates.Trades) bool {
	return quietDeclineWatched(trade) || swingDeclineWatched(trade)
}

// slowDeclineTick is the one row the gate hands back on a tick, if any, and the
// state as that row leaves it. Whatever the gate does on a tick — going pending,
// judging a new fill, resetting, latching — lands on that one row
// (buildRow), and one event beside it.
//
// A ladder a depth priority holds is paused: a pending gate is reset
// (slowDeclineReset), and nothing is triggered or judged. A pending ladder is
// judged on its unjudged newest fill (judgeTheNewestFill). A ladder not pending
// goes pending on the features that trigger, both read on every such tick
// (goesPending). The indecision latch comes last (latchIndecision) and joins
// the row, which it alone may write. The reading of the sophos latch is the one
// thing a hold does not pause.
func slowDeclineTick(trade aggragates.Trades, st state, block aggragates.SmartTakeLossIndicators) (state, *Row) {
	var found tick
	switch {
	case st.depthPriorityHeld:
		st, found = slowDeclineReset(st)
	case st.slowDeclinePending:
		st, found = judgeTheNewestFill(trade, st, block)
	case st.slowDeclineWatched:
		st, found = goesPending(st, block)
	}
	st, found = latchIndecision(trade, st, block, found)
	if found.kind == rowNone {
		return st, nil
	}
	row := buildRow(found.kind, st.lastFill().Price, found.findings)
	return st, &row
}

// goesPending is the tick a watched ladder the gate has not made pending —
// never yet, or since a cancelled or reset event — goes pending: the quiet
// decline (quietDeclineGoesPending) and the swing decline (swingDeclineReads)
// are both read, and the ladder is pending from its newest fill on whichever
// triggers, the row naming each that did.
func goesPending(st state, block aggragates.SmartTakeLossIndicators) (state, tick) {
	var found tick
	if reasons, goes := quietDeclineGoesPending(st, block); goes {
		found.findings = append(found.findings, Finding{Feature: FeatureQuietDecline, Reasons: reasons})
	}
	if reading, goes := swingDeclineReads(st, block); goes {
		found.findings = append(found.findings, Finding{Feature: FeatureSwingDecline, Reasons: reading.Reasons})
	}
	if len(found.findings) == 0 {
		return st, tick{}
	}
	found.kind = rowTriggered
	return st.pendingOn(found.findings), found
}
