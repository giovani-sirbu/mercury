package firstfill

import (
	"github.com/giovani-sirbu/mercury/strategies/gates"
	"github.com/giovani-sirbu/mercury/strategies/gates/cooldown"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// firstFillWaitingHold is the hold at the reference: an activated event with the tick and the reference.
func firstFillWaitingHold(trade aggragates.Trades, levels firstFillLevels, tick, reference float64) gates.Hold {
	data := cooldown.FirstFillEvent{
		Event:     cooldown.FirstFillActivated,
		Price:     tick,
		Reference: reference,
		Inverse:   levels.inverse,
	}

	return firstFillRefusal(data, firstFillWaitingMessage(trade, levels, data), aggragates.LogKeyHoldFirstFill)
}

// firstFillArmedHold is the hold of an armed entry: an armed event with the tick, the reference and the anchor.
func firstFillArmedHold(trade aggragates.Trades, levels firstFillLevels, tick, reference, anchor float64) gates.Hold {
	data := cooldown.FirstFillEvent{
		Event:     cooldown.FirstFillArmed,
		Price:     tick,
		Reference: reference,
		Anchor:    anchor,
		Inverse:   levels.inverse,
	}

	return firstFillRefusal(data, firstFillArmedMessage(trade, levels, data), aggragates.LogKeyHoldFirstFill)
}

// firstFillRefusal is the gate's refusal: the row's text, its key and the event beside it.
func firstFillRefusal(data cooldown.FirstFillEvent, message, key string) gates.Hold {
	return gates.Hold{
		Reason: message,
		Key:    key,
		Param:  aggragates.StrategyParamCooldown,
		Gate:   cooldown.GateFirstFill,
		Data:   data,
	}
}
