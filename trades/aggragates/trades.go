package aggragates

import (
	"gorm.io/gorm"
	"time"
)

type Trades struct {
	ID              uint            `gorm:"primaryKey" form:"id" json:"id" xml:"id"`
	UserID          uint            `gorm:"index" form:"userId" json:"userId" xml:"userId"`
	ParentID        uint            `gorm:"index" form:"parentId" json:"parentId" xml:"parentId"`
	Symbol          string          `gorm:"type:varchar(10); index" bson:"symbol" json:"symbol"`
	PositionType    string          `gorm:"type:varchar(50); default:new" bson:"positionType" json:"positionType"`
	PositionPrice   float64         `bson:"positionPrice" json:"positionPrice"`
	ExchangeID      int             `form:"exchangeId" json:"-" xml:"exchangeId"`
	Exchange        TradesExchanges `form:"exchange" json:"exchange" xml:"exchange"`
	StrategyID      int             `form:"strategyId" json:"-" xml:"strategyId"`
	Strategy        Strategies      `gorm:"foreignKey:StrategyID;references:ID"  form:"strategy" json:"strategy" xml:"strategy"`
	USDProfit       float64         `bson:"usdProfit" json:"usdProfit"`
	Profit          float64         `bson:"profit" json:"profit"`
	ProfitAsset     string          `bson:"profitAsset" json:"profitAsset"`
	PreventNewTrade bool            `gorm:"type:boolean;default:false" bson:"preventNewTrade" json:"preventNewTrade"`
	Inverse         bool            `gorm:"type:boolean;default:false" bson:"inverse" json:"inverse"`
	PendingOrder    int64           `bson:"pendingOrder" json:"pendingOrder"`
	History         []TradesHistory `gorm:"foreignKey:TradeID;references:ID" bson:"history" json:"history"`
	Logs            []TradesLogs    `gorm:"foreignKey:TradeID;references:ID" bson:"logs" json:"logs"`
	Status          Status          `gorm:"default:active;index" bson:"status" json:"status"`
	CreatedAt       time.Time       `form:"createdAt" json:"createdAt" xml:"createdAt"`
	UpdatedAt       time.Time       `form:"updatedAt" json:"updatedAt" xml:"updatedAt"`
	DeletedAt       gorm.DeletedAt  `form:"deletedAt" json:"-" xml:"deletedAt"`
}
