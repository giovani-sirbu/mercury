package smarttakeloss

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// Position is the forced position on a HIGH verdict, chosen by TrailingExit.
// The caller owns the SmartTakeLoss flag check and must run the result on
// rails that know the returned state.
func Position(trade aggragates.Trades, price float64, ai aggragates.AIIndicators) (string, bool) {
	if _, high := Evaluate(trade, price, ai); !high {
		return "", false
	}
	if TrailingExit {
		return "update_smartTakeLoss", true
	}
	return "sellLoss", true
}

// InStates is true while the trade is already on an STL or sellLoss rail,
// so the dispatcher must not re-force the entry.
func InStates(positionType string) bool {
	return positionType == "smartTakeLoss" ||
		positionType == "smartTakeLossTrail" ||
		positionType == "sellLoss"
}

// InTrailStates is true while the trade is riding the STL bounce (not yet
// on sellLoss), so the C.5 / 14-day policy still applies.
func InTrailStates(positionType string) bool {
	return positionType == "smartTakeLoss" || positionType == "smartTakeLossTrail"
}
