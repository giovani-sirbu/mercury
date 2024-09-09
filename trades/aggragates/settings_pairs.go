package aggragates

import (
	"gorm.io/gorm"
	"time"
)

type SettingsPairs struct {
	ID          uint           `gorm:"primaryKey" form:"id" json:"id" xml:"id"`
	Symbol      string         `gorm:"type:varchar(20)" bson:"symbol" json:"symbol" form:"symbol" xml:"symbol" validate:"required,min=3,max=20"`
	Exchange    string         `gorm:"type:varchar(50)" bson:"exchange" json:"exchange" form:"exchange" xml:"exchange" validate:"required,min=3,max=50"`
	LotSize     uint8          `bson:"lotSize" json:"lotSize" form:"lotSize" xml:"lotSize" validate:"required"`
	PriceFilter uint8          `bson:"priceFilter" json:"priceFilter" form:"priceFilter" xml:"priceFilter" validate:"required"`
	MinNotional float64        `bson:"minNotional" json:"minNotional" form:"minNotional" xml:"minNotional" validate:"required"`
	Rank        uint8          `bson:"rank" json:"rank" form:"rank" xml:"rank" validate:"required"`
	Status      string         `gorm:"default:active" bson:"status" json:"status"`
	Params      string         `gorm:"type:text" bson:"params" json:"params"`
	CreatedAt   time.Time      `form:"createdAt" json:"-" xml:"createdAt"`
	UpdatedAt   time.Time      `form:"updatedAt" json:"-" xml:"updatedAt"`
	DeletedAt   gorm.DeletedAt `form:"deletedAt" json:"-" xml:"deletedAt"`
}
