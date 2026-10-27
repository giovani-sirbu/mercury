// Package lineardecline holds the linear decline's settings (Enabled, ArmDepth) and its pure
// reading of whether a ladder's newest Window fills sit on one straight line, on the closes of the
// series every gate shares (series.Series, series.Trend, series.Fill). Two chords run from the
// window's first fill, to the newest fill and to the one before it (chord); the line holds when
// every close of both chords lies within MaxDeviationPct of its chord, both slopes fall, they differ
// by at most MaxSlopeChange and the net move is MinFallPct or lower, and Trigger triggers when the
// line holds and the trend reads down on at least MinSmcTrendDown rows (MeetsTheNeed, asked first).
// TriggerAt takes the fills by value and allocates nothing; a miss formats nothing (Breaks) and an
// unreadable chord fails closed. gates/smarttakeloss owns the latch it feeds and, with Enabled on,
// the pending row and the sale.
//
// Vocabulary: depth, fill, bar, chord, slope, deviation, hold, latch, reading.
package lineardecline
