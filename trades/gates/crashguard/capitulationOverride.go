package crashguard

import (
	"strings"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/regime"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

const (
	// CapitulationDisplacementSteps is how many unwidened grid steps below
	// (long) or above (inverse) the last fill count as a capitulation dump.
	CapitulationDisplacementSteps = 8
	// CapitulationClearQuietWindows is how many consecutive 15m sophos
	// windows without adverse shock clear a freeze when crash never armed.
	CapitulationClearQuietWindows = 2

	// CapitulationFreezeHold is the hold reason written while the one
	// capitulation add of an episode is already taken.
	CapitulationFreezeHold = "capitulation: freeze, one add already taken"

	CapitulationTaggedPrefix    = "Capitulation tagged"
	CapitulationAllowedPrefix   = "Capitulation add allowed"
	CapitulationFreezeOnPrefix  = "Capitulation freeze on"
	CapitulationFreezeOffPrefix = "Capitulation freeze off"
)

// ApplyCapitulationOverride is the stopLoss-only exception: a shallow
// 8-step dump that has printed a 5m reclaim may bypass a 15m shock or
// add-veto hold so the grid can take one extra fill. Crash-deep, STL
// freeze, and an already-taken shot in this episode are never bypassed.
// Owned by the CrashGuard flag: the caller runs it only under
// params.CrashGuard, so ai.CrashActive is read here on a crash-guard
// strategy only. `position` is the gate-normalised position (a force-trailing
// re-anchor reads as the rung it re-arms); the raw PositionType decides
// whether the live episode survives.
func ApplyCapitulationOverride(event events.Events, position string, ai aggragates.AIIndicators, holdReason string) (events.Events, string) {
	if !capitulationApplies(event, position) {
		// Leaving stopLoss (a profit arm, a close) ends the live episode
		// bookkeeping; the durable state stays in the trade logs. A
		// force-trailing re-anchor is NOT leaving the ladder: those states
		// only move PositionPrice and hand the rung back to stopLoss, and
		// clearing here wiped hadCrash and the quiet-window counter on every
		// ratchet — 263 times in run 97, exactly on the deep, falling trades
		// the episode exists for.
		if position != "stopLoss" && !capitulationEpisodeContinues(event.Trade.PositionType) {
			clearCapitulationLive(event.Trade.ID)
		}
		return event, holdReason
	}
	// The override only ever REFUSES or BYPASSES an existing hold. A tick the
	// gates already approved is never turned into a hold here: a stale tag
	// or a consumed episode has nothing to freeze when there is no hold to
	// bypass, and inventing one parks the normal grid.
	if holdReason == "" {
		return event, ""
	}
	ep := rebuildCapitulationEpisode(event.Trade)
	// A flush seen DURING an episode is what freezeClearReason waits out; a
	// crash flag before any episode exists must not leak into a later one.
	if ai.CrashActive && (ep.tagged || ep.freezing) {
		ep.hadCrash = true
		markCapitulationCrash(event.Trade.ID)
	}
	if (ep.tagged || ep.freezing) && liveHadCrash(event.Trade.ID) {
		ep.hadCrash = true
	}

	event, holdReason, frozen := resolveCapitulationFreeze(event, ai, ep, holdReason)
	if frozen {
		return event, holdReason
	}
	return maybeCapitulationAdd(event, ai, holdReason)
}

func capitulationApplies(event events.Events, position string) bool {
	if event.Params.OldPosition == "new" || event.Trade.ParentID != 0 {
		return false
	}
	switch position {
	case "smartTakeLoss", "smartTakeLossTrail", "sellLoss":
		return false
	}
	return position == "stopLoss"
}

func resolveCapitulationFreeze(event events.Events, ai aggragates.AIIndicators, ep capitulationEpisode, holdReason string) (events.Events, string, bool) {
	if !ep.freezing && !ep.consumed {
		return event, holdReason, false
	}
	if ep.freezing {
		if reason, ok := freezeClearReason(event, ai, ep); ok {
			event = logCapitulation(event, CapitulationFreezeOffPrefix+" ("+reason+")")
			clearCapitulationLive(event.Trade.ID)
			return event, holdReason, false
		}
		event = logCapitulation(event, CapitulationFreezeOnPrefix)
		return event, CapitulationFreezeHold, true
	}
	event = logCapitulation(event, CapitulationFreezeOnPrefix)
	return event, CapitulationFreezeHold, true
}

func maybeCapitulationAdd(event events.Events, ai aggragates.AIIndicators, holdReason string) (events.Events, string) {
	if holdReason == "" {
		return event, ""
	}
	if keepCapitulationHold(holdReason) {
		return event, holdReason
	}
	if !capitulationEligibleHold(holdReason) {
		return event, holdReason
	}
	filled := ladder.CountFilledEntries(event.Trade)
	if filled >= DeRiskMinDepth {
		return event, holdReason
	}
	lastFill, ok := lastFilledEntryPrice(event.Trade)
	if !ok || !capitulationDisplaced(event.Trade, event.Trade.PositionPrice, lastFill, filled) {
		return event, holdReason
	}
	event = logCapitulation(event, taggedMessage(lastFill, filled, ai.CrashActive))
	if !capitulationReclaim(event, lastFill) {
		return event, holdReason
	}
	event = logCapitulation(event, allowedMessage(lastFill, filled))
	return event, ""
}

// keepCapitulationHold: the deep flush park (DeepHoldReason) and the smart
// take loss HTF freeze are never bypassed. The smart-take-loss text is a
// literal on purpose: smarttakeloss imports this package, so this package
// cannot import its constant. actions.TestHoldReasonContractAcrossFamilies
// pins both sides.
func keepCapitulationHold(reason string) bool {
	return strings.Contains(reason, "crash-guard: deep") ||
		strings.Contains(reason, "smart-take-loss: HTF")
}

func capitulationEligibleHold(reason string) bool {
	return strings.Contains(reason, regime.ShockHoldPrefix) ||
		strings.HasPrefix(reason, regime.AddVetoPrefix) ||
		strings.HasPrefix(reason, regime.InverseAddVetoPrefix)
}
