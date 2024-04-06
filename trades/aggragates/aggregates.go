package aggragates

import (
	"gorm.io/gorm"
	"time"
)

type Status string

const (
	New     Status = "new"
	Active  Status = "active"
	Blocked Status = "blocked"
	Paused  Status = "paused"
	Closed  Status = "closed"
)

type (
	Position struct {
		Type  string  `bson:"type" json:"type"`
		Price float64 `bson:"price" json:"price"`
	}

	FeeDetails struct {
		ID         uint      `gorm:"primaryKey" form:"id" json:"id" xml:"id"`
		TradeID    uint      `form:"tradeId" json:"tradeId" xml:"tradeId"`
		HistoryID  uint      `form:"historyId" json:"historyId" xml:"historyId"`
		Asset      string    `bson:"asset" json:"asset"`
		Fee        float64   `bson:"fee" json:"fee"`
		FeeInQuote float64   `bson:"feeInQuote" json:"feeInQuote"`
		CreatedAt  time.Time `form:"createdAt" json:"createdAt" xml:"createdAt"`
		UpdatedAt  time.Time `form:"updatedAt" json:"updatedAt" xml:"updatedAt"`
	}

	History struct {
		ID         uint         `gorm:"primaryKey" form:"id" json:"id" xml:"id"`
		TradeID    uint         `form:"tradeId" json:"tradeId" xml:"tradeId"`
		Type       string       `bson:"type" json:"type"`
		Quantity   float64      `bson:"quantity" json:"quantity"`
		Price      float64      `bson:"price" json:"price"`
		FeeDetails []FeeDetails `bson:"feeDetails" json:"feeDetails"`
		OrderId    int64        `bson:"orderId" json:"orderId"`
		Status     string       `bson:"status" json:"status"`
		CreatedAt  time.Time    `json:"createdAt"`
		UpdatedAt  time.Time    `json:"updatedAt"`
	}

	Logs struct {
		ID         uint      `gorm:"primaryKey" form:"id" json:"id" xml:"id"`
		TradeID    uint      `form:"tradeId" json:"tradeId" xml:"tradeId"`
		Message    string    `bson:"message" json:"message"`
		Type       string    `bson:"type" json:"type"`
		Quantity   float64   `bson:"quantity" json:"quantity"`
		Price      float64   `bson:"price" json:"price"`
		Percentage float64   `bson:"percentage" json:"percentage"`
		CreatedAt  time.Time `json:"createdAt"`
		UpdatedAt  time.Time `json:"updatedAt"`
	}

	Exchange struct {
		Name      string `bson:"name" json:"name"`
		ApiKey    string `bson:"apiKey" json:"apiKey"`
		ApiSecret string `bson:"apiSecret" json:"apiSecret"`
		TestNet   bool   `bson:"testNet" json:"testNet"`
	}

	Strategy struct {
		Tolerance          float64 `bson:"tolerance" json:"tolerance"`
		TrailingTakeProfit float64 `bson:"trailingTakeProfit" json:"trailingTakeProfit"`
		InitialBid         float64 `bson:"initialBid" json:"initialBid"`
		Name               string  `bson:"name" json:"name"`
		Percentage         float64 `bson:"percentage" json:"percentage"`
		Multiplier         float64 `bson:"multiplier" json:"multiplier"`
	}

	Trade struct {
		ID                uint           `gorm:"primaryKey" form:"id" json:"id" xml:"id"`
		UserID            uint           `form:"userId" json:"userId" xml:"userId"`
		Symbol            string         `gorm:"type:varchar(10)" bson:"symbol" json:"symbol"`
		Status            Status         `gorm:"default:new" bson:"status" json:"status"`
		PositionType      string         `gorm:"type:varchar(50)" bson:"type" json:"type"`
		PositionPrice     float64        `bson:"price" json:"price"`
		ExchangeName      string         `gorm:"type:varchar(50)" bson:"exchangeName" json:"exchangeName"`
		ExchangeApiKey    string         `gorm:"type:varchar(200)" bson:"exchangeApiKey" json:"exchangeApiKey"`
		ExchangeApiSecret string         `gorm:"type:varchar(200)" bson:"exchangeApiSecret" json:"exchangeApiSecret"`
		ExchangeTestNet   bool           `gorm:"type:boolean;default:false" bson:"exchangeTestNet" json:"exchangeTestNet"`
		StrategySettings  string         `gorm:"type:text" bson:"strategySettings" json:"strategySettings"`
		USDProfit         float64        `bson:"usdProfit" json:"usdProfit"`
		Profit            float64        `bson:"profit" json:"profit"`
		ProfitAsset       string         `bson:"profitAsset" json:"profitAsset"`
		PreventNewTrade   bool           `gorm:"type:boolean;default:false" bson:"preventNewTrade" json:"preventNewTrade"`
		Inverse           bool           `gorm:"type:boolean;default:false" bson:"inverse" json:"inverse"`
		PendingOrder      int64          `bson:"pendingOrder" json:"pendingOrder"`
		History           []History      `bson:"history" json:"history"`
		Logs              []Logs         `bson:"logs" json:"logs"`
		CreatedAt         time.Time      `form:"createdAt" json:"createdAt" xml:"createdAt"`
		UpdatedAt         time.Time      `form:"updatedAt" json:"updatedAt" xml:"updatedAt"`
		DeletedAt         gorm.DeletedAt `form:"deletedAt" json:"deletedAt" xml:"deletedAt"`
	}

	TradeSettings struct {
		LotSize   int
		MinNotion float64
	}

	TradeResponse struct {
		ID              string    `bson:"_id" json:"_id"`
		UserId          string    `bson:"userId" json:"userId"`
		DeviceId        string    `bson:"deviceId" json:"deviceId"`
		Symbol          string    `bson:"symbol" json:"symbol"`
		Position        Position  `bson:"position" json:"position"`
		Strategy        Strategy  `bson:"strategy" json:"strategy"`
		Exchange        Exchange  `bson:"exchange" json:"exchange"`
		USDProfit       float64   `bson:"usdProfit" json:"usdProfit"`
		Profit          float64   `bson:"profit" json:"profit"`
		ProfitAsset     string    `bson:"profitAsset" json:"profitAsset"`
		Commission      float64   `bson:"commission" json:"commission"`
		Price           float64   `bson:"price" json:"price"`
		CommissionAsset string    `bson:"commissionAsset" json:"commissionAsset"`
		Closed          bool      `bson:"closed" json:"closed"`
		Paused          bool      `bson:"paused" json:"paused"`
		Inverse         bool      `bson:"inverse" json:"inverse"`
		History         []History `bson:"history" json:"history"`
		Logs            []Logs    `bson:"logs" json:"logs"`
		PendingOrder    int64     `bson:"pendingOrder" json:"pendingOrder"`
		PreventNewTrade bool      `bson:"preventNewTrade" json:"preventNewTrade"`
		CreatedAt       time.Time `bson:"createdAt" json:"createdAt"`
		UpdatedAt       time.Time `bson:"updatedAt" json:"updatedAt"`
	}
)
