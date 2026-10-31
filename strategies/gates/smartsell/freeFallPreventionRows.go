package smartsell

import "strings"

// FreeFallPreventionTriggeredRow is the row of the tick a reading opens an episode: fill the newest
// fill, reasons the reading's and the wait the sale keeps from the row (saleWaitReasons), listed in
// parentheses; its event is EventTriggered under GateFreeFallPrevention.
func FreeFallPreventionTriggeredRow(fill float64, reasons []string) Row {
	reasons = saleWaitReasons(reasons)
	message := RowPrefix + "free fall prevention triggered"
	if len(reasons) > 0 {
		message += " (" + strings.Join(reasons, ", ") + ")"
	}
	return Row{
		Message: message,
		Price:   fill,
		Gate:    GateFreeFallPrevention,
		Event:   EventTriggered,
		Reasons: reasons,
	}
}

// FreeFallPreventionSaleHeldRow is the row of an episode's tick the trigger holds the gate's sale
// on, the first tick of the episode and again whenever the trigger moves (profitPct from saleProfit
// under FullTrendTrigger, fill the newest fill); its event is EventSaleHeld under GateFreeFallPrevention.
func FreeFallPreventionSaleHeldRow(fill, profitPct float64) Row {
	return saleHeldByProfitRow(GateFreeFallPrevention, fill, profitPct, FullTrendTrigger(), nil)
}

// FreeFallPreventionActivatedRow is the row of the tick the shared action activates the
// sale on the gate's trigger (ActivatedRow with no features, under GateFreeFallPrevention).
func FreeFallPreventionActivatedRow(fill, profitPct float64) Row {
	return activatedRow(GateFreeFallPrevention, fill, profitPct, FullTrendTrigger(), nil)
}

// FreeFallPreventionResetRow is the row of the tick a new depth closes an open episode under
// freefallprevention.ResetOnNewDepth (fill the new newest fill); its event is EventReset under
// GateFreeFallPrevention.
func FreeFallPreventionResetRow(fill float64) Row {
	return Row{
		Message: RowPrefix + "free fall prevention reset at the new depth",
		Price:   fill,
		Gate:    GateFreeFallPrevention,
		Event:   EventReset,
	}
}
