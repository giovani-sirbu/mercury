package dynamicparams_test

import (
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/gates/dynamicparams"
)

// The opened row byte for byte: every engine writes this text and agora
// filters on its prefix; nothing reads the ladder's rows back from it. Only the
// parts that raise something are named, one amount each, joined with a comma,
// and an amount carries no trailing zeros.
func TestOpenedMessageBytes(t *testing.T) {
	cases := []struct {
		name   string
		points float64
		depths int
		want   string
	}{
		{"both", 0.4, 1, "dynamicParams: volatility expected, strategy params adjusted: percentage +0.4, depths +1"},
		{"the percentage alone", 0.4, 0, "dynamicParams: volatility expected, strategy params adjusted: percentage +0.4"},
		{"the depths alone", 0, 1, "dynamicParams: volatility expected, strategy params adjusted: depths +1"},
		{"a whole percentage and more depths", 1, 2, "dynamicParams: volatility expected, strategy params adjusted: percentage +1, depths +2"},
		{"no trailing zeros", 0.50, 0, "dynamicParams: volatility expected, strategy params adjusted: percentage +0.5"},
		{"a finer amount", 0.125, 3, "dynamicParams: volatility expected, strategy params adjusted: percentage +0.125, depths +3"},
	}

	for _, c := range cases {
		if got := dynamicparams.OpenedMessage(c.points, c.depths); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

// The prefix and the marker stay byte-stable, and every opened row opens with
// the prefix followed by the marker and a colon, which is how agora finds it.
func TestOpenedRowOpensWithThePrefixAndTheMarker(t *testing.T) {
	if dynamicparams.RowPrefix != "dynamicParams:" {
		t.Fatalf("RowPrefix = %q, want the byte-stable %q", dynamicparams.RowPrefix, "dynamicParams:")
	}
	if dynamicparams.OpenedMarker != "volatility expected, strategy params adjusted" {
		t.Fatalf("OpenedMarker = %q, want the byte-stable %q", dynamicparams.OpenedMarker, "volatility expected, strategy params adjusted")
	}

	head := dynamicparams.RowPrefix + " " + dynamicparams.OpenedMarker + ": "
	for _, amounts := range raiseAmounts {
		if message := dynamicparams.OpenedMessage(amounts.points, amounts.depths); !strings.HasPrefix(message, head) {
			t.Errorf("%q does not open with %q", message, head)
		}
	}
}

// The row is as long as the parts it names and says nothing else: neither the
// old closing words nor a joining word, whatever the amounts.
func TestOpenedMessageHasNoClosingWordsAndNoJoiningWord(t *testing.T) {
	for name, amounts := range raiseAmounts {
		message := dynamicparams.OpenedMessage(amounts.points, amounts.depths)
		for _, word := range []string{" on every row", " and ", "opened raised"} {
			if strings.Contains(message, word) {
				t.Errorf("%s: %q still carries %q", name, message, word)
			}
		}
	}
}
