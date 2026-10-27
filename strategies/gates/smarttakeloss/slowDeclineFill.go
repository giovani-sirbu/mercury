package smarttakeloss

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/lineardecline"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/swingdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// declineReading is what one decline feature read at a pending ladder's new
// fill: whether it can answer on this tick (taken), whether it holds, and the
// reasons it names either way.
type declineReading struct {
	feature string
	taken   bool
	holds   bool
	reasons []string
	breaks  []string
}

// slowDeclineFillUnjudged is whether a pending ladder's newest fill is one the gate has not judged:
// its price is not the one the ladder is pending from, a pending feature is judged at a new fill
// (judgesTheNewFill), and it is not the last depth's fill, which is never judged.
func slowDeclineFillUnjudged(trade aggragates.Trades, st state) bool {
	if !st.slowDeclinePending || st.lastFill().Price == st.slowDeclinePendingFrom || !st.judgesTheNewFill() {
		return false
	}
	return !lastDepthFilled(trade, len(st.fills))
}

// featureResetsOnNewDepth is whether a decline feature is judged at a new fill: the swing decline
// under swingdecline.ResetOnNewDepth, the linear decline under lineardecline.ResetOnNewDepth, the
// quiet decline always.
func featureResetsOnNewDepth(feature string) bool {
	switch feature {
	case FeatureSwingDecline:
		return swingDeclineResetOnNewDepth
	case FeatureLinearDecline:
		return linearDeclineResetOnNewDepth
	}
	return true
}

// judgesTheNewFill is whether any feature the ladder is pending on is judged at a new fill; a ladder
// pending on no named feature is.
func (st state) judgesTheNewFill() bool {
	if len(st.pendingFeatures) == 0 {
		return true
	}
	for _, feature := range st.pendingFeatures {
		if featureResetsOnNewDepth(feature) {
			return true
		}
	}
	return false
}

// keptPending are the findings of the pending features a new fill is not judged on
// (featureResetsOnNewDepth off), kept pending as they are, with no reason.
func keptPending(st state) []Finding {
	var kept []Finding
	for _, feature := range st.pendingFeatures {
		if !featureResetsOnNewDepth(feature) {
			kept = append(kept, Finding{Feature: feature})
		}
	}
	return kept
}

// newFillReadings are the readings of the decline features that apply to a pending ladder's new
// fill, each watching the ladder and pended on (pendsOn): the quiet decline once sophos serves the
// fill window (quietDeclineConfirms), the swing and the linear decline once the series closed the
// fill's bar (swingdecline.Trigger, lineardecline.Trigger); a reading not taken holds nothing.
func newFillReadings(st state, block aggragates.SmartTakeLossIndicators) []declineReading {
	newest := st.lastFill()
	var readings []declineReading
	if st.quietDeclineWatched && st.pendsOn(FeatureQuietDecline) {
		reading := declineReading{feature: FeatureQuietDecline, taken: block.QuietDeclineFillFrom > 0, breaks: block.QuietDeclineBreakReasons}
		if reading.taken {
			reading.reasons, reading.holds = quietDeclineConfirms(newest, block)
		}
		readings = append(readings, reading)
	}
	bars := series.SeriesOf(block)
	closed := bars.Served() && bars.LastBarOpen() >= series.BarOpen(newest.At)
	if st.swingDeclineWatched && swingDeclineResetOnNewDepth && bars.Served() && st.pendsOn(FeatureSwingDecline) {
		reading := declineReading{feature: FeatureSwingDecline, taken: closed}
		if reading.taken {
			read, holds := swingdecline.Trigger(swingFills(st.fills), bars, series.TrendOf(block))
			reading.holds, reading.reasons, reading.breaks = holds, read.Reasons, read.Breaks
		}
		readings = append(readings, reading)
	}
	if st.linearDeclineWatched && linearDeclineResetOnNewDepth && bars.Served() && st.pendsOn(FeatureLinearDecline) {
		reading := declineReading{feature: FeatureLinearDecline, taken: closed}
		if reading.taken {
			fills, trend := swingFills(st.fills), series.TrendOf(block)
			read, holds := lineardecline.Trigger(fills, bars, trend)
			reading.holds, reading.reasons = holds, read.Reasons
			if !holds {
				reading.breaks = lineardecline.Breaks(fills, bars, trend)
			}
		}
		readings = append(readings, reading)
	}
	return readings
}

// judgeTheNewestFill judges a pending ladder's unjudged newest fill on the readings that apply
// (newFillReadings), the features not judged at a new fill kept pending (keptPending): confirmed as
// soon as any taken reading holds, confirmed on the kept features alone once every reading is taken
// and none holds, cancelled then with no kept feature, otherwise waiting with no row.
func judgeTheNewestFill(trade aggragates.Trades, st state, block aggragates.SmartTakeLossIndicators) (state, tick) {
	if !slowDeclineFillUnjudged(trade, st) {
		return st, tick{}
	}
	readings := newFillReadings(st, block)
	holding := keptPending(st)
	kept, taken := len(holding), len(readings) > 0
	var missed []Finding
	for _, reading := range readings {
		switch {
		case !reading.taken:
			taken = false
		case reading.holds:
			holding = append(holding, Finding{Feature: reading.feature, Reasons: reading.reasons})
		default:
			missed = append(missed, Finding{Feature: reading.feature, Reasons: reading.breaks})
		}
	}
	switch {
	case len(holding) > kept || (taken && kept > 0):
		return st.pendingOn(holding), tick{kind: rowConfirmed, findings: holding}
	case taken:
		return st.notPending(), tick{kind: rowCancelled, findings: missed}
	}
	return st, tick{}
}

// slowDeclineTriggered is whether the gate's trigger stands for the shared action: the ladder is
// pending and its newest fill is judged (the last depth's fill counts as judged).
func slowDeclineTriggered(trade aggragates.Trades, st state) bool {
	return st.slowDeclinePending && !slowDeclineFillUnjudged(trade, st)
}
