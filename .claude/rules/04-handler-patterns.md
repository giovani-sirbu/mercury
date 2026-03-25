---
paths:
  - "handlers/**/*"
  - "routes/*"
  - "consumers/*"
  - "jobs/*"
---

# Handler Patterns

## The Handler Struct

Every handler domain has an `aggregates.go` with:
1. Handler struct (holds `*server.Server` + domain-specific flags)
2. Constructor (`NewXHandler(server) *Handler`)
3. All payload and response types for that domain

```go
// handlers/trades/aggregates.go
type Handler struct {
    server *s.Server
}

func NewTradesHandler(server *s.Server) *Handler {
    return &Handler{server: server}
}

type CreateTradePayload struct {
    Symbol     string `form:"symbol" json:"symbol" binding:"required" validate:"required"`
    ExchangeId int    `form:"exchangeId" json:"exchangeId" binding:"required" validate:"required"`
    StrategyId int    `form:"strategyId" json:"strategyId" binding:"required" validate:"required"`
}

type StopTradePayload struct {
    PreventNewTrade bool `form:"preventNewTrade" json:"preventNewTrade"`
}
```

---

## Rules

### CRITICAL: One action per file
`createTrade.go` contains `CreateTrade` (business) and `CreateTradeReq` (HTTP). `stopTrade.go` contains `StopTrade` and `StopTradeReq`. Never put multiple actions in one file.

### CRITICAL: aggregates.go is the only multi-declaration file
All payload types, response types, and the handler struct live here. No handler logic.

### CRITICAL: Payload validation at the boundary
Every HTTP handler validates its payload before doing anything:
```go
c.ShouldBindJSON(&payload)
err := helpers.ValidatePayload(payload)
if err != nil {
    adapter.ValidationResponse(c, err)
    return
}
```

### IMPORTANT: User session extraction pattern
Protected routes extract the user session immediately after validation:
```go
user, err := helpers.GetUserSession(c)
if err != nil {
    log.Error(err.Error(), "GetUserSession", "HandlerName")
    adapter.Response(c, http.StatusBadRequest, "FAILED_TO_FETCH_USER_SESSION")
    return
}
```

### IMPORTANT: Cache invalidation after mutations
After creating, updating, or deleting a record, clear relevant cache keys:
```go
cacheKey := fmt.Sprintf("exchange:%s:symbol:%s:active_trades_ids", exchange.Name, trade.Symbol)
handler.server.Cache.Delete(cacheKey)
```

---

## Route Registration

All routes registered in `routes/routes.go` via `routes.Init(server)`.

Pattern:
```go
func Init(server *s.Server) {
    r := server.Gin

    tradesHandler := trades.NewTradesHandler(server)
    r.POST("/", adapter.IsAuth, tradesHandler.CreateTradeReq)
    r.GET("/:tradeId", adapter.IsAuth, tradesHandler.GetTrade)

    adminRoutes := r.Group("/admin", adapter.IsAdmin)
    adminRoutes.GET("/trades", adminHandler.QueryTrades)
}
```

- `adapter.IsAuth` on all user routes
- `adapter.IsAdmin` on admin route groups
- Public routes (ping, app-version) have no middleware

---

## Consumer Pattern

Kafka consumers registered in `consumers/consumers.go`:
```go
func Init(server *s.Server) {
    handler := consumers.NewTradesHandler(server)
    go server.Broker.Consumer("topic-name", handler.MethodName)
}
```

Consumer handlers follow the same pattern as HTTP handlers but receive Kafka messages instead of `*gin.Context`.

---

## Job Pattern

Cron jobs registered in `jobs/init.go`, handler struct in `jobs/aggregates.go`, one job per file:
```go
// jobs/aggregates.go
type Handler struct { server *s.Server }
func NewJobsHandler(server *s.Server) *Handler { return &Handler{server: server} }

// jobs/handleBlockedTradesCronjob.go
func (h *Handler) HandleBlockedTradesCronjob() { ... }
```

---

## The Orchestrator Pattern

When a handler method gets long, decompose into named steps:

```go
func (h *Handler) CreateTrade(payload CreateTradePayload, exchange TradesExchanges, userId uint) (Trades, int, error) {
    // Step 1: Check trade doesn't already exist
    existing := h.findExistingTrade(payload, userId, exchange.ID)
    if existing.ID != 0 {
        return existing, http.StatusBadRequest, fmt.Errorf("TRADE_ALREADY_EXIST")
    }

    // Step 2: Build trade from payload
    trade := h.buildTrade(payload, exchange, userId)

    // Step 3: Validate exchange balance (spot only)
    if trade.Exchange.TradeType == aggragates.Spot {
        if err := h.CheckExchangeBalanceAndPermissions(trade); err != nil {
            return trade, http.StatusBadRequest, err
        }
    }

    // Step 4: Persist
    return h.saveTrade(trade)
}
```

Each sub-function is a named, testable unit. The orchestrator reads like a recipe.
