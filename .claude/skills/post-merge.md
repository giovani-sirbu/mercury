# /post-merge

---

## Mission: Post-Merge Architecture Sync

After every merged PR, verify that architecture documentation is up to date.

---

## Checklist

Scan the merged changes and answer each question:

### 1. Handler/Route Changes
- [ ] Was a new handler domain added (new folder in `handlers/`)?
- [ ] Was a handler domain removed?
- [ ] Were new routes added to `routes/routes.go`?
- [ ] Were routes removed or changed?
- [ ] Was auth middleware added/removed from a route?

### 2. Kafka Topic Changes
- [ ] Was a new Kafka topic added (producer or consumer)?
- [ ] Was a topic removed?
- [ ] Did consumer registration in `consumers/consumers.go` change?

### 3. Job Changes
- [ ] Was a new cron job added to `jobs/`?
- [ ] Was a job removed?

### 4. Config Changes
- [ ] Were new environment variables added to config?
- [ ] Was a config section added or removed?

### 5. Mercury Library Changes
- [ ] Was a new mercury package added?
- [ ] Was a mercury package removed or restructured?
- [ ] Did trade models/aggregates change?

### 6. Infrastructure Changes
- [ ] Did the tech stack change? (new database, new cache, new service)
- [ ] Was a new service added?
- [ ] Was a service removed?

---

## If ANY answer is YES:

Update these files:

1. **`.claude/architecture.md`** — Update the relevant section (routes, topics, jobs, mercury packages).

2. **`CLAUDE.md`** (root) — Update if the service table, stack table, or folder structure changed.

3. **`.claude/rules/03-architecture.md`** — Update if service patterns or mercury structure changed.

4. **`.claude/rules/05-trading-domain.md`** — Update if trade lifecycle, statuses, or strategy logic changed.

5. **`.claude/rules/10-service-communication.md`** — Update the topic map if Kafka topics were added/removed.

---

## If ALL answers are NO:

No documentation updates needed. Report: "Architecture docs are current. No updates required."

---

## Output

```
PR: [PR title or number]
Structural changes detected: [yes/no]
Files updated:
  - [list of updated doc files, or "none"]
Status: Architecture docs are current.
```
