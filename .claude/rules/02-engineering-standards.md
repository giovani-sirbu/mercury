# Engineering Standards

These rules apply to all Go services. PRs that violate them will be rejected.

## Severity Levels

| Level | Meaning |
|---|---|
| **CRITICAL** | PR rejected. No exceptions. |
| **IMPORTANT** | Must follow. Exceptions require a PR comment explaining why. |
| **STANDARD** | Expected practice. Flagged in review but won't block if justified. |

---

## File Rules

### CRITICAL: No file exceeds 200 lines
Split by responsibility before it gets there. One handler file per action. One helper file per concern.

### CRITICAL: One primary export per file
File name matches the main export.
- CORRECT: `createTrade.go` contains `CreateTrade` and `CreateTradeReq`
- VIOLATION: `trades.go` contains `CreateTrade`, `StopTrade`, `PauseTrade`

Exception: `aggregates.go` may define the `Handler` struct, constructor, and all payload/response types.

### IMPORTANT: No generic file names
No `utils.go`, `helpers.go`, `common.go` with mixed content. Name files after what they contain: `getUserSession.go`, `validatePayload.go`, `paginate.go`.

---

## Function Rules

### CRITICAL: HTTP handlers do not contain business logic
HTTP handlers only: bind payload, validate, extract session, call business method, respond.

```go
// HTTP handler — thin wrapper
func (h *Handler) CreateTradeReq(c *gin.Context) {
    var payload CreateTradePayload
    c.ShouldBindJSON(&payload)
    err := helpers.ValidatePayload(payload)
    if err != nil { adapter.ValidationResponse(c, err); return }
    user, _ := helpers.GetUserSession(c)
    trade, status, err := h.CreateTrade(payload, exchange, user.Id)
    if err != nil { adapter.Response(c, status, err.Error()); return }
    adapter.Response(c, http.StatusCreated, trade)
}

// Business method — no HTTP concerns
func (h *Handler) CreateTrade(...) (Trade, int, error) { ... }
```

### IMPORTANT: Business methods return (result, statusCode, error)
Separates HTTP concern from business logic. Makes business logic reusable from consumers and jobs.

### IMPORTANT: Early returns for edge cases
Happy path stays unindented.

```go
func (h *Handler) GetTrade(id, userId uint) (Trade, error) {
    if id == 0 { return Trade{}, errors.New("INVALID_ID") }
    var trade Trade
    h.server.DB.Where("id = ? AND user_id = ?", id, userId).Find(&trade)
    if trade.ID == 0 { return Trade{}, errors.New("TRADE_NOT_FOUND") }
    return trade, nil
}
```

---

## Naming Rules

### CRITICAL: Full words only. No abbreviations.
| Correct | Wrong |
|---|---|
| `CreateTrade` | `CreateTrd` |
| `GetUserSession` | `GetSess` |
| `UpdateTradesExchangeStatus` | `UpdExchStatus` |

### CRITICAL: Function names = verb + noun
`CreateTrade()`, `GetActivePairs()`, `CheckExchangeBalance()`.

### CRITICAL: File names match primary function
`createTrade.go` -> `CreateTrade`. `getUserSession.go` -> `GetUserSession`.

### IMPORTANT: HTTP handler = business method + `Req` suffix
`CreateTrade()` = business logic. `CreateTradeReq()` = gin handler that calls it.

### STANDARD: Error code strings are SCREAMING_SNAKE_CASE
`"TRADE_NOT_FOUND"`, `"INSUFFICIENT_FUNDS"`, `"COULD_NOT_CREATE_TRADE"`.

---

## Go-Specific Rules

### IMPORTANT: Always scope DB queries by user_id on user-facing routes
```go
// BAD
h.server.DB.Where("id = ?", tradeId).Find(&trade)

// GOOD
h.server.DB.Where("id = ? AND user_id = ?", tradeId, userId).Find(&trade)
```

### IMPORTANT: Check GORM errors and empty results separately
```go
result := h.server.DB.Where(...).Find(&trade)
if result.Error != nil { /* database error */ }
if trade.ID == 0 { /* record not found */ }
```

### STANDARD: Minimal scoped changes only
Only modify what was explicitly asked for. Never refactor, rename, or restructure code outside the request scope.

---

## Source of Truth: Trust Code, Not Docs

When docs and reality disagree, trust reality. Read actual code, check live configs, test actual behavior.
