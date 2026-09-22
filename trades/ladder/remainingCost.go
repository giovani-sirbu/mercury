package ladder

import (
	"strings"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// RemainingCost is the ladder's remaining PLANNED budget: what the entries
// from its next depth through its last configured one were sized to cost,
// together with the asset it spends them in.
//
// It is the amount the cooldown depth-priority gate reserves the wallet for.
// A depth alone says nothing about money — a ladder can be one entry from its
// ceiling and need more than the wallet holds, or five entries from it and
// need almost nothing — so the view carries the sum and the gate compares
// against it.
//
// PLANNED is the whole point, and it is the arithmetic the grid was laid out
// with: CalculateInitialBid sizes the initial bid so that the entries after
// it, each one the previous quantity times the row's multiplier at a price a
// further percentage step down, add up to the budget. So the walk here moves
// the price down the grid exactly as it moves the quantity up, and an inverse
// ladder is counted in bare base quantities because its grid is planned that
// way too — the percentage shapes only the quote proceeds there.
//
// Pricing the tail at TODAY's price instead would reserve far more than the
// grid can ever spend: every remaining entry would be charged at the level
// the ladder has already fallen to, and on a doubling ladder the compounded
// difference is enough for one ladder's remainder to exceed the whole wallet
// from its first depth — which turns a reserve into a queue, holding every
// sibling out of its first fill.
//
// The walk starts from the PLANNED quantity of the last filled depth, never
// from what that depth has executed so far. An entry settles in parts and the
// history row's quantity grows with them, so between an entry's first part
// and its completion the deepest row carries less than the row above it —
// read as a base, the ladder looks one depth shallower than it is and the
// reserve comes out roughly one depth short. Every sibling arming inside that
// window would pass a gate that should have held it, and the entries they
// place are exactly what the ladder then lacks for its own last depth. The
// plan has no such window: the first entry's quantity taken through the
// multipliers is what the depth WILL be worth, part-filled or not.
//
// Zero, meaning "this ladder reserves nothing", on every ladder that cannot
// name an amount: one that has filled no entry (there is no quantity to
// multiply), one whose pair carries no settings row, one with no configured
// ceiling or already at it, and a long one without a position price.
func RemainingCost(trade aggragates.Trades) (string, float64) {
	return remainingCostAt(trade, CountFilledEntries(trade), ConfiguredDepths(trade))
}

// remainingCostAt is RemainingCost for a caller that has already counted the
// ladder's filled entries and read its ceiling. DepthOf holds both, and every
// surface builds its whole wallet view through DepthOf — folding the same
// history again per ladder is work the tick path pays for nothing.
func remainingCostAt(trade aggragates.Trades, filled, ceiling int) (string, float64) {
	asset := SpendingAsset(trade)

	settings := trade.StrategyPair.StrategySettings
	if len(settings) == 0 {
		return asset, 0
	}

	if filled == 0 || ceiling <= 0 || filled >= ceiling {
		return asset, 0
	}

	if !trade.Inverse && trade.PositionPrice <= 0 {
		return asset, 0
	}

	quantity := plannedQuantityAtDepth(trade, filled)
	price := trade.PositionPrice
	cost := 0.0

	for depth := filled; depth < ceiling; depth++ {
		// Entry number depth+1 reads row depth, the same row-selection
		// contract Buy and the funds gate use for the entry they place.
		row := settings[SettingsIndexOrBase(settings, depth)]
		quantity *= row.Multiplier

		if trade.Inverse {
			cost += quantity
			continue
		}

		// One percentage step further down the grid per depth, the step the
		// ladder was planned to buy that entry at.
		price *= 1 - row.Percentage/100
		cost += quantity * price
	}

	return asset, cost
}

// SpendingAsset is the side of the pair a ladder's entries spend: the quote
// asset for a long ladder, the base asset for an inverse one, which is the
// asset the funds gate compares the wallet against. An unparsable symbol
// names no asset, and a view entry with no asset matches no other.
//
// Exported because the engines have to tag the balance they read with the
// same asset the wallet view names for that ladder, and they must not each
// split the symbol their own way: a balance tagged one way and ladders
// scoped another simply never meet, and the gate silently stops holding.
func SpendingAsset(trade aggragates.Trades) string {
	pair := strings.Split(trade.Symbol, "/")
	if len(pair) != 2 {
		return ""
	}

	if trade.Inverse {
		return pair[0]
	}
	return pair[1]
}

// plannedQuantityAtDepth is what the ladder's entry at the given depth was
// laid out to be: the first entry's quantity — the initial bid the whole grid
// is built from — taken through the multiplier of every row the entries after
// it read. It is the quantity the grid commits to, so it does not move while
// an entry is settling.
func plannedQuantityAtDepth(trade aggragates.Trades, depth int) float64 {
	settings := trade.StrategyPair.StrategySettings
	quantity := firstEntryQuantity(trade)

	// Entry k+1 reads row k, the row-selection contract every ladder read
	// shares; the first entry is the initial bid and reads no row at all.
	for entry := 1; entry < depth; entry++ {
		quantity *= settings[SettingsIndexOrBase(settings, entry)].Multiplier
	}

	return quantity
}

// firstEntryQuantity is the quantity of the ladder's first entry, read in
// history order: the earliest row CountFilledEntries would count, on the side
// this ladder enters from, skipping the bookkeeping rows an impasse child's
// profit transfer leaves on its parent.
//
// Order status is deliberately not consulted — the surfaces that build the
// wallet view do not agree on that field, and a reserve that read it would
// mean one thing in a replay and another in production.
func firstEntryQuantity(trade aggragates.Trades) float64 {
	entrySide := "BUY"
	if trade.Inverse {
		entrySide = "SELL"
	}

	for _, history := range trade.History {
		if history.Type != entrySide || history.Quantity <= 0 {
			continue
		}
		if history.Price <= AccountingPriceCeiling {
			continue
		}

		return history.Quantity
	}

	return 0
}

// lastEntryQuantity is the quantity of the ladder's latest entry, the one the
// next entry is sized from — GetLatestQuantityByHistory's answer, taken on a
// copy of the history.
//
// The copy is the point: that helper sorts the slice it is handed, and the
// wallet view is built over trades the engines hold by reference and read
// under a shared lock. Reordering their history rows from here would mutate
// the engine's own memory on every gated tick.
func lastEntryQuantity(trade aggragates.Trades) float64 {
	entrySide := "BUY"
	if trade.Inverse {
		entrySide = "SELL"
	}

	history := make([]aggragates.TradesHistory, len(trade.History))
	copy(history, trade.History)

	return GetLatestQuantityByHistory(history, entrySide)
}
