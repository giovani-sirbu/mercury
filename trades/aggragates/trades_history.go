package aggragates

import (
	"time"
)

type TradesHistory struct {
	ID        uint                `gorm:"primaryKey" form:"id" json:"id" xml:"id"`
	TradeID   uint                `gorm:"uniqueIndex:idx_trade_id_order_id,priority:1" form:"tradeId" json:"tradeId" xml:"tradeId"`
	Type      string              `bson:"type" json:"type"`
	Quantity  float64             `bson:"quantity" json:"quantity"`
	Price     float64             `bson:"price" json:"price"`
	Fees      []TradesHistoryFees `gorm:"foreignKey:HistoryID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"  bson:"fees" json:"fees"`
	OrderId   int64               `gorm:"uniqueIndex:idx_trade_id_order_id,priority:2" bson:"orderId" json:"orderId"`
	Status    string              `bson:"status" json:"status"`
	CreatedAt time.Time           `json:"createdAt"`
	UpdatedAt time.Time           `json:"updatedAt"`
}
