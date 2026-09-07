package smarttakeloss

import (
	"fmt"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
)

// The legs a forced exit can name in Result.Reason: long on the left, the
// inverse mirror on the right.
const (
	reasonResistanceLine = "resistance line"
	reasonSupportLine    = "support line"
	reasonUpperBand      = "upper bollinger band"
	reasonLowerBand      = "lower bollinger band"
	reasonToleranceUnder = "tolerance under the last fill"
	reasonToleranceAbove = "tolerance above the last fill"
)

// ExitMessage is the INFO row the engines write beside a forced sellLoss:
// the only trace of WHY the trade sold, and what the backtest verification
// reads. level is the price the sellLoss chain places its limit at, printed
// with the pair's PriceFilter decimals.
func ExitMessage(trade aggragates.Trades, reason string, level float64) string {
	return fmt.Sprintf("smartTakeLoss: sell at %s %s", reason, gates.FormatPriceLevel(trade, level))
}
