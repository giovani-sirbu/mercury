package aggragates

import (
	"gorm.io/gorm"
	"time"
)

type Exchanges struct {
	ID             uint           `gorm:"primaryKey" form:"id" json:"id" xml:"id"`
	Name           string         `gorm:"type:varchar(50)" bson:"name" json:"name" form:"name" xml:"name" validate:"required,min=3,max=50"`
	Icon           string         `gorm:"type:varchar(200)" bson:"icon" json:"icon" form:"icon" xml:"icon" validate:"required"`
	ApiTutorial    string         `gorm:"type:varchar(500)" bson:"apiTutorial" json:"apiTutorial" form:"apiTutorial" xml:"apiTutorial" validate:"required"`
	IpsToWhitelist string         `gorm:"type:varchar(500)" bson:"ipsToWhitelist" json:"ipsToWhitelist" form:"ipsToWhitelist" xml:"ipsToWhitelist" validate:"required"`
	Status         Status         `gorm:"type:varchar(50);default:active" bson:"status" json:"status" form:"status" xml:"status"`
	CreatedAt      time.Time      `form:"createdAt" json:"-" xml:"createdAt"`
	UpdatedAt      time.Time      `form:"updatedAt" json:"-" xml:"updatedAt"`
	DeletedAt      gorm.DeletedAt `form:"deletedAt" json:"-" xml:"deletedAt"`
}
