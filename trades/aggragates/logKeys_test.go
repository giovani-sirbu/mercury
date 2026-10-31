package aggragates

import (
	"regexp"
	"strings"
	"testing"
)

// Every key is distinct, upper snake case and within the column's 64 characters; the holds share
// the HOLD_ head the hold reason queries read.
func TestTheLogKeysAreDistinctUpperSnakeCodes(t *testing.T) {
	keys := []string{
		LogKeyHoldDepthPriority, LogKeyHoldFirstFill, LogKeyHoldFirstFillVerdict, LogKeyHoldDepthSpacing,
		LogKeyHoldSmartSellEntry, LogKeyHoldAI, LogKeyHoldPattern, LogKeyHoldFibonacci,
		LogKeyCooldownFirstFillEntered, LogKeyDynamicParamsOpened, LogKeyInsufficientFunds,
		LogKeyExchangeError, LogKeyActionFailed, LogKeyTradeUnblocked, LogKeyTradePausedDueInactivity,
		LogKeySellCanceled,
	}
	code := regexp.MustCompile(`^[A-Z]+(_[A-Z]+)*$`)
	seen := map[string]bool{}
	for _, key := range keys {
		if !code.MatchString(key) || len(key) > 64 || seen[key] {
			t.Errorf("%q must be a distinct upper snake code within 64 characters", key)
		}
		seen[key] = true
	}
	for _, hold := range keys[:8] {
		if !strings.HasPrefix(hold, LogKeyHoldPrefix) {
			t.Errorf("%q must open with %s", hold, LogKeyHoldPrefix)
		}
	}
	heads := [][2]string{
		{LogKeySmartSellPrefix, "SMART_SELL_"},
		{LogKeyHoldPrefix, "HOLD_"},
		{LogKeyCooldownPrefix, "COOLDOWN_"},
		{LogKeyDynamicParamsPrefix, "DYNAMIC_PARAMS_"},
	}
	for _, head := range heads {
		if head[0] != head[1] {
			t.Errorf("a gate rows' head is %q, want %q", head[0], head[1])
		}
	}
}

// A strategy gate's row is told by its key's head; a transition, a failure, agora's rows and "" are not.
func TestIsGateLogKeyReadsTheKeysHead(t *testing.T) {
	gates := []string{
		LogKeyHoldDepthPriority, LogKeyHoldFirstFill, LogKeyHoldFirstFillVerdict, LogKeyHoldDepthSpacing,
		LogKeyHoldSmartSellEntry, LogKeyHoldAI, LogKeyHoldPattern, LogKeyHoldFibonacci,
		LogKeyCooldownFirstFillEntered, LogKeyDynamicParamsOpened, LogKeySmartSellPrefix + "SOLD",
	}
	for _, key := range gates {
		if !IsGateLogKey(key) {
			t.Errorf("%q must be a gate's key", key)
		}
	}
	others := []string{
		"NEW_TO_BUY", "TRAILING_BUY_TO_TAKE_LOSS", LogKeyInsufficientFunds, LogKeyExchangeError,
		LogKeyActionFailed, LogKeyTradeUnblocked, LogKeyTradePausedDueInactivity, LogKeySellCanceled, "",
	}
	for _, key := range others {
		if IsGateLogKey(key) {
			t.Errorf("%q must not be a gate's key", key)
		}
	}
}
