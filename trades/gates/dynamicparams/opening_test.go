package dynamicparams_test

import (
	"reflect"
	"testing"
	"time"

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
// no fill, no log row, no strategy event.
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

// A ladder that opens on reads that raise something is handed the amounts it
// opens with, exactly what they raise at the amounts the constants name, and
// the opened pair the engines write for them reads back as those amounts: both
// bearish raises the percentage and the depths, mixed what MixedIncrease names.
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
		opened, ok := dynamicparams.Opening(trade, c.reads)
		if ok != raises {
			t.Fatalf("%s: Opening answered %v, want %v", c.name, ok, raises)
		}
		if !raises {
			if opened != (dynamicparams.Opened{}) {
				t.Fatalf("%s: a ladder the reads raise nothing on got the amounts %+v", c.name, opened)
			}
			continue
		}
		if want := (dynamicparams.Opened{Points: points, Depths: depths}); opened != want {
			t.Fatalf("%s: Opening = %+v, want %+v", c.name, opened, want)
		}
		if want := dynamicparams.OpenedMessage(points, depths); opened.Message() != want {
			t.Fatalf("%s: Message = %q, want %q", c.name, opened.Message(), want)
		}

		row, event := opened.Rows(trade, 100, time.Time{})
		trade = aggragates.AppendStrategyRow(trade, row, event)
		gotPoints, gotDepths, isOpened := dynamicparams.OpenedRaise(trade)
		if !isOpened || gotPoints != points || gotDepths != depths {
			t.Fatalf("%s: the pair reads back as %v, %d, %v, want %v, %d", c.name, gotPoints, gotDepths, isOpened, points, depths)
		}
	}
}

// The opened pair the engines append: the INFO row at the price and the stamp
// they name, for the trade, and the opened event beside it carrying the
// amounts, filed under the flag's own param and gate with the very same stamp,
// so (TradeID, CreatedAt) finds the pair. The row's text is OpenedMessage, byte
// for byte.
func TestOpenedRowsWriteThePairTheEnginesAppend(t *testing.T) {
	trade := openingTrade()
	trade.ID = 31
	at := time.Date(2022, time.May, 9, 14, 5, 0, 0, time.UTC)
	opened := dynamicparams.Opened{Points: 0.4, Depths: 1}

	row, event := opened.Rows(trade, 87.5, at)

	wantRow := aggragates.TradesLogs{
		TradeID:   31,
		Message:   "dynamic params: opened raised, percentage +0.4 and depths +1 on every row",
		Type:      aggragates.LOG_INFO,
		Price:     87.5,
		CreatedAt: at,
		UpdatedAt: at,
	}
	if !reflect.DeepEqual(row, wantRow) {
		t.Fatalf("row = %+v, want %+v", row, wantRow)
	}
	if event.TradeID != 31 || event.Param != aggragates.StrategyParamDynamicParams || event.Gate != dynamicparams.GateOpened {
		t.Fatalf("event is filed as trade %d, %q/%q", event.TradeID, event.Param, event.Gate)
	}
	if !event.CreatedAt.Equal(row.CreatedAt) || event.Kind() != dynamicparams.EventOpened {
		t.Fatalf("event kind %q stamped %s, want %q stamped %s like its row", event.Kind(), event.CreatedAt, dynamicparams.EventOpened, row.CreatedAt)
	}
	var data dynamicparams.OpenedEvent
	if err := event.DecodeData(&data); err != nil || data != (dynamicparams.OpenedEvent{Event: dynamicparams.EventOpened, Points: 0.4, Depths: 1}) {
		t.Fatalf("data = %+v (%v), want the opened event carrying the amounts", data, err)
	}

	// A part the increase does not name is left out of the document.
	if _, alone := (dynamicparams.Opened{Points: 0.4}).Rows(trade, 87.5, at); string(alone.Data) != `{"event":"opened","points":0.4}` {
		t.Fatalf("data = %s, want the percentage alone", alone.Data)
	}

	// The pair appends to a copy: the trade the caller holds keeps its slices.
	appended := aggragates.AppendStrategyRow(trade, row, event)
	if len(trade.Logs) != 0 || len(trade.StrategyEvents) != 0 || len(appended.Logs) != 1 || len(appended.StrategyEvents) != 1 {
		t.Fatalf("the pair must append to a copy: %d/%d rows, %d/%d events",
			len(trade.Logs), len(appended.Logs), len(trade.StrategyEvents), len(appended.StrategyEvents))
	}
}

