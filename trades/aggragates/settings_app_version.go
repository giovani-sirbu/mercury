package aggragates

import (
	"gorm.io/gorm"
	"time"
)

type AppVersion struct {
	ID             uint           `gorm:"primaryKey" form:"id" json:"id" xml:"id"`
	Os             string         `gorm:"type:varchar(50)" form:"os" json:"os" xml:"os"`
	Version        float64        `gorm:"" form:"version" json:"version" xml:"version"`
	UpdateRequired bool           `gorm:"default:false" bson:"update_required" json:"update_required" form:"update_required" xml:"update_required"`
	Description    string         `gorm:"type:varchar(500)" form:"description" json:"description" xml:"description"`
	CreatedAt      time.Time      `form:"createdAt" json:"-" xml:"createdAt"`
	UpdatedAt      time.Time      `form:"updatedAt" json:"-" xml:"updatedAt"`
	DeletedAt      gorm.DeletedAt `form:"deletedAt" json:"-" xml:"deletedAt"`
}
