# Regression Discovery Log

This is an **exploratory log** from systematic regression testing of the
SQLite→Dolt backend migration. It documents bugs found, protocol invariants
confirmed, and test ideas for future work. Not all findings here are
actionable — some are by-design tradeoffs. The audit column tracks triage.

## Session Log

| Date | What we did | Outcome |
|------|-------------|---------|
| 2026-02-22 | Manual testing of dep tree, blocking, close guard, labels, status filtering, reparenting, concurrency, validation | Found 14 bugs, confirmed 23 protocol invariants. Wrote `discovery_test.go` (34 tests). |
| 2026-02-22 | Audit of all bugs for fix vs wontfix | 5-6 clear fix PRs, 2 need design discussion, 5-6 wontfix/by-design |
| 2026-02-22 | Code review of labels.go, schema.go, dependencies.go for BUG-5 and BUG-7 root cause | BUG-5 upgraded to INVESTIGATE (not clearly wontfix). BUG-7 downgraded to FILE ISSUE (intentionally coded upsert, needs product decision). BUG-4 upgraded to DOCS FIX (help text promises "blocked" as a status). |
| 2026-02-22 | Submitted 8 PRs: 2 clear fixes, 3 DECISION PRs, 3 metadata features | PR #1992 (BUG-2+3) merged same day. All PRs rebased onto latest main. |
| 2026-02-22 | BUG-5 explicitly deferred | No deterministic repro; needs deeper investigation of Dolt working-set commit race. Deferred pending repro. |
| 2026-02-22 | Deep audit session 2: 8 areas (SQL patterns, blocked semantics, concurrency, import/export, routing, field validation, GitHub issues, telemetry) | Found 9 new bugs (BUG-15–BUG-23). Mined 80+ GitHub issues. |
| 2026-02-22 | Wrote discovery tests: 9 Tier A deterministic, 3 Tier B stress, 1 Tier C compile-time | 13 new tests in discovery_test.go + compile_test.go. BUG-23 confirmed immediately. |

## Audit Summary

