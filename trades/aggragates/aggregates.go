package aggragates

import (
	"github.com/giovani-sirbu/mercury/strategies"
	"time"
)

type (
	Position struct {
		Type  string  `bson:"type" json:"type"`
		Price float64 `bson:"price" json:"price"`
	}
	FeeDetails struct {
		Asset      string  `bson:"asset" json:"asset"`
		Fee        float64 `bson:"fee" json:"fee"`
		FeeInQuote float64 `bson:"feeInQuote" json:"feeInQuote"`
	}
	History struct {
		ID         string       `bson:"_id" json:"_id"`
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
		ID         string    `bson:"_id" json:"_id"`
		Message    string    `bson:"message" json:"message"`
		Type       string    `bson:"type" json:"type"`
		Quantity   float64   `bson:"quantity" json:"quantity"`
		Price      float64   `bson:"price" json:"price"`
		Percentage float64   `bson:"percentage" json:"percentage"`
		CreatedAt  time.Time `json:"createdAt"`
		UpdatedAt  time.Time `json:"updatedAt"`
	}

	Roulette struct {
		Name               string
		BidValue           uint16  `bson:"bidValue" json:"bidValue"`
		Percentage         float64 `bson:"percentage" json:"percentage"`
		Tolerance          float64 `bson:"tolerance" json:"tolerance"`
		TrailingTakeProfit float64 `bson:"trailingTakeProfit" json:"trailingTakeProfit"`
	}

	Exchange struct {
		Name      string `bson:"name" json:"name"`
		ApiKey    string `bson:"apiKey" json:"apiKey"`
		ApiSecret string `bson:"apiSecret" json:"apiSecret"`
		TestNet   bool   `bson:"testNet" json:"testNet"`
	}

	Strategy struct {
		Tolerance          float64   `bson:"tolerance" json:"tolerance"`
		TrailingTakeProfit float64   `bson:"trailingTakeProfit" json:"trailingTakeProfit"`
		InitialBid         float64   `bson:"initialBid" json:"initialBid"`
		Name               string    `bson:"name" json:"name"`
		Percentage         float64   `bson:"percentage" json:"percentage"`
		Progressive        []float64 `bson:"progressive" json:"progressive"`
	}

	Trade struct {
		ID              string              `bson:"_id" json:"_id"`
		UserId          string              `bson:"userId" json:"userId"`
		DeviceId        string              `bson:"deviceId" json:"deviceId"`
		Symbol          string              `bson:"symbol" json:"symbol"`
		Position        Position            `bson:"position" json:"position"`
		Strategy        interface{}         `bson:"strategy" json:"strategy"`
		Settings        strategies.Settings `bson:"settings" json:"settings"`
		Exchange        Exchange            `bson:"exchange" json:"exchange"`
		USDProfit       float64             `bson:"usdProfit" json:"usdProfit"`
		Profit          float64             `bson:"profit" json:"profit"`
		ProfitAsset     string              `bson:"profitAsset" json:"profitAsset"`
		Commission      float64             `bson:"commission" json:"commission"`
		CommissionAsset string              `bson:"commissionAsset" json:"commissionAsset"`
		Closed          bool                `bson:"closed" json:"closed"`
		Paused          bool                `bson:"paused" json:"paused"`
		PreventNewTrade bool                `bson:"preventNewTrade" json:"preventNewTrade"`
		Inverse         bool                `bson:"inverse" json:"inverse"`
		History         []History           `bson:"history" json:"history"`
		Logs            []Logs              `bson:"logs" json:"logs"`
		PendingOrder    int64               `bson:"pendingOrder" json:"pendingOrder"`
		CreatedAt       time.Time           `bson:"createdAt" json:"createdAt"`
		UpdatedAt       time.Time           `bson:"updatedAt" json:"updatedAt"`
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
