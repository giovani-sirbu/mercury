package smarttakeloss

import (
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// gapped is whether a move against the anchor falls under minus SaleGapTolerances tolerances; no floor without both above zero.
func gapped(move, tolerance float64) bool {
	return saleGapTolerances > 0 && tolerance > 0 && move < -tolerance*saleGapTolerances
}

// committedStep trails the committed trigger's ladder at now (trailingStep) through its gap episode (SaleGapWait).
func committedStep(trade aggragates.Trades, trigger saleTrigger, price, fill float64, now time.Time) ([]Row, saleStep, string) {
	step := trailingStep(trade, price)
	switch {
	case step == stepSell:
		return nil, stepSell, trigger.reason
	case step == stepGapHold && trigger.gapRowDue:
		move, row, _ := trailingMove(trade, price)
		return []Row{saleHeldByGapRow(trigger.gate, fill, -move, row.Tolerance*saleGapTolerances, saleGapTolerances, trigger.features)}, stepHold, ""
	case step == stepGapHold && gapWaited(trigger.gapHeldAt, now):
		return nil, stepSell, trigger.reason
	case step == stepGapHold:
		return nil, stepHold, ""
	case !trigger.gapHeldAt.IsZero():
		move, _, _ := trailingMove(trade, price)
		return []Row{gapClosedRow(trigger.gate, fill, move, trigger.features)}, step, ""
	}
	return nil, step, ""
}

// gapWaited is whether a gap stamped at heldAt has stood SaleGapWait at now; never with the knob off, no clock or no stamp.
func gapWaited(heldAt, now time.Time) bool {
	return saleGapWait > 0 && !now.IsZero() && !heldAt.IsZero() && now.Sub(heldAt) >= saleGapWait
}

// gapStamp is the gap stamp a gap kind leaves: the event's stamp when it holds a gap, none when it closes one.
func gapStamp(event string, at time.Time) time.Time {
	if event == EventGapHeld {
		return at
	}
	return time.Time{}
}

// heldByGap is the reason text of a gap hold: the print's move under the anchor over the gap, in tolerances and percent.
func heldByGap(movePct, gapPct, tolerances float64) string {
	return fmt.Sprintf("print %.2f%% under the anchor, over the gap of %g tolerances (%.2f%%)", movePct, tolerances, gapPct)
}

// saleHeldByGapRow is saleHeldRow's grammar for a committed sale the gap holds (heldByGap), its event EventGapHeld.
func saleHeldByGapRow(gate string, fill, movePct, gapPct, tolerances float64, features []string) Row {
	row := saleHeldRow(gate, fill, heldByGap(movePct, gapPct, tolerances), features)
	row.Event = EventGapHeld
	return row
}

// SaleHeldByGapRow is the row of a committed sale's gap hold opening a gap under gate (saleHeldByGapRow), its event EventGapHeld.
func SaleHeldByGapRow(gate string, fill, movePct, gapPct float64, tolerances float64, features ...string) Row {
	return saleHeldByGapRow(gate, fill, movePct, gapPct, tolerances, features)
}

// gapClosedRow is the row of a gap closing on a print at movePct against the anchor, under or over it by its sign.
func gapClosedRow(gate string, fill, movePct float64, features []string) Row {
	side := "under"
	if movePct > 0 {
		side = "over"
	}
	return Row{
		Message:  fmt.Sprintf("smartTakeLoss: %s gap closed, print %.2f%% %s the anchor", saleHeldLabel(gate), math.Abs(movePct), side),
		Price:    fill,
		Gate:     gate,
		Event:    EventGapClosed,
		Features: slices.Clone(features),
	}
}

// GapClosedRow is the row of a committed sale's gap closing under gate (gapClosedRow), its event EventGapClosed.
func GapClosedRow(gate string, fill, movePct float64, features ...string) Row {
	return gapClosedRow(gate, fill, movePct, features)
}
