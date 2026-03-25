# /feature

$ARGUMENTS

---

## Mission: Implement Feature — Standard Operating Protocol

Execute in full compliance with all rules in `.claude/rules/`. Each phase is mandatory.

---

## Phase 0: Reconnaissance (Read-Only)

- Scan the relevant parts of the codebase to build a complete mental model.
- **Deduplication scan:** Check existing handlers, helpers, mercury actions, and patterns relevant to this request. Reuse — never reimplement.
- Map which services are involved and how they communicate.
- Output: concise digest of findings.
- **No mutations permitted during this phase.**

---

## Phase 1: Planning

1. **Restate objectives.** Define success criteria.
2. **Check existing handlers.** The feature might already have the handler you need.
3. **Identify full impact surface.** All files, services, Kafka topics, and cache keys affected.
4. **Justify strategy.** Propose the approach. Explain WHY it's the best choice. Align with existing patterns.
5. **DRY analysis.** For every piece of logic you plan to write, confirm no existing implementation covers it. If shared logic is needed across services, it belongs in mercury.

---

## Phase 2: Execution

Execute incrementally, following all rules:

- **200 lines max per file.**
- **One handler = one file.** If creating a new handler:
  - Add payload/response types to `aggregates.go`
  - Create `[verb][Noun].go` with business method + HTTP handler
  - Register route in `routes/routes.go` with proper auth middleware
  - If Kafka consumer: register in `consumers/consumers.go`, create handler file
  - If cron job: create in `jobs/`, register in `jobs/init.go`
- **Follow existing patterns exactly.** Match aggregates.go structure, validation, session extraction, error returns.
- **Read-Write-Reread:** Read every file before and after modification.
- **Zero duplication:** Before writing any function, search for existing implementations.
- **System-wide ownership:** If you modify mercury or a shared handler, update ALL its consumers.

---

## Phase 3: Verification

1. Run `go build` and `go test ./...`.
2. If any gate fails, autonomously diagnose and fix.
3. Test the primary user workflow affected by your changes.

---

## Phase 4: Zero-Trust Self-Audit

1. **Re-verify final state.** All modified files correct.
2. **Hunt for regressions.** Test a related feature you didn't modify.
3. **Duplication audit.** Grep for similar function signatures. If duplication found, refactor.
4. **Cache audit.** If you added mutations, verify cache invalidation.

---

## Phase 5: Final Report

```
Changes: [list of created/modified files with file:line references]
Verification: [build + test results]
Impact: [all affected services and topics verified]
Architecture: [does .claude/architecture.md need updating? If yes, update it.]
Verdict: "System verified. No regressions." OR
         "CRITICAL ISSUE FOUND. [describe + next steps]"
```
