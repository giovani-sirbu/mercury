package aggragates

// The ladder states a trade's PositionType holds (trades.position_type).
const (
	PositionNew        = "new"
	PositionBuy        = "buy"
	PositionSell       = "sell"
	PositionTakeProfit = "takeProfit"
	// PositionTrailingBuy arms the next depth a step under the last fill, re-anchors lower and buys on the bounce.
	PositionTrailingBuy = "trailingBuy"
	// PositionTakeLoss sells the position under break even (the smartSell sale at a loss).
	PositionTakeLoss   = "takeLoss"
	PositionImpasse    = "impasse"
	PositionSellParent = "sellParent"
	// PositionForceTrailingBuy and PositionForceTrailingTakeProfit are the backtest's force-trailing re-anchors.
	PositionForceTrailingBuy        = "forceTrailingBuy"
	PositionForceTrailingTakeProfit = "forceTrailingTakeProfit"
	// PositionBuyForceTrailing is a lookup alias of the force-trailing logic, never stored.
	PositionBuyForceTrailing = "buyForceTrailing"
)
