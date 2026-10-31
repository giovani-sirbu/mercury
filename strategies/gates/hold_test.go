package gates

import "testing"

// A hold is a refusal by its reason alone: the key names the row, it never makes a hold.
func TestAHoldIsHeldByItsReason(t *testing.T) {
	for _, c := range []struct {
		hold Hold
		held bool
	}{
		{Hold{}, false},
		{Hold{Key: "HOLD_AI"}, false},
		{Hold{Reason: "AI market is bearish"}, true},
		{Hold{Reason: "AI market is bearish", Key: "HOLD_AI"}, true},
	} {
		if got := c.hold.Held(); got != c.held {
			t.Errorf("%+v.Held() = %v, want %v", c.hold, got, c.held)
		}
	}
}
