package aggragates

// ApplyPatternSide copies a /patterns action onto the legacy bullish/bearish
// flags; the merge uses it when the pattern leg stands in for the ML verdict
// (UseAI off).
func ApplyPatternSide(ai AIIndicators, action string) AIIndicators {
	switch action {
	case ActionLong:
		ai.AIMarketBullish = true
		ai.AIMarketBearish = false
	case ActionShort:
		ai.AIMarketBearish = true
		ai.AIMarketBullish = false
	}
	return ai
}

// MergeSophosVerdicts folds optional /patterns and ML legs. hasPattern / hasML
// say a fetch succeeded; a failed leg is omitted so the other still applies.
// The ML leg carries only the AI verdict: the smart take loss block is served
// by the pattern route alone, so an ML-only merge leaves it zero, which is
// inert. Neither leg carries the DynamicParams reads, so the merge leaves
// them zero too: they are the /smc-trend leg's (NeedsSmcTrendRoute), and the
// engines set AIIndicators.DynamicParams from that leg after the merge.
func MergeSophosVerdicts(
	params StrategyParams,
	patternVerdict AIIndicators,
	mlVerdict AIIndicators,
	hasPattern bool,
	hasML bool,
) AIIndicators {
	out := AIIndicators{}
	if hasPattern {
		out = patternVerdict
		out.PatternAction = patternVerdict.AIAction
		if params.UseAI {
			out.AIMarketBullish = false
			out.AIMarketBearish = false
			out.AIAction = ""
			out.AISignalStrength = 0
			out.StayOutReasons = nil
		} else {
			out = ApplyPatternSide(out, out.PatternAction)
		}
	}
	if hasML {
		out.AIAction = mlVerdict.AIAction
		out.AIMarketBullish = mlVerdict.AIMarketBullish
		out.AIMarketBearish = mlVerdict.AIMarketBearish
		out.AISignalStrength = mlVerdict.AISignalStrength
		out.StayOutReasons = mlVerdict.StayOutReasons
	}
	// The strategy flags are NOT stamped on the verdict: every gate reads
	// event.Trade.Strategy.Params, and the mirror fields the merge used to
	// write were read by nothing.
	return out
}
