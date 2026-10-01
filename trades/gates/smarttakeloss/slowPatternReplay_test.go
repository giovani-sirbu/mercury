package smarttakeloss

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/slowpattern"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// slowPatternReasons is what the pending row of the SOL ladder below names:
// the window and each reading against its constant.
var slowPatternReasons = []string{
	"depth 1 to 4, 1h bars 2021-10-08 14:00 to 2021-10-10 00:00 UTC",
	"weight 0.50 over 0.33",
	"4 legs down of 3 needed",
	"3 lower highs of 2 needed",
	"largest leg 51% of the fall under 60%",
	"down 7.7% past 4.0%",
}

// replayLadder is a real ladder of the pattern package's tapes: the stamps and
// prices its history carries, the first fills deep.
type replayLadder struct {
	tape     string
	fills    []testutil.LadderFill
	readBar  time.Time
	lastFill float64
}

func replayLadders() map[string]replayLadder {
	utc := func(month time.Month, day, hour, minute, second int) time.Time {
		return time.Date(2021, month, day, hour, minute, second, 0, time.UTC)
	}
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
	}
}

// The two ladders that locked the wallet, replayed on their real tapes: on the
// bar the newest fill sits in, once it has closed, the slow pattern decline
// writes ONE pending row at that fill's price and the latch beside it; the
// bar before it nothing; a second tick, or a later bar, no second row; and the
// ladder three fills deep, nothing at all.
func TestSlowPatternReplaysTheRealLadders(t *testing.T) {
	for name, tc := range replayLadders() {
		trade := testutil.LadderTrade(false, tc.fills...)
		barMs := slowpattern.SlowPatternBarMs
		read := patternTape(t, tc.tape, tc.readBar.UnixMilli())
		if last := read.SlowPatternOpens[len(read.SlowPatternOpens)-1]; last != tc.readBar.UnixMilli() {
			t.Fatalf("%s: fixture drifted, the tape must end at the read bar, ends at %d", name, last)
		}
		assertUntouched(t, Apply(trade, "", tc.lastFill, withBlock(patternTape(t, tc.tape, tc.readBar.UnixMilli()-barMs))), "")

		ticked, got := engineTick(trade, "", tc.lastFill, tc.readBar.Add(time.Hour), withBlock(read))
		if got.SlowDecline == nil || got.Indecision == nil || got.Position != "" || got.Reason != "" {
			t.Fatalf("%s: the read bar writes the pending row and the latch and forces nothing, got %+v", name, got)
		}
		row, latch := *got.SlowDecline, *got.Indecision
		if row.Gate != GateSlowPattern || row.Event != EventPending || row.Price != tc.lastFill || row.Message != SlowPatternMessage("buy", row.Reasons) {
			t.Fatalf("%s: the pending row is the slow pattern's at the newest fill, got %+v", name, row)
		}
		if len(row.Reasons) != 6 || row.Reasons[0][:len("depth 1 to 4, 1h bars ")] != "depth 1 to 4, 1h bars " {
			t.Errorf("%s: the row names the window and the five readings, got %q", name, row.Reasons)
		}
		if want := LatchedRow("buy", tc.lastFill, append([]string{"slow pattern decline"}, row.Reasons...)); latch.Message != want.Message || latch.Gate != GateIndecision || latch.Price != tc.lastFill {
			t.Errorf("%s: the latch names the pattern and its reasons, got %+v", name, latch)
		}
		if st := rebuildState(ticked); !st.slowPatternPending || st.slowPatternPendingFrom != tc.lastFill || !st.indecision {
			t.Errorf("%s: the rows leave the ladder pending from its fill and latched, got %+v", name, st)
		}
		for _, bars := range []int64{0, 1, 2} {
			later := patternTape(t, tc.tape, tc.readBar.UnixMilli()+bars*barMs)
			assertUntouched(t, Apply(ticked, "", tc.lastFill, withBlock(later)), "")
		}
		assertUntouched(t, Apply(testutil.LadderTrade(false, tc.fills[:3]...), "", tc.lastFill, withBlock(read)), "")
	}
}

// patternTape is the series of a tape in the slow pattern package's testdata —
// the two real ones, SOL in October 2021 and HBAR in December 2021 — as the
// block serves it: the open time and close of every bar, oldest first,
// optionally cut to end at the bar opening at lastOpen (zero keeps them all).
// The tapes are the pattern's own fixtures, shared with its tests; a bar that
// is not one hour after the one before it fails the test.
func patternTape(t *testing.T, name string, lastOpen int64) aggragates.SmartTakeLossIndicators {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "slowpattern", "testdata", name))
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
		if index > 0 && open != block.SlowPatternOpens[index-1]+slowpattern.SlowPatternBarMs {
			t.Fatalf("bar %d opens at %d, want one bar after the bar before it", index, open)
		}
		if lastOpen != 0 && open > lastOpen {
			break
		}
		closed, err := strconv.ParseFloat(text, 64)
		if err != nil {
			t.Fatalf("bar %d: %v", index, err)
		}
		block.SlowPatternOpens = append(block.SlowPatternOpens, open)
		block.SlowPatternCloses = append(block.SlowPatternCloses, closed)
	}
	return block
}
