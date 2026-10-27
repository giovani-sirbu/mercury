package smarttakeloss

import (
	"fmt"

	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The reasons a forced exit names in Result.Reason; the shared action (sellLoss) names the gate
// whose trigger activated a committed sale, else the first whose trigger is reached.
const (
	reasonSlowDecline        = "slow decline"
	reasonCapitalProtection  = "capital protection"
	reasonFlatMarket         = "flat market"
	reasonFreeFallPrevention = "free fall prevention"
)

// ExitMessage is the text of the INFO row beside a forced sellLoss, at the level the chain places
// its limit (the pair's PriceFilter decimals); the slow decline gate's sale names the features the
// ladder is pending on, read off the trade's events (rebuildState).
func ExitMessage(trade aggragates.Trades, reason string, level float64) string {
	return exitMessage(trade, reason, level, exitFeatures(trade, reason))
}

// exitFeatures are the decline features the slow decline gate's sale ends; the other gates' sales have none.
func exitFeatures(trade aggragates.Trades, reason string) []string {
	if reason == reasonCapitalProtection || reason == reasonFlatMarket || reason == reasonFreeFallPrevention {
		return nil
	}
	return rebuildState(trade).pendingFeatures
}

// exitMessage is ExitMessage for the features already read.
func exitMessage(trade aggragates.Trades, reason string, level float64, features []string) string {
	message := fmt.Sprintf("smartTakeLoss: sell at %s %s", reason, helpers.FormatPriceLevel(trade, level))
	if len(features) == 0 {
		return message
	}
	return message + " (" + labelList(features) + ")"
}
