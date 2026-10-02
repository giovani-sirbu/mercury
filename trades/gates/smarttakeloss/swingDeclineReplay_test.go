package smarttakeloss

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/swingdecline"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// replayLadder is a real ladder of the swing decline package's tapes: the stamps
// and prices its history carries, the first fills deep.
type replayLadder struct {
	tape     string
	fills    []testutil.LadderFill
	readBar  time.Time
	lastFill float64
}

func utc(month time.Month, day, hour, minute, second int) time.Time {
	return time.Date(2021, month, day, hour, minute, second, 0, time.UTC)
}

func replayLadders() map[string]replayLadder {
	return map[string]replayLadder{
		"SOL, trade 113054": {
			tape: "solusdt-1h-2021-10.json",
			fills: []testutil.LadderFill{
				{Price: 166.75, At: utc(time.October, 8, 14, 20, 12)},
				{Price: 160.35, At: utc(time.October, 8, 20, 57, 14)},
				{Price: 157.21, At: utc(time.October, 9, 1, 27, 40)},
				{Price: 154.13, At: utc(time.October, 10, 0, 55, 47)},
			},
			readBar:  utc(time.October, 10, 0, 0, 0),
			lastFill: 154.13,
		},
		"HBAR, trade 111284": {
			tape: "hbarusdt-1h-2021-12.json",
			fills: []testutil.LadderFill{
				{Price: 0.3567, At: utc(time.December, 1, 4, 15, 50)},
				{Price: 0.3497, At: utc(time.December, 1, 19, 23, 19)},
				{Price: 0.3428, At: utc(time.December, 2, 0, 44, 44)},
				{Price: 0.3345, At: utc(time.December, 3, 1, 0, 59)},
			},
			readBar:  utc(time.December, 3, 1, 0, 0),
			lastFill: 0.3345,
		},
		"SOL, trade 114820": {
			tape: "solusdt-1h-2021-10.json",
			fills: []testutil.LadderFill{
				{Price: 166.03, At: utc(time.October, 8, 11, 14, 0)},
				{Price: 159.65, At: utc(time.October, 8, 20, 56, 0)},
				{Price: 156.54, At: utc(time.October, 9, 1, 30, 0)},
				{Price: 153.48, At: utc(time.October, 10, 1, 12, 0)},
				{Price: 150.48, At: utc(time.October, 10, 21, 16, 0)},
			},
			readBar:  utc(time.October, 10, 21, 0, 0),
			lastFill: 150.48,
		},
	}
}

// constantsInForce names the swing decline's constants as the package reads
// them, for a test that cannot run under another set.
func constantsInForce() string {
	return fmt.Sprintf("SwingDeclineMinDepthsBetween %d, MinWeight %v, MinDownLegs %d, MinLowerHighs %d, MaxLegShare %v, MinFallPct %v, MinSmcTrendDown %d, PivotBars %d",
		swingdecline.SwingDeclineMinDepthsBetween, swingdecline.SwingDeclineMinWeight, swingdecline.SwingDeclineMinDownLegs, swingdecline.SwingDeclineMinLowerHighs,
		swingdecline.SwingDeclineMaxLegShare, swingdecline.SwingDeclineMinFallPct, swingdecline.SwingDeclineMinSmcTrendDown, swingdecline.SwingDeclinePivotBars)
}

