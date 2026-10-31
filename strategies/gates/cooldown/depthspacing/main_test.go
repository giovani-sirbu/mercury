package depthspacing

import (
	"os"
	"testing"
)

// TestMain pins the cascade's start to step 1, the profile every fixture here was written against, whatever the constants say; withStartStepFromDepth moves it per test.
func TestMain(m *testing.M) {
	startStepFromDepth, startStepOffset = false, 0
	os.Exit(m.Run())
}
