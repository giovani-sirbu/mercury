package actions

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/trades/tradelog"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/funds"
	"github.com/giovani-sirbu/mercury/trades/ladder"
	"github.com/giovani-sirbu/mercury/trades/quantities"
)

func HasFunds(event events.Events) (events.Events, error) {
	remainedQuantity, neededQuantity, assetSymbol, err := funds.GetFundsQuantities(event)

	if err != nil {
		return events.Events{}, err
	}

	if remainedQuantity < neededQuantity {
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
