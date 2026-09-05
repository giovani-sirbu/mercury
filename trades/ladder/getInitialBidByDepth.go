package ladder

import "math"

func GetInitialBidByDepth(amount float64, depth float64, multiplier float64, percentage float64) float64 {
	if multiplier <= 0 || depth <= 0 {
		return 0
	}
	if percentage < 0 || percentage >= 100 {
		return 0
	}

	// Calculate the adjusted ratio
	reductionFactor := 1 - (percentage / 100)
	ratio := multiplier * reductionFactor

	// Compute the first term (initial bid)
	numerator := amount * (1 - ratio)
	denominator := 1 - math.Pow(ratio, depth)
	if denominator == 0 {
		// A ratio of exactly 1 is a flat ladder: every rung costs the same.
		return amount / depth
	}
	initialBid := numerator / denominator

	/*
		//Generate the sequence
		sequence := make([]float64, int(depth))
		sequence[0] = initialBid

		// Calculate each depth's value
		for i := 1; i < int(depth); i++ {
			sequence[i] = sequence[i-1] * ratio
		}
	*/

	return initialBid
}
