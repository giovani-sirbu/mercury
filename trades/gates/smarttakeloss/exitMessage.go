package smarttakeloss

import (
	"fmt"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
)

// The reasons a forced exit can name in Result.Reason: the quiet slow
// decline's sale at the sell band sophos serves, and capital protection's at
// the upper Bollinger band sophos serves beside its SMC trend reading. Both
// rules watch long ladders only and have no inverse mirror.
const (
	reasonSellBand          = "slow-decline bollinger band"
	reasonCapitalProtection = "upper bollinger band (capital protection)"
)

// ExitMessage is the text of the INFO row the engines write beside a forced
// sellLoss: the human-readable trace of WHY the trade sold, and what the
// backtest verification reads. ExitRow builds the row from it, with the sold
// event that records the rule beside it. level is the price the sellLoss chain
// places its limit at, printed with the pair's PriceFilter decimals.
func ExitMessage(trade aggragates.Trades, reason string, level float64) string {
	return fmt.Sprintf("smartTakeLoss: sell at %s %s", reason, gates.FormatPriceLevel(trade, level))
}
