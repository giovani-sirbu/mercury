# Architecture — Living Document

> Updated after every structural PR.
> Last updated: 2026-03-25

---

## Repositories

| Repo | Type | Purpose |
|---|---|---|
| **hermatic** | React Native app | Mobile trading dashboard — auth, trades, stats, exchange config |
| **agora** | Go service | Trade management — create/pause/resume/stop trades, exchange config, admin |
| **hermes** | Go service | Core trading engine — price websocket, buy/sell logic, trade locking |
| **hellenes** | Go service | User service — auth, accounts, sessions |
| **sisyphus** | Go service | Backtesting + live strategy testing |
| **iris** | Go service | Email delivery (pub/sub consumer only) |
| **mercury** | Go library | Shared library — exchange adaptors, trade actions, message broker, crypto, log |
| **pythia** | TBD | TBD |

---

## hermatic — Mobile App (React Native)

**Stack:** React Native 0.74, TypeScript, Redux + Redux Saga, Axios, React Navigation

**Folder Structure:**
```
hermatic/
  src/
    App.tsx                    # Root component
    api/                       # HTTP client + endpoint modules
      client/                  # Axios instance + interceptors
      auth/                    # Auth API calls
      trades/                  # Trades API calls
      exchanges/               # Exchanges API calls
      settings/                # Settings API calls
      stats/                   # Stats API calls
      user/                    # User API calls
    screens/                   # Screens organized by flow
      AuthFlow/                # SignIn, SignUp, Onboarding, PasswordRecovery
      HomeFlow/                # Home (active trades, stats, trading controls)
      StatsFlow/               # Stats, TradeHistory, DatesFilter
      ProfileFlow/             # Profile, Exchanges, Settings
    components/                # Shared reusable components
      Buttons/                 # PrimaryButton, PopUpButton
      Inputs/                  # CommonInput, EmailInput, PasswordInput, SearchInput
      DropDown/                # Dropdown selector
      ...
    store/                     # Redux store
      modules/                 # Feature slices
        Account/               # actions/ reducer/ sagas/ selectors/
        Trades/                # actions/ reducer/ sagas/ selectors/
        Exchanges/             # actions/ reducer/ sagas/ selectors/
        Statistics/            # actions/ reducer/ sagas/ selectors/
        Prices/                # actions/ reducer/ selectors/
        AppCommon/             # actions/ reducer/ sagas/ selectors/ hooks/
        ActiveTab/             # actions/ reducer/ selectors/
        Linking/               # actions/ reducer/ sagas/ selectors/ hooks/
        Users/                 # actions/ reducer/ selectors/
    navigation/                # React Navigation setup
      Router.tsx               # Root navigator
      Auth.tsx                 # Auth stack
      Main.tsx                 # Main tab navigator
      components/              # Header, TabBar, BottomModal
      hooks/                   # Navigation hooks
    services/                  # Device services
      auth/                    # Auth token management
      keychain/                # Secure storage (react-native-keychain)
    constants/                 # colors, fonts, styles, env, exchanges, links
    utils/                     # date, numbers, email, password, exchange, device
    hooks/                     # Shared hooks
    translations/              # i18n locale files
    assets/                    # fonts, images, lottie, svg
```

**Screen Flows:**
| Flow | Screens | Purpose |
|---|---|---|
| AuthFlow | Onboarding, SignIn, CheckEmail, PasswordRecovery, AuthStatus | User authentication |
| HomeFlow | Home, Welcome | Active trades dashboard, trade controls |
| StatsFlow | Stats, TradeHistory, DatesFilter | Trading statistics and history |
| ProfileFlow | Profile | Exchange config, settings, account |

**API → Backend mapping:**
| App API module | Backend service | Base path |
|---|---|---|
| `api/auth` | hellenes | `/login`, `/refresh-token` |
| `api/user` | hellenes | `/`, `/:userId` |
| `api/trades` | agora | `/`, `/:tradeId/*` |
| `api/exchanges` | agora | `/exchanges/*` |
| `api/settings` | agora | `/settings/*` |
| `api/stats` | agora | `/exchanges/:id/stats`, `/exchanges/:id/best-pairs` |

---

## Service Details

