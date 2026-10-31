package dynamicparams

import (
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The gate and the kind of the opened strategy event; the flag has one gate and that gate one kind.
const (
	GateOpened  = "opened"
	EventOpened = "opened"
)

// OpenedEvent is the document of the opened event: the amounts the ladder trades from now on; a
// part the increase did not name reads back as zero.
type OpenedEvent struct {
	Event  string  `json:"event"`
	Points float64 `json:"points,omitempty"`
	Depths int     `json:"depths,omitempty"`
}

// Opened is what a ladder opens with, as Opening decides it: the points added to every row's
// percentage and the depths added to every row's depths.
type Opened struct {
	Points float64
	Depths int
}

// Message is the opened row's text (OpenedMessage).
func (o Opened) Message() string {
	return OpenedMessage(o.Points, o.Depths)
}

// Rows is the opened pair the engines append to the trade, stamped at: the INFO row at price (the
// caller's, a new trade's PositionPrice being zero) and the opened event OpenedRaise reads back.
func (o Opened) Rows(trade aggragates.Trades, price float64, at time.Time) (aggragates.TradesLogs, aggragates.TradesStrategyEvents) {
	row := aggragates.TradesLogs{
		TradeID:    trade.ID,
		Message:    o.Message(),
		MessageKey: aggragates.LogKeyDynamicParamsOpened,
		Type:       aggragates.LOG_INFO,
		Price:      price,
		CreatedAt:  at,
		UpdatedAt:  at,
	}
	data := OpenedEvent{Event: EventOpened, Points: o.Points, Depths: o.Depths}
	event := aggragates.NewStrategyEvent(trade.ID, aggragates.StrategyParamDynamicParams, GateOpened, data, at)

	return row, event
}
