---
paths:
  - "consumers/**/*"
  - "helpers/externalRequest.go"
---

# Service Communication Rules

## Kafka Topics

### Current Topic Map

| Topic | Producer | Consumer | Purpose |
|---|---|---|---|
| `update-trade` | hermes | agora | Trade state update after buy/sell |
| `create-children-trades` | hermes | agora | Create child impasse trade |
| `delete-user-associated-data` | hellenes | agora | Clean up when user deleted |
| `update-user-data` | agora | hellenes | Sync user data changes |
| `send-emails` | hellenes | iris | Send transactional emails |

### Rules

### CRITICAL: Kafka consumers run as goroutines
Every consumer is started with `go server.Broker.Consumer(topic, handler)` in `consumers/consumers.go`.

### CRITICAL: Consumer handlers must be idempotent
Messages may be delivered more than once. Consumer handlers must handle duplicate messages safely (check if action already performed, use upserts where possible).

### IMPORTANT: One consumer handler per file
Follow the same one-action-per-file rule as HTTP handlers. `updateTrade.go`, `createChildrenTrades.go`, `deleteUserAssociatedData.go`.

### IMPORTANT: Consumer handlers follow the handler struct pattern
```go
// consumers/aggregates.go
type Handler struct {
    server *s.Server
}

func NewTradesHandler(server *s.Server) *Handler {
    return &Handler{server: server}
}

// consumers/updateTrade.go
func (h *Handler) UpdateTrade(msg []byte) { ... }
```

---

## HTTP Cross-Service Calls

### Pattern
Services call each other via HTTP using `helpers/externalRequest.go`. Service URLs come from config (environment variables).

### IMPORTANT: Cross-service URLs from config, never hardcoded
```go
// CORRECT
url := fmt.Sprintf("%s/active-trades-ids", cfg.Common.AgoraUrl)

// VIOLATION
url := "http://agora:8080/active-trades-ids"
```

### IMPORTANT: Handle cross-service failures gracefully
If a cross-service HTTP call fails:
1. Log the error with context
2. Return an appropriate error to the caller
3. Never silently ignore a failed cross-service call

---

## Kafka Producer Usage

### Pattern
```go
server.Broker.Produce("topic-name", key, value, server.Broker.Producer)
```

### IMPORTANT: Serialize messages as JSON
```go
data, _ := json.Marshal(payload)
server.Broker.Produce("update-trade", []byte(tradeId), data, server.Broker.Producer)
```

---

## Adding a New Topic

When adding a new Kafka topic:
1. Add producer call in the producing service
2. Add consumer registration in `consumers/consumers.go` of the consuming service
3. Add consumer handler in `handlers/consumers/` with its own file
4. Update `.claude/architecture.md` with the new topic
5. Update this file's topic map
