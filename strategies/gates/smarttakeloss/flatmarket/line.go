package flatmarket

import "math"

// line is the least-squares line through the closes by bar index: its value at the first bar and
// its rise per bar.
type line struct {
	start, perBar float64
}

// fit is the line the closes lie nearest, by ordinary least squares over their index; one close
// alone is a flat line through it.
func fit(closes []float64) line {
	count := float64(len(closes))
	if count == 0 {
		return line{}
	}
	meanIndex := (count - 1) / 2
	var meanClose, covariance, variance float64
	for _, closed := range closes {
		meanClose += closed
	}
	meanClose /= count
	for index, closed := range closes {
		gap := float64(index) - meanIndex
		covariance += gap * (closed - meanClose)
		variance += gap * gap
	}
	if variance == 0 {
		return line{start: meanClose}
	}
	perBar := covariance / variance
	return line{start: meanClose - perBar*meanIndex, perBar: perBar}
}

// at is the line's value at a bar index.
func (fitted line) at(index int) float64 {
	return fitted.start + fitted.perBar*float64(index)
}

// movePct is the line's move from its first bar to its last, in percent of its start; NaN for a
// start at or under zero, which no check passes.
func (fitted line) movePct(last int) float64 {
	if !(fitted.start > 0) {
		return math.NaN()
	}
	return (fitted.at(last)/fitted.start - 1) * 100
}

// deviationPct is a close's distance from the line at its bar, in signed percent of the line;
// +Inf under a line at or under zero.
func deviationPct(closed, lineValue float64) float64 {
	if !(lineValue > 0) {
		return math.Inf(1)
	}
	return (closed/lineValue - 1) * 100
}
