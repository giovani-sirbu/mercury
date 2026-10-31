package capitalprotection

import "time"

var (
	// Enabled switches the capital protection gate: its watch of a ladder at its last depth whose
	// next entry was refused for insufficient funds (capitalProtectionWatched), the episode its trend reading opens (Triggered) and the sale the
	// shared action makes on its trigger.
	Enabled = false
	// MaxDrawdownPercentage is the loss, in percent of the amount used (saleProfit), a sale may book at
	// most: the shared action activates the sale once a pullback brings the loss to it or under
	// (takeProfit at no loss, trailingBuy at one; always takeProfit at zero), and sells a self-armed
	// trailingBuy there; over it the gate waits, one held row per episode.
	MaxDrawdownPercentage float64 = 0
	// WaitAfterTrigger is how long the gate waits from its triggered row before the shared action
	// judges its trigger: no held row, no activation, no sale meanwhile, the ladder running as its
	// own logic lets it and its own closes passing. A fill past the ceiling re-triggers and restarts
	// it. Zero or below waits nothing, as does an engine that hands Apply no clock.
	WaitAfterTrigger = 24 * time.Hour
)
