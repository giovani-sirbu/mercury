package trades

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// accountingHistoryPriceCeiling identifies bookkeeping history rows: an
// impasse child closing marks its profit onto the parent as a BUY row with the
// sentinel price 1e-13. Anything at or below this ceiling is a ledger entry,
// never an executed market fill.
const accountingHistoryPriceCeiling = 1e-12

// CountFilledEntries counts the trade's executed entry orders — BUYs for a
// normal trade, SELLs for an inverse one. It counts DISTINCT exchange orders,
// not history rows: partial fills update the same order id and must not
// advance the ladder row the strategy reads. Accounting rows (child-profit
// transfers onto an impasse parent) are bookkeeping, not entries, and are
// skipped by their sentinel price.
func CountFilledEntries(trade aggragates.Trades) int {
	entrySide := "BUY"
	if trade.Inverse {
		entrySide = "SELL"
	}

	seenOrders := make(map[int64]struct{}, len(trade.History))
	filledEntries := 0
	for index, history := range trade.History {
		if history.Type != entrySide || history.Quantity <= 0 {
			continue
		}
		if history.Price <= accountingHistoryPriceCeiling {
			continue
		}

		orderID := history.OrderId
		if orderID == 0 {
			// Legacy rows may not carry an exchange order id. They are separate
			// persisted entries, so give each one a stable synthetic identity.
			orderID = -int64(index + 1)
		}
		if _, exists := seenOrders[orderID]; exists {
			continue
		}
		seenOrders[orderID] = struct{}{}
		filledEntries++
	}

	return filledEntries
}

// SettingsIndexOrBase resolves which StrategySettings row governs a given
// ladder depth. The contract, per configuration semantics:
//   - a single configured row applies to every depth;
//   - a depth whose row exists uses exactly that row;
//   - a depth whose row does NOT exist falls back to row 0 (the base row) —
//     never to the last row.
func SettingsIndexOrBase(settings []aggragates.StrategySettings, index int) int {
	if index < 0 || index >= len(settings) {
		return 0
	}
	return index
}

// EffectiveStrategySettings returns the ladder hermes and agora must read for
// this trade: the per-trade override when one carries at least one row,
// otherwise the strategy pair ladder.
func EffectiveStrategySettings(trade aggragates.Trades) []aggragates.StrategySettings {
	if trade.SettingsOverride != nil && len(trade.SettingsOverride.StrategySettings) > 0 {
		return trade.SettingsOverride.StrategySettings
	}
	return trade.StrategyPair.StrategySettings
}

// EffectiveParams returns the strategy params for this trade: the override
// params when set (nil keeps the base strategy params), else the strategy's.
func EffectiveParams(trade aggragates.Trades) aggragates.StrategyParams {
	if trade.SettingsOverride != nil && trade.SettingsOverride.Params != nil {
		return *trade.SettingsOverride.Params
	}
	return trade.Strategy.Params
}

// ApplySettingsOverride returns a copy of the trade whose StrategyPair ladder
// and Strategy params are the effective ones, so every downstream reader of
// trade.StrategyPair.StrategySettings / trade.Strategy.Params sees the
// override without being changed. The ladder slice is cloned so callers can
// never alias the override's backing array. Call it once at the boundary
// (after the DB/cache load) and never persist the overlaid copy's
// StrategyPair or Strategy back to their tables.
func ApplySettingsOverride(trade aggragates.Trades) aggragates.Trades {
	if trade.SettingsOverride == nil {
		return trade
	}

	if len(trade.SettingsOverride.StrategySettings) > 0 {
		ladder := make([]aggragates.StrategySettings, len(trade.SettingsOverride.StrategySettings))
		copy(ladder, trade.SettingsOverride.StrategySettings)
		trade.StrategyPair.StrategySettings = ladder
	}

	if trade.SettingsOverride.Params != nil {
		trade.Strategy.Params = *trade.SettingsOverride.Params
	}

	return trade
}

// ApplySettingsOverrides overlays every trade of the slice; the input slice is
// left untouched.
func ApplySettingsOverrides(list []aggragates.Trades) []aggragates.Trades {
	overlaid := make([]aggragates.Trades, len(list))
	for index, trade := range list {
		overlaid[index] = ApplySettingsOverride(trade)
	}
	return overlaid
}
