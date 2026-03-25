# Trading Bot — Microservices + Mobile App

## Project
A crypto trading platform: Go microservices backend for automated trading on Binance (spot and futures) + React Native mobile app. Backend services communicate via Kafka topics and HTTP calls. Shared Go logic lives in the `mercury` library.

## Services

| Service | Type | Purpose |
|---|---|---|
| **hermatic** | React Native app | Mobile trading dashboard — auth, trade management, stats, exchange config |
| **agora** | Go service | Trade management — CRUD, exchanges, settings, admin |
| **hermes** | Go service | Core trading engine — buy/sell decisions, price management |
| **hellenes** | Go service | User service — auth, accounts, sessions |
| **sisyphus** | Go service | Backtesting + live strategy testing |
| **iris** | Go service | Email delivery (Kafka consumer only) |
| **mercury** | Go library | Shared library — exchange adaptors, trade actions, auth, logging |
| **pythia** | TBD | TBD |

Architecture, tech stack, Kafka topics, and service relationships: `.claude/architecture.md` — always check it for current state.

**After every PR that changes service structure** (new handler, new Kafka topic, new route group, new config field), update `.claude/architecture.md` and this file. Run `/post-merge` to check.

---

## Stack

### Backend (Go)
| Concern | Technology |
|---|---|
| HTTP framework | Gin |
| Database ORM | GORM (PostgreSQL) |
| Message broker | Kafka (via mercury `messagebroker`) |
| Cache | Redis (via mercury `storage/memory`) |
| Crypto exchange | Binance (via mercury `exchange/adaptors/binance`) |
| Auth | JWT middleware via mercury `adaptors/gin` |
| Shared library | `github.com/giovani-sirbu/mercury` |
| Config | `godotenv` + struct per section |

### Mobile App (hermatic)
| Concern | Technology |
|---|---|
| Framework | React Native 0.74 |
| Language | TypeScript |
| State management | Redux + Redux Saga + redux-persist |
| HTTP client | Axios (with interceptors) |
| Navigation | React Navigation (native-stack + bottom-tabs) |
| Secure storage | react-native-keychain |
| Config | react-native-config (.env) |
| Charts | react-native-gifted-charts |
| Animations | react-native-reanimated + Lottie |
| i18n | i18n-js |
| Linting | ESLint + Prettier + Husky + commitlint |

---

## Developer Rules

### Service Folder Structure
Every service follows this exact layout:
```
[service]/
  main.go              # Bootstrap only: init config, server, routes, consumers, jobs
  config/
    config.go          # Config struct + Init() entry point
    db.go              # DBConfig + LoadDBConfig()
    http.go            # HTTPConfig + LoadHTTPConfig()
    common.go          # CommonConfig + LoadCommonConfig()
  server/
    server.go          # Server struct + NewServer() + Start()
  routes/
    routes.go          # All route registrations (routes.Init)
  handlers/
    [domain]/
      aggregates.go    # Handler struct, constructor, payload types, response types
      [verb][Noun].go  # One handler action per file
  consumers/
    consumers.go       # Register all Kafka consumers
  jobs/
    aggregates.go      # Jobs handler struct + constructor
    [job].go           # One cron job per file
  db/
    connection.go      # DB connection init
  helpers/
    [specific-name].go # One helper per file, named after what it does
  models/
    [domain].go        # DB models for this service
```

### Handler Pattern
Every handler domain follows:
```go
// aggregates.go — types and constructor
type Handler struct {
    server *server.Server
}

func NewHandler(server *server.Server) *Handler {
    return &Handler{server: server}
}

type CreateXPayload struct { ... }

// createX.go — one file per action
func (handler *Handler) CreateX(c *gin.Context) {
    // 1. Bind + validate payload
    // 2. Get user session (if auth required)
    // 3. DB query / business logic
    // 4. Respond
}
```

### The Orchestrator Pattern
When a handler function is too long, split into private methods:
```go
func (handler *Handler) CreateTrade(payload CreateTradePayload, ...) (Trade, int, error) {
    trade, err := handler.validateTradeDoesNotExist(payload, userId)
    if err != nil { return trade, http.StatusBadRequest, err }
    trade = handler.buildTradeFromPayload(payload, exchange)
    if err := handler.checkExchangeBalance(trade); err != nil { ... }
    return handler.saveTrade(trade)
}
```
Business logic returns `(result, httpStatusCode, error)`. HTTP handlers call business methods and respond.

### Naming
- **Files:** verb + noun in camelCase matching function name. `createTrade.go` contains `CreateTrade`. `stopTrade.go` contains `StopTrade`.
- **Functions:** verb + noun. `GetTrade()`, `CreateTrade()`, `UpdateTradesExchangeStatus()`.
- **Handlers:** HTTP handlers have `Req` suffix when paired with a business logic method: `CreateTradeReq` calls `CreateTrade`.
- **Payloads:** `[Action][Domain]Payload`. `CreateTradePayload`, `UpdateTradePayload`.
- **No generic names.** No `utils.go`, `helpers.go`, `common.go`. Use `paginate.go`, `getUserSession.go`, `validatePayload.go`.

### File Rules
- One primary export per file.
- `aggregates.go` is the only allowed multi-declaration file — it holds the `Handler` struct, constructor, and all payload/response types for that domain.
- No file exceeds 200 lines. Split by responsibility before it gets there.

### Error Handling
- HTTP handler: check every error, log with context, respond with `adapter.Response(c, status, "ERROR_CODE")`.
- Business method: return `(result, httpStatusCode, error)` — never respond to HTTP directly.
- Never swallow errors silently. Every `if err != nil` must act.
- Log errors with `log.Error(err.Error(), "FunctionName", "HandlerName")` from mercury.

### Auth
- Protected routes: `adapter.IsAuth` middleware.
- Admin routes: `adapter.IsAdmin` middleware.
- Get user from context: `helpers.GetUserSession(c)`.

### Config
- All config via environment variables loaded through `godotenv`.
- Each section in its own file: `db.go`, `http.go`, `common.go`.
- Never hardcode URLs, credentials, or ports.

### Security
- Never commit secrets or `.env` files.
- Validate all request payloads with `helpers.ValidatePayload(payload)`.
- Scope every DB query to `user_id` on user-facing routes.
- Never expose exchange API keys or secrets in responses or logs.

### Git & PRs
- One PR = one concern.
- Branch naming: `feature/`, `fix/`, `refactor/` + short description.
- Commit messages: imperative mood, explain WHY not what.
- No PR merged without passing tests.

---

## Slash Commands
- `/fix-bug` — root cause analysis and remediation protocol
- `/feature` — structured feature implementation
- `/security-audit` — adversarial security review
- `/post-merge` — check if architecture docs need updating
- `/update-arch` — sync architecture docs with current state

## Detailed Rules
Full rules with examples and severity levels: `.claude/rules/`
