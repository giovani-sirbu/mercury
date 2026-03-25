# /fix-bug

$ARGUMENTS

---

## Mission: Root Cause Analysis & Remediation

Previous simpler attempts may have failed. Standard shortcuts are suspended. You will follow a deep diagnostic protocol.

Your approach must be systematic, evidence-based, and focused on the **absolute root cause.** Patching symptoms is a critical failure.

---

## Phase 0: Reconnaissance (Read-Only)

- Scan the relevant handlers, consumers, jobs, and mercury code.
- Map the data flow: which service produces, which consumes, what DB queries are involved.
- Check Kafka topics and cache keys that may be affected.
- Output: concise digest of findings.
- **No mutations permitted during this phase.**

---

## Phase 1: Isolate the Anomaly

1. **Define correctness:** State the expected, non-buggy behavior.
2. **Create a failing test:** Write a test that fails precisely because of this bug.
3. **Pinpoint the trigger:** Identify the exact conditions, inputs, or sequence that causes the failure.

**Do not attempt any fixes until you can reliably reproduce the failure.**

---

## Phase 2: Root Cause Analysis

1. **Formulate a testable hypothesis.** Clear, simple theory about the cause.
2. **Devise an experiment.** A safe, non-destructive test to prove or disprove.
3. **Execute and conclude.** Present evidence. If wrong, formulate a new hypothesis.

**FORBIDDEN:**
- Applying a fix without a confirmed root cause.
- Re-trying a previously failed fix without new data.
- Patching a symptom (e.g., adding a nil check) without understanding WHY the value is nil.

---

## Phase 3: Remediation

- Design and implement a **minimal, precise fix** that addresses the confirmed root cause.
- **Read-Write-Reread:** Read every file before and after modification.
- **System-Wide Ownership:** If the root cause is in mercury or a shared handler, analyze ALL affected consumers across services.
- Follow all engineering standards (200-line files, naming conventions, handler patterns).

---

## Phase 4: Verification

1. **Confirm the fix:** Re-run the failing test from Phase 1. It MUST now pass.
2. **Run full quality gates:** `go build`, `go test ./...`.
3. **Autonomous correction:** If your fix introduces new failures, diagnose and resolve them.

---

## Phase 5: Zero-Trust Self-Audit

1. **Re-verify final state:** Confirm all modified files are correct.
2. **Hunt for regressions:** Test a related feature you did NOT modify.
3. **Check all consumers:** If you changed shared code (mercury), verify all service callers still work.
4. **Check cache keys:** If the bug involved stale data, verify cache invalidation is correct.

---

## Phase 6: Final Report

```
Root Cause: [definitive statement + key evidence]
Remediation: [list of all changes with file:line references]
Verification: [proof the bug is fixed, no regressions]
Verdict: "Root cause addressed. System verified. No regressions." OR
         "CRITICAL ISSUE FOUND. [describe + recommend next steps]"
```
