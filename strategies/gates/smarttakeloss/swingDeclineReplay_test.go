package smarttakeloss

import (
	"encoding/json"
	"fmt"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/swingdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
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
	return fmt.Sprintf("swingdecline.MinDepthsBetween %d, LookbackBars %d, PivotBars %d, MinBarsBetween %d, MinDropPct %v, MaxBouncePct %v, MinLegsDown %d, MinLegsUp %d, MinSmcTrendDown %d",
		swingdecline.MinDepthsBetween, swingdecline.LookbackBars, swingdecline.PivotBars, swingdecline.MinBarsBetween, swingdecline.MinDropPct,
		swingdecline.MaxBouncePct, swingdecline.MinLegsDown, swingdecline.MinLegsUp, swingdecline.MinSmcTrendDown)
}

// The ladders that locked the wallet, replayed on their real tapes: on the bar
// the newest fill sits in, once it has closed, the swing decline writes ONE row
// at that fill's price — the ladder pending on the swing decline and the latch of
// the early take profit on the same row; a second tick, or a later bar, no
// second row. The bar before it, which has not closed that fill's bar, and the
// ladder three fills deep are read as of their own last closed bar, every tick
// reading to it (swingdecline.TriggerAtLastBar), and write the row exactly when it
// holds. The tapes are replayed under a trend that holds: what the dashboard
// really read at those bars is the swing decline package's replays'. The reasons
// are the ones swingdecline.Trigger names, so the rows follow the constants it is
// tuned to; a ladder whose window does not hold under them is skipped.
//
// The row is read at a print over the ladder's break even (gainPrint) with a buy order resting, where the
// ladder in buy is activated nothing and the tick writes the gate's row alone. At the
// print of the fill itself, under the break even, the same tick activates the stopLoss
// beside the row (engineStep leaves the ladder in stopLoss, as the engines' chain does),
// and the ticks of the later bars sell the ladder with no second row.
func TestSwingDeclineReplaysTheRealLadders(t *testing.T) {
	withLinearDecline(t, false)
	for name, tc := range replayLadders() {
		t.Run(name, func(t *testing.T) {
			trade := testutil.LadderTrade(false, tc.fills...)
			barMs, gain := series.BarMs, gainPrint(trade)
			read := patternTape(t, tc.tape, tc.readBar.UnixMilli())
			if last := read.SeriesOpens[len(read.SeriesOpens)-1]; last != tc.readBar.UnixMilli() {
				t.Fatalf("fixture drifted, the tape must end at the read bar, ends at %d", last)
			}
			reading, holds := swingdecline.Trigger(swingFills(rebuildState(trade).fills), series.SeriesOf(read), series.TrendOf(read))
			if !holds {
				t.Skipf("the window of this ladder does not hold under the constants in force (%s): %q", constantsInForce(), reading.Breaks)
			}
			before := patternTape(t, tc.tape, tc.readBar.UnixMilli()-barMs)
			_, heldBefore := readsAsOf(trade, before)
			if got := Apply(withRestingOrder(trade), "", gain, withBlock(before)); (len(got.Rows) != 0) != heldBefore {
				t.Errorf("the bar before the fill's: a row %v, want %v as the swing decline reads it as of that bar", len(got.Rows) != 0, heldBefore)
			}

			pendingRow := PendingRow(tc.lastFill, indecisionFinding(nil), swingFinding(reading.Reasons))
			ticked, got := tickResting(trade, "", gain, tc.readBar.Add(time.Hour), withBlock(read))
			if got.Position != "" || got.Reason != "" || got.Activated {
				t.Fatalf("the read bar writes the row and forces nothing, got %+v", got)
			}
			assertRow(t, got, pendingRow)
			if st := rebuildState(ticked); !st.slowDeclinePending || st.slowDeclinePendingFrom != tc.lastFill || !st.earlyTakeProfit {
				t.Errorf("the row leaves the ladder pending from its fill and latched, got %+v", st)
			}
			for _, bars := range []int64{0, 1, 2} {
				later := patternTape(t, tc.tape, tc.readBar.UnixMilli()+bars*barMs)
				assertUntouched(t, Apply(withRestingOrder(ticked), "", gain, withBlock(later)), "")
			}

			stepped, activated := engineStep(trade, "", tc.lastFill, tc.readBar.Add(time.Hour), withBlock(read))
			assertActivated(t, activated, pendingRow, StopLossActivatedRow(tc.lastFill, -handLossPct(trade, tc.lastFill), FeatureSwingDecline))
			for _, bars := range []int64{0, 1, 2} {
				later := patternTape(t, tc.tape, tc.readBar.UnixMilli()+bars*barMs)
				assertSold(t, Apply(stepped, "", tc.lastFill, withBlock(later)), reasonSlowDecline)
			}

			shallow := testutil.LadderTrade(false, tc.fills[:3]...)
			_, heldShallow := readsAsOf(shallow, read)
			if got := applyAtAGain(shallow, "", withBlock(read)); (len(got.Rows) != 0) != heldShallow {
				t.Errorf("the ladder three fills deep: a row %v, want %v as the swing decline reads it as of the last bar", len(got.Rows) != 0, heldShallow)
			}
		})
	}
}

