package smarttakeloss

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/regime"
)

// HTFFreezeReason is the add-freeze hold. The crash guard's capitulation
// override keeps it by its "smart-take-loss: HTF" prefix; crashguard cannot
// import this package, so actions.TestHoldReasonContractAcrossFamilies pins
// the two texts together.
const HTFFreezeReason = "smart-take-loss: HTF continuation, no add"

// AddFreeze is the SmartTakeLoss flag's hold: it parks stopLoss → buy (no
// new capital) on a long that is already in the STL arming zone while HTF
// continuation agrees down and reversal evidence is too weak to justify
// another rung. Inverse is exempt: a dump is their harvest. Never freeze on
// every ARM.
func AddFreeze(event events.Events, position string, ai aggragates.AIIndicators) string {
	trade := event.Trade
	// stopLoss only, like its sibling gates: a "no new capital" freeze must
	// never defer a profitable close (takeProfit), least of all on a
	// fund-blocked chain whose close is the capital the ladder waits for.
	if position != "stopLoss" {
		return ""
	}
	if !trade.Strategy.Params.SmartTakeLoss || trade.Inverse {
		return ""
	}
	if !tradeInZone(trade) {
		return ""
	}
	if ai.DownContinuationRisk < RiskThreshold {
		return ""
	}
	if ai.ReversalUpEvidence >= MinReversalEvidence {
		return ""
	}
	switch ai.Regimes["4h"] {
	case regime.DownPersist, regime.Shock, regime.ShockDown:
		return HTFFreezeReason
	}
	return ""
}
