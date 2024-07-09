package aggregates

type (
	// Exchange structure to initialize an exchange
	Exchange struct {
		ID        uint   `gorm:"primaryKey" form:"id" json:"id" xml:"id"`
		Label     string `gorm:"type:varchar(50)" bson:"label" json:"label" form:"label" xml:"label" validate:"required,min=3,max=50"`
		Name      string `gorm:"type:varchar(50)" bson:"name" json:"name" form:"name" xml:"name" validate:"required,min=3,max=50"`
		ApiKey    string `gorm:"type:varchar(200)" bson:"apiKey" json:"apiKey" form:"apiKey" xml:"apiKey" validate:"omitempty,min=10,max=150"`
		ApiSecret string `gorm:"type:varchar(200)" bson:"apiSecret" json:"apiSecret" form:"apiSecret" xml:"apiSecret" validate:"omitempty,min=10,max=150"`
		TestNet   bool   `gorm:"type:boolean;default:false" bson:"testNet" json:"testNet" form:"testNet" xml:"testNet"`
	}
	// CreateOrderResponse create order structure
	CreateOrderResponse struct {
		Symbol                   string `json:"symbol"`
		OrderID                  int64  `json:"orderId"`
		ClientOrderID            string `json:"clientOrderId"`
		TransactTime             int64  `json:"transactTime"`
		Price                    string `json:"price"`
		OrigQuantity             string `json:"origQty"`
		ExecutedQuantity         string `json:"executedQty"`
		CummulativeQuoteQuantity string `json:"cummulativeQuoteQty"`
		Status                   string `json:"status"`
	}
	// Order structure
	Order struct {
		Symbol                   string `json:"symbol"`
		OrderID                  int64  `json:"orderId"`
		OrderListId              int64  `json:"orderListId"`
		ClientOrderID            string `json:"clientOrderId"`
		Price                    string `json:"price"`
		OrigQuantity             string `json:"origQty"`
		ExecutedQuantity         string `json:"executedQty"`
		CummulativeQuoteQuantity string `json:"cummulativeQuoteQty"`
		Status                   string `json:"status"`
		Type                     string `json:"type"`
		Side                     string `json:"side"`
		StopPrice                string `json:"stopPrice"`
		IcebergQuantity          string `json:"icebergQty"`
		Time                     int64  `json:"time"`
		UpdateTime               int64  `json:"updateTime"`
		OrigQuoteOrderQuantity   string `json:"origQuoteOrderQty"`
	}
	// CancelOrderResponse may be returned included in a CancelOpenOrdersResponse.
	CancelOrderResponse struct {
		Symbol                   string `json:"symbol"`
		OrigClientOrderID        string `json:"origClientOrderId"`
		OrderID                  int64  `json:"orderId"`
		OrderListID              int64  `json:"orderListId"`
		ClientOrderID            string `json:"clientOrderId"`
		TransactTime             int64  `json:"transactTime"`
		Price                    string `json:"price"`
		OrigQuantity             string `json:"origQty"`
		ExecutedQuantity         string `json:"executedQty"`
		CummulativeQuoteQuantity string `json:"cummulativeQuoteQty"`
		Status                   string `json:"status"`
		Type                     string `json:"type"`
		Side                     string `json:"side"`
	}
	Trade struct {
		ID              int64  `json:"id"`
		Symbol          string `json:"symbol"`
		OrderID         int64  `json:"orderId"`
		OrderListId     int64  `json:"orderListId"`
		Price           string `json:"price"`
		Quantity        string `json:"qty"`
		QuoteQuantity   string `json:"quoteQty"`
		Commission      string `json:"commission"`
		CommissionAsset string `json:"commissionAsset"`
		Time            int64  `json:"time"`
		IsBuyer         bool   `json:"isBuyer"`
		IsMaker         bool   `json:"isMaker"`
		IsBestMatch     bool   `json:"isBestMatch"`
		IsIsolated      bool   `json:"isIsolated"`
	}
	// RateLimit struct
	RateLimit struct {
		RateLimitType string `json:"rateLimitType"`
		Interval      string `json:"interval"`
		IntervalNum   int64  `json:"intervalNum"`
		Limit         int64  `json:"limit"`
	}
	// Symbol market symbol
	Symbol struct {
		Symbol              string                   `json:"symbol"`
		BaseAsset           string                   `json:"baseAsset"`
		BaseAssetPrecision  int                      `json:"baseAssetPrecision"`
		QuoteAsset          string                   `json:"quoteAsset"`
		QuotePrecision      int                      `json:"quotePrecision"`
		QuoteAssetPrecision int                      `json:"quoteAssetPrecision"`
		Filters             []map[string]interface{} `json:"filters"`
	}

	// ExchangeInfo exchange info
	ExchangeInfo struct {
		Timezone        string        `json:"timezone"`
		ServerTime      int64         `json:"serverTime"`
		RateLimits      []RateLimit   `json:"rateLimits"`
		ExchangeFilters []interface{} `json:"exchangeFilters"`
		Symbols         []Symbol      `json:"symbols"`
	}

	// TradeFeeDetails represents details about fees
	TradeFeeDetails struct {
		Symbol          string `json:"symbol"`
		MakerCommission string `json:"makerCommission"`
		TakerCommission string `json:"takerCommission"`
	}

	CommissionRates struct {
		Maker  string `json:"maker"`
		Taker  string `json:"taker"`
		Buyer  string `json:"buyer"`
		Seller string `json:"seller"`
	}

	Account struct {
		MakerCommission  int64           `json:"makerCommission"`
		TakerCommission  int64           `json:"takerCommission"`
		BuyerCommission  int64           `json:"buyerCommission"`
		SellerCommission int64           `json:"sellerCommission"`
		CommissionRates  CommissionRates `json:"commissionRates"`
		CanTrade         bool            `json:"canTrade"`
		CanWithdraw      bool            `json:"canWithdraw"`
		CanDeposit       bool            `json:"canDeposit"`
		UpdateTime       uint64          `json:"updateTime"`
		AccountType      string          `json:"accountType"`
		Permissions      []string        `json:"permissions"`
	}

	UserAssetRecord struct {
		Asset        string `json:"asset"`
		Free         string `json:"free"`
		Locked       string `json:"locked"`
		Freeze       string `json:"freeze"`
		Withdrawing  string `json:"withdrawing"`
		Ipoable      string `json:"ipoable"`
		BtcValuation string `json:"btcValuation"`
	}

	// Actions All exchange actions types
	Actions struct {
		Buy             func(symbol string, quantity float64, price string) (CreateOrderResponse, error)
		Sell            func(symbol string, quantity float64, price string) (CreateOrderResponse, error)
		MarketBuy       func(symbol string, quantity float64) (CreateOrderResponse, error)
		MarketSell      func(symbol string, quantity float64) (CreateOrderResponse, error)
		GetOrder        func(orderId int64, symbol string) (Order, error)
		CancelOrder     func(orderId int64, symbol string) (CancelOrderResponse, error)
		GetTrades       func(orderId int64, symbol string) ([]Trade, error)
		GetExchangeInfo func(symbol string) (ExchangeInfo, error)
		GetFees         func(symbol string) (TradeFeeDetails, error)
		GetPrice        func(symbol string) (float64, error)
		GetProfile      func() (Account, error)
		GetUserAssets   func() ([]UserAssetRecord, error)
	}
)
