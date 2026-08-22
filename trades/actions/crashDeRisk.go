package actions

import (
	"strings"

	"github.com/giovani-sirbu/mercury/trades"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// CrashDeRiskMinDepth is how many filled entries make a trade "deep". Deep
// trades are the ones a flush traps, so they are the only ones the crash
// guard force-closes; shallower trades just get the widened ladder.
const CrashDeRiskMinDepth = 5

// CrashDeRiskPosition decides whether a deep trade should be closed NOW while
// the crash guard is armed. Policy: sell at a profit whenever the estimate
// clears zero; otherwise sell at an accepted loss only when the strategy
// opted into TakeLoss and the loss stays inside the governing row's
// TakeLossPercentage. Anything else answers false — the trade keeps its
// position and the widened ladder keeps working.
//
// The caller owns the CrashActive check and must run the result on a
// sellLoss-capable action chain (the sellLoss rails book a positive net just
// as well — AcceptLoss computes the real P&L either way). Inverse trades
// never de-risk, a flush moves in their favor; impasse children stay with
// their parent.
//
// The estimate prices the full remaining base at the current print and skips
// exchange fees; AcceptLoss recomputes the exact net at execution.
func CrashDeRiskPosition(trade aggragates.Trades, price float64) (string, bool) {
	if trade.Inverse || trade.ParentID != 0 || price <= 0 {
		return "", false
	}
	if len(trade.StrategyPair.StrategySettings) == 0 {
		return "", false
	}
	filledEntries := trades.CountFilledEntries(trade)
	if filledEntries < CrashDeRiskMinDepth {
		return "", false
	}

	profit, invested := estimateCloseProfit(trade, price)
	if invested <= 0 {
		return "", false
	}
	if profit > 0 {
		return "sellLoss", true
	}
	if !trade.Strategy.Params.TakeLoss {
		return "", false
	}
	row := trades.SettingsIndexOrBase(trade.StrategyPair.StrategySettings, filledEntries-1)
	bound := trade.StrategyPair.StrategySettings[row].TakeLossPercentage
	if bound <= 0 {
		return "", false
	}
	if profit/invested*100 >= -bound {
		return "sellLoss", true
	}
	return "", false
}

// estimateCloseProfit simulates selling the trade's remaining base at price
// and returns the quote P&L next to the invested quote, both fee-free.
func estimateCloseProfit(trade aggragates.Trades, price float64) (profit, invested float64) {
	var boughtBase, soldBase float64
	for _, history := range trade.History {
		if strings.ToLower(history.Type) == "buy" {
			boughtBase += history.Quantity
			invested += history.Quantity * history.Price
		} else {
			soldBase += history.Quantity
		}
	}
	remaining := boughtBase - soldBase
	if remaining <= 0 {
		return 0, invested
	}

	simulated := trade
	simulated.History = append(append([]aggragates.TradesHistory(nil), trade.History...), aggragates.TradesHistory{
		Type:     "SELL",
		Quantity: remaining,
		Price:    price,
	})
	return GetProfit(simulated), invested
}
