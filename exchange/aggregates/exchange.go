// Package aggregates contains the exchange-facing data types used across
// mercury. The shapes mirror the Binance REST and WebSocket responses with
// service-friendly Go naming; helpers in the `exchange` package convert to
// and from these types via jinzhu/copier.
//
// Types are split across several files by concern:
//   - exchange.go          — the core Exchange config struct (this file)
//   - orderTypes.go        — spot + futures order request/response types
//   - marketDataTypes.go   — symbols, klines, trades, ExchangeInfo
//   - accountTypes.go      — balance, profile, API-key permissions
//   - futuresTypes.go      — futures-specific balance, position, income
//   - userStreamTypes.go   — user-data WebSocket event + sub-events
//   - actions.go           — Actions / FuturesActions function-bag structs
package aggregates

// Exchange represents the configuration needed to talk to a single exchange
// account. ApiKey / ApiSecret are stored encrypted at rest; callers should
// decrypt via mercury/crypto immediately before use and never log the raw
// values.
type Exchange struct {
	ID        uint   `gorm:"primaryKey" form:"id" json:"id" xml:"id"`
	Label     string `gorm:"type:varchar(50)" bson:"label" json:"label" form:"label" xml:"label" validate:"required,min=3,max=50"`
	Name      string `gorm:"type:varchar(50)" bson:"name" json:"name" form:"name" xml:"name" validate:"required,min=3,max=50"`
	ApiKey    string `gorm:"type:varchar(200)" bson:"apiKey" json:"apiKey" form:"apiKey" xml:"apiKey" validate:"omitempty,min=10,max=150"`
	ApiSecret string `gorm:"type:varchar(200)" bson:"apiSecret" json:"apiSecret" form:"apiSecret" xml:"apiSecret" validate:"omitempty,min=10,max=150"`
	TestNet   bool   `gorm:"type:boolean;default:false" bson:"testNet" json:"testNet" form:"testNet" xml:"testNet"`
}
