package gates

// EventHeld is the kind of the strategy event a hold writes: the gate said no
// and the trade stayed where it was.
const EventHeld = "held"

// Hold is a gate's refusal, handed to SaveHoldLog. The zero value is no
// refusal: the gate lets the chain proceed.
//
// Param, Gate and Data name the strategy event SaveHoldLog writes beside the
// row, the one a later tick's fold reads. A gate that owns a strategy param
// fills them; a hold without a Param (usePatterns, useAI) writes the row
// alone.
type Hold struct {
	// Reason is the text of the refusal: the row's message after its
	// "Hold <position>: " frame, and the error that stops the chain.
	Reason string
	// Param is the strategy flag that owns the gate, one of the
	// aggragates.StrategyParam constants; empty writes no event.
	Param string
	// Gate is the gate of that flag the event is filed under.
	Gate string
	// Data is the gate's typed event document, marshalled by
	// aggragates.NewStrategyEvent. Its "event" key is the kind, EventHeld for
	// a gate whose hold is its only kind.
	Data any
}

// Held reports whether the gate refused.
func (h Hold) Held() bool {
	return h.Reason != ""
}
