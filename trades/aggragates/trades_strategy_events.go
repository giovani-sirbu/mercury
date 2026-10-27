package aggragates

import (
	"encoding/json"
	"time"
)

// The strategy params that write strategy events: the flag whose gate ran.
const (
	StrategyParamCooldown      = "cooldown"
	StrategyParamSmartTakeLoss = "smartTakeLoss"
	StrategyParamDynamicParams = "dynamicParams"
)

// TradesStrategyEvents is the typed record of one execution of a strategy
// gate on a trade: the flag that ran (Param), which of its gates (Gate) and
// the values the gate reads back on a later tick (Data, a JSON document whose
// "event" key names the kind). The gates keep their per-trade state here and
// nowhere else, never in the text of the log row written beside the event,
// which shares its CreatedAt. Events are append-only, folded in the order the
// trade carries them, and travel inside Trades as Logs and History do. Data is
// never nil, and the jsonb column is never `not null`.
type TradesStrategyEvents struct {
	ID        uint            `gorm:"primaryKey" json:"id"`
	TradeID   uint            `gorm:"index" json:"tradeId"`
	Param     string          `gorm:"type:varchar(32)" json:"param"`
	Gate      string          `gorm:"type:varchar(32)" json:"gate"`
	Data      json.RawMessage `gorm:"type:jsonb;serializer:json" json:"data"`
	CreatedAt time.Time       `json:"createdAt"`
}
