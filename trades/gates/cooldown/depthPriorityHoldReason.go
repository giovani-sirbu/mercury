package cooldown

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// Depth priority is the Cooldown flag's third gate, and the only one that
// looks past the trade it is asked about: it reads every ladder of the same
// wallet and the wallet's own free balance.
//
// The shape it answers to is a market where the pairs fall together. Every
// ladder of the wallet arms its next entry within hours of the others, the
// funds are spread over all of them, and not one reaches the depth its grid
// was sized for — the depth whose average entry price is low enough to be
// paid back by a small bounce. The wallet ends up holding several
// half-filled ladders, all of them under water, instead of one finished one.
//
// So the wallet is RESERVED for the deepest ladder: the funds it still needs
// for its remaining depths are kept out of every other ladder's reach. A
// sibling's entry is held only when placing it would leave the wallet under
// that amount — when the wallet covers both, everybody buys, which is what
// keeps the reserve from being a queue. The ladder the wallet is kept for is
// never held by its own reservation.
//
// The reserve is measured from the deepest ladder's FIRST depth, not from
// some band near its last one. A depth says nothing about money: a ladder one
// entry short of its ceiling can need more than the wallet holds, and one
// five entries short can need almost nothing. Arming only near the ceiling
// reserves the wallet at the point where the remaining entries — the biggest
// ones the ladder places — can no longer be afforded, which is too late to be
// a reserve at all. Measuring the money instead means the gate arms exactly
// when the sums stop fitting and stays out of the way while they do.
//
// Which ladder the wallet is kept for is the ranking's call, set out beside
// it in depthPriorityFor.go. The deepest ladder is in front, and of two at the
// same depth the one planned cheaper to finish from its last fill, then the
// lower trade id — a key that stands still between fills, where the reserve's
// own amount moves with the position price. A FULL ladder keeps its place in
// front until it closes but keeps nothing, so while it stands there the
// wallet is kept for the next ladder in line that still has depths left; with
// no such ladder ahead, every entry the wallet can pay for buys — that IS the
// release at the ceiling — and one it cannot pay for waits. A ladder blocked
// on its next entry reserves the wallet only once the wallet can re-admit it,
// and the managed trade's own depth and costs are always read from the trade
// it is asked about, never from the view.
//
// Every ladder's depth, ceiling and costs are read on the rows it TRADES
// (ladder.DepthOf): a ladder that opened with a raise — its opened event, read
// off its own strategy events — is full only at its raised ceiling and keeps
// the wallet for the depths that event added, on the managed trade's side of
// the gate and in every surface's view alike. The rows the chain hands the gate
// are the stored ones; the raise reaches it through the trade's events.
//
// Ladders are compared only against the ones spending the same asset: a long
// ladder spends the quote side of its pair and an inverse one the base side,
// and two ladders funded from different currencies never take money from each
// other.
//
// It reads no clock and no indicator: only the depths, the costs and the
// balance the engine handed it. The engines fill Params.WalletLadders and
// Params.WalletFree on the ticks DepthPriorityApplies allows and leave them
// nil everywhere else. A nil or empty view holds nothing, and a balance the
// engine could not name holds nothing either — the fail-open posture of this
// whole package. A hermes fetch that fails yields nil for the same reason: no
// wallet is held on a guess.
//
// The hold row names both ladders and never the balance, and so does the
// event beside it (DepthPriorityEvent). The balance moves on every tick and
// gates.SaveHoldLog deduplicates on the full string, so a row carrying it
// would be a new row per tick for as long as the reservation stood.

// DepthPriorityApplies reports whether this tick can consume the wallet view
// at all, so an engine knows whether to build or fetch one. It is the gate's
// own transition rule, kept here rather than in each engine so the four
// surfaces cannot drift apart on which ticks are gated.
//
// The gated transitions are the ones that spend NEW capital: the first fill
// (a new trade has the smallest depth of all and its entry competes for the
// same wallet) and every add, including the force-trailing re-anchors and
// re-arms that resolve to a stopLoss. Never a close: a gate on new capital
// must not defer an exit, and here the exit is literally what releases the
// ladders that are waiting.
//
// oldPosition is Params.OldPosition and position is the position the chain is
// about to run; a re-anchor is mapped onto the family it re-arms exactly as
// the gates themselves map it.
func DepthPriorityApplies(params aggragates.StrategyParams, oldPosition, position string) bool {
	if !DepthPriority || !params.Cooldown {
		return false
	}

	return oldPosition == "new" || gates.PositionType(position) == "stopLoss"
}

// DepthPriorityHoldMarker opens every depth priority hold message
// (depthPriorityHoldMessage). It is human-readable text, byte-stable for cp
// and the notification filter, never a schema: the gate's record is the
// DepthPriorityEvent that goes beside the row.
const DepthPriorityHoldMarker = "cooldown: depth priority"

// DepthPriorityHold is the gate. The zero Hold means the chain may proceed; a
// refusal names the text of its row and the DepthPriorityEvent that goes
// beside it (gates.SaveHoldLog writes both). The caller owns the flag,
// exactly like DepthSpacingHold.
//
// Impasse children are out of it on both sides: they belong to their impasse
// chain and spend what the parent's close freed, not the wallet the parent
// competes for — the same exclusion the smart take loss makes.
func DepthPriorityHold(event events.Events, position string) gates.Hold {
	if !DepthPriorityApplies(event.Trade.Strategy.Params, event.Params.OldPosition, position) {
		return gates.Hold{}
	}
	if event.Trade.ParentID != 0 {
		return gates.Hold{}
	}

	own := ladder.DepthOf(event.Trade)

	free, freeKnown := walletFreeFor(event, own.Asset)
	if !freeKnown {
		return gates.Hold{}
	}

	// Which ladder the wallet is kept for comes first, and the managed
	// trade's own entry is priced only once there is something to weigh it
	// against. Pricing it re-runs the ladder's own sizing, this gate is asked
	// on every entry tick of every ladder, and on most of those ticks the
	// wallet is keeping nothing at all — so the order of these two is the
	// difference between a reserve and a tax on the tick path.
	priority, found := depthPriorityFor(own, event.Params.WalletLadders)
	if !found {
		return gates.Hold{}
	}

	// The entry is priced as Buy will place it: a first entry on the rows the
	// engine named for it (Params.EntrySettings, through SizingTrade), an add
	// on the trade's own rows — an add's cost is the last entry times its
	// row's multiplier, which a raise does not move. The depth above is read
	// on the rows the ladder trades, a raise included.
	_, ownCost := ladder.NextEntryCost(event.Params.SizingTrade(event.Trade), free)
	if !depthPriorityHolds(ownCost, free, priority.RemainingCost) {
		return gates.Hold{}
	}

	data := DepthPriorityEvent{
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
		Gate:   GateDepthPriority,
		Data:   data,
	}
}

// walletFreeFor is the balance the managed trade's next entry would be placed
// from: the engine's reading for the asset that entry spends, less the quote
// this wallet's inverse trades have already committed — exactly what
// funds.GetFundsQuantities subtracts before it compares. An asset the engine
// named no balance for is UNKNOWN, and the gate holds nothing on an unknown
// balance.
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

// depthPriorityHolds is the reserve itself: the entry waits when placing it
// would leave the wallet under what the reserved ladder still needs. Level
// with it is not under it — a wallet that covers both lets everybody buy,
// which is what keeps the reserve from being a queue.
func depthPriorityHolds(ownCost, free, reserve float64) bool {
	return free-ownCost < reserve
}
