package cooldown

import (
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The Cooldown flag's gates as its strategy events name them
// (aggragates.TradesStrategyEvents.Gate). Every execution of one writes an
// event beside its log row, the ones no fold reads back included, so the
// trade's record of the flag is whole.
const (
	GateFirstFill     = "firstFill"
	GateDepthPriority = "depthPriority"
	GateDepthSpacing  = "depthSpacing"
)

// The kinds of the first-fill gate's events, in the order a hold goes through
// them: it activates at a reference, arms and trails an anchor, or the price
// runs through the reference and the entry goes to market (entered). A futures
// entry has no reference to price: its only event is the verdict that held it.
const (
	FirstFillActivated   = "activated"
	FirstFillArmed       = "armed"
	FirstFillEntered     = "entered"
	FirstFillVerdictHeld = "verdictHeld"
)

// FirstFillEvent is the document of a first-fill event. What the fold reads,
// and what the row's message is formatted from, and nothing else.
//
// Price is the price of the row the event was written beside: the tick the
// hold spoke on. It is what firstFillState reads, never Reference or Anchor,
// because the gate writes a standing hold again once its row is a day old,
// at the tick of that later day, and such an armed event can deepen the anchor
// on the day it is written.
//
// Reference is the price the hold activated at (the level up and arm are
// priced from), Anchor the extreme an armed hold trails, and Inverse the
// direction the hold ran in: a spot short side, which mirrors every level.
type FirstFillEvent struct {
	Event     string  `json:"event"`
	Price     float64 `json:"price,omitempty"`
	Reference float64 `json:"reference,omitempty"`
	Anchor    float64 `json:"anchor,omitempty"`
	Inverse   bool    `json:"inverse,omitempty"`
}

// DepthPriorityEvent is the document of a depth priority hold: the ladder the
// wallet is in front of (PrioritySymbol at PriorityDepth of PriorityMaxDepth)
// and the ladder that waits (Depth of MaxDepth). Its only kind is
// gates.EventHeld.
type DepthPriorityEvent struct {
	Event            string `json:"event"`
	PrioritySymbol   string `json:"prioritySymbol"`
	PriorityDepth    int    `json:"priorityDepth"`
	PriorityMaxDepth int    `json:"priorityMaxDepth"`
	Depth            int    `json:"depth"`
	MaxDepth         int    `json:"maxDepth"`
}

// DepthSpacingEvent is the document of a depth spacing hold: the ladder's
// depth, the escalation Step and the Hold that step earned, and the price that
// lifts the hold early, zero when the ladder's rows could not price one. Its
// only kind is gates.EventHeld.
type DepthSpacingEvent struct {
	Event   string        `json:"event"`
	Depth   int           `json:"depth"`
	Step    int           `json:"step"`
	Hold    time.Duration `json:"hold"`
	Release float64       `json:"release,omitempty"`
}

// NewFirstFillEvent is a first-fill event of the trade, stamped at. Production
// writes its entered event with it, and the fixtures of every repo build the
// state the gate reads back with it, so a fixture is never a hand-made copy of
// what production writes.
func NewFirstFillEvent(tradeID uint, data FirstFillEvent, at time.Time) aggragates.TradesStrategyEvents {
	return aggragates.NewStrategyEvent(tradeID, aggragates.StrategyParamCooldown, GateFirstFill, data, at)
}

// NewDepthPriorityEvent is a depth priority event of the trade, stamped at:
// the event gates.SaveHoldLog writes for the hold DepthPriorityHold returns.
func NewDepthPriorityEvent(tradeID uint, data DepthPriorityEvent, at time.Time) aggragates.TradesStrategyEvents {
	return aggragates.NewStrategyEvent(tradeID, aggragates.StrategyParamCooldown, GateDepthPriority, data, at)
}

// NewDepthSpacingEvent is a depth spacing event of the trade, stamped at: the
// event gates.SaveHoldLog writes for the hold DepthSpacingHold returns.
func NewDepthSpacingEvent(tradeID uint, data DepthSpacingEvent, at time.Time) aggragates.TradesStrategyEvents {
	return aggragates.NewStrategyEvent(tradeID, aggragates.StrategyParamCooldown, GateDepthSpacing, data, at)
}
