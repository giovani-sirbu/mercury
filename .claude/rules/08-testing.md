# Testing Rules

## Test Location

### STANDARD: Tests colocated or in a `_test.go` file next to source
Go convention: `createTrade_test.go` next to `createTrade.go`. Or domain-level test files like `trades_test.go`.

Mercury already follows this pattern (e.g., `trades/tests/buy_test.go`).

---

## Test Coverage

### IMPORTANT: Every business method should have a test
Business methods (the ones returning `(result, statusCode, error)`) are the primary test boundary. They contain the logic worth testing. HTTP handlers are thin wrappers and don't need separate tests.

### IMPORTANT: Test behavior, not implementation
Test what the function returns and its side effects — not which internal methods it calls.

```go
// BAD — tests implementation
func TestCreateTrade_CallsDBCreate(t *testing.T) { ... }

// GOOD — tests behavior
func TestCreateTrade_RejectsDuplicateSymbol(t *testing.T) {
    trade, status, err := handler.CreateTrade(payload, exchange, userId, 0)
    assert.Equal(t, http.StatusBadRequest, status)
    assert.EqualError(t, err, "TRADE_ALREADY_EXIST")
}
```

---

## Test Naming

### STANDARD: Test names describe the scenario
```go
// BAD
func TestCreateTrade(t *testing.T) {}

// GOOD
func TestCreateTrade_RejectsDuplicateSymbol(t *testing.T) {}
func TestCreateTrade_RejectsInsufficientBalance(t *testing.T) {}
func TestCreateTrade_SucceedsWithValidPayload(t *testing.T) {}
```

---

## Test Boundaries

| Service | Primary Test Target | What Tests Validate |
|---|---|---|
| **agora** | Handler business methods | Trade CRUD, exchange validation, cache invalidation |
| **hermes** | Trade management logic | Buy/sell decisions, lock/unlock, price management |
| **hellenes** | Auth and user handlers | Login, session, account lifecycle |
| **sisyphus** | Backtest execution | Strategy simulation, profit calculation |
| **mercury** | Trade actions | buy, sell, profit calc, fund checks |

---

## Quality Standards

### Before submitting a PR:
- Does it actually work? (Not just compile — does it function correctly?)
- Did I test the integration points?
- Are there edge cases I haven't considered?
- Will this perform okay? (No N+1 queries, no unbounded loops)
- Did I clean up? (No temp files, debug code, fmt.Println)
