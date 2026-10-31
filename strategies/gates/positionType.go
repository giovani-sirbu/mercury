package gates

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// PositionType maps a force-trailing re-anchor onto the position it re-arms (trailingBuy or
// takeProfit) so every gate keys on it; the state machine and tradelog.SaveHoldLog keep the raw name.
func PositionType(positionType string) string {
	switch positionType {
	case aggragates.PositionForceTrailingBuy:
		return aggragates.PositionTrailingBuy
	case aggragates.PositionForceTrailingTakeProfit:
		return aggragates.PositionTakeProfit
	}
	return positionType
}
