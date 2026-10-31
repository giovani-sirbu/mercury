package smartsell

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// The SmartSell flag's gates as its strategy events name them (aggragates.TradesStrategyEvents.Gate).
const (
	// GateSlowDecline files every row of the slow decline gate and the shared action's on its trigger.
	GateSlowDecline = "slowDecline"
	// GateCapitalProtection files the capital protection gate's episode rows and the shared action's
	// on its trigger (foldCapitalProtectionEvents).
	GateCapitalProtection = "capitalProtection"
	// GateEntryHold files the first-fill hold; its only kind is gates.EventHeld.
	GateEntryHold = "entryHold"
	// GateFlatMarket files the flat market gate's episode rows and the shared action's on its
	// trigger; its first-fill hold carries no price, so no fold reads it (readEvent).
	GateFlatMarket = "flatMarket"
	// GateFreeFallPrevention files the free fall prevention gate's episode rows and the shared
	// action's on its trigger; its first-fill hold carries no price, so no fold reads it (readEvent).
	GateFreeFallPrevention = "freeFallPrevention"
)

// The gates the slow decline and flat market gates replaced; their events stay on trades and fold
// as they did (legacyStates, foldFlatMarketEvents), nothing writes them now.
const (
	// gateIndecision was the early take profit: its latched kind latches.
	gateIndecision = "indecision"
	// gateSlowPattern was the slow pattern decline, now the swing decline: its pending and
	// cancelled kinds drive a pending state of their own.
	gateSlowPattern = "slowPattern"
	// gateLowInterest and gateNonInterestArea were the flat market's two readings as gates of
	// their own: their episode kinds fold into the flat market's episode.
	gateLowInterest     = "lowInterest"
	gateNonInterestArea = "nonInterestArea"
)

// The kinds of the gates' events (EventData.Event); Price is the newest fill unless said otherwise.
const (
	// EventPending is a ladder going pending from a fill, or a new fill confirming it.
	EventPending = "pending"
	// EventCancelled is a pending ladder's new fill breaking the exit.
	EventCancelled = "cancelled"
	// EventReset is a depth priority hold resetting a pending ladder, or a new depth closing a flat
	// market, free fall prevention or capital protection episode.
	EventReset = "reset"
	// EventSold is the committed sale; Price is the level its chain (SaleActions) places its limit at.
	EventSold = "sold"
	// EventLatched is the early take profit latching a ladder on a row of its own.
	EventLatched = "latched"
	// EventSaleHeld is the shared action holding its step, once per episode and gate, and again by
	// the profit whenever the trigger it is held under moves (EventData.Trigger).
	EventSaleHeld = "saleHeld"
	// EventActivated is the shared action activating the sale, which commits it; State names the
	// state it moved the ladder to (takeProfit or trailingBuy).
	EventActivated = "activated"
	// EventReached is the shared action reading a trigger reached on a print, which a reached print
	// between ActivationConfirmation and ActivationConfirmationWindow later confirms; it carries the trigger.
	EventReached = "reached"
	// EventGapHeld is the committed sale held on a print gapping under the anchor, opening a gap
	// episode that SaleGapWait bounds.
	EventGapHeld = "gapHeld"
	// EventGapClosed is the committed sale's gap closing on a print back over the floor without a sale.
	EventGapClosed = "gapClosed"
	// EventTriggered is the flat market or free fall prevention reading, or capital protection's
	// trend reading, opening an episode.
	EventTriggered = "triggered"
	// EventEnded is legacy: a low interest episode closing on its reading; the fold still reads it as a close.
	EventEnded = "ended"
)

// legacyState is one of the two pending states the legacy gates' events fold into: pending, and
// the price and slice index of its last pending event.
type legacyState struct {
	pending bool
	from    float64
	at      int
}

// legacyStates are the quiet decline's state from the feature-less events of GateSlowDecline and
// the swing decline's from gateSlowPattern's, folded apart; superseded once a featured event made
// the ladder pending or not pending.
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

// foldSwing is the legacy states after one event of gateSlowPattern, which never wrote a reset kind.
func (legacy legacyStates) foldSwing(data EventData, index int) legacyStates {
	if !legacy.superseded {
		legacy.swing = legacy.swing.after(data, index, false)
	}
	return legacy
}

// after is the legacy state after one event: pending from the event's price, cancelled (and reset
// where the gate wrote it) not pending, anything else unchanged.
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

// settle is the state with the legacy states still pending as the gate's own: pending while either
// is, from the most recent pending event's price, on the pending features; superseded adds nothing.
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

// EventData is the document of every event of this flag: the kind, the row's own price (none for a
// hold), the reasons the row names (never read back), the features it is about, in row order, and
// the trigger a held row by the profit was held under or a reached row reached (a pointer, so a zero
// trigger is written), and the state an activated event moved the ladder to; a document without
// features is pre-merge and folds as the gate it was written under.
type EventData struct {
	Event    string   `json:"event"`
	Price    float64  `json:"price,omitempty"`
	Reasons  []string `json:"reasons,omitempty"`
	Features []string `json:"features,omitempty"`
	Trigger  *float64 `json:"trigger,omitempty"`
	State    string   `json:"state,omitempty"`
}

// decodedEvents remembers the decoded smartSell events: a trade's few
// stored events are read again on every print of the tape.
var decodedEvents aggragates.EventMemo[EventData]

// readEvent is the event's document when a fold can use it: it decodes and carries a price above
// zero, or is the latch, which a ladder latched before its first fill writes at none.
func readEvent(event aggragates.TradesStrategyEvents) (EventData, bool) {
	data, err := decodedEvents.Decode(event)
	if err != nil {
		return EventData{}, false
	}
	if data.Price <= 0 && data.Event != EventLatched {
		return EventData{}, false
	}
	return data, true
}
