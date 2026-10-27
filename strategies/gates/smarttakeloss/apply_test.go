package smarttakeloss

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
	"math"
	"reflect"
	"slices"
	"testing"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/earlytakeprofit"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/quietdecline"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/swingdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// gridCloses are the positions a close was decided in: as the proposal or
// as the trade's own state, Apply replaces none of them but a take profit
// proposed in buy (gridYields).
var gridCloses = map[string]bool{
	"sell":                    true,
	"takeProfit":              true,
	"update_takeProfit":       true,
	"sellParent":              true,
	"impasse":                 true,
	"sellLoss":                true,
	"forceTrailingTakeProfit": true,
}

// gridYields is whether a close the grid proposes still meets the shared action: a take profit
// proposed in buy, which only the activation replaces.
func gridYields(position, state string) bool {
	return (position == "takeProfit" || position == "forceTrailingTakeProfit") && state == "buy"
}

// gridPassesThrough is whether Apply hands a grid tick back past the latch: a close in the state,
// or a close proposed that does not yield (gridYields).
func gridPassesThrough(position, state string) bool {
	return gridCloses[state] || (gridCloses[position] && !gridYields(position, state))
}

// The flag owns the overlay: without it, on a child, on no price or without settings, and on a trade
// no rule watches (short of every arm depth and the last depth, no fill, inverse short of its last
// depth), Apply hands the proposal back untouched with no row, in buy and in stopLoss, whatever the block serves.
func TestApplyInertWithoutTheFlagOnAChildOrWithoutInputs(t *testing.T) {
	withQuietDecline(t, true)
	withSwingDecline(t, true)
	withCapitalProtectionExit(t, true)
	reading := solBlock()
	reading.CapitalProtectionSmcBullish = true
	reading.QuietDeclineVerdict = true
	reading.QuietDeclineFillFrom = fillWindowFrom.UnixMilli()
	reading.Indecision = true
	reading.QuietDeclineBreakReasons = indecisionReasons
	reading.SeriesOpens, reading.SeriesCloses = []int64{1}, []float64{1}
	pending := func(trade aggragates.Trades) aggragates.Trades {
		return withRows(trade, PendingRow(trade.PositionPrice, quietFinding(nil), swingFinding(nil)))
	}
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
		"without the flag":     {pending(off), lastDepthPrint},
		"on a child":           {pending(child), lastDepthPrint},
		"without settings":     {pending(bare), lastDepthPrint},
		"without a price":      {pending(lastDepthLadder()), 0},
		"short of every watch": {pending(testutil.LadderTrade(false, fills(min(quietdecline.ArmDepth, earlytakeprofit.ArmDepth, swingdecline.ArmDepth)-1, "17:38:00")...)), lastDepthPrint},
		"with no fill":         {pending(testutil.LadderTrade(false)), lastDepthPrint},
		"on an inverse ladder": {pending(testutil.LadderTrade(true, fills(lastDepthFills-1, "21:30:00")...)), lastDepthPrint},
	} {
		for _, state := range []string{"buy", "stopLoss"} {
			for _, position := range []string{"", "stopLoss"} {
				if got := Apply(restingIn(tc.trade, state), position, tc.price, withBlock(reading)); !reflect.DeepEqual(got, Result{Position: position}) {
					t.Errorf("%s, in %s, %q: got %+v", name, state, position, got)
				}
			}
		}
	}
}

