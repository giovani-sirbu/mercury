package aggragates

// TrendLineAnchor is one end of a trend line: the open time (Unix ms) of the
// bar whose wick it sits on, and that wick's price.
type TrendLineAnchor struct {
	At    int64
	Price float64
}

// TrendLine is the line through two anchors, From earlier than To; the
// engines project it forward to the tick time. A line exists only when both
// anchor times are set — a zero anchor means sophos found no pair.
type TrendLine struct {
	From TrendLineAnchor
	To   TrendLineAnchor
}

// SmartTakeLossIndicators is the chart block sophos serves on
// GET /:symbol/patterns for the SmartTakeLoss flag, computed on one window of
// WindowBars closed bars of Interval: the body levels that say how many bars
// of that window closed under the price, the resistance (support) line through
// the last two lower highs (higher lows) with the count of newest closed bars
// beyond each, and the Bollinger band (BBPeriod, BBStdDev). HasVerdict is
// false — and every other field zero — when sophos had fewer than a full
// window or no readable band; every zero field is inert in
// gates/smarttakeloss.Apply.
type SmartTakeLossIndicators struct {
	HasVerdict bool
	// The four levels, all read against the tick price, all taken over that
	// same window of closed bars and all measured on the candle BODY —
	// min(open, close) on the long side — so a bar whose wick pierced the
	// price and closed back above it is NOT a bar to the left. A long price at
	// or under LowestBody has no bar of that window closing below it; at or
	// under LowBodyWithBarsLeft it has at most MaxBarsLeft of them.
	// HighestBody and HighBodyWithBarsLeft are the inverse ladder's mirror,
	// counting bodies that closed higher.
	LowestBody           float64
	LowBodyWithBarsLeft  float64
	HighestBody          float64
	HighBodyWithBarsLeft float64
	UpperBB              float64
	LowerBB              float64
	// Resistance runs through two lower highs (the long bounce exit);
	// Support through two higher lows (the inverse bounce exit). Wick
	// prices, bar open times.
	Resistance TrendLine
	Support    TrendLine
	// LowerLows runs through the two most recent lower lows: the support a
	// falling market holds, the line the long side's break rules read.
	// HigherHighs through the two most recent higher highs: the rising
	// market's resistance, the inverse ladder's mirror. Absent in a market
	// not making them.
	LowerLows   TrendLine
	HigherHighs TrendLine
	// SupportBarsUnder is how many of the newest closed bars of that window in
	// a row CLOSED under the LowerLows line (read at their own open time),
	// counted back from the last closed bar; ResistanceBarsOver is the mirror
	// over HigherHighs. Zero without the line. Sophos counts, the gate
	// compares them with smarttakeloss.SupportBreakBars.
	SupportBarsUnder   int
	ResistanceBarsOver int
	// SupportBounceLevel is the support the price bounced from: the
	// LowerLows line's value on the newest closed bar after the line's
	// newest anchor whose wick reached it and that the price then closed
	// back over (that bar or a later one) — the anchors themselves are the
	// lows the line is drawn through, never a bounce off it.
	// SupportBounceBarsUnder counts the
	// newest closed bars in a row, after the bounce, that closed under THAT
	// level, which stays put while the line goes on. Zero without a bounce.
	// The gate compares the count with smarttakeloss.SupportBounceBreakBars.
	// ResistanceBounceLevel / ResistanceBounceBarsOver mirror it on
	// HigherHighs.
	SupportBounceLevel       float64
	SupportBounceBarsUnder   int
	ResistanceBounceLevel    float64
	ResistanceBounceBarsOver int
}
