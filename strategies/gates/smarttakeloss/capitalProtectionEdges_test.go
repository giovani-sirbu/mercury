package smarttakeloss

import (
	"math/rand/v2"
	"slices"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/earlytakeprofit"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/quietdecline"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/swingdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// sampledLadder draws a ladder from none to its last depth, long or inverse,
// with at most one kind that changes what the rules watch: futures, impasse,
// a child, no flag, no settings, an unknown or lowered depth count, or a
// partial fill of the newest depth; its next depth refused for funds at the
// newest fill three draws in four, else before it or never.
func sampledLadder(r *rand.Rand) aggragates.Trades {
	depths := []int{0, 1, quietdecline.ArmDepth - 1, quietdecline.ArmDepth, earlytakeprofit.ArmDepth - 1, earlytakeprofit.ArmDepth, swingdecline.ArmDepth - 1, swingdecline.ArmDepth, lastDepthFills - 1, lastDepthFills}
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
	switch r.IntN(8) {
	case 0:
	case 1:
		trade = fundsRefused(trade)
		trade.Logs[len(trade.Logs)-1].CreatedAt = trade.Logs[len(trade.Logs)-1].CreatedAt.Add(-time.Minute)
	default:
		trade = fundsRefused(trade)
	}
	return trade
}

// sampledState is sampledGateState on the ladder, the funds refusal sampledLadder drew kept ahead
// of the gate's rows, where the engine writes it.
func sampledState(r *rand.Rand, trade aggragates.Trades) aggragates.Trades {
	refusal := slices.Clone(trade.Logs)
	trade = sampledGateState(r, trade)
	trade.Logs = append(refusal, trade.Logs...)
	return trade
}

// sampledGateState draws the trade's slow decline gate state, fourteen kinds with
// one draw: none, a pending row on the quiet decline at the newest fill or at the
// fill before it (the newest not judged yet), one a cancel took back, a cancel
// a pending row followed, the retired rule's text rows before a pending row, a
// pending row's text without its event, a latch row, one beside a pending row,
// a latch text without its event, a pending row on the swing decline, one on
// both declines, and the events the gates before the merge wrote — the quiet
// decline's, the slow pattern's. The rows are written the way the engines write
// them, each with its event; a row of text alone carries no event, and so no
// state.
func sampledGateState(r *rand.Rand, trade aggragates.Trades) aggragates.Trades {
	entries := entryFills(trade)
	newest, before := trade.PositionPrice, trade.PositionPrice
	if len(entries) >= 2 {
		before = entries[len(entries)-2].Price
	}
	marker := PendingRow(newest, quietFinding(nil))
	cancel := CancelledRow(newest, quietFinding(nil))
	earlier := PendingRow(before, quietFinding(nil))
	latch := LatchedRow(newest, indecisionFinding(nil))

	trade.Logs, trade.StrategyEvents = nil, nil
	switch r.IntN(14) {
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
	case 10:
		return withRows(trade, PendingRow(newest, swingFinding(nil)))
	case 11:
		return withRows(trade, PendingRow(newest, indecisionFinding(nil), quietFinding(nil), swingFinding(nil)))
	case 12:
		return withEvents(trade, eventOf(GateSlowDecline, legacyDocument(EventPending, newest)))
	case 13:
		return withEvents(trade, eventOf(gateSlowPattern, legacyDocument(EventPending, newest)), eventOf(gateIndecision, legacyDocument(EventLatched, newest)))
	}
	return trade
}

// sampledBlock draws a reading: the verdict, the leg on and quiet with its
// bars around the newest fill's stamp or none, a recent bar the verdict stood
// on or none with the look-back's oldest bar before or after that stamp or
// none, the fill window from before or after that stamp or none, the
// indecision reading, and capital protection's two trend flags, the Trend Direction read on
// every row (the trend refusal is read apart).
func sampledBlock(r *rand.Rand) aggragates.SmartTakeLossIndicators {
	stamp := testutil.At("21:30:00")
	bars := []int64{0, stamp.Add(-time.Hour).UnixMilli(), stamp.Add(time.Hour).UnixMilli()}
	return aggragates.SmartTakeLossIndicators{
		QuietDeclineVerdict:         r.IntN(2) == 0,
		QuietDeclineLegQuiet:        r.IntN(2) == 0,
		QuietDeclineSmoothFrom:      bars[r.IntN(len(bars))],
		QuietDeclineFillBefore:      bars[r.IntN(len(bars))],
		QuietDeclineExitReasons:     slowDeclineReasons,
		QuietDeclineBreakReasons:    quietDeclineBreakReasons,
		Indecision:                  r.IntN(2) == 0,
		QuietDeclineRecentAt:        bars[r.IntN(len(bars))],
		QuietDeclineRecentFrom:      bars[r.IntN(len(bars))],
		QuietDeclineRecentReasons:   quietDeclineRecentReasons,
		QuietDeclineFillFrom:        bars[r.IntN(len(bars))],
		CapitalProtectionSmcBearish: r.IntN(2) == 0,
		CapitalProtectionSmcBullish: r.IntN(2) == 0,
		SmcTrendDown:                everyRow,
		SmcTrendValid:               everyRow,
	}
}

// capitalProtectionTriggers is the trigger stated on the ladder's own count: the
// switch, a spot parent under the flag outside impasse, long or inverse, its last
// configured depth filled, and the SMC trend flag of its side (bearish for a long
// ladder, bullish for an inverse one).
func capitalProtectionTriggers(trade aggragates.Trades, block aggragates.SmartTakeLossIndicators) bool {
	depths := ladder.ConfiguredDepths(trade)
	flag := block.CapitalProtectionSmcBearish
	if trade.Inverse {
		flag = block.CapitalProtectionSmcBullish
	}
	return capitalProtectionExit && trade.Strategy.Params.SmartTakeLoss && trade.ParentID == 0 &&
		trade.Strategy.TradeType != aggragates.Futures && !trade.Strategy.Params.Impasse &&
		depths > 0 && ladder.CountFilledEntries(trade) >= depths && fundsRefusedLogged(trade) && flag
}

// capitalProtectionTriggeredRow is whether Apply's answer carries the gate's triggered row.
func capitalProtectionTriggeredRow(got Result) bool {
	return slices.ContainsFunc(got.Rows, func(row Row) bool {
		return row.Gate == GateCapitalProtection && row.Event == EventTriggered
	})
}

// The last depth is the ladder's own: ladder.ConfiguredDepths — the Depths of
// the settings row the next fill would use, the base row past the table,
// floored — against ladder.CountFilledEntries, which counts a legacy row
// without an order id as a depth of its own and an accounting row as none.
// capitalProtectionWatched, rebuildState's watch, Armed with the quiet decline,
// the early take profit, the swing decline and the flat market gate switched
// off — the swing decline watches every long spot ladder from
// swingdecline.ArmDepth fills and the flat market gate every spot
// ladder from its first — and Apply's triggered row on the trend reading all
// read it so.
func TestCapitalProtectionWatchIsTheLaddersOwnLastDepth(t *testing.T) {
	withCapitalProtectionExit(t, true)
	withQuietDecline(t, false)
	withEarlyTakeProfit(t, false)
	withSwingDecline(t, false)
	withFlatMarket(t, false)
	withFlatMarket(t, false)
	ladderOf := func(depths int) aggragates.Trades {
		return fundsRefused(testutil.LadderTrade(false, fills(depths, "21:30:00")...))
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
		if own := capitalProtectionTriggers(tc.trade, solBlock()); own != tc.want {
			t.Fatalf("%s: fixture drifted, the ladder's own count reads %v", name, own)
		}
		watched, state, armed := capitalProtectionWatched(tc.trade), rebuildState(tc.trade).capitalProtectionWatched, Armed(tc.trade)
		triggered := capitalProtectionTriggeredRow(Apply(tc.trade, "", lastDepthPrint, withBlock(solBlock())))
		if watched != tc.want || state != tc.want || armed != tc.want || triggered != tc.want {
			t.Errorf("%s: watched %v, rebuildState %v, armed %v, triggered %v; want %v", name, watched, state, armed, triggered, tc.want)
		}
	}
}
