package helpers

import "testing"

func TestIntervalToMinutesParsesAllUnits(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"15m", 15},
		{"1h", 60},
		{"4h", 240},
		{"1d", 1440},
		{"1w", 10080},
		{"30M", 30}, // case-insensitive unit
	}

	for _, tc := range cases {
		if got := IntervalToMinutes(tc.input); got != tc.want {
			t.Errorf("IntervalToMinutes(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestIntervalToMinutesReturnsMinusOneForInvalidInput(t *testing.T) {
	invalid := []string{"", "m", "0h", "-5h", "abc", "10x", "1"}
	for _, input := range invalid {
		if got := IntervalToMinutes(input); got != -1 {
			t.Errorf("IntervalToMinutes(%q) = %d, want -1", input, got)
		}
	}
}
