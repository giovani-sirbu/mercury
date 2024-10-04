package aggragates

import (
	"gorm.io/gorm"
	"time"
)

type (
	Trades struct {
		ID              uint            `gorm:"primaryKey" form:"id" json:"id" xml:"id"`
		UserID          uint            `gorm:"index:idx_dashboard_stats,priority:1" form:"userId" json:"userId" xml:"userId"`
		ParentID        uint            `gorm:"index" form:"parentId" json:"parentId" xml:"parentId"`
		Symbol          string          `gorm:"type:varchar(10); index" bson:"symbol" json:"symbol"`
		ParentSymbol    string          `gorm:"type:varchar(10); index" bson:"parentSymbol" json:"parentSymbol"`
		PositionType    string          `gorm:"type:varchar(50); default:new" bson:"positionType" json:"positionType"`
		PositionPrice   float64         `bson:"positionPrice" json:"positionPrice"`
		ExchangeID      int             `gorm:"index:idx_dashboard_stats,priority:2" form:"exchangeId" json:"-" xml:"exchangeId"`
		Exchange        TradesExchanges `form:"exchange" json:"exchange" xml:"exchange"`
		StrategyID      int             `form:"strategyId" json:"-" xml:"strategyId"`
		Strategy        Strategies      `gorm:"foreignKey:StrategyID;references:ID"  form:"strategy" json:"strategy" xml:"strategy"`
		Settings        SettingsPairs   `gorm:"foreignKey:StrategyID;references:StrategyID"  form:"settings" json:"settings" xml:"settings"`
		USDProfit       float64         `bson:"usdProfit" json:"usdProfit"`
		Profit          float64         `bson:"profit" json:"profit"`
		ProfitAsset     string          `bson:"profitAsset" json:"profitAsset"`
		PreventNewTrade bool            `gorm:"type:boolean;default:false" bson:"preventNewTrade" json:"preventNewTrade"`
		Inverse         bool            `gorm:"type:boolean;default:false" bson:"inverse" json:"inverse"`
		PendingOrder    int64           `gorm:"index" bson:"pendingOrder" json:"pendingOrder"`
		History         []TradesHistory `gorm:"foreignKey:TradeID;references:ID" bson:"history" json:"history"`
		Logs            []TradesLogs    `gorm:"foreignKey:TradeID;references:ID" bson:"logs" json:"logs"`
		Status          Status          `gorm:"default:active;index;index:idx_dashboard_stats,priority:3" bson:"status" json:"status"`
		CreatedAt       time.Time       `form:"createdAt" json:"createdAt" xml:"createdAt"`
		UpdatedAt       time.Time       `form:"updatedAt;index:idx_dashboard_stats,priority:4" json:"updatedAt" xml:"updatedAt"`
		DeletedAt       gorm.DeletedAt  `form:"deletedAt" json:"-" xml:"deletedAt"`
	}
	Params struct {
		OldPositionPrice float64
		Percentage       float64
		OldPosition      string
		PreventInfoLog   bool
		Quantity         float64
		Profit           float64
	}
)
