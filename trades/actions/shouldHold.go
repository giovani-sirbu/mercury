package actions

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
	"github.com/giovani-sirbu/mercury/trades/gates/ai"
	"github.com/giovani-sirbu/mercury/trades/gates/cooldown"
	"github.com/giovani-sirbu/mercury/trades/gates/crashguard"
	"github.com/giovani-sirbu/mercury/trades/gates/patterns"
	"github.com/giovani-sirbu/mercury/trades/gates/regime"
	"github.com/giovani-sirbu/mercury/trades/gates/smarttakeloss"
)

// ShouldHold blocks the action chain when a strategy flag advises against
// acting. Holds are recorded by gates.SaveHoldLog as INFO trade-log entries.
//
// OWNERSHIP. Every gate answers to exactly one strategy flag, flag first and
// payload second: `params.X && <payload present>`. Payload presence is only a
// degrade-open check, never a switch-on — a verdict fetched for one flag's
// sake gains no gate for another (see StrategyParams.NeedsSophos).
//
//	first fill (OldPosition "new")   Cooldown  → first-fill gate (1h + 15m)
//	                                 UseAI     → legacy bullish/bearish veto
//	open position                    RegimeHold    → 15m shock hold, add veto, profit hold
//	                                 UsePatterns   → chart-pattern and fibonacci holds
//	                                 UseAI         → legacy AI hold
//	                                 CrashGuard    → flush park, sticky reclaim, capitulation
//	                                 SmartTakeLoss → HTF add freeze
//	                                 Cooldown      → depth spacing (stopLoss only)
//
// Each family lives in its own package under trades/gates; this function
// only orders them. Cooldown owns TWO gates, one on each side of the first
// fill: the gate that decides whether the trade opens here at all, and depth
// spacing that keeps the ladder from cascading through every depth in one
// drop. They share a flag because they are the same idea — do not spend
// capital faster than the move deserves — but nothing else: the first-fill
// gate reads sophos /markers, the gate reads only the trade's own fill stamps.
//
// With every flag off nothing holds: the ladder runs exactly as the legacy
// engine ran it, stopped only by funds (matrix H, run R0).
//
// RegimeHold never reaches the first fill: the regime entry veto was removed
// (it measured one right call in four on run 97); the first fill is the
// cooldown's. Whether the first-buy chain runs this function at all is the
// engines' call through StrategyParams.InjectsEntryHold.
func ShouldHold(event events.Events) (events.Events, error) {
	if event.Params.OldPosition == "new" {
		return shouldHoldEntry(event)
	}
	return shouldHoldPosition(event)
}

// shouldHoldEntry is the first fill: cooldown owns it. No regime gate here.
//
// Both gates judge the direction the entry would take, resolved once by
// aggragates.EntrySide. They used to take event.Trade.Inverse, which is the
// direction on spot and never the direction on futures — where Inverse is
// always false and the ML verdict decides the side.
func shouldHoldEntry(event events.Events) (events.Events, error) {
	params := event.Trade.Strategy.Params
	side := aggragates.EntrySide(event.Trade, event.Params.AIIndicators)

	if params.Cooldown {
		if reason := cooldown.FirstFillHold(side, event.Params.CoolDownIndicators); reason != "" {
			return gates.SaveHoldLog(event, "entry", reason)
		}
	}
	if params.UseAI {
		if reason := ai.EntryHold(side, event.Params.AIIndicators); reason != "" {
			return gates.SaveHoldLog(event, "entry", reason)
		}
	}
	return event, nil
}

// shouldHoldPosition is every transition after the first fill: arming a
// depth (stopLoss), arming the exit (takeProfit) and the force-trailing
// re-anchors of either.
func shouldHoldPosition(event events.Events) (events.Events, error) {
	params := event.Trade.Strategy.Params
	indicators := event.Params.AIIndicators
	position := gates.PositionType(event.Trade.PositionType)

	// Every family answers for itself before anything is picked. Asking them
	// in a first-non-empty chain looked equivalent and was not: capitulation
	// bypasses a REGIME hold only (capitulationEligibleHold), so on a tick
	// where regime spoke first the pattern and legacy-AI verdicts were never
	// computed, and the bypass then released a trade that patterns would have
	// held on a verdict nobody ever asked for.
	regimeReason := ""
	if params.RegimeHold && indicators.HasRegimeVerdict {
		regimeReason = regime.HoldReason(event, position, indicators)
	}
	patternReason := ""
	if params.UsePatterns {
		patternReason = patterns.HoldReason(event, position, indicators)
	}
	aiReason := ""
	if params.UseAI {
		aiReason = ai.LegacyHoldReason(event, position, indicators)
	}

	reason := regimeReason
	if params.CrashGuard {
		// A flush reason replaces whatever held; capitulation may then refuse
		// or bypass a regime hold on a reclaimed dump. Both run before the
		// other families are consulted, so a bypass releases only what the
		// regime had to say. ApplyCapitulationOverride runs on every tick,
		// hold or not: leaving the ladder is what ends a live episode.
		reason = crashguard.ApplyToHold(event, position, indicators, reason)
		event, reason = crashguard.ApplyCapitulationOverride(event, position, indicators, reason)
	}
	if reason == "" {
		reason = patternReason
	}
	if reason == "" {
		reason = aiReason
	}

	if reason == "" && params.SmartTakeLoss {
		reason = smarttakeloss.AddFreeze(event, position, indicators)
	}
	if reason == "" && params.Cooldown {
		// Last, and only when nothing else spoke: depth spacing has no view of
		// the market at all, so every gate above names the reason for a hold
		// better than "the last depths were close together" ever could.
		reason = cooldown.DepthSpacingHoldReason(event, position)
	}

	if reason != "" {
		return gates.SaveHoldLog(event, event.Trade.PositionType, reason)
	}
	return event, nil
}
