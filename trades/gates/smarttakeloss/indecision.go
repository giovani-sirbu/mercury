package smarttakeloss

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/swingdecline"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// indecisionWatched is whether the indecision direction watches a trade:
// indecisionEligible admits it and it holds IndecisionArmDepth filled entries
// or more. The latch carries the newest fill's price, so the watch needs fills
// to carry. The flag and the parent check are Armed's and Apply's;
// rebuildState reads the same watch off the fills it has already folded, with
// the same bound. Fills never disappear, so a watched ladder stays watched for
// the rest of its life.
func indecisionWatched(trade aggragates.Trades) bool {
	return indecisionEligible(trade) && ladder.CountFilledEntries(trade) >= IndecisionArmDepth
}

// indecisionEligible is the part of the watch the fills do not decide:
// Indecision on, a long ladder and a spot one. A futures long keeps the take
// profit from its average entry price, as it does while the gate holds it
// pending (TakeProfitPercentage), and hermes' futures path never runs the
// overlay.
func indecisionEligible(trade aggragates.Trades) bool {
	return indecision && !trade.Inverse && trade.Strategy.TradeType != aggragates.Futures
}

// latchIndecision is the indecision direction's part of a tick: a watched
// ladder not latched yet is latched, for the rest of its life, one of two ways,
// and the latch joins the row the tick writes — it is the row itself, of kind
// rowLatched, when the tick found nothing else.
//
// Sophos serves the indecision (SlowDeclineIndecision) from any reading: the
// vote's, the thresholds' near miss, the SMC trend tolerance's, or the whole
// SMC trend table bearish. The flag alone decides it, whatever
// SlowDeclineLegQuiet and SlowDeclineExit say: the ladder need not be pending,
// so one whose own smoothness fails, or whose newest fill is outside the fill
// window, latches all the same, and the latch names SlowDeclineBreakReasons. A
// depth priority pauses the gate and capital protection, never this reading: it
// lands on a held tick too.
//
// Failing that the swing decline latches: when it is a subject of the tick's
// row — the trigger that made the ladder pending, or the confirmation of its new
// fill — the row names the latch beside it with no reasons of its own, the
// swing decline's being the row's; and when it reads at the judged newest fill
// of a ladder already pending (swingReadsOnAJudgedFill) the latch is the row,
// naming the swing decline's reasons. A held tick reads no swing decline.
//
// A latched ladder gets no second latch however often either reading comes
// again, and nothing takes the latch away: it holds until the trade closes. A
// ladder the feature does not watch gets nothing.
func latchIndecision(trade aggragates.Trades, st state, block aggragates.SmartTakeLossIndicators, found tick) (state, tick) {
	if !st.indecisionWatched || st.indecision {
		return st, found
	}
	if block.SlowDeclineIndecision {
		found.findings = append(found.findings, Finding{Feature: FeatureIndecision, Reasons: block.SlowDeclineBreakReasons})
	} else if found.names(FeatureSwingDecline) {
		found.findings = append(found.findings, Finding{Feature: FeatureIndecision})
	} else if reading, reads := swingReadsOnAJudgedFill(trade, st, block, found); reads {
		found.findings = []Finding{{Feature: FeatureSwingDecline, Reasons: reading.Reasons}}
	} else {
		return st, found
	}
	st.indecision = true
	if found.kind == rowNone {
		found.kind = rowLatched
	}
	return st, found
}

// swingReadsOnAJudgedFill is the swing decline's reading for a ladder already
// pending whose newest fill is judged — no row on this tick, not held, and the
// fill neither unjudged nor waiting — within swingdecline.SwingDeclineReadBars
// bars of that fill's bar (swingDeclineReads): the ladder that went pending on
// the quiet decline, or whose swing decline was off or not read then, is latched
// once the swing decline reads for its newest fill.
func swingReadsOnAJudgedFill(trade aggragates.Trades, st state, block aggragates.SmartTakeLossIndicators, found tick) (swingdecline.Reading, bool) {
	if found.kind != rowNone || st.depthPriorityHeld || !st.slowDeclinePending || slowDeclineFillUnjudged(trade, st) {
		return swingdecline.Reading{}, false
	}
	return swingDeclineReads(st, block)
}
