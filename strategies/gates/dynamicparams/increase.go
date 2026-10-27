package dynamicparams

// Increase is what a tier raises on every row of the ladder.
type Increase string

const (
	// IncreasePercentage raises every row's percentage by
	// BearPercentagePoints.
	IncreasePercentage Increase = "percentage"
	// IncreaseDepths raises every row's depths by BearDepths.
	IncreaseDepths Increase = "depths"
	// IncreaseBoth raises the percentage and the depths.
	IncreaseBoth Increase = "both"
	// IncreaseNone raises nothing: the configured rows.
	IncreaseNone Increase = "none"
)

// raisesPercentage reports whether the increase names the percentage.
func (increase Increase) raisesPercentage() bool {
	return increase == IncreasePercentage || increase == IncreaseBoth
}

// raisesDepths reports whether the increase names the depths.
func (increase Increase) raisesDepths() bool {
	return increase == IncreaseDepths || increase == IncreaseBoth
}

// changesRows is whether the increase names an amount that is not zero.
func (increase Increase) changesRows() bool {
	return (increase.raisesPercentage() && BearPercentagePoints != 0) ||
		(increase.raisesDepths() && BearDepths != 0)
}
