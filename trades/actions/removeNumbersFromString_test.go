package actions

import "testing"

func TestRemoveNumbersFromStringStripsDigits(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"abc123", "abc"},
		{"0x42deadbeef", "xdeadbeef"},
		{"no-digits", "no-digits"},
		{"", ""},
		{"Trade #42: profit 12.34", "Trade #: profit ."},
	}

	for _, tc := range cases {
		got := RemoveNumbersFromString(tc.input)
		if got != tc.want {
			t.Errorf("RemoveNumbersFromString(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
