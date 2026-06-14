package aggregates

// WsUserDataEvent is the unified Go shape for every event delivered on the
// Binance user-data stream (order updates, balance updates, account updates,
// OCO updates).
//
// Binance reuses single-letter JSON tags (e.g. "T", "s", "d") across the
// sub-events that share the user-data stream. Flattening every sub-struct
// via embedding produces ambiguous tags that vet flags and that silently
// drop fields when (un)marshalled. The struct is never (un)marshalled in
// our codebase — wire types live in go-binance and fields are transferred
// via jinzhu/copier — but to keep vet happy and make ambiguity impossible,
// only WsOrderUpdate (the variant actually consumed downstream) remains
// promoted. The other variants live as named fields accessed through their
// own namespace.
type WsUserDataEvent struct {
	Event               string              `json:"e"`
	Time                int64               `json:"E"`
	WsOrderUpdate                           // promoted: callers use event.Status, event.Id, ...
	WsAccountUpdateList WsAccountUpdateList `json:"accountUpdate"`
	WsBalanceUpdate     WsBalanceUpdate     `json:"balanceUpdate"`
	WsOCOUpdate         WsOCOUpdate         `json:"ocoUpdate"`
}

// WsAccountUpdateList is the "outboundAccountPosition" event payload.
type WsAccountUpdateList struct {
	AccountUpdateTime int64             `json:"u"`
	WsAccountUpdates  []WsAccountUpdate `json:"B"`
}

// WsAccountUpdate is a single balance line inside an account update.
type WsAccountUpdate struct {
	Asset  string `json:"a"`
	Free   string `json:"f"`
	Locked string `json:"l"`
}

// WsBalanceUpdate is the "balanceUpdate" event payload (deposits / withdrawals).
type WsBalanceUpdate struct {
	Asset           string `json:"a"`
	Change          string `json:"d"`
	TransactionTime int64  `json:"T"`
}

// WsOrderUpdate is the "executionReport" event payload — the variant consumed
// by the agora user-websocket handler.
type WsOrderUpdate struct {
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

	// Optional fields that appear in the payload only if certain conditions are met.
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

// WsOCOUpdate is the "listStatus" event payload for OCO / OTO orders.
type WsOCOUpdate struct {
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

// WsOCOOrderList holds the list of orders attached to an OCO event.
type WsOCOOrderList struct {
	WsOCOOrders []WsOCOOrder `json:"O"`
}

// WsOCOOrder is a single leg of an OCO order list.
type WsOCOOrder struct {
	Symbol        string `json:"s"`
	OrderId       int64  `json:"i"`
	ClientOrderId string `json:"c"`
}
