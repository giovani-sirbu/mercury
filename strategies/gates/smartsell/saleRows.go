package smartsell

import (
	"fmt"
	"slices"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// saleHeldRow is the one grammar of a held sale's row, RowPrefix + "<gate> sale held, <why>", at
// the newest fill's price under gate with the event EventSaleHeld and a clone of features.
func saleHeldRow(gate string, fill float64, why string, features []string) Row {
	return Row{
		Message:  saleHeldHead(gate) + why,
		Price:    fill,
		Gate:     gate,
		Event:    EventSaleHeld,
		Features: slices.Clone(features),
	}
}

// saleHeldHead is what a held row writes before its reason; mergedMessage cuts it off.
func saleHeldHead(gate string) string {
	return RowPrefix + saleHeldLabel(gate) + " sale held, "
}

// activatedHead is what an activation row into state writes before its profit; mergedMessage cuts it off.
func activatedHead(gate, state string) string {
	return RowPrefix + saleHeldLabel(gate) + " " + activatedVerb(state) + ", "
}

// activatedVerb is how a row names the activation by the path it takes: take profit into
// takeProfit, take loss into trailingBuy.
func activatedVerb(state string) string {
	if state == aggragates.PositionTakeProfit {
		return "take profit activated"
	}
	return "take loss activated"
}

// saleHeldLabel is how a held or activation row names the gate.
func saleHeldLabel(gate string) string {
	switch gate {
	case GateFlatMarket:
		return "flat market"
	case GateFreeFallPrevention:
		return "free fall prevention"
	case GateCapitalProtection:
		return "capital protection"
	}
	return "slow decline"
}

// saleHeldByProfitRow is saleHeldRow for a sale the profit holds, naming the trigger it is held under.
func saleHeldByProfitRow(gate string, fill, profitPct, triggerPct float64, features []string) Row {
	row := saleHeldRow(gate, fill, heldByProfit(profitPct, triggerPct), features)
	row.Trigger = &triggerPct
	return row
}

// heldByProfit is the reason text of a sale the trigger holds: the profit under the trigger (triggerPct), two decimals.
func heldByProfit(profitPct, triggerPct float64) string {
	return fmt.Sprintf("profit %.2f%% of the used amount under %.2f%%", profitPct, triggerPct)
}

// activatedByProfit is the reason text of an activation: the profit at or over the trigger (triggerPct), two decimals.
func activatedByProfit(profitPct, triggerPct float64) string {
	return fmt.Sprintf("profit %.2f%% of the used amount at or over %.2f%%", profitPct, triggerPct)
}

// activatedRow is the one grammar of an activation's row, "smartSell: <gate> take profit activated,
// <why>" or "... take loss activated, <why>" by the sign of profitPct (activationState), at the
// newest fill's price under gate with the event EventActivated carrying the state it moves to.
func activatedRow(gate string, fill, profitPct, triggerPct float64, features []string) Row {
	state := activationState(profitPct)
	return Row{
		Message:  activatedHead(gate, state) + activatedByProfit(profitPct, triggerPct),
		Price:    fill,
		Gate:     gate,
		Event:    EventActivated,
		Features: slices.Clone(features),
		State:    state,
	}
}

// ActivatedRow is the row of the tick the shared action activates the sale on the slow decline
// gate's trigger (profitPct from saleProfit, fill the newest fill, features the pending ones), at
// the trigger of a dashboard reading the ladder's way on every read (FullTrendTrigger); other
// dashboards build on activatedRow.
func ActivatedRow(fill, profitPct float64, features ...string) Row {
	return activatedRow(GateSlowDecline, fill, profitPct, FullTrendTrigger(), features)
}

// heldAtDepth is the reason text of a sale the depth bound holds: the filled entries, the ceiling
// and the deepest depth the sale is made at; the entries alone when no ceiling is known.
func heldAtDepth(filled, ceiling int) string {
	if ceiling <= 0 {
		return fmt.Sprintf("depth %d of no known ceiling", filled)
	}
	return fmt.Sprintf("depth %d of %d over %d", filled, ceiling, ceiling-saleIgnoreLastDepths)
}

// heldUnderDepth is the reason text of an activation the floor holds: the filled entries under the
// depths the activation needs (saleStartAtDepth).
func heldUnderDepth(filled, need int) string {
	return fmt.Sprintf("depth %d under the %d depths the activation needs", filled, need)
}

// SaleHeldAtDepthRow is the row of the first tick of an episode the depth bound holds the slow
// decline gate's sale on (heldAtDepth), its event EventSaleHeld under GateSlowDecline.
func SaleHeldAtDepthRow(fill float64, filled, ceiling int, features ...string) Row {
	return saleHeldRow(GateSlowDecline, fill, heldAtDepth(filled, ceiling), features)
}

// FlatMarketSaleHeldAtDepthRow is SaleHeldAtDepthRow for the flat market gate, with no features.
func FlatMarketSaleHeldAtDepthRow(fill float64, filled, ceiling int) Row {
	return saleHeldRow(GateFlatMarket, fill, heldAtDepth(filled, ceiling), nil)
}

// FreeFallPreventionSaleHeldAtDepthRow is SaleHeldAtDepthRow for the free fall prevention gate, with no features.
func FreeFallPreventionSaleHeldAtDepthRow(fill float64, filled, ceiling int) Row {
	return saleHeldRow(GateFreeFallPrevention, fill, heldAtDepth(filled, ceiling), nil)
}

// SaleHeldByTrendRow is the row of the first tick of an episode the SMC trend holds the slow decline
// gate's sale on: the block's dashboard meeting no rule of SaleTriggers, or not read, read for a
// long or an inverse ladder (heldByTrendOf); its event is EventSaleHeld under GateSlowDecline.
func SaleHeldByTrendRow(fill float64, block aggragates.SmartSellIndicators, inverse bool, features ...string) Row {
	return saleHeldRow(GateSlowDecline, fill, heldByTrendOf(block, inverse), features)
}

// FlatMarketSaleHeldByTrendRow is SaleHeldByTrendRow for the flat market gate, with no features.
func FlatMarketSaleHeldByTrendRow(fill float64, block aggragates.SmartSellIndicators, inverse bool) Row {
	return saleHeldRow(GateFlatMarket, fill, heldByTrendOf(block, inverse), nil)
}

// FreeFallPreventionSaleHeldByTrendRow is SaleHeldByTrendRow for the free fall prevention gate, with no features.
func FreeFallPreventionSaleHeldByTrendRow(fill float64, block aggragates.SmartSellIndicators, inverse bool) Row {
	return saleHeldRow(GateFreeFallPrevention, fill, heldByTrendOf(block, inverse), nil)
}

// activationRows is the activation row of every trigger, in trigger order, naming the profit
// (profitPct) against its own trigger at the newest fill's price (fill).
func activationRows(triggers []saleTrigger, fill, profitPct float64, shared sharedTrigger) []Row {
	rows := make([]Row, 0, len(triggers))
	for _, trigger := range triggers {
		rows = append(rows, activatedRow(trigger.gate, fill, profitPct, trigger.terms.triggerPct(shared), trigger.features))
	}
	return rows
}
