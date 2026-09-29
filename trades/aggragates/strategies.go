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
		// RegimeHold owns the regime holds on an OPEN position: the shock
		// hold, the long/inverse add veto and the profit hold. It never
		// touches the first fill — that is Cooldown's — and it is the only
		// flag those gates answer to. Were they keyed on payload presence
		// instead, they would fire for any strategy that merely fetches the
		// verdict for another flag; that is the FETCH IS NOT GATE rule on
		// NeedsSophos below.
		RegimeHold bool `form:"regimeHold" bson:"regimeHold" json:"regimeHold"`
		// PowerLawQuantiles is reserved: plumbed end to end, read by nothing
		// yet.
		PowerLawQuantiles bool `form:"powerLawQuantiles" bson:"powerLawQuantiles" json:"powerLawQuantiles"`
		// DynamicParams raises a long spot parent ladder's rows while sophos'
		// Super Guppy and Bull Market Support Band reads on one timeframe are
		// bearish: gates/dynamicparams adds BearPercentagePoints to every
		// row's percentage and BearDepths to its depths while both are, and
		// what MixedIncrease names while exactly one is. The raised rows are a
		// per-tick copy the position is computed from and a new ladder's first
		// entry is sized from; nothing is stored, and the flag holds nothing.
		DynamicParams bool `form:"dynamicParams" bson:"dynamicParams" json:"dynamicParams"`
	}
)

// NeedsSophos is true when any flag that consumes a sophos verdict is on.
//
// FETCH IS NOT GATE. These predicates decide only what is fetched. A flag
// that fetches a payload gains no gate from it: CrashGuard and SmartTakeLoss
// fetch /patterns (which carries the regime block) and UseAI fetches the ML
// route (which carries it too when no pattern leg exists), yet none of them
// may run a regime gate. Every gate in ShouldHold keys on its own flag first
// and treats payload presence only as a degrade-open check. DynamicParams
// fetches /patterns for its two reads and gains no gate either: it shapes the
// rows the engines hand the position and the first entry's sizing
// (dynamicparams.RaisedSettings), and holds nothing.
func (p StrategyParams) NeedsSophos() bool {
	return p.UseAI || p.UsePatterns || p.CrashGuard || p.SmartTakeLoss || p.RegimeHold || p.DynamicParams
}

// NeedsPatternRoute is the GET /:symbol/patterns fetch: the pattern verdict,
// plus the regime block (RegimeHold), crash, the smart take loss block and
// the dynamic params reads (DynamicParams), which all live on that payload.
func (p StrategyParams) NeedsPatternRoute() bool {
	return p.UsePatterns || p.CrashGuard || p.SmartTakeLoss || p.RegimeHold || p.DynamicParams
}

// NeedsAIRoute is the GET /:symbol ML fetch.
func (p StrategyParams) NeedsAIRoute() bool {
	return p.UseAI
}

// InjectsEntryHold is true when the first-buy action chain should include
// shouldHold. This is a PRODUCT rule: cooldown owns the first-fill gate,
// UseAI owns its legacy entry veto, and SmartTakeLoss holds a long first fill
// while its quiet slow-decline verdict stands, so no new ladder opens into
// the decline its exit sells out of; UsePatterns keeps its seat for the
// futures pre-chain. CrashGuard and RegimeHold gate only adds and exits by
// design, so a strategy running just those keeps its first buy ungated
// (TestStrategyParamsNeedsSophosAndEntryHold pins this). DynamicParams is not
// here: it sizes a first entry for the raised rows but never holds one.
func (p StrategyParams) InjectsEntryHold() bool {
	return p.Cooldown || p.UseAI || p.UsePatterns || p.SmartTakeLoss
}
