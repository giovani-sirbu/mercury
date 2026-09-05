package aggragates

// ApplyPatternSide copies a /patterns action onto the legacy bullish/bearish
// flags used when UsePatterns is on and there is no regime verdict.
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
		if !hasPattern {
			copyRegimeCrashContinuation(&out, mlVerdict)
		}
	}
	// The strategy flags are NOT stamped on the verdict: every gate reads
	// event.Trade.Strategy.Params, and the mirror fields the merge used to
	// write were read by nothing.
	return out
}

func copyRegimeCrashContinuation(dst *AIIndicators, src AIIndicators) {
	dst.HasRegimeVerdict = src.HasRegimeVerdict
	dst.EnterAllowed = src.EnterAllowed
	dst.AddAllowed = src.AddAllowed
	dst.Regime = src.Regime
	dst.Regimes = src.Regimes
	dst.CrashActive = src.CrashActive
	dst.CrashScore = src.CrashScore
	dst.CrashReasons = src.CrashReasons
	dst.HasContinuationVerdict = src.HasContinuationVerdict
	dst.DownContinuationRisk = src.DownContinuationRisk
	dst.UpContinuationRisk = src.UpContinuationRisk
	dst.ReversalUpEvidence = src.ReversalUpEvidence
	dst.ReversalDownEvidence = src.ReversalDownEvidence
	dst.DailyNatrPct = src.DailyNatrPct
	dst.ContinuationReasons = src.ContinuationReasons
}
