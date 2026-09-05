package smarttakeloss

import (
	"fmt"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/crashguard"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// StaleCutPrefix is the trade-log prefix of a stale-bag exit. Distinct from
// TRIGGERED on purpose: TriggeredAt keys the 14-day emergency clock on
// TRIGGERED rows, and a stale cut is already the exit, not the start of a
// watch. Run 97 closed four trades through this branch (−2,827 USDT) with
// nothing on the trade but BUY_TO_SELLLOSS.
const StaleCutPrefix = "Smart take loss STALE-CUT"

// staleBagStatus is staleBagDue with its numbers: whether the cut is due,
// the trade's age in days and the estimated close profit at price.
func staleBagStatus(trade aggragates.Trades, price float64, now time.Time) (due bool, ageDays float64, pnl float64) {
	if !trade.Strategy.Params.SmartTakeLoss || trade.CreatedAt.IsZero() || now.IsZero() {
		return false, 0, 0
	}
	age := now.UTC().Sub(trade.CreatedAt.UTC())
	ageDays = age.Hours() / 24
	if age < StaleAfter {
		return false, ageDays, 0
	}
	if ladder.CountFilledEntries(trade) < crashguard.DeRiskMinDepth {
		return false, ageDays, 0
	}
	pnl, invested := estimateCloseProfit(trade, price)
	return invested > 0 && pnl <= 0, ageDays, pnl
}

// staleCutMessage is the reason row of a stale-bag exit.
func staleCutMessage(eval Evaluation) string {
	return fmt.Sprintf("%s: age %.0f days, depth %d/%d, est. close %.2f, no continuation verdict read",
		StaleCutPrefix, eval.AgeDays, eval.FilledEntries, eval.MaxDepths, eval.EstProfit)
}
