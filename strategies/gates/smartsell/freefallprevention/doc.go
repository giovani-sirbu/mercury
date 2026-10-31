// Package freefallprevention holds the free fall prevention gate's settings (Enabled,
// HoldInitialDepth, ResetOnNewDepth, ArmDepth) and its pure reading of the SMC trend dashboard's
// free fall (a resistance and no support left below within a row's S/R look-back; free rise, the
// mirror, for an inverse ladder) on the rows FreeFallTimeframes counts (every row when it names
// none): the rows as of the last closed bar (series.ReadsOf) and the masks sophos serves for the
// last closed bars of Timeframe over WindowTimeframe (Series, one bit per row). Trigger holds when
// at least MinFreeFallTimeframes counted rows are free on the last closed bar, or on strictly more
// than MinShare of the window's bars; a series not served fails the share closed, and
// MinFreeFallTimeframes at or below zero reads nothing. gates/smartsell owns the hold, the
// latch, the episode and the sale.
//
// Vocabulary: bar, row, mask, free fall, free rise, share, window, served, reading.
package freefallprevention
