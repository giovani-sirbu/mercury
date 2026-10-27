package depthpriority

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/strategies/gates"
	"github.com/giovani-sirbu/mercury/strategies/gates/cooldown"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// DepthPriorityApplies is whether the tick consumes the wallet view: Enabled and Cooldown on, and a
// transition that spends new capital (a first fill, or an add resolving to stopLoss), never a close.
func DepthPriorityApplies(params aggragates.StrategyParams, oldPosition, position string) bool {
	if !Enabled || !params.Cooldown {
		return false
	}

	return oldPosition == "new" || gates.PositionType(position) == "stopLoss"
}

// DepthPriorityHold is the gate: the zero Hold lets the chain proceed; a refusal carries the row's
// text and the DepthPriorityEvent beside it. Impasse children are never held, on either side. The
// wallet is held only on a balance the engine named (walletFreeFor) and a non-empty view.
func DepthPriorityHold(event events.Events, position string) gates.Hold {
	if !DepthPriorityApplies(event.Trade.Strategy.Params, event.Params.OldPosition, position) {
		return gates.Hold{}
	}
	if event.Trade.ParentID != 0 {
		return gates.Hold{}
	}

	own := LadderOf(event.Trade)

	free, freeKnown := walletFreeFor(event, own.Asset)
	if !freeKnown {
		return gates.Hold{}
	}

	// The priority first: pricing own's entry re-runs the sizing, and most ticks keep nothing.
	priority, found := depthPriorityFor(own, event.Params.WalletLadders)
	if !found {
		return gates.Hold{}
	}

	// Priced as Buy places it: a first entry on Params.EntrySettings (SizingTrade), an add on the trade's rows.
	_, ownCost := ladder.NextEntryCost(event.Params.SizingTrade(event.Trade), free)
	if !depthPriorityHolds(ownCost, free, priority.RemainingCost) {
		return gates.Hold{}
	}

	data := cooldown.DepthPriorityEvent{
		Event:            gates.EventHeld,
		PrioritySymbol:   priority.Symbol,
		PriorityDepth:    priority.Depth,
		PriorityMaxDepth: priority.MaxDepth,
		Depth:            own.Depth,
		MaxDepth:         own.MaxDepth,
	}

	return gates.Hold{
		Reason: depthPriorityHoldMessage(data),
		Param:  aggragates.StrategyParamCooldown,
		Gate:   cooldown.GateDepthPriority,
		Data:   data,
	}
}

// walletFreeFor is the free balance of the asset the next entry spends, less the quote this wallet's
// inverse trades committed (as funds.GetFundsQuantities subtracts it); an asset without a balance is
// unknown and holds nothing.
func walletFreeFor(event events.Events, asset string) (float64, bool) {
	if asset == "" {
		return 0, false
	}

	for _, balance := range event.Params.WalletFree {
		if balance.Asset != asset {
			continue
		}

		free := balance.Free
		if !event.Trade.Inverse {
			free -= helpers.FindUsedAmount(event.Params.InverseUsedAmount, asset)
		}

		return free, true
	}

	return 0, false
}

// depthPriorityHolds is the reserve: the entry waits when placing it would leave the wallet under
// what the reserved ladder still needs; level with it buys.
func depthPriorityHolds(ownCost, free, reserve float64) bool {
	return free-ownCost < reserve
}
