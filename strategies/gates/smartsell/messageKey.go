package smartsell

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// keyOf is a log line's messageKey, SMART_SELL_<GATE>_<EVENT> from its gate and event: the
// activation names its path (SMART_SELL_<GATE>_TAKE_PROFIT_ACTIVATED, ..._TAKE_LOSS_ACTIVATED), the
// exit is SMART_SELL_SOLD and the latch SMART_SELL_EARLY_TAKE_PROFIT_LATCHED.
func keyOf(line Row) string {
	switch line.Event {
	case EventSold:
		return aggragates.LogKeySmartSellPrefix + "SOLD"
	case EventLatched:
		return aggragates.LogKeySmartSellPrefix + "EARLY_TAKE_PROFIT_LATCHED"
	case EventActivated:
		return gateKey(line.Gate) + activatedPath(line.State) + "_ACTIVATED"
	}
	return gateKey(line.Gate) + aggragates.UpperSnake(line.Event)
}

// gateKey is the head of a gate's keys, "SMART_SELL_<GATE>_".
func gateKey(gate string) string {
	return aggragates.LogKeySmartSellPrefix + aggragates.UpperSnake(gate) + "_"
}

// activatedPath is the path an activation into state takes, in a key: TAKE_PROFIT into takeProfit, TAKE_LOSS into trailingBuy.
func activatedPath(state string) string {
	if state == aggragates.PositionTakeProfit {
		return "TAKE_PROFIT"
	}
	return "TAKE_LOSS"
}
