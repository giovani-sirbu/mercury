package smarttakeloss

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// The SmartTakeLoss flag's gates as its strategy events name them
// (aggragates.TradesStrategyEvents.Gate). Every execution of one writes an
// event beside its log row, the ones no fold reads back included, so the
// trade's record of the flag is whole.
const (
	// GateSlowDecline is the slow decline gate: a ladder going pending on its
	// quiet decline or its swing decline, the judgement of its new fill, the
	// reset a depth priority hold forces, the latch of the indecision direction,
	// the sale at the sell band and the sale the loss cap holds. Every row of the
	// gate files under it, one event per row.
	GateSlowDecline = "slowDecline"
	// GateCapitalProtection is the capital protection exit. It keeps no state:
	// its only event is the sale at the upper band.
	GateCapitalProtection = "capitalProtection"
	// GateEntryHold is the first-fill hold while the verdict stands. Its only
	// kind is gates.EventHeld.
	GateEntryHold = "entryHold"
)

// The gates the slow decline gate replaced. Events written under them before
// the merge stay on trades, and the fold reads them as it did then (the legacy
// states of rebuildState) without ever rewriting them; nothing writes them now.
const (
	// gateIndecision was the indecision direction: its latched kind latches.
	gateIndecision = "indecision"
	// gateSlowPattern was the slow pattern decline, now the swing decline: its
	// pending and cancelled kinds drive a pending state of their own.
	gateSlowPattern = "slowPattern"
)

// The kinds of the events of the gates above, the "event" key of their
// document (EventData.Event).
const (
	// EventPending: a ladder goes pending from a fill, or a new fill confirms a
	// pending one. Price is that fill.
	EventPending = "pending"
	// EventCancelled: the judgement of a pending ladder's new fill broke the
	// exit. Price is that fill.
	EventCancelled = "cancelled"
	// EventReset: a depth priority hold took a pending ladder's exit away.
	// Price is the newest fill.
	EventReset = "reset"
	// EventSold: the exit forced the sale, on the slow decline gate or capital
	// protection's. Price is the level the sellLoss chain places its limit at.
	EventSold = "sold"
	// EventLatched: the indecision direction latched a ladder, on a row of its
	// own. Price is the newest fill.
	EventLatched = "latched"
	// EventSaleHeld: the loss cap held a pending ladder's sale at the sell band
	// (bandSaleLoss), once per pending episode. Price is the newest fill, as on
	// its row. The fold reads that the episode's sale was held (state.saleHeld),
	// never at which fill, so the row is not written again until a pending event
	// (a confirmation at a new fill included), a cancelled, a reset or a sold one
	// clears it: the fill that takes the ladder to its last depth is never judged,
	// so it leaves the sale held as it was.
	EventSaleHeld = "saleHeld"
)

// The features of the slow decline gate as an event names them
// (EventData.Features) and as a row names its findings (Finding.Feature).
const (
	// FeatureIndecision is the indecision direction: it latches a ladder, and an
	// event that carries it latched the ladder on that row.
	FeatureIndecision = "indecision"
	// FeatureQuietDecline is the quiet decline: sophos' verdict, or its leg on
	// and quiet, makes a ladder pending.
	FeatureQuietDecline = "quietDecline"
	// FeatureSwingDecline is the swing decline: the shape of the decline
	// between a ladder's fills, under the SMC trend, makes a ladder pending.
	FeatureSwingDecline = "swingDecline"
)

// legacyState is one of the two pending states the events of those gates fold
// into: whether it is pending, the price of its last pending event and where in
// the slice that event stood.
type legacyState struct {
	pending bool
	from    float64
	at      int
}

// legacyStates are the two states those events drive, folded apart as they were
// before the merge: the quiet decline's by the feature-less events of
// GateSlowDecline, the swing decline's by the events of gateSlowPattern. Either
// can be pending without the other. superseded: an event of the gate that names
// features has made the ladder pending or not pending, so the legacy states are
// over and the later events of the legacy gates are ignored.
type legacyStates struct {
	quiet, swing legacyState
	superseded   bool
}

// foldQuiet is the legacy states after one feature-less event of GateSlowDecline.
func (legacy legacyStates) foldQuiet(data EventData, index int) legacyStates {
	if !legacy.superseded {
		legacy.quiet = legacy.quiet.after(data, index, true)
	}
	return legacy
}

// foldSwing is the legacy states after one event of gateSlowPattern; that gate
// never wrote a reset kind, so a reset is no kind of its own there.
func (legacy legacyStates) foldSwing(data EventData, index int) legacyStates {
	if !legacy.superseded {
		legacy.swing = legacy.swing.after(data, index, false)
	}
	return legacy
}

// after is the legacy state after one event: pending makes it pending from the
// price the event carries, cancelled makes it not pending, and so does reset
// where the gate wrote that kind; every other kind, the sale included, changes
// nothing.
func (current legacyState) after(data EventData, index int, resets bool) legacyState {
	switch data.Event {
	case EventPending:
		return legacyState{pending: true, from: data.Price, at: index}
	case EventCancelled:
		return legacyState{}
	case EventReset:
		if resets {
			return legacyState{}
		}
	}
	return current
}

// settle is the state with the legacy states that are still pending as the
// gate's own: the ladder is pending while either is, from the price of the most
// recent pending event among them, on the decline features whose legacy states
// are pending. A superseded fold has nothing to add.
func (legacy legacyStates) settle(st state) state {
	if legacy.superseded {
		return st
	}
	var features []string
	var recent legacyState
	for _, one := range []struct {
		feature string
		folded  legacyState
	}{{FeatureQuietDecline, legacy.quiet}, {FeatureSwingDecline, legacy.swing}} {
		if !one.folded.pending {
			continue
		}
		features = append(features, one.feature)
		if len(features) == 1 || one.folded.at > recent.at {
			recent = one.folded
		}
	}
	if len(features) > 0 {
		st.slowDeclinePending, st.slowDeclinePendingFrom, st.pendingFeatures = true, recent.from, features
	}
	return st
}

// EventData is the document of every event of this flag: the kind, the price
// the fold reads back, the reasons the row beside it names and the features it
// is about.
//
// Price is the row's own price, never trade.PositionPrice: the newest fill for
// the events that read a fill, the level of the sale for EventSold, and none for
// a hold. The pending event's price is what slowDeclineFillUnjudged compares the
// newest fill with, so it must stay the fill's exact price; the sale held event's
// is read by no fold, which only needs it to be a price above zero (readEvent).
//
// Features are the features the event is about, in the order the rows name them
// (FeatureIndecision, FeatureQuietDecline, FeatureSwingDecline): the decline
// features a pending or a cancelled event stands for or broke, the ones a
// reset, a sold or a sale held event ended or waits on, and FeatureIndecision
// when that row latched the ladder. The fold reads them: a pending event makes
// the ladder pending on its decline features alone, and FeatureIndecision
// latches it. A document without features is one written before the merge, and
// folds as the gate it was written under then (rebuildState). Reasons are the
// reasons the row names, in the order its groups print them; nothing reads them
// back.
type EventData struct {
	Event    string   `json:"event"`
	Price    float64  `json:"price,omitempty"`
	Reasons  []string `json:"reasons,omitempty"`
	Features []string `json:"features,omitempty"`
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
