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
		SmartTakeLoss    bool  `form:"smartTakeLoss" bson:"smartTakeLoss" json:"smartTakeLoss"`
		// DynamicParams decides a long spot parent ladder's rows once, as it
		// opens, on sophos' Super Guppy and Bull Market Support Band reads
		// (NeedsSmcTrendRoute). On each tick that judges the ladder before its
		// first entry fills, reads that raise something have
		// dynamicparams.Opening name the amounts the ladder opens with, which
		// the engines record as its opened strategy event (with a
		// human-readable row beside it): what it adds to every row's
		// percentage and depths — both bearish raise both
		// (dynamicparams.IncreaseBoth), exactly one what
		// dynamicparams.MixedIncrease names. From then on every tick trades
		// the stored rows raised by the amounts that event carries
		// (dynamicparams.RaisedSettings), whatever the reads say, until the
		// ladder closes. The flag holds nothing.
		DynamicParams bool `form:"dynamicParams" bson:"dynamicParams" json:"dynamicParams"`
	}
)

// NeedsSophos is true when any flag that consumes a sophos verdict is on.
//
// FETCH IS NOT GATE. These predicates decide only what is fetched. A flag
// that fetches a payload gains no gate from it: SmartTakeLoss fetches
// /patterns, which carries the pattern verdict, yet only UsePatterns may run
// a pattern hold. Every gate in ShouldHold keys on its own flag first and
// treats payload presence only as a degrade-open check. DynamicParams
// fetches /smc-trend for its two reads and gains no gate either: it decides
// the rows a ladder opens with (dynamicparams.Opening), which the engines
// hand the position and the first entry's sizing
// (dynamicparams.RaisedSettings), and holds nothing.
func (p StrategyParams) NeedsSophos() bool {
	return p.UseAI || p.UsePatterns || p.SmartTakeLoss || p.DynamicParams
}

// NeedsPatternRoute is the GET /:symbol/patterns fetch: the pattern verdict,
// plus the smart take loss block (SmartTakeLoss), which lives on that
// payload.
func (p StrategyParams) NeedsPatternRoute() bool {
	return p.UsePatterns || p.SmartTakeLoss
}

// NeedsSmcTrendRoute is the GET /:symbol/smc-trend fetch at
// dynamicparams.Interval: the SMC trend dashboard whose chart row carries the
// DynamicParams reads (SophosSmcTrend.DynamicParams). No other flag reads
// that route, and neither the pattern nor the ML route carries the reads.
//
// FETCH IS NOT GATE here either: the fetch arms no hold, and the reads decide
// nothing on their own — the engines hand them to dynamicparams.Opening for a
// `new` ladder alone, and a ladder past its opening never reads them again.
func (p StrategyParams) NeedsSmcTrendRoute() bool {
	return p.DynamicParams
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
// futures pre-chain. DynamicParams is not here: it sizes a first entry for
// the raised rows but never holds one.
func (p StrategyParams) InjectsEntryHold() bool {
	return p.Cooldown || p.UseAI || p.UsePatterns || p.SmartTakeLoss
}
