package dynamicparams_test

import (
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/gates/dynamicparams"
)

// The opened row byte for byte: every engine writes this text, agora filters
// on its prefix and OpenedRaise reads the ladder's rows back from it. Only
// the parts that raise something are named, and an amount carries no
// trailing zeros.
func TestOpenedMessageBytes(t *testing.T) {
	cases := []struct {
		name   string
		points float64
		depths int
		want   string
	}{
		{"both", 0.4, 1, "dynamic params: opened raised, percentage +0.4 and depths +1 on every row"},
		{"the percentage alone", 0.4, 0, "dynamic params: opened raised, percentage +0.4 on every row"},
		{"the depths alone", 0, 1, "dynamic params: opened raised, depths +1 on every row"},
		{"a whole percentage and more depths", 1, 2, "dynamic params: opened raised, percentage +1 and depths +2 on every row"},
		{"no trailing zeros", 0.50, 0, "dynamic params: opened raised, percentage +0.5 on every row"},
		{"a finer amount", 0.125, 3, "dynamic params: opened raised, percentage +0.125 and depths +3 on every row"},
	}

	for _, c := range cases {
		if got := dynamicparams.OpenedMessage(c.points, c.depths); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

// The prefix and the marker stay byte-stable, and every opened row opens with
// the prefix followed by the marker, which is how agora and OpenedRaise find
// it.
func TestOpenedRowOpensWithThePrefixAndTheMarker(t *testing.T) {
	if dynamicparams.RowPrefix != "dynamic params:" {
		t.Fatalf("RowPrefix = %q, want the byte-stable %q", dynamicparams.RowPrefix, "dynamic params:")
	}
	if dynamicparams.OpenedMarker != "opened raised" {
		t.Fatalf("OpenedMarker = %q, want the byte-stable %q", dynamicparams.OpenedMarker, "opened raised")
	}

	head := dynamicparams.RowPrefix + " " + dynamicparams.OpenedMarker + ", "
	for _, amounts := range raiseAmounts {
		if message := dynamicparams.OpenedMessage(amounts.points, amounts.depths); !strings.HasPrefix(message, head) {
			t.Errorf("%q does not open with %q", message, head)
		}
	}
}
