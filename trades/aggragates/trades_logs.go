package aggragates

import (
	"time"
)

const LOG_INFO = "INFO"
const LOG_WARNING = "WARNING"

type TradesLogs struct {
	ID         uint      `gorm:"primaryKey" form:"id" json:"id" xml:"id"`
	TradeID    uint      `gorm:"index:idx_trade_id_type,priority:1" form:"tradeId" json:"tradeId" xml:"tradeId"`
	Message    string    `bson:"message" json:"message"`
	Type       string    `gorm:"index:idx_trade_id_type,priority:2" bson:"type" json:"type"`
	Quantity   float64   `bson:"quantity" json:"quantity"`
	Price      float64   `bson:"price" json:"price"`
	Percentage float64   `bson:"percentage" json:"percentage"`
	CreatedAt  time.Time `gorm:"index" json:"createdAt"`
	UpdatedAt  time.Time `gorm:"index" json:"updatedAt"`
}
