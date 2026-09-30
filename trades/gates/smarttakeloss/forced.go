package smarttakeloss

// forced is the overlay's answer when one of its rules sells: the ladder's
// proposal replaced by the sellLoss chain, and Reason naming the rule for the
// engine's ExitRow.
func forced(result Result, reason string) Result {
	result.Position = "sellLoss"
	result.Reason = reason
	return result
}
