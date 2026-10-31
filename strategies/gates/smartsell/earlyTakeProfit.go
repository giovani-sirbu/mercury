package smartsell

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/earlytakeprofit"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/quietdecline"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/swingdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// earlyTakeProfitWatched is whether the early take profit watches a trade: earlyTakeProfitEligible
// and at least earlytakeprofit.ArmDepth filled entries; fills never disappear, so it stays watched.
func earlyTakeProfitWatched(trade aggragates.Trades) bool {
	return earlyTakeProfitEligible(trade) && ladder.CountFilledEntries(trade) >= earlytakeprofit.ArmDepth
}

// earlyTakeProfitEligible is the part of the watch the fills do not decide: earlytakeprofit.Enabled
// and a long spot ladder.
func earlyTakeProfitEligible(trade aggragates.Trades) bool {
	return earlyTakeProfit && !trade.Inverse && trade.Strategy.TradeType != aggragates.Futures
}

// latchMarker is the finding that marks a row's event as the latch (FeatureEarlyTakeProfit).
var latchMarker = Finding{Feature: FeatureEarlyTakeProfit}

// latchEarlyTakeProfit latches a watched ladder for life on the first of ten readings: sophos'
// indecision (indecisionReads) and its look-back (indecisionReadsRecently), a depth priority hold
// (depthPriorityLatch) and a ladder that opened raised (dynamicParamsLatch), held ticks too; then,
// never held, the quiet decline, the swing decline, the linear decline, the flat market and the free
// fall prevention, each behind its switch in package earlytakeprofit. The tick's row carries the
// marker and the reading's finding; a tick with no row writes the latched row.
func latchEarlyTakeProfit(trade aggragates.Trades, st state, block aggragates.SmartSellIndicators, found tick) (state, tick) {
	if !st.earlyTakeProfitWatched || st.earlyTakeProfit {
		return st, found
	}
	if indecisionReads(block) {
		found.findings = append(found.findings, latchMarker, Finding{Feature: FeatureIndecision, Reasons: block.QuietDeclineBreakReasons})
	} else if indecisionReadsRecently(block) {
		found.findings = append(found.findings, latchMarker, Finding{Feature: FeatureIndecision, Reasons: block.IndecisionRecentReasons})
	} else if finding, holds := depthPriorityLatch(trade, st); holds {
		found.findings = append(found.findings, latchMarker, finding)
	} else if finding, holds := dynamicParamsLatch(trade); holds {
		found.findings = append(found.findings, latchMarker, finding)
	} else if earlyTakeProfitQuietDecline && found.names(FeatureQuietDecline) {
		found.findings = append(found.findings, latchMarker)
	} else if reasons, reads := quietReadsOnAJudgedFill(trade, st, block, found); reads {
		found.findings = append(found.findings, latchMarker, Finding{Feature: FeatureQuietDecline, Reasons: reasons})
	} else if earlyTakeProfitSwingDecline && found.names(FeatureSwingDecline) {
		found.findings = append(found.findings, latchMarker)
	} else if reading, reads := swingReadsOnAJudgedFill(trade, st, block, found); reads {
		found.findings = append(found.findings, latchMarker, Finding{Feature: FeatureSwingDecline, Reasons: reading.Reasons})
	} else if earlyTakeProfitLinearDecline && found.names(FeatureLinearDecline) {
		found.findings = append(found.findings, latchMarker)
	} else if reading, reads := linearDeclineReads(st, block, found); reads {
		found.findings = append(found.findings, latchMarker, Finding{Feature: FeatureLinearDecline, Reasons: reading.Reasons})
	} else if reading, reads := earlyTakeProfitFlatMarketReads(trade, st, block, found); reads {
		found.findings = append(found.findings, latchMarker, Finding{Feature: FeatureFlatMarket, Reasons: reading.Reasons})
	} else if reading, reads := earlyTakeProfitFreeFallPreventionReads(st, block, found); reads {
		found.findings = append(found.findings, latchMarker, Finding{Feature: FeatureFreeFallPrevention, Reasons: reading.Reasons})
	} else {
		return st, found
	}
	st.earlyTakeProfit = true
	if found.kind == rowNone {
		found.kind = rowLatched
	}
	return st, found
}

// quietReadsOnAJudgedFill is the quiet decline's reading for the latch on a tick with no row and not
// held: sophos' verdict for the ladder (quietdecline.ReadsForTheLadder) or its look-back marking it
// (quietdecline.PendsRecently), of a pending ladder whose newest fill is judged while
// quietdecline.Enabled is on, of any ladder the latch watches with it off; nothing with
// earlyTakeProfitQuietDecline off or under quietdecline.ArmDepth fills.
func quietReadsOnAJudgedFill(trade aggragates.Trades, st state, block aggragates.SmartSellIndicators, found tick) ([]string, bool) {
	if !earlyTakeProfitQuietDecline || found.kind != rowNone || st.depthPriorityHeld || len(st.fills) < quietdecline.ArmDepth {
		return nil, false
	}
	if st.quietDeclineWatched && (!st.slowDeclinePending || slowDeclineFillUnjudged(trade, st)) {
		return nil, false
	}
	newest := st.lastFill()
	if quietdecline.ReadsForTheLadder(newest.At, block) {
		return block.QuietDeclineExitReasons, true
	}
	if quietdecline.PendsRecently(st.fills[0].At, newest.At, block) {
		return block.QuietDeclineRecentReasons, true
	}
	return nil, false
}

// swingReadsOnAJudgedFill is the swing decline's reading (swingDeclineReads) for the latch on a tick
// with no row and not held: of a pending ladder whose newest fill is judged while swingdecline.Enabled
// is on, of any ladder the latch watches with it off; nothing with earlyTakeProfitSwingDecline off.
func swingReadsOnAJudgedFill(trade aggragates.Trades, st state, block aggragates.SmartSellIndicators, found tick) (swingdecline.Reading, bool) {
	if !earlyTakeProfitSwingDecline || found.kind != rowNone || st.depthPriorityHeld || len(st.fills) < swingdecline.ArmDepth {
		return swingdecline.Reading{}, false
	}
	if st.swingDeclineWatched && (!st.slowDeclinePending || slowDeclineFillUnjudged(trade, st)) {
		return swingdecline.Reading{}, false
	}
	return swingDeclineReads(st, block)
}
