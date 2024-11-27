package aggragates

import (
	"gorm.io/gorm"
	"time"
)

type TradesExchanges struct {
	ID           uint           `gorm:"primaryKey" form:"id" json:"id" xml:"id"`
	UserID       uint           `form:"userId" json:"-" xml:"userId"`
	Name         string         `gorm:"type:varchar(50)" bson:"name" json:"name" form:"name" xml:"name" validate:"required,min=3,max=50"`
	Label        string         `gorm:"type:varchar(50)" bson:"label" json:"label" form:"label" xml:"label" validate:"required,min=3,max=50"`
	ApiKey       string         `gorm:"type:varchar(200)" bson:"apiKey" json:"apiKey" form:"apiKey" xml:"apiKey" validate:"required,min=10,max=150"`
	ApiSecret    string         `gorm:"type:varchar(200)" bson:"apiSecret" json:"apiSecret" form:"apiSecret" xml:"apiSecret" validate:"required,min=10,max=150"`
	TestNet      bool           `gorm:"type:boolean;default:false" bson:"testNet" json:"testNet" form:"testNet" xml:"testNet"`
	TradesStatus Status         `gorm:"type:varchar(50);default:new" bson:"tradesStatus" json:"tradesStatus" form:"tradesStatus" xml:"tradesStatus"`
	CreatedAt    time.Time      `form:"createdAt" json:"-" xml:"createdAt"`
	UpdatedAt    time.Time      `form:"updatedAt" json:"-" xml:"updatedAt"`
	DeletedAt    gorm.DeletedAt `form:"deletedAt" json:"-" xml:"deletedAt"`
}
