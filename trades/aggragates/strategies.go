package aggragates

type (
	Strategies struct {
		ID     uint   `gorm:"primaryKey" form:"id" json:"id" xml:"id"`
		Name   string `gorm:"type:varchar(50)" bson:"name" json:"name" form:"name" xml:"name" validate:"required,min=3,max=50"`
		Params string `gorm:"type:text" bson:"params" json:"params"`
	}
	StrategyParams struct {
		Tolerance          float64 `bson:"tolerance" json:"tolerance"`
		TrailingTakeProfit float64 `bson:"trailingTakeProfit" json:"trailingTakeProfit"`
		InitialBid         float64 `bson:"initialBid" json:"initialBid"`
		Name               string  `bson:"name" json:"name"`
		Percentage         float64 `bson:"percentage" json:"percentage"`
		Multiplier         float64 `bson:"multiplier" json:"multiplier"`
	}
)
