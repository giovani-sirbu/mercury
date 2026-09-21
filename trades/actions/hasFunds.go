package actions

import (
	"fmt"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/trades/funds"
	"github.com/giovani-sirbu/mercury/trades/ladder"
	"github.com/giovani-sirbu/mercury/trades/quantities"
	"github.com/giovani-sirbu/mercury/trades/tradelog"
)

// AllowRoundedQuantityPercentage is how far under the quantity an entry needs
// the wallet may sit and still place that entry, as a percentage of the needed
// quantity. Inside it the shortfall is treated as the wallet having drifted a
// fraction below the ladder's own arithmetic — fees charged in the quote
// asset, a balance reconciled after a partial fill — and the entry is placed
// for what the wallet holds instead of blocking the trade. A shortfall wider
// than this is a real one and blocks as before.
//
// Zero turns the waiver off: every shortfall blocks, whatever its size.
const AllowRoundedQuantityPercentage = 10.0

func HasFunds(event events.Events) (events.Events, error) {
	remainedQuantity, neededQuantity, assetSymbol, err := funds.GetFundsQuantities(event)

	if err != nil {
		return events.Events{}, err
	}

	if remainedQuantity < neededQuantity {
		// A wallet only marginally short of the entry buys what it can afford
		// rather than blocking the trade. Buy reads the amount off Params;
		// the chains that place no order ignore it.
		if AllowsRoundedQuantity(event, remainedQuantity, neededQuantity) {
			event.Params.AvailableQuantity = remainedQuantity
			return event, nil
		}

		// A trade with no fills holds nothing: there is no position to
		// average down, so impasse does not apply to a first entry (same
		// predicate as the sisyphus HasFunds). Without it a first entry that
		// lost the wallet to InverseUsedAmount flipped to impasse and the
		// next tick ran createChildrenTrades → sellAll against no position.
		firstEntry := len(event.Trade.History) == 0
		// set trade to impasse if this feature is activated for this strategy
		if !firstEntry && event.Trade.Strategy.Params.Impasse && event.Trade.ParentID == 0 {
			usedAmount := quantities.GetUsedQuantities(event) * event.Trade.PositionPrice
			_, hasFundsError := ladder.CalculateInitialBid(usedAmount, event.Trade, 0)
			if hasFundsError == nil {
				event.Trade.PositionType = "impasse"
			}
		}

		return tradelog.SaveError(event, fmt.Errorf("Insufficient funds (%f %s) for the requested action (%s). You need at least %f %s to resume this trade.", remainedQuantity, assetSymbol, event.Trade.PositionType, neededQuantity, assetSymbol))
	}

	return event, nil
}

// AllowsRoundedQuantity reports whether the wallet sits close enough under the
// quantity the next entry needs for that entry to be placed for the wallet's
// amount instead of blocking the trade.
//
// Entries only. A shortfall on the closing side is a missing position, not a
// missing budget: trimming the close to the wallet leaves behind a remainder
// that no later tick can sell once it falls under the exchange minimum, so
// those keep blocking.
//
// Exported because the sisyphus backtest owns its own copy of HasFunds and has
// to answer this the same way the live engine does; a run that blocks entries
// production would place is not calibrating the same strategy. That copy adds
// one rule of its own — it prices a first entry, where this gate reads zero —
// so it asks about first entries before it asks this.
func AllowsRoundedQuantity(event events.Events, remainedQuantity, neededQuantity float64) bool {
	if AllowRoundedQuantityPercentage <= 0 {
		return false
	}

	// A needed quantity of zero is not a shortfall to round off: the entry
	// asks for nothing and only an overdrawn wallet puts it here.
	if neededQuantity <= 0 || remainedQuantity <= 0 || event.Trade.PositionPrice <= 0 {
		return false
	}

	if funds.IsSellAction(event.Trade.PositionType) {
		return false
	}

	if neededQuantity-remainedQuantity > neededQuantity*AllowRoundedQuantityPercentage/100 {
		return false
	}

	// The entry the wallet can afford still has to be one the exchange
	// accepts. Below the minimum order quantity Buy raises it back to that
	// minimum, which puts the order over the wallet again and turns this
	// waiver into an exchange rejection — a worse outcome than the block it
	// replaced.
	affordableQuantity := remainedQuantity
	if !event.Trade.Inverse {
		affordableQuantity = remainedQuantity / event.Trade.PositionPrice
	}
	affordableQuantity = helpers.ToFixed(affordableQuantity, int(event.Trade.StrategyPair.TradeFilters.LotSize))

	return affordableQuantity >= quantities.CalculateMinOrderQty(event.Trade)
}
