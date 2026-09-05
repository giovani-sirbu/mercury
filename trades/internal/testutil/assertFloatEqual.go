package testutil

import (
	"math"
	"testing"
)

// AssertFloatEqual fails the test if got and want differ by more than epsilon.
func AssertFloatEqual(t *testing.T, got, want, epsilon float64, msg string) {
	t.Helper()
	if math.Abs(got-want) > epsilon {
		t.Errorf("%s: got %f, want %f (epsilon %g)", msg, got, want, epsilon)
	}
}
