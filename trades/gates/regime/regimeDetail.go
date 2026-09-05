package regime

import (
	"strings"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// regimeDetail names the timeframes that carried a long add veto, for the
// trade log: sophos' addAllowed is false exactly when one of
// longAddVetoTimeframes reads downtrend-persist, so those are named — all of
// them — instead of the old first-match scan that could print the wrong
// timeframe, or a shock-up, as the reason a long add was held.
func regimeDetail(ai aggragates.AIIndicators) string {
	var blockers []string
	for _, timeframe := range longAddVetoTimeframes {
		if ai.Regimes[timeframe] == DownPersist {
			blockers = append(blockers, timeframe)
		}
	}
	if len(blockers) > 0 {
		return strings.Join(blockers, "+") + " " + DownPersist
	}
	// addAllowed false for a reason the labels do not carry (a sophos shock
	// knob, an older payload): report the headline rather than inventing one.
	return ai.Regime
}
