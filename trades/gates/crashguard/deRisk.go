package crashguard

import (
	"fmt"
	"strings"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// DeRiskMinDepth is how many filled entries make a trade "deep".
// Deep trades are the ones a flush traps: the crash guard parks further
// capital and widens fallback rungs. It does not flatten — sellLoss is
// Smart Take Loss only. The threshold sits just below the depth where a
// trapped ladder starts doubling its quantity through CLEAR windows, so the
// shallow, cheap fills still trade while the fills that carry most of the
// quantity are the ones parked.
const DeRiskMinDepth = 4

const (
	// ArmedPrefix / ClearedPrefix are the trade-log prefixes engines
	// persist on an ARM/CLEAR edge so sticky crash can survive a sophos
	// CLEAR while the higher timeframe is still against the trade.
	ArmedPrefix   = "Crash guard ARMED"
	ClearedPrefix = "Crash guard CLEARED"
)

// TransitionMessage is the ARM/CLEAR log body. Engines must use this so
// TradeHasCrashArmed can see live and backtest rows the same way.
func TransitionMessage(ai aggragates.AIIndicators) string {
	if !ai.CrashActive {
		return fmt.Sprintf(
			"%s: score %.0f, back to normal flow (deep-trade rules off)",
			ClearedPrefix, ai.CrashScore)
	}
	message := fmt.Sprintf(
		"%s: score %.0f (deep-trade rules on; ladder widening only past explicit rows)",
		ArmedPrefix, ai.CrashScore)
	if len(ai.CrashReasons) > 0 {
		message += " | " + strings.Join(ai.CrashReasons, "; ")
	}
	return message
}

// TradeHasCrashArmed is true once this trade has logged an ARM. CLEAR does
// not forget it — sticky hold lasts until the higher timeframe reclaims.
func TradeHasCrashArmed(trade aggragates.Trades) bool {
	for _, entry := range trade.Logs {
		if strings.HasPrefix(entry.Message, ArmedPrefix) {
			return true
		}
	}
	return false
}
