package dynamicparams_test

import (
	"reflect"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/gates/dynamicparams"
)

// The values the flag ships with. A retune changes them here on purpose,
// never by accident.
func TestConstantsArePinned(t *testing.T) {
	if dynamicparams.Interval != "1d" {
		t.Errorf("Interval = %q, want %q", dynamicparams.Interval, "1d")
	}
	if dynamicparams.BearPercentagePoints != 0.4 {
		t.Errorf("BearPercentagePoints = %v, want 0.4", dynamicparams.BearPercentagePoints)
	}
	if dynamicparams.BearDepths != 1 {
		t.Errorf("BearDepths = %v, want 1", dynamicparams.BearDepths)
	}
	if dynamicparams.MixedIncrease != dynamicparams.IncreasePercentage {
		t.Errorf("MixedIncrease = %q, want %q", dynamicparams.MixedIncrease, dynamicparams.IncreasePercentage)
	}
}

// Both amounts widen the ladder or leave it as it is, never narrow it: the
// backtest's skip gates keep reading the stored rows and skip only prints a
// ladder at least that wide does not act on.
func TestAmountsNeverNarrowTheLadder(t *testing.T) {
	if dynamicparams.BearPercentagePoints < 0 {
		t.Errorf("BearPercentagePoints = %v, must be at or above zero", dynamicparams.BearPercentagePoints)
	}
	if dynamicparams.BearDepths < 0 {
		t.Errorf("BearDepths = %v, must be at or above zero", dynamicparams.BearDepths)
	}
}

// MixedIncrease is one of the four increases: any other value would raise
// nothing without saying so.
func TestMixedIncreaseIsOneOfTheFourIncreases(t *testing.T) {
	known := map[dynamicparams.Increase]bool{
		dynamicparams.IncreasePercentage: true,
		dynamicparams.IncreaseDepths:     true,
		dynamicparams.IncreaseBoth:       true,
		dynamicparams.IncreaseNone:       true,
	}
	if !known[dynamicparams.MixedIncrease] {
		t.Fatalf("MixedIncrease = %q, want one of %v", dynamicparams.MixedIncrease, known)
	}
}

// Interval is a Binance kline interval, written the way the exchange writes
// it: sophos answers /smc-trend at any other — a dashboard label such as
// "1D" included — with a 400, which every engine reads as not read, so a slip
// here would open every ladder on its configured rows without a word.
func TestIntervalIsABinanceInterval(t *testing.T) {
	binance := map[string]bool{
		"1m": true, "3m": true, "5m": true, "15m": true, "30m": true,
		"1h": true, "2h": true, "4h": true, "6h": true, "8h": true, "12h": true,
		"1d": true, "3d": true, "1w": true, "1M": true,
	}
	if !binance[dynamicparams.Interval] {
		t.Fatalf("Interval = %q, want one of the Binance kline intervals %v", dynamicparams.Interval, binance)
	}
}

// BearDepths is a whole number of depths: a fraction added to a row's depths
// can keep ladder.CalculateInitialBid's walk off every half depth and refuse
// every new ladder.
func TestBearDepthsIsAnInteger(t *testing.T) {
	if kind := reflect.TypeOf(dynamicparams.BearDepths).Kind(); kind != reflect.Int {
		t.Fatalf("BearDepths is a %s, want an int", kind)
	}
}
