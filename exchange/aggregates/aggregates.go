package aggregates

import (
	"github.com/adshao/go-binance/v2/common"
)

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
		MakerCommission  int64             `json:"makerCommission"`
		TakerCommission  int64             `json:"takerCommission"`
		BuyerCommission  int64             `json:"buyerCommission"`
		SellerCommission int64             `json:"sellerCommission"`
		CommissionRates  CommissionRates   `json:"commissionRates"`
		CanTrade         bool              `json:"canTrade"`
		CanWithdraw      bool              `json:"canWithdraw"`
		CanDeposit       bool              `json:"canDeposit"`
		UpdateTime       uint64            `json:"updateTime"`
		AccountType      string            `json:"accountType"`
		Permissions      []string          `json:"permissions"`
		Balances         []UserAssetRecord `json:"balances"`
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

	PriceWSResponseData struct {
		Price    string `json:"p"`
		Symbol   string `json:"s"`
		Exchange string `json:"exchange"`
	}

	// WsUserDataEvent define user data event
	WsUserDataEvent struct {
		Event string `json:"e"`
		Time  int64  `json:"E"`
		WsAccountUpdateList
		WsBalanceUpdate
		WsOrderUpdate
		WsOCOUpdate
	}

	WsAccountUpdateList struct {
		AccountUpdateTime int64             `json:"u"`
		WsAccountUpdates  []WsAccountUpdate `json:"B"`
	}

	// WsAccountUpdate define account update
	WsAccountUpdate struct {
		Asset  string `json:"a"`
		Free   string `json:"f"`
		Locked string `json:"l"`
	}

	WsBalanceUpdate struct {
		Asset           string `json:"a"`
		Change          string `json:"d"`
		TransactionTime int64  `json:"T"`
	}

	WsOrderUpdate struct {
		Symbol                  string `json:"s"`
		ClientOrderId           string `json:"c"`
		Side                    string `json:"S"`
		Type                    string `json:"o"`
		TimeInForce             string `json:"f"`
		Volume                  string `json:"q"`
		Price                   string `json:"p"`
		StopPrice               string `json:"P"`
		IceBergVolume           string `json:"F"`
		OrderListId             int64  `json:"g"` // for OCO
		OrigCustomOrderId       string `json:"C"` // customized order ID for the original order
		ExecutionType           string `json:"x"` // execution type for this event NEW/TRADE...
		Status                  string `json:"X"` // order status
		RejectReason            string `json:"r"`
		Id                      int64  `json:"i"` // order id
		LatestVolume            string `json:"l"` // quantity for the latest trade
		FilledVolume            string `json:"z"`
		LatestPrice             string `json:"L"` // price for the latest trade
		FeeAsset                string `json:"N"`
		FeeCost                 string `json:"n"`
		TransactionTime         int64  `json:"T"`
		TradeId                 int64  `json:"t"`
		IgnoreI                 int64  `json:"I"` // ignore
		IsInOrderBook           bool   `json:"w"` // is the order in the order book?
		IsMaker                 bool   `json:"m"` // is this order maker?
		IgnoreM                 bool   `json:"M"` // ignore
		CreateTime              int64  `json:"O"`
		FilledQuoteVolume       string `json:"Z"` // the quote volume that already filled
		LatestQuoteVolume       string `json:"Y"` // the quote volume for the latest trade
		QuoteVolume             string `json:"Q"`
		SelfTradePreventionMode string `json:"V"`

		//These are fields that appear in the payload only if certain conditions are met.
		TrailingDelta              int64  `json:"d"` // Appears only for trailing stop orders.
		TrailingTime               int64  `json:"D"`
		StrategyId                 int64  `json:"j"` // Appears only if the strategyId parameter was provided upon order placement.
		StrategyType               int64  `json:"J"` // Appears only if the strategyType parameter was provided upon order placement.
		PreventedMatchId           int64  `json:"v"` // Appears only for orders that expired due to STP.
		PreventedQuantity          string `json:"A"`
		LastPreventedQuantity      string `json:"B"`
		TradeGroupId               int64  `json:"u"`
		CounterOrderId             int64  `json:"U"`
		CounterSymbol              string `json:"Cs"`
		PreventedExecutionQuantity string `json:"pl"`
		PreventedExecutionPrice    string `json:"pL"`
		PreventedExecutionQuoteQty string `json:"pY"`
		WorkingTime                int64  `json:"W"` // Appears when the order is working on the book
		MatchType                  string `json:"b"`
		AllocationId               int64  `json:"a"`
		WorkingFloor               string `json:"k"`  // Appears for orders that could potentially have allocations
		UsedSor                    bool   `json:"uS"` // Appears for orders that used SOR
	}

	WsOCOUpdate struct {
		Symbol          string `json:"s"`
		OrderListId     int64  `json:"g"`
		ContingencyType string `json:"c"`
		ListStatusType  string `json:"l"`
		ListOrderStatus string `json:"L"`
		RejectReason    string `json:"r"`
		ClientOrderId   string `json:"C"` // List Client Order ID
		TransactionTime int64  `json:"T"`
		WsOCOOrderList
	}

	WsOCOOrderList struct {
		WsOCOOrders []WsOCOOrder `json:"O"`
	}

	WsOCOOrder struct {
		Symbol        string `json:"s"`
		OrderId       int64  `json:"i"`
		ClientOrderId string `json:"c"`
	}

	KlineResponse struct {
		OpenTime                 int64  `json:"openTime"`
		Open                     string `json:"open"`
		High                     string `json:"high"`
		Low                      string `json:"low"`
		Close                    string `json:"close"`
		Volume                   string `json:"volume"`
		CloseTime                int64  `json:"closeTime"`
		QuoteAssetVolume         string `json:"quoteAssetVolume"`
		TradeNum                 int64  `json:"tradeNum"`
		TakerBuyBaseAssetVolume  string `json:"takerBuyBaseAssetVolume"`
		TakerBuyQuoteAssetVolume string `json:"takerBuyQuoteAssetVolume"`
	}

	KlinePayload struct {
		Symbol    string
		Interval  string
		StartTime int64
		EndTime   int64
		Limit     int
	}

	AggTradesPayload struct {
		Symbol    string
		FromId    int64
		StartTime int64
		EndTime   int64
		Limit     int
	}

	AggTradesResponse struct {
		AggTradeID       int64  `json:"a"`
		Price            string `json:"p"`
		Quantity         string `json:"q"`
		FirstTradeID     int64  `json:"f"`
		LastTradeID      int64  `json:"l"`
		Timestamp        int64  `json:"T"`
		IsBuyerMaker     bool   `json:"m"`
		IsBestPriceMatch bool   `json:"M"`
	}

	APIKeyPermission struct {
		IPRestrict                     bool   `json:"ipRestrict"`
		CreateTime                     uint64 `json:"createTime"`
		EnableWithdrawals              bool   `json:"enableWithdrawals"`
		EnableInternalTransfer         bool   `json:"enableInternalTransfer"`
		PermitsUniversalTransfer       bool   `json:"permitsUniversalTransfer"`
		EnableVanillaOptions           bool   `json:"enableVanillaOptions"`
		EnableReading                  bool   `json:"enableReading"`
		EnableFutures                  bool   `json:"enableFutures"`
		EnableMargin                   bool   `json:"enableMargin"`
		EnableSpotAndMarginTrading     bool   `json:"enableSpotAndMarginTrading"`
		TradingAuthorityExpirationTime uint64 `json:"tradingAuthorityExpirationTime"`
	}

	// Actions All exchange actions types
	Actions struct {
		Buy              func(symbol string, quantity float64, price string) (CreateOrderResponse, *common.APIError)
		Sell             func(symbol string, quantity float64, price string) (CreateOrderResponse, *common.APIError)
		MarketBuy        func(symbol string, quantity float64) (CreateOrderResponse, *common.APIError)
		MarketSell       func(symbol string, quantity float64) (CreateOrderResponse, *common.APIError)
		GetOrder         func(orderId int64, symbol string) (Order, *common.APIError)
		CancelOrder      func(orderId int64, symbol string) (CancelOrderResponse, *common.APIError)
		GetTrades        func(orderId int64, symbol string) ([]Trade, *common.APIError)
		GetExchangeInfo  func(symbol string) (ExchangeInfo, *common.APIError)
		GetFees          func(symbol string) (TradeFeeDetails, *common.APIError)
		GetPrice         func(symbol string) (float64, *common.APIError)
		GetProfile       func() (Account, *common.APIError)
		GetUserAssets    func() ([]UserAssetRecord, *common.APIError)
		PriceWSHandler   func(pairs []string, handler func(PriceWSResponseData), done <-chan string)
		UserWSHandler    func(listenKey string, handler func(order WsUserDataEvent, expireEvent string), done <-chan string)
		PingUserStream   func(listenKey string) *common.APIError
		StartUserStream  func() (string, *common.APIError)
		AggTrades        func(payload AggTradesPayload) ([]AggTradesResponse, *common.APIError)
		KlineData        func(KlinePayload) ([]KlineResponse, *common.APIError)
		APIKeyPermission func() (APIKeyPermission, *common.APIError)
	}
)
