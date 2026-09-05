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
		VolatilityScore     float64 `json:"volatilityScore"`
		MarketBullish       bool    `json:"marketBullish"`
		MarketBearish       bool    `json:"marketBearish"`
		HasFirstFillVerdict bool    `json:"hasFirstFillVerdict"`
		AllowLongEntry      bool    `json:"allowLongEntry"`
		AllowShortEntry     bool    `json:"allowShortEntry"`
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
		// regime veto applied to Action; Level/LevelKind is the structure the
		// pattern is built on (resistance, support, neckline, breakout).
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
		// Multi-timeframe regime verdict, served by sophos when it has enough
		// closed candles. HasRegimeVerdict is the degrade-open switch: while
		// false (older sophos, or a cache entry written before the field
		// existed) the RegimeHold gates read nothing below. It is never a
		// switch-on: the gates answer to StrategyParams.RegimeHold first.
		HasRegimeVerdict bool
		// EnterAllowed is served by sophos and deliberately UNREAD by the
		// engine: the regime lens has no seat on the first fill (Cooldown
		// owns it), so sophos' long-only enterAllowed and the
		// regimeEntryRequires1h / regimeEntryShockAnyVeto knobs folded into
		// it have no effect here. Kept on the wire for older readers.
		EnterAllowed bool
		// AddAllowed IS read, for long adds only: false when 4h or 1h reads
		// downtrend-persist (sophos regime/set.go). Inverse adds mirror the
		// rule locally from Regimes.
		AddAllowed bool
		Regime     string
		Regimes    map[string]string
		// Crash guard: a market-wide flush is in progress (10.10.2025-style).
		// Arms the ladder widening and the deep-trade rebuy hold; distinct from
		// a single-symbol volatility shock, which travels as the "shock-*"
		// labels in Regimes.
		// CrashReasons names the components that carried the score, so the
		// engines can say WHY the guard armed instead of just that it did.
		CrashActive  bool
		CrashScore   float64
		CrashReasons []string
		// CrashSticky is engine-local: this trade already saw an ARM (redis
		// on live, trade logs in backtest). Sophos does not serve it.
		CrashSticky bool
		// Smart take loss: per-symbol continuation-risk verdict. Every field is
		// inert at its zero value — a risk of 0 never crosses the threshold and
		// DailyNatrPct=0 is an explicit bail — so unlike the regime verdict no
		// HasRegimeVerdict-style compatibility switch is needed; the flag below
		// only says the verdict was actually computed (observability + early
		// bail). Down risk endangers long trades, up risk endangers inverse
		// ones; reversal evidence in the trade's favor vetoes a forced exit.
		HasContinuationVerdict bool
		DownContinuationRisk   float64
		UpContinuationRisk     float64
		ReversalUpEvidence     float64
		ReversalDownEvidence   float64
		DailyNatrPct           float64
		ContinuationReasons    []string
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
		// PortfolioBlocked: another trade of this wallet is funds-blocked.
		// A profitable close is then the capital the ladder waits for, so
		// the regime profit hold stands down. Set by the engines on a
		// takeProfit tick; zero elsewhere.
		PortfolioBlocked bool
	}
)
