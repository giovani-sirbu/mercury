package aggragates

import (
	"time"
)

type TradesFees struct {
	ID        uint      `gorm:"primaryKey" form:"id" json:"id" xml:"id"`
	TradeID   uint      `form:"tradeId" json:"tradeId" xml:"tradeId"`
	HistoryID uint      `form:"historyId" json:"historyId" xml:"historyId"`
	Asset     string    `bson:"asset" json:"asset"`
	Fee       float64   `bson:"fee" json:"fee"`
	CreatedAt time.Time `form:"createdAt" json:"createdAt" xml:"createdAt"`
	UpdatedAt time.Time `form:"updatedAt" json:"updatedAt" xml:"updatedAt"`
}
