package aggragates

import (
	"time"

	"gorm.io/gorm"
)

type (
	Trades struct {
		ID              uint            `gorm:"primaryKey" form:"id" json:"id" xml:"id"`
		UserID          uint            `gorm:"index:idx_dashboard_stats,priority:1;index:idx_user_status,priority:1;" form:"userId" json:"userId" xml:"userId"`
		ParentID        uint            `gorm:"index" form:"parentId" json:"parentId" xml:"parentId"`
		Symbol          string          `gorm:"type:varchar(10);uniqueIndex:idx_symbol_strategy_id,priority:1;" bson:"symbol" json:"symbol"`
		PositionType    string          `gorm:"type:varchar(50); default:new" bson:"positionType" json:"positionType"`
		PositionPrice   float64         `bson:"positionPrice" json:"positionPrice"`
		ExchangeID      int             `gorm:"index:idx_dashboard_stats,priority:2;" form:"exchangeId" json:"exchangeId" xml:"exchangeId"`
		Exchange        TradesExchanges `gorm:"foreignKey:ExchangeID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" form:"exchange" json:"exchange" xml:"exchange"`
		ExchangeName    string          `gorm:"type:varchar(50);uniqueIndex:idx_symbol_strategy_id,priority:3;" bson:"exchangeName" json:"-"`
		StrategyID      int             `gorm:"uniqueIndex:idx_symbol_strategy_id,priority:2;" form:"strategyId" json:"strategyId" xml:"strategyId"`
		Strategy        Strategies      `gorm:"foreignKey:StrategyID;references:ID"  form:"strategyInfo" json:"strategyInfo" xml:"strategyInfo"`
		StrategyPair    StrategiesPairs `gorm:"foreignKey:Symbol,StrategyID,ExchangeName;references:Symbol,StrategyID,Exchange" json:"strategyPair"`
		USDProfit       float64         `gorm:"index" bson:"usdProfit" json:"usdProfit"`
		Profit          float64         `bson:"profit" json:"profit"`
		ProfitAsset     string          `bson:"profitAsset" json:"profitAsset"`
		Dust            float64         `bson:"dust" json:"dust"`
		PreventNewTrade bool            `gorm:"type:boolean;default:false" bson:"preventNewTrade" json:"preventNewTrade"`
		Inverse         bool            `gorm:"type:boolean;default:false" bson:"inverse" json:"inverse"`
		PendingOrder    int64           `gorm:"index" bson:"pendingOrder" json:"pendingOrder"`
		History         []TradesHistory `gorm:"foreignKey:TradeID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" bson:"history" json:"history"`
		Logs            []TradesLogs    `gorm:"foreignKey:TradeID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" bson:"logs" json:"logs"`
		// StrategyEvents is the state of the strategy gates, one event per gate execution, append-only.
		StrategyEvents []TradesStrategyEvents `gorm:"foreignKey:TradeID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"strategyEvents"`
		Status         Status                 `gorm:"default:active;index;index:idx_dashboard_stats,priority:3;index:idx_user_status,priority:2;" bson:"status" json:"status"`
		CreatedAt      time.Time              `form:"createdAt" json:"createdAt" xml:"createdAt"`
		UpdatedAt      time.Time              `gorm:"index;index:idx_dashboard_stats,priority:4" form:"updatedAt" json:"updatedAt" xml:"updatedAt"`
		DeletedAt      gorm.DeletedAt         `gorm:"index" form:"deletedAt" json:"-" xml:"deletedAt"`
	}
	UsedAmountResult struct {
		UsedAmount    float64 `json:"usedAmount"`
		QuoteCurrency string  `json:"quoteCurrency"`
	}

	CoolDownIndicators struct {
		HasFirstFillVerdict bool `json:"hasFirstFillVerdict"`
		AllowLongEntry      bool `json:"allowLongEntry"`
		AllowShortEntry     bool `json:"allowShortEntry"`
	}

	AIIndicators struct {
		AIMarketBearish  bool
		AIMarketBullish  bool
		AIAction         string
		AISignalStrength float64
		StayOutReasons   []string
		// PatternAction is the /patterns side, kept apart from AIAction.
		PatternAction string
		// The chart-pattern verdict from /patterns; zero values mean no pattern.
		PatternName        string
		PatternDisplayName string
		PatternDirection   string
		PatternScore       float64
		PatternLevel       float64
		PatternLevelKind   string
		PatternStopLoss    float64
		PatternTakeProfit  float64
		PatternInterval    string
		// Fibonacci retracement of the last up-swing; empty means no swing.
		FibSwingLow  float64
		FibSwingHigh float64
		FibLevels    []float64
		// SmartTakeLoss is the /patterns chart block the engines hand to
		// gates/smarttakeloss.Apply; every zero field is inert.
		SmartTakeLoss SmartTakeLossIndicators
		// DynamicParams is the SMC trend dashboard's reads a ladder opens on
		// (dynamicparams.Opening); the zero block raises nothing.
		DynamicParams DynamicParamsIndicators
	}

	// AssetFree is one asset's free balance on the wallet, as of the tick.
	AssetFree struct {
		Asset string  `json:"asset"`
		Free  float64 `json:"free"`
	}

	Params struct {
		OldPositionPrice   float64
		Percentage         float64
		OldPosition        string
		PreventInfoLog     bool
		MarketSellOrder    bool
		Quantity           float64
		Profit             float64
		InverseUsedAmount  []UsedAmountResult
		CoolDownIndicators CoolDownIndicators
		AIIndicators       AIIndicators
		// WalletLadders is every active parent ladder of this wallet, set by the
		// engines only on ticks depthpriority.DepthPriorityApplies allows; nil holds nothing.
		WalletLadders []LadderDepth
		// WalletFree is the wallet's free balance per asset on the same ticks;
		// an asset with no entry is unknown and holds nothing.
		WalletFree []AssetFree
		// AvailableQuantity is the balance the entry must not exceed, set by
		// HasFunds when it waives a shortfall.
		AvailableQuantity float64
		// EntrySettings sizes the first entry when set (the raised rows); never persisted.
		EntrySettings []StrategySettings
	}
)
