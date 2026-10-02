package smarttakeloss

import (
	"math"
	"reflect"
	"slices"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// gridCloses are the positions a close was decided in: as the proposal or
// as the trade's own state, Apply replaces none of them.
var gridCloses = map[string]bool{
	"sell":                    true,
	"takeProfit":              true,
	"update_takeProfit":       true,
	"sellParent":              true,
	"impasse":                 true,
	"sellLoss":                true,
	"forceTrailingTakeProfit": true,
}

// The flag owns the overlay: without it, on a child, on no price or without
// settings Apply hands the proposal back untouched with no row, whatever the
// block — and so it does on a trade no rule watches: a long ladder short of
// QuietDeclineArmDepth, of IndecisionArmDepth, of SwingDeclineArmDepth and of
// its last depth, a ladder with no fill, an inverse ladder. The block serves
// every reading at once.
func TestApplyInertWithoutTheFlagOnAChildOrWithoutInputs(t *testing.T) {
	reading := solBlock()
	reading.SlowDeclineExit = true
	reading.SlowDeclineSellBand = capitalProtectionBand
	reading.SlowDeclineIndecision = true
	reading.SlowDeclineBreakReasons = indecisionReasons
	reading.SwingDeclineOpens, reading.SwingDeclineCloses = []int64{1}, []float64{1}
	off := lastDepthLadder()
	off.Strategy.Params.SmartTakeLoss = false
	child := lastDepthLadder()
	child.ParentID = 7
	bare := lastDepthLadder()
	bare.StrategyPair.StrategySettings = nil

	for name, tc := range map[string]struct {
		trade aggragates.Trades
		price float64
	}{
		"without the flag":     {off, capitalProtectionBand},
		"on a child":           {child, capitalProtectionBand},
		"without settings":     {bare, capitalProtectionBand},
		"without a price":      {lastDepthLadder(), 0},
		"short of every watch": {testutil.LadderTrade(false, fills(min(QuietDeclineArmDepth, IndecisionArmDepth, SwingDeclineArmDepth)-1, "17:38:00")...), capitalProtectionBand},
		"with no fill":         {testutil.LadderTrade(false), capitalProtectionBand},
		"on an inverse ladder": {testutil.LadderTrade(true, fills(lastDepthFills, "21:30:00")...), capitalProtectionBand},
	} {
		for _, position := range []string{"", "stopLoss"} {
			if got := Apply(tc.trade, position, tc.price, withBlock(reading)); !reflect.DeepEqual(got, Result{Position: position}) {
				t.Errorf("%s, %q: got %+v", name, position, got)
			}
		}
	}
}

// gridState is a state the grid's ladders carry in their events — written the
// way the engines write it, or as an older release wrote it — with what it puts
// the ladder in when the features that read it watch it: the decline features it
// is pending on, and whether it latched the ladder. Each is stated here by hand,
// never read off the fold it checks.
type gridState struct {
	name      string
	write     func(trade aggragates.Trades) aggragates.Trades
	pendingOn []string
	latched   bool
}

// writing is the state's step of the trade's newest-fill rows, built with the
// price of that fill.
func writing(build func(price float64) []Row) func(aggragates.Trades) aggragates.Trades {
	return func(trade aggragates.Trades) aggragates.Trades {
		return withRows(trade, build(trade.PositionPrice)...)
	}
}

// pendingStep, cancelledStep and latchedStep are the steps of one row each at
// the newest fill, on the findings given: the engine writing it.
func pendingStep(findings ...Finding) func(aggragates.Trades) aggragates.Trades {
	return writing(func(price float64) []Row {
		return []Row{PendingRow(price, findings...)}
	})
}

func cancelledStep(findings ...Finding) func(aggragates.Trades) aggragates.Trades {
	return writing(func(price float64) []Row {
		return []Row{CancelledRow(price, findings...)}
	})
}

func latchedStep(findings ...Finding) func(aggragates.Trades) aggragates.Trades {
	return writing(func(price float64) []Row {
		return []Row{LatchedRow(price, findings...)}
	})
}

// legacyEvent is the step of an event the gates before the merge wrote, a
// document without features, at the newest fill's price.
func legacyEvent(gate, kind string) func(aggragates.Trades) aggragates.Trades {
	return func(trade aggragates.Trades) aggragates.Trades {
		return withEvents(trade, eventOf(gate, legacyDocument(kind, trade.PositionPrice)))
	}
}

// chained is the steps in order.
func chained(steps ...func(aggragates.Trades) aggragates.Trades) func(aggragates.Trades) aggragates.Trades {
	return func(trade aggragates.Trades) aggragates.Trades {
		for _, apply := range steps {
			trade = apply(trade)
		}
		return trade
	}
}

func gridStates() []gridState {
	quiet, swing, both := []string{FeatureQuietDecline}, []string{FeatureSwingDecline}, []string{FeatureQuietDecline, FeatureSwingDecline}
	onQuiet := pendingStep(quietFinding(nil))
	onSwing := pendingStep(swingFinding(nil))
	cancelled := cancelledStep(quietFinding(nil), swingFinding(nil))
	legacyQuiet, legacySwing := legacyEvent(GateSlowDecline, EventPending), legacyEvent(gateSlowPattern, EventPending)
	return []gridState{
		{"nothing", chained(), nil, false},
		{"pending on the quiet decline", onQuiet, quiet, false},
		{"pending on the swing decline", onSwing, swing, false},
		{"pending on both", pendingStep(quietFinding(nil), swingFinding(nil)), both, false},
		{"latched", latchedStep(indecisionFinding(nil)), nil, true},
		{"pending and latched on one row", pendingStep(indecisionFinding(nil), quietFinding(nil)), quiet, true},
		{"pending on the quiet decline, cancelled, pending on the swing decline", chained(onQuiet, cancelled, onSwing), swing, false},
		{"legacy: the quiet decline pending", legacyQuiet, quiet, false},
		{"legacy: the slow pattern pending", legacySwing, swing, false},
		{"legacy: both pending", chained(legacyQuiet, legacySwing), both, false},
		{"legacy: both pending, the pattern cancelled", chained(legacyQuiet, legacySwing, legacyEvent(gateSlowPattern, EventCancelled)), quiet, false},
		{"legacy: latched", legacyEvent(gateIndecision, EventLatched), nil, true},
		{"legacy: the quiet decline pending, a cancelled row after", chained(legacyQuiet, cancelled), nil, false},
		{"legacy: the quiet decline pending, a swing decline pending row after", chained(legacyQuiet, onSwing), swing, false},
	}
}

// gridCase is a ladder with its state.
type gridCase struct {
	trade aggragates.Trades
	stand gridState
}

// gridDepths are the fill counts of the grid's ladders: one short of each
// rule's watch and at it — QuietDeclineArmDepth, IndecisionArmDepth,
// SwingDeclineArmDepth and the last depth — each once.
func gridDepths() []int {
	var depths []int
	for _, depth := range []int{QuietDeclineArmDepth - 1, QuietDeclineArmDepth, IndecisionArmDepth - 1, IndecisionArmDepth, SwingDeclineArmDepth - 1, SwingDeclineArmDepth, lastDepthFills - 1, lastDepthFills} {
		if depth > 0 && !slices.Contains(depths, depth) {
			depths = append(depths, depth)
		}
	}
	return depths
}

// gridCases are the ladders the grid runs on: the w3s ladder at every
// gridDepths count — long, inverse, futures, under an impasse strategy, a
// child and without the flag — each carrying every state, the newest fill the
// one every state names.
func gridCases() []gridCase {
	kinds := []func(*aggragates.Trades){
		func(*aggragates.Trades) {},
		func(trade *aggragates.Trades) { trade.Strategy.TradeType = aggragates.Futures },
		func(trade *aggragates.Trades) { trade.Strategy.Params.Impasse = true },
		func(trade *aggragates.Trades) { trade.ParentID = 7 },
		func(trade *aggragates.Trades) { trade.Strategy.Params.SmartTakeLoss = false },
	}
	var cases []gridCase
	for _, depth := range gridDepths() {
		ladders := []aggragates.Trades{testutil.LadderTrade(true, fills(depth, "21:30:00")...)}
		for _, kind := range kinds {
			trade := testutil.LadderTrade(false, fills(depth, "21:30:00")...)
			kind(&trade)
			ladders = append(ladders, trade)
		}
		for _, trade := range ladders {
			for _, st := range gridStates() {
				cases = append(cases, gridCase{st.write(trade), st})
			}
		}
	}
	return cases
}

// expectedApply is Apply stated on its own for the grid's ladders, whose
// states name the newest fill and whose blocks serve no verdict, no quiet leg
// and no series, so no row but the sophos latch ever comes back. The switches
// are, in order, the quiet decline's, capital protection's, the indecision
// direction's and the swing decline's.
//
// Past the guards, a ladder the indecision direction watches — a long spot one
// from IndecisionArmDepth fills — that no state latched gets its row on a block
// serving the indecision reading, whatever the proposal and the trade's state.
// Past the closes the ladder or the trade already decided, a ladder pending on a
// decline feature that is switched on and watches it sells at the sell band, and
// else a ladder at its last depth capital protection watches sells at the upper
// band while the SMC trend reads bearish.
func expectedApply(trade aggragates.Trades, st gridState, position string, price float64, block aggragates.SmartTakeLossIndicators, switches [4]bool) Result {
	want := Result{Position: position}
	if !trade.Strategy.Params.SmartTakeLoss || trade.ParentID != 0 || price <= 0 || len(trade.StrategyPair.StrategySettings) == 0 {
		return want
	}
	filled := ladder.CountFilledEntries(trade)
	spot := !trade.Inverse && trade.Strategy.TradeType != aggragates.Futures
	quiet := switches[0] && !trade.Inverse && filled >= QuietDeclineArmDepth
	indecisionWatches := switches[2] && spot && filled >= IndecisionArmDepth
	swing := switches[3] && spot && filled >= SwingDeclineArmDepth
	if indecisionWatches && block.SlowDeclineIndecision && !st.latched {
		want.Rows = []Row{LatchedRow(trade.PositionPrice, indecisionFinding(block.SlowDeclineBreakReasons))}
	}
	if gridCloses[position] || gridCloses[trade.PositionType] {
		return want
	}
	pending := false
	for _, feature := range st.pendingOn {
		pending = pending || (feature == FeatureQuietDecline && quiet) || (feature == FeatureSwingDecline && swing)
	}
	lastDepth := switches[1] && spot && !trade.Strategy.Params.Impasse && filled >= int(trade.StrategyPair.StrategySettings[0].Depths)
	switch {
	case pending && block.SlowDeclineSellBand > 0 && price >= block.SlowDeclineSellBand:
		want.Position, want.Reason = "sellLoss", reasonSellBand
	case lastDepth && block.CapitalProtectionSmcBearish && block.CapitalProtectionUpperBB > 0 && price >= block.CapitalProtectionUpperBB:
		want.Position, want.Reason = "sellLoss", reasonCapitalProtection
	}
	return want
}

// expectedUnderTheCap is expectedApply under the shipped loss cap, stated on its
// own: where expectedApply sells at the sell band and that sale loses more than
// the cap — the loss worked out by hand from the ladder's fills (handLossPct) —
// the sale is held. Held, the tick sells nothing at the band, so what is left is
// what the ladder gets when no band is served: its proposal, or capital
// protection's sale, and the latch row if one lands. The held sale's row comes
// last, after it, at the newest fill and naming the declines the state is pending
// on that watch the ladder; the grid's states never carry a held event, so the
// row is always due.
func expectedUnderTheCap(trade aggragates.Trades, st gridState, position string, price float64, block aggragates.SmartTakeLossIndicators, switches [4]bool) Result {
	want := expectedApply(trade, st, position, price, block, switches)
	if want.Reason != reasonSellBand || handLossPct(trade, price) <= SlowDeclineMaxLossPct {
		return want
	}
	unserved := block
	unserved.SlowDeclineSellBand = 0
	want = expectedApply(trade, st, position, price, unserved, switches)
	want.Rows = append(want.Rows, SaleHeldRow(trade.PositionPrice, handLossPct(trade, price), heldDeclines(trade, st, switches)...))
	return want
}

// heldDeclines are the declines a state is pending on that watch the ladder, in
// the order the rows name them: the quiet decline from QuietDeclineArmDepth fills
// on any long ladder, the swing decline from SwingDeclineArmDepth on a spot one.
func heldDeclines(trade aggragates.Trades, st gridState, switches [4]bool) []string {
	filled := ladder.CountFilledEntries(trade)
	spot := !trade.Inverse && trade.Strategy.TradeType != aggragates.Futures
	var declines []string
	if slices.Contains(st.pendingOn, FeatureQuietDecline) && switches[0] && !trade.Inverse && filled >= QuietDeclineArmDepth {
		declines = append(declines, FeatureQuietDecline)
	}
	if slices.Contains(st.pendingOn, FeatureSwingDecline) && switches[3] && spot && filled >= SwingDeclineArmDepth {
		declines = append(declines, FeatureSwingDecline)
	}
	return declines
}

// gridBlocks serve the two bands in either order with the SMC trend bearish,
// both bands with it not bearish, and no band at all — then the indecision
// reading with the bands and the trend bearish, and on its own.
func gridBlocks() []aggragates.SmartTakeLossIndicators {
	lower, upper := capitalProtectionBand, slowDeclineBand
	return []aggragates.SmartTakeLossIndicators{
		{SlowDeclineSellBand: upper, CapitalProtectionUpperBB: lower, CapitalProtectionSmcBearish: true},
		{SlowDeclineSellBand: lower, CapitalProtectionUpperBB: upper, CapitalProtectionSmcBearish: true},
		{SlowDeclineSellBand: lower, CapitalProtectionUpperBB: lower},
		{CapitalProtectionSmcBearish: true},
		{SlowDeclineSellBand: upper, CapitalProtectionUpperBB: lower, CapitalProtectionSmcBearish: true, SlowDeclineIndecision: true, SlowDeclineBreakReasons: indecisionReasons},
		{SlowDeclineIndecision: true},
	}
}

// gridPrices sit under, at, between and over the two bands.
func gridPrices() []float64 {
	lower, upper := capitalProtectionBand, slowDeclineBand
	return []float64{math.Nextafter(lower, 0), lower, (lower + upper) / 2, upper, upper + 1}
}

// gridSwitches are the four switches' combinations the grids run under, in the
// order the quiet decline, capital protection, the indecision direction and the
// swing decline are read: every one of the sixteen.
func gridSwitches() [][4]bool {
	var combinations [][4]bool
	for bits := 15; bits >= 0; bits-- {
		combinations = append(combinations, [4]bool{bits&8 != 0, bits&4 != 0, bits&2 != 0, bits&1 != 0})
	}
	return combinations
}

// setGridSwitches runs the calling test with the four switches as given.
func setGridSwitches(t *testing.T, switches [4]bool) {
	t.Helper()
	withQuietDecline(t, switches[0])
	withCapitalProtectionExit(t, switches[1])
	withIndecision(t, switches[2])
	withSwingDecline(t, switches[3])
}

// Every grid ladder, state, proposal, block and price, under the four
// switches in every position: Apply answers expectedApply exactly.
func TestApplyOnTheGrid(t *testing.T) {
	positions := []string{"", "buy", "stopLoss", "update_stopLoss", "update_buy", "forceTrailingStopLoss", "takeProfit", "forceTrailingTakeProfit"}
	states := []string{"buy", "stopLoss", "takeProfit", "sellLoss"}
	blocks, prices := gridBlocks(), gridPrices()
	sold := map[string]int{}
	rows := 0
	for _, switches := range gridSwitches() {
		setGridSwitches(t, switches)
		for _, c := range gridCases() {
			trade := c.trade
			for _, state := range states {
				trade.PositionType = state
				for _, position := range positions {
					for _, block := range blocks {
						for _, price := range prices {
							got := Apply(trade, position, price, withBlock(block))
							want := expectedApply(trade, c.stand, position, price, block, switches)
							if !reflect.DeepEqual(got, want) {
								t.Fatalf("switches %v, gate state %q, %d fills (inverse %v, state %q, events %d), %q at %v, block %+v:\ngot  %+v\nwant %+v",
									switches, c.stand.name, len(trade.History), trade.Inverse, state, len(trade.StrategyEvents), position, price, block, got, want)
							}
							sold[got.Reason]++
							rows += len(got.Rows)
						}
					}
				}
			}
		}
	}
	if sold[reasonSellBand] == 0 || sold[reasonCapitalProtection] == 0 || sold[""] == 0 || rows == 0 {
		t.Fatalf("fixture drifted: the grid must sell under both reasons, keep proposals and hand back latch rows, got %v and %d rows", sold, rows)
	}
	t.Logf("answers by reason: %v, rows: %d", sold, rows)
}

// The same grid under the shipped loss cap, over the combinations of switches that
// differ in what watches a ladder: Apply answers expectedUnderTheCap exactly. The
// sale at the sell band is held where it loses more than the cap and goes where it
// does not, capital protection sells over a held band sale, and every other
// answer is the uncapped grid's. One block more than the grid's serves both bands
// at the lower one with the trend bearish, the only one where capital protection's
// band is reached below the loss the cap holds.
func TestApplyOnTheGridUnderTheCap(t *testing.T) {
	withLossCap(t, SlowDeclineMaxLossPct)
	positions := []string{"", "buy", "stopLoss", "takeProfit"}
	states := []string{"buy", "stopLoss", "sellLoss"}
	prices := gridPrices()
	blocks := append(gridBlocks(), aggragates.SmartTakeLossIndicators{
		SlowDeclineSellBand:         capitalProtectionBand,
		CapitalProtectionUpperBB:    capitalProtectionBand,
		CapitalProtectionSmcBearish: true,
	})
	seen := map[string]int{}
	for _, switches := range [][4]bool{{true, true, true, true}, {true, false, true, true}, {true, true, false, false}, {false, true, true, true}, {false, false, false, true}} {
		setGridSwitches(t, switches)
		for _, c := range gridCases() {
			trade := c.trade
			for _, state := range states {
				trade.PositionType = state
				for _, position := range positions {
					for _, block := range blocks {
						for _, price := range prices {
							got := Apply(trade, position, price, withBlock(block))
							want := expectedUnderTheCap(trade, c.stand, position, price, block, switches)
							if !reflect.DeepEqual(got, want) {
								t.Fatalf("switches %v, gate state %q, %d fills (inverse %v, state %q, events %d), %q at %v, block %+v:\ngot  %+v\nwant %+v",
									switches, c.stand.name, len(trade.History), trade.Inverse, state, len(trade.StrategyEvents), position, price, block, got, want)
							}
							held := len(want.Rows) > 0 && want.Rows[len(want.Rows)-1].Event == EventSaleHeld
							switch {
							case held && want.Reason == reasonCapitalProtection:
								seen["capital protection over a held sale"]++
							case held:
								seen["held"]++
							case want.Reason == reasonSellBand:
								seen["sold at the band"]++
							}
						}
					}
				}
			}
		}
	}
	for _, key := range []string{"held", "sold at the band", "capital protection over a held sale"} {
		if seen[key] == 0 {
			t.Fatalf("fixture drifted: the grid must reach %q under the cap, got %v", key, seen)
		}
	}
	t.Logf("answers under the cap: %v", seen)
}
