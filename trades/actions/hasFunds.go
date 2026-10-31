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
// the wallet may sit and still place that entry for what it holds, as a share
// of the needed quantity; a wider shortfall blocks. Zero turns the waiver off.
const AllowRoundedQuantityPercentage = 10.0

// HasFunds checks the wallet covers the next order. A small entry shortfall
// is waived (AllowsRoundedQuantity) and Buy trims the entry to the wallet; a
// ladder with fills under the Impasse flag flips to impasse when a fresh
// ladder could be sized from what it used; anything else blocks the trade
// with an insufficient-funds error.
func HasFunds(event events.Events) (events.Events, error) {
	remainedQuantity, neededQuantity, assetSymbol, err := funds.GetFundsQuantities(event)

	if err != nil {
		return events.Events{}, err
	}

	if remainedQuantity < neededQuantity {
		// A marginal shortfall buys what the wallet affords; Buy reads the amount off Params.
		if AllowsRoundedQuantity(event, remainedQuantity, neededQuantity) {
			event.Params.AvailableQuantity = remainedQuantity
			return event, nil
		}

		// A first entry has no position to average down, so impasse never applies to it.
		firstEntry := len(event.Trade.History) == 0
		// The impasse check sizes a ladder from the trade's own rows, never Params.EntrySettings.
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
// amount instead of blocking. Entries only: a short close is a missing
// position, not a missing budget, and a trimmed close would leave a remainder
// under the exchange minimum. Exported because the sisyphus backtest's own
// HasFunds must answer it the same way.
func AllowsRoundedQuantity(event events.Events, remainedQuantity, neededQuantity float64) bool {
	if AllowRoundedQuantityPercentage <= 0 {
		return false
	}

	// A needed quantity of zero is no shortfall to round off.
	if neededQuantity <= 0 || remainedQuantity <= 0 || event.Trade.PositionPrice <= 0 {
		return false
	}

	if funds.IsSellAction(event.Trade.PositionType) {
		return false
	}

	if neededQuantity-remainedQuantity > neededQuantity*AllowRoundedQuantityPercentage/100 {
		return false
	}

	// The affordable entry must still clear the exchange minimum, or Buy raises it back over the wallet.
	affordableQuantity := remainedQuantity
	if !event.Trade.Inverse {
		affordableQuantity = remainedQuantity / event.Trade.PositionPrice
	}
	affordableQuantity = helpers.ToFixed(affordableQuantity, int(event.Trade.StrategyPair.TradeFilters.LotSize))

	return affordableQuantity >= quantities.CalculateMinOrderQty(event.Trade)
}
