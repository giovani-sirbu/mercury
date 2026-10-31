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
		SmartSell        bool  `form:"smartSell" bson:"smartSell" json:"smartSell"`
		// DynamicParams decides a long spot parent ladder's rows once, as it
		// opens, on sophos' SMC trend reads (dynamicparams.Opening); every
		// later tick trades the raised rows its opened event carries. The flag holds nothing.
		DynamicParams bool `form:"dynamicParams" bson:"dynamicParams" json:"dynamicParams"`
	}
)

// NeedsSophos is true when any flag consumes a sophos verdict. Fetch is not
// gate: these predicates decide only what is fetched, and a payload fetched
// for one flag's sake arms no gate of another (SmartSell fetches
// /patterns, yet only UsePatterns may run a pattern hold).
func (p StrategyParams) NeedsSophos() bool {
	return p.UseAI || p.UsePatterns || p.SmartSell || p.DynamicParams
}

// NeedsPatternRoute is the /patterns fetch: the pattern verdict and the smart sell block.
func (p StrategyParams) NeedsPatternRoute() bool {
	return p.UsePatterns || p.SmartSell
}

// NeedsSmcTrendRoute is the /smc-trend fetch for the DynamicParams reads; it arms no hold.
func (p StrategyParams) NeedsSmcTrendRoute() bool {
	return p.DynamicParams
}

// NeedsAIRoute is the GET /:symbol ML fetch.
func (p StrategyParams) NeedsAIRoute() bool {
	return p.UseAI
}

// InjectsEntryHold is true when the first-buy chain should include shouldHold:
// Cooldown, UseAI, UsePatterns and SmartSell own first-fill holds;
// DynamicParams holds nothing.
func (p StrategyParams) InjectsEntryHold() bool {
	return p.Cooldown || p.UseAI || p.UsePatterns || p.SmartSell
}