// gridState is a state the grid's ladders carry in their events (as the engines or an older release
// wrote it) with what it puts the ladder in: the pending features, the latch, saleHeld and activated;
// stated by hand, never read off the fold.
type gridState struct {
	name      string
	write     func(trade aggragates.Trades) aggragates.Trades
	pendingOn []string
	latched   bool
	saleHeld  bool
	activated bool
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

// heldStep and activatedStep are the steps of the shared action's rows at the newest fill, naming the features given.
func heldStep(features ...string) func(aggragates.Trades) aggragates.Trades {
	return writing(func(price float64) []Row {
		return []Row{SaleHeldRow(price, -1, features...)}
	})
}

func activatedStep(features ...string) func(aggragates.Trades) aggragates.Trades {
	return writing(func(price float64) []Row {
		return []Row{StopLossActivatedRow(price, -0.25, features...)}
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
	onBoth := pendingStep(quietFinding(nil), swingFinding(nil))
	cancelled := cancelledStep(quietFinding(nil), swingFinding(nil))
	legacyQuiet, legacySwing := legacyEvent(GateSlowDecline, EventPending), legacyEvent(gateSlowPattern, EventPending)
	return []gridState{
		{name: "nothing", write: chained()},
		{name: "pending on the quiet decline", write: onQuiet, pendingOn: quiet},
		{name: "pending on the swing decline", write: onSwing, pendingOn: swing},
		{name: "pending on both", write: onBoth, pendingOn: both},
		{name: "latched", write: latchedStep(indecisionFinding(nil)), latched: true},
		{name: "pending and latched on one row", write: pendingStep(indecisionFinding(nil), quietFinding(nil)), pendingOn: quiet, latched: true},
		{name: "pending on the quiet decline, cancelled, pending on the swing decline", write: chained(onQuiet, cancelled, onSwing), pendingOn: swing},
		{name: "pending on the quiet decline, its sale held", write: chained(onQuiet, heldStep(FeatureQuietDecline)), pendingOn: quiet, saleHeld: true},
		{name: "pending on the quiet decline, its stopLoss activated", write: chained(onQuiet, activatedStep(FeatureQuietDecline)), pendingOn: quiet, activated: true},
		{name: "pending on both, the sale held and the stopLoss activated", write: chained(onBoth, heldStep(both...), activatedStep(both...)), pendingOn: both, saleHeld: true, activated: true},
		{name: "pending on the quiet decline, activated, pending again on the swing decline", write: chained(onQuiet, activatedStep(FeatureQuietDecline), onSwing), pendingOn: swing},
		{name: "pending on the quiet decline, held and activated, cancelled, pending on both", write: chained(onQuiet, heldStep(FeatureQuietDecline), activatedStep(FeatureQuietDecline), cancelled, onBoth), pendingOn: both},
		{name: "legacy: the quiet decline pending", write: legacyQuiet, pendingOn: quiet},
		{name: "legacy: the slow pattern pending", write: legacySwing, pendingOn: swing},
		{name: "legacy: both pending", write: chained(legacyQuiet, legacySwing), pendingOn: both},
		{name: "legacy: both pending, the pattern cancelled", write: chained(legacyQuiet, legacySwing, legacyEvent(gateSlowPattern, EventCancelled)), pendingOn: quiet},
		{name: "legacy: latched", write: legacyEvent(gateIndecision, EventLatched), latched: true},
		{name: "legacy: the quiet decline pending, a cancelled row after", write: chained(legacyQuiet, cancelled)},
		{name: "legacy: the quiet decline pending, a swing decline pending row after", write: chained(legacyQuiet, onSwing), pendingOn: swing},
		{name: "legacy: the quiet decline pending, its stopLoss activated", write: chained(legacyQuiet, activatedStep(FeatureQuietDecline)), pendingOn: quiet, activated: true},
	}
}

// gridCase is a ladder with its state.
type gridCase struct {
	trade     aggragates.Trades
	gateState gridState
}

// gridDepths are the fill counts of the grid's ladders: one short of each rule's watch and at it, one
// short of the activation floor and at it, the deepest the depth bound lets through and past it, and the last depth.
func gridDepths() []int {
	var depths []int
	for _, depth := range []int{
		quietdecline.ArmDepth - 1, quietdecline.ArmDepth, earlytakeprofit.ArmDepth - 1, earlytakeprofit.ArmDepth, swingdecline.ArmDepth - 1, swingdecline.ArmDepth,
		SellLossStartAtDepth - 1, SellLossStartAtDepth,
		lastDepthFills - SellLossIgnoreLastDepths, lastDepthFills - SellLossIgnoreLastDepths + 1, lastDepthFills - 1, lastDepthFills,
	} {
		if depth > 0 && depth <= lastDepthFills && !slices.Contains(depths, depth) {
			depths = append(depths, depth)
		}
	}
	return depths
}

// gridCases are the ladders the grid runs on: the w3s ladder at every gridDepths count, long, inverse,
// futures, impasse, child and flagless, each carrying every state; a child and a flagless ladder stand
// at the two deepest depths alone, and a futures ladder carries no state pending on the quiet decline.
// At the last depth the next depth was refused for funds (gridLadder), and one long ladder stands
// there with no refusal.
func gridCases() []gridCase {
	kinds := []struct {
		guarded bool
		build   func(*aggragates.Trades)
	}{
		{false, func(*aggragates.Trades) {}},
		{false, func(trade *aggragates.Trades) { trade.Strategy.TradeType = aggragates.Futures }},
		{false, func(trade *aggragates.Trades) { trade.Strategy.Params.Impasse = true }},
		{true, func(trade *aggragates.Trades) { trade.ParentID = 7 }},
		{true, func(trade *aggragates.Trades) { trade.Strategy.Params.SmartTakeLoss = false }},
	}
	var cases []gridCase
	for _, depth := range gridDepths() {
		ladders := []aggragates.Trades{gridLadder(true, depth)}
		for _, kind := range kinds {
			if kind.guarded && depth < lastDepthFills-1 {
				continue
			}
			trade := gridLadder(false, depth)
			kind.build(&trade)
			ladders = append(ladders, trade)
		}
		if depth == lastDepthFills {
			ladders = append(ladders, testutil.LadderTrade(false, fills(depth, "21:30:00")...))
		}
		for _, trade := range ladders {
			for _, st := range gridStates() {
				if trade.Strategy.TradeType == aggragates.Futures && slices.Contains(st.pendingOn, FeatureQuietDecline) {
					continue
				}
				cases = append(cases, gridCase{st.write(trade), st})
			}
		}
	}
	return cases
}

// gridLadder is the w3s ladder depth deep on its side, its next depth refused for funds at the last
// depth, where capital protection watches it.
func gridLadder(inverse bool, depth int) aggragates.Trades {
	trade := testutil.LadderTrade(inverse, fills(depth, "21:30:00")...)
	if depth >= lastDepthFills {
		trade = fundsRefused(trade)
	}
	return trade
}

// gridBounds are the shared action's three bounds as a grid runs it: the floor and the depths left,
// each off at zero or below, and the loss the trigger takes (capPct, the trigger negated).
type gridBounds struct {
	minFilled  int
	depthsLeft int
	capPct     float64
}

// oracleTrigger is a trigger as the oracle states it: the gate that files its rows and the features
// they name, its reason, whether its held row is due (the grid writes its held rows under the trigger
// in force, heldStep, so a row held by the profit is due by the same rule), whether it activated the
// stopLoss, whether the depth bounds hold it and the loss it takes (capPct).
type oracleTrigger struct {
	gate      string
	features  []string
	reason    string
	rowDue    bool
	activated bool
	bounded   bool
	capPct    float64
}

// gridLossPct is the loss of a sale at price: a long ladder's worked by hand on what it used
// (handLossPct); an inverse ladder's is the action's own reading on the base its entries sold
// (saleProfit, negated), since the close it simulates rounds the base bought back to the pair's
// step, which a hand count on the w3s prices misses by a hundredth of a percent.
func gridLossPct(trade aggragates.Trades, price float64) float64 {
	if trade.Inverse {
		profitPct, _ := saleProfit(trade, price, 0)
		return 0 - profitPct
	}
	return handLossPct(trade, price)
}

// capitalProtectionGridTrigger is whether capital protection's trigger stands on a grid ladder: its
// switch, a spot ladder outside impasse, long or inverse, at its last depth with its next depth
// refused for funds, the block's trend flag on its side; the grid states hold or activate nothing
// of its own, so its held row is always due.
func capitalProtectionGridTrigger(trade aggragates.Trades, block aggragates.SmartTakeLossIndicators, switches [5]bool) bool {
	flag := block.CapitalProtectionSmcBearish
	if trade.Inverse {
		flag = block.CapitalProtectionSmcBullish
	}
	spotParent := trade.Strategy.TradeType != aggragates.Futures && !trade.Strategy.Params.Impasse
	return switches[1] && spotParent && flag && fundsRefusedLogged(trade) && ladder.CountFilledEntries(trade) >= int(trade.StrategyPair.StrategySettings[0].Depths)
}

// expectedTriggers are the triggers standing on a grid ladder, in the order Apply reads them: the slow
// decline gate's on the declines that watch it (heldDeclines), under the shared bounds and the trigger
// given, then capital protection's (capitalProtectionGridTrigger), unbounded and on its drawdown.
func expectedTriggers(trade aggragates.Trades, st gridState, block aggragates.SmartTakeLossIndicators, switches [5]bool, bounds gridBounds) []oracleTrigger {
	var triggers []oracleTrigger
	if declines := heldDeclines(trade, st, switches); len(declines) > 0 {
		triggers = append(triggers, oracleTrigger{GateSlowDecline, declines, reasonSlowDecline, !st.saleHeld, st.activated, true, bounds.capPct})
	}
	if capitalProtectionGridTrigger(trade, block, switches) {
		triggers = append(triggers, oracleTrigger{gate: GateCapitalProtection, reason: reasonCapitalProtection, rowDue: true, capPct: capitalProtectionMaxDrawdown})
	}
	return triggers
}

// expectedApply is Apply stated on its own for the grid's ladders, whose blocks serve no verdict,
// quiet leg or series, so no row but the sophos latch, capital protection's triggered row and the
// shared action's comes back. Past the guards an unlatched ladder the early take profit watches
// gets its latch on the indecision reading; past the closes (a take profit proposed in buy goes on,
// gridYields) capital protection's trigger writes its triggered row, and the action decides the
// standing triggers by the state (expectedAction). The
// switches are, in order, quiet, capital protection, indecision, swing, the linear decline's latch.
func expectedApply(trade aggragates.Trades, st gridState, position string, price float64, block aggragates.SmartTakeLossIndicators, switches [5]bool, bounds gridBounds) Result {
	want := Result{Position: position}
	if !trade.Strategy.Params.SmartTakeLoss || trade.ParentID != 0 || price <= 0 || len(trade.StrategyPair.StrategySettings) == 0 {
		return want
	}
	filled := ladder.CountFilledEntries(trade)
	spot := !trade.Inverse && trade.Strategy.TradeType != aggragates.Futures
	indecisionWatches := switches[2] && spot && filled >= earlytakeprofit.ArmDepth
	if indecisionWatches && block.Indecision && !st.latched {
		want.Rows = []Row{LatchedRow(trade.PositionPrice, indecisionFinding(block.QuietDeclineBreakReasons))}
	}
	if gridPassesThrough(position, trade.PositionType) {
		return want
	}
	if capitalProtectionGridTrigger(trade, block, switches) {
		want.Rows = append(want.Rows, CapitalProtectionTriggeredRow(trade.PositionPrice, filled, int(trade.StrategyPair.StrategySettings[0].Depths), trade.Inverse))
	}
	rows, step, reason, _ := expectedAction(trade, st, price, block, switches, bounds)
	if step == stepSell {
		want.Position, want.Reason = "sellLoss", reason
		return want
	}
	want.Rows = append(want.Rows, rows...)
	switch step {
	case stepActivate, stepReanchor:
		want.Position, want.Activated = "stopLoss", true
	case stepHold:
		want.Position = ""
	}
	return want
}

// The branches of the shared action expectedAction names, one per way it answers.
const (
	branchNoTrigger  = "no trigger"
	branchInertState = "inert state"
	branchFloor      = "held by the floor"
	branchDepthBound = "held by the depth bound"
	branchCommitted  = "sold, committed"
	branchReanchored = "re-anchored, committed"
	branchHeldTrail  = "held between the bounds, committed"
	branchLossCap    = "held by the loss cap"
	branchSoldInStop = "sold from stopLoss"
	branchActivated  = "activated"
	// withCapital marks a branch answered with capital protection's trigger live on the tick.
	withCapital = " with capital protection"
)

// committedBranches are the three ways the shared action answers a ladder a gate
// activated, within the depth bound: the trail's sale, re-anchor and hold.
var committedBranches = []string{branchCommitted, branchReanchored, branchHeldTrail}

// expectedAction is the shared action on a grid ladder under the bounds, stated on its own with the
// branch it answers by: the triggers stand per expectedTriggers; buy passes the floor first, both
// candidates the depth bound, each bound refusing the bounded triggers with their due held rows; with
// none live that is the answer, else a ladder any trigger activated trails from its anchor by the one
// settings row, sold for the first live trigger that activated it, and any other is judged on the hand-worked loss (gridLossPct) against each live
// trigger's own loss: none reached is held with the due rows, a self-armed stopLoss is sold under the
// first reached, buy activated with the activation row of each reached trigger. The refusal rows come
// first; a trail and a sale carry none.
func expectedAction(trade aggragates.Trades, st gridState, price float64, block aggragates.SmartTakeLossIndicators, switches [5]bool, bounds gridBounds) (rows []Row, step saleStep, reason, branch string) {
	triggers := expectedTriggers(trade, st, block, switches, bounds)
	if len(triggers) == 0 {
		return nil, stepNone, "", branchNoTrigger
	}
	switch trade.PositionType {
	case "buy":
		step = stepActivate
	case "stopLoss", "forceTrailingStopLoss":
		step = stepSell
	default:
		return nil, stepNone, "", branchInertState
	}
	filled, fill := ladder.CountFilledEntries(trade), trade.PositionPrice
	ceiling := int(trade.StrategyPair.StrategySettings[0].Depths)
	why, refused := "", ""
	switch {
	case step == stepActivate && bounds.minFilled > 0 && filled < bounds.minFilled:
		why, refused = fmt.Sprintf("depth %d under the %d depths the activation needs", filled, bounds.minFilled), branchFloor
	case bounds.depthsLeft > 0 && filled > ceiling-bounds.depthsLeft:
		why, refused = atDepthText(filled, ceiling, ceiling-bounds.depthsLeft), branchDepthBound
	}
	var live []oracleTrigger
	for _, trigger := range triggers {
		if refused != "" && trigger.bounded {
			if trigger.rowDue {
				rows = append(rows, heldRowOf(saleTrigger{gate: trigger.gate, features: trigger.features}, fill, why))
			}
			continue
		}
		live = append(live, trigger)
	}
	if len(live) == 0 {
		return rows, stepNone, "", refused
	}
	if step == stepSell && slices.ContainsFunc(triggers, func(trigger oracleTrigger) bool { return trigger.activated }) {
		settings, move := trade.StrategyPair.StrategySettings[0], moveAgainst(price, trade.PositionPrice)
		if trade.Inverse {
			move = -move
		}
		switch {
		case move < -settings.Tolerance:
			reason = live[0].reason
			if index := slices.IndexFunc(live, func(trigger oracleTrigger) bool { return trigger.activated }); index >= 0 {
				reason = live[index].reason
			}
			return nil, stepSell, reason, branchCommitted
		case move > settings.TrailingTakeProfit:
			return nil, stepReanchor, "", branchReanchored
		}
		return nil, stepHold, "", branchHeldTrail
	}
	loss := gridLossPct(trade, price)
	var hits, misses []oracleTrigger
	for _, trigger := range live {
		if loss > trigger.capPct {
			misses = append(misses, trigger)
		} else {
			hits = append(hits, trigger)
		}
	}
	if len(hits) == 0 {
		for _, trigger := range misses {
			if trigger.rowDue {
				rows = append(rows, heldByLossRowOf(saleTrigger{gate: trigger.gate, features: trigger.features}, fill, loss, trigger.capPct))
			}
		}
		return rows, stepNone, "", branchLossCap
	}
	if step == stepSell {
		return nil, stepSell, hits[0].reason, branchSoldInStop
	}
	for _, trigger := range hits {
		rows = append(rows, activationRowOf(saleTrigger{gate: trigger.gate, features: trigger.features}, fill, activatedText(loss, trigger.capPct)))
	}
	return rows, stepActivate, "", branchActivated
}

// heldDeclines are the declines a state is pending on that watch the ladder, in row order: the quiet
// decline from quietdecline.ArmDepth on a long ladder, the swing decline from swingdecline.ArmDepth on a long spot one.
func heldDeclines(trade aggragates.Trades, st gridState, switches [5]bool) []string {
	filled := ladder.CountFilledEntries(trade)
	spot := !trade.Inverse && trade.Strategy.TradeType != aggragates.Futures
	var declines []string
	if slices.Contains(st.pendingOn, FeatureQuietDecline) && switches[0] && !trade.Inverse && filled >= quietdecline.ArmDepth {
		declines = append(declines, FeatureQuietDecline)
	}
	if slices.Contains(st.pendingOn, FeatureSwingDecline) && switches[3] && spot && filled >= swingdecline.ArmDepth {
		declines = append(declines, FeatureSwingDecline)
	}
	return declines
}

// gridOutcome names what the grid reached on one answer: the action's branch, marked withCapital
// where capital protection's trigger stood on the tick.
func gridOutcome(trade aggragates.Trades, st gridState, position string, price float64, block aggragates.SmartTakeLossIndicators, switches [5]bool, bounds gridBounds) string {
	if gridPassesThrough(position, trade.PositionType) {
		return ""
	}
	_, _, _, branch := expectedAction(trade, st, price, block, switches, bounds)
	if capitalProtectionGridTrigger(trade, block, switches) {
		return branch + withCapital
	}
	return branch
}

// gridBlocks serve the trend bearish, bullish, both, the indecision reading with and without the
// bearish trend, and nothing, each with the Trend Direction read on every row (withEveryRowTrend,
// the trend refusal being read apart); none serves the verdict or a window.
func gridBlocks() []aggragates.SmartTakeLossIndicators {
	blocks := []aggragates.SmartTakeLossIndicators{
		{CapitalProtectionSmcBearish: true},
		{CapitalProtectionSmcBullish: true},
		{CapitalProtectionSmcBearish: true, CapitalProtectionSmcBullish: true},
		{CapitalProtectionSmcBearish: true, Indecision: true, QuietDeclineBreakReasons: indecisionReasons},
		{Indecision: true},
		{},
	}
	for index := range blocks {
		blocks[index] = withEveryRowTrend(blocks[index])
	}
	return blocks
}

// gridHighPrice sits over the deepest ladders' break even and under the shallowest'; gridGainPrice
// over every ladder's break even (a gain, no activation); gridHoldPrice a fifth of a percent over
// the third fill, inside both bounds of an activated ladder anchored there.
const (
	gridHighPrice = 186.0
	gridGainPrice = 210.0
	gridHoldPrice = 189.0
)

// gridPrices sit under, at and over lastDepthPrint, at and either side of the last-depth ladder's
// drawdown, at gridHighPrice and over it, at gridHoldPrice and at gridGainPrice.
func gridPrices() []float64 {
	last := lastDepthLadder()
	prices := []float64{math.Nextafter(lastDepthPrint, 0), lastDepthPrint, (lastDepthPrint + gridHighPrice) / 2, gridHighPrice, gridHighPrice + 1, gridHoldPrice, gridGainPrice}
	for cents := int(math.Round(last.PositionPrice * 100)); float64(cents)/100 <= ladder.AverageEntryPrice(last); cents++ {
		if handLossPct(last, float64(cents)/100) <= capitalProtectionMaxDrawdown {
			prices = append(prices, float64(cents-1)/100, float64(cents)/100)
			break
		}
	}
	return prices
}

// gridSwitches are the thirty-two combinations of the five switches, in the order they are read.
func gridSwitches() [][5]bool {
	var combinations [][5]bool
	for bits := 31; bits >= 0; bits-- {
		combinations = append(combinations, [5]bool{bits&16 != 0, bits&8 != 0, bits&4 != 0, bits&2 != 0, bits&1 != 0})
	}
	return combinations
}

// setGridSwitches runs the calling test with the five switches as given.
func setGridSwitches(t *testing.T, switches [5]bool) {
	t.Helper()
	withQuietDecline(t, switches[0])
	withCapitalProtectionExit(t, switches[1])
	withEarlyTakeProfit(t, switches[2])
	withSwingDecline(t, switches[3])
	withLinearDecline(t, false)
	withEarlyTakeProfitLinearDecline(t, switches[4])
}

// Every grid ladder, state, proposal, block and price, under every switch combination, no depth
// bound on and the trigger taking any loss: Apply answers expectedApply exactly, and the grid reaches
// every answer (activated, sold, trailed, and each with capital protection's trigger live: held by
// its drawdown, activated, sold from stopLoss), a take profit in buy both replaced and standing.
func TestApplyOnTheGrid(t *testing.T) {
	bounds := gridBounds{capPct: anyLoss}
	withAllowedLoss(t, bounds.capPct)
	withSellLossIgnoreLastDepths(t, bounds.depthsLeft)
	withSellLossStartAtDepth(t, bounds.minFilled)
	allPositions := []string{"", "buy", "stopLoss", "update_stopLoss", "update_buy", "forceTrailingStopLoss", "takeProfit", "forceTrailingTakeProfit"}
	allStates := []string{"new", "buy", "stopLoss", "forceTrailingStopLoss", "takeProfit", "sellLoss"}
	blocks, prices := gridBlocks(), gridPrices()
	seen := map[string]int{}
	for _, switches := range gridSwitches() {
		setGridSwitches(t, switches)
		// With the linear decline on the blocks serve no series, so it moves no answer; a smaller grid shows it.
		positions, states := allPositions, allStates
		if switches[4] {
			positions, states = []string{"", "stopLoss", "takeProfit"}, []string{"buy", "stopLoss", "sellLoss"}
		}
		for _, c := range gridCases() {
			trade := c.trade
			for _, state := range states {
				trade.PositionType = state
				for _, position := range positions {
					for _, block := range blocks {
						for _, price := range prices {
							got := Apply(trade, position, price, withBlock(block))
							want := expectedApply(trade, c.gateState, position, price, block, switches, bounds)
							if !reflect.DeepEqual(got, want) {
								t.Fatalf("switches %v, gate state %q, %d fills (inverse %v, state %q, events %d), %q at %v, block %+v:\ngot  %+v\nwant %+v",
									switches, c.gateState.name, len(trade.History), trade.Inverse, state, len(trade.StrategyEvents), position, price, block, got, want)
							}
							seen[gridOutcome(trade, c.gateState, position, price, block, switches, bounds)]++
							seen["sold: "+got.Reason]++
							switch {
							case gridYields(position, state) && got.Activated:
								seen["take profit stopLoss"]++
							case gridYields(position, state):
								seen["take profit takeProfit"]++
							}
							for _, row := range got.Rows {
								seen["row: "+row.Event]++
							}
						}
					}
				}
			}
		}
	}
	for _, key := range []string{
		branchNoTrigger, branchInertState, branchActivated, branchSoldInStop, branchCommitted, branchReanchored, branchHeldTrail,
		branchActivated + withCapital, branchSoldInStop + withCapital, branchLossCap + withCapital, branchInertState + withCapital, branchReanchored + withCapital,
		"sold: " + reasonSlowDecline, "sold: " + reasonCapitalProtection, "sold: ", "row: " + EventLatched, "row: " + EventActivated, "row: " + EventTriggered, "row: " + EventSaleHeld,
		"take profit stopLoss", "take profit takeProfit",
	} {
		if seen[key] == 0 {
			t.Fatalf("fixture drifted: the grid never reached %q, got %v", key, seen)
		}
	}
	for _, key := range []string{branchFloor, branchDepthBound, branchLossCap} {
		if seen[key] != 0 {
			t.Fatalf("fixture drifted: with no bound on the grid reached %q, got %v", key, seen)
		}
	}
	t.Logf("answers by outcome: %v", seen)
}

// runBoundedGrid is the grid under the shipped floor, depth bound or cap, or any together: a refused
// pending ladder is held for the first bound that fails (floor, then depth bound, then cap) with that
// bound's row once per episode, one within every bound goes ahead, capital protection's trigger is
// judged on its own beside a refusal, and every other answer is the unbounded grid's. It skips unless
// the grid reaches each key given.
func runBoundedGrid(t *testing.T, bounds gridBounds, keys []string) {
	t.Helper()
	withAllowedLoss(t, bounds.capPct)
	withSellLossIgnoreLastDepths(t, bounds.depthsLeft)
	withSellLossStartAtDepth(t, bounds.minFilled)
	positions := []string{"", "buy", "stopLoss", "update_stopLoss", "forceTrailingStopLoss", "takeProfit"}
	states := []string{"buy", "stopLoss", "forceTrailingStopLoss", "sellLoss"}
	prices := gridPrices()
	seen := map[string]int{}
	for _, switches := range [][5]bool{{true, true, true, true, true}, {true, false, true, true, false}, {true, true, false, false, true}, {false, true, true, true, false}, {false, false, false, true, true}} {
		setGridSwitches(t, switches)
		for _, c := range gridCases() {
			trade := c.trade
			for _, state := range states {
				trade.PositionType = state
				for _, position := range positions {
					for _, block := range gridBlocks() {
						for _, price := range prices {
							got := Apply(trade, position, price, withBlock(block))
							want := expectedApply(trade, c.gateState, position, price, block, switches, bounds)
							if !reflect.DeepEqual(got, want) {
								t.Fatalf("bounds %+v, switches %v, gate state %q, %d fills (inverse %v, state %q, events %d), %q at %v, block %+v:\ngot  %+v\nwant %+v",
									bounds, switches, c.gateState.name, len(trade.History), trade.Inverse, state, len(trade.StrategyEvents), position, price, block, got, want)
							}
							seen[gridOutcome(trade, c.gateState, position, price, block, switches, bounds)]++
						}
					}
				}
			}
		}
	}
	t.Logf("answers under the bounds %+v: %v", bounds, seen)
	for _, key := range keys {
		if seen[key] == 0 {
			t.Skipf("the grid reaches no %q under the bounds %+v, so SellLossTriggerPercentage or SellLossIgnoreLastDepths leaves nothing of it to measure, got %v", key, bounds, seen)
		}
	}
}

// The grid under the shipped cap alone: a pending ladder is held where its sale loses more than the
// cap, a committed one excepted (it trails), and goes within it; capital protection activates on its
// own drawdown while the cap holds the slow decline.
func TestApplyOnTheGridUnderTheCap(t *testing.T) {
	if shippedLoss <= 0 {
		t.Skip("SellLossTriggerPercentage takes no loss: the grid has no action to hold for its loss")
	}
	runBoundedGrid(t, gridBounds{capPct: shippedLoss}, slices.Concat([]string{branchLossCap, branchSoldInStop, branchActivated, branchLossCap + withCapital, branchActivated + withCapital}, committedBranches))
}

// The grid under the shipped depth bound alone: a pending ladder past the bound is held at whatever
// loss, a committed one too, and goes within it; at the last depth capital protection acts beside the refusal.
func TestApplyOnTheGridUnderTheDepthBound(t *testing.T) {
	if SellLossIgnoreLastDepths <= 0 || lastDepthFills-SellLossIgnoreLastDepths < 1 {
		t.Skip("SellLossIgnoreLastDepths is off or leaves no depth the action is made at")
	}
	runBoundedGrid(t, gridBounds{depthsLeft: SellLossIgnoreLastDepths, capPct: anyLoss}, slices.Concat([]string{branchDepthBound, branchSoldInStop, branchActivated, branchActivated + withCapital, branchLossCap + withCapital}, committedBranches))
}

// The grid under both bounds: the depth bound is read first, so a ladder past it is held for its depth, within it for its loss.
func TestApplyOnTheGridUnderBothBounds(t *testing.T) {
	if shippedLoss <= 0 || SellLossIgnoreLastDepths <= 0 || lastDepthFills-SellLossIgnoreLastDepths < 1 {
		t.Skip("SellLossTriggerPercentage takes no loss, or SellLossIgnoreLastDepths is off or leaves no depth the action is made at")
	}
	runBoundedGrid(t, gridBounds{depthsLeft: SellLossIgnoreLastDepths, capPct: shippedLoss}, slices.Concat([]string{branchLossCap, branchDepthBound, branchSoldInStop, branchActivated, branchActivated + withCapital}, committedBranches))
}

// The grid under the shipped floor alone: a ladder in buy under the floor is held at whatever loss with
// the floor's row, at the floor it goes ahead; stopLoss is not asked the floor.
func TestApplyOnTheGridUnderTheFloor(t *testing.T) {
	if SellLossStartAtDepth <= 0 {
		t.Skip("SellLossStartAtDepth is off: the grid has no activation to hold for its depth")
	}
	runBoundedGrid(t, gridBounds{minFilled: SellLossStartAtDepth, capPct: anyLoss}, slices.Concat([]string{branchFloor, branchActivated, branchSoldInStop}, committedBranches))
}

// The grid under the three bounds: floor, then depth bound, then cap, so a ladder under the floor is
// held for the floor although over the cap; it asks no activation of the grid.
func TestApplyOnTheGridUnderEveryBound(t *testing.T) {
	if SellLossStartAtDepth <= 0 || shippedLoss <= 0 || SellLossIgnoreLastDepths <= 0 || lastDepthFills-SellLossIgnoreLastDepths < SellLossStartAtDepth {
		t.Skip("SellLossStartAtDepth, SellLossTriggerPercentage takes no loss, or SellLossIgnoreLastDepths is off or leaves no depth the activation is made at")
	}
	bounds := gridBounds{minFilled: SellLossStartAtDepth, depthsLeft: SellLossIgnoreLastDepths, capPct: shippedLoss}
	runBoundedGrid(t, bounds, slices.Concat([]string{branchFloor, branchDepthBound, branchLossCap, branchSoldInStop, branchActivated + withCapital}, committedBranches))
}

// The line's latch on the states the gate's ladders carry: the long spot ladder on one line
// (linearScene) and its variants (inverse, futures, impasse, child, flagless), every state, every
// switch combination, the block serving the line and a trend that meets the need, at a print over
// break even with a buy order resting, so no action runs. The latch is due exactly where the early take profit watches the
// ladder, its latch on the line is on and no state latched it: then Apply hands back the latched row
// alone and the fold reads the ladder latched; otherwise nothing, the proposal untouched.
func TestApplyOnTheLinearGrid(t *testing.T) {
	withSwingMemo(t, newSwingDeclineMemo(4, func([]series.Fill, series.Series, series.Trend) (swingdecline.Reading, bool) {
		return swingdecline.Reading{}, false
	}))
	long, block := linearScene(linearTrendMet)
	inverse := testutil.LadderTrade(true, linearFills(linearSlope, linearDepth)...)
	futures, impasse, child, flagless := long, long, long, long
	futures.Strategy.TradeType = aggragates.Futures
	impasse.Strategy.Params.Impasse = true
	child.ParentID = 7
	flagless.Strategy.Params.SmartTakeLoss = false
	kinds := []struct {
		name      string
		trade     aggragates.Trades
		latchable bool
	}{
		{"a long ladder", long, true}, {"an inverse ladder", inverse, false}, {"a futures ladder", futures, false},
		{"an impasse strategy", impasse, true}, {"a child", child, false}, {"a strategy without the flag", flagless, false},
	}

	row, latched := linearRow(t, long, block), 0
	gain := ladder.AverageEntryPrice(long) + 1
	for _, switches := range gridSwitches() {
		setGridSwitches(t, switches)
		for _, kind := range kinds {
			for _, gate := range gridStates() {
				trade := gate.write(kind.trade)
				got, want := Apply(withRestingOrder(trade), "", gain, withBlock(block)), Result{}
				due := kind.latchable && switches[2] && switches[4] && !gate.latched
				if due {
					want.Rows = []Row{row}
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("switches %v, %s, gate state %q: latch due %v\ngot  %+v\nwant %+v", switches, kind.name, gate.name, due, got, want)
				}
				if !due {
					continue
				}
				latched++
				before, after := rebuildState(trade), rebuildState(withRow(trade, row, testutil.At("23:00:00")))
				if !after.earlyTakeProfit || after.slowDeclinePending != before.slowDeclinePending || after.slowDeclinePendingFrom != before.slowDeclinePendingFrom || !slices.Equal(after.pendingFeatures, before.pendingFeatures) ||
					after.saleHeld != before.saleHeld || after.slowDeclineActivated != before.slowDeclineActivated {
					t.Fatalf("switches %v, %s, gate state %q: the latch must leave the pending state and the action's memory as it was, before %+v, after %+v", switches, kind.name, gate.name, before, after)
				}
			}
		}
	}
	if latched == 0 {
		t.Fatal("fixture drifted: the grid must reach the line's latch")
	}
	t.Logf("latches by the line: %d", latched)
}
