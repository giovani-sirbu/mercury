# Security Rules

## Secrets, Credentials & Sensitive Data

### CRITICAL: Never commit secrets, credentials, or `.env` files
This includes: API keys, exchange secrets, JWT secrets, database credentials, Kafka certs, Redis passwords.

Use `.gitignore`. If accidentally committed, rotate the secret immediately.

### CRITICAL: Never hardcode credentials anywhere
Not in source code, not in config files, not in Docker configs. All credentials come from environment variables via `godotenv`.

```go
// VIOLATION
brokerUrl := "kafka://prod-broker:9092"

// CORRECT
brokerUrl := cfg.Common.BrokerUrl  // loaded from .env
```

### CRITICAL: Never log, display, or expose exchange API keys/secrets
Exchange credentials are the user's money. Never include in:
- Log messages
- Error responses
- API responses
- Debug output

```go
// VIOLATION
log.Error(fmt.Sprintf("Exchange error for key: %s", exchange.ApiKey), ...)

// CORRECT
log.Error("Exchange API error", "CheckBalance", "CreateTrade")
```

---

## Input Validation

### CRITICAL: Validate all request payloads at the handler boundary
Every HTTP handler validates with `helpers.ValidatePayload(payload)` before processing.

```go
err := helpers.ValidatePayload(payload)
if err != nil {
    adapter.ValidationResponse(c, err)
    return
}
```

### CRITICAL: Use struct tags for binding validation
```go
type CreateTradePayload struct {
    Symbol     string `json:"symbol" binding:"required" validate:"required"`
    ExchangeId int    `json:"exchangeId" binding:"required" validate:"required"`
}
```

### IMPORTANT: Sanitize external input
Never use user input directly in:
- Raw SQL queries (use GORM parameterized queries)
- Log format strings (use structured logging)
- Cache key generation without validation

---

## Auth & Authorization

### CRITICAL: Auth middleware on every protected route
`adapter.IsAuth` on all user routes. `adapter.IsAdmin` on all admin routes.

```go
r.POST("/", adapter.IsAuth, handler.CreateTradeReq)
adminRoutes := r.Group("/admin", adapter.IsAdmin)
```

### CRITICAL: Tenant isolation in every read/write
Every DB query on user-facing routes must scope to the authenticated user's ID. A user must never access another user's trades, exchanges, or settings.

```go
// ALWAYS
h.server.DB.Where("user_id = ?", user.Id).Find(&trades)
```

### IMPORTANT: Session extraction pattern
Always use `helpers.GetUserSession(c)` to get the authenticated user. Never trust client-sent user IDs for ownership checks.

---

## Exchange Security

### CRITICAL: Exchange secrets encrypted at rest
API secrets stored in the database are encrypted via mercury's `crypto` package. Decrypt only when making exchange API calls.

### IMPORTANT: Validate exchange permissions before trade creation
Before creating a spot trade, call the exchange API to verify:
1. API key is valid
2. Trading permissions are enabled
3. IP restrictions are satisfied

---

## Infrastructure

### IMPORTANT: All config via environment variables
No hardcoded URLs, ports, credentials, or connection strings. Everything via `godotenv` + `config/` package.

### STANDARD: Redis and Kafka connections from config
```go
Cache: memory.Memory{
    Address:  []string{cfg.Common.RedisUrl},
    User:     cfg.Common.RedisUser,
    Password: cfg.Common.RedisPass,
}
```

---

## Audit Checklist (for PRs)

- [ ] No secrets in code, logs, configs, or responses
- [ ] No hardcoded credentials or connection strings
- [ ] `.env` files in `.gitignore`
- [ ] Inputs validated with `ValidatePayload`
- [ ] Auth middleware on every protected route
- [ ] DB queries scoped to `user_id`
- [ ] Exchange secrets never logged or returned
- [ ] GORM parameterized queries (no raw SQL string concat)
- [ ] Error messages don't leak internal details
