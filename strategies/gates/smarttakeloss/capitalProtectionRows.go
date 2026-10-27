package smarttakeloss

import (
	"fmt"
	"strings"
)

// CapitalProtectionTriggeredRow is the row of the tick the trend reading opens an episode: fill the
// newest fill, filled of ceiling the depths, the trend named on the ladder's side and the wait the
// sale keeps from the row (capitalprotection.WaitAfterTrigger); its event is EventTriggered under
// GateCapitalProtection.
func CapitalProtectionTriggeredRow(fill float64, filled, ceiling int, inverse bool) Row {
	reasons := capitalProtectionReasons(filled, ceiling, inverse)
	return Row{
		Message: "smartTakeLoss: capital protection triggered (" + strings.Join(reasons, ", ") + ")",
		Price:   fill,
		Gate:    GateCapitalProtection,
		Event:   EventTriggered,
		Reasons: reasons,
	}
}

// capitalProtectionReasons word the trigger: the last depth filled, the next refused for funds, the
// SMC trend against the ladder and, while one stands, the wait the sale keeps from the trigger.
func capitalProtectionReasons(filled, ceiling int, inverse bool) []string {
	direction := "down"
	if inverse {
		direction = "up"
	}
	reasons := []string{
		fmt.Sprintf("depth %d of %d filled", filled, ceiling),
		"insufficient funds for the next depth",
		"SMC trend " + direction + " on every timeframe",
	}
	if capitalProtectionWaitAfterTrigger > 0 {
		reasons = append(reasons, "the sale waits "+capitalProtectionWaitAfterTrigger.String()+" from the trigger")
	}
	return reasons
}

// CapitalProtectionSaleHeldRow is the row of an episode's tick the drawdown holds the gate's sale on,
// the first tick of the episode and again whenever the trigger moves, which the drawdown never does
// (profitPct from saleProfit, fill the newest fill); its event is EventSaleHeld under GateCapitalProtection.
func CapitalProtectionSaleHeldRow(fill, profitPct float64) Row {
	return saleHeldByProfitRow(GateCapitalProtection, fill, profitPct, capitalProtectionTerms.triggerPct(sharedTrigger{}), nil)
}

// CapitalProtectionStopLossActivatedRow is the row of the tick the shared action activates the
// stopLoss on the gate's trigger (stopLossActivatedRow with no features, under GateCapitalProtection).
func CapitalProtectionStopLossActivatedRow(fill, profitPct float64) Row {
	return stopLossActivatedRow(GateCapitalProtection, fill, profitPct, capitalProtectionTerms.triggerPct(sharedTrigger{}), nil)
}

// CapitalProtectionResetRow is the row of the tick a new fill past the ceiling closes an open
// episode (fill the new newest fill); its event is EventReset under GateCapitalProtection.
func CapitalProtectionResetRow(fill float64) Row {
	return Row{
		Message: "smartTakeLoss: capital protection reset at the new depth",
		Price:   fill,
		Gate:    GateCapitalProtection,
		Event:   EventReset,
	}
}
