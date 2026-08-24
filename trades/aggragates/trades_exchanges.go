package aggragates

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"gorm.io/gorm"
)

type Assets struct{ aggregates.UserAssetRecord }

// TradesExchanges is a user's exchange credential row. ApiSecret (encrypted)
// is never serialized to JSON: user responses, SSE frames and broker payloads
// must not carry it. The only JSON path that needs it is agora's system-role
// trade wrapper for hermes, which re-emits it explicitly (SystemTradeExchange).
// msgpack (Redis cache) ignores json tags, so cached trades keep it.
type TradesExchanges struct {
	ID           uint                         `gorm:"primaryKey" form:"id" json:"id" xml:"id"`
	UserID       uint                         `gorm:"index:idx_user_id_created_at,priority:1" form:"userId" json:"-" xml:"userId"`
	Name         string                       `gorm:"type:varchar(50)" bson:"name" json:"name" form:"name" xml:"name" validate:"required,min=3,max=50"`
	Label        string                       `gorm:"type:varchar(50)" bson:"label" json:"label" form:"label" xml:"label" validate:"required,min=3,max=50"`
	ApiKey       string                       `gorm:"type:varchar(200)" bson:"apiKey" json:"apiKey" form:"apiKey" xml:"apiKey" validate:"required,min=10,max=150"`
	ApiSecret    string                       `gorm:"type:varchar(200)" bson:"apiSecret" json:"-" form:"apiSecret" xml:"-" validate:"required,min=10,max=150"`
	TestNet      bool                         `gorm:"type:boolean;default:false" bson:"testNet" json:"testNet" form:"testNet" xml:"testNet"`
	TradesStatus Status                       `gorm:"type:varchar(50);default:new" bson:"tradesStatus" json:"tradesStatus" form:"tradesStatus" xml:"tradesStatus"`
	Status       Status                       `gorm:"type:varchar(50);default:new;index:idx_status_created_at,priority:1" bson:"status" json:"status" form:"status" xml:"status"`
	TradeType    TradeTypes                   `gorm:"type:varchar(50);default:spot" bson:"tradeType" json:"tradeType" form:"tradeType" xml:"tradeType"`
	Balance      float64                      `form:"balance" json:"balance" xml:"balance"`
	Assets       []aggregates.UserAssetRecord `gorm:"type:jsonb;serializer:json;" bson:"assets" form:"assets" json:"assets" xml:"assets"`
	CreatedAt    time.Time                    `gorm:"index:idx_status_created_at,priority:2" form:"createdAt" json:"createdAt" xml:"createdAt"`
	UpdatedAt    time.Time                    `gorm:"index:idx_user_id_created_at,priority:2" form:"updatedAt" json:"-" xml:"updatedAt"`
	DeletedAt    gorm.DeletedAt               `form:"deletedAt" json:"-" xml:"deletedAt"`
}

const apiKeyMaskVisibleChars = 4

// MaskApiKey keeps the first and last four characters of the key and hides
// the middle with four asterisks. Keys too short to expose eight characters
// without revealing most of the secret are fully masked; an empty key stays
// empty so "no key" keeps reading as "no key".
func MaskApiKey(apiKey string) string {
	trimmed := strings.TrimSpace(apiKey)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= apiKeyMaskVisibleChars*2 {
		return "****"
	}
	return trimmed[:apiKeyMaskVisibleChars] + "****" + trimmed[len(trimmed)-apiKeyMaskVisibleChars:]
}

// Sanitized returns a copy of the exchange that is safe to send to an end
// user over any channel (REST response, SSE frame, push payload): the
// encrypted secret is dropped and the API key is masked to `first4****last4`.
// Shared here because every service that serializes an exchange for a user
// must apply the same rule; only agora's system-role path keeps the raw row.
func (exchange TradesExchanges) Sanitized() TradesExchanges {
	exchange.ApiSecret = ""
	exchange.ApiKey = MaskApiKey(exchange.ApiKey)
	return exchange
}

// Value Marshal
func (a Assets) Value() (driver.Value, error) {
	return json.Marshal(a)
}

// Scan Unmarshal
func (a *Assets) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, &a)
}
