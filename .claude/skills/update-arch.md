# /update-arch

---

## Mission: Update Architecture Documentation

Walk through the living architecture document and sync it with the current state of the codebase.

---

## Steps

### 1. Scan Current State
- Read `routes/routes.go` to get current routes
- Read `consumers/consumers.go` to get current Kafka consumers
- Read `jobs/` directory to get current cron jobs
- Read `handlers/` directory to get current handler domains
- Read `config/common.go` to get current config fields
- Check other services (hermes, hellenes, sisyphus, iris) for any changes

### 2. Compare Against `.claude/architecture.md`
- Identify additions (new routes, topics, jobs, handler domains)
- Identify removals (deleted routes, topics, handlers)
- Identify changes (renamed, moved, restructured)

### 3. Update Files
For each discrepancy found:

1. **`.claude/architecture.md`** — Update the living state document
2. **`CLAUDE.md`** — Update if the service table or stack changed
3. **`.claude/rules/03-architecture.md`** — Update if service patterns changed
4. **`.claude/rules/10-service-communication.md`** — Update topic map

### 4. Verify
- Confirm all routes in docs match actual `routes.go`
- Confirm all Kafka topics in docs match actual consumers
- Confirm all handler domains in docs exist as directories
- Confirm mercury package list matches actual packages

---

## Output

```
Scanned: [date]
Services: [count]
Routes: [count per service] (added: X, removed: Y)
Kafka Topics: [count] (added: X, removed: Y)
Jobs: [count] (added: X, removed: Y)
Files updated:
  - [list]
Status: Architecture docs synced with codebase.
```
