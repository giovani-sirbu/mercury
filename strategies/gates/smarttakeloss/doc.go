// Package smarttakeloss is the SmartTakeLoss flag: the slow decline gate (GateSlowDecline), the
// flat market gate (docFlatMarket.go), the free fall prevention gate (docFreeFallPrevention.go) and
// the capital protection gate (capitalProtection.go), read off the closed window of sophos' smart
// take loss block; the leaves (quietdecline, swingdecline, earlytakeprofit, lineardecline,
// capitalprotection, flatmarket, freefallprevention, series) import no parent, and constants.go
// holds the shared bounds. The slow decline gate watches a long spot ladder from each
// feature's ArmDepth: the first feature that triggers makes it pending from its newest fill and
// judges each new fill (goesPending, judgeTheNewestFill); the early take profit latches it for
// life and only moves its take profit (latchEarlyTakeProfit, TakeProfitPercentage). The four
// gates' triggers feed one shared action (sellLoss) once the wait after a gate's last trigger has
// passed (WaitAfterTrigger, capital protection's capitalprotection.WaitAfterTrigger), decided by
// the ladder's state and each trigger's terms (saleTerms): buy is activated within the depth
// bounds and the SellLossTriggers rule the SMC trend dashboard meets once a reached print confirms a
// reached row between ActivationConfirmation and ActivationConfirmationWindow (the ladder held
// meanwhile), even over a take profit the ladder proposes in buy (yieldsToActivation), a committed
// stopLoss trails and sells for the trigger that activated it unless the print gaps under
// SaleGapTolerances of its tolerance, which holds for SaleGapWait, a self-armed one sells on the
// same confirmation, and a refusal writes one held row per episode, again by the profit whenever
// the trigger moves. A depth priority hold or a dynamicParams raise latches the
// ladder. The state is rebuilt from the trade's events on every tick (rebuildState), a tick writes
// one log line per gate (TickRows), and Apply is the engines' single entry point.
//
// Vocabulary: depth, fill, floor, ceiling, tolerance, take profit, stop loss, held, verdict,
// watched, pending, judged, confirmed, cancelled, reset, latched, trigger, episode, activated, bar, drawdown.
package smarttakeloss
