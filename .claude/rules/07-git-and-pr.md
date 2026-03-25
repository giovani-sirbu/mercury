# Git & PR Rules

## Branching

### STANDARD: Branch naming: `type/short-description`
```
feature/futures-loss-streak
fix/blocked-trade-unblock
refactor/extract-exchange-validation
```
Types: `feature/`, `fix/`, `refactor/`, `chore/`, `docs/`.

---

## Commits

### IMPORTANT: Imperative mood. Explain WHY, not what.
```
# BAD
"updated trade handler"
"fix bug"
"changes"

# GOOD
"reject trade creation when exchange balance below min notional"
"unlock blocked trades older than 24h to prevent stale positions"
"add cache invalidation after trade status change to fix stale reads"
```

### CRITICAL: Never commit secrets
If you accidentally commit a secret, rotate it immediately. Removing from history is not enough.

### STANDARD: No emojis in commit messages
Concise, technically descriptive. Proper technical terminology.

---

## Pull Requests

### CRITICAL: One PR = one concern
A PR either adds a feature, fixes a bug, or refactors code. Never all three. If a refactor is needed to add a feature, submit the refactor first as a separate PR.

### CRITICAL: No PR merged without passing tests
All tests must pass before merge.

### IMPORTANT: PR description includes:
- **What:** One-sentence summary of the change.
- **Why:** Why this change is needed.
- **Testing:** What was tested and how.

### IMPORTANT: Review AI-generated code with the same rigor as human code
"Claude wrote it" is not a justification for skipping review. You are responsible for every line in your PR.

---

## Post-Merge

### CRITICAL: Update architecture docs if structure changed
If your PR adds a new handler domain, Kafka topic, route group, config field, or cron job — update:
- `.claude/architecture.md`
- `CLAUDE.md` (if the summary needs updating)

Enforced by the `/post-merge` skill.
