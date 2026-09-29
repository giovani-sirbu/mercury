package dynamicparams

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// scenarioReads is a run of ticks whose reads flip the way the forming daily
// bar can flip them on any close of the chart bar: nothing read, both bearish
// and staying so, the bearish read passing from one row to the other, a
// neutral read, a bull turn, both bearish again, an outage over stale bearish
// reads, and both bearish once more.
var scenarioReads = []aggragates.DynamicParamsIndicators{
	{},
	{Timeframe: "1D", Guppy: -1, BMSB: -1, Valid: true},
	{Timeframe: "1D", Guppy: -1, BMSB: -1, Valid: true},
	{Timeframe: "1D", Guppy: -1, BMSB: 0, Valid: true},
	{Timeframe: "1D", Guppy: 1, BMSB: -1, Valid: true},
	{Timeframe: "1D", Guppy: 0, BMSB: 0, Valid: true},
	{Timeframe: "1D", Guppy: -1, BMSB: 1, Valid: true},
	{Timeframe: "1D", Guppy: 1, BMSB: 1, Valid: true},
	{Timeframe: "1D", Guppy: -1, BMSB: -1, Valid: true},
	{Timeframe: "1D", Guppy: -1, BMSB: -1},
	{Timeframe: "1D", Guppy: -1, BMSB: -1, Valid: true},
}

// scenarioBearish counts a block's bearish reads the way the flag's rule is
// stated, apart from TierOf: a read counts only when it is exactly bearish
// and the block was read.
func scenarioBearish(reads aggragates.DynamicParamsIndicators) int {
	if !reads.Valid {
		return 0
	}
	count := 0
	for _, read := range []int{reads.Guppy, reads.BMSB} {
		if read == -1 {
			count++
		}
	}
	return count
}

// scenarioRaises is what the rule raises for a count of bearish reads, apart
// from raiseFor: both raise the percentage and the depths, one raises what the
// mixed increase names, none raises nothing.
func scenarioRaises(bearish int, mixed Increase) (percentage bool, depths bool) {
	switch bearish {
	case 2:
		return true, true
	case 1:
		return mixed == IncreasePercentage || mixed == IncreaseBoth, mixed == IncreaseDepths || mixed == IncreaseBoth
	default:
		return false, false
	}
}

// scenarioLadder is a row-per-depth ladder whose rows differ in every field a
// raise could touch, one of them carrying its own timeframe list.
func scenarioLadder() []aggragates.StrategySettings {
	return []aggragates.StrategySettings{
		{Tolerance: 0.25, MinDepths: 6, Depths: 8, Percentage: 2, Multiplier: 2, TrailingTakeProfit: 0.1, ImpasseDepth: 4, Timeframes: aggragates.Timeframes{Values: []string{"1h"}}},
		{Tolerance: 0.3, MinDepths: 5, Depths: 7, Percentage: 2.25, Multiplier: 1.8, TrailingTakeProfit: 0.2, ImpasseDepth: 3},
		{Tolerance: 0.35, MinDepths: 4, Depths: 6, Percentage: 3, Multiplier: 1.5, TrailingTakeProfit: 0.3, ImpasseDepth: 2, InitialBid: 2},
	}
}

// Tick by tick over a run of flipping reads, under every increase the mixed
// tier can name: each tick trades the configured rows plus exactly the amounts
// its own reads name — on every row, in the named fields only — and never the
// rows a tick before it raised; a tick that raises nothing trades the very
// configured slice, and one that raises trades a copy. The configured rows
// never move. An edge falls on exactly the ticks whose effective increase
// differs from the tick before — a tier change that keeps it is none — and
// its row names what that tick raises. Under the shipped MixedIncrease the
// calls the engines make (RaisedSettings, Adjust, Changed, TransitionMessage)
// answer the same run the same way.
func TestScenarioEveryTickRaisesTheConfiguredRowsAfresh(t *testing.T) {
	points := "percentage +" + strconv.FormatFloat(BearPercentagePoints, 'f', -1, 64)
	depths := "depths +" + strconv.Itoa(BearDepths)

	for _, mixed := range []Increase{IncreasePercentage, IncreaseDepths, IncreaseBoth, IncreaseNone} {
		stored := scenarioLadder()
		configured := scenarioLadder()
		trade := aggragates.Trades{PositionType: "buy", PositionPrice: 100}
		trade.Strategy.TradeType = aggragates.Spot
		trade.Strategy.Params.DynamicParams = true
		trade.StrategyPair.StrategySettings = stored

		var previous aggragates.DynamicParamsIndicators
		edges := 0
		for tick, reads := range scenarioReads {
			raisesPercentage, raisesDepths := scenarioRaises(scenarioBearish(reads), mixed)
			changes := (raisesPercentage && BearPercentagePoints != 0) || (raisesDepths && BearDepths != 0)

			rows := raiseRows(stored, raiseFor(TierOf(reads), mixed))
			if sameSlice := &rows[0] == &stored[0]; sameSlice == changes {
				t.Fatalf("mixed %q tick %d: raising %v, handed the configured slice itself %v", mixed, tick, changes, sameSlice)
			}
			for index, row := range configured {
				if raisesPercentage {
					row.Percentage += BearPercentagePoints
				}
				if raisesDepths {
					row.Depths += float64(BearDepths)
				}
				if !reflect.DeepEqual(rows[index], row) {
					t.Fatalf("mixed %q tick %d row %d: %+v, want the configured row raised once %+v", mixed, tick, index, rows[index], row)
				}
			}
			if !reflect.DeepEqual(stored, configured) {
				t.Fatalf("mixed %q tick %d: the configured rows moved: %+v", mixed, tick, stored)
			}

			wasPercentage, wasDepths := scenarioRaises(scenarioBearish(previous), mixed)
			edge := wasPercentage != raisesPercentage || wasDepths != raisesDepths
			if got := changedFor(previous, reads, mixed); got != edge {
				t.Fatalf("mixed %q tick %d: changedFor = %v, want %v", mixed, tick, got, edge)
			}
			message := transitionMessageFor(reads, mixed)
			if edge {
				edges++
				named := strings.Contains(message, points) == (raisesPercentage && BearPercentagePoints != 0) &&
					strings.Contains(message, depths) == (raisesDepths && BearDepths != 0) &&
					strings.HasSuffix(message, "configured rows") == !changes
				if !named || !strings.HasPrefix(message, TransitionPrefix) {
					t.Fatalf("mixed %q tick %d: the edge row %q does not name what the tick raises", mixed, tick, message)
				}
			}

			if mixed == MixedIncrease {
				engineRows, raised := RaisedSettings(trade, reads)
				if raised != changes || !reflect.DeepEqual(engineRows, rows) {
					t.Fatalf("tick %d: RaisedSettings = %+v, %v, want %+v, %v", tick, engineRows, raised, rows, changes)
				}
				if adjusted := Adjust(stored, reads); !reflect.DeepEqual(adjusted, rows) {
					t.Fatalf("tick %d: Adjust = %+v, want %+v", tick, adjusted, rows)
				}
				if Changed(previous, reads) != edge || TransitionMessage(reads) != message {
					t.Fatalf("tick %d: Changed or TransitionMessage part from the run under the shipped MixedIncrease", tick)
				}
			}

			previous = reads
		}

		if edges < 4 {
			t.Fatalf("fixture drifted: the run under mixed %q has %d edges, want the raise to open and close at least twice", mixed, edges)
		}
	}
}
