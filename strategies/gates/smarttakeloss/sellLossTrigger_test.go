package smarttakeloss

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
	"reflect"
	"testing"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// The shared trigger is read off the tick's SMC trend dashboard on every tick (sharedTriggerOf): of
// the rules of SellLossTriggers that hold — at least the rows asked reading the ladder's way, down
// for a long and up for an inverse, and each row asked reading that way on at least the reads asked
// — the strictest names the trigger (more rows, then a higher sum of reads, then more rows asked in
// free fall, then the last listed);
// none holding, or a dashboard not read, leaves it off, when the activation and a self-armed sale
// wait (trendRefusal) while a committed sale trails on. The rows name the trigger in force.

// exampleRules are the user's three examples: two rows with 1h at seven and 4h at eight of nine
// reads, every row with the first two full and 1D at seven, and every row full.
var exampleRules = []SellLossTrigger{
	{MinTrendDirection: 2, MinReads: []int{7, 8}, Percentage: 0.3},
	{MinTrendDirection: 3, MinReads: []int{9, 9, 7}, Percentage: -0.1},
	{MinTrendDirection: 3, MinReads: []int{9, 9, 9}, Percentage: -1.0},
}

// readingDown is the block with the three rows reading down on these reads (the rest up), every row read.
func readingDown(block aggragates.SmartTakeLossIndicators, reads ...int) aggragates.SmartTakeLossIndicators {
	up := make([]int, len(reads))
	for index, down := range reads {
		up[index] = everyRead - down
	}
	return withTrendReads(block, reads, up)
}

// trendNotRead is the block with no row of the trend read.
func trendNotRead(block aggragates.SmartTakeLossIndicators) aggragates.SmartTakeLossIndicators {
	block.SmcTrendDown, block.SmcTrendUp, block.SmcTrendValid = 0, 0, 0
	block.SmcTrendTimeframes, block.SmcTrendRowDown, block.SmcTrendRowUp, block.SmcTrendRowReads = nil, nil, nil, nil
	return block
}

// quietReading is the quiet decline's block (slowDeclineBlock, no verdict) reading down on these reads.
func quietReading(reads ...int) aggragates.AIIndicators {
	return withBlock(readingDown(slowDeclineBlock(false).SmartTakeLoss, reads...))
}

// sharedOf is the trigger the block gives a long or an inverse ladder.
func sharedOf(block aggragates.SmartTakeLossIndicators, inverse bool) sharedTrigger {
	side := aggragates.SideLong
	if inverse {
		side = aggragates.SideShort
	}
	return sharedTriggerOf(series.TrendOf(block), series.ReadsOf(block), side)
}

// inFreeFall is the block with its rows' free fall and free rise as given, one entry per row.
func inFreeFall(block aggragates.SmartTakeLossIndicators, fall, rise []bool) aggragates.SmartTakeLossIndicators {
	block.SmcTrendRowFreeFall, block.SmcTrendRowFreeRise = fall, rise
	return block
}

// profitAt is the profit the ladder's sale books at price, as the rows print it (saleProfit).
func profitAt(trade aggragates.Trades, price float64) float64 {
	pct, _ := saleProfit(trade, price, 0)
	return pct
}

func TestSharedTriggerOfPicksTheStrictestRuleTheDashboardMeets(t *testing.T) {
	withTable(t, exampleRules)
	none := aggragates.SmartTakeLossIndicators{}
	cases := []struct {
		name  string
		block aggragates.SmartTakeLossIndicators
		want  float64
		on    bool
	}{
		{"two rows, 1h seven and 4h eight: the first rule", readingDown(none, 7, 8, 3), 0.3, true},
		{"two rows, 1h seven and 4h nine, 1D four down: still the first rule", readingDown(none, 7, 9, 4), 0.3, true},
		{"two rows, 4h a read short: no rule", readingDown(none, 9, 7, 3), 0, false},
		{"three rows, 1D short of seven: the first rule", readingDown(none, 9, 9, 6), 0.3, true},
		{"three rows, 1D seven: the second rule over the first", readingDown(none, 9, 9, 7), -0.1, true},
		{"every read: the strictest rule over both", readingDown(none, 9, 9, 9), -1.0, true},
		{"three rows down but 1h short of nine: the first rule alone", readingDown(none, 8, 9, 9), 0.3, true},
		{"one row down: no rule", readingDown(none, 9, 4, 3), 0, false},
		{"no row read", trendNotRead(none), 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := sharedOf(c.block, false)
			if got.on != c.on || got.pct != c.want {
				t.Fatalf("trigger = %+v, want on %v at %v", got, c.on, c.want)
			}
		})
	}
}

