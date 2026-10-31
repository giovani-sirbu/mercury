// Package capitalprotection holds the capital protection gate's settings (Enabled, MaxDrawdownPercentage)
// and the pure read (Triggered) of the sophos block: a ladder at its last configured depth whose
// next entry was refused for insufficient funds opens an episode when the SMC trend dashboard reads
// every timeframe against it, and the shared action of gates/smartsell then waits for a
// pullback within MaxDrawdownPercentage, activates the sale (takeProfit at no loss, trailingBuy at
// one; takeProfit always at the shipped zero drawdown) and trails it. gates/smartsell owns the
// watch, the episode, its rows and the sale.
//
// Vocabulary: depth, fill, trend, watched, episode, triggered, drawdown, sale.
package capitalprotection
