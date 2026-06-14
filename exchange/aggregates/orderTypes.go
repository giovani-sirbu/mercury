package aggregates

// CreateOrderResponse is the response returned by the exchange after placing
// an order (spot or futures).
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
	IsIsolated               bool   `json:"isIsolated"` // for isolated margin

	TimeInForce string `json:"timeInForce"`
	Type        string `json:"type"`
	Side        string `json:"side"`
}

// Order represents an existing spot order fetched from the exchange.
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

// FuturesOrder represents an existing futures order fetched from the exchange.
type FuturesOrder struct {
	Symbol                  string `json:"symbol"`
	OrderID                 int64  `json:"orderId"`
	ClientOrderID           string `json:"clientOrderId"`
	Price                   string `json:"price"`
	ReduceOnly              bool   `json:"reduceOnly"`
	OrigQuantity            string `json:"origQty"`
	ExecutedQuantity        string `json:"executedQty"`
	CumQuantity             string `json:"cumQty"`
	CumQuote                string `json:"cumQuote"`
	Status                  string `json:"status"`
	TimeInForce             string `json:"timeInForce"`
	Type                    string `json:"type"`
	Side                    string `json:"side"`
	StopPrice               string `json:"stopPrice"`
	Time                    int64  `json:"time"`
	UpdateTime              int64  `json:"updateTime"`
	WorkingType             string `json:"workingType"`
	ActivatePrice           string `json:"activatePrice"`
	PriceRate               string `json:"priceRate"`
	AvgPrice                string `json:"avgPrice"`
	OrigType                string `json:"origType"`
	PositionSide            string `json:"positionSide"`
	PriceProtect            bool   `json:"priceProtect"`
	ClosePosition           bool   `json:"closePosition"`
	PriceMatch              string `json:"priceMatch"`
	SelfTradePreventionMode string `json:"selfTradePreventionMode"`
	GoodTillDate            int64  `json:"goodTillDate"`
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

// CancelFuturesOrderResponse is returned when a futures order is cancelled.
type CancelFuturesOrderResponse struct {
	ClientOrderID    string `json:"clientOrderId"`
	CumQuantity      string `json:"cumQty"`
	CumQuote         string `json:"cumQuote"`
	ExecutedQuantity string `json:"executedQty"`
	OrderID          int64  `json:"orderId"`
	OrigQuantity     string `json:"origQty"`
	Price            string `json:"price"`
	ReduceOnly       bool   `json:"reduceOnly"`
	Side             string `json:"side"`
	Status           string `json:"status"`
	StopPrice        string `json:"stopPrice"`
	Symbol           string `json:"symbol"`
	TimeInForce      string `json:"timeInForce"`
	Type             string `json:"type"`
	UpdateTime       int64  `json:"updateTime"`
	WorkingType      string `json:"workingType"`
	ActivatePrice    string `json:"activatePrice"`
	PriceRate        string `json:"priceRate"`
	OrigType         string `json:"origType"`
	PositionSide     string `json:"positionSide"`
	PriceProtect     bool   `json:"priceProtect"`
}

// ModifyFuturesOrderResponse represents the response from the Binance modify
// order endpoint.
type ModifyFuturesOrderResponse struct {
	OrderID       int64  `json:"orderId"`
	Symbol        string `json:"symbol"`
	Status        string `json:"status"`
	ClientOrderID string `json:"clientOrderId"`
	Price         string `json:"price"`
	OrigQty       string `json:"origQty"`
	ExecutedQty   string `json:"executedQty"`
	CumQuote      string `json:"cumQuote"`
	TimeInForce   string `json:"timeInForce"`
	Type          string `json:"type"`
	Side          string `json:"side"`
	UpdateTime    int64  `json:"updateTime"`
}

// Trade is a single executed trade (one side of an order fill).
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
