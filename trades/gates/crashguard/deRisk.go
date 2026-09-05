package crashguard

import (
	"fmt"
	"strings"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// DeRiskMinDepth is how many filled entries make a trade "deep".
// Deep trades are the ones a flush traps: the crash guard parks further
// capital and widens fallback rungs. It does not flatten — sellLoss is
// Smart Take Loss only. Run 90's HBAR/LINK bags doubled through CLEAR
// windows from fill 5 onward; parking from 4 leaves the cheap rungs and
// blocks the rungs that held ~88% of the quantity.
const DeRiskMinDepth = 4

const (
	// ArmedPrefix / ClearedPrefix are the trade-log prefixes engines
	// persist on an ARM/CLEAR edge so sticky crash can survive a sophos
	// CLEAR while 4h is still against the trade.
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
// not forget it — sticky hold lasts until 4h reclaims.
func TradeHasCrashArmed(trade aggragates.Trades) bool {
	for _, entry := range trade.Logs {
		if strings.HasPrefix(entry.Message, ArmedPrefix) {
			return true
		}
	}
	return false
}
