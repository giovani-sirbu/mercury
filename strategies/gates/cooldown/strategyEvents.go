package cooldown

import (
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// FirstFillEvent is the document of a first-fill event: Price is the tick price of the row it was
// written beside (what firstFillState reads), Reference the activation price, Anchor the extreme an
// armed hold trails, Inverse the side.
type FirstFillEvent struct {
	Event     string  `json:"event"`
	Price     float64 `json:"price,omitempty"`
	Reference float64 `json:"reference,omitempty"`
	Anchor    float64 `json:"anchor,omitempty"`
	Inverse   bool    `json:"inverse,omitempty"`
}

// DepthPriorityEvent is the document of a depth priority hold: the ladder in front and the one that
// waits, each at its depth of its ceiling; its only kind is gates.EventHeld.
type DepthPriorityEvent struct {
	Event            string `json:"event"`
	PrioritySymbol   string `json:"prioritySymbol"`
	PriorityDepth    int    `json:"priorityDepth"`
	PriorityMaxDepth int    `json:"priorityMaxDepth"`
	Depth            int    `json:"depth"`
	MaxDepth         int    `json:"maxDepth"`
}

// DepthSpacingEvent is the document of a depth spacing hold; the fold reads Depth and the stamp
// alone (the first event of a depth is its activation), while Step, Hold and Release format the row.
type DepthSpacingEvent struct {
	Event   string        `json:"event"`
	Depth   int           `json:"depth"`
	Step    int           `json:"step"`
	Hold    time.Duration `json:"hold"`
	Release float64       `json:"release,omitempty"`
}

// NewFirstFillEvent is a first-fill event of the trade, stamped at; production and the fixtures
// build it the same way.
func NewFirstFillEvent(tradeID uint, data FirstFillEvent, at time.Time) aggragates.TradesStrategyEvents {
	return aggragates.NewStrategyEvent(tradeID, aggragates.StrategyParamCooldown, GateFirstFill, data, at)
}

// NewDepthPriorityEvent is a depth priority event of the trade, stamped at.
func NewDepthPriorityEvent(tradeID uint, data DepthPriorityEvent, at time.Time) aggragates.TradesStrategyEvents {
	return aggragates.NewStrategyEvent(tradeID, aggragates.StrategyParamCooldown, GateDepthPriority, data, at)
}

// NewDepthSpacingEvent is a depth spacing event of the trade, stamped at.
func NewDepthSpacingEvent(tradeID uint, data DepthSpacingEvent, at time.Time) aggragates.TradesStrategyEvents {
	return aggragates.NewStrategyEvent(tradeID, aggragates.StrategyParamCooldown, GateDepthSpacing, data, at)
}
