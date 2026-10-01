package smarttakeloss

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// sampledLadder draws a ladder from none to its last depth, long or inverse,
// with at most one kind that changes what the rules watch: futures, impasse,
// a child, no flag, no settings, an unknown or lowered depth count, or a
// partial fill of the newest depth.
func sampledLadder(r *rand.Rand) aggragates.Trades {
	depths := []int{0, 1, SlowDeclineArmDepth - 1, SlowDeclineArmDepth, IndecisionArmDepth - 1, IndecisionArmDepth, lastDepthFills - 1, lastDepthFills}
	var ladderFills []testutil.LadderFill
	if n := depths[r.IntN(len(depths))]; n > 0 {
		ladderFills = fills(n, "21:30:00")
	}
	trade := testutil.LadderTrade(r.IntN(6) == 0, ladderFills...)
	switch r.IntN(14) {
	case 0:
		trade.Strategy.TradeType = aggragates.Futures
	case 1:
		trade.Strategy.Params.Impasse = true
	case 2:
		trade.ParentID = 7
	case 3:
		trade.Strategy.Params.SmartTakeLoss = false
	case 4:
		trade.StrategyPair.StrategySettings = nil
	case 5:
		trade.StrategyPair.StrategySettings[0].Depths = 0
	case 6:
		trade.StrategyPair.StrategySettings[0].Depths = lastDepthFills - 1
	case 7:
		if n := len(trade.History); n > 0 {
			trade.History = append(trade.History, trade.History[n-1])
			trade.History[n].Quantity /= 2
		}
	}
	return trade
}

// sampledState draws the trade's slow-decline and indecision state, ten kinds
// with one draw: none, a marker at the newest fill or at the fill before it
// (the newest not judged yet), a marker a cancel took back, a cancel a marker
// followed, the retired rule's text rows before a marker, a marker's text
// without its event, an indecision row, one beside a marker, or an
// indecision text without its event. The rows are written the way the engines
// write them, each with its event; a row of text alone carries no event, and
// so no state.
func sampledState(r *rand.Rand, trade aggragates.Trades) aggragates.Trades {
	entries := entryFills(trade)
	newest, before := trade.PositionPrice, trade.PositionPrice
	if len(entries) >= 2 {
		before = entries[len(entries)-2].Price
	}
	marker := PendingRow("buy", newest, nil)
	cancel := CancelledRow("buy", newest, nil)
	earlier := PendingRow("buy", before, nil)
	latch := LatchedRow("buy", newest, nil)

	trade.Logs, trade.StrategyEvents = nil, nil
	switch r.IntN(10) {
	case 1:
		return withRows(trade, marker)
	case 2:
		return withRows(trade, earlier)
	case 3:
		return withRows(trade, marker, cancel)
	case 4:
		return withRows(trade, cancel, marker)
	case 5:
		trade.Logs = retiredRows(newest)
		return withRows(trade, marker)
	case 6:
		trade.Logs = []aggragates.TradesLogs{{Message: marker.Message}}
	case 7:
		return withRows(trade, latch)
	case 8:
		return withRows(trade, marker, latch)
	case 9:
		trade.Logs = []aggragates.TradesLogs{{Message: latch.Message}}
	}
	return trade
}

// sampledBlock draws a reading: the verdict, the leg on and quiet with its
// bars around the newest fill's stamp or none, a recent bar the verdict stood
// on or none with the look-back's oldest bar before or after that stamp or
// none, the fill window from before or after that stamp or none, each band
// zero, no price, or at one of the two fixture bands, the indecision
// reading, and the SMC trend condition.
func sampledBlock(r *rand.Rand) aggragates.SmartTakeLossIndicators {
	stamp := testutil.At("21:30:00")
	bars := []int64{0, stamp.Add(-time.Hour).UnixMilli(), stamp.Add(time.Hour).UnixMilli()}
	bands := []float64{0, -capitalProtectionBand, capitalProtectionBand, slowDeclineBand}
	return aggragates.SmartTakeLossIndicators{
		SlowDeclineExit:             r.IntN(2) == 0,
		SlowDeclineLegQuiet:         r.IntN(2) == 0,
		SlowDeclineSmoothFrom:       bars[r.IntN(len(bars))],
		SlowDeclineFillBefore:       bars[r.IntN(len(bars))],
		SlowDeclineSellBand:         bands[r.IntN(len(bands))],
		SlowDeclineExitReasons:      slowDeclineReasons,
		SlowDeclineBreakReasons:     slowDeclineBreakReasons,
		SlowDeclineIndecision:       r.IntN(2) == 0,
		SlowDeclineRecentAt:         bars[r.IntN(len(bars))],
		SlowDeclineRecentFrom:       bars[r.IntN(len(bars))],
		SlowDeclineRecentReasons:    slowDeclineRecentReasons,
		SlowDeclineFillFrom:         bars[r.IntN(len(bars))],
		CapitalProtectionUpperBB:    bands[r.IntN(len(bands))],
		CapitalProtectionSmcBearish: r.IntN(2) == 0,
	}
}

