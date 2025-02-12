package aggragates

import (
	"time"

	"gorm.io/gorm"
)

type (
	Trades struct {
		ID              uint            `gorm:"primaryKey" form:"id" json:"id" xml:"id"`
		UserID          uint            `gorm:"index:idx_dashboard_stats,priority:1" form:"userId" json:"userId" xml:"userId"`
		ParentID        uint            `gorm:"index" form:"parentId" json:"parentId" xml:"parentId"`
		Symbol          string          `gorm:"type:varchar(10);uniqueIndex:idx_symbol_strategy_id,priority:1;" bson:"symbol" json:"symbol"`
		PositionType    string          `gorm:"type:varchar(50); default:new" bson:"positionType" json:"positionType"`
		PositionPrice   float64         `bson:"positionPrice" json:"positionPrice"`
		ExchangeID      int             `gorm:"index:idx_dashboard_stats,priority:2;constraint:OnDelete:CASCADE;" form:"exchangeId" json:"exchangeId" xml:"exchangeId"`
		Exchange        TradesExchanges `form:"exchange" json:"exchange" xml:"exchange"`
		ExchangeName    string          `gorm:"type:varchar(50);uniqueIndex:idx_symbol_strategy_id,priority:3;" bson:"exchangeName" json:"-"`
		StrategyID      int             `gorm:"uniqueIndex:idx_symbol_strategy_id,priority:2;" form:"strategyId" json:"strategyId" xml:"strategyId"`
		Strategy        Strategies      `gorm:"foreignKey:StrategyID;references:ID"  form:"strategyInfo" json:"strategyInfo" xml:"strategyInfo"`
		StrategyPair    StrategiesPairs `gorm:"foreignKey:Symbol,StrategyID,ExchangeName;references:Symbol,StrategyID,Exchange" json:"strategyPair"`
		USDProfit       float64         `bson:"usdProfit" json:"usdProfit"`
		Profit          float64         `bson:"profit" json:"profit"`
		ProfitAsset     string          `bson:"profitAsset" json:"profitAsset"`
		Dust            float64         `bson:"dust" json:"dust"`
		PreventNewTrade bool            `gorm:"type:boolean;default:false" bson:"preventNewTrade" json:"preventNewTrade"`
		Inverse         bool            `gorm:"type:boolean;default:false" bson:"inverse" json:"inverse"`
		PendingOrder    int64           `gorm:"index" bson:"pendingOrder" json:"pendingOrder"`
		History         []TradesHistory `gorm:"foreignKey:TradeID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" bson:"history" json:"history"`
		Logs            []TradesLogs    `gorm:"foreignKey:TradeID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" bson:"logs" json:"logs"`
		Status          Status          `gorm:"default:active;index;index:idx_dashboard_stats,priority:3" bson:"status" json:"status"`
		CreatedAt       time.Time       `form:"createdAt" json:"createdAt" xml:"createdAt"`
		UpdatedAt       time.Time       `form:"updatedAt;index:idx_dashboard_stats,priority:4" json:"updatedAt" xml:"updatedAt"`
		DeletedAt       gorm.DeletedAt  `form:"deletedAt" json:"-" xml:"deletedAt"`
	}
	UsedAmountResult struct {
		UsedAmount    float64 `json:"usedAmount"`
		QuoteCurrency string  `json:"quoteCurrency"`
	}

	CoolDownIndicators struct {
		VolatilityScore float64 `json:"volatilityScore"`
		MarketBullish   bool    `json:"marketBullish"`
		MarketBearish   bool    `json:"marketBearish"`
	}

	Params struct {
		OldPositionPrice   float64
		Percentage         float64
		OldPosition        string
		PreventInfoLog     bool
		Quantity           float64
		Profit             float64
		InverseUsedAmount  []UsedAmountResult
		CoolDownIndicators CoolDownIndicators
	}
)
