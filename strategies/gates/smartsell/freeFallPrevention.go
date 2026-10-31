package smartsell

import (
	"time"

	"github.com/giovani-sirbu/mercury/strategies/gates"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/freefallprevention"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// freeFallPreventionEligible is the part of the watch the fills do not decide:
// freefallprevention.Enabled and a spot ladder, long or inverse.
func freeFallPreventionEligible(trade aggragates.Trades) bool {
	return freeFallPrevention && trade.Strategy.TradeType != aggragates.Futures
}

// freeFallPreventionWatched is whether the gate watches a trade: freeFallPreventionEligible and at
// least freefallprevention.ArmDepth filled entries; before that it only holds the entry (EntryHold).
func freeFallPreventionWatched(trade aggragates.Trades) bool {
	return freeFallPreventionEligible(trade) && ladder.CountFilledEntries(trade) >= freefallprevention.ArmDepth
}

// freeFallPreventionReading is freefallprevention.Trigger on the block for the ladder's entry side
// (aggragates.EntrySide); asked only while no episode is open.
func freeFallPreventionReading(trade aggragates.Trades, block aggragates.SmartSellIndicators, ai aggragates.AIIndicators) freefallprevention.Reading {
	return freefallprevention.Trigger(block, aggragates.EntrySide(trade, ai))
}

// freeFallPreventionTick is the gate's part of a tick on a watched ladder: the state with its
// episode, the episode's rows and whether its trigger stands. An open episode is sticky: with
// freefallprevention.ResetOnNewDepth a new depth resets it (FreeFallPreventionResetRow) and the
// reading is read afresh, else it stands through new fills; none open, the reading opens one
// (FreeFallPreventionTriggeredRow) unless another sale gate's episode is open (saleEpisodeOpenBesides).
func freeFallPreventionTick(trade aggragates.Trades, st state, block aggragates.SmartSellIndicators, ai aggragates.AIIndicators) (next state, rows []Row, holds bool) {
	fill := st.lastFill().Price
	if st.freeFallPreventionActive && fill != st.freeFallPreventionFrom && freeFallPreventionResetOnNewDepth {
		rows = append(rows, FreeFallPreventionResetRow(fill))
		st = st.freeFallPreventionClosed()
	}
	if st.freeFallPreventionActivated && gates.PositionType(trade.PositionType) != st.freeFallPreventionActivatedIn {
		st.freeFallPreventionActivated, st.freeFallPreventionActivatedIn = false, ""
	}
	if st.freeFallPreventionActive {
		return st, rows, true
	}
	if st.saleEpisodeOpenBesides(GateFreeFallPrevention) {
		return st, rows, false
	}

	reading := freeFallPreventionReading(trade, block, ai)
	if !reading.Holds {
		return st, rows, false
	}
	rows = append(rows, FreeFallPreventionTriggeredRow(fill, reading.Reasons))
	return st.freeFallPreventionOpened(fill), rows, true
}

// freeFallPreventionOpened is the state with the episode open at fill: nothing held, nothing
// activated, no trigger or reached stamp, no gap row yet.
func (st state) freeFallPreventionOpened(fill float64) state {
	st.freeFallPreventionActive, st.freeFallPreventionSaleHeld, st.freeFallPreventionActivated = true, false, false
	st.freeFallPreventionActivatedIn = ""
	st.freeFallPreventionHeldUnder, st.freeFallPreventionReachedAt, st.freeFallPreventionGapHeldAt = heldTrigger{}, time.Time{}, time.Time{}
	st.freeFallPreventionFrom, st.freeFallPreventionTriggeredAt = fill, time.Time{}

	return st
}

// freeFallPreventionClosed is the state with the episode closed.
func (st state) freeFallPreventionClosed() state {
	st.freeFallPreventionActive, st.freeFallPreventionSaleHeld, st.freeFallPreventionActivated = false, false, false
	st.freeFallPreventionActivatedIn = ""
	st.freeFallPreventionHeldUnder, st.freeFallPreventionReachedAt, st.freeFallPreventionGapHeldAt = heldTrigger{}, time.Time{}, time.Time{}
	st.freeFallPreventionFrom, st.freeFallPreventionTriggeredAt = 0, time.Time{}

	return st
}

// foldFreeFallPreventionEvents is the state with the episode read off the gate's events in slice
// order; held events, which carry no price, and events that do not decode are skipped (readEvent).
func (st state) foldFreeFallPreventionEvents(trade aggragates.Trades) state {
	for _, event := range trade.StrategyEvents {
		if event.Param != aggragates.StrategyParamSmartSell || event.Gate != GateFreeFallPrevention {
			continue
		}
		if data, ok := readEvent(event); ok {
			st = st.foldFreeFallPrevention(data, event.CreatedAt)
		}
	}
	return st
}

// foldFreeFallPrevention is the state after one event of the gate stamped at: triggered opens and stamps
// the episode, the shared action's kinds join it and set their flags or stamp, reset and sold close it.
func (st state) foldFreeFallPrevention(data EventData, at time.Time) state {
	switch data.Event {
	case EventTriggered:
		st = st.freeFallPreventionOpened(data.Price)
		st.freeFallPreventionTriggeredAt = at
	case EventReached:
		st = st.freeFallPreventionJoined(data.Price)
		st.freeFallPreventionReachedAt = at
	case EventSaleHeld:
		st = st.freeFallPreventionJoined(data.Price)
		st.freeFallPreventionSaleHeld, st.freeFallPreventionHeldUnder = true, heldTriggerOf(data)
		st.freeFallPreventionReachedAt = time.Time{}
	case EventActivated:
		st = st.freeFallPreventionJoined(data.Price)
		st.freeFallPreventionActivated, st.freeFallPreventionGapHeldAt, st.freeFallPreventionReachedAt = true, time.Time{}, time.Time{}
		st.freeFallPreventionActivatedIn = data.State
	case EventGapHeld, EventGapClosed:
		st = st.freeFallPreventionJoined(data.Price)
		st.freeFallPreventionGapHeldAt = gapStamp(data.Event, at)
	case EventReset, EventSold:
		st = st.freeFallPreventionClosed()
	}
	return st
}

// freeFallPreventionJoined is the state with the episode open, opened at price when none was.
func (st state) freeFallPreventionJoined(price float64) state {
	if !st.freeFallPreventionActive {
		st = st.freeFallPreventionOpened(price)
	}

	return st
}
