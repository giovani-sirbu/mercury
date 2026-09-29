package smarttakeloss

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// SaleActions is the action chain an engine runs for a position, with the
// indecision direction's one change to it: on a ladder it has latched, the
// `sell` chain — the trailing take profit's sale, TAKEPROFIT_TO_SELL — runs
// acceptLoss where it runs hasProfit, so that sale goes through under the
// minimum profit, below zero, when the price falls back after the take
// profit armed. acceptLoss prices the close as hasProfit does and never
// refuses it, as in the sellLoss chain. The `takeProfit` and
// `update_takeProfit` chains keep hasProfit, so the take profit still arms
// only where the close clears the minimum profit.
//
// It returns actions itself — the engine's own slice, untouched — unless the
// position is `sell`, the strategy carries the flag, the trade is a parent and
// rebuildState reads it latched and not held by a depth priority
// (depthPriorityHeld: the latch stands, its effects wait for the next fill);
// then a new slice with every hasProfit replaced. It reads the fills and the
// rows alone, never the position type, which the engines have already
// rewritten to the position by then, and it asks the cheap marker scan
// (carriesMarker) before the fold. hermes and both sisyphus engines call it
// right after their GetActionsByPosition, so the rule lives here and the
// engines cannot drift apart on it.
func SaleActions(trade aggragates.Trades, position string, actions []string) []string {
	if position != "sell" || !trade.Strategy.Params.SmartTakeLoss || trade.ParentID != 0 {
		return actions
	}
	if !indecisionDirection || !carriesMarker(trade, IndecisionMarker) {
		return actions
	}
	if st := rebuildState(trade); !st.indecision || st.depthPriorityHeld {
		return actions
	}
	swapped := make([]string, len(actions))
	for index, action := range actions {
		swapped[index] = action
		if action == "hasProfit" {
			swapped[index] = "acceptLoss"
		}
	}
	return swapped
}
