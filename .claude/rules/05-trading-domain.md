---
paths:
  - "handlers/trades/**/*"
  - "handlers/exchanges/**/*"
  - "handlers/orders/**/*"
  - "jobs/*"
---

# Trading Domain Rules

## Trade Lifecycle

```
new -> active -> inPosition -> active (buy/sell cycle)
              -> closed (trade ended)
     active -> paused -> active (user paused/resumed)
     active -> blocked -> active (error -> unblocked)
     active -> impasse (child trade for loss recovery)
```

### Status values (from mercury aggregates)
| Status | Meaning |
|---|---|
| `active` | Trade is running, hermes processes it |
| `blocked` | Exchange error, needs unblocking |
| `paused` | User paused, hermes skips it |
| `closed` | Trade completed (sold or stopped) |
| `impasse` | Loss recovery child trade |
| `inPosition` | Currently holding a position |

### Trade types
| Type | Description |
|---|---|
| `spot` | Buy low, sell high on spot market |
| `futures` | Leveraged futures trading |

---

## Rules

### CRITICAL: All trade mutations must invalidate cache
When a trade status changes (create, pause, resume, stop, update), clear:
- Active trade IDs cache: `exchange:{name}:symbol:{symbol}:active_trades_ids`
- User active trades cache: `user:{id}:exchange:{id}:active_trades`

### CRITICAL: Exchange balance checks before spot trade creation
Before creating a spot trade, verify:
1. Exchange API key is valid (no permission errors)
2. Sufficient balance for the initial bid
3. Balance >= minimum notional for the trading pair

### CRITICAL: Scope all trade queries to user_id
Every user-facing trade query must include `WHERE user_id = ?`. Admin routes are the only exception.

### IMPORTANT: Trade locking in hermes
Hermes locks trades by symbol pair before processing to prevent concurrent buy/sell on the same trade. Lock pattern:
1. Lock pair mutex
2. Get active trade IDs for pair
3. Process each trade in parallel (goroutines + WaitGroup)
4. Wait for all trades to complete
5. Sleep 1 second
6. Unlock pair

### IMPORTANT: Inverse trades
When `Inverse = true`, the profit asset is the base asset (first in pair), not the quote. This affects:
- Profit calculation
- Initial bid calculation (multiply by position price)
- Used amount calculation

### IMPORTANT: Child trades (impasse)
When a trade is in loss, hermes can create a child trade via the `create-children-trades` Kafka topic. The child has `ParentID` set. Impasse depth is different from normal depth.

---

## Exchange Configuration

Exchange records (`TradesExchanges`) store:
- Exchange name (e.g., "binance")
- Encrypted API key + secret (encrypted via mercury crypto)
- Trade type (spot/futures)
- Status (active/inactive)

### CRITICAL: Never log or expose exchange API keys/secrets
Exchange secrets are encrypted at rest. Only decrypt when making exchange API calls. Never include in responses, logs, or error messages.

### IMPORTANT: Exchange status management
When creating the first trade on an exchange, set exchange status to `active`. When all trades on an exchange are closed, consider setting to inactive.

---

## Strategy and Pairs

Each trade is linked to:
- A **strategy** — defines trade type, depth settings, percentages
- A **strategy pair** — symbol-specific settings (min notional, filters)

Strategy settings control:
- `Depths` / `MinDepths` — how deep to average down
- `Percentage` — price drop percentage between buys
- `Multiplier` — quantity multiplier per depth level
- `ImpasseDepth` — depth for child/impasse trades
