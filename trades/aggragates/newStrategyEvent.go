package aggragates

import (
	"encoding/json"
	"time"
)

// emptyStrategyEventData is the document an event carries when its data has
// none to give: a fold decodes it to the zero value, whose kind is unknown, so
// the event is inert.
const emptyStrategyEventData = "{}"

// NewStrategyEvent is the event of one gate execution: the data marshalled as
// the event's document, stamped at. The one function every writer and every
// fixture builds an event with, so they cannot drift apart.
//
// It is total. The only marshal error a gate's data can raise is a NaN or an
// infinite float, and the event then carries an empty document instead of
// none, so Data is never nil. The same goes for data that marshals to null.
// The fold that reads such an event skips it, as it skips a row without a
// price. A zero `at` is kept: an event without a stamp never expires a first
// fill and never holds a depth priority, as a row without one does not.
func NewStrategyEvent(tradeID uint, param, gate string, data any, at time.Time) TradesStrategyEvents {
	document, err := json.Marshal(data)
	if err != nil || string(document) == "null" {
		document = []byte(emptyStrategyEventData)
	}

	return TradesStrategyEvents{
		TradeID:   tradeID,
		Param:     param,
		Gate:      gate,
		Data:      document,
		CreatedAt: at,
	}
}

// DecodeData reads the event's document into v. An event with no document
// leaves v as it was, the zero value a caller declared, and no error.
func (e TradesStrategyEvents) DecodeData(v any) error {
	if len(e.Data) == 0 {
		return nil
	}

	return json.Unmarshal(e.Data, v)
}

// Kind is the event's kind: the "event" key of its document, the empty string
// when the document has none or cannot be read. Every gate names its kinds
// beside its data struct.
func (e TradesStrategyEvents) Kind() string {
	var document struct {
		Event string `json:"event"`
	}
	if err := e.DecodeData(&document); err != nil {
		return ""
	}

	return document.Event
}
