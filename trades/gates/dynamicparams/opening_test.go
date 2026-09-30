package dynamicparams_test

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/dynamicparams"
)

// Blocks for each tier on the timeframe the tests read.
var (
	bothBearish = aggragates.DynamicParamsIndicators{Timeframe: "1D", Guppy: bearish, BMSB: bearish, Valid: true}
	mixedRead   = aggragates.DynamicParamsIndicators{Timeframe: "1D", Guppy: bullish, BMSB: bearish, Valid: true}
	notBearish  = aggragates.DynamicParamsIndicators{Timeframe: "1D", Guppy: bullish, BMSB: neutral, Valid: true}
	notRead     = aggragates.DynamicParamsIndicators{Timeframe: "1D", Guppy: bearish, BMSB: bearish}
)

// mixedRaises is what the mixed tier raises under the shipped MixedIncrease.
func mixedRaises() (percentage bool, depths bool) {
	switch dynamicparams.MixedIncrease {
	case dynamicparams.IncreasePercentage:
		return true, false
	case dynamicparams.IncreaseDepths:
		return false, true
	case dynamicparams.IncreaseBoth:
		return true, true
	default:
		return false, false
	}
}

// openingTrade is the flagged long spot parent before its first entry: `new`,
// no fill, no log row.
func openingTrade() aggragates.Trades {
	trade := flaggedTrade()
	trade.PositionType = "new"
	trade.PositionPrice = 0
	return trade
}

// constantAmounts is what the shipped constants add for an increase naming
// the percentage and/or the depths.
func constantAmounts(percentage, depths bool) (float64, int) {
	var points float64
	if percentage {
		points = dynamicparams.BearPercentagePoints
	}
	var added int
	if depths {
		added = dynamicparams.BearDepths
	}
	return points, added
}

// A ladder that opens on reads that raise something is handed the opened row
// naming exactly what they raise, at the amounts the constants name, and the
// row reads back as those amounts: both bearish raises the percentage and the
// depths, mixed what MixedIncrease names.
func TestOpeningWritesTheAmountsTheReadsRaise(t *testing.T) {
	mixedPercentage, mixedDepths := mixedRaises()
	cases := []struct {
		name       string
		reads      aggragates.DynamicParamsIndicators
		percentage bool
		depths     bool
	}{
		{"both bearish", bothBearish, true, true},
		{"mixed", mixedRead, mixedPercentage, mixedDepths},
	}

	for _, c := range cases {
		points, depths := constantAmounts(c.percentage, c.depths)
		raises := points != 0 || depths != 0

		trade := openingTrade()
		message, ok := dynamicparams.Opening(trade, c.reads)
		if ok != raises {
			t.Fatalf("%s: Opening answered %v, want %v", c.name, ok, raises)
		}
		if !raises {
			if message != "" {
				t.Fatalf("%s: a ladder the reads raise nothing on got the row %q", c.name, message)
			}
			continue
		}
		if want := dynamicparams.OpenedMessage(points, depths); message != want {
			t.Fatalf("%s: Opening = %q, want %q", c.name, message, want)
		}

		trade = withRows(trade, message)
		gotPoints, gotDepths, opened := dynamicparams.OpenedRaise(trade)
		if !opened || gotPoints != points || gotDepths != depths {
			t.Fatalf("%s: the row reads back as %v, %d, %v, want %v, %d", c.name, gotPoints, gotDepths, opened, points, depths)
		}
	}
}

// Reads that raise nothing open the ladder on its configured rows, and no
// row is written: the base tier, a block sophos did not read over stale
// bearish reads, and the zero block.
func TestOpeningAnswersNothingWhenTheReadsRaiseNothing(t *testing.T) {
	for name, reads := range map[string]aggragates.DynamicParamsIndicators{
		"not bearish":   notBearish,
		"not read":      notRead,
		"the zero read": {},
	} {
		if message, ok := dynamicparams.Opening(openingTrade(), reads); ok || message != "" {
			t.Errorf("%s: Opening = %q, %v, want nothing", name, message, ok)
		}
	}
}

// A trade the flag does not shape never gets the row, whatever the reads.
func TestOpeningAnswersNothingForATradeTheFlagDoesNotShape(t *testing.T) {
	cases := []struct {
		name   string
		change func(*aggragates.Trades)
	}{
		{"the flag off", func(trade *aggragates.Trades) { trade.Strategy.Params.DynamicParams = false }},
		{"an inverse ladder", func(trade *aggragates.Trades) { trade.Inverse = true }},
		{"a futures strategy", func(trade *aggragates.Trades) { trade.Strategy.TradeType = aggragates.Futures }},
		{"an impasse child", func(trade *aggragates.Trades) { trade.ParentID = 7 }},
	}

	for _, c := range cases {
		trade := openingTrade()
		c.change(&trade)
		if message, ok := dynamicparams.Opening(trade, bothBearish); ok || message != "" {
			t.Errorf("%s: Opening = %q, %v, want nothing", c.name, message, ok)
		}
	}
}

// The reads are consulted once per ladder. A first entry held or refused
// funds, and judged again on a later tick, already carries its opened row —
// plain or under a hold frame — and writes no second one, whatever the reads
// say now — even a row whose amounts it cannot read, which is still its
// opened row. A ladder with an entry fill has opened: with or without a row
// it is never judged again. No row the per-tick release wrote is an opened
// row, so none of them — alone or all together — stops a ladder from opening
// on the row the reads of its opening raise.
func TestOpeningAnswersOnlyUntilTheLadderOpens(t *testing.T) {
	opened := dynamicparams.OpenedMessage(dynamicparams.BearPercentagePoints, 0)
	filled := func(trade aggragates.Trades) aggragates.Trades {
		trade.History = []aggragates.TradesHistory{{Type: "BUY", Quantity: 1, Price: 100, OrderId: 1}}
		trade.PositionType = "buy"
		trade.PositionPrice = 100
		return trade
	}

	for name, trade := range map[string]aggragates.Trades{
		"a held first entry carrying its row":                   withRows(openingTrade(), opened),
		"a held first entry under a hold frame":                 withRows(openingTrade(), "Hold entry: "+opened),
		"a held first entry whose row names no amount it reads": withRows(openingTrade(), "dynamic params: opened raised, on every row"),
		"a ladder filled on its configured rows":                filled(openingTrade()),
		"a ladder filled on the rows it opened with":            filled(withRows(openingTrade(), opened)),
	} {
		for readsName, reads := range map[string]aggragates.DynamicParamsIndicators{
			"both bearish": bothBearish,
			"mixed":        mixedRead,
		} {
			if message, ok := dynamicparams.Opening(trade, reads); ok || message != "" {
				t.Errorf("%s, %s: Opening = %q, %v, want nothing", name, readsName, message, ok)
			}
		}
	}

	want := dynamicparams.OpenedMessage(dynamicparams.BearPercentagePoints, dynamicparams.BearDepths)
	carrying := map[string][]string{"all of the per-tick release's rows": earlierReleaseRows}
	for _, row := range earlierReleaseRows {
		carrying[row] = []string{row}
	}
	for name, rows := range carrying {
		if message, ok := dynamicparams.Opening(withRows(openingTrade(), rows...), bothBearish); !ok || message != want {
			t.Errorf("%s: Opening = %q, %v, want the ladder to open with %q", name, message, ok, want)
		}
	}
}
