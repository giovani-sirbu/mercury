// Package fees aggregates the commissions on a trade's history, literally
// (CalculateFees), cross-converted into base and quote (GetFeesBaseQuote,
// GetFees) and as the settlement leg still owed (GetSettlementFees).
package fees

import (
	"fmt"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/log"
)

// GetFees returns the aggregated fees in the denomination relevant to the trade
// direction: quote for spot, base for inverse.
//
// This is the TOTAL COST of all commissions expressed in one denomination —
// a reporting figure, and (pre-close) a fair one-leg estimate of the closing
// fee. It is NOT the amount to subtract from GetProfit's gross: the opening
// leg's commission is already embodied in the fill quantities, so realized
// profit math must use GetSettlementFees instead or it double-charges it.
func GetFees(event events.Events) float64 {
	feesInBase, feesInQuote := GetFeesBaseQuote(event)

	fees := feesInQuote
	if event.Trade.Inverse {
		fees = feesInBase
	}

	log.Debug(fmt.Sprintf("GetFees: fees(%f), feesInBase(%f), feesInQuote(%f), inverse(%t)", fees, feesInBase, feesInQuote, event.Trade.Inverse))

	return fees
}
