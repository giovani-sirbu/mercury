package smarttakeloss

import (
	"fmt"
	"slices"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
	"github.com/giovani-sirbu/mercury/trades/profit"
)

// bandSaleLoss is the loss the slow decline gate's sale at the sell band would
// take at this tick price, as a percentage of the amount the trade has used
// (ladder.EntryNotional), and whether that is over the cap
// (SlowDeclineMaxLossPct): over it, Apply holds the sale. A loss exactly at the
// cap sells. A close that books zero or a gain, a trade that has used nothing
// and a cap at zero or below are never over: the loss is read only when the cap
// is on.
//
// The loss is the figure acceptLoss books for the sale, profit.SimulatedCloseProfit,
// on a copy of the trade the sellLoss chain would hold — PositionType "sellLoss"
// and PositionPrice at the tick, the limit the chain places — so the dust, the
// opening commissions and the closing leg count as they do there. A fee paid in a
// third asset is the one cost left out: pricing it takes a wallet snapshot or the
// exchange, which Apply has neither of and a backtest never has fees in, so the
// history the simulation reads is a deep copy without those entries (the trade
// the caller holds keeps its own).
func bandSaleLoss(trade aggragates.Trades, price float64) (lossPct float64, over bool) {
	used := ladder.EntryNotional(trade)
	if slowDeclineMaxLossPct <= 0 || used <= 0 {
		return 0, false
	}

	sale := trade
	sale.PositionType = "sellLoss"
	sale.PositionPrice = price
	sale.History = withoutThirdAssetFees(trade)
	net := profit.SimulatedCloseProfit(events.Events{Trade: sale})
	if net >= 0 {
		return 0, false
	}

	return -net / used * 100, -net*100 > slowDeclineMaxLossPct*used
}

// withoutThirdAssetFees is a deep copy of the trade's history, every row's fees
// copied with it, less the fee entries paid in neither the base nor the quote
// asset of the trade's symbol: what fees.GetFees and fees.UnembodiedOpeningFees
// would price through a wallet snapshot or the exchange.
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

// SaleHeldRow is the row of the tick the loss cap holds a pending ladder's sale
// at the sell band, the first such tick of the episode: lossPct is what the sale
// would have lost (bandSaleLoss), fill the newest fill and features the decline
// features the ladder is pending on. The message names the loss and the cap with
// two decimals, and it opens with no hold frame and never with the words of a
// sold row; its event is EventSaleHeld, filed under GateSlowDecline at the
// fill's price. The fold reads that the episode's sale was held and never at
// which fill (state.saleHeld), so one row stands for the whole episode.
func SaleHeldRow(fill, lossPct float64, features ...string) Row {
	return Row{
		Message:  fmt.Sprintf("smartTakeLoss: slow decline sale held at the bollinger band, loss %.2f%% of the used amount over %.2f%%", lossPct, slowDeclineMaxLossPct),
		Price:    fill,
		Gate:     GateSlowDecline,
		Event:    EventSaleHeld,
		Features: slices.Clone(features),
	}
}

// saleHeldRowDue is whether the held sale needs its row now: the loss cap has
// not held the sale in this pending episode yet. It is due once per episode —
// the pending, cancelled, reset and sold events clear what the fold remembers,
// so a confirmation at a new fill makes it due again — and not again when the
// price leaves the band and comes back, nor at the fill that takes the ladder to
// its last depth, which is never judged and leaves the episode, the ladder
// pending from where it was, as it was.
func (st state) saleHeldRowDue() bool {
	return !st.saleHeld
}

// afterSale is the state after a sale event of the slow decline gate, the only
// two kinds that move the held sale alone: a sale held remembers that the
// episode's sale was held, a sold one forgets it.
func (st state) afterSale(data EventData) state {
	st.saleHeld = data.Event == EventSaleHeld

	return st
}