// The ladders that locked the wallet, replayed on their real tapes: on the bar
// the newest fill sits in, once it has closed, the swing decline writes ONE row
// at that fill's price — the ladder pending on the swing decline and the latch of
// the indecision direction on the same row; the bar before it nothing; a second
// tick, or a later bar, no second row; and the ladder three fills deep, nothing at
// all. The tapes are replayed under a trend that holds: what the dashboard really
// read at those bars is the swing decline package's replays'. The reasons are the
// ones swingdecline.Trigger names, so the rows follow the constants it is tuned
// to; a ladder whose window does not hold under them is skipped.
func TestSwingDeclineReplaysTheRealLadders(t *testing.T) {
	for name, tc := range replayLadders() {
		t.Run(name, func(t *testing.T) {
			trade := testutil.LadderTrade(false, tc.fills...)
			barMs := swingdecline.SwingDeclineBarMs
			read := patternTape(t, tc.tape, tc.readBar.UnixMilli())
			if last := read.SwingDeclineOpens[len(read.SwingDeclineOpens)-1]; last != tc.readBar.UnixMilli() {
				t.Fatalf("fixture drifted, the tape must end at the read bar, ends at %d", last)
			}
			reading, holds := swingdecline.Trigger(swingFills(rebuildState(trade).fills), swingdecline.SeriesOf(read), swingdecline.TrendOf(read))
			if !holds {
				t.Skipf("the window of this ladder does not hold under the constants in force (%s): %q", constantsInForce(), reading.Breaks)
			}
			assertUntouched(t, Apply(trade, "", tc.lastFill, withBlock(patternTape(t, tc.tape, tc.readBar.UnixMilli()-barMs))), "")

			ticked, got := engineTick(trade, "", tc.lastFill, tc.readBar.Add(time.Hour), withBlock(read))
			if got.Position != "" || got.Reason != "" {
				t.Fatalf("the read bar writes the row and forces nothing, got %+v", got)
			}
			assertRow(t, got, PendingRow(tc.lastFill, indecisionFinding(nil), swingFinding(reading.Reasons)))
			if st := rebuildState(ticked); !st.slowDeclinePending || st.slowDeclinePendingFrom != tc.lastFill || !st.indecision {
				t.Errorf("the row leaves the ladder pending from its fill and latched, got %+v", st)
			}
			for _, bars := range []int64{0, 1, 2} {
				later := patternTape(t, tc.tape, tc.readBar.UnixMilli()+bars*barMs)
				assertUntouched(t, Apply(ticked, "", tc.lastFill, withBlock(later)), "")
			}
			assertUntouched(t, Apply(testutil.LadderTrade(false, tc.fills[:3]...), "", tc.lastFill, withBlock(read)), "")
		})
	}
}

// Trade 114820 on its real tape: five fills, the fifth in the 21:00 bar of the
// tenth day. At the first tick after that bar closes the swing decline reads a
// staircase from the first fill, the SMC trend reads down on enough rows, and the
// gate writes ONE row — "indecision direction and swing decline triggered", the
// window naming the swing decline alone, the indecision direction latching beside
// it with no reasons of its own — and ONE event: pending, on the features
// indecision and swing decline, at the fifth fill's price. The tick before the
// bar closes writes nothing.
func TestSwingDeclineReplaysTrade114820AsOneRowAndOneEvent(t *testing.T) {
	tc := replayLadders()["SOL, trade 114820"]
	trade := testutil.LadderTrade(false, tc.fills...)
	read := patternTape(t, tc.tape, tc.readBar.UnixMilli())
	reading, holds := swingdecline.Trigger(swingFills(rebuildState(trade).fills), swingdecline.SeriesOf(read), swingdecline.TrendOf(read))
	if !holds {
		t.Skipf("the window of trade 114820 does not hold under the constants in force (%s): %q", constantsInForce(), reading.Breaks)
	}
	if reading.FromDepth != 1 || reading.ToDepth != len(tc.fills) {
		t.Fatalf("fixture drifted: the window runs from the first fill to the fifth, got %+v", reading)
	}

	before := patternTape(t, tc.tape, tc.readBar.UnixMilli()-swingdecline.SwingDeclineBarMs)
	assertUntouched(t, Apply(trade, "", tc.lastFill, withBlock(before)), "")

	ticked, got := engineTick(trade, "", tc.lastFill, tc.readBar.Add(time.Hour+30*time.Second), withBlock(read))
	assertRow(t, got, PendingRow(tc.lastFill, indecisionFinding(nil), swingFinding(reading.Reasons)))
	row := got.Rows[0]
	if want := "smartTakeLoss: indecision direction and swing decline triggered (swing decline: " + strings.Join(reading.Reasons, ", ") + ")"; row.Message != want {
		t.Fatalf("the row is the one the grammar writes for the two features,\n got  %q\n want %q", row.Message, want)
	}
	if strings.Contains(row.Message, "Hold ") {
		t.Fatalf("no row of the gate carries the hold frame, got %q", row.Message)
	}
	t.Logf("the row under %s: %s", constantsInForce(), row.Message)
	if len(ticked.Logs) != 1 || len(ticked.StrategyEvents) != 1 {
		t.Fatalf("one row and one event over the tick, got %d and %d", len(ticked.Logs), len(ticked.StrategyEvents))
	}
	var data EventData
	if err := ticked.StrategyEvents[0].DecodeData(&data); err != nil {
		t.Fatal(err)
	}
	wantData := EventData{Event: EventPending, Price: tc.lastFill, Reasons: reading.Reasons, Features: []string{FeatureIndecision, FeatureSwingDecline}}
	if !reflect.DeepEqual(data, wantData) || ticked.StrategyEvents[0].Gate != GateSlowDecline {
		t.Fatalf("the event is the pending kind of the gate on indecision and swingDecline, got %+v under %q, want %+v", data, ticked.StrategyEvents[0].Gate, wantData)
	}
	if st := rebuildState(ticked); !st.slowDeclinePending || st.slowDeclinePendingFrom != tc.lastFill || !st.indecision || !slices.Equal(st.pendingFeatures, []string{FeatureSwingDecline}) {
		t.Errorf("the ladder reads pending on the swing decline alone, and latched, got %+v", st)
	}
}

