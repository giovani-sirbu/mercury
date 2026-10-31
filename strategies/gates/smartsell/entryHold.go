package smartsell

import (
	"github.com/giovani-sirbu/mercury/strategies/gates"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/flatmarket"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/freefallprevention"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// SlowDeclineEntryHoldReason is the constant row text of a first fill held on the quiet decline's
// verdict; constant so tradelog.SaveHoldLog writes one row.
const SlowDeclineEntryHoldReason = RowPrefix + "quiet decline, first fill held"

// FlatMarketEntryHoldReason is the constant row text of a first fill held on the flat market
// reading; the bars ride in the held event's reasons, never in the text.
const FlatMarketEntryHoldReason = RowPrefix + "flat market, first fill held"

// FreeFallPreventionEntryHoldReason is the constant row text of a first fill held on the free fall
// prevention reading; the rows ride in the held event's reasons, never in the text.
const FreeFallPreventionEntryHoldReason = RowPrefix + "free fall prevention, first fill held"

// EntryHold is the flag's first-fill hold on a parent: the quiet decline's verdict first, then the
// flat market reading, then the free fall prevention reading (side is aggragates.EntrySide); a
// refusal carries its row text and held event.
func EntryHold(trade aggragates.Trades, side string, ai aggragates.AIIndicators) gates.Hold {
	if !trade.Strategy.Params.SmartSell || trade.ParentID != 0 {
		return gates.Hold{}
	}
	if hold := quietDeclineEntryHold(side, ai); hold.Held() {
		return hold
	}
	if hold := flatMarketEntryHold(trade, side, ai.SmartSell); hold.Held() {
		return hold
	}
	return freeFallPreventionEntryHold(trade, side, ai.SmartSell)
}

// quietDeclineEntryHold holds a long entry while sophos' verdict stands (QuietDeclineVerdict), filed
// under GateEntryHold; the look-back holds nothing, and off with quietdecline.Enabled.
func quietDeclineEntryHold(side string, ai aggragates.AIIndicators) gates.Hold {
	if !quietDecline || side != aggragates.SideLong || !ai.SmartSell.QuietDeclineVerdict {
		return gates.Hold{}
	}

	return gates.Hold{
		Reason: SlowDeclineEntryHoldReason,
		Key:    aggragates.LogKeyHoldSmartSellEntry,
		Param:  aggragates.StrategyParamSmartSell,
		Gate:   GateEntryHold,
		Data:   EventData{Event: gates.EventHeld},
	}
}

// flatMarketEntryHold holds a spot entry of either side while the flat market reading holds on the
// base percentage the ladder trades (flatmarket.HoldInitialDepth, whatever flatmarket.Enabled says;
// flatmarket.Trigger), filed under GateFlatMarket with the reading's reasons in the event; no expiry.
func flatMarketEntryHold(trade aggragates.Trades, side string, block aggragates.SmartSellIndicators) gates.Hold {
	if !flatMarketHoldInitialDepth || trade.Strategy.TradeType == aggragates.Futures {
		return gates.Hold{}
	}
	reading := flatmarket.Trigger(flatmarket.BarsOf(block), basePercentage(trade), side, series.TrendOf(block))
	if !reading.Holds {
		return gates.Hold{}
	}

	return gates.Hold{
		Reason: FlatMarketEntryHoldReason,
		Key:    aggragates.LogKeyHoldSmartSellEntry,
		Param:  aggragates.StrategyParamSmartSell,
		Gate:   GateFlatMarket,
		Data:   EventData{Event: gates.EventHeld, Reasons: reading.Reasons},
	}
}

// freeFallPreventionEntryHold holds a spot entry of either side while the free fall prevention
// reading holds (freefallprevention.HoldInitialDepth, whatever freefallprevention.Enabled says;
// freefallprevention.Trigger), filed under GateFreeFallPrevention with the reading's reasons in the
// event; no expiry.
func freeFallPreventionEntryHold(trade aggragates.Trades, side string, block aggragates.SmartSellIndicators) gates.Hold {
	if !freeFallPreventionHoldInitialDepth || trade.Strategy.TradeType == aggragates.Futures {
		return gates.Hold{}
	}
	reading := freefallprevention.Trigger(block, side)
	if !reading.Holds {
		return gates.Hold{}
	}

	return gates.Hold{
		Reason: FreeFallPreventionEntryHoldReason,
		Key:    aggragates.LogKeyHoldSmartSellEntry,
		Param:  aggragates.StrategyParamSmartSell,
		Gate:   GateFreeFallPrevention,
		Data:   EventData{Event: gates.EventHeld, Reasons: reading.Reasons},
	}
}