### agora — Trade Management
**Routes:**
| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/` | IsAuth | Create trade |
| GET | `/active-trades-ids` | IsAuth | Get active trade IDs |
| GET | `/:tradeId` | IsAuth | Get trade |
| POST | `/:tradeId/pause` | IsAuth | Pause trade |
| POST | `/:tradeId/resume` | IsAuth | Resume trade |
| POST | `/:tradeId/stop` | IsAuth | Stop trade |
| POST | `/:tradeId/unblock` | — | Unblock trade |
| GET | `/:tradeId/children-trades` | IsAuth | Get child trades |
| POST | `/generate` | IsAuth | Generate trades |
| GET | `/exchanges/:id/loss-streak` | IsAuth | Futures loss streak |
| GET | `/exchanges/:id/active-pairs` | IsAuth | Active pairs |
| GET | `/exchanges/:id/stats` | IsAuth | Dashboard stats |
| GET | `/exchanges/:id/inverse-used-amount` | IsAuth | Inverse used amount |
| GET | `/exchanges/:id/best-pairs` | IsAuth | Best pairs |
| POST | `/exchanges/:id/pause-trades` | IsAuth | Pause all trades |
| POST | `/exchanges/:id/resume-trades` | IsAuth | Resume all trades |
| POST | `/exchanges/:id/stop-trades` | IsAuth | Stop all trades |
| GET | `/exchanges/:id` | IsAuth | Get exchange |
| GET | `/exchanges/:id/assets` | IsAuth | User assets |
| GET | `/exchanges` | IsAuth | List exchanges |
| GET | `/exchanges/user-id/:userId` | IsAdmin | Admin: list user exchanges |
| POST | `/exchanges` | IsAuth | Add exchange |
| PATCH | `/exchanges/:id` | IsAuth | Update exchange |
| DELETE | `/exchanges/:id` | IsAuth | Delete exchange |
| GET | `/settings/exchange-grouped-active-symbols` | IsAuth | Active symbols by exchange |
| POST | `/settings/pairs` | IsAdmin | Add trading pair |
| GET | `/settings/app-version` | — | App version |
| GET | `/settings/active-exchanges` | IsAuth | Active exchanges |
| GET | `/admin/trades` | IsAdmin | Query all trades |
| POST | `/admin/trades/:id/pause` | IsAdmin | Admin pause trade |
| POST | `/admin/trades/:id/stop` | IsAdmin | Admin stop trade |
| GET | `/admin/dashboard/overview` | IsAdmin | Dashboard overview |
| GET | `/admin/dashboard/top-profits` | IsAdmin | Top profits |

**Pub/Sub Consumers:**
| Topic | Handler | Description |
|---|---|---|
| `update-trade` | UpdateTrade | Hermes updates trade after buy/sell |
| `create-children-trades` | CreateChildrenTrade | Hermes creates child impasse trades |
| `delete-user-associated-data` | DeleteUserAssociatedData | Hellenes triggers cleanup |

**Cron Jobs:**
| Job | Description |
|---|---|
| handleBlockedTradesCronjob | Unblocks stuck trades |
| handleOldBlockedTradesCronjob | Handles old blocked trades |
| processPendingOrder | Processes pending exchange orders |

**Infrastructure:** PostgreSQL, Redis (cache), messagebus Postgres (pub/sub outbox)

---

### hermes — Core Trading Engine
**Purpose:** Subscribes to Binance WebSocket price feeds, runs buy/sell logic against active trades, produces pub/sub events back to agora.

**Pub/Sub Producers:**
| Topic | Description |
|---|---|
| `update-trade` | Sends trade state updates to agora |
| `create-children-trades` | Requests child trade creation from agora |

**Key handlers:**
- `manageTrades.go` — main loop: get active trade IDs → lock → run trade logic per goroutine
- `handleTrade.go` — spot trade logic
- `handleFuturesTrade.go` — futures trade logic
- `managePrices.go` — WebSocket price subscription
- `recordWsPrice.go` — records latest prices into the in-process wsPrices map (the old cacheWsPrice.go pushed the whole map to Dragonfly on every unlock; now it is a plain lock-protected write)
- `snapshotWsPrices.go` — returns a defensive copy of the in-process map; consumed by handleTrade/handleFuturesTrade (WsPrices on events.Events) and by the new prices HTTP domain
- `handlers/prices/` — GET /prices endpoint gated by `adapter.IsAuth`, exposes the snapshot to peer services (agora mints a service-role JWT via `helpers.GenerateToken`)
- `lockTrade.go` / `unlockTrade.go` — prevents concurrent processing per trade

**Infrastructure:** Redis (trade locks only; price map is in-process since Faza 3), messagebus Postgres (pub/sub outbox)

---

### hellenes — User Service
**Routes:**
| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/login` | — | Login |
| POST | `/refresh-token` | IsAuth | Refresh JWT |
| POST | `/` | — | Create account |
| POST | `/reset-password` | — | Reset password |
| GET | `/session` | IsAuth | Get session |
| GET | `/:userId` | IsAuth | Get account info |
| DELETE | `/:userId` | IsAuth | Delete account |
| GET | `/:userId/activate/:code` | — | Activate account |
| PATCH | `/:userId/set-password/:code` | — | Change password |
| GET | `/user-ids/:userIds` | IsAuth | Get accounts by IDs |
| GET | `/admin/accounts` | IsAdmin | Query accounts |
| POST | `/admin/accounts` | IsAdmin | Create account |
| PUT | `/admin/accounts/:id` | IsAdmin | Update account |
| DELETE | `/admin/accounts/:id` | IsAdmin | Delete account |