func TestSharedTriggerOfReadsTheRowsAndTheReadsAsMinimums(t *testing.T) {
	none := aggragates.SmartTakeLossIndicators{}
	cases := []struct {
		name  string
		rules []SellLossTrigger
		block aggragates.SmartTakeLossIndicators
		want  float64
		on    bool
	}{
		{"rows asked are a minimum", []SellLossTrigger{{MinTrendDirection: 2, Percentage: 0.3}}, readingDown(none, 9, 9, 9), 0.3, true},
		{"no rows asked holds on any served trend", []SellLossTrigger{{Percentage: 0.3}}, readingDown(none, 1, 2, 3), 0.3, true},
		{"a zero read asks nothing of its row", []SellLossTrigger{{MinReads: []int{0, 8}, Percentage: 0.3}}, readingDown(none, 1, 8, 1), 0.3, true},
		{"a read asked of a row not served fails", []SellLossTrigger{{MinReads: []int{9, 9, 9, 9}, Percentage: 0.3}}, readingDown(none, 9, 9, 9), 0, false},
		{"reads alone, the counts unasked", []SellLossTrigger{{MinReads: []int{7}, Percentage: 0.3}}, readingDown(none, 7, 0, 0), 0.3, true},
		{"more rows asked wins", []SellLossTrigger{{MinTrendDirection: 3, Percentage: 0.1}, {MinTrendDirection: 1, MinReads: []int{9, 9, 9}, Percentage: 0.2}}, readingDown(none, 9, 9, 9), 0.1, true},
		{"on equal rows the higher sum of reads wins", []SellLossTrigger{{MinTrendDirection: 1, MinReads: []int{9, 9}, Percentage: 0.1}, {MinTrendDirection: 1, MinReads: []int{5, 5, 5}, Percentage: 0.2}}, readingDown(none, 9, 9, 9), 0.1, true},
		{"on equal asks the last listed wins", []SellLossTrigger{{MinTrendDirection: 2, Percentage: 0.1}, {MinTrendDirection: 2, Percentage: 0.2}}, readingDown(none, 9, 9, 9), 0.2, true},
		{"the table's order does not matter", []SellLossTrigger{{MinTrendDirection: 3, MinReads: []int{9, 9, 9}, Percentage: -1}, {MinTrendDirection: 2, MinReads: []int{7, 8}, Percentage: 0.3}}, readingDown(none, 9, 9, 9), -1, true},
		{"an empty table is off", nil, readingDown(none, 9, 9, 9), 0, false},
		{"not read is off whatever the rows say", []SellLossTrigger{{Percentage: 0.3}}, trendNotRead(none), 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withTable(t, c.rules)
			got := sharedOf(c.block, false)
			if got.on != c.on || got.pct != c.want {
				t.Fatalf("trigger = %+v, want on %v at %v", got, c.on, c.want)
			}
			if got.read != (c.block.SmcTrendValid > 0) {
				t.Fatalf("read = %v, want %v", got.read, c.block.SmcTrendValid > 0)
			}
		})
	}
}

// An inverse ladder reads the rows and the reads up: a dashboard that is bearish on every read
// meets no rule for it, one bullish on every read meets the strictest.
func TestSharedTriggerOfReadsTheRowsUpForAnInverseLadder(t *testing.T) {
	withTable(t, exampleRules)
	none := aggragates.SmartTakeLossIndicators{}
	bearish, bullish := readingDown(none, 9, 9, 9), withTrendReads(none, nil, []int{9, 9, 9})
	if got := sharedOf(bearish, true); got.on || !got.inverse {
		t.Fatalf("a bearish dashboard meets no rule for an inverse ladder, got %+v", got)
	}
	if got := sharedOf(bullish, true); !got.on || got.pct != -1.0 {
		t.Fatalf("a bullish dashboard meets the strictest rule for an inverse ladder, got %+v", got)
	}
	if got := sharedOf(withTrendReads(none, nil, []int{7, 8, 3}), true); !got.on || got.pct != 0.3 {
		t.Fatalf("two rows up at seven and eight meet the first rule for an inverse ladder, got %+v", got)
	}
	if got := sharedOf(bullish, false); got.on {
		t.Fatalf("a bullish dashboard meets no rule for a long ladder, got %+v", got)
	}
}