// capitalProtectionHolds is the rule stated on the ladder's own count: the
// switch, a long spot parent under the flag outside impasse, its last
// configured depth filled, the SMC trend bearish and the price at or over a
// band above zero.
func capitalProtectionHolds(trade aggragates.Trades, price float64, block aggragates.SmartTakeLossIndicators) bool {
	depths := ladder.ConfiguredDepths(trade)
	return capitalProtectionExit && trade.Strategy.Params.SmartTakeLoss && trade.ParentID == 0 && !trade.Inverse &&
		trade.Strategy.TradeType != aggragates.Futures && !trade.Strategy.Params.Impasse &&
		depths > 0 && ladder.CountFilledEntries(trade) >= depths &&
		block.CapitalProtectionSmcBearish && block.CapitalProtectionUpperBB > 0 && price >= block.CapitalProtectionUpperBB
}

// The last depth is the ladder's own: ladder.ConfiguredDepths — the Depths of
// the settings row the next fill would use, the base row past the table,
// floored — against ladder.CountFilledEntries, which counts a legacy row
// without an order id as a depth of its own and an accounting row as none.
// capitalProtectionWatched, rebuildState's watch, Armed with the slow decline,
// the indecision direction and the slow pattern decline switched off — the
// pattern watches every long spot ladder from SlowPatternArmDepth fills — and
// Apply's sale at the band all read it so.
func TestCapitalProtectionWatchIsTheLaddersOwnLastDepth(t *testing.T) {
	withCapitalProtectionExit(t, true)
	withQuietSlowDeclineExit(t, false)
	withIndecisionDirection(t, false)
	withSlowPatternDeclineExit(t, false)
	ladderOf := func(depths int) aggragates.Trades {
		return testutil.LadderTrade(false, fills(depths, "21:30:00")...)
	}
	fractional, fractionalShort := ladderOf(lastDepthFills-1), ladderOf(lastDepthFills-2)
	fractional.StrategyPair.StrategySettings[0].Depths = lastDepthFills - 0.5
	fractionalShort.StrategyPair.StrategySettings[0].Depths = lastDepthFills - 0.5
	legacy := lastDepthLadder()
	for index := range legacy.History {
		legacy.History[index].OrderId = 0
	}
	accounting := ladderOf(lastDepthFills - 1)
	accounting.History = append(accounting.History, aggragates.TradesHistory{
		Type: "BUY", Quantity: 1, Price: ladder.AccountingPriceCeiling / 10, OrderId: lastDepthFills,
	})
	// Rows deeper than the ladder everywhere but on the row the next fill
	// reads: past the table that is the base row.
	nextRow, baseRow := lastDepthLadder(), lastDepthLadder()
	row := nextRow.StrategyPair.StrategySettings[0]
	deeper := row
	deeper.Depths = lastDepthFills + 2
	nextRow.StrategyPair.StrategySettings = make([]aggragates.StrategySettings, lastDepthFills+2)
	for index := range nextRow.StrategyPair.StrategySettings {
		nextRow.StrategyPair.StrategySettings[index] = deeper
	}
	nextRow.StrategyPair.StrategySettings[lastDepthFills] = row
	baseRow.StrategyPair.StrategySettings = []aggragates.StrategySettings{row, deeper, deeper}

	for name, tc := range map[string]struct {
		trade aggragates.Trades
		want  bool
	}{
		"a fractional Depths floored, at its last depth": {fractional, true},
		"a fractional Depths floored, one depth short":   {fractionalShort, false},
		"legacy rows without an order id":                {legacy, true},
		"an accounting row one depth short":              {accounting, false},
		"the row of the next fill":                       {nextRow, true},
		"the base row past the table":                    {baseRow, true},
	} {
		if own := capitalProtectionHolds(tc.trade, capitalProtectionBand, solBlock()); own != tc.want {
			t.Fatalf("%s: fixture drifted, the ladder's own count reads %v", name, own)
		}
		watched, state, armed := capitalProtectionWatched(tc.trade), rebuildState(tc.trade).capitalProtectionWatched, Armed(tc.trade)
		sold := Apply(tc.trade, "", capitalProtectionBand, withBlock(solBlock())).Reason == reasonCapitalProtection
		if watched != tc.want || state != tc.want || armed != tc.want || sold != tc.want {
			t.Errorf("%s: watched %v, rebuildState %v, armed %v, sold %v; want %v", name, watched, state, armed, sold, tc.want)
		}
	}
}
