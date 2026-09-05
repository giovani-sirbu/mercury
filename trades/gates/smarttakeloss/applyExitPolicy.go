package smarttakeloss

import (
	"strings"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/crashguard"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

const (
	// ArmedPrefix / TriggeredPrefix are the trade-log prefixes engines
	// persist so a later tick can recover the arm/trigger moment without
	// extra columns.
	ArmedPrefix     = "Smart take loss ARMED"
	TriggeredPrefix = "Smart take loss TRIGGERED"
	// EmergencyAfter is the hopeless-cut horizon: a trade that has sat in
	// STL states this long, still underwater, with the crash guard clear may
	// emit sellLoss so bags do not sit forever.
	EmergencyAfter = 14 * 24 * time.Hour
	// StaleAfter cuts an underwater deep bag that never triggered STL
	// (reversal evidence kept the HIGH gate closed). Run 90's HBAR sat
	// Nov–Aug. 21 days from CreatedAt, depth >= crashguard.DeRiskMinDepth.
	StaleAfter = 21 * 24 * time.Hour
)

// ApplyExitPolicy is the C.5 / break-even / 14-day gate.
// Underwater sellLoss from the trail is swallowed (empty position — stay
// in the current STL state) unless the emergency horizon has elapsed with
// crash clear. sellLoss is emitted only from this policy and the STL trail.
func ApplyExitPolicy(
	trade aggragates.Trades,
	position string,
	price float64,
	now time.Time,
	crashActive bool,
) string {
	pnl, invested := EstimateCloseProfit(trade, price)
	underwater := invested > 0 && pnl <= 0
	if !crashActive && underwater && emergencyDue(trade, now) {
		return "sellLoss"
	}
	if !crashActive && position != "takeProfit" && staleBagDue(trade, price, now) {
		return "sellLoss"
	}
	if position == "sellLoss" && pnl <= 0 {
		return ""
	}
	return position
}

func emergencyDue(trade aggragates.Trades, now time.Time) bool {
	triggeredAt := TriggeredAt(trade)
	if triggeredAt.IsZero() || now.IsZero() {
		return false
	}
	return now.UTC().Sub(triggeredAt.UTC()) >= EmergencyAfter
}

func staleBagDue(trade aggragates.Trades, price float64, now time.Time) bool {
	if !trade.Strategy.Params.SmartTakeLoss || trade.CreatedAt.IsZero() || now.IsZero() {
		return false
	}
	if now.UTC().Sub(trade.CreatedAt.UTC()) < StaleAfter {
		return false
	}
	if ladder.CountFilledEntries(trade) < crashguard.DeRiskMinDepth {
		return false
	}
	pnl, invested := estimateCloseProfit(trade, price)
	return invested > 0 && pnl <= 0
}

// TriggeredAt is the first TRIGGERED log's CreatedAt, or zero.
func TriggeredAt(trade aggragates.Trades) time.Time {
	for _, entry := range trade.Logs {
		if strings.HasPrefix(entry.Message, TriggeredPrefix) {
			return entry.CreatedAt
		}
	}
	return time.Time{}
}
