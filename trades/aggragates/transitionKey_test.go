package aggragates

import "testing"

// UpperSnake inserts an underscore before each upper-case letter after the first and upper-cases
// the word; "" stays "".
func TestUpperSnakeCasesEveryState(t *testing.T) {
	for word, want := range map[string]string{
		"":                        "",
		"new":                     "NEW",
		"buy":                     "BUY",
		"trailingBuy":             "TRAILING_BUY",
		"takeProfit":              "TAKE_PROFIT",
		"takeLoss":                "TAKE_LOSS",
		"sellParent":              "SELL_PARENT",
		"forceTrailingBuy":        "FORCE_TRAILING_BUY",
		"forceTrailingTakeProfit": "FORCE_TRAILING_TAKE_PROFIT",
	} {
		if got := UpperSnake(word); got != want {
			t.Errorf("UpperSnake(%q) = %q, want %q", word, got, want)
		}
	}
}

// TransitionKey is the old state and the new one in upper snake case: every transition the engines
// write, an impasse child's "" → buy as _TO_BUY, every key within the column's 64 characters.
func TestTransitionKeyNamesEveryTransition(t *testing.T) {
	for _, c := range []struct {
		old, new, want string
	}{
		{"new", "buy", "NEW_TO_BUY"},
		{"buy", "buy", "BUY_TO_BUY"},
		{"buy", "trailingBuy", "BUY_TO_TRAILING_BUY"},
		{"trailingBuy", "trailingBuy", "TRAILING_BUY_TO_TRAILING_BUY"},
		{"trailingBuy", "buy", "TRAILING_BUY_TO_BUY"},
		{"buy", "takeProfit", "BUY_TO_TAKE_PROFIT"},
		{"takeProfit", "takeProfit", "TAKE_PROFIT_TO_TAKE_PROFIT"},
		{"takeProfit", "sell", "TAKE_PROFIT_TO_SELL"},
		{"takeProfit", "takeLoss", "TAKE_PROFIT_TO_TAKE_LOSS"},
		{"takeProfit", "buy", "TAKE_PROFIT_TO_BUY"},
		{"trailingBuy", "takeLoss", "TRAILING_BUY_TO_TAKE_LOSS"},
		{"trailingBuy", "sell", "TRAILING_BUY_TO_SELL"},
		{"buy", "takeLoss", "BUY_TO_TAKE_LOSS"},
		{"takeLoss", "takeLoss", "TAKE_LOSS_TO_TAKE_LOSS"},
		{"sell", "buy", "SELL_TO_BUY"},
		{"buy", "sell", "BUY_TO_SELL"},
		{"buy", "impasse", "BUY_TO_IMPASSE"},
		{"impasse", "sellParent", "IMPASSE_TO_SELL_PARENT"},
		{"sellParent", "sellParent", "SELL_PARENT_TO_SELL_PARENT"},
		{"sellParent", "buy", "SELL_PARENT_TO_BUY"},
		{"new", "sell", "NEW_TO_SELL"},
		{"sell", "sell", "SELL_TO_SELL"},
		{"", "buy", "_TO_BUY"},
		{"buy", "forceTrailingBuy", "BUY_TO_FORCE_TRAILING_BUY"},
		{"forceTrailingTakeProfit", "forceTrailingTakeProfit", "FORCE_TRAILING_TAKE_PROFIT_TO_FORCE_TRAILING_TAKE_PROFIT"},
	} {
		got := TransitionKey(c.old, c.new)
		if got != c.want {
			t.Errorf("TransitionKey(%q, %q) = %q, want %q", c.old, c.new, got, c.want)
		}
		if len(got) > 64 {
			t.Errorf("%q is longer than the column's 64 characters", got)
		}
	}
}
