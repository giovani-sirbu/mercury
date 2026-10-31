package smartsell

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/dynamicparams"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// dynamicParamsLatch is the finding a ladder that opened raised (dynamicparams.RaisedSettings)
// latches the early take profit on, held or not; the reasons name the amounts the opened event carries.
func dynamicParamsLatch(trade aggragates.Trades) (Finding, bool) {
	if !earlyTakeProfitDynamicParams {
		return Finding{}, false
	}
	if _, raised := dynamicparams.RaisedSettings(trade); !raised {
		return Finding{}, false
	}
	points, depths, _ := dynamicparams.OpenedRaise(trade)
	return Finding{Feature: FeatureDynamicParams, Reasons: []string{"opened raised by " + dynamicparams.AmountsOf(points, depths)}}, true
}
