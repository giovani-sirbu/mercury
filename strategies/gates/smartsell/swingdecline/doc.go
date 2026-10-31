// Package swingdecline holds the swing decline's settings (Enabled, ArmDepth) and its pure
// reading of the shape of a ladder's decline between its fills, on the wicks of the closed bars,
// the Trend Direction row and the fills of package series. A window from an earlier fill's bar
// (the first fill's led by LookbackBars) to a later fill's bar, or to the series' last bar, is
// read on the highs and lows: swing pivots at PivotBars each side, legs from the highest swing
// high, a down leg counting once it runs MinBarsBetween bars and falls MinDropPct, an up leg once
// it rises at most MaxBouncePct, and the window holding on MinLegsDown and MinLegsUp of them
// (Read). Trigger reads every window at least MinDepthsBetween depths back ending at the newest
// fill's bar, TriggerAtLastBar at the series' last bar; either triggers when a window holds and
// the trend reads down on MinSmcTrendDown rows, failing closed on a missing bar or an unread
// trend. gates/smartsell owns the pending row, the sale and the latch it feeds.
//
// Vocabulary: depth, fill, bar, wick, window, swing, leg, bounce, counts, holds, reading.
package swingdecline