**Pub/Sub Consumers:**
| Topic | Handler |
|---|---|
| `update-user-data` | UpdateUserData |

**Infrastructure:** PostgreSQL, messagebus Postgres (pub/sub outbox)

---

### sisyphus — Backtesting + Testing
**Routes:**
| Method | Path | Description |
|---|---|---|
| POST | `/backtests` | Run backtest |
| GET | `/backtests` | Query backtests |
| GET | `/backtests/:id` | Get backtest info |
| POST | `/backtests/:id/stop` | Stop backtest |
| DELETE | `/backtests/:id` | Delete backtest |
| POST | `/run-testing` | Run live strategy test |

**Infrastructure:** PostgreSQL, MinIO (historical data storage)

---

### iris — Email Service
**Pub/Sub Consumers:**
| Topic | Handler |
|---|---|
| `send-emails` | SendEmail |

**Infrastructure:** messagebus Postgres (pub/sub outbox), SMTP

---

## mercury — Shared Library

| Package | Purpose |
|---|---|
| `adaptors/gin` | `IsAuth`, `IsAdmin`, `Response()`, `ValidationResponse()` |
| `auth` | JWT validation |
| `crypto` | Encrypt/decrypt exchange API secrets |
| `exchange` | Exchange interface + Binance adaptor |
| `exchange/adaptors/binance` | Binance spot + futures client |
| `log` | `Error()`, `Info()`, `Warn()`, `Debug()` |
| `messagebroker` | Postgres LISTEN/NOTIFY pub/sub + outbox table (drop-in replacement for the former Kafka implementation) |
| `storage/memory` | Redis cache |
| `strategies` | Strategy definitions |
| `trades/actions` | buy, sell, calculateProfit, hasFunds, etc. |
| `trades/aggragates` | Shared trade models: Trades, TradesHistory, TradesExchanges, etc. |
| `virtualExchange` | Virtual exchange for backtesting |
| `events` | lockTradeBackoff |

---

## Trade Status Lifecycle

```
new → active → inPosition → active (cycle)
             ↘ closed
   active → paused → active
   active → blocked → active (via unblock)
   active → impasse (child trade for loss recovery)
```

**Status values:** `active`, `blocked`, `paused`, `closed`, `impasse`, `inPosition`
**Trade types:** `spot`, `futures`

---

## Key Architectural Decisions

| Decision | Rationale | Date |
|---|---|---|
| Separate repos per service | Independent deploy cycles, clear ownership | — |
| mercury as shared library | Avoid duplication of exchange logic, trade math, auth | — |
| Postgres LISTEN/NOTIFY + outbox for hermes→agora comms | Decoupled: hermes fires and forgets, agora persists. Replaces Kafka — simpler ops, durable via outbox, same messagebroker API | 2026-04-18 |
| Redis for trade locks in hermes | Prevents concurrent buy/sell on same trade | — |
| Redis cache in agora | Active trade IDs are hot path — reduces DB load | — |
| GORM direct in handlers | No separate repository layer — keep services small | — |
| Handler returns (result, status, error) | Separates HTTP concern from business concern | — |
| Memory cache singleton + in-process WsPrices snapshot | Prior Memory re-created the Redis client on every Set/Get/Delete (measured 3.5ms/op). Singleton with sync.Once reuses the client + TinyLFU local cache; hermes' in-process map feeds events.Events.WsPrices so GetFees no longer round-trips Dragonfly on the trade-decision hot path. Peer reads of the snapshot go through GET /prices on hermes (JWT-gated via `adapter.IsAuth`; agora mints a service-role token) instead of a shared Dragonfly key | 2026-04-21 |
| HTTP cross-service helpers carry timeout + retry | All externalRequest.go use a shared *http.Client with a 5s timeout and a 2-attempt retry on transport errors. Prior bare http.Client{} could hang a goroutine indefinitely when a peer stalled | 2026-04-21 |
| agora UpdateTrade consumer gates side-effects on state transitions | At-least-once delivery means a replay of the same payload is expected. The Closed→(create futures replacement) block now fires only on a real non-Closed→Closed transition, not on every replay of the final state | 2026-04-21 |

---

## Known Deviations

_Document deviations from the architecture rules here with rationale._

### Committed secrets — provider rotation pending

The live values that were in-tree (`API_SECRET`, DO Postgres credentials) have been blanked in the `.env.sample` files. Git history still contains the old values, so the follow-up work owned externally is:

1. Rotate the DO Postgres user password and update every running sisyphus deployment.
2. Regenerate `API_SECRET` on agora + hermes + any other caller, re-encrypt stored exchange secrets (decrypt with old key, re-encrypt with new).
3. Optional: scrub git history with BFG or filter-repo; otherwise the old values remain discoverable via `git log`.

