package aggragates

type (
	Strategies struct {
		ID        uint           `gorm:"primaryKey" form:"id" json:"id" xml:"id"`
		Name      string         `gorm:"type:varchar(50)" bson:"name" json:"name" form:"name" xml:"name" validate:"required,min=3,max=50"`
		TradeType TradeTypes     `gorm:"type:varchar(50); default:spot" bson:"tradeType" json:"tradeType" form:"tradeType" xml:"tradeType"`
		Params    StrategyParams `gorm:"type:jsonb;serializer:json;" bson:"params" json:"params" form:"params" xml:"params"`
	}
	StrategyParams struct {
		Pairs    uint8 `form:"pairs" json:"pairs" xml:"pairs"`
		Impasse  bool  `form:"impasse" bson:"impasse" json:"impasse"`
		Cooldown bool  `form:"cooldown" bson:"cooldown" json:"cooldown"`
		UseAI    bool  `form:"useAI" bson:"useAI" json:"useAI"`
		SkipHold bool  `form:"skipHold" bson:"skipHold" json:"skipHold"`
	}
)