| Bug | Verdict | PR | Status |
|-----|---------|-----|--------|
| BUG-1 | INFRASTRUCTURE | — | `bd export` removed; test harness needs adaptation. Decision pending on `bd dump` or alternative. |
| BUG-2 | **FIX** | [#1992](https://github.com/steveyegge/beads/pull/1992) | **MERGED** |
| BUG-3 | **FIX** | [#1992](https://github.com/steveyegge/beads/pull/1992) | **MERGED** (bundled with BUG-2) |
| BUG-4 | NOT A BUG | — | `blocked` is a valid stored status. Help text is correct. Dropped. |
| BUG-5 | **DEFERRED** | — | No deterministic repro. Concurrent Dolt working-set commit race suspected. Deferred pending repro. |
| BUG-6 | WONTFIX | — | By-design for collaboration. Only affects test infrastructure. |
| BUG-7 | **DECISION** | [#1999](https://github.com/steveyegge/beads/pull/1999) | Open — `dep add` silently overwrites type. Fix: check-then-error. Test: `dep_type_overwrite_test.go`. |
| BUG-8 | **DECISION** | [#2001](https://github.com/steveyegge/beads/pull/2001) | Open — reparented child under both parents. Fix: exclude explicitly reparented from LIKE. Test: `reparent_test.go`. |
| BUG-9 | WONTFIX | — | Documented in help text already. |
| BUG-10 | **FIX** | [#1993](https://github.com/steveyegge/beads/pull/1993) | Open — exit non-zero on soft failures. Test: `exit_code_test.go`. |
| BUG-11 | **FIX** | [#1994](https://github.com/steveyegge/beads/pull/1994) | Open — status validation. Test: `input_validation_test.go`. |
| BUG-12 | **FIX** | [#1994](https://github.com/steveyegge/beads/pull/1994) | Open — empty title rejection (bundled with BUG-11+14). |
| BUG-13 | **DECISION** | [#2000](https://github.com/steveyegge/beads/pull/2000) | Open — reopen clears defer_until. Test: `reopen_defer_test.go`. |
| BUG-14 | **FIX** | [#1994](https://github.com/steveyegge/beads/pull/1994) | Open — empty label rejection (bundled with BUG-11+12). |

### PR Scoreboard

| PR | Bugs | Branch | Status |
|----|------|--------|--------|
| [#1992](https://github.com/steveyegge/beads/pull/1992) | BUG-2+3 | `fix/dep-tree-parent-id` | **MERGED** |
| [#1993](https://github.com/steveyegge/beads/pull/1993) | BUG-10 | `fix/exit-codes-close-claim` | Open (rebased) |
| [#1994](https://github.com/steveyegge/beads/pull/1994) | BUG-11+12+14 | `fix/input-validation-gaps` | Open (rebased) |
| [#1999](https://github.com/steveyegge/beads/pull/1999) | BUG-7 | `fix/dep-add-type-overwrite` | Open DECISION (rebased) |
| [#2000](https://github.com/steveyegge/beads/pull/2000) | BUG-13 | `fix/reopen-clears-defer` | Open DECISION (rebased) |
| [#2001](https://github.com/steveyegge/beads/pull/2001) | BUG-8 | `fix/reparent-dual-parent` | Open DECISION (rebased) |

### Remaining work

- **BUG-1** (infrastructure): Decide whether to restore `bd export`/`bd dump` or adapt harness to `bd list --json` + `bd show --json`. Needs maintainer input.
- **BUG-5** (deferred): Concurrent label race. No deterministic repro yet. Suspected Dolt working-set commit race in `execContext` BeginTx/Commit pattern. Deferred pending repro.

---

## CONFIRMED BUGS

### BUG-1: `bd export` command removed from main

**Severity: HIGH** — Breaks entire regression test suite
**Affected:** `tests/regression/` — all 85 tests rely on `compareExports()` → `bd export`

The `bd export` command was removed during the JSONL→Dolt-native refactor
(commit 1e1568fa). The regression test framework calls `w.export()` which
runs `bd export` — this fails with "unknown command" on the candidate binary.

**Impact:** No differential regression testing is possible until either:
- `bd export` is restored (even as a read-only dump)
- The test harness is rewritten to use `bd show --json` / `bd list --json --all -n 0`
- A new `bd dump` or `bd export-jsonl` command is added

**Fix proposal:** Add a `bd dump` command that produces JSONL-per-issue output
(same schema as old `bd export`) for debugging and testing. Alternatively,
adapt the regression harness to use `bd list --all -n 0 --json` + `bd show <id> --json`
for each issue, but this requires restructuring the normalization pipeline.

---

### BUG-2: `dep tree` shows no children — ParentID never set (GH#1954)

**Severity: HIGH** — Core feature completely broken
**File:** `internal/storage/dolt/dependencies.go:646-649`
**Root cause:** `buildDependencyTree()` creates `TreeNode` without setting `ParentID`:

```go
node := &types.TreeNode{
    Issue: *issue,
    Depth: depth,  // ← Depth is set correctly
    // ParentID is NEVER set ← BUG
}
```

The `renderTree()` function at `cmd/bd/dep.go:721-729` builds a children map
keyed by `ParentID`. Since `ParentID` is always empty, all children go into
`children[""]` instead of `children[rootID]`. Root's children lookup returns empty.

**Fix:** Pass parent ID into recursive `buildDependencyTree` and set `node.ParentID`:

```go
func (s *DoltStore) buildDependencyTree(ctx context.Context, issueID string,
    depth, maxDepth int, reverse bool, visited map[string]bool,
    parentID string) ([]*types.TreeNode, error) {
    // ...
    node := &types.TreeNode{
        Issue:    *issue,
        Depth:    depth,
        ParentID: parentID,  // ← FIX
    }
    // ...
    for _, childID := range childIDs {
        children, err := s.buildDependencyTree(ctx, childID, depth+1,
            maxDepth, reverse, visited, issueID)  // ← pass issueID as parent
```

---

### BUG-3: `dep tree` shows `[READY]` for blocked root issue

**Severity: MEDIUM**
**File:** `cmd/bd/dep.go:835`

```go
if node.Status == types.StatusOpen && node.Depth == 0 {
    line += " " + ui.PassStyle.Bold(true).Render("[READY]")
}
```

The ready check only looks at `status == open && depth == 0`. It doesn't check
whether the issue has open blocking dependencies. A blocked issue at depth 0
(the root of a "down" tree) shows `[READY]` when it should show `[BLOCKED]`.

**Fix:** Check for open blocking dependencies before showing `[READY]`. Either
query the store or compute from the tree data.

---

### BUG-4: `list --status blocked` and `count --status blocked` return empty

**Severity: MEDIUM** — Documented status value doesn't work
**Affects:** `bd list --status blocked`, `bd count --status blocked`, `bd query "status=blocked"`

The help text for `list` says: `--status string  Filter by status (open, in_progress, blocked, deferred, closed)`

But "blocked" is a computed status derived from dependency relationships, never
stored in the `issues.status` column (which stays "open"). So:
- `bd blocked` → 4 issues ✓
- `bd list --status blocked` → 0 issues ✗
- `bd count --status blocked` → 0 ✗

**Fix options:**
1. Materialize blocked status: When a blocking dep is added, update status to "blocked"
2. Compute on query: In the list/count SQL, join with dependencies to detect blocked
3. Remove "blocked" from the documented status values and point users to `bd blocked`

---

### BUG-5: Concurrent label operations produce race conditions

**Severity: MEDIUM** — Data loss under concurrency
**Reproduction:**

```bash
# Parallel adds — expect 5 labels, get 0
for i in 1 2 3 4 5; do
  bd label add <id> "stress-$i" &
done
wait
bd show <id> --json  # labels: []
```

Sequential label adds work perfectly (5/5). Parallel adds produce 0 labels
visible immediately. After subsequent operations, some labels eventually appear.

**Root cause:** Likely a lost-update race in the Dolt server. Each concurrent
`label add` reads the current label set, adds its label, writes back. If two
writers read the same state, the last writer wins and the other's label is lost.

**Fix:** Use row-level INSERT into a labels junction table instead of
read-modify-write on a labels array/column. Or use SELECT FOR UPDATE / SERIALIZABLE
transactions.

---

### BUG-6: Workspace data isolation with shared Dolt server

**Severity: LOW for end users, HIGH for test infrastructure**

All `bd init --prefix test` workspaces on the same Dolt server (127.0.0.1:3307)
share the same `beads_test` database. Issues created in one workspace are visible
from any other workspace with the same prefix.

This is by design for collaborative use, but it breaks the regression test
harness which creates isolated workspaces with `newWorkspace(t, bdPath)`. Each
test's workspace shares the database, causing cross-test contamination.

**Fix for tests:** Use unique prefixes per test (e.g., `test-<random>`) or
create a fresh Dolt database per test workspace.

---

### BUG-7: `dep add` silently overwrites when changing dep type on same pair

**Severity: HIGH** — Silent data loss of blocking relationships
**Reproduction:**

```bash
bd dep add A B --type blocks    # ✓ Added dependency
bd dep add A B --type caused-by # ✓ Added dependency  (SILENTLY REPLACES blocks)
# DB now only has caused-by — blocks relationship is LOST
# A is no longer blocked!
```

The `dependencies` table has a unique constraint on `(issue_id, depends_on_id)`
without including `type`. Adding a second dep type on the same pair does an
upsert, replacing the existing type. Both operations report success.

**Impact:** A user who adds `caused-by` to an already-blocked pair silently
removes the blocking relationship. The issue becomes unblocked without warning.

**Fix:** Either:
1. Make the unique key `(issue_id, depends_on_id, type)` to allow multiple dep types
2. Reject the second `dep add` with an error: "dependency already exists with type X"
3. Warn the user: "changing dep type from X to Y"

---

### BUG-8: Reparented child appears under BOTH old and new parent

**Severity: MEDIUM** — Confusing behavior after reparenting
**File:** `internal/storage/dolt/queries.go:211`
**Root cause:** Parent filter uses `OR id LIKE CONCAT(?, '.%')` in addition to
dependency lookup. After `bd create --title X --parent P1` creates `P1.1`,
reparenting with `bd update P1.1 --parent P2` correctly updates the
parent-child dep to P2, but the ID `P1.1` still matches `P1.%` via LIKE.

```sql
(id IN (SELECT issue_id FROM dependencies WHERE type = 'parent-child' AND depends_on_id = ?)
 OR id LIKE CONCAT(?, '.%'))
```

**Impact:** `bd children P1` shows `P1.1` even after reparenting to P2.
`bd children P2` also correctly shows it. The child appears under BOTH parents.

**Fix options:**
1. After reparent, rename the issue ID to match new parent (e.g., `P1.1` → `P2.1`)
2. Remove the LIKE clause from parent filtering (rely solely on dependency table)
3. Add EXCEPT clause: `AND id NOT IN (SELECT issue_id FROM dependencies WHERE type = 'parent-child' AND depends_on_id != ?)`

---

### BUG-9: `list --ready` includes blocked issues (documented but confusing)

**Severity: LOW** (documented in help text)
**File:** `bd list --ready` help says "Note: 'bd list --ready' is NOT equivalent"

`bd list --ready -n 0` returns 34 issues including blocked ones.
`bd ready -n 0` returns 29 truly ready issues (excludes blocked).

The discrepancy of 5 issues = exactly the issues with open `blocks` dependencies.
The help text documents this, but the `--ready` flag name is misleading.

---

### BUG-10: Commands exit 0 on soft failures (close guard, claim, etc.)

**Severity: MEDIUM** — Breaks scripting and automation
**Affects:** `bd close` (close guard), `bd update --claim` (already claimed), likely others
**Files:** `cmd/bd/close.go:117`, `cmd/bd/update.go:278`

When close guard prevents closing a blocked issue, the command prints a message
to stderr ("cannot close X: blocked by open issues") but exits with code 0.
Similarly, `update --claim` on an already-claimed issue prints "already claimed"
to stderr but exits 0.

The pattern is: `fmt.Fprintf(os.Stderr, ...) + continue` in a loop, with no
tracking of whether any operations actually succeeded. When the loop finishes,
the command exits 0 regardless.

**Impact:** Scripts and CI/CD pipelines cannot detect these failures via exit code.
They must parse stderr text instead, which is fragile.

**Fix:** Track `errorCount` and call `os.Exit(1)` if `errorCount > 0` and
`closedCount == 0` at end of the command.

---

### BUG-11: `bd update --status` accepts arbitrary values

**Severity: MEDIUM** — Data integrity issue
**File:** `cmd/bd/update.go`

`bd update X --status "bogus"` succeeds and stores "bogus" as the status.
Valid statuses should be: open, in_progress, closed, deferred.
The `--type` flag correctly validates against a whitelist, but `--status` does not.

**Impact:** Invalid statuses are stored in the DB. Issues with invalid status
won't appear in any filtered list (they're not open, not closed, not deferred).

**Fix:** Add status validation in update command, same pattern as type validation.

---

### BUG-12: `bd update --title ""` accepts empty title

**Severity: LOW** — Data quality issue
**File:** `cmd/bd/update.go`

`bd create --title ""` correctly fails with "title required".
`bd update X --title ""` succeeds and stores an empty title.
Validation is inconsistent between create and update.

**Fix:** Add empty-title check in update command.

---

### BUG-13: Reopen of closed+deferred issue creates limbo state

**Severity: MEDIUM** — Issue becomes invisible
**Reproduction:**

```bash
bd defer X --until 2099-12-31   # status=deferred
bd close X                      # status=closed, defer_until preserved
bd reopen X                     # status=open, defer_until STILL SET
```

After reopening, the issue has status "open" but defer_until is still set.
- Not in `bd ready` (excluded by defer_until check) ✓
- Not in `bd list --status deferred` (status is "open", not "deferred") ✗
- Appears in `bd list --status open` but won't show in ready ✗

The issue is effectively invisible to normal workflows.

**Fix options:**
1. `reopen` should clear defer_until when setting status to "open"
2. `reopen` should restore "deferred" status if defer_until is still in the future
3. `close` should clear defer_until when closing a deferred issue

---

### BUG-14: `bd label add` accepts empty string label

**Severity: LOW** — Data quality issue

`bd label add X ""` succeeds and stores an empty string as a label.
This creates invisible/confusing entries in the label list.

**Fix:** Validate label is non-empty before inserting.

---

## MINOR ISSUES / OBSERVATIONS

### OBS-1: `bd supersede` and `bd duplicate` don't set close_reason

When `bd supersede X --with Y` or `bd duplicate X --of Y` closes issue X,
the `close_reason` field is empty. The relationship is tracked via a
`supersedes`/`duplicate-of` dependency, but there's no close_reason like
"superseded" or "duplicate" set on the issue. Users querying closed issues
by reason would miss these.

### OBS-2: `count --by-status` doesn't show "blocked" count

`count --by-status` shows only "open" and "closed" (and "in_progress",
"deferred" when applicable). Issues with open blocking dependencies show as
"open", not "blocked". This is consistent with BUG-4 but may confuse users.

### OBS-3: `bd sql` allows arbitrary writes (no safety check)

`bd sql "UPDATE issues SET title = 'X'"` succeeds without warning. Only
`--readonly` flag prevents it (but blocks ALL sql, even reads). There's no
write-specific safety prompt or `--force` requirement for mutating SQL.

### OBS-4: `bd label rm` is not a recognized alias for `bd label remove`

Running `bd label rm <id> <label>` shows the `bd label` help text instead of
an error message. Users might expect `rm` as a common alias. The `bd delete`
command uses `--force` not `--yes`.

### OBS-3: `bd label add` syntax is `[issue-id...] [label]` (last arg = label)

The syntax treats all args except the last as issue IDs and the last as the
label. This means you can label multiple issues at once, but only one label
at a time. This is correct but potentially confusing — `bd label add id lab1 lab2`
adds label "lab2" to issues "id" and "lab1".

---

## PROTOCOL TEST IDEAS

These are candidates for porting to the protocol test suite (PR #1910) once it
lands. Tests are classified as:

- **DATA INTEGRITY**: invariants about data correctness (cycle prevention,
  dep cleanup, data preservation). These are hard protocol guarantees.
- **POLICY/UX**: invariants about behavior that could reasonably change
  (epic auto-close, claim semantics, message text). Useful as regression
  tests but not immutable protocol.
- **MESSAGE CONTRACT**: tests that assert specific CLI output strings.
  Brittle — useful for regression detection but will break if wording changes.

### PT-1: Close guard respects dep types — DATA INTEGRITY

```
GIVEN issue A with caused-by dep on open issue B
WHEN close A
THEN close succeeds (caused-by is non-blocking)

GIVEN issue C with blocks dep on open issue D
WHEN close C
THEN close is rejected with suggestion to use --force
```

Already tested manually — works correctly. Good protocol invariant to formalize.

### PT-2: Epic lifecycle — children don't auto-close parent — POLICY/UX

```
GIVEN epic E with children C1, C2
WHEN close C1, close C2 (all children closed)
THEN E remains open
AND E appears in bd ready output
WHEN close E
THEN E is closed
```

Works correctly. Note: auto-close-on-all-children-done is a reasonable
alternative policy. This test documents current behavior, not a hard invariant.

### PT-3: Delete cleans up dependency links — DATA INTEGRITY

```
GIVEN A depends on B (blocks)
WHEN delete B --force
THEN A has no dependencies
AND A appears in bd ready output
```

Works correctly. CASCADE DELETE on FK ensures this at the schema level.

### PT-4: Reopen preserves dependencies — DATA INTEGRITY

```
GIVEN A depends on B (caused-by)
WHEN close A, then reopen A
THEN A still has dep on B
```

Works correctly.

### PT-5: `dep tree` shows full tree — DATA INTEGRITY

```
GIVEN diamond dependency: A→B, A→C, B→D, C→D
WHEN dep tree A
THEN output shows all 4 nodes at correct depths
AND D appears twice (or once with "shown above" marker)
```

Fixed by PR #1992 (BUG-2 fix merged).

### PT-6: Ready semantics exclude blocked issues — DATA INTEGRITY

```
GIVEN A→B (blocks), A→C (blocks), D (no deps)
WHEN bd ready
THEN A is NOT in ready list (blocked by B and C)
AND B is in ready list (no blockers)
AND C is in ready list (no blockers)
AND D is in ready list
```

Works correctly.

### PT-7: Deferred issues excluded from ready — DATA INTEGRITY

```
GIVEN A deferred until 2099-12-31
WHEN bd ready
THEN A is NOT in ready list
WHEN undefer A
THEN A IS in ready list
```

Works correctly.

### PT-8: Concurrent create is safe — DATA INTEGRITY

```
WHEN 10 parallel bd create commands
THEN all 10 issues exist with unique IDs
AND count matches expected total
```

Works correctly.

### PT-9: Concurrent label add is NOT safe (documents BUG-5) — DATA INTEGRITY

```
WHEN 5 parallel bd label add <id> "label-N"
THEN only 0-4 labels survive (lost update race)
```

This would be a regression test to verify when the fix lands.

### PT-10: `list --status blocked` should match `blocked` output — POLICY/UX

```
GIVEN A→B (blocks), both open
THEN bd list --status blocked should include A
AND bd blocked should include A
AND counts should match
```

Currently fails — documents BUG-4.

### PT-11: Status transitions round-trip — DATA INTEGRITY

```
open → in_progress → open → closed → open (via update)
open → deferred → open (via defer/undefer)
All transitions preserve issue data (deps, labels, comments)
```

Works correctly.

### PT-12: Notes append vs overwrite — DATA INTEGRITY

```
GIVEN issue with notes "Original"
WHEN update --notes "Replaced"
THEN notes = "Replaced" (overwrite)
WHEN update --append-notes "Extra"
THEN notes = "Replaced\nExtra" (append with newline)
```

Works correctly.

### PT-13: Special characters in fields — DATA INTEGRITY

```
GIVEN bd create --title 'Test "quotes" & <brackets>'
THEN show --json correctly escapes and preserves the title
```

Works correctly.

### PT-14: Export command existence (BLOCKED by BUG-1) — POLICY/UX

```
WHEN bd export
THEN command exists and produces JSONL output
```

Currently fails — export removed from main.

### PT-15: Supersede creates dependency and closes issue — DATA INTEGRITY

```
GIVEN issue A and B
WHEN bd supersede A --with B
THEN A is closed
AND A has supersedes dependency on B
```

Works correctly (though close_reason is empty — see OBS-1).

### PT-16: Duplicate marks issue as closed with dependency — DATA INTEGRITY

```
GIVEN issue A and B
WHEN bd duplicate B --of A
THEN B is closed
AND B has duplicate-of dependency on A
```

Works correctly (though close_reason is empty — see OBS-1).

### PT-17: Type change round-trip — DATA INTEGRITY

```
GIVEN task T
WHEN update T --type bug, then update T --type epic
THEN type=epic
```

Works correctly.

### PT-18: Transitive blocking chain — DATA INTEGRITY

```
GIVEN A→B→C→D (all blocks)
THEN only D is ready, A/B/C are blocked
WHEN close D: only C becomes ready
WHEN close C: only B becomes ready
WHEN close B: only A becomes ready
```

Works correctly. Good chain-invariant test.

### PT-19: Circular dependency prevention — DATA INTEGRITY

```
GIVEN A→B→C (blocks)
WHEN dep add C→A (blocks)
THEN error "would create a cycle"
AND the dependency is NOT added
AND dep cycles shows no cycles
```

Works correctly. Critical invariant.

### PT-20: Close --force overrides close guard — POLICY/UX

```
GIVEN A→B (blocks), B is open
WHEN close A (no force)
THEN rejected
WHEN close A --force
THEN A is closed
```

Works correctly.

### PT-21: Claim semantics (atomic) — POLICY/UX + MESSAGE CONTRACT

```
WHEN update X --claim
THEN X.status = in_progress, X.assignee = current user
WHEN update X --claim (again)
THEN error "already claimed"
```

Works correctly.

### PT-22: Create with --parent creates dotted ID — DATA INTEGRITY

```
WHEN create --title "Child" --parent P
THEN child ID is P.N (e.g., P.1)
AND children P shows the child
AND child has parent-child dep on P
```

Works correctly.

### PT-23: Create with --deps creates blocks dependency — DATA INTEGRITY

```
WHEN create --title "X" --deps B
THEN X has blocks dep on B
AND X is in blocked list
```

Works correctly.

### PT-24: count --by-status, --by-type, --by-priority grouping — DATA INTEGRITY

```
GIVEN mixed issues with various statuses, types, priorities
THEN count --by-status groups correctly
AND count --by-type groups correctly
AND count --by-priority groups correctly
AND totals match count without filter
```

Works correctly.

### PT-25: Due date and defer round-trip — DATA INTEGRITY

```
GIVEN issue I
WHEN update I --due "2099-06-15"
THEN show --json has due_at with 2099-06-15 date
WHEN defer I --until 2099-12-31
THEN status=deferred, defer_until has 2099-12-31 date
```

Works correctly.

### PT-26: dep rm unblocks issue — DATA INTEGRITY

```
GIVEN A→B (blocks)
WHEN dep rm A B
THEN A is in ready list
AND A is NOT in blocked list
```

Works correctly.

### PT-27: Self-dependency prevention — DATA INTEGRITY

```
WHEN dep add A A --type blocks
THEN error "would create a cycle"
```

Works correctly (caught by cycle detection).

### PT-28: Create with --deps creates blocking dep — DATA INTEGRITY

```
GIVEN issue B
WHEN create --title "X" --deps B
THEN X is blocked by B
AND B is in ready list
AND X is NOT in ready list
```

Works correctly.

### PT-29: Label add/remove round-trip — DATA INTEGRITY

```
GIVEN issue I with no labels
WHEN label add I "bug-fix"
WHEN label add I "urgent"
THEN I has 2 labels
WHEN label remove I "bug-fix"
THEN I has 1 label ("urgent")
```

Works correctly.

### PT-30: Comments preserved through close/reopen — DATA INTEGRITY

```
GIVEN issue I with 2 comments
WHEN close I, reopen I
THEN I still has 2 comments
```

Works correctly.

### PT-31: Due date round-trip — DATA INTEGRITY

```
GIVEN issue I
WHEN update I --due "2099-06-15"
THEN show --json has due_at containing "2099-06-15"
```

Works correctly.

### PT-32: Status transition round-trip — DATA INTEGRITY

```
open → in_progress → open → closed → open (reopen)
All transitions work, data preserved at each step
```

Works correctly.

---

## PRIOR ART: Dolt migration bugs already fixed

These were found and fixed before this discovery session. Documented here so
future investigators don't re-discover them. All are merged to main.

| PR | What it fixed | Why it matters for regression testing |
|---|---|---|
| #1969 (nmelo) | `execContext` didn't commit writes under `--no-auto-commit` | Root cause of many "data disappears" bugs. `execContext` now wraps each statement in `BeginTx/Commit`. Directly relevant to BUG-5 investigation — concurrent `Commit()` to Dolt working set may still race. |
| #1966 (turian) | Labels, comments, deps lost during batch import | `ImportIssues` didn't persist associated data. |
| #1967 (turian) | `scanIssueIDs` lost ORDER BY | `ready` and `list` returned results in wrong order. |
| #1968 (turian) | `UpdateIssue` didn't normalize metadata/waiters | Nullable JSON fields stored as `null` instead of `{}`, breaking downstream code. |
| #1914 (turian) | Column drift in issue scan projection | Centralized column list prevents SELECT * from silently gaining/losing columns after schema migration. |
| #1816 (sjsyrek) | Silent empty results on Dolt lock errors | Dolt lock contention returned empty results instead of errors. |
| #1797 (sjsyrek) | Locking, migration, compaction stability | Major stabilization pass on Dolt backend. |
| #1948 (Xexr) | Parent-child deps mixed with blocking deps in `bd list` | `list --parent` was showing blocking deps as children. |
| #1909 (zjrosen) | `AddDependency`/`RemoveDependency` not in explicit transactions | Writes could be lost under `--no-auto-commit`. Directly relevant to BUG-7 — the upsert at `dependencies.go:78` is now inside an explicit tx. |

### Key Dolt constraints learned from prior fixes

- **`execContext` wraps in BeginTx/Commit**: Every write is its own mini-transaction (store.go:214). This means two writes to the same table from different goroutines each commit independently to the Dolt working set, which can race.
- **Close rows before nested queries**: Dolt with `MaxOpenConns=1` deadlocks if you open a second query while iterating the first. This is why `GetIssuesByLabel` collects IDs first, closes rows, then fetches issues.
- **Schema version check skips init**: PR #1765 added version check so `ensureSchema` doesn't re-run DDL on every command. If you add a new table/column, bump the schema version.

---

## TEST INFRASTRUCTURE NOTES

### Regression harness needs adaptation for Dolt-only main

The current regression test harness (`regression_test.go`) is designed around:
1. `bd export` producing JSONL
2. SQLite-based baseline binary (v0.49.6) that doesn't need a server
3. Isolated workspaces (each test gets a fresh `.beads/` dir)

On current main:
- `bd export` doesn't exist (BUG-1)
- Candidate binary requires a running Dolt server
- All workspaces with same prefix share the same Dolt database (BUG-6)

To fix the harness:
1. Replace `w.export()` with `w.run("list", "--all", "-n", "0", "--json")`
   combined with `w.run("show", id, "--json")` per issue for full data
2. The baseline binary still works with SQLite (no server needed)
3. Use unique prefixes per test: `test-<testname>-<random>`
4. Or spin up a separate Dolt server per test on a random port

---

## DEEP AUDIT — Session 2 (2026-02-22)

Systematic audit across 8 discovery areas: GitHub issue mining, SQL pattern
analysis, mode parity, blocked semantics, concurrency, field validation,
cross-rig routing, and import/export survivability.

### Session Log Update

| Date | What we did | Outcome |
|------|-------------|---------|
| 2026-02-22 | Mined 80+ GitHub issues for Dolt pain points | 10 high-value issues mapped to discovery areas |
| 2026-02-22 | Audited ON DUPLICATE KEY UPDATE (13 instances), INSERT IGNORE (8), LIKE CONCAT (6) | Found 4 new potential bugs, 3 by-design |
| 2026-02-22 | Audited blocked semantics across 5 commands | Confirmed BUG-4 at code level; found inconsistency in `waits-for` handling |
| 2026-02-22 | Audited concurrency in execContext, labels, UpdateIssue | Found TOCTOU race in UpdateIssue; confirmed label atomicity gap |
| 2026-02-22 | Audited import/export/migrate_dolt paths | Found 2 missing columns in migrate_dolt; silent dep skipping |
| 2026-02-22 | Audited cross-rig routing code | Routing is sound but error paths return partial results |
| 2026-02-22 | Found InstrumentedStorage build break | RunInTransaction signature mismatch after commitMsg addition |

---

### AREA 1: GitHub Issues — Dolt Migration Pain Points

Mined all 80+ GitHub issues. The following open issues map directly to
discovery areas and represent real user pain:

| # | Issue | Symptom | Area | Actionable? |
|---|-------|---------|------|-------------|
| 1 | [#1984](https://github.com/steveyegge/beads/issues/1984) | `bd create-form` blocks DB — lock held during editor | Lock contention | FIX — release lock before $EDITOR |
| 2 | [#1881](https://github.com/steveyegge/beads/issues/1881) | Unstaged modified files lost after commit | Hooks / data loss | INVESTIGATED — no code path found |
| 3 | [#1858](https://github.com/steveyegge/beads/issues/1858) | `bd list` shows resolved blockers as still blocking | Blocked semantics | LIKELY FIXED by fedf1daa (#1884) |
| 4 | [#1945](https://github.com/steveyegge/beads/issues/1945) | `bd repo sync` fails: duplicate key + no cross-prefix hydration | Import/export | FIX — re-import without existence check |
| 5 | [#1962](https://github.com/steveyegge/beads/issues/1962) | `bd create --ephemeral` generates empty ID, UNIQUE failure | ID generation | FIX — generate real ephemeral ID |
| 6 | [#1977](https://github.com/steveyegge/beads/issues/1977) | 2s CPU burn on every invocation (wazero WASM) | Performance | FIX — lazy-load SQLite embed |
| 7 | [#2007](https://github.com/steveyegge/beads/issues/2007) | `bd prime` references removed `--status` flag | Stale docs | FIX — trivial |
| 8 | [#1921](https://github.com/steveyegge/beads/issues/1921) | BD_BRANCH breaks beads and gastown | Routing | INVESTIGATE |
| 9 | [#1853](https://github.com/steveyegge/beads/issues/1853) | `bd update --type` fails with BEADS_DIR + custom types | Field validation | FIX — load config from BEADS_DIR |
| 10 | [#1524](https://github.com/steveyegge/beads/issues/1524) | Close guard may diverge from blocked-cache for non-`blocks` types | Blocked semantics | DECISION — policy question |

**Closed issues with regression relevance:**

| Issue | What it was | Why it matters |
|-------|-------------|----------------|
| [#1773](https://github.com/steveyegge/beads/issues/1773) | `bd list` returns empty on Dolt lock contention | Fixed by #1816 but pattern may recur |
| [#1833](https://github.com/steveyegge/beads/issues/1833) | `no-db: true` ignored in v0.50+ | Config key silently became no-op |
| [#1609](https://github.com/steveyegge/beads/issues/1609) | Label writes silently dropped with multiple daemons | Concurrency root cause |
| [#1669](https://github.com/steveyegge/beads/issues/1669) | `bd migrate --to-dolt` hardcodes DB name, loses 3455 issues | Migration data loss |
| [#1934](https://github.com/steveyegge/beads/issues/1934) | `bd import` fails with duplicate primary key | Import idempotency |

---

### AREA 2: Dangerous SQL Patterns

#### 2a. ON DUPLICATE KEY UPDATE — 13 instances

| File:Line | Table | Risk | Verdict |
|-----------|-------|------|---------|
| `dependencies.go:78` | dependencies | **HIGH** — silently overwrites dep type+metadata | **BUG-7** (already tracked) |
| `wisps.go:836` | wisp_dependencies | **HIGH** — same pattern as BUG-7 for wisps | **BUG-15** (NEW) |
| `transaction.go:310` | dependencies (tx) | **MEDIUM** — same upsert but inside explicit tx | **BUG-7 variant** |
| `queries.go:1136` | child_counters | **MEDIUM** — concurrent child creation race | **BUG-16** (NEW) — see Area 5 |
| `issues.go:242` | labels (import) | LOW — no-op update (`label = label`), safe | BY-DESIGN |
| `issues.go:280` | dependencies (import) | LOW — no-op update (`type = type`), safe | BY-DESIGN |
| `store.go:696` | config | LOW — schema version upsert, always same value | BY-DESIGN |
| `credentials.go:132` | federation_peers | LOW — intentional config upsert | BY-DESIGN |
| `config.go:16` | config | LOW — config upsert, intentional | BY-DESIGN |
| `config.go:88` | metadata | LOW — metadata upsert, intentional | BY-DESIGN |
| `transaction.go:420` | config (tx) | LOW — config upsert in tx | BY-DESIGN |
| `transaction.go:439` | metadata (tx) | LOW — metadata upsert in tx | BY-DESIGN |
| `git_remote_test.go:490` | config (test) | N/A — test fixture | N/A |

**New bugs found:**

**BUG-15: Wisp dependency type silently overwritten** (`wisps.go:836`)

Same pattern as BUG-7 but in the wisp (ephemeral) code path. If a wisp
dependency is re-added with a different type, the old type is silently
overwritten. `created_at`, `created_by`, and `thread_id` are preserved (not
updated), but `type` and `metadata` are replaced without warning.

**BUG-16: Child counter race** (`queries.go:1136`)

`GetNextChildID` reads `last_child`, increments, then upserts. The read + write
are inside a single transaction, but Dolt's default isolation level may allow
two concurrent transactions to read the same `last_child` value. Both would
compute the same `nextChild`, and the upsert means the second writer silently
wins. Result: **duplicate child IDs** (e.g., two issues both named `epic.3`).

```
Process A: SELECT last_child → 2, nextChild = 3
Process B: SELECT last_child → 2, nextChild = 3  (A hasn't committed yet)
Process A: UPSERT last_child = 3 → COMMIT
Process B: UPSERT last_child = 3 → COMMIT (overwrites same value)
Both create issue "epic.3" — duplicate ID
```

**Fix:** Use `SELECT ... FOR UPDATE` or add a UNIQUE constraint on child IDs
to prevent the duplicate insert.

#### 2b. INSERT IGNORE — 8 instances

| File:Line | Table | Risk |
|-----------|-------|------|
| `labels.go:17` | labels | LOW — idempotent label add, correct |
| `wisps.go:1038` | wisp_labels | LOW — same pattern, correct |
| `transaction.go:374` | labels (tx) | LOW — same pattern in tx |
| `ephemeral_routing.go:84` | labels (promote) | LOW — promotion copy, correct |
| `ephemeral_routing.go:101` | dependencies (promote) | **MEDIUM** — silently drops deps that already exist during wisp promotion |
| `ephemeral_routing.go:114` | events (promote) | LOW — best-effort event copy |
| `ephemeral_routing.go:121` | comments (promote) | LOW — best-effort comment copy |
| `schema.go:245` | config (init) | LOW — default config values |

**BUG-17: Wisp promotion silently drops conflicting deps** (`ephemeral_routing.go:101`)

When a wisp (ephemeral issue) is promoted to a persistent issue, its
dependencies are copied via `INSERT IGNORE`. If the persistent issues table
already has a dependency with the same `(issue_id, depends_on_id)` pair, the
wisp's dependency is silently dropped — including its type, metadata, and
thread_id. No error, no warning.

**Impact:** If an agent creates a wisp with a `blocks` dependency, and a
concurrent process already added a `caused-by` dependency with the same pair,
the promotion silently drops the `blocks` relationship. The issue becomes
unblocked without warning.

#### 2c. LIKE CONCAT — 6 instances

| File:Line | Usage | Risk |
|-----------|-------|------|
| `queries.go:212` | Parent filter (children) | **MEDIUM** — BUG-8 (already tracked) |
| `wisps.go:544` | Same pattern for wisps | **MEDIUM** — BUG-8 variant |
| `transaction.go:172` | Same pattern in tx | **MEDIUM** — BUG-8 variant |
| `adaptive_length.go:105` | ID collision counting | LOW — read-only, correct |
| `rename.go:123` | Bulk rename deps | LOW — intentional prefix rename |
| `rename.go:133` | Bulk rename deps (other side) | LOW — intentional prefix rename |

The `LIKE CONCAT(?, '.%')` pattern in parent filtering (BUG-8) appears in
**three places** — `queries.go:212`, `wisps.go:544`, `transaction.go:172`.
Any fix needs to cover all three.

---

### AREA 3: Direct vs Daemon Mode Parity

Since daemon mode was removed (replaced by direct Dolt access), this area now
maps to **read-only store vs read-write store** parity.

The `withStorage` function in `list.go:30` opens a read-only connection when
`dbPath` is set but `store` is nil:

```go
roStore, err := dolt.New(ctx, &dolt.Config{Path: dbPath, ReadOnly: true})
```

**Observations:**

1. **Lock contention (#1984)**: `bd create-form` holds a write lock during
   `$EDITOR`, blocking all other `bd` commands. The fix should acquire the
   lock only after the editor exits.

2. **BEADS_DIR override**: When `BEADS_DIR` is set, routing is skipped
   (`beadsDirOverride()` returns true). But custom type validation (#1853)
   loads types from the working directory's config, not from `BEADS_DIR`.
   This means `bd update --type=custom` fails when `BEADS_DIR` points to a
   different directory.

3. **No mode-specific code paths remain**: After embedded Dolt removal, all
   operations go through the same `DoltStore`. The risk of mode-specific bugs
   is low. The main risk vector is now **concurrent access** (multiple `bd`
   processes hitting the same Dolt server).

---

### AREA 4: Blocked Semantics Drift

**Core finding confirmed:** "blocked" is both a stored status value
(`types.StatusBlocked = "blocked"`) AND a computed property (derived from
`blocks` dependencies). These two concepts are conflated in the codebase.

**Where `StatusBlocked` is used as a stored status:**
- `types.go:391` — declared as a valid status value
- `types.go:401` — passes `IsValidStatus` check
- `jira/fieldmapper.go:56` — mapped from Jira statuses
- `linear/mapping.go:312` — mapped from Linear statuses
- Tests create issues with `Status: types.StatusBlocked` directly

**Where blockedness is computed:**
- `computeBlockedIDs()` in `queries.go:814` — computes from `dependencies`
  table, checks `blocks` + `waits-for` deps, returns issue IDs
- `GetBlockedIssues()` in `queries.go:501` — similar but only checks `blocks`
  deps (NOT `waits-for`!)
- `GetReadyWork()` in `queries.go:343` — excludes computed-blocked issues

**BUG-18: `GetBlockedIssues` and `computeBlockedIDs` disagree on `waits-for`** (NEW)

`computeBlockedIDs()` (used by `GetReadyWork`) considers both `blocks` AND
`waits-for` dependencies. `GetBlockedIssues()` (used by `bd blocked`) only
considers `blocks` dependencies. An issue blocked by a `waits-for` gate will:
- NOT appear in `bd ready` (correctly excluded by `computeBlockedIDs`)
- NOT appear in `bd blocked` (incorrectly excluded — `GetBlockedIssues` doesn't check `waits-for`)
- Appear in `bd list --status open` (status column is "open")

The issue is invisible to both `ready` and `blocked` commands.

**Fix:** `GetBlockedIssues` should include `waits-for` deps the same way
`computeBlockedIDs` does.

**BUG-4 code-level confirmation:**

`SearchIssues` at `queries.go:57-59` does a simple column match:
```go
if filter.Status != nil {
    whereClauses = append(whereClauses, "status = ?")
    args = append(args, *filter.Status)
}
```

`bd list --status blocked` passes `"blocked"` as the filter value. This only
matches issues where `status` was explicitly set to `"blocked"` (e.g., via
Jira/Linear sync), not issues computed-blocked via dependency graph.

`bd count --status blocked` uses the same path, so counts are also wrong.

---

### AREA 5: Concurrency / Lost Updates

#### 5a. `execContext` isolation

Every call to `execContext` (store.go:281) wraps a single statement in
`BeginTx/Commit`. Multi-statement operations that call `execContext` multiple
times are **NOT atomic**:

- `AddLabel` (labels.go:12): label insert + event insert = 2 transactions
- `RemoveLabel` (labels.go:34): label delete + event insert = 2 transactions
- `SetConfig` + `SetMetadata` in sequence = 2 transactions
- Any sequence of `execContext` calls can interleave with concurrent processes

**Impact:** If `AddLabel` succeeds on the label insert but fails on the event
insert, the label exists without an audit trail. Not data loss, but
inconsistency.

#### 5b. TOCTOU race in `UpdateIssue` (BUG-19, NEW)

`UpdateIssue` (issues.go:363) reads the old issue at line 369 **before**
beginning the transaction at line 410:

```go
func (s *DoltStore) UpdateIssue(...) error {
    oldIssue, err := s.GetIssue(ctx, id)  // READ outside tx
    // ... build update query ...
    tx, err := s.db.BeginTx(ctx, nil)      // TX starts here
    tx.ExecContext(ctx, query, args...)     // WRITE inside tx
    recordEvent(ctx, tx, ...)               // uses oldIssue for diff
    tx.Commit()
}
```

Between the read and the tx, another process can modify the issue. The event
recorded will show stale "old" values. More critically, `manageClosedAt` at
line 406 uses `oldIssue` to decide whether to set `closed_at` — a stale read
could result in incorrect `closed_at` management.

**Contrast with `ClaimIssue`** (issues.go:434) which correctly uses a
conditional `UPDATE ... WHERE assignee = '' OR assignee IS NULL` inside the
transaction. `UpdateIssue` should follow the same pattern.

#### 5c. Child counter race (BUG-16, see Area 2)

Already documented above. `GetNextChildID` is vulnerable to concurrent
duplicate child IDs.

#### 5d. Label add is safe against duplicates but not atomic

`INSERT IGNORE INTO labels` prevents duplicate labels, so concurrent
`bd label add` with the same label is idempotent. But concurrent adds of
*different* labels to the same issue are safe because labels is a junction
table — each insert is independent. The BUG-5 "0 labels after parallel adds"
from the first session was likely a Dolt working-set race that has since been
fixed by #1969 (execContext now commits per-statement).

**Recommendation:** Re-test BUG-5 on current main. The underlying issue may
be resolved.

---

### AREA 6: Field Validation and Normalization

**Status validation gap (BUG-11, already tracked):**
`bd update --status "bogus"` succeeds. The update command validates `--type`
but not `--status`. `types.IsValidStatus` exists but isn't called from
`cmd/bd/update.go`.

**Empty field acceptance (BUG-12, BUG-14, already tracked):**
`bd update --title ""` and `bd label add X ""` both succeed.

**New findings:**

**BUG-20: JSON metadata roundtrip inconsistency** (NEW)

`UpdateIssue` normalizes metadata via `storage.NormalizeMetadataValue()` at
`issues.go:395`. But `CreateIssuesWithFullOptions` stores metadata directly
from the issue struct without normalization. If the incoming metadata is
`nil`, it gets stored as SQL NULL. Subsequent reads via `scanIssue` may return
`nil` metadata, while the update path would have stored `"{}"`.

This means an issue's metadata can be `nil` or `"{}"` depending on whether
it was created via `bd create` (which defaults to `{}`) or imported. Code that
does `if issue.Metadata == nil` vs `if issue.Metadata == "{}"` will behave
differently.

**BUG-21: `migrate_dolt` drops `metadata` and `spec_id`** (NEW)

`importToDolt()` at `cmd/bd/migrate_dolt.go:460-495` has the INSERT column
list but is missing two fields:
- `metadata` (JSON) — custom metadata is silently dropped during migration
- `spec_id` — spec references are lost

Any user who migrates from SQLite to Dolt via `bd migrate --to-dolt` loses
all custom metadata and spec associations. This is a real data loss path.

---

### AREA 7: Cross-Rig Routing and External Dependencies

**Architecture is sound:** Two distinct mechanisms:
1. Prefix-based routing via `routes.jsonl` (read-only cross-rig lookups)
2. External dependency references (`external:<project>:<issue-id>`)

**Observations:**

1. **Store lifetime management is correct**: `RoutedResult.Close()` cleans up
   routed storage connections. All callers properly defer `result.Close()`.

2. **Error fallthrough is graceful**: If a routed store lookup fails, it falls
   back to the local store (routed.go:76-77). Not-found errors don't propagate.

3. **`resolveExternalDepsViaRouting`** creates placeholder entries for
   unresolvable external deps — no crashes, just "(unresolved external
   dependency)" labels.

4. **BEADS_DIR override is respected**: When `BEADS_DIR` is set, routing is
   skipped entirely (routed.go:57-59), matching the contract for gastown
   callers.

**Potential issues:**

**BUG-22: Cross-rig `bd close` doesn't validate dep store liveness** (INVESTIGATE)

`bd close` with routing resolves the issue from a routed store, but the close
guard checks blockers from the local store's `computeBlockedIDs`. If an issue
is blocked by a cross-rig dependency, the blocked check may not find the
blocker (it's in a different database). This could allow closing an issue that
is actually blocked by a cross-rig dependency.

**Needs investigation:** Does `computeBlockedIDs` query dependencies that
reference external IDs? If external deps are stored as `external:project:id`
in `depends_on_id`, the `activeIDs` check (`activeIDs[blockerID]`) will fail
because the external ID isn't in the local `issues` table. This means external
blocking deps are silently ignored by the close guard.

---

### AREA 8: Import/Export Survivability

#### 8a. `CreateIssuesWithFullOptions` (the main import path)

**What survives:**
- All 50+ issue columns (including metadata, spec_id, due_at, defer_until)
- Labels (GH#1844 fix — separate INSERT pass)
- Comments (GH#1844 fix — separate INSERT pass)
- Dependencies (second pass after all issues exist)

**What is lost:**
- **Dependencies to missing targets** — silently skipped (issues.go:273-274,
  `continue` with no log/warning). This is the most dangerous data loss path
  in imports. A partial import will lose all cross-batch dependencies.
- **Dependency metadata** — only `type`, `created_by`, `created_at` are
  written. The `metadata` and `thread_id` columns from the dep struct are
  NOT in the import INSERT (issues.go:278-281).
- **Event history** — only a single `EventCreated` is recorded per issue.
  The original event history (status changes, label adds, etc.) is lost.

#### 8b. `importToDolt` (SQLite→Dolt migration path)

**Missing columns (BUG-21):**
- `metadata` (JSON) — silently dropped
- `spec_id` — silently dropped

**Duplicate handling:** Uses `strings.Contains(err.Error(), "Duplicate entry")`
to detect and skip duplicates (migrate_dolt.go:497-499). This is fragile —
it depends on Dolt error message format, which could change between versions.

#### 8c. Ephemeral promotion (`ephemeral_routing.go`)

When a wisp is promoted to persistent storage:
- Issue data is inserted via `insertIssue` (correct)
- Labels: `INSERT IGNORE` (correct but drops conflicts — BUG-17)
- Dependencies: `INSERT IGNORE` (drops conflicts — BUG-17)
- Events: `INSERT IGNORE` with `_, _ =` (best-effort, errors silenced)
- Comments: `INSERT IGNORE` with `_, _ =` (best-effort, errors silenced)

**Impact:** Event and comment copy failures during promotion are silently
ignored. If the promotion succeeds but event/comment copy fails, the
promoted issue has no audit trail.

---

### Build Issues

**BUG-23: `InstrumentedStorage` doesn't implement `Storage` interface** (BUILD BREAK)

`internal/telemetry/storage.go:393` has:
```go
func (s *InstrumentedStorage) RunInTransaction(ctx context.Context,
    fn func(tx storage.Transaction) error) error {
```

But the `Storage` interface at `internal/storage/storage.go:80` now requires:
```go
RunInTransaction(ctx context.Context, commitMsg string,
    fn func(tx Transaction) error) error
```

The `commitMsg string` parameter was added to `DoltStore.RunInTransaction`
(transaction.go:31) and the interface, but `InstrumentedStorage` wasn't
updated. This is a compile error that blocks the otel feature.

---

### New Bugs Summary (Session 2)

| Bug | Severity | Area | Description |
|-----|----------|------|-------------|
| BUG-15 | MEDIUM | SQL patterns | Wisp dep type silently overwritten (same as BUG-7 for wisps) |
| BUG-16 | HIGH | Concurrency | Child counter race — duplicate child IDs possible |
| BUG-17 | MEDIUM | Import/export | Wisp promotion silently drops conflicting deps |
| BUG-18 | HIGH | Blocked semantics | `GetBlockedIssues` ignores `waits-for` deps, issue invisible to both `ready` and `blocked` |
| BUG-19 | MEDIUM | Concurrency | TOCTOU race in `UpdateIssue` — stale read outside tx |
| BUG-20 | LOW | Validation | JSON metadata `nil` vs `"{}"` inconsistency between create and import |
| BUG-21 | HIGH | Import/export | `migrate_dolt` drops `metadata` and `spec_id` columns |
| BUG-22 | INVESTIGATE | Routing | Cross-rig close guard may ignore external blocking deps |
| BUG-23 | BUILD BREAK | Telemetry | `InstrumentedStorage.RunInTransaction` signature mismatch |

### Test Coverage — Session 2

Tests written in `tests/regression/discovery_test.go` and
`internal/telemetry/compile_test.go` targeting bugs BUG-15 through BUG-23.

#### Tier A: Deterministic, High-Yield (in discovery_test.go)

| Test | Bug | What it checks |
|------|-----|----------------|
| `TestA1_ExternalBlockerSemanticsEnforced` | BUG-22 | External `blocks` deps enforced by close guard and `bd ready` |
| `TestA1b_ExternalDepStoredAndVisible` | (prereq) | External deps can be added and appear in `dep list` |
| `TestA2_WaitsForBlockingAppearsInBdBlocked` | BUG-18 | `waits-for` blocked issues visible in `bd blocked` |
| `TestA3_MetadataRoundTrip` | BUG-21 | Metadata survives create → update → show → import round-trip |
| `TestA4_WispDepTypeOverwrite` | BUG-15 | Wisp dep type not silently overwritten by re-add |
| `TestBug16_ChildCounterUniqueness` | BUG-16 | Sequential child creates produce unique IDs |
| `TestBug18_BlockedSemanticsConsistency` | BUG-18 | Every open issue appears in EITHER `ready` OR `blocked` |
| `TestBug19_UpdateIssueEventConsistency` | BUG-19 | Event chain consistent after sequential title updates |
| `TestBug20_MetadataNilVsEmptyObject` | BUG-20 | Metadata normalization consistent between create and update |

#### Tier B: Stress Tests (gated behind BD_STRESS=1)

| Test | Bug | What it checks |
|------|-----|----------------|
| `TestB1_ChildCounterConcurrency` | BUG-16 | 10 concurrent child creates produce unique IDs |
| `TestB2_ConcurrentLabelAdd` | BUG-5 | 5 concurrent label adds all survive |
| `TestB3_ConcurrentUpdate` | BUG-19 | 5 concurrent title updates don't crash |

#### Tier C: Compile-Time (in internal/telemetry/compile_test.go)

| Test | Bug | What it checks |
|------|-----|----------------|
| `var _ storage.Storage = (*InstrumentedStorage)(nil)` | BUG-23 | Interface compliance — catches method signature drift |

**Status:** BUG-23 compile test immediately caught the `RunInTransaction`
signature mismatch, confirming the test works. The remaining tests require
`go test -tags=regression` with a running Dolt server.

#### Planned Tests (not yet written)

| Test | Bug | Needs |
|------|-----|-------|
| `TestBug21_MigrateToDoltPreservesMetadataAndSpecID` | BUG-21 | Real SQLite fixture file for `bd migrate --to-dolt` |
| `TestBug17_PromoteWispDoesNotSilentlyDropConflictingDeps` | BUG-17 | Wisp promotion API from CLI (may need internal test) |
| `TestBug_CreateFormDoesNotHoldWriteLockDuringEditor` | #1984 | Concurrent `bd` commands during `bd create-form` |

---

### Remaining Discovery Backlog

These are areas that warrant further investigation but were not fully explored:

1. **Re-test BUG-5** (concurrent label race) on current main — the underlying
   `execContext` fix (#1969) may have resolved it.

2. **Audit exit code handling** across all multi-ID commands beyond what BUG-10
   covers. `bd label add/remove`, `bd dep add/rm` with multiple IDs — do they
   exit non-zero on partial failure?

3. **Test `bd repo sync`** end-to-end (#1945) — the duplicate key on re-import
   and missing cross-prefix hydration are likely still broken.

4. **Audit timezone handling** in `due_at` and `defer_until` — are these
   stored as UTC? What happens when a user in UTC+9 defers until "2026-03-01"?

5. **Audit `bd rename` cascading** — `rename.go:121-134` uses LIKE CONCAT for
   bulk dependency updates. Does it correctly handle nested hierarchical IDs
   (e.g., `bd-abc.1.1`)? The `LIKE CONCAT(?, '%')` would match too broadly.

6. **Test `waits-for` + `children-of(...)` gates** end-to-end — #1899 was
   closed but the interaction between `waits-for` and `computeBlockedIDs` is
   complex (queries.go:845-970) and may have edge cases.

7. **Audit `bd sql` write safety** (OBS-3) — arbitrary writes without
   confirmation. Could silently corrupt data if an agent runs `bd sql "UPDATE ..."`.

8. **Test partial failure in `bd close` with mixed valid/invalid IDs** —
   does it close the valid ones and exit non-zero? Or abort on first failure?

9. **Audit `bd move` cross-prefix correctness** — moving an issue to a
   different prefix involves dep updates via LIKE CONCAT. Same risk as
   reparenting (BUG-8) but with prefix instead of parent ID.

10. **Profile `computeBlockedIDs` at scale** — it does 3+ full table scans
    (issues, dependencies, child lookup). At 10K+ issues this could be slow,
    especially without the cache.
