package aggragates

type (
	Strategies struct {
		ID     uint   `gorm:"primaryKey" form:"id" json:"id" xml:"id"`
		Name   string `gorm:"type:varchar(50)" bson:"name" json:"name" form:"name" xml:"name" validate:"required,min=3,max=50"`
		Params string `gorm:"type:text" bson:"params" json:"params"`
	}
	StrategyInfoSettings struct {
		Pairs   uint8 `form:"pairs" json:"pairs" xml:"pairs"`
		Impasse bool  `form:"impasse" bson:"impasse" json:"impasse"`
	}
	Strategy struct {
		ID       uint                 `gorm:"primaryKey" form:"id" json:"id" xml:"id"`
		Name     string               `gorm:"type:varchar(50)" bson:"name" json:"name" form:"name" xml:"name" validate:"required,min=3,max=50"`
		Settings StrategyInfoSettings `gorm:"type:jsonb;serializer:json;" bson:"settings" json:"settings" form:"settings" xml:"settings"`
	}
)
