package dynamicparams

import (
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The gate and the kind of the strategy event a ladder's opening writes
// (aggragates.TradesStrategyEvents.Gate, and the "event" key of its document).
// The flag has one gate and that gate one kind.
const (
	GateOpened  = "opened"
	EventOpened = "opened"
)

// OpenedEvent is the document of the opened event: the amounts the ladder
// trades from now on, and nothing else. A part the increase does not name is
// left out of the document and reads back as zero, which adds nothing.
type OpenedEvent struct {
	Event  string  `json:"event"`
	Points float64 `json:"points,omitempty"`
	Depths int     `json:"depths,omitempty"`
}

// Opened is what a ladder opens with: the points added to every row's
// percentage and the depths added to every row's depths, as Opening decides
// them. The engines write it as the opened pair (Rows) on the tick the ladder
// opens; from then on OpenedRaise reads the amounts back from the event.
type Opened struct {
	Points float64
	Depths int
}

// Message is the text of the opened row: OpenedMessage of the amounts.
func (o Opened) Message() string {
	return OpenedMessage(o.Points, o.Depths)
}

// Rows is the opened pair the engines append to the trade, stamped at: the
// INFO row an operator reads, at the price the ladder opens on, and the opened
// event beside it that OpenedRaise reads on every later tick. The row's
// Price is the price the caller names — trade.PositionPrice is zero on a new
// trade, so it would carry no level.
func (o Opened) Rows(trade aggragates.Trades, price float64, at time.Time) (aggragates.TradesLogs, aggragates.TradesStrategyEvents) {
	row := aggragates.TradesLogs{
		TradeID:   trade.ID,
		Message:   o.Message(),
		Type:      aggragates.LOG_INFO,
		Price:     price,
		CreatedAt: at,
		UpdatedAt: at,
	}
	data := OpenedEvent{Event: EventOpened, Points: o.Points, Depths: o.Depths}
	event := aggragates.NewStrategyEvent(trade.ID, aggragates.StrategyParamDynamicParams, GateOpened, data, at)

	return row, event
}
