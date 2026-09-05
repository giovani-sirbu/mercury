package smarttakeloss

import (
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// ProtectiveTick is the smart-take-loss override that live and backtest
// must apply identically after the ladder (and any crash widen) has chosen
// a position. The caller owns CrashActive; this function does not fetch
// sophos. Crash Guard does not emit sellLoss — it only widens and holds.
//
// STLForced is true for EVERY exit this overlay forces, the risk-driven
// trigger and the stale-bag cut alike, so an engine that logs on it records
// both. Eval.StaleCut tells them apart and TriggeredMessage prints the
// matching row.
type ProtectiveTick struct {
	Position  string
	STLForced bool
	Eval      Evaluation
	SmartHigh bool
}

// ApplyProtectiveTick overlays smart take-loss on a ladder position.
// UseAI is not a gate: SmartTakeLoss is required on the strategy params.
// crashActive still feeds the STL emergency gate (do not cut underwater
// while a flush is armed).
func ApplyProtectiveTick(
	trade aggragates.Trades,
	position string,
	price float64,
	now time.Time,
	ai aggragates.AIIndicators,
	crashActive bool,
) ProtectiveTick {
	result := ProtectiveTick{Position: position}

	if !trade.Strategy.Params.SmartTakeLoss {
		return result
	}

	eval, high := Evaluate(trade, price, ai)
	result.Eval = eval
	result.SmartHigh = high

	inStates := InStates(trade.PositionType)
	if high && !inStates && !protectedPosition(result.Position) {
		if forced, ok := Position(trade, price, ai); ok {
			result.Position = forced
			result.STLForced = true
		}
	}

	if result.STLForced || InTrailStates(trade.PositionType) {
		result.Position = ApplyExitPolicy(
			trade,
			result.Position,
			price,
			now,
			crashActive,
		)
		return result
	}

	// The stale-bag cut: an underwater deep bag past StaleAfter with the
	// crash guard clear. It used to set the position silently, so the trade
	// showed BUY_TO_SELLLOSS and nothing else; it is now a forced exit like
	// the trigger, with its own reason row.
	if !crashActive && !protectedPosition(result.Position) {
		if due, ageDays, pnl := staleBagStatus(trade, price, now); due {
			result.Position = "sellLoss"
			result.STLForced = true
			result.Eval.StaleCut = true
			result.Eval.AgeDays = ageDays
			result.Eval.EstProfit = pnl
		}
	}

	return result
}
