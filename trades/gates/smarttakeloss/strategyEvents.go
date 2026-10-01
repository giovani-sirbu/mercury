package smarttakeloss

// The SmartTakeLoss flag's gates as its strategy events name them
// (aggragates.TradesStrategyEvents.Gate). Every execution of one writes an
// event beside its log row, the ones no fold reads back included, so the
// trade's record of the flag is whole.
const (
	// GateSlowDecline is the quiet slow-decline exit: a ladder going pending,
	// the judgement of its new fill, the reset a depth priority hold forces,
	// and the sale at the sell band.
	GateSlowDecline = "slowDecline"
	// GateCapitalProtection is the capital protection exit. It keeps no state:
	// its only event is the sale at the upper band.
	GateCapitalProtection = "capitalProtection"
	// GateIndecision is the indecision direction: the ladder latched.
	GateIndecision = "indecision"
	// GateEntryHold is the first-fill hold while the verdict stands. Its only
	// kind is gates.EventHeld.
	GateEntryHold = "entryHold"
	// GateSlowPattern is the slow pattern decline: a ladder going pending on
	// the shape of its decline between its fills, a new fill confirming or
	// cancelling it, and the sale at the sell band. It has no reset kind — a
	// depth priority hold neither resets nor cancels it — and its own fold
	// (foldSlowPattern), apart from the quiet slow decline's.
	GateSlowPattern = "slowPattern"
)

// The kinds of the events of the gates above, the "event" key of their
// document (EventData.Event).
const (
	// EventPending: a ladder goes pending from a fill, or a new fill confirms
	// a pending one, on the slow decline's gate and the slow pattern's. Price
	// is that fill.
	EventPending = "pending"
	// EventCancelled: the judgement of a pending ladder's new fill broke the
	// exit, on the slow decline's gate and the slow pattern's. Price is that
	// fill.
	EventCancelled = "cancelled"
	// EventReset: a depth priority hold took a pending ladder's exit away.
	// Price is the newest fill.
	EventReset = "reset"
	// EventSold: the exit forced the sale, on the slow decline's gate, the slow
	// pattern's or capital protection's. Price is the level the sellLoss chain
	// places its limit at.
	EventSold = "sold"
	// EventLatched: the indecision direction latched a ladder. Price is the
	// newest fill.
	EventLatched = "latched"
)

// EventData is the document of every event of this flag: the kind, the price
// the fold reads back, and the reasons the row beside it names.
//
// Price is the row's own price, never trade.PositionPrice: the newest fill for
// the events that read a fill, the level of the sale for EventSold, and none
// for a hold. The pending event's price is what slowDeclineFillUnjudged and
// slowPatternFillUnjudged compare the newest fill with, so it must stay the
// fill's exact price.
// Reasons is the reasons sophos served for the reading, in the order the
// message names them; nothing reads them back.
type EventData struct {
	Event   string   `json:"event"`
	Price   float64  `json:"price,omitempty"`
	Reasons []string `json:"reasons,omitempty"`
}
