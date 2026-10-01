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
// gate on a trade: the strategy flag that ran (Param), which of its gates
// (Gate) and the values that gate reads back on a later tick (Data). The
// gates keep their per-trade state in these events and nowhere else, so a
// fold reads typed fields and never the text of a log row.
//
// The log row a gate writes beside an event (TradesLogs) is the human-readable
// output — what cp shows and the notifications filter — and carries no state.
// An event exists exactly when its row does, and the two share one CreatedAt,
// so a pair is found by (TradeID, CreatedAt). A hold that only says no on the
// market's behalf (usePatterns, useAI) writes its row alone.
//
// Events are append-only: a fold takes them in the order the trade carries
// them (agora loads them by id, the backtest appends), and nothing rewrites
// or deletes one. They travel inside Trades, as Logs and History do: hermes
// has no database, so the trade reaches it as JSON with its events.
//
// Data is the gate's own JSON document. Its "event" key names the kind, and
// the rest is the typed struct the gate package defines for it. It is never
// nil (NewStrategyEvent stores an empty object for what it cannot marshal),
// and the jsonb column is never `not null`: the serializer writes a nil
// RawMessage as SQL NULL.
type TradesStrategyEvents struct {
	ID        uint            `gorm:"primaryKey" json:"id"`
	TradeID   uint            `gorm:"index" json:"tradeId"`
	Param     string          `gorm:"type:varchar(32)" json:"param"`
	Gate      string          `gorm:"type:varchar(32)" json:"gate"`
	Data      json.RawMessage `gorm:"type:jsonb;serializer:json" json:"data"`
	CreatedAt time.Time       `json:"createdAt"`
}
