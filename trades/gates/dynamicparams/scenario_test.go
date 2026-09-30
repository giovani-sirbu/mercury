package dynamicparams

import (
	"reflect"
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// scenarioReads is a run of ticks whose reads flip the way the forming daily
// bar can flip them on any close of the chart bar: nothing read, both bearish
// and staying so, mixed, a bull turn, an outage over stale bearish reads, a
// neutral read, mixed on the other row, both bearish again, mixed, a bull
// turn and both bearish once more.
var scenarioReads = []aggragates.DynamicParamsIndicators{
	{},
	{Timeframe: "1D", Guppy: -1, BMSB: -1, Valid: true},
	{Timeframe: "1D", Guppy: -1, BMSB: -1, Valid: true},
	{Timeframe: "1D", Guppy: -1, BMSB: 0, Valid: true},
	{Timeframe: "1D", Guppy: 1, BMSB: 1, Valid: true},
	{Timeframe: "1D", Guppy: -1, BMSB: -1},
	{Timeframe: "1D", Guppy: 0, BMSB: 0, Valid: true},
	{Timeframe: "1D", Guppy: 1, BMSB: -1, Valid: true},
	{Timeframe: "1D", Guppy: -1, BMSB: -1, Valid: true},
	{Timeframe: "1D", Guppy: -1, BMSB: 1, Valid: true},
	{Timeframe: "1D", Guppy: 1, BMSB: 1, Valid: true},
	{Timeframe: "1D", Guppy: -1, BMSB: -1, Valid: true},
}

// scenarioLadders are the ladders of the run, one after the other: each is
// judged from its first tick, its first entry fills at the end of the tick
// fillsAt names, and it closes after its last tick, when the next opens.
var scenarioLadders = []struct {
	name    string
	first   int
	fillsAt int
	last    int
}{
	{"a ladder whose first entry is held until the reads turn", 0, 2, 5},
	{"a ladder that opens on its configured rows", 6, 6, 8},
	{"the next ladder, opening on the then-current reads", 9, 9, 11},
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

// scenarioAmounts is what the rule adds for a count of bearish reads, apart
// from raiseFor and increaseAmounts: both raise the percentage and the
// depths, one raises what the mixed increase names, none raises nothing.
func scenarioAmounts(bearish int, mixed Increase) (float64, int) {
	percentage, depths := false, false
	switch bearish {
	case 2:
		percentage, depths = true, true
	case 1:
		percentage = mixed == IncreasePercentage || mixed == IncreaseBoth
		depths = mixed == IncreaseDepths || mixed == IncreaseBoth
	}

	var points float64
	if percentage {
		points = BearPercentagePoints
	}
	var added int
	if depths {
		added = BearDepths
	}
	return points, added
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

// scenarioTrade is a flagged long spot parent before its first entry, on the
// stored rows.
func scenarioTrade(stored []aggragates.StrategySettings) aggragates.Trades {
	trade := aggragates.Trades{PositionType: "new"}
	trade.Strategy.TradeType = aggragates.Spot
	trade.Strategy.Params.DynamicParams = true
	trade.StrategyPair.StrategySettings = stored
	return trade
}

// Tick by tick over a run of flipping reads, under every increase the mixed
// tier can name, ladder after ladder: a ladder consults the reads only until
// its first entry fills, and writes its opened row on the first of those
// ticks whose reads raise something, naming the amounts the constants give
// them. From then on — through bull turns, outages, mixed and both-bearish
// reads — it trades the configured rows raised by exactly those amounts and
// writes nothing more; a ladder that opened on reads raising nothing trades
// the very configured slice for life. The next ladder opens on the reads of
// its own first tick. The configured rows never move. Under the shipped
// MixedIncrease the calls the engines make (Opening, RaisedSettings,
// OpenedRaise) answer the same run the same way.
func TestScenarioALadderTradesTheRaiseItOpenedWith(t *testing.T) {
	for _, mixed := range []Increase{IncreasePercentage, IncreaseDepths, IncreaseBoth, IncreaseNone} {
		stored := scenarioLadder()
		configured := scenarioLadder()
		raisedLadders, configuredLadders := 0, 0

		for _, plan := range scenarioLadders {
			trade := scenarioTrade(stored)
			openedAt := -1
			var points float64
			var depths int

			for tick := plan.first; tick <= plan.last; tick++ {
				reads := scenarioReads[tick]
				if openedAt < 0 && tick <= plan.fillsAt {
					if tickPoints, tickDepths := scenarioAmounts(scenarioBearish(reads), mixed); tickPoints != 0 || tickDepths != 0 {
						openedAt, points, depths = tick, tickPoints, tickDepths
					}
				}

				message, ok := openingFor(trade, reads, mixed)
				if ok != (tick == openedAt) {
					t.Fatalf("mixed %q, %s, tick %d: the ladder was handed a row %v, want %v", mixed, plan.name, tick, ok, tick == openedAt)
				}
				if mixed == MixedIncrease {
					if engineMessage, engineOK := Opening(trade, reads); engineMessage != message || engineOK != ok {
						t.Fatalf("tick %d: Opening = %q, %v, want %q, %v", tick, engineMessage, engineOK, message, ok)
					}
				}
				if ok {
					if message != OpenedMessage(points, depths) {
						t.Fatalf("mixed %q, %s, tick %d: the row %q does not name the amounts %v and %d", mixed, plan.name, tick, message, points, depths)
					}
					trade.Logs = append(trade.Logs, aggragates.TradesLogs{Message: message, Type: aggragates.LOG_INFO})
				}

				rows, raised := RaisedSettings(trade)
				opened := openedAt >= 0 && tick >= openedAt
				if raised != opened {
					t.Fatalf("mixed %q, %s, tick %d: RaisedSettings raised %v, want %v", mixed, plan.name, tick, raised, opened)
				}
				if !opened && &rows[0] != &stored[0] {
					t.Fatalf("mixed %q, %s, tick %d: a ladder on its configured rows must trade the very configured slice", mixed, plan.name, tick)
				}
				for index, row := range configured {
					if opened {
						row.Percentage += points
						row.Depths += float64(depths)
					}
					if !reflect.DeepEqual(rows[index], row) {
						t.Fatalf("mixed %q, %s, tick %d row %d: %+v, want %+v", mixed, plan.name, tick, index, rows[index], row)
					}
				}
				wantPoints, wantDepths := 0.0, 0
				if opened {
					wantPoints, wantDepths = points, depths
				}
				if gotPoints, gotDepths, gotOpened := OpenedRaise(trade); gotOpened != opened || gotPoints != wantPoints || gotDepths != wantDepths {
					t.Fatalf("mixed %q, %s, tick %d: OpenedRaise = %v, %d, %v, want %v, %d, %v", mixed, plan.name, tick, gotPoints, gotDepths, gotOpened, wantPoints, wantDepths, opened)
				}
				if !reflect.DeepEqual(stored, configured) {
					t.Fatalf("mixed %q, %s, tick %d: the configured rows moved: %+v", mixed, plan.name, tick, stored)
				}

				if tick == plan.fillsAt {
					trade.History = append(trade.History, aggragates.TradesHistory{Type: "BUY", Quantity: 1, Price: 100, OrderId: int64(tick + 1)})
					trade.PositionType = "buy"
					trade.PositionPrice = 100
				}
			}

			rowCount := 0
			for _, row := range trade.Logs {
				if strings.HasPrefix(row.Message, RowPrefix) {
					rowCount++
				}
			}
			wantRows := 0
			if openedAt >= 0 {
				wantRows = 1
				raisedLadders++
			} else {
				configuredLadders++
			}
			if rowCount != wantRows {
				t.Fatalf("mixed %q, %s: %d rows written, want %d", mixed, plan.name, rowCount, wantRows)
			}
		}

		if mixed == MixedIncrease && (raisedLadders == 0 || configuredLadders == 0) {
			t.Fatalf("fixture drifted: the run under the shipped MixedIncrease must open a ladder raised and one on its configured rows, got %d and %d", raisedLadders, configuredLadders)
		}
	}
}
