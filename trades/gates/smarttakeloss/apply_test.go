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
// SlowDeclineArmDepth, of IndecisionArmDepth and of its last depth, a ladder
// with no fill, an inverse ladder. The block serves every reading at once.
func TestApplyInertWithoutTheFlagOnAChildOrWithoutInputs(t *testing.T) {
	reading := solBlock()
	reading.SlowDeclineExit = true
	reading.SlowDeclineSellBand = capitalProtectionBand
	reading.SlowDeclineIndecision = true
	reading.SlowDeclineBreakReasons = indecisionReasons
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
		"short of every watch": {testutil.LadderTrade(false, fills(min(SlowDeclineArmDepth, IndecisionArmDepth)-1, "17:38:00")...), capitalProtectionBand},
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

// gridDepths are the fill counts of the grid's ladders: one short of each
// rule's watch and at it — SlowDeclineArmDepth, IndecisionArmDepth and the
// last depth — each once.
func gridDepths() []int {
	var depths []int
	for _, depth := range []int{SlowDeclineArmDepth - 1, SlowDeclineArmDepth, IndecisionArmDepth - 1, IndecisionArmDepth, lastDepthFills - 1, lastDepthFills} {
		if depth > 0 && !slices.Contains(depths, depth) {
			depths = append(depths, depth)
		}
	}
	return depths
}

// gridTrades are the ladders the grid runs on: the w3s ladder at every
// gridDepths count — long, inverse, futures, under an impasse strategy, a
// child and without the flag — each with no event, pending from its newest
// fill, latched at it by the indecision direction, and both.
func gridTrades() []aggragates.Trades {
	kinds := []func(*aggragates.Trades){
		func(*aggragates.Trades) {},
		func(trade *aggragates.Trades) { trade.Strategy.TradeType = aggragates.Futures },
		func(trade *aggragates.Trades) { trade.Strategy.Params.Impasse = true },
		func(trade *aggragates.Trades) { trade.ParentID = 7 },
		func(trade *aggragates.Trades) { trade.Strategy.Params.SmartTakeLoss = false },
	}
	var trades []aggragates.Trades
	for _, depth := range gridDepths() {
		ladders := []aggragates.Trades{testutil.LadderTrade(true, fills(depth, "21:30:00")...)}
		for _, kind := range kinds {
			trade := testutil.LadderTrade(false, fills(depth, "21:30:00")...)
			kind(&trade)
			ladders = append(ladders, trade)
		}
		for _, trade := range ladders {
			marker := PendingRow("buy", trade.PositionPrice, nil)
			latch := LatchedRow("buy", trade.PositionPrice, nil)
			trades = append(trades, trade, withRows(trade, marker), withRows(trade, latch), withRows(trade, marker, latch))
		}
	}
	return trades
}

// expectedApply is Apply stated on its own for the grid's ladders, whose
// events are a pending and a latched event at the newest fill and whose
// blocks serve no verdict and no quiet leg, so no slow-decline row ever comes
// back.
// Past the guards, a ladder the indecision direction watches — a long spot
// one from IndecisionArmDepth fills — that carries no indecision row gets
// its row on a block serving the indecision reading, whatever the proposal
// and the trade's state. Past the closes the ladder or the trade already
// decided, a pending ladder the quiet slow decline watches sells at its sell
// band, and else a ladder at its last depth capital protection watches sells
// at the upper band while the SMC trend reads bearish.
func expectedApply(trade aggragates.Trades, position string, price float64, block aggragates.SmartTakeLossIndicators, slowDecline, capitalProtection, indecision bool) Result {
	want := Result{Position: position}
	if !trade.Strategy.Params.SmartTakeLoss || trade.ParentID != 0 || price <= 0 || len(trade.StrategyPair.StrategySettings) == 0 {
		return want
	}
	filled := ladder.CountFilledEntries(trade)
	watched := indecision && !trade.Inverse && trade.Strategy.TradeType != aggragates.Futures && filled >= IndecisionArmDepth
	if watched && block.SlowDeclineIndecision && !gridHas(trade, GateIndecision, EventLatched) {
		row := LatchedRow(trade.PositionType, trade.PositionPrice, block.SlowDeclineBreakReasons)
		want.Indecision = &row
	}
	if gridCloses[position] || gridCloses[trade.PositionType] {
		return want
	}
	pending := slowDecline && !trade.Inverse && filled >= SlowDeclineArmDepth && gridHas(trade, GateSlowDecline, EventPending)
	lastDepth := capitalProtection && !trade.Inverse && trade.Strategy.TradeType != aggragates.Futures &&
		!trade.Strategy.Params.Impasse && filled >= int(trade.StrategyPair.StrategySettings[0].Depths)
	switch {
	case pending && block.SlowDeclineSellBand > 0 && price >= block.SlowDeclineSellBand:
		want.Position, want.Reason = "sellLoss", reasonSellBand
	case lastDepth && block.CapitalProtectionSmcBearish && block.CapitalProtectionUpperBB > 0 && price >= block.CapitalProtectionUpperBB:
		want.Position, want.Reason = "sellLoss", reasonCapitalProtection
	}
	return want
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

// Every grid ladder, state, proposal, block and price, under the three
// switches in every position: Apply answers expectedApply exactly.
func TestApplyOnTheGrid(t *testing.T) {
	positions := []string{"", "buy", "stopLoss", "update_stopLoss", "update_buy", "forceTrailingStopLoss", "takeProfit", "forceTrailingTakeProfit"}
	states := []string{"buy", "stopLoss", "takeProfit", "sellLoss"}
	blocks, prices := gridBlocks(), gridPrices()
	sold := map[string]int{}
	rows := 0
	for _, switches := range [][3]bool{{true, true, true}, {true, false, true}, {false, true, true}, {false, false, true}, {true, true, false}, {true, false, false}, {false, true, false}, {false, false, false}} {
		withQuietSlowDeclineExit(t, switches[0])
		withCapitalProtectionExit(t, switches[1])
		withIndecisionDirection(t, switches[2])
		for _, trade := range gridTrades() {
			for _, state := range states {
				trade.PositionType = state
				for _, position := range positions {
					for _, block := range blocks {
						for _, price := range prices {
							got := Apply(trade, position, price, withBlock(block))
							want := expectedApply(trade, position, price, block, switches[0], switches[1], switches[2])
							if !reflect.DeepEqual(got, want) {
								t.Fatalf("switches %v, %d fills (inverse %v, state %q, events %d), %q at %v, block %+v:\ngot  %+v\nwant %+v",
									switches, len(trade.History), trade.Inverse, state, len(trade.StrategyEvents), position, price, block, got, want)
							}
							sold[got.Reason]++
							if got.Indecision != nil {
								rows++
							}
						}
					}
				}
			}
		}
	}
	if sold[reasonSellBand] == 0 || sold[reasonCapitalProtection] == 0 || sold[""] == 0 || rows == 0 {
		t.Fatalf("fixture drifted: the grid must sell under both reasons, keep proposals and hand back indecision rows, got %v and %d rows", sold, rows)
	}
	t.Logf("answers by reason: %v, indecision rows: %d", sold, rows)
}
