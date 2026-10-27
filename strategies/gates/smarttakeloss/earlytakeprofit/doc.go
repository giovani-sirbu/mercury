// Package earlytakeprofit holds the early take profit's settings. The latch sells nothing: a
// watched long spot ladder is latched for life on the first of ten readings, in this order —
// sophos' indecision on the last closed bar and its look-back (indecisionReads,
// indecisionReadsRecently), a depth priority hold (DepthPriority), a ladder that opened raised
// (DynamicParams), the quiet decline (QuietDecline), the swing decline (SwingDecline), the linear
// decline (LinearDecline), the flat market's latch reading (FlatMarket), the free fall
// prevention's reading (FreeFallPrevention) — and from break even up its take profit reads its
// position price, the last buy, as well as its average entry.
// gates/smarttakeloss owns the watch, the latch, the events and the rows.
//
// Vocabulary: depth, fill, bar, look-back, watched, latched, take profit.
package earlytakeprofit