// Trade 114820 on its real tape: five fills, the fifth in the 21:00 bar of the
// tenth day. At the first tick after that bar closes the swing decline reads a
// staircase from the first fill, the SMC trend reads down on enough rows, and the
// gate writes ONE row — "early take profit latched, swing decline triggered", the
// window naming the swing decline alone, the early take profit latching beside
// it with no reasons of its own — and ONE event: pending, on the features
// indecision and swing decline, at the fifth fill's price. The tick before the
// bar closes reads the bar before it and writes a row only when that holds. The
// ticks run at a print over the ladder's break even (gainPrint) with a buy order resting, where the ladder in
// buy is activated nothing and the row stands alone; the shared action's own answer
// to that very tick, on the real ladder, is asked after them.
func TestSwingDeclineReplaysTrade114820AsOneRowAndOneEvent(t *testing.T) {
	withLinearDecline(t, false)
	tc := replayLadders()["SOL, trade 114820"]
	trade := testutil.LadderTrade(false, tc.fills...)
	read := patternTape(t, tc.tape, tc.readBar.UnixMilli())
	reading, holds := swingdecline.Trigger(swingFills(rebuildState(trade).fills), series.SeriesOf(read), series.TrendOf(read))
	if !holds {
		t.Skipf("the window of trade 114820 does not hold under the constants in force (%s): %q", constantsInForce(), reading.Breaks)
	}
	if reading.FromDepth != 1 || reading.ToDepth != len(tc.fills) {
		t.Fatalf("fixture drifted: the window runs from the first fill to the fifth, got %+v", reading)
	}

	before := patternTape(t, tc.tape, tc.readBar.UnixMilli()-series.BarMs)
	_, heldBefore := readsAsOf(trade, before)
	if got := applyAtAGain(trade, "", withBlock(before)); (len(got.Rows) != 0) != heldBefore {
		t.Errorf("the tick before the bar closes: a row %v, want %v as the swing decline reads it as of the last closed bar", len(got.Rows) != 0, heldBefore)
	}

	ticked, got := tickAtAGain(trade, "", tc.readBar.Add(time.Hour+30*time.Second), withBlock(read))
	assertRow(t, got, PendingRow(tc.lastFill, indecisionFinding(nil), swingFinding(reading.Reasons)))
	assertActedOn(t, trade, withBlock(read), got.Rows...)
	row := got.Rows[0]
	if want := "smartTakeLoss: early take profit latched, swing decline triggered (swing decline: " + strings.Join(reading.Reasons, ", ") + ")"; row.Message != want {
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
	wantData := EventData{Event: EventPending, Price: tc.lastFill, Reasons: reading.Reasons, Features: []string{FeatureEarlyTakeProfit, FeatureSwingDecline}}
	if !reflect.DeepEqual(data, wantData) || ticked.StrategyEvents[0].Gate != GateSlowDecline {
		t.Fatalf("the event is the pending kind of the gate on indecision and swingDecline, got %+v under %q, want %+v", data, ticked.StrategyEvents[0].Gate, wantData)
	}
	if st := rebuildState(ticked); !st.slowDeclinePending || st.slowDeclinePendingFrom != tc.lastFill || !st.earlyTakeProfit || !slices.Equal(st.pendingFeatures, []string{FeatureSwingDecline}) {
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
	raw, err := os.ReadFile(filepath.Join("swingdecline", "testdata", name))
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
		if len(bar) != len(file.Fields) || json.Unmarshal(bar[0], &open) != nil {
			t.Fatalf("bar %d: %s is not an open time and four values", index, bar)
		}
		if index > 0 && open != block.SeriesOpens[index-1]+series.BarMs {
			t.Fatalf("bar %d opens at %d, want one bar after the bar before it", index, open)
		}
		if lastOpen != 0 && open > lastOpen {
			break
		}
		wicks := make([]float64, 3)
		for column := range wicks {
			var text string
			if json.Unmarshal(bar[column+1], &text) != nil {
				t.Fatalf("bar %d: %s is not a decimal string", index, bar[column+1])
			}
			price, err := strconv.ParseFloat(text, 64)
			if err != nil {
				t.Fatalf("bar %d: %v", index, err)
			}
			wicks[column] = price
		}
		block.SeriesOpens = append(block.SeriesOpens, open)
		block.SeriesHighs = append(block.SeriesHighs, wicks[0])
		block.SeriesLows = append(block.SeriesLows, wicks[1])
		block.SeriesCloses = append(block.SeriesCloses, wicks[2])
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
	withBothDeclines(t)
	tape := stairFourthBlock()
	retired := map[string]any{
		"slowPatternOpens": tape.SeriesOpens, "slowPatternCloses": tape.SeriesCloses,
		"slowPatternSmcTrendDown": patternTrendDown, "slowPatternSmcTrendValid": patternTrendValid,
	}
	current := map[string]any{
		"seriesOpens": tape.SeriesOpens, "seriesCloses": tape.SeriesCloses, "seriesHighs": tape.SeriesHighs, "seriesLows": tape.SeriesLows,
		"smcTrendDown": patternTrendDown, "smcTrendValid": patternTrendValid,
	}
	served := func(verdict bool, families ...map[string]any) aggragates.AIIndicators {
		t.Helper()
		wire := map[string]any{"quietDeclineFillFrom": fillWindowFrom.UnixMilli()}
		if verdict {
			wire["quietDeclineVerdict"], wire["quietDeclineExitReasons"] = true, slowDeclineReasons
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
	if only.SeriesOpens != nil || only.SeriesCloses != nil || only.SmcTrendDown != 0 || only.SmcTrendValid != 0 || series.SeriesOf(only).Served() {
		t.Fatalf("the retired keys alone must decode to no series and no trend, got %+v", only)
	}
	gain := gainPrint(stairFourth())
	assertUntouched(t, Apply(stairFourth(), "", slowDeclineBand-1, served(false, retired)), "")
	assertRow(t, Apply(withRestingOrder(stairFourth()), "", gain, served(true, retired)), PendingRow(stairPrices[3], quietFinding(slowDeclineReasons)))

	if got, want := served(false, retired, current).SmartTakeLoss, served(false, current).SmartTakeLoss; !reflect.DeepEqual(got, want) || !series.SeriesOf(got).Served() {
		t.Fatalf("with both families of keys only the current ones are read:\n got %+v\nwant %+v", got, want)
	}
	reasons := stairReading(t, stairFourth(), tape, true).Reasons
	assertRow(t, Apply(withRestingOrder(stairFourth()), "", gain, served(false, retired, current)), PendingRow(stairPrices[3], indecisionFinding(nil), swingFinding(reasons)))
}

// The wire, end to end: the block as sophos serves it decodes through mercury's own
// SophosPrediction and the gate reads it. The staircase's series with the two Trend
// Direction keys at the rows down the need asks goes pending at its fourth fill and
// the shared action acts on it there: the stopLoss of the ladder in buy activated, the
// ladder in stopLoss sold, with the pending row beside; the same series with a row
// short of it, with the keys served zero — sophos could not read its dashboard — or
// from a sophos without them, triggers nothing and nothing is done in either state,
// whatever the ladder proposes.
func TestSwingDeclineReadsTheTrendOffTheWire(t *testing.T) {
	needsTheTrend(t)
	withBothDeclines(t)
	tape := stairFourthBlock()
	for name, tc := range map[string]struct {
		keys  map[string]any
		sells bool
	}{
		"the rows down at the need": {map[string]any{"smcTrendDown": patternTrendDown, "smcTrendValid": patternTrendValid}, true},
		"a row short of the need":   {map[string]any{"smcTrendDown": shortTrendDown, "smcTrendValid": patternTrendValid}, false},
		"the keys served zero":      {map[string]any{"smcTrendDown": 0, "smcTrendValid": 0}, false},
		"a sophos without the keys": {nil, false},
	} {
		wire := map[string]any{"quietDeclineFillFrom": fillWindowFrom.UnixMilli(), "seriesOpens": tape.SeriesOpens, "seriesCloses": tape.SeriesCloses, "seriesHighs": tape.SeriesHighs, "seriesLows": tape.SeriesLows}
		maps.Copy(wire, tc.keys)
		raw, err := json.Marshal(map[string]any{"smartTakeLoss": wire})
		var prediction aggragates.SophosPrediction
		if err != nil || json.Unmarshal(raw, &prediction) != nil {
			t.Fatalf("%s: the wire did not round trip: %v", name, err)
		}
		t.Run(name, func(t *testing.T) {
			if tc.sells {
				assertActedOn(t, stairFourth(), prediction.Indicators(), stairPendingRow(t))
				return
			}
			assertSoldNever(t, stairFourth(), prediction.Indicators())
		})
	}
}
