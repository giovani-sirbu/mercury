package aggragates

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

type (
	TradeFilters struct {
		LotSize     uint8   `bson:"lotSize" json:"lotSize" form:"lotSize" xml:"lotSize" validate:"required"`
		PriceFilter uint8   `bson:"priceFilter" json:"priceFilter" form:"priceFilter" xml:"priceFilter" validate:"required"`
		MinNotional float64 `bson:"minNotional" json:"minNotional" form:"minNotional" xml:"minNotional" validate:"required"`
	}
	StrategySettings struct {
		Tolerance          float64 `bson:"tolerance" json:"tolerance"`
		MinDepths          float64 `bson:"minDepths" json:"minDepths"`
		Depths             float64 `bson:"depths" json:"depths"`
		ImpasseDepth       float64 `bson:"impasseDepths" json:"impasseDepths"`
		TrailingTakeProfit float64 `bson:"trailingTakeProfit" json:"trailingTakeProfit"`
		InitialBid         float64 `bson:"initialBid" json:"initialBid"`
		Percentage         float64 `bson:"percentage" json:"percentage"`
		Multiplier         float64 `bson:"multiplier" json:"multiplier"`
	}
	ImpassePairs struct {
		Symbol  string
		Inverse bool
	}
	StrategiesPairs struct {
		ID               uint               `gorm:"primaryKey" form:"id" json:"-" xml:"id"`
		StrategyID       int                `gorm:"uniqueIndex:idx_symbol_strategy_id,priority:2;" form:"strategyId" json:"-" xml:"strategyId"`
		Symbol           string             `gorm:"type:varchar(20);uniqueIndex:idx_symbol_strategy_id,priority:1;" bson:"symbol" json:"-" form:"symbol" xml:"symbol" validate:"required,min=3,max=20"`
		Exchange         string             `gorm:"type:varchar(50);uniqueIndex:idx_symbol_strategy_id,priority:3;" bson:"exchange" json:"-" form:"exchange" xml:"exchange" validate:"required,min=3,max=50"`
		Rank             uint16             `bson:"rank" json:"rank" form:"rank" xml:"rank" validate:"required"`
		Status           string             `gorm:"default:active" bson:"status" json:"status"`
		TradeFilters     TradeFilters       `gorm:"type:jsonb;serializer:json;" bson:"tradeFilters" json:"tradeFilters" form:"tradeFilters" xml:"tradeFilters"`
		StrategySettings []StrategySettings `gorm:"type:jsonb;serializer:json;" bson:"strategySettings" json:"strategySettings" form:"strategySettings" xml:"strategySettings"`
		ImpassePairs     []ImpassePairs     `gorm:"type:jsonb;serializer:json;" bson:"impassePairs" json:"impassePairs" form:"impassePairs" xml:"impassePairs"`
		CreatedAt        time.Time          `form:"createdAt" json:"-" xml:"createdAt"`
		UpdatedAt        time.Time          `form:"updatedAt" json:"-" xml:"updatedAt"`
		DeletedAt        gorm.DeletedAt     `form:"deletedAt" json:"-" xml:"deletedAt"`
	}
)

// Value Marshal
func (a TradeFilters) Value() (driver.Value, error) {
	return json.Marshal(a)
}

// Scan Unmarshal
func (a *TradeFilters) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, &a)
}

// Value Marshal
func (a StrategySettings) Value() (driver.Value, error) {
	return json.Marshal(a)
}

// Scan Unmarshal
func (a *StrategySettings) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, &a)
}

// Value Marshal
func (a ImpassePairs) Value() (driver.Value, error) {
	return json.Marshal(a)
}

// Scan Unmarshal
func (a *ImpassePairs) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, &a)
}
