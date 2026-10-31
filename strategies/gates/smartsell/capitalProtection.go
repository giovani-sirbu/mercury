package smartsell

import (
	"time"

	"github.com/giovani-sirbu/mercury/strategies/gates"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/capitalprotection"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
	"github.com/giovani-sirbu/mercury/trades/tradelog"
)

// capitalProtectionWatched is whether the gate watches a trade (capitalProtectionWatches on its fills).
func capitalProtectionWatched(trade aggragates.Trades) bool {
	return capitalProtectionWatches(trade, entryFills(trade))
}

// capitalProtectionWatches is whether the gate watches the trade with these fills:
// capitalProtectionEligible, its last configured depth filled (lastDepthFilled) and the next entry
// refused for insufficient funds since the newest fill (fundsRefusedSince).
func capitalProtectionWatches(trade aggragates.Trades, fills []entryFill) bool {
	if !capitalProtectionEligible(trade) || !lastDepthFilled(trade, len(fills)) {
		return false
	}
	return fundsRefusedSince(trade, fills[len(fills)-1].At)
}

// fundsRefusedSince is whether the trade logged an insufficient funds error stamped at or after from.
func fundsRefusedSince(trade aggragates.Trades, from time.Time) bool {
	for _, entry := range trade.Logs {
		if entry.Type == aggragates.LOG_ERROR && tradelog.HasInsufficientBalance(entry.Message) && !entry.CreatedAt.Before(from) {
			return true
		}
	}
	return false
}

// capitalProtectionEligible is the part of the watch the fills do not decide: capitalprotection.Enabled,
// a spot ladder, long or inverse, and no Impasse strategy (whose parent reaches buy again with
// children a forced sale would orphan).
func capitalProtectionEligible(trade aggragates.Trades) bool {
	return capitalProtectionExit &&
		trade.Strategy.TradeType != aggragates.Futures &&
		!trade.Strategy.Params.Impasse
}

// lastDepthFilled is whether the ladder filled its last configured depth: ladder.ConfiguredDepths
// known and at least that many entries filled.
func lastDepthFilled(trade aggragates.Trades, filled int) bool {
	depths := ladder.ConfiguredDepths(trade)
	return depths > 0 && filled >= depths
}

// capitalProtectionTick is the gate's part of a tick at now on a watched ladder: the state with its
// episode, the episode's rows and whether its trigger stands. An open episode is sticky: a new fill
// past the ceiling resets it (CapitalProtectionResetRow) and the trend is read afresh; none open,
// the trend reading opens one (CapitalProtectionTriggeredRow). The trigger stands once the wait
// after the trigger has passed (capitalProtectionWaiting).
func capitalProtectionTick(trade aggragates.Trades, st state, block aggragates.SmartSellIndicators, now time.Time) (next state, rows []Row, holds bool) {
	fill := st.lastFill().Price
	if st.capitalProtectionActive && fill != st.capitalProtectionFrom {
		rows = append(rows, CapitalProtectionResetRow(fill))
		st = st.capitalProtectionClosed()
	}
	if st.capitalProtectionActivated && gates.PositionType(trade.PositionType) != st.capitalProtectionActivatedIn {
		st.capitalProtectionActivated, st.capitalProtectionActivatedIn = false, ""
	}
	if !st.capitalProtectionActive {
		if !capitalprotection.Triggered(trade.Inverse, block) {
			return st, rows, false
		}
		rows = append(rows, CapitalProtectionTriggeredRow(fill, len(st.fills), ladder.ConfiguredDepths(trade), trade.Inverse))
		st = st.capitalProtectionOpened(fill)
		st.capitalProtectionTriggeredAt = now
	}
	return st, rows, !capitalProtectionWaiting(st, now)
}

// capitalProtectionWaiting is whether the wait after the trigger (capitalprotection.WaitAfterTrigger)
// still holds the trigger back: the clock known, the episode opened by a triggered event and not
// yet activated (a committed sale trails whatever), and less than the wait passed since its stamp.
func capitalProtectionWaiting(st state, now time.Time) bool {
	if now.IsZero() || st.capitalProtectionTriggeredAt.IsZero() || st.capitalProtectionActivated || capitalProtectionWaitAfterTrigger <= 0 {
		return false
	}
	return now.Sub(st.capitalProtectionTriggeredAt) < capitalProtectionWaitAfterTrigger
}

