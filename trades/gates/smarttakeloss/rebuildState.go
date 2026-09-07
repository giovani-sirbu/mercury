package smarttakeloss

import (
	"strings"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// state is what the trade's own rows say about the smart take loss: whether
// the ladder is deep enough to arm, whether a fill has activated it and at
// what price, the entry fills themselves, and how many depths filled after
// the activating one. It is rebuilt from trade.Logs and trade.History on
// every tick, the way cooldown.firstFillState rebuilds the first-fill gate:
// the rows are the only state. Nothing is kept in Redis, in a column or on
// trade.PositionPrice.
type state struct {
	armed  bool
	active bool
	// activationPrice is the Price column of the FIRST activation row: the
	// newest fill at the moment the price reached the window's level. A later
	// row with the marker is a duplicate written under a lost lock, never a
	// new activation.
	activationPrice float64
	fills           []entryFill
	// depthsAfterActivation counts the distinct entry fills placed AFTER
	// the activating one; the trade is on its last permitted depth once it
	// reaches PermittedDepths.
	depthsAfterActivation int
	// waitLogged: the refusal that the wait after this very fill causes has
	// already been written to the trade, so it is not proposed again on the
	// next tick.
	waitLogged bool
}

// lastFill is the newest entry fill in slice order, zero when none filled.
func (st state) lastFill() entryFill {
	if len(st.fills) == 0 {
		return entryFill{}
	}
	return st.fills[len(st.fills)-1]
}

// rebuildState folds the rows. Each is matched by its marker anywhere in the
// message (they carry gates.SaveHoldLog's "Hold buy: " frame) and must carry
// a price: the FIRST activation row wins, and a wait row counts as written
// for the fill whose price it holds, so the refusal is logged once per fill
// and not once per tick.
func rebuildState(trade aggragates.Trades) state {
	st := state{fills: entryFills(trade), armed: armed(trade)}
	lastFillPrice := st.lastFill().Price
	for _, row := range trade.Logs {
		if row.Price <= 0 {
			continue
		}
		switch {
		case strings.Contains(row.Message, ActivationMarker):
			if !st.active {
				st.active = true
				st.activationPrice = row.Price
			}
		case strings.Contains(row.Message, WaitMarker):
			if row.Price == lastFillPrice {
				st.waitLogged = true
			}
		}
	}
	if st.active {
		st.depthsAfterActivation = depthsAfterActivation(trade.Inverse, st.fills, st.activationPrice)
	}
	return st
}

// depthsAfterActivation locates the activating fill by its price — the row
// was written with the fill's own history price, so equality holds — and
// counts the distinct entry fills after it in slice order. The order, not
// the price, is what makes a later fill count: after the activation the
// ladder can take profit, sell, re-anchor (update_buy) and fill the next
// depth ABOVE the activating price, and that is still the one depth
// permitted. When no fill carries the price (a row written by hand, a
// re-priced history) the count falls back to the fills strictly beyond it:
// lower on a long, higher on an inverse ladder.
func depthsAfterActivation(inverse bool, fills []entryFill, activationPrice float64) int {
	for index, fill := range fills {
		if fill.Price == activationPrice {
			return len(fills) - index - 1
		}
	}
	beyond := 0
	for _, fill := range fills {
		if (inverse && fill.Price > activationPrice) || (!inverse && fill.Price < activationPrice) {
			beyond++
		}
	}
	return beyond
}