// A rule asking rows in free fall (MinFreeFall) holds only when each of them is in free fall the
// ladder's way beside its other terms: with the 4h and the 1D asked, one of them not in free fall,
// a dashboard serving none, the reads short or the rows short fails it.
func TestAFreeFallRuleHoldsOnlyWhenEveryRowAskedIsInFreeFall(t *testing.T) {
	none := aggragates.SmartTakeLossIndicators{}
	rule := SellLossTrigger{MinTrendDirection: 2, MinReads: []int{7, 8}, MinFreeFall: []int{0, 1, 1}, Percentage: 0.3}
	cases := []struct {
		name  string
		rule  SellLossTrigger
		block aggragates.SmartTakeLossIndicators
		on    bool
	}{
		{"the 4h and the 1D in free fall", rule, inFreeFall(readingDown(none, 7, 8, 3), []bool{false, true, true}, nil), true},
		{"every row in free fall", rule, inFreeFall(readingDown(none, 7, 8, 3), []bool{true, true, true}, nil), true},
		{"only the 4h in free fall", rule, inFreeFall(readingDown(none, 7, 8, 3), []bool{false, true, false}, nil), false},
		{"only the 1D in free fall", rule, inFreeFall(readingDown(none, 7, 8, 3), []bool{false, false, true}, nil), false},
		{"only the 1h in free fall", rule, inFreeFall(readingDown(none, 7, 8, 3), []bool{true, false, false}, nil), false},
		{"no row in free fall", rule, inFreeFall(readingDown(none, 7, 8, 3), []bool{false, false, false}, nil), false},
		{"no free fall served, an older sophos", rule, readingDown(none, 7, 8, 3), false},
		{"free fall served for too few rows", rule, inFreeFall(readingDown(none, 7, 8, 3), []bool{true, true}, nil), false},
		{"the free rise does not count for a long ladder", rule, inFreeFall(readingDown(none, 7, 8, 3), nil, []bool{false, true, true}), false},
		{"both in free fall, the 4h a read short", rule, inFreeFall(readingDown(none, 7, 7, 3), []bool{false, true, true}, nil), false},
		{"both in free fall, the rows short", SellLossTrigger{MinTrendDirection: 3, MinFreeFall: []int{0, 1, 1}, Percentage: 0.3}, inFreeFall(readingDown(none, 7, 8, 3), []bool{true, true, true}, nil), false},
		{"a larger entry asks it like one", SellLossTrigger{MinFreeFall: []int{0, 5, 0}, Percentage: 0.3}, inFreeFall(readingDown(none, 7, 8, 3), []bool{false, true, false}, nil), true},
		{"a free fall row asked of nothing else", SellLossTrigger{MinFreeFall: []int{1}, Percentage: 0.3}, inFreeFall(readingDown(none, 1, 1, 1), []bool{true, false, false}, nil), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withTable(t, []SellLossTrigger{c.rule})
			if got := sharedOf(c.block, false); got.on != c.on || (c.on && got.pct != 0.3) {
				t.Fatalf("trigger = %+v, want on %v at 0.3", got, c.on)
			}
		})
	}
}

// An inverse ladder asks the free rise of the rows instead: a dashboard bullish on the reads and
// with its 4h and 1D in free rise holds the rule, free fall on those rows does not, and the long
// ladder on the same rows reads the free fall alone.
func TestAFreeFallRuleReadsTheFreeRiseForAnInverseLadder(t *testing.T) {
	withTable(t, []SellLossTrigger{{MinTrendDirection: 2, MinReads: []int{7, 8}, MinFreeFall: []int{0, 1, 1}, Percentage: 0.3}})
	none := aggragates.SmartTakeLossIndicators{}
	bullish := withTrendReads(none, nil, []int{7, 8, 3})
	bearish := readingDown(none, 7, 8, 3)
	cases := []struct {
		name    string
		block   aggragates.SmartTakeLossIndicators
		inverse bool
		on      bool
	}{
		{"free rise on the 4h and the 1D, an inverse ladder", inFreeFall(bullish, nil, []bool{false, true, true}), true, true},
		{"free rise on the 4h alone, an inverse ladder", inFreeFall(bullish, nil, []bool{false, true, false}), true, false},
		{"free fall on the 4h and the 1D, an inverse ladder", inFreeFall(bullish, []bool{false, true, true}, nil), true, false},
		{"no free rise served, an inverse ladder", bullish, true, false},
		{"free fall on the 4h and the 1D, a long ladder", inFreeFall(bearish, []bool{false, true, true}, nil), false, true},
		{"free rise on the 4h and the 1D, a long ladder", inFreeFall(bearish, nil, []bool{false, true, true}), false, false},
		{"both served in the 4h and the 1D, each ladder its own", inFreeFall(bearish, []bool{false, true, true}, []bool{false, true, true}), false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sharedOf(c.block, c.inverse); got.on != c.on || (c.on && got.pct != 0.3) {
				t.Fatalf("trigger = %+v, want on %v at 0.3", got, c.on)
			}
		})
	}
}

