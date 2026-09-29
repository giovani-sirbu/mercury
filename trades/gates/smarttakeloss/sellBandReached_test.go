package smarttakeloss

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The sell target is the band sophos serves, inclusive: at or over it the
// price sells, under it and against no band served — zero, or a band that is
// not a price — nothing does.
func TestSellBandReached(t *testing.T) {
	band := aggragates.SmartTakeLossIndicators{SlowDeclineSellBand: slowDeclineBand}
	for price, want := range map[float64]bool{slowDeclineBand: true, slowDeclineBand + 0.01: true, underTheBand: false} {
		if got := sellBandReached(price, band); got != want {
			t.Errorf("price %v against the band %v: %v, want %v", price, slowDeclineBand, got, want)
		}
	}
	for _, none := range []float64{0, -slowDeclineBand} {
		if sellBandReached(slowDeclineBand, aggragates.SmartTakeLossIndicators{SlowDeclineSellBand: none}) {
			t.Errorf("a band of %v must sell nothing", none)
		}
	}
}
