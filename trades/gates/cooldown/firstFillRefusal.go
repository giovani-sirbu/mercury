package cooldown

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
)

// firstFillWaitingHold is the hold that waits at the reference: an activated
// event carrying the tick the row is written on and the reference its message
// names. The two differ once the hold has stood: a later row of the same wait
// names the reference the first one set.
func firstFillWaitingHold(trade aggragates.Trades, levels firstFillLevels, tick, reference float64) gates.Hold {
	data := FirstFillEvent{
		Event:     FirstFillActivated,
		Price:     tick,
		Reference: reference,
		Inverse:   levels.inverse,
	}

	return firstFillRefusal(data, firstFillWaitingMessage(trade, levels, data))
}

// firstFillArmedHold is the hold of an armed entry: an armed event carrying
// the tick the row is written on, the reference the arm level is priced from
// and the anchor the hold trails.
func firstFillArmedHold(trade aggragates.Trades, levels firstFillLevels, tick, reference, anchor float64) gates.Hold {
	data := FirstFillEvent{
		Event:     FirstFillArmed,
		Price:     tick,
		Reference: reference,
		Anchor:    anchor,
		Inverse:   levels.inverse,
	}

	return firstFillRefusal(data, firstFillArmedMessage(trade, levels, data))
}

// firstFillRefusal is a first-fill gate's refusal: the text of the row and
// the event document that goes beside it.
func firstFillRefusal(data FirstFillEvent, message string) gates.Hold {
	return gates.Hold{
		Reason: message,
		Param:  aggragates.StrategyParamCooldown,
		Gate:   GateFirstFill,
		Data:   data,
	}
}