// Zeros, no entries at all, a short list and a negative entry ask nothing of free fall: the rule is
// read as it was before, the same trigger on the same dashboards whatever they serve.
func TestFreeFallMinimumsAtZeroOrMissingAskNothing(t *testing.T) {
	none := aggragates.SmartTakeLossIndicators{}
	askedNothing := [][]int{nil, {}, {0, 0, 0}, {0}, {0, -1, 0}}
	dashboards := map[string]aggragates.SmartTakeLossIndicators{
		"no free fall served":    readingDown(none, 7, 8, 3),
		"no row in free fall":    inFreeFall(readingDown(none, 7, 8, 3), []bool{false, false, false}, []bool{false, false, false}),
		"every row in free fall": inFreeFall(readingDown(none, 7, 8, 3), []bool{true, true, true}, nil),
		"the reads short":        inFreeFall(readingDown(none, 7, 7, 3), []bool{true, true, true}, nil),
	}
	for name, block := range dashboards {
		withTable(t, []SellLossTrigger{{MinTrendDirection: 2, MinReads: []int{7, 8}, Percentage: 0.3}})
		want := sharedOf(block, false)
		for _, free := range askedNothing {
			withTable(t, []SellLossTrigger{{MinTrendDirection: 2, MinReads: []int{7, 8}, MinFreeFall: free, Percentage: 0.3}})
			if got := sharedOf(block, false); got.on != want.on || got.pct != want.pct {
				t.Errorf("%s with MinFreeFall %v: trigger = on %v at %v, want on %v at %v as without it", name, free, got.on, got.pct, want.on, want.pct)
			}
		}
	}
	withTable(t, []SellLossTrigger{{MinTrendDirection: 2, MinReads: []int{7, 8}, MinFreeFall: []int{0, 0, 0}, Percentage: 0.3}})
	if got := sharedOf(readingDown(none, 7, 8, 3), false); !got.on || got.pct != 0.3 {
		t.Fatalf("fixture drifted: the dashboard must meet the rule without free fall, got %+v", got)
	}
}

// A row asked in free fall that the dashboard did not serve fails the rule, whether it served fewer
// rows than the entry's place or none at all (the counts alone, an older sophos).
func TestARowAskedInFreeFallThatWasNotServedFailsTheRule(t *testing.T) {
	none := aggragates.SmartTakeLossIndicators{}
	twoRows := inFreeFall(readingDown(none, 9, 9), []bool{true, true}, nil)
	counts := aggragates.SmartTakeLossIndicators{SmcTrendDown: 2, SmcTrendValid: 3}
	cases := []struct {
		name  string
		free  []int
		block aggragates.SmartTakeLossIndicators
		on    bool
	}{
		{"the 4h asked of two rows in free fall", []int{0, 1}, twoRows, true},
		{"the 1D asked of two rows in free fall", []int{0, 0, 1}, twoRows, false},
		{"a fourth row asked of three in free fall", []int{0, 0, 0, 1}, inFreeFall(readingDown(none, 9, 9, 9), []bool{true, true, true}, nil), false},
		{"the 1D asked with the counts alone", []int{0, 0, 1}, counts, false},
		{"nothing asked with the counts alone", []int{0, 0, 0}, counts, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withTable(t, []SellLossTrigger{{MinTrendDirection: 1, MinFreeFall: c.free, Percentage: 0.3}})
			if got := sharedOf(c.block, false); got.on != c.on {
				t.Fatalf("trigger = %+v, want on %v", got, c.on)
			}
		})
	}
}

