package aggragates

import "testing"

// SettledStatus is the status a tick leaves: held, hold over an open status; not held, active over
// hold; a blocked, paused, closed or unknown status is kept either way.
func TestSettledStatusMovesOnlyBetweenTheOpenStatuses(t *testing.T) {
	for _, c := range []struct {
		status Status
		held   bool
		want   Status
	}{
		{Active, true, Hold},
		{Hold, true, Hold},
		{Active, false, Active},
		{Hold, false, Active},
		{Blocked, true, Blocked},
		{Blocked, false, Blocked},
		{Paused, true, Paused},
		{Paused, false, Paused},
		{Closed, true, Closed},
		{"", true, ""},
		{"", false, ""},
	} {
		if got := SettledStatus(c.status, c.held); got != c.want {
			t.Errorf("SettledStatus(%q, %v) = %q, want %q", c.status, c.held, got, c.want)
		}
	}
}
