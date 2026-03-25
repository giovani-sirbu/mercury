# Architecture Rules

## Separate Repos Per Service

| Repo | Contains |
|---|---|
| **agora** | Trade management service |
| **hermes** | Core trading engine |
| **hellenes** | User service |
| **sisyphus** | Backtesting + testing |
| **iris** | Email delivery |
| **mercury** | Shared library |

No service logic goes into mercury. Mercury is infrastructure only: exchange adaptors, trade math, messaging, logging, auth middleware.

See `.claude/architecture.md` for current routes, topics, and infrastructure per service.

---

## Service Internal Architecture

Every service follows one pattern:

```
main.go → config.Init() → server.NewServer() → routes.Init() → consumers.Init() → jobs.Init() → app.Start()
```

### Server struct — the service container
```go
type Server struct {
    Gin    *gin.Engine
    DB     *gorm.DB
    Config *config.Config
    Broker messagebroker.BrokerMethods
    Cache  memory.Memory
}
```

Not every service uses all fields. Hermes has no DB. Iris has no DB. Sisyphus has no Broker.

### Handlers access everything through `handler.server`
No separate repository or service layer. Handlers query the DB directly via `handler.server.DB`. This is a deliberate choice — keep services small.

```go
type Handler struct {
    server *server.Server
}
```

---

## Service Folder Structure

### CRITICAL: Every service follows this exact layout
```
[service]/
  main.go
  config/
    config.go
    db.go
    http.go
    common.go
  server/
    server.go
  routes/
    routes.go
  handlers/
    [domain]/
      aggregates.go
      [verb][Noun].go
  consumers/
    consumers.go
  helpers/
    [specific-name].go
  jobs/
    aggregates.go
    [job].go
  db/
    connection.go
  models/
    [domain].go
```

No variations. Predictability is the point.

---

## Cross-Service Communication

### CRITICAL: Async communication via Kafka topics
Hermes produces trade updates. Agora consumes them. Hellenes produces user events. Services never share a database.

### CRITICAL: Sync communication via HTTP helpers
When a service needs data from another service (e.g., agora calls hermes for trade logic), it uses `helpers/externalRequest.go` with the service URL from config.

### CRITICAL: Never import another service's code directly
Services are separate repos. Cross-service logic lives in mercury.

---

## Mercury Library Rules

### IMPORTANT: Mercury contains only shared infrastructure
Mercury provides:
- Exchange adaptors (Binance spot + futures)
- Trade models and aggregates (`trades/aggragates`)
- Trade actions (buy, sell, profit calculation, fund checks)
- Message broker (Kafka producer/consumer)
- Auth middleware (`adaptors/gin`)
- Logging (`log`)
- Crypto (encrypt/decrypt exchange secrets)
- Redis cache (`storage/memory`)
- Virtual exchange (backtesting)

### IMPORTANT: Service-specific logic never goes into mercury
If logic is only used by one service, it stays in that service. Mercury is for genuinely shared concerns.

### STANDARD: Mercury import path
All services import mercury as `github.com/giovani-sirbu/mercury`.
Common import aliases:
```go
import (
    adapter "github.com/giovani-sirbu/mercury/adaptors/gin"
    "github.com/giovani-sirbu/mercury/log"
    "github.com/giovani-sirbu/mercury/trades/aggragates"
    "github.com/giovani-sirbu/mercury/messagebroker"
    "github.com/giovani-sirbu/mercury/storage/memory"
)
```
