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

// SmartTakeLossIndicators is the 1d chart block sophos serves on
// GET /:symbol/patterns for the SmartTakeLoss flag: the levels that say how
// many bars of its window printed under the price, the resistance (support)
// line through the last two lower highs (higher lows) and the
// Bollinger(20, 2.0) bands. HasVerdict is false — and every other field zero
// — when sophos had fewer than a full window or no readable band; every zero
// field is inert in gates/smarttakeloss.Apply.
type SmartTakeLossIndicators struct {
	HasVerdict bool
	// The four levels, all read against the tick price and all taken over the
	// same window of closed daily bars. A long price at or under LowestLow has
	// NO bar of that window below it; at or under LowWithBarsLeft it has at
	// most the five the gate allows. HighestHigh and HighWithBarsLeft are the
	// inverse ladder's mirror, counting bars that printed higher.
	LowestLow        float64
	LowWithBarsLeft  float64
	HighestHigh      float64
	HighWithBarsLeft float64
	UpperBB          float64
	LowerBB          float64
	// Resistance runs through two lower highs (the long exit); Support
	// through two higher lows (the inverse exit). Wick prices, bar open
	// times.
	Resistance TrendLine
	Support    TrendLine
}
