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
		Status          Status          `gorm:"default:active;index;index:idx_dashboard_stats,priority:3;index:idx_user_status,priority:2;" bson:"status" json:"status"`
		CreatedAt       time.Time       `form:"createdAt" json:"createdAt" xml:"createdAt"`
		UpdatedAt       time.Time       `gorm:"index;index:idx_dashboard_stats,priority:4" form:"updatedAt" json:"updatedAt" xml:"updatedAt"`
		DeletedAt       gorm.DeletedAt  `gorm:"index" form:"deletedAt" json:"-" xml:"deletedAt"`
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
		// PatternAction is the GET /:symbol/patterns side. Kept separate from
		// AIAction so UseAI (ML) and UsePatterns can both be on.
		PatternAction string
		// The 15m chart-pattern verdict from GET /:symbol/patterns. Zero
		// values mean "no pattern" (older sophos, no detection). Direction is
		// the detector set that fired ("long" | "short"), independent of the
		// sophos regime veto that turns Action into HOLD on a shock headline;
		// Level/LevelKind is the structure the pattern is built on
		// (resistance, support, neckline, breakout).
		PatternName        string
		PatternDisplayName string
		PatternDirection   string
		PatternScore       float64
		PatternLevel       float64
		PatternLevelKind   string
		PatternStopLoss    float64
		PatternTakeProfit  float64
		PatternInterval    string
		// Fibonacci retracement of the last 15m up-swing; Levels descend
		// (0.382, 0.5, 0.618, 0.786 of the swing). Empty means no swing.
		FibSwingLow  float64
		FibSwingHigh float64
		FibLevels    []float64
		// Smart take loss: the chart block from GET /:symbol/patterns — the
		// quiet slow-decline verdict with its sell band, and the capital
		// protection band with its SMC trend reading, both read on the closed
		// window of sophos' smart take loss. Every zero field is inert; the
		// engines hand it to gates/smarttakeloss.Apply after the ladder has
		// decided, and ShouldHold reads the slow-decline verdict on the first
		// fill.
		SmartTakeLoss SmartTakeLossIndicators
		// Dynamic params: the Super Guppy and Bull Market Support Band reads
		// of the chart row of sophos' SMC trend dashboard, from
		// GET /:symbol/smc-trend at dynamicparams.Interval
		// (SophosSmcTrend.DynamicParams), set by the engines after the merge.
		// The zero block is not read and raises nothing. The engines consult
		// it only when a ladder opens: dynamicparams.Opening turns it into the
		// ladder's opened row, and from then on every tick rebuilds the
		// ladder's rows from that row (dynamicparams.RaisedSettings), never
		// from these reads. No gate holds on it.
		DynamicParams DynamicParamsIndicators
	}

	// AssetFree is one asset's free balance on the wallet the managed trade
	// spends from, as of the tick that carries it. The engines read it where
	// they read the funds gate's own budget, so the gate that consumes it
	// compares against the same number HasFunds would.
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
		// WalletLadders is every active parent trade of this wallet, the
		// managed one included while it is active; a ladder blocked on its
		// next entry is left out, since it cannot take that entry and the
		// others must not wait for a retry only a close could fund. The
		// cooldown depth-priority gate reads it to find the deepest ladder
		// that has filled at least one entry, and then what that ladder says
		// its remaining entries cost — the amount the wallet is kept for. A
		// full ladder keeps its place at a reserve of zero until it closes; full
		// means full at the ceiling of the rows the ladder trades, which a
		// ladder that opened raised has read off its own logs (ladder.DepthOf).
		// Set by the engines only on the ticks
		// cooldown.DepthPriorityApplies says can consume it; nil elsewhere,
		// and a nil slice holds nothing.
		WalletLadders []LadderDepth
		// WalletFree is the wallet's free balance per asset as of this tick,
		// the other half of that reserve: the gate holds an entry only when
		// placing it would leave the wallet under what the deepest ladder
		// still needs. Filled by the engines on the same ticks WalletLadders
		// is, nil elsewhere — and an asset with no entry here is an UNKNOWN
		// balance, which holds nothing rather than guessing.
		WalletFree []AssetFree
		// AvailableQuantity is the wallet balance the entry being placed must
		// not exceed, counted in the asset that entry spends: the quote asset
		// on a spot buy, the base asset on an inverse one. HasFunds sets it
		// only on the ticks where it waives a shortfall; zero leaves the
		// entry sized by the ladder alone.
		AvailableQuantity float64
		// EntrySettings is the rows the FIRST entry of the ladder is sized
		// with when set; nil leaves it on the trade's own rows. The engines
		// set it to the rows dynamicparams.RaisedSettings raises, on the
		// ticks it raises them. Only the first-entry sizing reads it, through
		// SizingTrade: the adds and everything written keep the trade's rows.
		// The readers of the ladder's depths do not take the raise from here —
		// they read it off the trade's own logs, so a wallet view built from
		// a trade alone agrees with the engine that ticks it. Never persisted.
		EntrySettings []StrategySettings
	}
)
