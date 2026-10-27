package smarttakeloss

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// rowKind is the kind of row the gate writes on a tick, which fixes the head of
// its message (buildRow) and the kind of its event. The zero value is no row.
type rowKind int

const (
	rowNone rowKind = iota
	// rowTriggered is the ladder going pending.
	rowTriggered
	// rowConfirmed is a pending ladder's new fill confirmed.
	rowConfirmed
	// rowCancelled is a pending ladder's new fill cancelled.
	rowCancelled
	// rowReset is a depth priority hold resetting a pending ladder.
	rowReset
	// rowLatched is the early take profit latching the ladder on a tick that wrote no other row.
	rowLatched
)

// event is the kind of the event beside a row of this kind; a confirmation shares the pending kind.
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

// tick is what the gate found on one tick before it becomes the tick's one row.
type tick struct {
	kind     rowKind
	findings []Finding
}

// names is whether the tick's row is a trigger or a confirmation naming the feature.
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

// slowDeclineWatched is whether the gate watches a trade: any decline does.
func slowDeclineWatched(trade aggragates.Trades) bool {
	return quietDeclineWatched(trade) || swingDeclineWatched(trade) || linearDeclineWatched(trade)
}

// slowDeclineTick is the gate's one row on a tick, if any, and the state it leaves: a held ladder is
// reset (slowDeclineReset), a pending one judged on its newest fill (judgeTheNewestFill), a watched
// one with no other gate's episode open read for going pending (goesPending), and the early take profit
// latch joins or writes the row last.
func slowDeclineTick(trade aggragates.Trades, st state, block aggragates.SmartTakeLossIndicators) (state, *Row) {
	var found tick
	switch {
	case st.depthPriorityHeld:
		st, found = slowDeclineReset(st)
	case st.slowDeclinePending:
		st, found = judgeTheNewestFill(trade, st, block)
	case st.slowDeclineWatched && !st.saleEpisodeOpenBesides(GateSlowDecline):
		st, found = goesPending(st, block)
	}
	st, found = latchEarlyTakeProfit(trade, st, block, found)
	if found.kind == rowNone {
		return st, nil
	}
	row := buildRow(found.kind, st.lastFill().Price, found.findings)
	return st, &row
}

// goesPending goes pending from a watched ladder's newest fill on the first decline that triggers
// (firstDecline), one decline at a time.
func goesPending(st state, block aggragates.SmartTakeLossIndicators) (state, tick) {
	finding, goes := firstDecline(st, block)
	if !goes {
		return st, tick{}
	}
	found := tick{kind: rowTriggered, findings: []Finding{finding}}
	return st.pendingOn(found.findings), found
}

// firstDecline is the first decline watching the ladder that triggers: the quiet decline
// (quietDeclineGoesPending), the swing decline (swingDeclineReads), the linear decline (linearDeclineTriggers).
func firstDecline(st state, block aggragates.SmartTakeLossIndicators) (Finding, bool) {
	if st.quietDeclineWatched {
		if reasons, goes := quietDeclineGoesPending(st, block); goes {
			return Finding{Feature: FeatureQuietDecline, Reasons: reasons}, true
		}
	}
	if st.swingDeclineWatched {
		if reading, goes := swingDeclineReads(st, block); goes {
			return Finding{Feature: FeatureSwingDecline, Reasons: reading.Reasons}, true
		}
	}
	if st.linearDeclineWatched {
		if reading, goes := linearDeclineTriggers(st, block); goes {
			return Finding{Feature: FeatureLinearDecline, Reasons: reading.Reasons}, true
		}
	}
	return Finding{}, false
}
