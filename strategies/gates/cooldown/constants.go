package cooldown

// The Cooldown flag's gates as its strategy events name them (aggragates.TradesStrategyEvents.Gate).
const (
	GateFirstFill     = "firstFill"
	GateDepthPriority = "depthPriority"
	GateDepthSpacing  = "depthSpacing"
)

// The kinds of the first-fill gate's events: activated at a reference, armed on an anchor, entered
// when the price ran through the reference, verdictHeld for a futures entry.
const (
	FirstFillActivated   = "activated"
	FirstFillArmed       = "armed"
	FirstFillEntered     = "entered"
	FirstFillVerdictHeld = "verdictHeld"
)

// The prefixes of the first-fill gate's rows: byte-stable text for cp and the notification filter,
// never read back as state (that is FirstFillEvent).
const (
	// FirstFillWaitingPrefix opens the row written while the entry waits at the reference.
	FirstFillWaitingPrefix = "cooldown: trying to get a better entry price: reference "
	// FirstFillArmedPrefix opens every armed row; the anchor in the text makes a new low a new row.
	FirstFillArmedPrefix = "cooldown: trying to get a better entry price, armed "
	// FirstFillEnteredPrefix opens the INFO row of the entered event; the next word is the direction.
	FirstFillEnteredPrefix = "cooldown: entered "
)

// DepthPriorityHoldMarker opens every depth priority hold row (depthPriorityHoldMessage); the gate's
// record is the DepthPriorityEvent beside it.
const DepthPriorityHoldMarker = "cooldown: depth priority"
