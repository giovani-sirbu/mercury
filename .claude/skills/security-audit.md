# /security-audit

$ARGUMENTS

---

## Mission: Adversarial Security Review

You are a principal engineer and adversarial reviewer. Review the changes with a hostile mindset, assuming there is a bug, exploit, race condition, or abuse path unless proven otherwise.

---

## Scope

- Review the current task changes AND impacted flows, dependencies, and interfaces.
- Consider the full stack: Go services, Kafka topics, PostgreSQL, Redis, Binance API, mercury library.
- Be strict, skeptical, and specific. No generic praise.

---

## Audit Categories

### 1. Functional Correctness
- Logic bugs, broken edge cases, invalid assumptions
- Floating point precision issues (trade amounts, profits)
- Off-by-one, nil pointer, zero-value issues
- Trade status transitions that skip states
- Cache/DB inconsistency (stale reads after mutations)

### 2. Concurrency & State Safety
- Race conditions in trade processing (hermes parallel goroutines)
- Double-buy / double-sell on same trade
- Trade lock bypass (concurrent price updates)
- Stale cache reads after Kafka consumer updates
- Non-atomic DB writes that can leave partial state

### 3. Security
- Auth bypass (missing `adapter.IsAuth` middleware)
- Broken tenant isolation (missing `user_id` in WHERE clause)
- Exchange API key/secret exposure in logs or responses
- IDOR — user accessing another user's trades/exchanges by manipulating IDs
- Injection via unsanitized user input in GORM queries
- Missing payload validation (`helpers.ValidatePayload`)

### 4. Reliability & Operations
- Missing error handling (unchecked `err != nil`)
- Silent failures in Kafka consumers
- Missing cache invalidation after trade mutations
- Missing retry/timeout on cross-service HTTP calls
- Resource leaks (unclosed connections, goroutine leaks)
- Blocked trades that never get unblocked

### 5. Trading-Specific Risks
- Insufficient balance check bypass
- Wrong initial bid calculation (depth/multiplier edge cases)
- Inverse trade profit calculation errors
- Futures position not properly closed
- Child trade created for already-closed parent
- Loss streak calculation errors affecting strategy decisions

### 6. Performance
- N+1 queries (loading trades with history/logs without preload)
- Unbounded trade list queries without pagination
- Redis cache misses causing DB storms
- Goroutine leaks in hermes trade management loop

---

## Output Format

For every finding:
- **Title**
- **Severity:** Critical / High / Medium / Low
- **Why it matters**
- **Exact vulnerable flow or code pattern**
- **Exact fix**
- **Test to add**
- **Exploit scenario** (if relevant)

---

## If No Issues Found

Do NOT stop at "looks good." Return:
- Residual risks
- What was checked
- Hardening improvements
- Tests still worth adding

---

## Mandatory Checklist

- [ ] Inputs validated with `helpers.ValidatePayload`?
- [ ] Auth middleware on every protected route?
- [ ] DB queries scoped to `user_id`?
- [ ] Cross-service calls have timeout and error handling?
- [ ] Trade state changes invalidate cache?
- [ ] Duplicate Kafka messages handled safely?
- [ ] Exchange secrets excluded from logs and responses?
- [ ] Error messages don't leak internal details?
- [ ] Trade locks prevent concurrent processing?
- [ ] Profit calculations handle edge values (zero, negative, very small)?
- [ ] Inverse trade math correct?
- [ ] Futures positions properly closed?

---

## Final Output

1. **Merge decision:** Safe to merge / Safe with follow-ups / Do not merge
2. **Top 3 must-fix items**
3. **Tests missing before production**
