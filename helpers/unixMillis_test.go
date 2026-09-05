package helpers

import "testing"

func TestUnixMillisNormalizesEveryUnitToMilliseconds(t *testing.T) {
	const want = int64(1_700_000_000_000)
	cases := []struct {
		name  string
		input int64
	}{
		{"seconds", 1_700_000_000},
		{"milliseconds", 1_700_000_000_000},
		{"microseconds", 1_700_000_000_000_000},
		{"nanoseconds", 1_700_000_000_000_000_000},
	}

	for _, tc := range cases {
		if got := UnixMillis(tc.input); got != want {
			t.Errorf("%s: UnixMillis(%d) = %d, want %d", tc.name, tc.input, got, want)
		}
	}
}

func TestUnixMillisTreatsSmallOrNegativeValuesAsUnset(t *testing.T) {
	for _, input := range []int64{0, -1, 1, 999_999_999} {
		if got := UnixMillis(input); got != 0 {
			t.Errorf("UnixMillis(%d) = %d, want 0", input, got)
		}
	}
}
