package smarttakeloss

import (
	"slices"
	"time"

	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/earlytakeprofit"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/flatmarket"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/freefallprevention"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/lineardecline"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/quietdecline"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/swingdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// state is what the trade's own strategy events and history say about the smart take loss, rebuilt
// on every tick (rebuildState): the events are the only state.
type state struct {
	fills []entryFill
	// The watches (quietDeclineWatched, swingDeclineWatched, linearDeclineWatched; slowDeclineWatched
	// is any decline's).
	quietDeclineWatched  bool
	swingDeclineWatched  bool
	linearDeclineWatched bool
	slowDeclineWatched   bool
	// slowDeclinePending is a pending event not followed by a cancelled or reset one;
	// slowDeclinePendingFrom its price, the fill the ladder is pending from; pendingFeatures the
	// decline features it is pending on, in row order; slowDeclineTriggeredAt the stamp of the
	// pending event it is pending on, which WaitAfterTrigger runs from, read only while pending.
	slowDeclinePending     bool
	slowDeclinePendingFrom float64
	pendingFeatures        []string
	slowDeclineTriggeredAt time.Time
	// saleHeld is the shared sale held in this pending episode (EventSaleHeld), saleHeldUnder the
	// trigger its newest held row was held under, saleReachedAt the stamp of the reached event the
	// activation confirms from (EventReached); slowDeclineActivated the stopLoss activated on the
	// gate's trigger (EventActivated), slowDeclineGapHeldAt the stamp of its open gap (EventGapHeld); pending,
	// cancelled, reset and sold events clear them.
	saleHeld             bool
	saleHeldUnder        heldTrigger
	saleReachedAt        time.Time
	slowDeclineActivated bool
	slowDeclineGapHeldAt time.Time
	// capitalProtectionWatched is the capital protection gate's watch; capitalProtectionActive its
	// open episode, capitalProtectionSaleHeld and capitalProtectionActivated the held sale and the
	// activation in it, never set on a closed episode, capitalProtectionHeldUnder the trigger the
	// newest held row was held under; capitalProtectionFrom the fill the episode opened at, which a
	// new depth is told from; capitalProtectionTriggeredAt the stamp of the triggered event that
	// opened it, which the wait after the trigger runs from, zero when a held or activated event
	// opened it; capitalProtectionReachedAt as saleReachedAt, capitalProtectionGapHeldAt as slowDeclineGapHeldAt.
	capitalProtectionWatched     bool
	capitalProtectionActive      bool
	capitalProtectionSaleHeld    bool
	capitalProtectionHeldUnder   heldTrigger
	capitalProtectionReachedAt   time.Time
	capitalProtectionActivated   bool
	capitalProtectionGapHeldAt   time.Time
	capitalProtectionFrom        float64
	capitalProtectionTriggeredAt time.Time
	// earlyTakeProfitWatched is the early take profit's watch; earlyTakeProfit the latch, which nothing takes away.
	earlyTakeProfitWatched bool
	earlyTakeProfit        bool
	// depthPriorityHeld is the pause a depth priority puts on the gates until the next fill.
	depthPriorityHeld bool
	// flatMarketWatched is the flat market gate's watch; flatMarketActive its open episode,
	// flatMarketSaleHeld and flatMarketActivated the held sale and the activation in it, never set
	// on a closed episode, flatMarketHeldUnder as capitalProtectionHeldUnder; flatMarketFrom the
	// price the episode opened at, which a new depth is told from; flatMarketTriggeredAt the stamp
	// of the triggered event that opened it, which WaitAfterTrigger runs from, zero when a held or
	// activated event opened it; flatMarketReachedAt as saleReachedAt, flatMarketGapHeldAt as slowDeclineGapHeldAt.
	flatMarketWatched     bool
	flatMarketActive      bool
	flatMarketSaleHeld    bool
	flatMarketHeldUnder   heldTrigger
	flatMarketReachedAt   time.Time
	flatMarketActivated   bool
	flatMarketGapHeldAt   time.Time
	flatMarketFrom        float64
	flatMarketTriggeredAt time.Time
	// freeFallPreventionWatched is the free fall prevention gate's watch; freeFallPreventionActive
	// its open episode, freeFallPreventionSaleHeld and freeFallPreventionActivated the held sale and
	// the activation in it, never set on a closed episode, freeFallPreventionHeldUnder as
	// capitalProtectionHeldUnder; freeFallPreventionFrom the price the episode opened at, which a
	// new depth is told from; freeFallPreventionTriggeredAt as flatMarketTriggeredAt,
	// freeFallPreventionReachedAt as saleReachedAt, freeFallPreventionGapHeldAt as slowDeclineGapHeldAt.
	freeFallPreventionWatched     bool
	freeFallPreventionActive      bool
	freeFallPreventionSaleHeld    bool
	freeFallPreventionHeldUnder   heldTrigger
	freeFallPreventionReachedAt   time.Time
	freeFallPreventionActivated   bool
	freeFallPreventionGapHeldAt   time.Time
	freeFallPreventionFrom        float64
	freeFallPreventionTriggeredAt time.Time
}

// rebuildState folds the trade's smartTakeLoss events in slice order: featured gate events drive
// the pending state, the latch and the sale's memory (foldGateEvent), legacy events read as the
// gates they were written under (legacyStates), the flat market, free fall prevention and capital
// protection gates' apart (foldFlatMarketEvents, foldFreeFallPreventionEvents,
// foldCapitalProtectionEvents).
func rebuildState(trade aggragates.Trades) state {
	fills := entryFills(trade)
	st := state{
		fills:                     fills,
		quietDeclineWatched:       quietDeclineEligible(trade) && len(fills) >= quietdecline.ArmDepth,
		swingDeclineWatched:       swingDeclineEligible(trade) && len(fills) >= swingdecline.ArmDepth,
		linearDeclineWatched:      linearDeclineEligible(trade) && len(fills) >= lineardecline.ArmDepth,
		capitalProtectionWatched:  capitalProtectionWatches(trade, fills),
		earlyTakeProfitWatched:    earlyTakeProfitEligible(trade) && len(fills) >= earlytakeprofit.ArmDepth,
		flatMarketWatched:         flatMarketEligible(trade) && len(fills) >= flatmarket.ArmDepth,
		freeFallPreventionWatched: freeFallPreventionEligible(trade) && len(fills) >= freefallprevention.ArmDepth,
	}
	st.slowDeclineWatched = st.quietDeclineWatched || st.swingDeclineWatched || st.linearDeclineWatched
	st.depthPriorityHeld = depthPriorityHeld(trade, st.lastFill())
	if st.flatMarketWatched {
		st = st.foldFlatMarketEvents(trade)
	}
	if st.freeFallPreventionWatched {
		st = st.foldFreeFallPreventionEvents(trade)
	}
	if st.capitalProtectionWatched {
		st = st.foldCapitalProtectionEvents(trade)
	}
	if !st.slowDeclineWatched && !st.earlyTakeProfitWatched {
		return st
	}
	var legacy legacyStates
	for index, event := range trade.StrategyEvents {
		if event.Param != aggragates.StrategyParamSmartTakeLoss {
			continue
		}
		data, ok := readEvent(event)
		if !ok {
			continue
		}
		switch {
		case event.Gate == GateSlowDecline && len(data.Features) > 0:
			st, legacy = foldGateEvent(st, legacy, data, event.CreatedAt)
		case event.Gate == GateSlowDecline && st.quietDeclineWatched:
			legacy = legacy.foldQuiet(data, index)
		case event.Gate == gateSlowPattern && st.swingDeclineWatched:
			legacy = legacy.foldSwing(data, index)
		case event.Gate == gateIndecision && st.earlyTakeProfitWatched && data.Event == EventLatched:
			st.earlyTakeProfit = true
		}
	}
	return legacy.settle(st)
}

// foldGateEvent is the state after one featured slow decline event stamped at: the pending kinds move
// the pending state, reached stamps the sale, the sale kinds move its memory (afterSale), the latch latches.
func foldGateEvent(st state, legacy legacyStates, data EventData, at time.Time) (state, legacyStates) {
	switch data.Event {
	case EventPending:
		st = st.notPending()
		if declines := watchedDeclines(st, data.Features); len(declines) > 0 {
			st.slowDeclinePending, st.slowDeclinePendingFrom, st.pendingFeatures = true, data.Price, declines
			st.slowDeclineTriggeredAt = at
		}
		legacy = legacyStates{superseded: true}
	case EventCancelled, EventReset:
		st = st.notPending()
		legacy = legacyStates{superseded: true}
	case EventReached:
		st.saleReachedAt = at
		return st, legacy
	case EventSaleHeld, EventActivated, EventSold, EventGapHeld, EventGapClosed:
		return st.afterSale(data, at), legacy
	case EventLatched:
	default:
		return st, legacy
	}
	// FeatureIndecision: the latch's own name before it was the early take profit.
	if st.earlyTakeProfitWatched && (data.Event == EventLatched || slices.Contains(data.Features, FeatureEarlyTakeProfit) || slices.Contains(data.Features, FeatureIndecision)) {
		st.earlyTakeProfit = true
	}
	return st, legacy
}

// watchedDeclines are the decline features among those an event names that watch the ladder, in
// the order the rows name them.
func watchedDeclines(st state, features []string) []string {
	var declines []string
	if st.quietDeclineWatched && slices.Contains(features, FeatureQuietDecline) {
		declines = append(declines, FeatureQuietDecline)
	}
	if st.swingDeclineWatched && slices.Contains(features, FeatureSwingDecline) {
		declines = append(declines, FeatureSwingDecline)
	}
	if st.linearDeclineWatched && slices.Contains(features, FeatureLinearDecline) {
		declines = append(declines, FeatureLinearDecline)
	}
	return declines
}
