package flatmarket

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
	"testing"
)

// BenchmarkTrigger is the print the gate asks on every tick of a watched ladder, on a
// window as long as sophos serves the gate: a hold, which formats the reasons once,
// on each side; a miss with the trend met, which is every step but the formatting,
// the pass over the bars included, and ends on the move; a miss on the stray, which
// spends the pass over the closes as well and ends on it; and a miss whose trend is
// short, which looks at no bar. Not run by default:
//
//	go test -run '^$' -bench Trigger -benchmem ./strategies/gates/smartsell/flatmarket/
func BenchmarkTrigger(b *testing.B) {
	long, inverse := rangeSideCases[0], rangeSideCases[1]
	over := window{quiet: rangeFixtureBars, movePct: overMove, side: long.side}.build(fixturePercentage)
	strayed := window{quiet: rangeFixtureBars, movePct: holdMove, strayPct: overStray, side: long.side}.build(fixturePercentage)
	for _, tc := range []struct {
		name  string
		side  string
		bars  Bars
		trend series.Trend
		holds bool
	}{
		{"hold", long.side, holding(long.side), long.met, true},
		{"hold inverse", inverse.side, holding(inverse.side), inverse.met, true},
		{"miss", long.side, over, long.met, false},
		{"miss on the stray", long.side, strayed, long.met, false},
		{"trend short", long.side, holding(long.side), long.short, false},
	} {
		reading := TriggerRange(tc.bars, fixturePercentage, tc.side, tc.trend)
		if reading.Holds != tc.holds || (tc.name == "miss" && reading.Of == 0) || (tc.name == "miss on the stray" && reading.StrayAt == 0) {
			b.Skipf("%s: fixture drifted under the constants in force, got %+v", tc.name, reading)
		}
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				TriggerRange(tc.bars, fixturePercentage, tc.side, tc.trend)
			}
		})
	}
}