// Of two rules with equal MinTrendDirection and equal MinReads, the one asking more rows in free
// fall is stricter: when both hold it names the trigger whatever the table's order, and when the
// dashboard serves no free fall the other alone holds. A higher sum of reads still beats more free
// fall rows, equal free fall rows fall to the last listed, and FullTrendTrigger takes the strictest.
func TestTheRuleAskingMoreRowsInFreeFallIsTheStricter(t *testing.T) {
	none := aggragates.SmartTakeLossIndicators{}
	plain := SellLossTrigger{MinTrendDirection: 2, MinReads: []int{7, 8}, Percentage: 0.3}
	asking := SellLossTrigger{MinTrendDirection: 2, MinReads: []int{7, 8}, MinFreeFall: []int{0, 1, 1}, Percentage: 0.1}
	oneRow := SellLossTrigger{MinTrendDirection: 2, MinReads: []int{7, 8}, MinFreeFall: []int{0, 0, 1}, Percentage: 0.2}
	falling := inFreeFall(readingDown(none, 9, 9, 3), []bool{false, true, true}, nil)
	cases := []struct {
		name  string
		rules []SellLossTrigger
		block aggragates.SmartTakeLossIndicators
		want  float64
	}{
		{"both hold, the free fall rule listed last", []SellLossTrigger{plain, asking}, falling, 0.1},
		{"both hold, the free fall rule listed first", []SellLossTrigger{asking, plain}, falling, 0.1},
		{"no free fall served, the plain rule alone", []SellLossTrigger{asking, plain}, readingDown(none, 9, 9, 3), 0.3},
		{"two rows asked over one", []SellLossTrigger{asking, oneRow}, falling, 0.1},
		{"two rows asked over one, the other order", []SellLossTrigger{oneRow, asking}, falling, 0.1},
		{"one row asked over none", []SellLossTrigger{oneRow, plain}, falling, 0.2},
		{"equal free fall rows, the last listed", []SellLossTrigger{asking, {MinTrendDirection: 2, MinReads: []int{7, 8}, MinFreeFall: []int{1, 1, 0}, Percentage: 0.4}}, inFreeFall(readingDown(none, 9, 9, 3), []bool{true, true, true}, nil), 0.4},
		{"equal free fall rows, the last listed, the other order", []SellLossTrigger{{MinTrendDirection: 2, MinReads: []int{7, 8}, MinFreeFall: []int{1, 1, 0}, Percentage: 0.4}, asking}, inFreeFall(readingDown(none, 9, 9, 3), []bool{true, true, true}, nil), 0.1},
		{"a higher sum of reads over more free fall rows", []SellLossTrigger{{MinTrendDirection: 2, MinReads: []int{9, 9}, Percentage: 0.6}, {MinTrendDirection: 2, MinReads: []int{5, 5}, MinFreeFall: []int{1, 1, 1}, Percentage: 0.5}}, inFreeFall(readingDown(none, 9, 9, 3), []bool{true, true, true}, nil), 0.6},
		{"more rows over more free fall rows", []SellLossTrigger{{MinTrendDirection: 3, Percentage: 0.6}, {MinTrendDirection: 2, MinFreeFall: []int{1, 1, 1}, Percentage: 0.5}}, inFreeFall(readingDown(none, 9, 9, 9), []bool{true, true, true}, nil), 0.6},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withTable(t, c.rules)
			if got := sharedOf(c.block, false); !got.on || got.pct != c.want {
				t.Fatalf("trigger = %+v, want on at %v", got, c.want)
			}
		})
	}
	withTable(t, []SellLossTrigger{plain, asking, oneRow})
	if got := FullTrendTrigger(); got != 0.1 {
		t.Fatalf("FullTrendTrigger = %v, want the rule asking the most rows in free fall", got)
	}
	withTable(t, []SellLossTrigger{asking, plain})
	if got := FullTrendTrigger(); got != 0.1 {
		t.Fatalf("FullTrendTrigger = %v, want the rule asking free fall whatever the order", got)
	}
}