// patternTape is the series of a tape in the swing decline package's testdata —
// the two real ones, SOL in October 2021 and HBAR in December 2021 — as the
// block serves it: the open time and close of every bar, oldest first,
// optionally cut to end at the bar opening at lastOpen (zero keeps them all),
// with a Trend Direction row that holds beside it (withSmcTrend), so the shape
// of the tape is what the replays decide on. The tapes are the swing decline's
// own fixtures, shared with its tests; a bar that is not one hour after the one
// before it fails the test.
func patternTape(t *testing.T, name string, lastOpen int64) aggragates.SmartTakeLossIndicators {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "swingdecline", "testdata", name))
	if err != nil {
		t.Fatalf("read the tape: %v", err)
	}
	var file struct {
		Interval string              `json:"interval"`
		Fields   []string            `json:"fields"`
		Bars     [][]json.RawMessage `json:"bars"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("decode the tape: %v", err)
	}
	if file.Interval != "1h" || !slices.Equal(file.Fields, []string{"openTime", "high", "low", "close", "volume"}) {
		t.Fatalf("the tape holds %s bars of %q, want 1h bars of open time, high, low, close and volume", file.Interval, file.Fields)
	}
	var block aggragates.SmartTakeLossIndicators
	for index, bar := range file.Bars {
		var open int64
		var text string
		if len(bar) != len(file.Fields) || json.Unmarshal(bar[0], &open) != nil || json.Unmarshal(bar[3], &text) != nil {
			t.Fatalf("bar %d: %s is not an open time and four values", index, bar)
		}
		if index > 0 && open != block.SwingDeclineOpens[index-1]+swingdecline.SwingDeclineBarMs {
			t.Fatalf("bar %d opens at %d, want one bar after the bar before it", index, open)
		}
		if lastOpen != 0 && open > lastOpen {
			break
		}
		closed, err := strconv.ParseFloat(text, 64)
		if err != nil {
			t.Fatalf("bar %d: %v", index, err)
		}
		block.SwingDeclineOpens = append(block.SwingDeclineOpens, open)
		block.SwingDeclineCloses = append(block.SwingDeclineCloses, closed)
	}
	return withSmcTrend(block, patternTrendDown, patternTrendValid)
}

// The keys the swing decline's input was served under before the rename — the four
// slowPattern ones — are retired, and mercury reads none of them: a payload from a
// sophos not redeployed yet, which serves the staircase's series and its Trend
// Direction row under them alone, decodes to no series and no trend, so the swing
// decline is not served and the gate fails closed on it — it triggers nothing and
// latches nothing, the quiet decline alone deciding, whatever else the payload
// carries. With both families of keys on the wire only the current ones are read:
// the block is the one the current keys give alone.
func TestSwingDeclineReadsNoneOfTheRetiredSlowPatternKeysOffTheWire(t *testing.T) {
	series := stairFourthBlock()
	retired := map[string]any{
		"slowPatternOpens": series.SwingDeclineOpens, "slowPatternCloses": series.SwingDeclineCloses,
		"slowPatternSmcTrendDown": patternTrendDown, "slowPatternSmcTrendValid": patternTrendValid,
	}
	current := map[string]any{
		"swingDeclineOpens": series.SwingDeclineOpens, "swingDeclineCloses": series.SwingDeclineCloses,
		"swingDeclineSmcTrendDown": patternTrendDown, "swingDeclineSmcTrendValid": patternTrendValid,
	}
	served := func(verdict bool, families ...map[string]any) aggragates.AIIndicators {
		t.Helper()
		wire := map[string]any{"slowDeclineSellBand": slowDeclineBand}
		if verdict {
			wire["slowDeclineExit"], wire["slowDeclineExitReasons"], wire["slowDeclineFillFrom"] = true, slowDeclineReasons, fillWindowFrom.UnixMilli()
		}
		for _, keys := range families {
			maps.Copy(wire, keys)
		}
		raw, err := json.Marshal(map[string]any{"smartTakeLoss": wire})
		var prediction aggragates.SophosPrediction
		if err != nil || json.Unmarshal(raw, &prediction) != nil {
			t.Fatalf("the wire did not round trip: %v", err)
		}
		return prediction.Indicators()
	}

	only := served(false, retired).SmartTakeLoss
	if only.SwingDeclineOpens != nil || only.SwingDeclineCloses != nil || only.SwingDeclineSmcTrendDown != 0 || only.SwingDeclineSmcTrendValid != 0 || swingdecline.SeriesOf(only).Served() {
		t.Fatalf("the retired keys alone must decode to no series and no trend, got %+v", only)
	}
	assertUntouched(t, Apply(stairFourth(), "", slowDeclineBand-1, served(false, retired)), "")
	assertRow(t, Apply(stairFourth(), "", slowDeclineBand-1, served(true, retired)), PendingRow(stairPrices[3], quietFinding(slowDeclineReasons)))

	if got, want := served(false, retired, current).SmartTakeLoss, served(false, current).SmartTakeLoss; !reflect.DeepEqual(got, want) || !swingdecline.SeriesOf(got).Served() {
		t.Fatalf("with both families of keys only the current ones are read:\n got %+v\nwant %+v", got, want)
	}
	reasons := stairReading(t, stairFourth(), series, true).Reasons
	assertRow(t, Apply(stairFourth(), "", slowDeclineBand-1, served(false, retired, current)), PendingRow(stairPrices[3], indecisionFinding(nil), swingFinding(reasons)))
}

// The wire, end to end: the block as sophos serves it decodes through mercury's own
// SophosPrediction and the gate reads it. The staircase's series with the two Trend
// Direction keys at the rows down the need asks goes pending and sells at its fourth
// fill; the same series with a row short of it, with the keys served zero — sophos
// could not read its dashboard — or from a sophos without them, triggers nothing.
func TestSwingDeclineReadsTheTrendOffTheWire(t *testing.T) {
	needsTheTrend(t)
	series := stairFourthBlock()
	for name, tc := range map[string]struct {
		keys  map[string]any
		sells bool
	}{
		"the rows down at the need": {map[string]any{"swingDeclineSmcTrendDown": patternTrendDown, "swingDeclineSmcTrendValid": patternTrendValid}, true},
		"a row short of the need":   {map[string]any{"swingDeclineSmcTrendDown": shortTrendDown, "swingDeclineSmcTrendValid": patternTrendValid}, false},
		"the keys served zero":      {map[string]any{"swingDeclineSmcTrendDown": 0, "swingDeclineSmcTrendValid": 0}, false},
		"a sophos without the keys": {nil, false},
	} {
		wire := map[string]any{"slowDeclineSellBand": slowDeclineBand, "swingDeclineOpens": series.SwingDeclineOpens, "swingDeclineCloses": series.SwingDeclineCloses}
		maps.Copy(wire, tc.keys)
		raw, err := json.Marshal(map[string]any{"smartTakeLoss": wire})
		var prediction aggragates.SophosPrediction
		if err != nil || json.Unmarshal(raw, &prediction) != nil {
			t.Fatalf("%s: the wire did not round trip: %v", name, err)
		}
		got := Apply(stairFourth(), "", slowDeclineBand+1, prediction.Indicators())
		if sold := got.Reason == reasonSellBand; sold != tc.sells || (len(got.Rows) == 1) != tc.sells || len(got.Rows) > 1 {
			t.Errorf("%s: want the ladder pending and sold %v, got %+v", name, tc.sells, got)
		}
	}
}
