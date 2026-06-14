package aggregates

// RateLimit describes a single exchange rate-limit bucket.
type RateLimit struct {
	RateLimitType string `json:"rateLimitType"`
	Interval      string `json:"interval"`
	IntervalNum   int64  `json:"intervalNum"`
	Limit         int64  `json:"limit"`
}

// Symbol is a market symbol tradable on the exchange.
type Symbol struct {
	Symbol              string                   `json:"symbol"`
	BaseAsset           string                   `json:"baseAsset"`
	BaseAssetPrecision  int                      `json:"baseAssetPrecision"`
	QuoteAsset          string                   `json:"quoteAsset"`
	QuotePrecision      int                      `json:"quotePrecision"`
	QuoteAssetPrecision int                      `json:"quoteAssetPrecision"`
	Filters             []map[string]interface{} `json:"filters"`
}

// ExchangeInfo is the aggregated metadata returned by /exchangeInfo.
type ExchangeInfo struct {
	Timezone        string        `json:"timezone"`
	ServerTime      int64         `json:"serverTime"`
	RateLimits      []RateLimit   `json:"rateLimits"`
	ExchangeFilters []interface{} `json:"exchangeFilters"`
	Symbols         []Symbol      `json:"symbols"`
}

// TradeFeeDetails is the maker/taker commission pair for a single symbol.
type TradeFeeDetails struct {
	Symbol          string `json:"symbol"`
	MakerCommission string `json:"makerCommission"`
	TakerCommission string `json:"takerCommission"`
}

// KlineResponse is a single candlestick bar.
type KlineResponse struct {
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

// KlinePayload is the request shape for KlineData.
type KlinePayload struct {
	Symbol    string
	Interval  string
	StartTime int64
	EndTime   int64
	Limit     int
}

// AggTradesPayload is the request shape for AggTrades.
type AggTradesPayload struct {
	Symbol    string
	FromId    int64
	StartTime int64
	EndTime   int64
	Limit     int
}

// AggTradesResponse is a single aggregated trade entry.
type AggTradesResponse struct {
	AggTradeID       int64  `json:"a"`
	Price            string `json:"p"`
	Quantity         string `json:"q"`
	FirstTradeID     int64  `json:"f"`
	LastTradeID      int64  `json:"l"`
	Timestamp        int64  `json:"T"`
	IsBuyerMaker     bool   `json:"m"`
	IsBestPriceMatch bool   `json:"M"`
}

// PriceWSResponseData is the per-tick payload delivered by the aggTrade WS
// stream; `exchange` is filled in by mercury before dispatch.
type PriceWSResponseData struct {
	Price    string `json:"p"`
	Symbol   string `json:"s"`
	Exchange string `json:"exchange"`
}
