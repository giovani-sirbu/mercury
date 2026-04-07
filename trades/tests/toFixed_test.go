package tests

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/actions"
)

func TestToFixed(t *testing.T) {
	tests := []struct {
		name      string
		num       float64
		precision int
		want      float64
	}{
		{"PositiveFloor", 1.23456, 2, 1.23},
		{"FloorNotRound", 1.239, 2, 1.23},
		{"Zero", 0.0, 5, 0.0},
		{"PrecisionZero", 5.999, 0, 5.0},
		{"HighPrecision", 0.00000001, 8, 0.00000001},
		{"LargeNumber", 999999.999, 2, 999999.99},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := actions.ToFixed(tt.num, tt.precision)
			AssertFloatEqual(t, got, tt.want, 1e-10, "ToFixed")
		})
	}
}