// Reads that raise nothing open the ladder on its configured rows, and no
// pair is written: the base tier, a block sophos did not read over stale
// bearish reads, and the zero block.
func TestOpeningAnswersNothingWhenTheReadsRaiseNothing(t *testing.T) {
	for name, reads := range map[string]aggragates.DynamicParamsIndicators{
		"not bearish":   notBearish,
		"not read":      notRead,
		"the zero read": {},
	} {
		if opened, ok := dynamicparams.Opening(openingTrade(), reads); ok || opened != (dynamicparams.Opened{}) {
			t.Errorf("%s: Opening = %+v, %v, want nothing", name, opened, ok)
		}
	}
}

// A trade the flag does not shape never gets the pair, whatever the reads.
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
		if opened, ok := dynamicparams.Opening(trade, bothBearish); ok || opened != (dynamicparams.Opened{}) {
			t.Errorf("%s: Opening = %+v, %v, want nothing", c.name, opened, ok)
		}
	}
}

// The reads are consulted once per ladder. A first entry held or refused
// funds, and judged again on a later tick, already carries its opened event
// and writes no second pair, whatever the reads say now — even an event whose
// amounts it cannot read, which is still its opened event. A ladder with an
// entry fill has opened: with or without an event it is never judged again.
func TestOpeningAnswersOnlyUntilTheLadderOpens(t *testing.T) {
	filled := func(trade aggragates.Trades) aggragates.Trades {
		trade.History = []aggragates.TradesHistory{{Type: "BUY", Quantity: 1, Price: 100, OrderId: 1}}
		trade.PositionType = "buy"
		trade.PositionPrice = 100
		return trade
	}
	opened := func() aggragates.Trades {
		return withOpened(openingTrade(), dynamicparams.BearPercentagePoints, 0)
	}

	for name, trade := range map[string]aggragates.Trades{
		"a held first entry carrying its pair":                    opened(),
		"a held first entry whose event names no amount it reads": withOpened(openingTrade(), 0, 0),
		"a ladder filled on its configured rows":                  filled(openingTrade()),
		"a ladder filled on the rows it opened with":              filled(opened()),
	} {
		for readsName, reads := range map[string]aggragates.DynamicParamsIndicators{
			"both bearish": bothBearish,
			"mixed":        mixedRead,
		} {
			if got, ok := dynamicparams.Opening(trade, reads); ok || got != (dynamicparams.Opened{}) {
				t.Errorf("%s, %s: Opening = %+v, %v, want nothing", name, readsName, got, ok)
			}
		}
	}
}

// The rows are the operator's text, never the ladder's state: a ladder whose
// log carries every row that ever named an opening — the opened row's text,
// plain or under a hold frame, the per-tick rows an earlier release wrote,
// each alone and all of them together — but no opened event has not opened, so
// it opens on the amounts the reads of its opening raise.
func TestOpeningIsNotSilencedByTheRowText(t *testing.T) {
	want := dynamicparams.Opened{Points: dynamicparams.BearPercentagePoints, Depths: dynamicparams.BearDepths}
	carrying := map[string][]string{
		"the opened row's text":                      {dynamicparams.OpenedMessage(dynamicparams.BearPercentagePoints, 0)},
		"the opened row's text under a hold frame":   {"Hold entry: " + dynamicparams.OpenedMessage(dynamicparams.BearPercentagePoints, 0)},
		"all of the per-tick release's rows":         earlierReleaseRows,
		"an opened row naming no amount it can read": {"dynamic params: opened raised, on every row"},
	}
	for _, row := range earlierReleaseRows {
		carrying[row] = []string{row}
	}
	for name, rows := range carrying {
		if got, ok := dynamicparams.Opening(withRows(openingTrade(), rows...), bothBearish); !ok || got != want {
			t.Errorf("%s: Opening = %+v, %v, want the ladder to open with %+v", name, got, ok, want)
		}
	}
}
