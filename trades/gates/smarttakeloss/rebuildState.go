package smarttakeloss

import (
	"slices"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// state is what the trade's own strategy events say about the smart take
// loss: the entry fills, where the ladder stands with the slow decline gate —
// whether it is pending, and on which decline features — whether capital
// protection watches it, and whether the indecision direction latched it. It is
// rebuilt from trade.StrategyEvents and trade.History on every tick, the way
// cooldown.firstFillState rebuilds the first-fill gate: the events are the only
// state. Nothing is kept in Redis, in a column or on trade.PositionPrice, and
// the log rows beside the events are output that nothing reads back.
type state struct {
	fills []entryFill
	// quietDeclineWatched and swingDeclineWatched: the quiet decline and the
	// swing decline watch this ladder (quietDeclineWatched,
	// swingDeclineWatched). slowDeclineWatched: either does, which is what
	// watches the ladder for the gate's pending machinery (slowDeclineWatched).
	quietDeclineWatched bool
	swingDeclineWatched bool
	slowDeclineWatched  bool
	// slowDeclinePending: the gate's last pending event of a watched ladder is
	// not followed by a cancelled or reset one, so Apply reads its band.
	// slowDeclinePendingFrom is that event's Price: the fill the ladder is
	// pending from, the one slowDeclineFillUnjudged compares its newest fill
	// with, zero while not pending. pendingFeatures are the decline features the
	// ladder is pending on, in the order the rows name them.
	slowDeclinePending     bool
	slowDeclinePendingFrom float64
	pendingFeatures        []string
	// saleHeld: the loss cap already held the sale at the band in this pending
	// episode (EventSaleHeld), so the held sale's row is not due again. Pending (a
	// confirmation at a new fill included), cancelled, reset and sold events clear
	// it, and nothing else does: the fill that takes the ladder to its last depth
	// is never judged and leaves it as it was.
	saleHeld bool
	// capitalProtectionWatched: the capital protection exit watches this
	// ladder (capitalProtectionWatched).
	capitalProtectionWatched bool
	// indecisionWatched: the indecision direction watches this ladder
	// (indecisionWatched). indecision: a watched ladder carries an event that
	// latched it; no event takes the latch away, and it holds until the trade
	// closes.
	indecisionWatched bool
	indecision        bool
	// depthPriorityHeld: a depth priority holds this ladder
	// (depthPriorityHeld), which pauses the slow decline gate and capital
	// protection on it until its next fill; the sophos latch goes on.
	depthPriorityHeld bool
}

// lastFill is the newest entry fill in slice order, zero when none filled.
func (st state) lastFill() entryFill {
	if len(st.fills) == 0 {
		return entryFill{}
	}
	return st.fills[len(st.fills)-1]
}

// pendingOn is the state with the ladder pending from its newest fill on the
// decline features of the findings, and no sale held for it yet.
func (st state) pendingOn(findings []Finding) state {
	st.slowDeclinePending = true
	st.slowDeclinePendingFrom = st.lastFill().Price
	st.saleHeld = false
	st.pendingFeatures = nil
	for _, finding := range findings {
		if finding.Feature != FeatureIndecision {
			st.pendingFeatures = append(st.pendingFeatures, finding.Feature)
		}
	}
	return st
}

// notPending is the state with the ladder watched and not pending: where a
// cancelled or a reset event leaves it, with no sale held either.
func (st state) notPending() state {
	st.slowDeclinePending = false
	st.slowDeclinePendingFrom = 0
	st.saleHeld = false
	st.pendingFeatures = nil
	return st
}

// rebuildState folds the trade's smartTakeLoss events in slice order — hermes
// loads them in id order, sisyphus appends them — never their stamps. An event
// of the slow decline gate that names features is read by them: a pending event
// makes the ladder pending from the fill its price names, on the decline
// features it names that watch the ladder; a cancelled or reset event makes it
// not pending, whichever feature made it so; FeatureIndecision on a pending,
// cancelled, reset or latched event latches a ladder the indecision direction
// watches, and nothing takes the latch away; a sale held event remembers that
// the loss cap held the episode's sale (saleHeld), whatever fill its row
// carries, and a sold one forgets it, as pending, cancelled and reset do,
// changing nothing else. A feature a switch has turned off is ignored with its
// events, as is the whole fold of a ladder nothing watches. The depth priority
// hold is read apart, on every trade, off the cooldown events' stamps and the
// newest fill (depthPriorityHeld).
//
// Events written before the merge are read as the gates they were written under
// (legacyStates): a feature-less event of the slow decline gate drives the
// pending state of the quiet decline, an event of gateSlowPattern the one of the
// swing decline, a latched event of gateIndecision latches; the first event
// that names features and makes a ladder pending, or not pending, supersedes
// those states and the events after it are the gate's own.
//
// An event the fold cannot use is skipped, and the fold goes on: a document
// that does not decode, a kind the gate does not have — a `{}` event names
// none — and a price at or under zero, since a fill carries a real one. Every
// other event is ignored: the other gates' events (capital protection's sale,
// the entry hold) and the other params'. The log rows are never read.
//
// Every watch is read off the fills already folded here: entryFills counts
// them the way ladder.CountFilledEntries does, row for row, so they agree
// with quietDeclineWatched, swingDeclineWatched, capitalProtectionWatched and
// indecisionWatched exactly.
func rebuildState(trade aggragates.Trades) state {
	fills := entryFills(trade)
	st := state{
		fills:                    fills,
		quietDeclineWatched:      quietDecline && !trade.Inverse && len(fills) >= QuietDeclineArmDepth,
		swingDeclineWatched:      swingDeclineEligible(trade) && len(fills) >= SwingDeclineArmDepth,
		capitalProtectionWatched: capitalProtectionEligible(trade) && lastDepthFilled(trade, len(fills)),
		indecisionWatched:        indecisionEligible(trade) && len(fills) >= IndecisionArmDepth,
	}
	st.slowDeclineWatched = st.quietDeclineWatched || st.swingDeclineWatched
	st.depthPriorityHeld = depthPriorityHeld(trade, st.lastFill())
	if !st.slowDeclineWatched && !st.indecisionWatched {
		return st
	}
	var legacy legacyStates
	for index, event := range trade.StrategyEvents {
		if event.Param != aggragates.StrategyParamSmartTakeLoss {
			continue
		}
		data, ok := readEvent(event)
		if !ok {
			continue
		}
		switch {
		case event.Gate == GateSlowDecline && len(data.Features) > 0:
			st, legacy = foldGateEvent(st, legacy, data)
		case event.Gate == GateSlowDecline && st.quietDeclineWatched:
			legacy = legacy.foldQuiet(data, index)
		case event.Gate == gateSlowPattern && st.swingDeclineWatched:
			legacy = legacy.foldSwing(data, index)
		case event.Gate == gateIndecision && st.indecisionWatched && data.Event == EventLatched:
			st.indecision = true
		}
	}
	return legacy.settle(st)
}

// foldGateEvent is the state after one event of the slow decline gate that
// names features: pending makes the ladder pending from the price it carries on
// the decline features it names that watch the ladder — on none of them, not
// pending: the event stood for features that are switched off — cancelled and
// reset make it not pending, either supersedes the legacy states, the sale
// kinds move the held sale alone (afterSale), and every other kind changes
// nothing but the latch, which FeatureIndecision sets on the kinds that write it.
func foldGateEvent(st state, legacy legacyStates, data EventData) (state, legacyStates) {
	switch data.Event {
	case EventPending:
		st = st.notPending()
		if declines := watchedDeclines(st, data.Features); len(declines) > 0 {
			st.slowDeclinePending, st.slowDeclinePendingFrom, st.pendingFeatures = true, data.Price, declines
		}
		legacy = legacyStates{superseded: true}
	case EventCancelled, EventReset:
		st = st.notPending()
		legacy = legacyStates{superseded: true}
	case EventSaleHeld, EventSold:
		return st.afterSale(data), legacy
	case EventLatched:
	default:
		return st, legacy
	}
	if st.indecisionWatched && slices.Contains(data.Features, FeatureIndecision) {
		st.indecision = true
	}
	return st, legacy
}

// watchedDeclines are the decline features among those an event names that
// watch the ladder, in the order the rows name them.
func watchedDeclines(st state, features []string) []string {
	var declines []string
	if st.quietDeclineWatched && slices.Contains(features, FeatureQuietDecline) {
		declines = append(declines, FeatureQuietDecline)
	}
	if st.swingDeclineWatched && slices.Contains(features, FeatureSwingDecline) {
		declines = append(declines, FeatureSwingDecline)
	}
	return declines
}
