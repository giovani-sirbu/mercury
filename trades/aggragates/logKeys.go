package aggragates

import "strings"

// The messageKey of the rows that are not transitions (TradesLogs.MessageKey); a transition's is
// TransitionKey, a smartSell row's SMART_SELL_<GATE>_<EVENT> (smartsell.keyOf).
const (
	// The holds (gates.Hold.Key), one per gate that refuses.
	LogKeyHoldDepthPriority    = "HOLD_DEPTH_PRIORITY"
	LogKeyHoldFirstFill        = "HOLD_FIRST_FILL"
	LogKeyHoldFirstFillVerdict = "HOLD_FIRST_FILL_VERDICT"
	LogKeyHoldDepthSpacing     = "HOLD_DEPTH_SPACING"
	LogKeyHoldSmartSellEntry   = "HOLD_SMART_SELL_ENTRY"
	LogKeyHoldAI               = "HOLD_AI"
	LogKeyHoldPattern          = "HOLD_PATTERN"
	LogKeyHoldFibonacci        = "HOLD_FIBONACCI"
	// LogKeyCooldownFirstFillEntered is the first fill entering over the cooldown's reference.
	LogKeyCooldownFirstFillEntered = "COOLDOWN_FIRST_FILL_ENTERED"
	// LogKeyDynamicParamsOpened is the row a ladder opens its raised rows on.
	LogKeyDynamicParamsOpened = "DYNAMIC_PARAMS_OPENED"
	// The failures tradelog.SaveError writes: a funds refusal, an exchange error, any other.
	LogKeyInsufficientFunds = "INSUFFICIENT_FUNDS"
	LogKeyExchangeError     = "EXCHANGE_ERROR"
	LogKeyActionFailed      = "ACTION_FAILED"
	// The rows agora writes: an unblock, a pause for inactivity, a cancelled sell.
	LogKeyTradeUnblocked           = "TRADE_UNBLOCKED"
	LogKeyTradePausedDueInactivity = "TRADE_PAUSED_DUE_INACTIVITY"
	LogKeySellCanceled             = "SELL_CANCELED"
	// LogKeySmartSellPrefix heads every smartSell row's key.
	LogKeySmartSellPrefix = "SMART_SELL_"
	// LogKeyHoldPrefix heads every hold row's key.
	LogKeyHoldPrefix = "HOLD_"
	// LogKeyCooldownPrefix heads the cooldown rows' keys.
	LogKeyCooldownPrefix = "COOLDOWN_"
	// LogKeyDynamicParamsPrefix heads the dynamic params rows' keys.
	LogKeyDynamicParamsPrefix = "DYNAMIC_PARAMS_"
)

// IsGateLogKey is whether key opens with a strategy gate's head: a hold, a smartSell row, a cooldown row, a dynamic params row.
func IsGateLogKey(key string) bool {
	return strings.HasPrefix(key, LogKeyHoldPrefix) ||
		strings.HasPrefix(key, LogKeySmartSellPrefix) ||
		strings.HasPrefix(key, LogKeyCooldownPrefix) ||
		strings.HasPrefix(key, LogKeyDynamicParamsPrefix)
}
