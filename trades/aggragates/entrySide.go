package aggragates

// The two directions a first fill can take. Same vocabulary the pattern
// verdict uses for PatternDirection, so a gate comparing the two needs no
// translation.
const (
	SideLong  = "long"
	SideShort = "short"
)

// EntrySide is the direction a first fill would take.
//
// SPOT AND FUTURES CARRY THE DIRECTION IN DIFFERENT PLACES, and reading the
// wrong one is silent. On spot, Inverse IS the direction: an inverse ladder
// sells the base asset first. On futures, Inverse is always false — nothing in
// the codebase ever sets it for a futures trade — and the direction is the ML
// verdict, which createFuturesOrders reads to pick SideTypeBuy or SideTypeSell.
//
// The entry gates used to take Inverse directly, so a futures SHORT verdict
// read as "bearish" and was vetoed by the very gate meant to protect the entry:
// futures could only ever open long, and the cooldown gate judged a short
// against the cheap-LONG lens.
//
// An empty result means the verdict names no direction. Only futures can
// produce it, and it is not a hold: the engines refuse such an entry before the
// action chain is built, because the first action of the futures entry chain
// cancels every open order on the symbol.
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
