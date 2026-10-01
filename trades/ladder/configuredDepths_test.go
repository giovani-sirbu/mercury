package ladder

import (
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/dynamicparams"
)

// depthTrade is a long whose ladder has filled the given number of entries,
// one distinct exchange order each, on the given settings rows.
func depthTrade(filled int, settings ...aggragates.StrategySettings) aggragates.Trades {
	trade := aggragates.Trades{
		ID:           7412,
		Symbol:       "LINK/USDT",
		StrategyPair: aggragates.StrategiesPairs{StrategySettings: settings},
	}

	for index := 0; index < filled; index++ {
		trade.History = append(trade.History, aggragates.TradesHistory{
			Type:     "BUY",
			Quantity: 1,
			Price:    100 - float64(index),
			OrderId:  int64(index + 1),
		})
	}

	return trade
}

// The amounts the opened events below carry. They are this suite's own, apart
// from the shipped constants, so a retune of BearPercentagePoints or
// BearDepths moves no expectation here: the ladder trades whatever its own
// event says.
const (
	raisePoints = 0.5
	raiseDepths = 2
)

// withOpenedEvent is the trade as the engine leaves it once its ladder opened
// raised: the DynamicParams flag on for a long spot parent, and the opened pair
// appended with the writer the engine uses (dynamicparams.Opened.Rows) — the
// event every reader takes the amounts from, beside the row an operator reads.
// The rows the trade stores are untouched.
func withOpenedEvent(trade aggragates.Trades, points float64, depths int) aggragates.Trades {
	trade.Strategy.Params.DynamicParams = true
	trade.Strategy.TradeType = aggragates.Spot
	row, event := dynamicparams.Opened{Points: points, Depths: depths}.Rows(trade, trade.PositionPrice, time.Time{})

	return aggragates.AppendStrategyRow(trade, row, event)
}

// A single configured row governs every depth, so the ceiling it carries is
// the answer however far down the ladder already is.
func TestConfiguredDepthsReadsTheOnlyRow(t *testing.T) {
	settings := aggragates.StrategySettings{Percentage: 2.5, Depths: 8}

	for _, filled := range []int{0, 1, 5, 8, 20} {
		if got := ConfiguredDepths(depthTrade(filled, settings)); got != 8 {
			t.Errorf("%d entries filled: ConfiguredDepths = %d, want 8", filled, got)
		}
	}
}

// With a row per depth the answer is the row the NEXT fill would use — the
// row that governs the entry being decided, not the one that placed the last.
func TestConfiguredDepthsReadsTheRowOfTheNextFill(t *testing.T) {
	settings := []aggragates.StrategySettings{
		{Depths: 4},
		{Depths: 5},
		{Depths: 6},
		{Depths: 7},
	}

	if got := ConfiguredDepths(depthTrade(2, settings...)); got != 6 {
		t.Errorf("ConfiguredDepths = %d, want the row of the third entry", got)
	}
	// Past the configured rows SettingsIndexOrBase falls back to the base
	// row, never to the last one.
	if got := ConfiguredDepths(depthTrade(9, settings...)); got != 4 {
		t.Errorf("ConfiguredDepths = %d, want the base row", got)
	}
}

// Depths is configured as a float and a fraction of an entry cannot be
// placed, so the ceiling is floored — the same read the smart take loss arms
// on.
func TestConfiguredDepthsFloorsAFractionalRow(t *testing.T) {
	if got := ConfiguredDepths(depthTrade(1, aggragates.StrategySettings{Depths: 8.7})); got != 8 {
		t.Errorf("ConfiguredDepths = %d, want the fractional ceiling floored", got)
	}
}

// No settings row means no known ceiling. The callers read 0 as unknown, so
// answering anything else would invent a ladder the pair never configured.
func TestConfiguredDepthsIsZeroWithoutSettings(t *testing.T) {
	if got := ConfiguredDepths(depthTrade(3)); got != 0 {
		t.Errorf("ConfiguredDepths = %d, want 0 without a settings row", got)
	}
}

// A ladder that opened raised is allowed the depths its opened event added:
// the ceiling is the row of the next fill raised by the event's depths, at
// every depth the ladder can stand at — the stored ceiling included, where the
// same ladder without the opened event is full. A fraction is floored after
// the raise, and an opened event that raises only the percentage moves no
// ceiling.
func TestConfiguredDepthsOfARaisedLadderAddsTheOpenedEventsDepths(t *testing.T) {
	cases := []struct {
		name   string
		stored float64
		points float64
		depths int
		want   int
	}{
		{"depths", 8, 0, raiseDepths, 8 + raiseDepths},
		{"percentage and depths", 8, raisePoints, raiseDepths, 8 + raiseDepths},
		{"a fractional stored row", 8.7, 0, raiseDepths, 8 + raiseDepths},
		{"percentage only", 8, raisePoints, 0, 8},
	}

	for _, c := range cases {
		settings := aggragates.StrategySettings{Percentage: 2.5, Depths: c.stored}

		for _, filled := range []int{0, 1, 5, 8, 20} {
			plain := depthTrade(filled, settings)
			raised := withOpenedEvent(plain, c.points, c.depths)

			if got := ConfiguredDepths(raised); got != c.want {
				t.Errorf("%s, %d entries filled: ConfiguredDepths = %d, want %d", c.name, filled, got, c.want)
			}
			if got, want := ConfiguredDepths(plain), int(c.stored); got != want {
				t.Errorf("%s, %d entries filled: the same ladder without the opened event = %d, want the stored %d", c.name, filled, got, want)
			}
		}
	}
}

// With a row per depth the raise reaches every row, so the ceiling still moves
// with the ladder: the row of the next fill, raised — and past the last row
// the base row, raised.
func TestConfiguredDepthsOfARaisedLadderFlipsOnTheRaisedRows(t *testing.T) {
	rows := []float64{9, 5, 6, 7}

	for _, filled := range []int{0, 1, 2, 3, len(rows), len(rows) + 3} {
		plain := rowPerDepthTrade(filled, rows...)
		raised := withOpenedEvent(plain, 0, raiseDepths)

		want := int(rows[SettingsIndexOrBase(plain.StrategyPair.StrategySettings, filled)]) + raiseDepths
		if got := ConfiguredDepths(raised); got != want {
			t.Errorf("%d entries filled: ConfiguredDepths = %d, want the raised row of the next fill (%d)", filled, got, want)
		}
	}
}

// The opened event is the raise and the opened row beside it is only text: a
// ladder whose logs carry the row but whose strategy events carry no opened
// event trades its stored rows, however many entries it has filled.
func TestConfiguredDepthsReadsTheOpenedEventNeverTheRow(t *testing.T) {
	plain := depthTrade(8, aggragates.StrategySettings{Percentage: 2.5, Depths: 8})
	textOnly := withOpenedEvent(plain, raisePoints, raiseDepths)
	textOnly.StrategyEvents = nil

	if len(textOnly.Logs) != 1 {
		t.Fatalf("fixture drifted: the trade carries %d rows, want the opened row", len(textOnly.Logs))
	}
	if got := ConfiguredDepths(textOnly); got != 8 {
		t.Errorf("ConfiguredDepths = %d, want the stored 8: a row without its event opens nothing", got)
	}
}
