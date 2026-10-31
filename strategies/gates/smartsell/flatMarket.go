package smartsell

import (
	"time"

	"github.com/giovani-sirbu/mercury/strategies/gates"
	"github.com/giovani-sirbu/mercury/strategies/gates/dynamicparams"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/flatmarket"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// flatMarketEligible is the part of the watch the fills do not decide: flatmarket.Enabled and a
// spot ladder, long or inverse.
func flatMarketEligible(trade aggragates.Trades) bool {
	return flatMarket && trade.Strategy.TradeType != aggragates.Futures
}

// flatMarketWatched is whether the gate watches a trade: flatMarketEligible and at least
// flatmarket.ArmDepth filled entries; before that it only holds the entry (EntryHold).
func flatMarketWatched(trade aggragates.Trades) bool {
	return flatMarketEligible(trade) && ladder.CountFilledEntries(trade) >= flatmarket.ArmDepth
}

// basePercentage is the percentage of the base row the ladder trades (dynamicparams.RaisedSettings:
// the pair's row, raised once dynamic params opened the ladder raised); zero without settings, which
// the readings fail closed on.
func basePercentage(trade aggragates.Trades) float64 {
	settings, _ := dynamicparams.RaisedSettings(trade)
	if len(settings) == 0 {
		return 0
	}
	return settings[ladder.SettingsIndexOrBase(settings, 0)].Percentage
}

// flatMarketReading is flatmarket.Trigger on the block's bars, the ladder's base percentage, its
// entry side (aggragates.EntrySide) and the block's SMC trend; asked only while no episode is open.
func flatMarketReading(trade aggragates.Trades, block aggragates.SmartSellIndicators, ai aggragates.AIIndicators) flatmarket.Reading {
	return flatmarket.Trigger(flatmarket.BarsOf(block), basePercentage(trade), aggragates.EntrySide(trade, ai), series.TrendOf(block))
}

// flatMarketTick is the gate's part of a tick on a watched ladder: the state with its episode, the
// episode's rows and whether its trigger stands. An open episode is sticky: with
// flatmarket.ResetOnNewDepth a new depth resets it (FlatMarketResetRow) and the reading is read
// afresh, else it stands through new fills; none open, the reading opens one (FlatMarketTriggeredRow)
// unless another sale gate's episode is open (saleEpisodeOpenBesides).
func flatMarketTick(trade aggragates.Trades, st state, block aggragates.SmartSellIndicators, ai aggragates.AIIndicators) (next state, rows []Row, holds bool) {
	fill := st.lastFill().Price
	if st.flatMarketActive && fill != st.flatMarketFrom && flatMarketResetOnNewDepth {
		rows = append(rows, FlatMarketResetRow(fill))
		st = st.flatMarketClosed()
	}
	if st.flatMarketActivated && gates.PositionType(trade.PositionType) != st.flatMarketActivatedIn {
		st.flatMarketActivated, st.flatMarketActivatedIn = false, ""
	}
	if st.flatMarketActive {
		return st, rows, true
	}
	if st.saleEpisodeOpenBesides(GateFlatMarket) {
		return st, rows, false
	}

	reading := flatMarketReading(trade, block, ai)
	if !reading.Holds {
		return st, rows, false
	}
	rows = append(rows, FlatMarketTriggeredRow(fill, reading.Reasons))
	return st.flatMarketOpened(fill), rows, true
}

// flatMarketOpened is the state with the episode open at fill: nothing held, nothing activated,
// no trigger or reached stamp, no gap row yet.
func (st state) flatMarketOpened(fill float64) state {
	st.flatMarketActive, st.flatMarketSaleHeld, st.flatMarketActivated, st.flatMarketActivatedIn = true, false, false, ""
	st.flatMarketHeldUnder, st.flatMarketReachedAt, st.flatMarketGapHeldAt = heldTrigger{}, time.Time{}, time.Time{}
	st.flatMarketFrom, st.flatMarketTriggeredAt = fill, time.Time{}

	return st
}

// flatMarketClosed is the state with the episode closed.
func (st state) flatMarketClosed() state {
	st.flatMarketActive, st.flatMarketSaleHeld, st.flatMarketActivated, st.flatMarketActivatedIn = false, false, false, ""
	st.flatMarketHeldUnder, st.flatMarketReachedAt, st.flatMarketGapHeldAt = heldTrigger{}, time.Time{}, time.Time{}
	st.flatMarketFrom, st.flatMarketTriggeredAt = 0, time.Time{}

	return st
}

// foldFlatMarketEvents is the state with the episode read off the gate's events in slice order, the
// legacy gates' among them, a triggered, reached or gap held event stamping it; events readEvent skips are skipped.
func (st state) foldFlatMarketEvents(trade aggragates.Trades) state {
	for _, event := range trade.StrategyEvents {
		if event.Param != aggragates.StrategyParamSmartSell || !flatMarketGate(event.Gate) {
			continue
		}
		if data, ok := readEvent(event); ok {
			st = st.foldFlatMarket(data)
			switch data.Event {
			case EventTriggered:
				st.flatMarketTriggeredAt = event.CreatedAt
			case EventReached:
				st.flatMarketReachedAt = event.CreatedAt
			case EventGapHeld:
				st.flatMarketGapHeldAt = event.CreatedAt
			}
		}
	}
	return st
}

// flatMarketGate is whether an event's gate is the flat market's: its own, or a legacy one.
func flatMarketGate(gate string) bool {
	return gate == GateFlatMarket || gate == gateLowInterest || gate == gateNonInterestArea
}

// foldFlatMarket is the state after one event of the gate: triggered opens the episode, the shared
// action's kinds join it and set their flags, reset, sold and ended close it; foldFlatMarketEvents stamps.
func (st state) foldFlatMarket(data EventData) state {
	switch data.Event {
	case EventTriggered:
		st = st.flatMarketOpened(data.Price)
	case EventReached:
		st = st.flatMarketJoined(data.Price)
	case EventSaleHeld:
		st = st.flatMarketJoined(data.Price)
		st.flatMarketSaleHeld, st.flatMarketHeldUnder, st.flatMarketReachedAt = true, heldTriggerOf(data), time.Time{}
	case EventActivated:
		st = st.flatMarketJoined(data.Price)
		st.flatMarketActivated, st.flatMarketGapHeldAt, st.flatMarketReachedAt = true, time.Time{}, time.Time{}
		st.flatMarketActivatedIn = data.State
	case EventGapHeld, EventGapClosed:
		st = st.flatMarketJoined(data.Price)
		st.flatMarketGapHeldAt = time.Time{}
	case EventEnded, EventReset, EventSold:
		st = st.flatMarketClosed()
	}
	return st
}

// flatMarketJoined is the state with the episode open, opened at price when none was.
func (st state) flatMarketJoined(price float64) state {
	if !st.flatMarketActive {
		st = st.flatMarketOpened(price)
	}

	return st
}
