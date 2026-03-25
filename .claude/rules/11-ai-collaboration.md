# AI Collaboration Rules

## CLAUDE.md is Infrastructure

### CRITICAL: Every service repo has a `CLAUDE.md` at the root
It describes the service purpose, routes, consumers, and conventions. Claude reads it at the start of every conversation.

### CRITICAL: `.claude/architecture.md` is updated per structural PR
If a PR adds a handler domain, Kafka topic, route group, or cron job — architecture.md must be updated.

---

## Search-Friendly Code

### CRITICAL: If Claude can't find it by searching, it's named wrong
Claude finds code by grepping names. If `grep "CreateTrade"` doesn't lead to the trade creation logic, something is misnamed.

### How Claude navigates this codebase:
1. Reads `CLAUDE.md` first (loaded at conversation start)
2. Checks `.claude/architecture.md` for service structure
3. Searches by name (Grep for content, Glob for files)
4. Reads targeted files
5. Follows the import trail

### What makes navigation fast vs slow:
| Fast (1-2 searches) | Slow (5+ searches) |
|---|---|
| `handlers/trades/createTrade.go` | `handlers/misc/operations.go` |
| `CreateTrade()` | `Process()` |
| `helpers/getUserSession.go` | `helpers/utils.go` |
| One function per file | Five functions per file |

---

## AI Code Review

### IMPORTANT: Review AI-generated code with the same rigor as human code
AI code must pass the same rules, same tests, same review process. You are responsible for every line in your PR.

### IMPORTANT: When adding a new handler, follow existing patterns exactly
Before writing a new handler, read 2-3 existing handlers in the same domain. Match:
- aggregates.go structure
- Payload type definitions
- Validation + session extraction
- Business method signature `(result, status, error)`
- Cache invalidation

---

## Research-First Protocol

For complex work (features, bugs, refactors):

### Phase 1: Discovery (Read-Only)
1. Read `CLAUDE.md` and `.claude/architecture.md`
2. Map the data flow: which services involved, which Kafka topics, which handlers
3. Search for existing implementations that solve similar problems
4. Study existing patterns before building new

### Phase 2: Verification
5. Verify understanding by explaining the flow
6. Check for blockers: ambiguous requirements? Multiple valid approaches?
7. If blockers exist, ask. If not, proceed.

### Phase 3: Execution
8. Execute autonomously. Default to action. Complete entire task chain.
9. Read-Write-Reread: read every file before and after modification.

### When to proceed autonomously:
- Research leads to clear implementation path
- Error discovered with understood root cause
- Task A complete, discovered related task B

### When to stop and ask:
- Ambiguous requirements
- Multiple valid architectural choices
- Security/risk concerns (exchange operations, data loss)
- Missing info only user can provide

---

## Context Window Management

- Read only directly relevant files
- Grep with specific patterns before reading entire files
- Start narrow, expand as needed
- Use `head_limit` on search results
- Don't retry the exact same search if it returns nothing — try different terms
- Avoid reading Go library/vendor directories — they are large and not needed
