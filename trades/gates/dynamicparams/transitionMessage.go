package dynamicparams

import (
	"strconv"
	"strings"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// TransitionPrefix opens every row the engines write on a change of the
// increase the reads raise (Changed). agora keeps the rows carrying it off the
// users' notifications, so it must stay byte-stable.
const TransitionPrefix = "dynamic params:"

// bullishRead is the value a bullish read carries on the wire, sophos'
// smctypes.Bullish.
const bullishRead = 1

// TransitionMessage is the row body for the reads a change lands on: the
// timeframe, each read, the tier and what it raises on every row, amounts
// included. Every engine writes this one text, so live and replayed trades
// read the same, and it must stay byte-stable for the same reason. A block
// sophos did not read names its timeframe alone.
func TransitionMessage(reads aggragates.DynamicParamsIndicators) string {
	return transitionMessageFor(reads, MixedIncrease)
}

// transitionMessageFor is TransitionMessage with the mixed tier's increase
// handed in, as raiseFor takes it.
func transitionMessageFor(reads aggragates.DynamicParamsIndicators, mixed Increase) string {
	head := TransitionPrefix
	if reads.Timeframe != "" {
		head += " " + reads.Timeframe
	}

	if !reads.Valid {
		return head + " not read, configured rows"
	}

	tier := TierOf(reads)

	return head +
		" Super Guppy " + readWord(reads.Guppy) +
		", BMSB " + readWord(reads.BMSB) +
		": " + tierWord(tier) +
		", " + increasePhrase(raiseFor(tier, mixed))
}

// readWord names a read: bearish and bullish by their wire values, and every
// other value neutral, as TierOf counts it.
func readWord(read int) string {
	switch read {
	case bearishRead:
		return "bearish"
	case bullishRead:
		return "bullish"
	default:
		return "neutral"
	}
}

// tierWord names a tier.
func tierWord(tier Tier) string {
	switch tier {
	case TierBothBearish:
		return "both bearish"
	case TierMixed:
		return "mixed"
	default:
		return "not bearish"
	}
}

// increasePhrase names what an increase does to every row, with the amounts
// it adds; one that changes no row reads as the configured rows.
func increasePhrase(increase Increase) string {
	if !increase.changesRows() {
		return "configured rows"
	}

	var raised []string
	if increase.raisesPercentage() && BearPercentagePoints != 0 {
		raised = append(raised, "percentage +"+formatAmount(BearPercentagePoints))
	}
	if increase.raisesDepths() && BearDepths != 0 {
		raised = append(raised, "depths +"+formatAmount(float64(BearDepths)))
	}

	return strings.Join(raised, " and ") + " on every row"
}

// formatAmount writes an amount with no trailing zeros.
func formatAmount(amount float64) string {
	return strconv.FormatFloat(amount, 'f', -1, 64)
}
