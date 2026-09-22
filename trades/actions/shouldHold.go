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
)

// ShouldHold blocks the action chain when a strategy flag advises against
// acting. Holds are recorded by gates.SaveHoldLog as INFO trade-log entries.
//
// OWNERSHIP. Every gate answers to exactly one strategy flag, flag first and
// payload second: `params.X && <payload present>`. Payload presence is only a
// degrade-open check, never a switch-on — a verdict fetched for one flag's
// sake gains no gate for another (see StrategyParams.NeedsSophos).
//
//	first fill (OldPosition "new")   Cooldown  → depth priority, then the first-fill gate (higher-highs hold, released by price)
//	                                 UseAI     → legacy bullish/bearish veto
//	open position                    RegimeHold    → shock hold, add veto, profit hold
//	                                 UsePatterns   → chart-pattern and fibonacci holds
//	                                 UseAI         → legacy AI hold
//	                                 CrashGuard    → flush park, sticky reclaim, capitulation
//	                                 Cooldown      → depth priority, then depth spacing (stopLoss only)
//
// SmartTakeLoss owns no hold gate: it forces exits after the ladder decides
// (gates/smarttakeloss.Apply, called by the engines).
//
// Each family lives in its own package under trades/gates; this function
// only orders them. Cooldown owns THREE gates. They share a flag because they
// are the same idea — do not spend capital faster than the move deserves —
// but nothing else, and each reads something different:
//
//   - the first-fill gate decides whether the trade opens here at all. It
//     takes one higher-highs verdict from sophos /cooldown to activate and
//     from then on reads only the tick price and its own log rows;
//   - depth spacing keeps one ladder from cascading through every depth in
//     one drop. It reads only that trade's own fill stamps;
//   - depth priority keeps the ladders of one wallet from all stopping
//     half-finished when the pairs fall together. It reads the depths of the
//     wallet's other ladders and nothing about the market at all.
//
// Depth priority runs on both sides of the first fill, and first on each:
// its subject is the wallet, so a trade it holds spends nothing and needs no
// verdict from the gate that would have spoken next.
//
// With every flag off nothing holds: the ladder runs exactly as the legacy
// engine ran it, stopped only by funds.
//
// RegimeHold never reaches the first fill: the regime entry veto was removed
// because a regime read was wrong about a first fill far more often than it
// was right; the first fill is the cooldown's. Whether the first-buy chain
// runs this function at all is the engines' call through
// StrategyParams.InjectsEntryHold.
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
		// The wallet before the entry: while a sibling ladder is short of the
		// depth its grid was sized for, the wallet is reserved for that
		// entry, and a first fill is new capital like any other. It runs
		// before the first-fill gate so a held entry neither consumes nor
		// records a first-fill verdict — that judgement belongs to the tick
		// the wallet is actually free on.
		if reason := cooldown.DepthPriorityHoldReason(event, event.Trade.PositionType); reason != "" {
			return gates.SaveHoldLog(event, "entry", reason)
		}

		// The gate hands the event back: on the tick it releases an entry
		// above its reference it has written the row NextDepthDoubled reads,
		// and only the event that continues down the chain reaches updateTrade.
		var reason string
		event, reason = cooldown.FirstFillHold(event, side)
		if reason != "" {
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

	if reason == "" && params.Cooldown {
		// The wallet before the ladder. Neither cooldown gate has a view of
		// the market, so nothing above is being displaced; between the two of
		// them, "a deeper ladder of this wallet takes the next entry" names
		// the situation an operator is looking at, and "the last depths were
		// close together" does not.
		reason = cooldown.DepthPriorityHoldReason(event, position)
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