// capitalProtectionTrigger is the gate's trigger as Apply hands it to the shared action, on its own
// terms: its held row by the profit due through heldUnder, its gap row through gapRowDue.
func capitalProtectionTrigger(st state) saleTrigger {
	return saleTrigger{
		reason:      reasonCapitalProtection,
		gate:        GateCapitalProtection,
		heldUnder:   st.capitalProtectionHeldUnder,
		reachedAt:   st.capitalProtectionReachedAt,
		gapRowDue:   st.capitalProtectionGapHeldAt.IsZero(),
		gapHeldAt:   st.capitalProtectionGapHeldAt,
		activated:   st.capitalProtectionActivated,
		activatedIn: st.capitalProtectionActivatedIn,
		terms:       capitalProtectionTerms,
	}
}

// capitalProtectionOpened is the state with the episode open at fill: nothing held, nothing
// activated, no trigger or reached stamp, no gap row yet.
func (st state) capitalProtectionOpened(fill float64) state {
	st.capitalProtectionActive, st.capitalProtectionSaleHeld, st.capitalProtectionActivated = true, false, false
	st.capitalProtectionActivatedIn = ""
	st.capitalProtectionHeldUnder, st.capitalProtectionReachedAt, st.capitalProtectionGapHeldAt = heldTrigger{}, time.Time{}, time.Time{}
	st.capitalProtectionFrom, st.capitalProtectionTriggeredAt = fill, time.Time{}

	return st
}

// capitalProtectionClosed is the state with the episode closed.
func (st state) capitalProtectionClosed() state {
	st.capitalProtectionActive, st.capitalProtectionSaleHeld, st.capitalProtectionActivated = false, false, false
	st.capitalProtectionActivatedIn = ""
	st.capitalProtectionHeldUnder, st.capitalProtectionReachedAt, st.capitalProtectionGapHeldAt = heldTrigger{}, time.Time{}, time.Time{}
	st.capitalProtectionFrom, st.capitalProtectionTriggeredAt = 0, time.Time{}

	return st
}

// foldCapitalProtectionEvents is the state with the episode read off the gate's events in slice
// order; events that do not decode are skipped (readEvent).
func (st state) foldCapitalProtectionEvents(trade aggragates.Trades) state {
	for _, event := range trade.StrategyEvents {
		if event.Param != aggragates.StrategyParamSmartSell || event.Gate != GateCapitalProtection {
			continue
		}
		if data, ok := readEvent(event); ok {
			st = st.foldCapitalProtection(data, event.CreatedAt)
		}
	}
	return st
}

// foldCapitalProtection is the state after one event of the gate stamped at: triggered opens and stamps
// the episode, the shared action's kinds join it and set their flags or stamp, reset and sold close it.
func (st state) foldCapitalProtection(data EventData, at time.Time) state {
	switch data.Event {
	case EventTriggered:
		st = st.capitalProtectionOpened(data.Price)
		st.capitalProtectionTriggeredAt = at
	case EventReached:
		st = st.capitalProtectionJoined(data.Price)
		st.capitalProtectionReachedAt = at
	case EventSaleHeld:
		st = st.capitalProtectionJoined(data.Price)
		st.capitalProtectionSaleHeld, st.capitalProtectionHeldUnder = true, heldTriggerOf(data)
		st.capitalProtectionReachedAt = time.Time{}
	case EventActivated:
		st = st.capitalProtectionJoined(data.Price)
		st.capitalProtectionActivated, st.capitalProtectionGapHeldAt, st.capitalProtectionReachedAt = true, time.Time{}, time.Time{}
		st.capitalProtectionActivatedIn = data.State
	case EventGapHeld, EventGapClosed:
		st = st.capitalProtectionJoined(data.Price)
		st.capitalProtectionGapHeldAt = gapStamp(data.Event, at)
	case EventReset, EventSold:
		st = st.capitalProtectionClosed()
	}
	return st
}

// capitalProtectionJoined is the state with the episode open, opened at price when none was.
func (st state) capitalProtectionJoined(price float64) state {
	if !st.capitalProtectionActive {
		st = st.capitalProtectionOpened(price)
	}

	return st
}
