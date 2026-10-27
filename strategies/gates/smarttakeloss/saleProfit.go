package smarttakeloss

import (
	"time"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
	"github.com/giovani-sirbu/mercury/trades/profit"
)

// triggerStanding is where the profit of a sale stands against the trigger (saleProfit).
type triggerStanding int

const (
	// nothingUsed is a trade that used nothing: no candidate to activate the stopLoss; a self-armed stopLoss sells.
	nothingUsed triggerStanding = iota
	// reached is a sale booking the trigger or more.
	reached
	// short is a sale booking less than the trigger.
	short
)

// saleProfit is the profit a sale of the whole position at this price would book, in percent of the
// amount used (ladder.EntryNotional for a long, ladder.EntryQuantity inverse; negative a loss), and
// whether it reaches triggerPct; it is what acceptLoss books, without the third-asset fees.
func saleProfit(trade aggragates.Trades, price, triggerPct float64) (profitPct float64, standing triggerStanding) {
	used := ladder.EntryNotional(trade)
	if trade.Inverse {
		used = ladder.EntryQuantity(trade)
	}
	if used <= 0 {
		return 0, nothingUsed
	}

	sale := trade
	sale.PositionType = "sellLoss"
	sale.PositionPrice = price
	sale.History = withoutThirdAssetFees(trade)
	net := profit.SimulatedCloseProfit(events.Events{Trade: sale})
	profitPct = net / used * 100
	if net*100 < triggerPct*used {
		return profitPct, short
	}
	return profitPct, reached
}

// withoutThirdAssetFees is a deep copy of the trade's history less the fees paid in neither the base
// nor the quote asset of the symbol.
func withoutThirdAssetFees(trade aggragates.Trades) []aggragates.TradesHistory {
	base, quote := helpers.SplitSymbol(trade.Symbol)
	history := make([]aggragates.TradesHistory, len(trade.History))
	for index, row := range trade.History {
		history[index] = row
		history[index].Fees = nil
		for _, fee := range row.Fees {
			if fee.Asset == base || fee.Asset == quote {
				history[index].Fees = append(history[index].Fees, fee)
			}
		}
	}

	return history
}

// SaleHeldRow is the row of a pending episode's tick the trigger holds the slow decline gate's sale
// on, the first tick of the episode and again whenever the trigger moves: profitPct from saleProfit
// under the trigger of a dashboard reading the ladder's way on every read (FullTrendTrigger), fill
// the newest fill, features the pending ones; its event is EventSaleHeld under GateSlowDecline.
func SaleHeldRow(fill, profitPct float64, features ...string) Row {
	return saleHeldByProfitRow(GateSlowDecline, fill, profitPct, FullTrendTrigger(), features)
}

// saleHeldRowDue is whether the held sale needs its row: once per pending episode, by whichever bound.
func (st state) saleHeldRowDue() bool {
	return !st.saleHeld
}

// afterSale is the state after a sale event of the slow decline gate stamped at: held, activated and
// the gap kinds move the sale's memory and gap stamp (gapStamp), sold forgets them all.
func (st state) afterSale(data EventData, at time.Time) state {
	switch data.Event {
	case EventSaleHeld:
		st.saleHeld, st.saleHeldUnder, st.saleReachedAt = true, heldTriggerOf(data), time.Time{}
	case EventActivated:
		st.slowDeclineActivated, st.slowDeclineGapHeldAt, st.saleReachedAt = true, time.Time{}, time.Time{}
	case EventGapHeld, EventGapClosed:
		st.slowDeclineGapHeldAt = gapStamp(data.Event, at)
	case EventSold:
		st.saleHeld, st.saleHeldUnder, st.slowDeclineActivated = false, heldTrigger{}, false
		st.slowDeclineGapHeldAt, st.saleReachedAt = time.Time{}, time.Time{}
	}

	return st
}
