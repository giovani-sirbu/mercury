package aggragates

import (
	"encoding/json"
	"time"
)

// emptyStrategyEventData is the document an event carries when its data has
// none to give: a fold decodes it to the zero value, whose kind is unknown, so
// the event is inert.
const emptyStrategyEventData = "{}"

// NewStrategyEvent is the event of one gate execution, data marshalled as its
// document and stamped at: the one constructor every writer and fixture uses.
// It is total: data that fails to marshal, or marshals to null, gives an
// empty document, never nil, which the folds skip; a zero at is kept.
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
