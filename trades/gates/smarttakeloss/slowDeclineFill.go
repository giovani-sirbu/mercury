package smarttakeloss

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/swingdecline"
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

// slowDeclineFillUnjudged is whether a pending ladder's newest fill (in
// history slice order, st.lastFill) is one the gate has not judged: its price
// is not the one the ladder is pending from (st.slowDeclinePendingFrom).
//
// The fill that takes the ladder to its last depth (lastDepthFilled, on
// st.fills, counted the way ladder.CountFilledEntries counts them) is never
// judged: the ladder stays pending from where it was, its band live — and
// capital protection watches it from that fill on.
//
// The pending events hold prices, not fills, so a later fill at exactly the
// price the ladder is pending from reads as judged. That limit is accepted.
func slowDeclineFillUnjudged(trade aggragates.Trades, st state) bool {
	if !st.slowDeclinePending || st.lastFill().Price == st.slowDeclinePendingFrom {
		return false
	}
	return !lastDepthFilled(trade, len(st.fills))
}

// newFillReadings are the readings of the decline features that apply to a
// pending ladder's new fill, the quiet decline first. The quiet decline applies
// to a ladder it watches and is taken on a tick sophos serves its sell band on —
// sophos serves the band whenever it read the window, so none is sophos down or
// no window cached — and holds on quietDeclineConfirms. The swing decline
// applies to a ladder it watches while the series is served, is taken once the
// series has closed the fill's bar, and holds when swingdecline.Trigger does,
// the window read again with the new fill as its newest and the trend of this
// tick. A reading not taken holds nothing and names nothing.
func newFillReadings(st state, block aggragates.SmartTakeLossIndicators) []declineReading {
	newest := st.lastFill()
	var readings []declineReading
	if st.quietDeclineWatched {
		reading := declineReading{feature: FeatureQuietDecline, taken: block.SlowDeclineSellBand > 0, breaks: block.SlowDeclineBreakReasons}
		if reading.taken {
			reading.reasons, reading.holds = quietDeclineConfirms(newest, block)
		}
		readings = append(readings, reading)
	}
	if series := swingdecline.SeriesOf(block); st.swingDeclineWatched && series.Served() {
		reading := declineReading{feature: FeatureSwingDecline, taken: series.LastBarOpen() >= swingdecline.BarOpen(newest.At)}
		if reading.taken {
			read, holds := swingdecline.Trigger(swingFills(st.fills), series, swingdecline.TrendOf(block))
			reading.holds, reading.reasons, reading.breaks = holds, read.Reasons, read.Breaks
		}
		readings = append(readings, reading)
	}
	return readings
}

// judgeTheNewestFill judges a pending ladder's unjudged newest fill
// (slowDeclineFillUnjudged) on the readings that apply to it (newFillReadings),
// each answering once it is taken. The gate CONFIRMS the fill as soon as any
// taken reading holds: the confirmed row, naming the features that held, and
// the ladder pending from that fill on them. It CANCELS it once every reading
// that applies is taken and none holds: the cancelled row, naming what each
// broke, and the ladder watched and not pending, which only goesPending makes
// pending again. It WAITS otherwise — a reading not taken yet, no reading that
// applies at all: no row, and the fill stays unjudged, which sells nothing
// (sellsAtTheBand). The sale and the take profit read the state this leaves, so
// a cancelled ladder sells nothing on the cancel tick itself, and a confirmed
// one sells on the confirming tick. The take profit goes back to the average
// entry price alone from the next tick: the engines read TakeProfitPercentage
// before Apply, so the cancelled event reaches it a tick after the one that
// wrote it.
//
// The reading served on the fill's first tick is that of the last closed bar,
// so a flush that starts and fills inside the forming bar is judged on the bar
// before it. That limit is accepted. A bar among the last closed ones on which
// the verdict stood keeps holding new fills for as long as sophos looks back
// over it, whatever the last closed bar reads.
func judgeTheNewestFill(trade aggragates.Trades, st state, block aggragates.SmartTakeLossIndicators) (state, tick) {
	if !slowDeclineFillUnjudged(trade, st) {
		return st, tick{}
	}
	readings := newFillReadings(st, block)
	var holding, missed []Finding
	for _, reading := range readings {
		switch {
		case !reading.taken:
		case reading.holds:
			holding = append(holding, Finding{Feature: reading.feature, Reasons: reading.reasons})
		default:
			missed = append(missed, Finding{Feature: reading.feature, Reasons: reading.breaks})
		}
	}
	switch {
	case len(holding) > 0:
		return st.pendingOn(holding), tick{kind: rowConfirmed, findings: holding}
	case len(readings) > 0 && len(missed) == len(readings):
		return st.notPending(), tick{kind: rowCancelled, findings: missed}
	}
	return st, tick{}
}

// sellBandReached is a pending ladder's one sell target: the tick price at
// or over the sell band sophos serves (SlowDeclineSellBand). A zero band —
// sophos down, a window too short to compute it — sells nothing, and the
// ladder stays pending.
func sellBandReached(price float64, block aggragates.SmartTakeLossIndicators) bool {
	return block.SlowDeclineSellBand > 0 && price >= block.SlowDeclineSellBand
}

// sellsAtTheBand is the gate's one sale, decided in one place: the ladder is
// pending, its newest fill is judged — the ladder's last-depth fill is never
// judged and counts as judged, and one the gate is still waiting on sells
// nothing — and the price is at or over the band. Apply asks it past the
// ladder's own closes and after the tick's row, so the state it reads is the
// one that row left, and not on a ladder a depth priority holds. Under break
// even a pending ladder's take profit reads the average entry price alone
// (TakeProfitPercentage), so the gate's one sale there is its sell band; from
// break even up the ladder proposes its own take profit, a close Apply passes
// through. It names the sale the gate would make; whether the loss cap lets it
// go (bandSaleLoss) is Apply's to ask next.
func sellsAtTheBand(trade aggragates.Trades, st state, price float64, block aggragates.SmartTakeLossIndicators) bool {
	return st.slowDeclinePending && !slowDeclineFillUnjudged(trade, st) && sellBandReached(price, block)
}
