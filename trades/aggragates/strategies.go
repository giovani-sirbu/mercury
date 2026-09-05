package aggragates

type (
	Strategies struct {
		ID        uint           `gorm:"primaryKey" form:"id" json:"id" xml:"id"`
		Name      string         `gorm:"type:varchar(50)" bson:"name" json:"name" form:"name" xml:"name" validate:"required,min=3,max=50"`
		TradeType TradeTypes     `gorm:"type:varchar(50); default:spot" bson:"tradeType" json:"tradeType" form:"tradeType" xml:"tradeType"`
		Params    StrategyParams `gorm:"type:jsonb;serializer:json;" bson:"params" json:"params" form:"params" xml:"params"`
	}
	StrategyParams struct {
		Pairs            uint8 `form:"pairs" json:"pairs" xml:"pairs"`
		Impasse          bool  `form:"impasse" bson:"impasse" json:"impasse"`
		Cooldown         bool  `form:"cooldown" bson:"cooldown" json:"cooldown"`
		UseAI            bool  `form:"useAI" bson:"useAI" json:"useAI"`
		UsePatterns      bool  `form:"usePatterns" bson:"usePatterns" json:"usePatterns"`
		UseForceTrailing bool  `form:"useForceTrailing" bson:"useForceTrailing" json:"useForceTrailing"`
		CrashGuard       bool  `form:"crashGuard" bson:"crashGuard" json:"crashGuard"`
		SmartTakeLoss    bool  `form:"smartTakeLoss" bson:"smartTakeLoss" json:"smartTakeLoss"`
		// RegimeHold owns the regime-lens holds on an OPEN position: the 15m
		// shock hold, the long/inverse add veto and the profit hold. It never
		// touches the first fill — that is Cooldown's — and it is the only
		// flag those gates answer to (run 97/98: they fired for any strategy
		// that happened to fetch the verdict for another flag).
		RegimeHold bool `form:"regimeHold" bson:"regimeHold" json:"regimeHold"`
		// PowerLawQuantiles is reserved: plumbed end to end, read by nothing
		// yet.
		PowerLawQuantiles bool `form:"powerLawQuantiles" bson:"powerLawQuantiles" json:"powerLawQuantiles"`
	}
)

// NeedsSophos is true when any flag that consumes a sophos verdict is on.
//
// FETCH IS NOT GATE. These predicates decide only what is fetched. A flag
// that fetches a payload gains no gate from it: CrashGuard and SmartTakeLoss
// fetch /patterns (which carries the regime block) and UseAI fetches the ML
// route (which carries it too when no pattern leg exists), yet none of them
// may run a regime gate. Every gate in ShouldHold keys on its own flag first
// and treats payload presence only as a degrade-open check.
func (p StrategyParams) NeedsSophos() bool {
	return p.UseAI || p.UsePatterns || p.CrashGuard || p.SmartTakeLoss || p.RegimeHold
}

// NeedsPatternRoute is the GET /:symbol/patterns fetch: the pattern verdict,
// plus the regime block (RegimeHold), crash and continuation, which all live
// on that payload.
func (p StrategyParams) NeedsPatternRoute() bool {
	return p.UsePatterns || p.CrashGuard || p.SmartTakeLoss || p.RegimeHold
}

// NeedsAIRoute is the GET /:symbol ML fetch.
func (p StrategyParams) NeedsAIRoute() bool {
	return p.UseAI
}

// InjectsEntryHold is true when the first-buy action chain should include
// shouldHold. This is a PRODUCT rule: cooldown owns the first-fill lens, and
// UseAI owns its legacy entry veto; UsePatterns keeps its seat for the
// futures pre-chain. CrashGuard, SmartTakeLoss and RegimeHold gate only adds
// and exits by design, so a strategy running just those keeps its first buy
// ungated (TestStrategyParamsNeedsSophosAndEntryHold pins this).
func (p StrategyParams) InjectsEntryHold() bool {
	return p.Cooldown || p.UseAI || p.UsePatterns
}
