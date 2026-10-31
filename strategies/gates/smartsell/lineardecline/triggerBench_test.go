package lineardecline

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
	"testing"
)

// benchBars is how many bars the series of the benchmark holds: as long a window as
// sophos serves the rule. benchFillBars are the bars of the deep ladder's fills, all
// inside it, so the chords run over most of the series.
const benchBars = 360

var benchFillBars = []int{150, 170, 200, 240, 280, 320, 355}

// benchLadder is the deep ladder of benchFillBars on the line of that slope, in
// percent of the first fill's price per hour, and benchBars bars of it.
func benchLadder(slope float64) ([]series.Fill, series.Series) {
	fills := make([]series.Fill, len(benchFillBars))
	for index, bar := range benchFillBars {
		fills[index] = fillIn(bar, linePrice(slope, float64(bar-benchFillBars[0])))
	}
	tape := series.Series{Opens: make([]int64, benchBars), Closes: make([]float64, benchBars)}
	for index := range benchBars {
		tape.Opens[index] = barOpenAt(index)
		tape.Closes[index] = linePrice(slope, float64(index+1-benchFillBars[0])-fillOffset.Hours())
	}
	return fills, tape
}

// BenchmarkTrigger is the print the gate asks on every tick of a watched ladder, on
// a series as long as sophos serves it: a hold, which formats the reasons once; a
// miss with the trend met, which is the pass over the bars and nothing else; and a
// miss whose trend is short, which looks at nothing. Not run by default:
//
//	go test -run '^$' -bench Trigger -benchmem ./strategies/gates/smartsell/lineardecline/
func BenchmarkTrigger(b *testing.B) {
	slope := -fall / float64(benchFillBars[len(benchFillBars)-1]-benchFillBars[len(benchFillBars)-Window])
	for _, tc := range []struct {
		name  string
		slope float64
		trend series.Trend
		holds bool
	}{
		{"hold", slope, bearishTrend, true},
		{"miss", -slope, bearishTrend, false},
		{"trend short", slope, shortTrend, false},
	} {
		fills, tape := benchLadder(tc.slope)
		if reading, holds := Trigger(fills, tape, tc.trend); holds != tc.holds || (tc.name == "miss" && reading.EndAt == 0) {
			b.Skipf("%s: fixture drifted under the constants in force, got holds %v, %+v", tc.name, holds, reading)
		}
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				Trigger(fills, tape, tc.trend)
			}
		})
	}
}