// The held text marks a row in free fall the ladder's way: " free fall" after a long ladder's row,
// " free rise" after an inverse one's; a row not in it, or in the other one's, prints as before.
func TestTheHeldTextMarksTheRowsInFreeFall(t *testing.T) {
	withTable(t, exampleRules)
	none := aggragates.SmartTakeLossIndicators{}
	served := readingDown(none, 7, 5, 3)
	cases := []struct {
		name    string
		block   aggragates.SmartTakeLossIndicators
		inverse bool
		want    string
	}{
		{"a long ladder, the 4h in free fall", inFreeFall(served, []bool{false, true, false}, nil), false, "SMC trend meets no trigger rule (down on 2 of 3 rows: 1h 7/9, 4h 5/9 free fall, 1D 3/9)"},
		{"a long ladder, every row in free fall", inFreeFall(served, []bool{true, true, true}, nil), false, "SMC trend meets no trigger rule (down on 2 of 3 rows: 1h 7/9 free fall, 4h 5/9 free fall, 1D 3/9 free fall)"},
		{"a long ladder, only a free rise", inFreeFall(served, nil, []bool{false, true, false}), false, "SMC trend meets no trigger rule (down on 2 of 3 rows: 1h 7/9, 4h 5/9, 1D 3/9)"},
		{"a long ladder, no row in free fall", inFreeFall(served, []bool{false, false, false}, []bool{false, false, false}), false, "SMC trend meets no trigger rule (down on 2 of 3 rows: 1h 7/9, 4h 5/9, 1D 3/9)"},
		{"an inverse ladder, the 4h in free rise", inFreeFall(served, nil, []bool{false, true, false}), true, "SMC trend meets no trigger rule (up on 1 of 3 rows: 1h 2/9, 4h 4/9 free rise, 1D 6/9)"},
		{"an inverse ladder, the 1D in free rise", inFreeFall(served, nil, []bool{false, false, true}), true, "SMC trend meets no trigger rule (up on 1 of 3 rows: 1h 2/9, 4h 4/9, 1D 6/9 free rise)"},
		{"an inverse ladder, only a free fall", inFreeFall(served, []bool{false, true, false}, nil), true, "SMC trend meets no trigger rule (up on 1 of 3 rows: 1h 2/9, 4h 4/9, 1D 6/9)"},
	}
	for _, c := range cases {
		if got := sharedOf(c.block, c.inverse).heldByTrend(); got != c.want {
			t.Errorf("%s: heldByTrend = %q, want %q", c.name, got, c.want)
		}
		if got := trendHeldWords(c.block, c.inverse); got != c.want {
			t.Errorf("%s: trendHeldWords = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestFullTrendTriggerIsTheStrictestRule(t *testing.T) {
	withTable(t, exampleRules)
	if got := FullTrendTrigger(); got != -1.0 {
		t.Fatalf("FullTrendTrigger = %v, want the strictest rule's", got)
	}
	withTable(t, []SellLossTrigger{{MinTrendDirection: 3, Percentage: 0.25}, {MinTrendDirection: 1, Percentage: 0.5}, {MinTrendDirection: 3, Percentage: 0.1}})
	if got := FullTrendTrigger(); got != 0.1 {
		t.Fatalf("FullTrendTrigger = %v, want the last of the strictest on a tie", got)
	}
	withTable(t, nil)
	if got := FullTrendTrigger(); got != 0 {
		t.Fatalf("FullTrendTrigger = %v, want zero on an empty table", got)
	}
	if got := StopLossActivatedRow(1, 0.3, FeatureQuietDecline); got.Message != "smartTakeLoss: slow decline stopLoss activated, profit 0.30% of the used amount at or over 0.00%" {
		t.Fatalf("the exported activation row names the full-trend trigger, got %q", got.Message)
	}
}

func TestTheTrendsHeldReasonsNameTheDashboard(t *testing.T) {
	withTable(t, exampleRules)
	none := aggragates.SmartTakeLossIndicators{}
	reasons := []struct {
		name    string
		block   aggragates.SmartTakeLossIndicators
		inverse bool
		want    string
	}{
		{"not read", trendNotRead(none), false, "SMC trend not read"},
		{"rows served, a long ladder", readingDown(none, 7, 5, 3), false, "SMC trend meets no trigger rule (down on 2 of 3 rows: 1h 7/9, 4h 5/9, 1D 3/9)"},
		{"rows served, an inverse ladder", readingDown(none, 7, 5, 3), true, "SMC trend meets no trigger rule (up on 1 of 3 rows: 1h 2/9, 4h 4/9, 1D 6/9)"},
		{"counts alone, an old sophos", withTrendCounts(none, 2, 1), false, "SMC trend meets no trigger rule (down on 2 of 3 rows: 1h 9/9, 4h 9/9, 1D 0/9)"},
	}
	for _, r := range reasons {
		if got := sharedOf(r.block, r.inverse).heldByTrend(); got != r.want {
			t.Errorf("%s: heldByTrend = %q, want %q", r.name, got, r.want)
		}
	}
	countsAlone := aggragates.SmartTakeLossIndicators{SmcTrendDown: 2, SmcTrendValid: 3}
	if got := sharedOf(countsAlone, false).heldByTrend(); got != "SMC trend meets no trigger rule (down on 2 of 3 rows)" {
		t.Errorf("without rows served the reason names the counts alone, got %q", got)
	}
	served := readingDown(none, 7, 5, 3)
	builders := []struct {
		name string
		got  Row
		want Row
	}{
		{"slow decline", SaleHeldByTrendRow(1, served, false, FeatureQuietDecline), saleHeldRow(GateSlowDecline, 1, trendHeldWords(served, false), []string{FeatureQuietDecline})},
		{"flat market", FlatMarketSaleHeldByTrendRow(1, served, true), saleHeldRow(GateFlatMarket, 1, trendHeldWords(served, true), nil)},
		{"flat market", FlatMarketSaleHeldByTrendRow(1, trendNotRead(none), false), saleHeldRow(GateFlatMarket, 1, "SMC trend not read", nil)},
	}
	for _, b := range builders {
		if !reflect.DeepEqual(b.got, b.want) {
			t.Errorf("%s row = %+v, want %+v", b.name, b.got, b.want)
		}
	}
}

// The decision alone: with no rule holding, the bounded triggers are held with their due row, a
// self-armed stopLoss is not sold, capital protection's trigger goes on by its own terms, and a
// committed ladder trails as it does under any dashboard.
func TestSellLossHoldsTheBoundedTriggersWhileNoRuleHolds(t *testing.T) {
	sellLossOn(t)
	withSellLossIgnoreLastDepths(t, 0)
	none := aggragates.SmartTakeLossIndicators{}
	noRule := sharedOf(withTrendReads(none, nil, []int{9, 9, 9}), false)
	notRead := sharedOf(trendNotRead(none), false)
	trade := flatMarketLadder(false)
	fill := trade.PositionPrice
	capital := saleTrigger{reason: reasonCapitalProtection, gate: GateCapitalProtection, rowDue: true, terms: capitalProtectionTerms}
	gain := ladder.AverageEntryPrice(trade) * gainFactor

	var cases []sellLossCase
	for _, state := range []string{"buy", "stopLoss"} {
		cases = append(cases,
			onLadder(state+", no rule, each gate held", trade, state, gain, eachGate()...).holding(fill, noRule.heldByTrend()).under(noRule),
			onLadder(state+", not read, each gate held", trade, state, gain, eachGate()...).holding(fill, notRead.heldByTrend()).under(notRead),
			onLadder(state+", no rule, rows already written", trade, state, gain, slowTrigger(false), lowTrigger(false)).under(noRule),
			onLadder(state+", no rule, capital protection goes on alone", trade, state, gain, slowTrigger(true), capital).
				withCapitalProtection(fill, gain, state, noRule.heldByTrend()).under(noRule),
		)
	}
	trailing := trailCases("no rule, committed", trade, "stopLoss", gain, activatedOf(slowTrigger(true)), lowTrigger(true))
	for index := range trailing {
		trailing[index].shared = &noRule
	}
	runSellLoss(t, append(cases, trailing...))
}

// under is the case decided under this shared trigger.
func (c sellLossCase) under(shared sharedTrigger) sellLossCase {
	c.shared = &shared
	return c
}

// withCapitalProtection is the case of a slow decline trigger the trend holds beside capital
// protection's, which goes on by its own terms: in buy the slow decline's held row and the
// activation row of capital protection (a gain is within the drawdown), in stopLoss its sale alone.
func (c sellLossCase) withCapitalProtection(fill, price float64, state, why string) sellLossCase {
	capital := c.triggers[len(c.triggers)-1]
	if state == "stopLoss" {
		c.step, c.reason, c.rows = stepSell, capital.reason, nil
		return c
	}
	c.step = stepActivate
	c.rows = []Row{
		heldRowOf(c.triggers[0], fill, why),
		activationRowOf(capital, fill, activatedText(-profitAt(c.trade, price), capitalProtectionMaxDrawdown)),
	}
	return c
}

// Through Apply, the example of trade 162024 under the user's rules: a pending quiet decline whose
// sale would book a small gain is held while the dashboard meets the first rule (a gain under its
// entry), activated once it meets the second (from break even) and activated on a loss within the
// last rule once every read reads down; a dashboard meeting no rule, or not read, holds it with the
// trend's row once per episode whatever the profit, and a self-armed stopLoss is not sold there
// (ExitReached agrees). The dashboard is read on each tick, never kept from the tick the gate
// triggered: held on one bar, the same price activates on the next, and the committed ladder then
// trails to its sale under a dashboard meeting no rule at all.
func TestApplyReadsTheTriggerOffTheDashboardOnEveryTick(t *testing.T) {
	withQuietDecline(t, true)
	withTable(t, exampleRules)
	start := pendingTrade()
	fill := slowDeclineLastFill
	gain, loss := ladder.AverageEntryPrice(start)*1.001, ladder.AverageEntryPrice(start)*0.997
	if pct := profitAt(start, gain); pct <= 0 || pct >= 0.3 {
		t.Fatalf("fixture drifted: the gain must book between the second and the first rule, got %.4f%%", pct)
	}
	if pct := profitAt(start, loss); pct >= -0.1 || pct < -1.0 {
		t.Fatalf("fixture drifted: the loss must book within the last rule alone, got %.4f%%", pct)
	}
	features := []string{FeatureQuietDecline}
	noRule := quietReading(9, 4, 3)
	heldByTrend := SaleHeldByTrendRow(fill, noRule.SmartTakeLoss, false, FeatureQuietDecline)

	assertWaits(t, Apply(start, "", gain, quietReading(7, 8, 3)), "", saleHeldByProfitRow(GateSlowDecline, fill, profitAt(start, gain), 0.3, features))
	assertActivated(t, Apply(start, "", gain, quietReading(9, 9, 7)), stopLossActivatedRow(GateSlowDecline, fill, profitAt(start, gain), -0.1, features))
	assertActivated(t, Apply(start, "", loss, quietReading(9, 9, 9)), stopLossActivatedRow(GateSlowDecline, fill, profitAt(start, loss), -1.0, features))
	assertWaits(t, Apply(start, "", loss, quietReading(9, 9, 7)), "", saleHeldByProfitRow(GateSlowDecline, fill, profitAt(start, loss), -0.1, features))
	for _, price := range []float64{gain, loss} {
		assertWaits(t, Apply(start, "", price, noRule), "", heldByTrend)
		assertUntouched(t, Apply(withRows(start, heldByTrend), "", price, noRule), "")
		assertWaits(t, Apply(start, "", price, withBlock(trendNotRead(noRule.SmartTakeLoss))), "", saleHeldRow(GateSlowDecline, fill, "SMC trend not read", features))

		armed := restingIn(start, "stopLoss")
		assertWaits(t, Apply(armed, "update_stopLoss", price, noRule), "update_stopLoss", heldByTrend)
		if ExitReached(armed, price, noRule.SmartTakeLoss) {
			t.Fatalf("ExitReached must agree that no rule holding sells nothing at %v", price)
		}
		assertSold(t, Apply(armed, "update_stopLoss", price, quietReading(9, 9, 9)), reasonSlowDecline)
	}

	trade, got := engineStep(start, "", gain, testutil.At("18:00:00"), quietReading(7, 8, 3))
	assertWaits(t, got, "", saleHeldByProfitRow(GateSlowDecline, fill, profitAt(start, gain), 0.3, features))
	trade, got = engineStep(trade, "", gain, testutil.At("19:00:00"), quietReading(9, 9, 7))
	assertActivated(t, got, stopLossActivatedRow(GateSlowDecline, fill, profitAt(start, gain), -0.1, features))
	if trade.PositionType != "stopLoss" || trade.PositionPrice != gain {
		t.Fatalf("fixture drifted: the activation must anchor the ladder in stopLoss at %v, got %s at %v", gain, trade.PositionType, trade.PositionPrice)
	}
	assertTrailHeld(t, Apply(trade, "buy", gain, noRule))
	assertSold(t, Apply(trade, "buy", priceAtMove(trade, gain, -(trailRow(trade).Tolerance+1)), noRule), reasonSlowDecline)
}

// An inverse ladder reads the rows up: with the flat market episode open, a bearish dashboard
// holds its sale with the trend's row, two rows up at the first rule's reads hold it under that
// rule's entry, every read up activates it on the strictest.
func TestAnInverseLadderReadsTheTriggerOffTheRowsUp(t *testing.T) {
	flatMarketAlone(t)
	withTable(t, exampleRules)
	start := flatMarketLadder(true)
	ai, fill := flatMarketMarket(start), start.PositionPrice
	open := withRows(start, triggeredBy(start, ai))
	price := ladder.AverageEntryPrice(start) * 1.003
	if pct := profitAt(open, price); pct >= -0.1 || pct < -1.0 {
		t.Fatalf("fixture drifted: the inverse ladder must book a loss within the last rule alone at %v, got %.4f%%", price, pct)
	}
	reading := func(down, up []int) aggragates.AIIndicators {
		return withBlock(withTrendReads(ai.SmartTakeLoss, down, up))
	}
	bearish := reading([]int{9, 9, 9}, nil)

	assertWaits(t, Apply(open, "", price, bearish), "", FlatMarketSaleHeldByTrendRow(fill, bearish.SmartTakeLoss, true))
	assertWaits(t, Apply(open, "", price, reading(nil, []int{7, 8, 3})), "", saleHeldByProfitRow(GateFlatMarket, fill, profitAt(open, price), 0.3, nil))
	assertActivated(t, Apply(open, "", price, reading(nil, []int{9, 9, 9})), stopLossActivatedRow(GateFlatMarket, fill, profitAt(open, price), -1.0, nil))
}

// Capital protection reads no table: it activates within its drawdown under a dashboard meeting no
// rule, the shared trigger off.
func TestCapitalProtectionIgnoresTheTriggerTable(t *testing.T) {
	capitalProtectionAlone(t)
	withTable(t, exampleRules)
	trade := lastDepthLadder()
	within, _ := firstPriceWithinTheDrawdown(t, trade)
	for _, block := range []aggragates.SmartTakeLossIndicators{readingDown(solBlock(), 1, 1, 1), trendNotRead(solBlock())} {
		assertActivated(t, Apply(trade, "", within, withBlock(block)), capitalProtectionTriggered(trade), CapitalProtectionStopLossActivatedRow(trade.PositionPrice, -handLossPct(trade, within)))
	}
}
