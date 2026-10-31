package aggragates

// StrategyEventsOf is the trade's events of one gate of one strategy param,
// in the order the trade carries them: the order every fold reads, which the
// engines keep chronological.
func (t Trades) StrategyEventsOf(param, gate string) []TradesStrategyEvents {
	var matched []TradesStrategyEvents
	for _, event := range t.StrategyEvents {
		if event.Param == param && event.Gate == gate {
			matched = append(matched, event)
		}
	}

	return matched
}
