package aggragates

// BinanceOrderMapping carries the OrderID → TradeID linkage that hermes writes
// to Redis directly (bypassing the messagebus outbox) immediately after a
// successful Buy / Sell / CreateFuturesOrders action. Agora's user-data-stream
// handler reads it back when an executionReport arrives for an order whose
// PendingOrder field has not yet been persisted to agora's DB.
//
// This closes the "broker MESSAGEBUS down after Binance order placed" failure
// mode: the order is live on Binance, hermes cannot publish update-trade, but
// the mapping survives in Redis and lets agora reconcile via the user stream
// alone — without ever needing the broker for the critical handoff.
//
// Lifetime: written with a 24h TTL, which is far longer than a normal pending
// order survives but bounded enough that orphan mappings can't accumulate.
// agora is expected to delete the key on terminal status (FILLED / CANCELED /
// REJECTED / EXPIRED) but the TTL is the safety net.
type BinanceOrderMapping struct {
	TradeID uint   `json:"tradeId"`
	UserID  uint   `json:"userId"`
	Symbol  string `json:"symbol"`
}
