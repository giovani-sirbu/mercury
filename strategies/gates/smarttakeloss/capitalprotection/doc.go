// Package capitalprotection holds the capital protection gate's settings (Enabled, MaxDrawdownPercentage)
// and the pure read (Triggered) of the sophos block: a ladder at its last configured depth whose
// next entry was refused for insufficient funds opens an episode when the SMC trend dashboard reads
// every timeframe against it, and the shared action of gates/smarttakeloss then waits for a
// pullback within MaxDrawdownPercentage, activates the stopLoss and trails it. gates/smarttakeloss
// owns the watch, the episode, its rows and the sale.
//
// Vocabulary: depth, fill, trend, watched, episode, triggered, drawdown, sale.
package capitalprotection
