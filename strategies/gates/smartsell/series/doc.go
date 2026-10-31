// Package series holds what sophos' smart sell block serves every gate alike: the closed 1h
// bars (Series, SeriesOf) with their wicks when served (WicksServed), a fill's bar on them (Fill,
// BarOpen, BarMs) and the SMC trend dashboard as of the last bar (Trend, TrendOf, TrendRow,
// ReadsOf), each row's free fall with it (no support left below, Free). It reads the block and
// nothing of the gates; the swing, linear and flat market readings and the shared sale trigger read it.
//
// Vocabulary: bar, series, wick, fill, stamp, trend, row, read, free fall, served.
package series
