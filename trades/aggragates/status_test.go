package aggragates

import (
	"slices"
	"testing"
)

// The open statuses are the ones the engines tick, active and hold; the others are not open.
func TestTheOpenStatusesAreActiveAndHold(t *testing.T) {
	if got := OpenStatuses(); !slices.Equal(got, []Status{Active, Hold}) {
		t.Fatalf("OpenStatuses = %v, want active and hold", got)
	}
	for status, want := range map[Status]bool{Active: true, Hold: true, Blocked: false, Paused: false, Closed: false, Impasse: false, InPosition: false, "": false} {
		if got := status.IsOpen(); got != want {
			t.Errorf("%q.IsOpen() = %v, want %v", status, got, want)
		}
	}
	if Hold != "hold" {
		t.Fatalf("the hold status is stored as %q, want hold", Hold)
	}
}
