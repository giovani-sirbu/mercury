package helpers

import "testing"

func TestToFixedRoundsDownAtRequestedPrecision(t *testing.T) {
	cases := []struct {
		name      string
		input     float64
		precision int
		want      float64
	}{
		{"two decimals truncates extras", 1.23456, 2, 1.23},
		{"floors instead of rounding", 1.239, 2, 1.23},
		{"zero precision returns floor", 9.99, 0, 9},
		{"negative number floors toward minus infinity", -1.234, 2, -1.24},
		{"already exact value unchanged", 5, 4, 5},
		{"zero stays zero", 0, 5, 0},
		{"eight decimals keeps precision", 0.123456789, 8, 0.12345678},
		{"tiny value at eight decimals", 0.00000001, 8, 0.00000001},
		{"large number keeps two decimals", 999999.999, 2, 999999.99},
	}

	for _, tc := range cases {
		got := ToFixed(tc.input, tc.precision)
		if got != tc.want {
			t.Errorf("%s: ToFixed(%v, %d) = %v, want %v", tc.name, tc.input, tc.precision, got, tc.want)
		}
	}
}
