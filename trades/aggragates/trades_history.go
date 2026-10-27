package aggragates

import (
	"time"
)

type TradesHistory struct {
	ID       uint         `gorm:"primaryKey" form:"id" json:"id" xml:"id"`
	TradeID  uint         `gorm:"uniqueIndex:idx_trade_id_order_id,priority:1" form:"tradeId" json:"tradeId" xml:"tradeId"`
	Type     string       `bson:"type" json:"type"`
	Quantity float64      `bson:"quantity" json:"quantity"`
	Price    float64      `bson:"price" json:"price"`
	Fees     []TradesFees `gorm:"foreignKey:HistoryID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"  bson:"fees" json:"fees"`
	OrderId  int64        `gorm:"uniqueIndex:idx_trade_id_order_id,priority:2" bson:"orderId" json:"orderId"`
	Status   string       `bson:"status" json:"status"`
	// FilledAt is when the order executed in full: the simulated tick a replay settled it on, the
	// reconciliation in production; nil while it rests or fills in part. CreatedAt stays the
	// placement in a replay, the stamp the gates read a fill at.
	FilledAt  *time.Time `json:"filledAt"`
	CreatedAt time.Time  `gorm:"index" json:"createdAt"`
	UpdatedAt time.Time  `gorm:"index" json:"updatedAt"`
}

// FilledMoment is when the row's order executed: FilledAt when stamped, else CreatedAt.
func (h TradesHistory) FilledMoment() time.Time {
	if h.FilledAt != nil && !h.FilledAt.IsZero() {
		return *h.FilledAt
	}
	return h.CreatedAt
}
