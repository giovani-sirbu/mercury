package gates

// EventHeld is the kind of the strategy event a hold writes.
const EventHeld = "held"

// Hold is a gate's refusal, handed to tradelog.SaveHoldLog; the zero value lets the chain proceed.
// Param, Gate and Data name the strategy event written beside the row; a hold without a Param
// (usePatterns, useAI) writes the row alone.
type Hold struct {
	// Reason is the row's message after its "Hold <position>: " frame, and the error that stops the chain.
	Reason string
	// Key is the row's messageKey (an aggragates.LogKeyHold constant).
	Key string
	// Param is the aggragates.StrategyParam that owns the gate; empty writes no event.
	Param string
	// Gate is the gate of that param the event is filed under.
	Gate string
	// Data is the gate's event document (aggragates.NewStrategyEvent); its "event" key is the kind.
	Data any
}

// Held reports whether the gate refused.
func (h Hold) Held() bool {
	return h.Reason != ""
}
