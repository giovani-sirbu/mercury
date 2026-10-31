package smartsell

import (
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// fundsRefusedLogged is the oracle's read of the funds refusal, stated apart from the code: an
// ERROR row opening with "Insufficient funds" stamped at or after the newest entry row of the
// ladder's side (the accounting rows left out).
func fundsRefusedLogged(trade aggragates.Trades) bool {
	entrySide := "BUY"
	if trade.Inverse {
		entrySide = "SELL"
	}
	var newest time.Time
	for _, row := range trade.History {
		if row.Type == entrySide && row.Quantity > 0 && row.Price > ladder.AccountingPriceCeiling && row.CreatedAt.After(newest) {
			newest = row.CreatedAt
		}
	}
	for _, row := range trade.Logs {
		if row.Type == aggragates.LOG_ERROR && strings.HasPrefix(row.Message, "Insufficient funds") && !row.CreatedAt.Before(newest) {
			return true
		}
	}
	return false
}

// Capital protection watches a ladder at its last depth only once its next depth was refused for
// funds after its newest fill: the ERROR row SaveError writes (tradelog.HasInsufficientBalance),
// stamped at or after that fill, whatever rows follow it. No refusal, one stamped before the newest
// fill and an info row at the fill leave it unwatched; a fill past the ceiling outdates the refusal
// until the next one; a ladder with no stamps at all (sisyphus live-testing stamps no history)
// counts any refusal. capitalProtectionWatched, rebuildState's watch, the oracle and Apply's
// triggered row on the trend reading all read it so.
func TestCapitalProtectionWatchesOnlyAfterTheFundsRefusal(t *testing.T) {
	withCapitalProtectionExit(t, true)
	refused := lastDepthLadder()
	newestAt := entryFills(refused)[lastDepthFills-1].At
	bare := testutil.LadderTrade(false, fills(lastDepthFills, "21:30:00")...)
	stale := lastDepthLadder()
	stale.Logs[len(stale.Logs)-1].CreatedAt = newestAt.Add(-time.Minute)
	info := lastDepthLadder()
	info.Logs[len(info.Logs)-1] = aggragates.TradesLogs{Type: aggragates.LOG_INFO, Message: "TRADE_UNBLOCKED", CreatedAt: newestAt}
	later := lastDepthLadder()
	later.Logs[len(later.Logs)-1].CreatedAt = newestAt.Add(time.Hour)
	later.Logs = append(later.Logs, aggragates.TradesLogs{Type: aggragates.LOG_INFO, Message: "TRADE_UNBLOCKED", CreatedAt: newestAt.Add(2 * time.Hour)})
	refilled := withFill(refused, 97, newestAt.Add(time.Hour))
	unstamped := lastDepthLadder()
	for index := range unstamped.History {
		unstamped.History[index].CreatedAt = time.Time{}
	}
	unstamped.Logs[len(unstamped.Logs)-1].CreatedAt = time.Time{}

	for name, tc := range map[string]struct {
		trade aggragates.Trades
		want  bool
	}{
		"refused at the newest fill":                {refused, true},
		"refused after it, then unblocked":          {later, true},
		"never refused":                             {bare, false},
		"refused before the newest fill":            {stale, false},
		"an info row at the fill":                   {info, false},
		"a fill past the ceiling after the refusal": {refilled, false},
		"refused again past the ceiling":            {fundsRefused(refilled), true},
		"a ladder with no stamps":                   {unstamped, true},
	} {
		if got := capitalProtectionWatched(tc.trade); got != tc.want {
			t.Errorf("%s: watched %v, want %v", name, got, tc.want)
		}
		if got := rebuildState(tc.trade).capitalProtectionWatched; got != tc.want {
			t.Errorf("%s: rebuildState reads the watch as %v, want %v", name, got, tc.want)
		}
		if got := fundsRefusedLogged(tc.trade) && lastDepthFilled(tc.trade, ladder.CountFilledEntries(tc.trade)); got != tc.want {
			t.Errorf("%s: the oracle reads %v, want %v", name, got, tc.want)
		}
		if got := capitalProtectionTriggeredRow(Apply(tc.trade, "", lastDepthPrint, withBlock(solBlock()))); got != tc.want {
			t.Errorf("%s: Apply writes the triggered row %v, want %v", name, got, tc.want)
		}
	}
}
