package aggregates

// Exchange structure to initialize an exchange
type Exchange struct {
	Name      string `bson:"name" json:"name"`
	ApiKey    string `bson:"apiKey" json:"apiKey"`
	ApiSecret string `bson:"apiSecret" json:"apiSecret"`
	TestNet   bool   `bson:"testNet" json:"testNet"`
}

// CreateOrderResponse create order structure
type CreateOrderResponse struct {
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
type Order struct {
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
type CancelOrderResponse struct {
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

type Trade struct {
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
type RateLimit struct {
	RateLimitType string `json:"rateLimitType"`
	Interval      string `json:"interval"`
	IntervalNum   int64  `json:"intervalNum"`
	Limit         int64  `json:"limit"`
}

// ExchangeInfo exchange info
type ExchangeInfo struct {
	Timezone        string        `json:"timezone"`
	ServerTime      int64         `json:"serverTime"`
	RateLimits      []RateLimit   `json:"rateLimits"`
	ExchangeFilters []interface{} `json:"exchangeFilters"`
	Symbols         []string      `json:"symbols"`
}

// TradeFeeDetails represents details about fees
type TradeFeeDetails struct {
	Symbol          string `json:"symbol"`
	MakerCommission string `json:"makerCommission"`
	TakerCommission string `json:"takerCommission"`
}

type CommissionRates struct {
	Maker  string `json:"maker"`
	Taker  string `json:"taker"`
	Buyer  string `json:"buyer"`
	Seller string `json:"seller"`
}

type Account struct {
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

type UserAssetRecord struct {
	Asset        string `json:"asset"`
	Free         string `json:"free"`
	Locked       string `json:"locked"`
	Freeze       string `json:"freeze"`
	Withdrawing  string `json:"withdrawing"`
	Ipoable      string `json:"ipoable"`
	BtcValuation string `json:"btcValuation"`
}

// Actions All exchange actions types
type Actions struct {
	Buy             func(symbol string, quantity float64, price string) (CreateOrderResponse, error)
	Sell            func(symbol string, quantity float64, price string) (CreateOrderResponse, error)
	MarketBuy       func(symbol string, quantity float64) (CreateOrderResponse, error)
	MarketSell      func(symbol string, quantity float64) (CreateOrderResponse, error)
	GetOrder        func(orderId int64, symbol string) (Order, error)
	CancelOrder     func(orderId int64, symbol string) (CancelOrderResponse, error)
	GetTrades       func(orderId int64, symbol string) ([]Trade, error)
	GetExchangeInfo func(symbol string) (ExchangeInfo, error)
	GetFees         func(symbol string) (TradeFeeDetails, error)
	GetProfile      func() (Account, error)
	GetUserAssets   func() ([]UserAssetRecord, error)
}
