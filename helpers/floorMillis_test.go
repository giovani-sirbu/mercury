package helpers

import "testing"

func TestFloorMillisSnapsToTheStartOfTheWindow(t *testing.T) {
	cases := []struct {
		name    string
		ms      int64
		minutes int64
		want    int64
	}{
		{"five minute window", 1_700_000_123_456, 5, 1_700_000_100_000},
		{"fifteen minute window", 1_700_000_923_456, 15, 1_700_000_100_000},
		{"exact window start is unchanged", 1_700_000_100_000, 5, 1_700_000_100_000},
		{"one minute window", 1_700_000_059_999, 1, 1_700_000_040_000},
	}

	for _, tc := range cases {
		if got := FloorMillis(tc.ms, tc.minutes); got != tc.want {
			t.Errorf("%s: FloorMillis(%d, %d) = %d, want %d", tc.name, tc.ms, tc.minutes, got, tc.want)
		}
	}
}

func TestFloorMillisReturnsZeroWithoutAClockOrWindow(t *testing.T) {
	cases := []struct {
		ms      int64
		minutes int64
	}{
		{0, 5},
		{-1, 5},
		{1_700_000_123_456, 0},
		{1_700_000_123_456, -15},
	}

	for _, tc := range cases {
		if got := FloorMillis(tc.ms, tc.minutes); got != 0 {
			t.Errorf("FloorMillis(%d, %d) = %d, want 0", tc.ms, tc.minutes, got)
		}
	}
}
