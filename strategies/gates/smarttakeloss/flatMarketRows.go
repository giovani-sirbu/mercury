package smarttakeloss

import "strings"

// FlatMarketTriggeredRow is the row of the tick a reading opens an episode: fill the newest fill,
// reasons the reading's and the wait the sale keeps from the row (saleWaitReasons), listed in
// parentheses; its event is EventTriggered under GateFlatMarket.
func FlatMarketTriggeredRow(fill float64, reasons []string) Row {
	reasons = saleWaitReasons(reasons)
	message := "smartTakeLoss: flat market triggered"
	if len(reasons) > 0 {
		message += " (" + strings.Join(reasons, ", ") + ")"
	}
	return Row{
		Message: message,
		Price:   fill,
		Gate:    GateFlatMarket,
		Event:   EventTriggered,
		Reasons: reasons,
	}
}

// FlatMarketSaleHeldRow is the row of an episode's tick the trigger holds the gate's sale on, the
// first tick of the episode and again whenever the trigger moves (profitPct from saleProfit under
// FullTrendTrigger, fill the newest fill); its event is EventSaleHeld under GateFlatMarket.
func FlatMarketSaleHeldRow(fill, profitPct float64) Row {
	return saleHeldByProfitRow(GateFlatMarket, fill, profitPct, FullTrendTrigger(), nil)
}

// FlatMarketStopLossActivatedRow is the row of the tick the shared action activates the stopLoss
// on the gate's trigger (StopLossActivatedRow with no features, under GateFlatMarket).
func FlatMarketStopLossActivatedRow(fill, profitPct float64) Row {
	return stopLossActivatedRow(GateFlatMarket, fill, profitPct, FullTrendTrigger(), nil)
}

// FlatMarketResetRow is the row of the tick a new depth closes an open episode under
// flatmarket.ResetOnNewDepth (fill the new newest fill); its event is EventReset under GateFlatMarket.
func FlatMarketResetRow(fill float64) Row {
	return Row{
		Message: "smartTakeLoss: flat market reset at the new depth",
		Price:   fill,
		Gate:    GateFlatMarket,
		Event:   EventReset,
	}
}
