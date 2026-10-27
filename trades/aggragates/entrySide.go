package aggragates

// The two directions a first fill can take, the vocabulary PatternDirection uses too.
const (
	SideLong  = "long"
	SideShort = "short"
)

// EntrySide is the direction a first fill would take. Spot and futures carry
// it in different places: on spot Inverse IS the direction (an inverse ladder
// sells the base first); on futures Inverse is always false and the ML
// verdict decides, as createFuturesOrders reads it. An empty result, futures
// only, means the verdict names no direction: the engines refuse such an
// entry before the chain is built, since its first action cancels every open order.
func EntrySide(trade Trades, ai AIIndicators) string {
	if trade.Strategy.TradeType == Futures {
		switch ai.AIAction {
		case ActionLong:
			return SideLong
		case ActionShort:
			return SideShort
		}

		return ""
	}

	if trade.Inverse {
		return SideShort
	}

	return SideLong
}
