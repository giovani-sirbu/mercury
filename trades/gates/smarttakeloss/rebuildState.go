package smarttakeloss

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// state is what the trade's own strategy events say about the smart take
// loss: the entry fills, where the ladder stands with the quiet slow-decline
// exit, whether capital protection watches it, and where it stands with the
// indecision direction. It is rebuilt from trade.StrategyEvents and
// trade.History on every tick, the way cooldown.firstFillState rebuilds the
// first-fill gate: the events are the only state. Nothing is kept in Redis, in
// a column or on trade.PositionPrice, and the log rows beside the events are
// output that nothing reads back.
type state struct {
	fills []entryFill
	// slowDeclineWatched: the quiet slow-decline exit watches this ladder
	// (slowDeclineWatched). slowDeclinePending: the last slow-decline event of
	// a watched ladder is a pending event, so Apply reads its band; a
	// cancelled or reset event after it takes that away until the next
	// pending event. slowDeclinePendingFrom is that event's Price: the fill
	// the ladder is pending from, the one slowDeclineFillUnjudged compares its
	// newest fill with. Zero while not pending.
	slowDeclineWatched     bool
	slowDeclinePending     bool
	slowDeclinePendingFrom float64
	// capitalProtectionWatched: the capital protection exit watches this
	// ladder (capitalProtectionWatched).
	capitalProtectionWatched bool
	// indecisionWatched: the indecision direction watches this ladder
	// (indecisionWatched). indecision: a watched ladder carries a latched
	// event, so it is latched; no event takes the latch away, and it holds
	// until the trade closes.
	indecisionWatched bool
	indecision        bool
	// depthPriorityHeld: a depth priority holds this ladder
	// (depthPriorityHeld), which pauses the quiet slow-decline exit and capital
	// protection on it until its next fill; the indecision direction goes on.
	depthPriorityHeld bool
}

// lastFill is the newest entry fill in slice order, zero when none filled.
func (st state) lastFill() entryFill {
	if len(st.fills) == 0 {
		return entryFill{}
	}
	return st.fills[len(st.fills)-1]
}

// rebuildState folds the trade's smartTakeLoss events in slice order — hermes
// loads them in id order, sisyphus appends them — never their stamps. A
// slow-decline event makes a watched ladder pending from the fill its price
// names (the pending kind), and a cancelled or reset event makes it not
// pending — the last of them wins, the sold kind changes nothing, and none
// touches a ladder the exit does not watch, so while QuietSlowDeclineExit is
// off every such event is ignored. The depth priority hold is read apart, on
// every trade, off the cooldown events' stamps and the newest fill
// (depthPriorityHeld). A latched event of the indecision gate latches a
// ladder the indecision direction watches, and nothing takes the latch away;
// it touches no ladder that rule does not watch, so while IndecisionDirection
// is off every such event is ignored too. The two folds are independent: a
// ladder one rule watches folds that rule's events whether or not the other
// watches it.
//
// An event the fold cannot use is skipped, and the fold goes on: a document
// that does not decode, a kind the gate does not have — a `{}` event names
// none — and a price at or under zero, since a fill carries a real one. Every
// other event is ignored: the other gates' events (capital protection's sale,
// the entry hold) and the other params'. The log rows are never read.
//
// Every watch is read off the fills already folded here: entryFills counts
// them the way ladder.CountFilledEntries does, row for row, so they agree
// with slowDeclineWatched, capitalProtectionWatched and indecisionWatched
// exactly.
func rebuildState(trade aggragates.Trades) state {
	fills := entryFills(trade)
	st := state{
		fills:                    fills,
		slowDeclineWatched:       quietSlowDeclineExit && !trade.Inverse && len(fills) >= SlowDeclineArmDepth,
		capitalProtectionWatched: capitalProtectionEligible(trade) && lastDepthFilled(trade, len(fills)),
		indecisionWatched:        indecisionEligible(trade) && len(fills) >= IndecisionArmDepth,
	}
	st.depthPriorityHeld = depthPriorityHeld(trade, st.lastFill())
	if !st.slowDeclineWatched && !st.indecisionWatched {
		return st
	}
	for _, event := range trade.StrategyEvents {
		if event.Param != aggragates.StrategyParamSmartTakeLoss {
			continue
		}
		switch {
		case st.slowDeclineWatched && event.Gate == GateSlowDecline:
			st = foldSlowDecline(st, event)
		case st.indecisionWatched && event.Gate == GateIndecision:
			st = foldIndecision(st, event)
		}
	}
	return st
}

// foldSlowDecline is the state after one event of the slow-decline gate:
// pending makes the ladder pending from the price it carries, cancelled and
// reset make it not pending, and every other kind — the sale included —
// changes nothing.
func foldSlowDecline(st state, event aggragates.TradesStrategyEvents) state {
	data, ok := readEvent(event)
	if !ok {
		return st
	}
	switch data.Event {
	case EventPending:
		st.slowDeclinePending = true
		st.slowDeclinePendingFrom = data.Price
	case EventCancelled, EventReset:
		st.slowDeclinePending = false
		st.slowDeclinePendingFrom = 0
	}
	return st
}

// foldIndecision is the state after one event of the indecision gate: latched
// latches the ladder, and nothing else does anything.
func foldIndecision(st state, event aggragates.TradesStrategyEvents) state {
	if data, ok := readEvent(event); ok && data.Event == EventLatched {
		st.indecision = true
	}
	return st
}

// readEvent is the event's document when a fold can use it: it decodes and
// carries a price above zero, as a fill's does.
func readEvent(event aggragates.TradesStrategyEvents) (EventData, bool) {
	var data EventData
	if err := event.DecodeData(&data); err != nil || data.Price <= 0 {
		return EventData{}, false
	}
	return data, true
}
