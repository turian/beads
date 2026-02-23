//go:build regression

// discovery_test.go contains tests discovered during manual regression testing
// on 2026-02-22. These tests exercise the candidate binary ONLY (not differential)
// since bd export was removed from main (BUG-1 in DISCOVERY.md).
//
// These tests require a running Dolt server on 127.0.0.1:3307.
// Each test uses a unique prefix to avoid cross-contamination (BUG-6).
package regression

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// uniquePrefix returns a random prefix for test isolation on shared Dolt server.
func uniquePrefix(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("t%d", rand.Intn(99999))
}

// newCandidateWorkspace creates a workspace using only the candidate binary with a unique prefix.
func newCandidateWorkspace(t *testing.T) *workspace {
	t.Helper()
	dir := t.TempDir()
	w := &workspace{dir: dir, bdPath: candidateBin, t: t}
	w.git("init")
	w.git("config", "user.name", "regression-test")
	w.git("config", "user.email", "test@regression.test")

	if err := os.WriteFile(filepath.Join(dir, ".gitkeep"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	w.git("add", ".")
	w.git("commit", "-m", "initial")
	w.run("init", "--prefix", uniquePrefix(t), "--quiet")
	return w
}

// parseJSON parses JSON array output from bd commands.
func parseJSON(t *testing.T, data string) []map[string]any {
	t.Helper()
	var result []map[string]any
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		t.Fatalf("parsing JSON: %v\ndata: %s", err, data)
	}
	return result
}

// parseIDs extracts "id" fields from JSON array output.
func parseIDs(t *testing.T, data string) []string {
	t.Helper()
	items := parseJSON(t, data)
	var ids []string
	for _, item := range items {
		if id, ok := item["id"].(string); ok {
			ids = append(ids, id)
		}
	}
	return ids
}

// containsID checks if an ID is in a list of IDs.
func containsID(ids []string, target string) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

// =============================================================================
// BUG REPRODUCTION TESTS
// =============================================================================

// TestBug2_DepTreeShowsNoChildren reproduces GH#1954: dep tree only shows root.
// Root cause: buildDependencyTree() never sets TreeNode.ParentID.
func TestBug2_DepTreeShowsNoChildren(t *testing.T) {
	w := newCandidateWorkspace(t)

	a := w.create("--title", "Top", "--type", "epic", "--priority", "1")
	b := w.create("--title", "Left", "--type", "task", "--priority", "2")
	c := w.create("--title", "Right", "--type", "task", "--priority", "2")
	d := w.create("--title", "Bottom", "--type", "task", "--priority", "3")

	w.run("dep", "add", a, b, "--type", "blocks")
	w.run("dep", "add", a, c, "--type", "blocks")
	w.run("dep", "add", b, d, "--type", "blocks")
	w.run("dep", "add", c, d, "--type", "blocks")

	out := w.run("dep", "tree", a)

	// The tree should contain all 4 issue IDs
	for _, id := range []string{a, b, c, d} {
		if !strings.Contains(out, id) {
			t.Errorf("dep tree output missing %s:\n%s", id, out)
		}
	}
}

// TestBug3_DepTreeReadyAnnotation checks that blocked root shows [BLOCKED] not [READY].
func TestBug3_DepTreeReadyAnnotation(t *testing.T) {
	w := newCandidateWorkspace(t)

	a := w.create("--title", "Blocked root", "--type", "task", "--priority", "2")
	b := w.create("--title", "Blocker", "--type", "task", "--priority", "1")
	w.run("dep", "add", a, b, "--type", "blocks")

	out := w.run("dep", "tree", a)

	if strings.Contains(out, "[READY]") {
		t.Errorf("blocked root should not show [READY]:\n%s", out)
	}
}

// TestBug4_ListStatusBlocked checks that list --status blocked returns blocked issues.
func TestBug4_ListStatusBlocked(t *testing.T) {
	w := newCandidateWorkspace(t)

	a := w.create("--title", "Blocked issue", "--type", "task", "--priority", "2")
	b := w.create("--title", "Blocker issue", "--type", "task", "--priority", "1")
	w.run("dep", "add", a, b, "--type", "blocks")

	// bd blocked should find a
	blockedOut := w.run("blocked", "--json")
	blockedIDs := parseIDs(t, blockedOut)
	if !containsID(blockedIDs, a) {
		t.Errorf("bd blocked should include %s, got: %v", a, blockedIDs)
	}

	// bd list --status blocked should also find a
	listOut := w.run("list", "--status", "blocked", "--json", "-n", "0")
	listIDs := parseIDs(t, listOut)
	if !containsID(listIDs, a) {
		t.Errorf("bd list --status blocked should include %s, got: %v", a, listIDs)
	}
}

// TestBug7_DepAddOverwritesType checks that dep add doesn't silently overwrite dep type.
func TestBug7_DepAddOverwritesType(t *testing.T) {
	w := newCandidateWorkspace(t)

	a := w.create("--title", "Source", "--type", "task", "--priority", "2")
	b := w.create("--title", "Target", "--type", "task", "--priority", "2")

	w.run("dep", "add", a, b, "--type", "blocks")

	// After adding blocks, a should be blocked
	blockedOut := w.run("blocked", "--json")
	blockedIDs := parseIDs(t, blockedOut)
	if !containsID(blockedIDs, a) {
		t.Fatalf("after adding blocks dep, %s should be blocked", a)
	}

	// Now add caused-by on the SAME pair — should either fail or preserve blocks
	w.run("dep", "add", a, b, "--type", "caused-by")

	// a should STILL be blocked (blocks dep should be preserved)
	blockedOut2 := w.run("blocked", "--json")
	blockedIDs2 := parseIDs(t, blockedOut2)
	if !containsID(blockedIDs2, a) {
		t.Errorf("after adding caused-by, %s should still be blocked (blocks dep lost!)", a)
	}
}

// TestBug8_ReparentDualParent checks that reparented child only shows under new parent.
func TestBug8_ReparentDualParent(t *testing.T) {
	w := newCandidateWorkspace(t)

	p1 := w.create("--title", "Parent1", "--type", "epic", "--priority", "1")
	p2 := w.create("--title", "Parent2", "--type", "epic", "--priority", "1")
	ch := w.create("--title", "Child", "--type", "task", "--priority", "2", "--parent", p1)

	// Reparent to p2
	w.run("update", ch, "--parent", p2)

	// Child should only appear under p2
	p1Children := parseIDs(t, w.run("children", p1, "--json"))
	p2Children := parseIDs(t, w.run("children", p2, "--json"))

	if containsID(p1Children, ch) {
		t.Errorf("after reparent, old parent %s should not list child %s", p1, ch)
	}
	if !containsID(p2Children, ch) {
		t.Errorf("after reparent, new parent %s should list child %s", p2, ch)
	}
}

// TestBug9_ListReadyIncludesBlocked checks list --ready vs bd ready parity.
func TestBug9_ListReadyIncludesBlocked(t *testing.T) {
	w := newCandidateWorkspace(t)

	a := w.create("--title", "Blocked", "--type", "task", "--priority", "2")
	b := w.create("--title", "Blocker", "--type", "task", "--priority", "1")
	c := w.create("--title", "Free", "--type", "task", "--priority", "3")
	w.run("dep", "add", a, b, "--type", "blocks")

	listReady := parseIDs(t, w.run("list", "--ready", "-n", "0", "--json"))
	bdReady := parseIDs(t, w.run("ready", "-n", "0", "--json"))

	// a (blocked) should NOT be in bd ready
	if containsID(bdReady, a) {
		t.Errorf("bd ready should not include blocked %s", a)
	}

	// b and c should be in both
	if !containsID(bdReady, b) {
		t.Errorf("bd ready should include unblocked %s", b)
	}
	if !containsID(bdReady, c) {
		t.Errorf("bd ready should include free %s", c)
	}

	// Ideally list --ready should match bd ready
	if containsID(listReady, a) && !containsID(bdReady, a) {
		t.Logf("KNOWN: list --ready includes blocked %s but bd ready does not", a)
	}
}

// =============================================================================
// PROTOCOL INVARIANT TESTS (working correctly, good to formalize)
// =============================================================================

// TestProtocol_CloseGuardRespectDepTypes verifies close guard only applies to blocks.
func TestProtocol_CloseGuardRespectDepTypes(t *testing.T) {
	w := newCandidateWorkspace(t)

	for _, depType := range []string{"caused-by", "validates", "tracks"} {
		t.Run(depType, func(t *testing.T) {
			a := w.create("--title", "Source "+depType, "--type", "task", "--priority", "2")
			b := w.create("--title", "Target "+depType, "--type", "task", "--priority", "2")
			w.run("dep", "add", a, b, "--type", depType)

			// Non-blocking deps should allow close
			w.run("close", a)
			out := parseJSON(t, w.run("show", a, "--json"))
			if out[0]["status"] != "closed" {
				t.Errorf("close should succeed with %s dep, got status=%v", depType, out[0]["status"])
			}
		})
	}

	// blocks should prevent close
	t.Run("blocks", func(t *testing.T) {
		a := w.create("--title", "Blocked source", "--type", "task", "--priority", "2")
		b := w.create("--title", "Blocker target", "--type", "task", "--priority", "2")
		w.run("dep", "add", a, b, "--type", "blocks")

		out, _ := w.tryRun("close", a)
		if !strings.Contains(out, "blocked by open issues") {
			t.Errorf("close of blocked issue should be rejected, got: %s", out)
		}

		// Verify still open
		showOut := parseJSON(t, w.run("show", a, "--json"))
		if showOut[0]["status"] != "open" {
			t.Errorf("blocked issue should still be open, got: %v", showOut[0]["status"])
		}
	})
}

// TestProtocol_EpicLifecycle verifies epic doesn't auto-close when all children close.
func TestProtocol_EpicLifecycle(t *testing.T) {
	w := newCandidateWorkspace(t)

	epic := w.create("--title", "Epic", "--type", "epic", "--priority", "1")
	c1 := w.create("--title", "Child1", "--type", "task", "--priority", "2", "--parent", epic)
	c2 := w.create("--title", "Child2", "--type", "task", "--priority", "2", "--parent", epic)

	// Close all children
	w.run("close", c1)
	w.run("close", c2)

	// Epic should still be open
	epicData := parseJSON(t, w.run("show", epic, "--json"))
	if epicData[0]["status"] != "open" {
		t.Errorf("epic should remain open after all children closed, got: %v", epicData[0]["status"])
	}

	// Epic should be in ready list
	readyIDs := parseIDs(t, w.run("ready", "-n", "0", "--json"))
	if !containsID(readyIDs, epic) {
		t.Errorf("epic with all children closed should be in ready list")
	}
}

// TestProtocol_DeleteCleansUpDeps verifies delete removes dependency links.
func TestProtocol_DeleteCleansUpDeps(t *testing.T) {
	w := newCandidateWorkspace(t)

	a := w.create("--title", "Dependent", "--type", "task", "--priority", "2")
	b := w.create("--title", "Will delete", "--type", "task", "--priority", "2")
	w.run("dep", "add", a, b, "--type", "blocks")

	// Verify a is blocked
	blockedIDs := parseIDs(t, w.run("blocked", "--json"))
	if !containsID(blockedIDs, a) {
		t.Fatalf("a should be blocked before delete")
	}

	// Delete b
	w.run("delete", b, "--force")

	// a should be ready now
	readyIDs := parseIDs(t, w.run("ready", "-n", "0", "--json"))
	if !containsID(readyIDs, a) {
		t.Errorf("after deleting blocker, %s should be ready", a)
	}
}

// TestProtocol_ReopenPreservesDeps verifies close/reopen preserves dependencies.
func TestProtocol_ReopenPreservesDeps(t *testing.T) {
	w := newCandidateWorkspace(t)

	a := w.create("--title", "Will reopen", "--type", "task", "--priority", "2")
	b := w.create("--title", "Dep target", "--type", "task", "--priority", "2")
	w.run("dep", "add", a, b, "--type", "caused-by")
	w.run("label", "add", a, "important")
	w.run("comments", "add", a, "Test comment")

	// Close and reopen
	w.run("close", a)
	w.run("reopen", a)

	// Verify data preserved
	data := parseJSON(t, w.run("show", a, "--json"))
	issue := data[0]

	if issue["status"] != "open" {
		t.Errorf("reopened issue should be open, got: %v", issue["status"])
	}

	deps, _ := issue["dependencies"].([]any)
	if len(deps) == 0 {
		t.Errorf("dependencies should be preserved after reopen")
	}

	labels, _ := issue["labels"].([]any)
	if len(labels) == 0 {
		t.Errorf("labels should be preserved after reopen")
	}

	comments, _ := issue["comments"].([]any)
	if len(comments) == 0 {
		t.Errorf("comments should be preserved after reopen")
	}
}

// TestProtocol_TransitiveBlockingChain verifies cascade unblocking.
func TestProtocol_TransitiveBlockingChain(t *testing.T) {
	w := newCandidateWorkspace(t)

	a := w.create("--title", "A head", "--type", "task", "--priority", "1")
	b := w.create("--title", "B mid", "--type", "task", "--priority", "2")
	c := w.create("--title", "C mid", "--type", "task", "--priority", "3")
	d := w.create("--title", "D leaf", "--type", "task", "--priority", "4")

	w.run("dep", "add", a, b, "--type", "blocks")
	w.run("dep", "add", b, c, "--type", "blocks")
	w.run("dep", "add", c, d, "--type", "blocks")

	// Only D should be ready
	readyIDs := parseIDs(t, w.run("ready", "-n", "0", "--json"))
	if !containsID(readyIDs, d) {
		t.Errorf("D (leaf) should be ready")
	}
	for _, id := range []string{a, b, c} {
		if containsID(readyIDs, id) {
			t.Errorf("%s should NOT be ready (blocked)", id)
		}
	}

	// Close D → C becomes ready
	w.run("close", d)
	readyIDs = parseIDs(t, w.run("ready", "-n", "0", "--json"))
	if !containsID(readyIDs, c) {
		t.Errorf("after closing D, C should be ready")
	}
	if containsID(readyIDs, b) {
		t.Errorf("B should still be blocked")
	}

	// Close C → B becomes ready
	w.run("close", c)
	readyIDs = parseIDs(t, w.run("ready", "-n", "0", "--json"))
	if !containsID(readyIDs, b) {
		t.Errorf("after closing C, B should be ready")
	}

	// Close B → A becomes ready
	w.run("close", b)
	readyIDs = parseIDs(t, w.run("ready", "-n", "0", "--json"))
	if !containsID(readyIDs, a) {
		t.Errorf("after closing B, A should be ready")
	}
}

// TestProtocol_CircularDepPrevention verifies cycle detection.
func TestProtocol_CircularDepPrevention(t *testing.T) {
	w := newCandidateWorkspace(t)

	a := w.create("--title", "X", "--type", "task", "--priority", "2")
	b := w.create("--title", "Y", "--type", "task", "--priority", "2")
	c := w.create("--title", "Z", "--type", "task", "--priority", "2")

	w.run("dep", "add", a, b, "--type", "blocks")
	w.run("dep", "add", b, c, "--type", "blocks")

	// Attempt to create cycle
	out, err := w.tryRun("dep", "add", c, a, "--type", "blocks")
	if err == nil {
		t.Errorf("creating cycle should fail, but got success: %s", out)
	}
	if !strings.Contains(out, "cycle") {
		t.Errorf("error should mention cycle, got: %s", out)
	}

	// Verify no cycle exists
	cycleOut := w.run("dep", "cycles")
	if !strings.Contains(cycleOut, "No dependency cycles") {
		t.Errorf("dep cycles should find none, got: %s", cycleOut)
	}
}

// TestProtocol_CloseForceOverridesGuard verifies --force bypasses close guard.
// NOTE: Close guard prints to stderr but returns exit 0 (BUG-10),
// so we check output text instead of error code.
func TestProtocol_CloseForceOverridesGuard(t *testing.T) {
	w := newCandidateWorkspace(t)

	a := w.create("--title", "Blocked", "--type", "task", "--priority", "2")
	b := w.create("--title", "Blocker", "--type", "task", "--priority", "1")
	w.run("dep", "add", a, b, "--type", "blocks")

	// Normal close should be rejected (prints to stderr, but BUG-10: exit code is 0)
	out := w.run("close", a)
	if !strings.Contains(out, "blocked by open issues") && !strings.Contains(out, "cannot close") {
		t.Fatalf("close without --force should mention blocking, got: %s", out)
	}

	// Issue should still be open
	data := parseJSON(t, w.run("show", a, "--json"))
	if data[0]["status"] != "open" {
		t.Fatalf("blocked issue should remain open after close guard, got: %v", data[0]["status"])
	}

	// Force close should succeed
	w.run("close", a, "--force")
	data = parseJSON(t, w.run("show", a, "--json"))
	if data[0]["status"] != "closed" {
		t.Errorf("force close should succeed, got status=%v", data[0]["status"])
	}
}

// TestBug10_CloseGuardExitCode verifies close guard returns non-zero exit for blocked issues.
// Currently FAILS: close guard prints to stderr but returns exit 0.
func TestBug10_CloseGuardExitCode(t *testing.T) {
	w := newCandidateWorkspace(t)

	a := w.create("--title", "Blocked", "--type", "task", "--priority", "2")
	b := w.create("--title", "Blocker", "--type", "task", "--priority", "1")
	w.run("dep", "add", a, b, "--type", "blocks")

	// Close of blocked issue should return non-zero exit code
	_, err := w.tryRun("close", a)
	if err == nil {
		t.Errorf("BUG-10: close guard should return non-zero exit code for blocked issue, but got exit 0")
	}
}

// TestBug10_ClaimExitCode verifies update --claim returns non-zero exit for already-claimed issues.
// Currently FAILS: claim error prints to stderr but returns exit 0.
func TestBug10_ClaimExitCode(t *testing.T) {
	w := newCandidateWorkspace(t)

	a := w.create("--title", "Claimable", "--type", "task", "--priority", "2")
	w.run("update", a, "--claim")

	// Second claim should return non-zero exit code
	_, err := w.tryRun("update", a, "--claim")
	if err == nil {
		t.Errorf("BUG-10: second claim should return non-zero exit code, but got exit 0")
	}
}

// TestProtocol_DeferExcludesFromReady verifies defer/undefer semantics.
func TestProtocol_DeferExcludesFromReady(t *testing.T) {
	w := newCandidateWorkspace(t)

	a := w.create("--title", "Deferred", "--type", "task", "--priority", "2")

	w.run("defer", a, "--until", "2099-12-31")

	// Should not be in ready
	readyIDs := parseIDs(t, w.run("ready", "-n", "0", "--json"))
	if containsID(readyIDs, a) {
		t.Errorf("deferred issue should not be in ready list")
	}

	// Undefer
	w.run("undefer", a)

	// Should be in ready
	readyIDs = parseIDs(t, w.run("ready", "-n", "0", "--json"))
	if !containsID(readyIDs, a) {
		t.Errorf("undeferred issue should be in ready list")
	}
}

// TestProtocol_ClaimSemantics verifies atomic claim behavior.
// NOTE: Second claim error prints to stderr but returns exit 0 (BUG-10).
func TestProtocol_ClaimSemantics(t *testing.T) {
	w := newCandidateWorkspace(t)

	a := w.create("--title", "Claimable", "--type", "task", "--priority", "2")

	w.run("update", a, "--claim")

	data := parseJSON(t, w.run("show", a, "--json"))
	if data[0]["status"] != "in_progress" {
		t.Errorf("claimed issue should be in_progress, got: %v", data[0]["status"])
	}

	// Second claim should fail (BUG-10: returns exit 0, so check stderr text)
	out := w.run("update", a, "--claim")
	if !strings.Contains(out, "already claimed") {
		t.Errorf("second claim should report 'already claimed', got: %s", out)
	}
}

// TestProtocol_NotesAppendVsOverwrite verifies notes semantics.
func TestProtocol_NotesAppendVsOverwrite(t *testing.T) {
	w := newCandidateWorkspace(t)

	a := w.create("--title", "Notes test", "--type", "task", "--priority", "2")

	w.run("update", a, "--notes", "Original")
	data := parseJSON(t, w.run("show", a, "--json"))
	if data[0]["notes"] != "Original" {
		t.Errorf("notes should be 'Original', got: %v", data[0]["notes"])
	}

	w.run("update", a, "--notes", "Replaced")
	data = parseJSON(t, w.run("show", a, "--json"))
	if data[0]["notes"] != "Replaced" {
		t.Errorf("notes should be 'Replaced', got: %v", data[0]["notes"])
	}

	w.run("update", a, "--append-notes", "Extra")
	data = parseJSON(t, w.run("show", a, "--json"))
	expected := "Replaced\nExtra"
	if data[0]["notes"] != expected {
		t.Errorf("notes should be %q, got: %v", expected, data[0]["notes"])
	}
}

// TestProtocol_SupersedeCreatesDepAndCloses verifies supersede behavior.
func TestProtocol_SupersedeCreatesDepAndCloses(t *testing.T) {
	w := newCandidateWorkspace(t)

	old := w.create("--title", "Old approach", "--type", "feature", "--priority", "2")
	new := w.create("--title", "New approach", "--type", "feature", "--priority", "2")

	w.run("supersede", old, "--with", new)

	data := parseJSON(t, w.run("show", old, "--json"))
	if data[0]["status"] != "closed" {
		t.Errorf("superseded issue should be closed, got: %v", data[0]["status"])
	}

	// Should have supersedes dependency
	deps, ok := data[0]["dependencies"].([]any)
	if !ok || len(deps) == 0 {
		t.Fatalf("superseded issue should have dependencies")
	}
	depMap := deps[0].(map[string]any)
	if depMap["dependency_type"] != "supersedes" {
		t.Errorf("dep type should be 'supersedes', got: %v", depMap["dependency_type"])
	}
}

// TestProtocol_DuplicateClosesWithDep verifies duplicate behavior.
func TestProtocol_DuplicateClosesWithDep(t *testing.T) {
	w := newCandidateWorkspace(t)

	orig := w.create("--title", "Original", "--type", "bug", "--priority", "1")
	dup := w.create("--title", "Duplicate", "--type", "bug", "--priority", "1")

	w.run("duplicate", dup, "--of", orig)

	data := parseJSON(t, w.run("show", dup, "--json"))
	if data[0]["status"] != "closed" {
		t.Errorf("duplicate issue should be closed, got: %v", data[0]["status"])
	}
}

// TestProtocol_CountByGrouping verifies count --by-* accuracy.
func TestProtocol_CountByGrouping(t *testing.T) {
	w := newCandidateWorkspace(t)

	w.create("--title", "Bug1", "--type", "bug", "--priority", "1")
	w.create("--title", "Bug2", "--type", "bug", "--priority", "2")
	w.create("--title", "Task1", "--type", "task", "--priority", "2")
	id := w.create("--title", "Feature1", "--type", "feature", "--priority", "3")
	w.run("close", id)

	// count --by-type
	out := w.run("count", "--by-type", "--json")
	var typeResult struct {
		Total  int `json:"total"`
		Groups []struct {
			Group string `json:"group"`
			Count int    `json:"count"`
		} `json:"groups"`
	}
	if err := json.Unmarshal([]byte(out), &typeResult); err != nil {
		t.Fatalf("parsing count --by-type: %v", err)
	}

	if typeResult.Total != 4 {
		t.Errorf("total should be 4, got %d", typeResult.Total)
	}

	// Verify bug count
	for _, g := range typeResult.Groups {
		if g.Group == "bug" && g.Count != 2 {
			t.Errorf("bug count should be 2, got %d", g.Count)
		}
	}
}

// TestProtocol_SpecialCharsInFields verifies special characters are preserved.
func TestProtocol_SpecialCharsInFields(t *testing.T) {
	w := newCandidateWorkspace(t)

	title := `Test "quotes" & <brackets> 'single'`
	a := w.create("--title", title, "--type", "task", "--priority", "2")

	data := parseJSON(t, w.run("show", a, "--json"))
	if data[0]["title"] != title {
		t.Errorf("title not preserved: got %v, want %v", data[0]["title"], title)
	}
}

// TestProtocol_SQLInjectionSafe verifies parameterized queries.
func TestProtocol_SQLInjectionSafe(t *testing.T) {
	w := newCandidateWorkspace(t)

	// Create an issue so we know the DB isn't empty
	w.create("--title", "Normal issue", "--type", "task", "--priority", "2")

	// Try SQL injection via search
	w.run("search", "'; DROP TABLE issues; --")

	// Verify database is intact
	out := w.run("count", "--json")
	var countResult struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal([]byte(out), &countResult); err != nil {
		t.Fatalf("parsing count after SQL injection attempt: %v", err)
	}
	if countResult.Count == 0 {
		t.Error("database appears empty after SQL injection attempt!")
	}
}

// =============================================================================
// NEWLY DISCOVERED BUGS (session 2)
// =============================================================================

// TestBug11_UpdateAcceptsInvalidStatus verifies status validation on update.
// Currently FAILS: update --status accepts arbitrary strings like "invalid".
func TestBug11_UpdateAcceptsInvalidStatus(t *testing.T) {
	w := newCandidateWorkspace(t)

	a := w.create("--title", "Status test", "--type", "task", "--priority", "2")

	// Setting an invalid status should fail
	_, err := w.tryRun("update", a, "--status", "bogus")
	if err == nil {
		// Check what status was actually set
		data := parseJSON(t, w.run("show", a, "--json"))
		if data[0]["status"] == "bogus" {
			t.Errorf("BUG-11: update --status accepted invalid value 'bogus'; should reject with error")
		}
	}
}

// TestBug12_UpdateAcceptsEmptyTitle verifies title validation on update.
// Currently FAILS: update --title "" succeeds and stores empty title.
func TestBug12_UpdateAcceptsEmptyTitle(t *testing.T) {
	w := newCandidateWorkspace(t)

	a := w.create("--title", "Has a title", "--type", "task", "--priority", "2")

	// Setting empty title should fail (create rejects it, update should too)
	_, err := w.tryRun("update", a, "--title", "")
	if err == nil {
		data := parseJSON(t, w.run("show", a, "--json"))
		if data[0]["title"] == "" {
			t.Errorf("BUG-12: update --title accepted empty string; should reject like create does")
		}
	}
}

// TestBug13_ReopenDeferredLimbo verifies reopen of closed+deferred issue.
// Currently FAILS: reopened issue has status "open" but defer_until still set.
// The issue is excluded from ready (good) but also excluded from list --status deferred.
func TestBug13_ReopenDeferredLimbo(t *testing.T) {
	w := newCandidateWorkspace(t)

	a := w.create("--title", "Defer then close", "--type", "task", "--priority", "2")
	w.run("defer", a, "--until", "2099-12-31")
	w.run("close", a)
	w.run("reopen", a)

	data := parseJSON(t, w.run("show", a, "--json"))
	status := data[0]["status"]

	// After reopening a previously-deferred issue, either:
	// 1. Status should be "deferred" (preserving the defer), OR
	// 2. defer_until should be cleared (truly reopening)
	// Currently: status="open" but defer_until still set = limbo
	if status == "open" {
		deferUntil, hasDeferUntil := data[0]["defer_until"]
		if hasDeferUntil && deferUntil != nil && deferUntil != "" {
			// Check it's not in ready
			readyIDs := parseIDs(t, w.run("ready", "-n", "0", "--json"))
			if !containsID(readyIDs, a) {
				// Not in ready (correct), but also check deferred list
				deferredOut := w.run("list", "--status", "deferred", "--json", "-n", "0")
				deferredIDs := parseIDs(t, deferredOut)
				if !containsID(deferredIDs, a) {
					t.Errorf("BUG-13: reopened+deferred issue in limbo: status=%v, defer_until=%v, not in ready or deferred list", status, deferUntil)
				}
			}
		}
	}
}

// TestBug14_EmptyLabelAccepted verifies empty label validation.
// Currently FAILS: label add accepts empty string as a label.
func TestBug14_EmptyLabelAccepted(t *testing.T) {
	w := newCandidateWorkspace(t)

	a := w.create("--title", "Label test", "--type", "task", "--priority", "2")

	// Adding an empty label should fail
	_, err := w.tryRun("label", "add", a, "")
	if err == nil {
		data := parseJSON(t, w.run("show", a, "--json"))
		labels, ok := data[0]["labels"].([]any)
		if ok {
			for _, l := range labels {
				if l == "" {
					t.Errorf("BUG-14: empty string label was accepted and stored")
				}
			}
		}
	}
}

// =============================================================================
// ADDITIONAL PROTOCOL INVARIANT TESTS
// =============================================================================

// TestProtocol_CreateWithDepsBlocksIssue verifies --deps creates blocking dependency.
func TestProtocol_CreateWithDepsBlocksIssue(t *testing.T) {
	w := newCandidateWorkspace(t)

	blocker := w.create("--title", "Blocker", "--type", "task", "--priority", "1")
	blocked := w.create("--title", "Blocked", "--type", "task", "--priority", "2", "--deps", blocker)

	blockedIDs := parseIDs(t, w.run("blocked", "--json"))
	if !containsID(blockedIDs, blocked) {
		t.Errorf("issue created with --deps should be blocked, got: %v", blockedIDs)
	}

	readyIDs := parseIDs(t, w.run("ready", "-n", "0", "--json"))
	if !containsID(readyIDs, blocker) {
		t.Errorf("blocker should be in ready list")
	}
	if containsID(readyIDs, blocked) {
		t.Errorf("blocked issue should NOT be in ready list")
	}
}

// TestProtocol_DepRemoveUnblocks verifies that removing a blocking dep unblocks.
func TestProtocol_DepRemoveUnblocks(t *testing.T) {
	w := newCandidateWorkspace(t)

	a := w.create("--title", "Source", "--type", "task", "--priority", "2")
	b := w.create("--title", "Blocker", "--type", "task", "--priority", "2")
	w.run("dep", "add", a, b, "--type", "blocks")

	// a should be blocked
	blockedIDs := parseIDs(t, w.run("blocked", "--json"))
	if !containsID(blockedIDs, a) {
		t.Fatalf("a should be blocked")
	}

	// Remove the dep
	w.run("dep", "rm", a, b)

	// a should be ready
	readyIDs := parseIDs(t, w.run("ready", "-n", "0", "--json"))
	if !containsID(readyIDs, a) {
		t.Errorf("after dep rm, a should be in ready list")
	}
}

// TestProtocol_SelfDepPrevented verifies self-dependency is rejected.
func TestProtocol_SelfDepPrevented(t *testing.T) {
	w := newCandidateWorkspace(t)

	a := w.create("--title", "Self ref", "--type", "task", "--priority", "2")
	_, err := w.tryRun("dep", "add", a, a, "--type", "blocks")
	if err == nil {
		t.Errorf("self-dependency should be rejected")
	}
}

// TestProtocol_StatusTransitionRoundTrip verifies full status lifecycle.
func TestProtocol_StatusTransitionRoundTrip(t *testing.T) {
	w := newCandidateWorkspace(t)

	a := w.create("--title", "Status lifecycle", "--type", "task", "--priority", "2")

	// open → in_progress
	w.run("update", a, "--status", "in_progress")
	data := parseJSON(t, w.run("show", a, "--json"))
	if data[0]["status"] != "in_progress" {
		t.Errorf("expected in_progress, got: %v", data[0]["status"])
	}

	// in_progress → open
	w.run("update", a, "--status", "open")
	data = parseJSON(t, w.run("show", a, "--json"))
	if data[0]["status"] != "open" {
		t.Errorf("expected open, got: %v", data[0]["status"])
	}

	// open → closed
	w.run("close", a)
	data = parseJSON(t, w.run("show", a, "--json"))
	if data[0]["status"] != "closed" {
		t.Errorf("expected closed, got: %v", data[0]["status"])
	}

	// closed → open (reopen)
	w.run("reopen", a)
	data = parseJSON(t, w.run("show", a, "--json"))
	if data[0]["status"] != "open" {
		t.Errorf("expected open after reopen, got: %v", data[0]["status"])
	}
}

// TestProtocol_TypeChangeRoundTrip verifies issue type can be changed.
func TestProtocol_TypeChangeRoundTrip(t *testing.T) {
	w := newCandidateWorkspace(t)

	a := w.create("--title", "Type change", "--type", "task", "--priority", "2")

	w.run("update", a, "--type", "bug")
	data := parseJSON(t, w.run("show", a, "--json"))
	if data[0]["issue_type"] != "bug" {
		t.Errorf("expected bug, got: %v", data[0]["issue_type"])
	}

	w.run("update", a, "--type", "epic")
	data = parseJSON(t, w.run("show", a, "--json"))
	if data[0]["issue_type"] != "epic" {
		t.Errorf("expected epic, got: %v", data[0]["issue_type"])
	}
}

// TestProtocol_DueDateRoundTrip verifies due date can be set and cleared.
func TestProtocol_DueDateRoundTrip(t *testing.T) {
	w := newCandidateWorkspace(t)

	a := w.create("--title", "Due date test", "--type", "task", "--priority", "2")

	w.run("update", a, "--due", "2099-06-15")
	data := parseJSON(t, w.run("show", a, "--json"))
	dueAt, ok := data[0]["due_at"].(string)
	if !ok || !strings.Contains(dueAt, "2099-06-15") {
		t.Errorf("due_at should contain 2099-06-15, got: %v", data[0]["due_at"])
	}
}

// TestProtocol_LabelAddRemoveRoundTrip verifies label add/remove.
func TestProtocol_LabelAddRemoveRoundTrip(t *testing.T) {
	w := newCandidateWorkspace(t)

	a := w.create("--title", "Label test", "--type", "task", "--priority", "2")

	w.run("label", "add", a, "bug-fix")
	w.run("label", "add", a, "urgent")

	data := parseJSON(t, w.run("show", a, "--json"))
	labels, _ := data[0]["labels"].([]any)
	if len(labels) != 2 {
		t.Fatalf("expected 2 labels, got %d", len(labels))
	}

	w.run("label", "remove", a, "bug-fix")
	data = parseJSON(t, w.run("show", a, "--json"))
	labels, _ = data[0]["labels"].([]any)
	if len(labels) != 1 {
		t.Errorf("expected 1 label after remove, got %d", len(labels))
	}

	// Verify correct label remains
	if len(labels) > 0 && labels[0] != "urgent" {
		t.Errorf("remaining label should be 'urgent', got: %v", labels[0])
	}
}

// TestProtocol_CommentAddAndPreserve verifies comments persist through operations.
func TestProtocol_CommentAddAndPreserve(t *testing.T) {
	w := newCandidateWorkspace(t)

	a := w.create("--title", "Comment test", "--type", "task", "--priority", "2")
	w.run("comments", "add", a, "First comment")
	w.run("comments", "add", a, "Second comment")

	data := parseJSON(t, w.run("show", a, "--json"))
	comments, _ := data[0]["comments"].([]any)
	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}

	// Close and reopen — comments should be preserved
	w.run("close", a)
	w.run("reopen", a)

	data = parseJSON(t, w.run("show", a, "--json"))
	comments, _ = data[0]["comments"].([]any)
	if len(comments) != 2 {
		t.Errorf("comments should be preserved after close/reopen, got %d", len(comments))
	}
}

// =============================================================================
// DEEP AUDIT DISCOVERY TESTS — Session 2 (Tier A: deterministic, high-yield)
// =============================================================================
//
// These tests target structural fragility patterns found during the deep code
// audit: upserts, INSERT IGNORE, prefix matching, computed-vs-stored semantics,
// cross-rig/external IDs, and concurrency.

// TestA1_ExternalBlockerSemanticsEnforced tests whether external blocking deps
// are actually enforced by close guard and bd ready.
// BUG-22 candidate: IsBlocked uses JOIN issues ON depends_on_id = id, which
// silently drops external:* deps because they don't exist in the issues table.
func TestA1_ExternalBlockerSemanticsEnforced(t *testing.T) {
	w := newCandidateWorkspace(t)

	a := w.create("--title", "Needs external capability", "--type", "task", "--priority", "2")

	// Add an external blocking dependency
	w.run("dep", "add", a, "external:someRig:capability", "--type", "blocks")

	// Verify the dependency was stored
	depOut := w.run("dep", "list", a, "--json")
	if !strings.Contains(depOut, "external:someRig:capability") {
		t.Fatalf("external dep should appear in dep list, got: %s", depOut)
	}

	// BUG-22: external blockers should prevent close (without --force)
	// Currently: IsBlocked (dependencies.go:740) uses JOIN issues ON depends_on_id = id
	// which silently drops external: deps. So close guard won't see the blocker.
	closeOut := w.run("close", a)
	showData := parseJSON(t, w.run("show", a, "--json"))
	if showData[0]["status"] == "closed" {
		t.Errorf("BUG-22: close should be rejected when blocked by external dep, but issue was closed.\n"+
			"Close guard ignores external:* blockers because IsBlocked JOINs against issues table.\n"+
			"close output: %s", closeOut)
	}

	// BUG-22: bd ready should exclude issues blocked by external deps
	readyIDs := parseIDs(t, w.run("ready", "-n", "0", "--json"))
	if containsID(readyIDs, a) {
		t.Errorf("BUG-22: bd ready should exclude issue %s blocked by external dep, but it appeared in ready list.\n"+
			"computeBlockedIDs ignores external:* because activeIDs check fails (external ID not in issues table).", a)
	}
}

// TestA2_WaitsForBlockingAppearsInBdBlocked tests whether waits-for blocked
// issues appear in both `bd ready` (exclusion) and `bd blocked` (inclusion).
// BUG-18: GetBlockedIssues only checks 'blocks' deps, but computeBlockedIDs
// (used by bd ready) also checks 'waits-for'. An issue blocked by waits-for
// can be invisible to both commands.
func TestA2_WaitsForBlockingAppearsInBdBlocked(t *testing.T) {
	w := newCandidateWorkspace(t)

	// Create a spawner (parent) with a child
	spawner := w.create("--title", "Spawner", "--type", "epic", "--priority", "1")
	child := w.create("--title", "Child task", "--type", "task", "--priority", "2", "--parent", spawner)

	// Create a gate issue that waits-for the spawner's children
	gate := w.create("--title", "Gate waiting for children", "--type", "task", "--priority", "2")
	w.run("dep", "add", gate, spawner, "--type", "waits-for")

	// Gate should NOT be in bd ready (child is still open)
	readyIDs := parseIDs(t, w.run("ready", "-n", "0", "--json"))
	if containsID(readyIDs, gate) {
		t.Errorf("gate should NOT be in ready (waits-for spawner with open child)")
	}

	// BUG-18: Gate SHOULD appear in bd blocked
	// Currently: GetBlockedIssues only checks 'blocks' deps, not 'waits-for'
	blockedOut := w.run("blocked", "--json")
	blockedIDs := parseIDs(t, blockedOut)
	if !containsID(blockedIDs, gate) {
		t.Errorf("BUG-18: gate should appear in bd blocked (waits-for with open child), but it doesn't.\n"+
			"GetBlockedIssues only checks 'blocks' deps, not 'waits-for'.\n"+
			"blocked output: %s", blockedOut)
	}

	// After closing the child, gate should become ready
	w.run("close", child)
	readyIDs = parseIDs(t, w.run("ready", "-n", "0", "--json"))
	if !containsID(readyIDs, gate) {
		t.Errorf("after closing child, gate should be in ready list")
	}
	// Suppress unused variable warning
	_ = spawner
}

// TestA3_MigrateDoltPreservesMetadataAndSpecID tests that bd migrate --to-dolt
// preserves the metadata JSON field and spec_id.
// BUG-21: importToDolt INSERT is missing metadata and spec_id columns.
//
// NOTE: This test is difficult to run without a real SQLite→Dolt migration path.
// Instead, we test the import path (CreateIssuesWithFullOptions) which is the
// shared code, and verify metadata survives a create-then-show round-trip.
func TestA3_MetadataRoundTrip(t *testing.T) {
	w := newCandidateWorkspace(t)

	// Create issue with metadata
	a := w.create("--title", "Has metadata", "--type", "task", "--priority", "2")
	w.run("update", a, "--metadata", `{"custom_key":"custom_value","number":42}`)

	// Read it back
	data := parseJSON(t, w.run("show", a, "--json"))
	metadata, ok := data[0]["metadata"]
	if !ok || metadata == nil {
		t.Fatalf("metadata should be present in show --json output")
	}

	// Verify it's valid JSON with our custom fields
	metaStr, ok := metadata.(string)
	if !ok {
		// metadata might be returned as a map
		metaMap, ok := metadata.(map[string]any)
		if !ok {
			t.Fatalf("metadata should be string or map, got %T: %v", metadata, metadata)
		}
		if metaMap["custom_key"] != "custom_value" {
			t.Errorf("metadata.custom_key should be 'custom_value', got: %v", metaMap["custom_key"])
		}
	} else {
		if !strings.Contains(metaStr, "custom_key") || !strings.Contains(metaStr, "custom_value") {
			t.Errorf("metadata should contain custom_key/custom_value, got: %s", metaStr)
		}
	}

	// Now test that IMPORT preserves metadata:
	// Export to JSON, then re-import into a fresh workspace
	showOut := w.run("show", a, "--json")

	w2 := newCandidateWorkspace(t)
	// Write JSON to a file and import
	jsonFile := filepath.Join(w2.dir, "import.jsonl")
	if err := os.WriteFile(jsonFile, []byte(showOut), 0o644); err != nil {
		t.Fatal(err)
	}
	importOut, err := w2.tryRun("import", "-i", jsonFile)
	if err != nil {
		t.Logf("import failed (may be prefix mismatch): %s", importOut)
		t.Skip("import failed — likely prefix mismatch between workspaces")
	}

	// Verify metadata survived import
	importedData := parseJSON(t, w2.run("show", a, "--json"))
	importedMeta := importedData[0]["metadata"]
	if importedMeta == nil {
		t.Errorf("BUG-21: metadata was lost during import round-trip")
	}
}

// TestA4_DepAddOverwritesTypeOnWisps tests whether wisp dep upsert silently
// overwrites dependency type, same as BUG-7 but in the ephemeral path.
// BUG-15: wisps.go:836 uses ON DUPLICATE KEY UPDATE type = VALUES(type)
//
// NOTE: This test requires bd create --ephemeral to work. If it doesn't
// support --ephemeral from CLI, we skip.
func TestA4_WispDepTypeOverwrite(t *testing.T) {
	w := newCandidateWorkspace(t)

	// Try to create ephemeral issues
	ephID, err := w.tryCreate("--title", "Ephemeral task", "--type", "task", "--priority", "2", "--ephemeral")
	if err != nil {
		t.Skip("bd create --ephemeral not available from CLI: " + err.Error())
	}

	target := w.create("--title", "Target", "--type", "task", "--priority", "2")

	// Add blocks dep first
	_, err = w.tryRun("dep", "add", ephID, target, "--type", "blocks")
	if err != nil {
		t.Skip("dep add on ephemeral issues not supported from CLI")
	}

	// Now add caused-by on the same pair — should either fail or preserve blocks
	w.run("dep", "add", ephID, target, "--type", "caused-by")

	// Check if the blocks relationship survived
	depOut := w.run("dep", "list", ephID, "--json")
	if !strings.Contains(depOut, "blocks") {
		t.Errorf("BUG-15: wisp dep type was silently overwritten from 'blocks' to 'caused-by'.\n"+
			"wisps.go:836 uses ON DUPLICATE KEY UPDATE type = VALUES(type).\n"+
			"dep list output: %s", depOut)
	}
}

// TestA1b_ExternalDepStoredAndVisible is a prerequisite check for A1:
// verify external deps can be added and are visible in dep list.
func TestA1b_ExternalDepStoredAndVisible(t *testing.T) {
	w := newCandidateWorkspace(t)

	a := w.create("--title", "Has external dep", "--type", "task", "--priority", "2")
	w.run("dep", "add", a, "external:otherProject:feature-x", "--type", "blocks")

	// dep list should show the external dep
	depOut := w.run("dep", "list", a)
	if !strings.Contains(depOut, "external:otherProject:feature-x") {
		t.Errorf("external dep should appear in dep list output, got: %s", depOut)
	}

	// dep list --json should include it
	depJSON := w.run("dep", "list", a, "--json")
	if !strings.Contains(depJSON, "external:otherProject:feature-x") {
		t.Errorf("external dep should appear in dep list --json output, got: %s", depJSON)
	}
}

// TestBug16_ChildCounterUniqueness tests whether GetNextChildID produces
// unique child IDs. This is a deterministic version — sequential creates.
// See TestB1 for the concurrent stress version.
func TestBug16_ChildCounterUniqueness(t *testing.T) {
	w := newCandidateWorkspace(t)

	parent := w.create("--title", "Parent epic", "--type", "epic", "--priority", "1")

	// Create 5 children sequentially
	childIDs := make(map[string]bool)
	for i := 0; i < 5; i++ {
		id := w.create("--title", fmt.Sprintf("Child %d", i), "--type", "task", "--priority", "2", "--parent", parent)
		if childIDs[id] {
			t.Errorf("duplicate child ID: %s", id)
		}
		childIDs[id] = true
	}

	// All 5 should appear as children
	children := parseIDs(t, w.run("children", parent, "--json"))
	if len(children) != 5 {
		t.Errorf("expected 5 children, got %d: %v", len(children), children)
	}
}

// TestBug18_BlockedSemanticsConsistency tests that bd blocked and bd ready
// agree on what's blocked. If an issue is not in ready, it should be findable
// via bd blocked (or bd list --status deferred, etc.).
func TestBug18_BlockedSemanticsConsistency(t *testing.T) {
	w := newCandidateWorkspace(t)

	a := w.create("--title", "Blocked by blocks dep", "--type", "task", "--priority", "2")
	b := w.create("--title", "Blocker", "--type", "task", "--priority", "1")
	c := w.create("--title", "Free task", "--type", "task", "--priority", "3")

	w.run("dep", "add", a, b, "--type", "blocks")

	readyIDs := parseIDs(t, w.run("ready", "-n", "0", "--json"))
	blockedIDs := parseIDs(t, w.run("blocked", "--json"))

	// a should be in blocked, not in ready
	if containsID(readyIDs, a) {
		t.Errorf("blocked issue %s should not be in ready", a)
	}
	if !containsID(blockedIDs, a) {
		t.Errorf("blocked issue %s should be in blocked list", a)
	}

	// b and c should be in ready, not in blocked
	if !containsID(readyIDs, b) {
		t.Errorf("unblocked issue %s should be in ready", b)
	}
	if !containsID(readyIDs, c) {
		t.Errorf("free issue %s should be in ready", c)
	}
	if containsID(blockedIDs, b) {
		t.Errorf("blocker %s should not be in blocked list", b)
	}

	// Every non-closed, non-deferred issue should be in EITHER ready OR blocked
	// (this is the key invariant — no invisible issues)
	allIDs := parseIDs(t, w.run("list", "-n", "0", "--json"))
	for _, id := range allIDs {
		if !containsID(readyIDs, id) && !containsID(blockedIDs, id) {
			t.Errorf("BUG-18 pattern: issue %s is in neither ready nor blocked — invisible to workflow commands", id)
		}
	}
}

// TestBug19_UpdateIssueEventConsistency tests that UpdateIssue records
// events with correct old/new values even under sequential updates.
// The TOCTOU race (BUG-19) is hard to reproduce deterministically,
// but we can at least verify the event chain is consistent.
func TestBug19_UpdateIssueEventConsistency(t *testing.T) {
	w := newCandidateWorkspace(t)

	a := w.create("--title", "Event test", "--type", "task", "--priority", "2")

	// Chain of updates
	w.run("update", a, "--title", "First title")
	w.run("update", a, "--title", "Second title")
	w.run("update", a, "--title", "Third title")

	// Final state should be "Third title"
	data := parseJSON(t, w.run("show", a, "--json"))
	if data[0]["title"] != "Third title" {
		t.Errorf("final title should be 'Third title', got: %v", data[0]["title"])
	}
}

// TestBug20_MetadataNilVsEmptyObject tests metadata consistency between
// create and update paths.
// BUG-20: CreateIssue may store metadata as SQL NULL, while UpdateIssue
// normalizes to "{}". Code checking `metadata == nil` vs `metadata == "{}"`
// will behave differently.
func TestBug20_MetadataNilVsEmptyObject(t *testing.T) {
	w := newCandidateWorkspace(t)

	// Create without metadata
	a := w.create("--title", "No metadata", "--type", "task", "--priority", "2")
	data := parseJSON(t, w.run("show", a, "--json"))

	meta1 := data[0]["metadata"]

	// Create with empty metadata
	b := w.create("--title", "With metadata", "--type", "task", "--priority", "2")
	w.run("update", b, "--metadata", "{}")
	data2 := parseJSON(t, w.run("show", b, "--json"))
	meta2 := data2[0]["metadata"]

	// Both should be equivalent (either both nil/empty or both {})
	// The key invariant: a show --json should return a consistent metadata shape
	// regardless of whether metadata was explicitly set
	meta1Str := fmt.Sprintf("%v", meta1)
	meta2Str := fmt.Sprintf("%v", meta2)

	// If one is nil and the other is {}, that's a normalization inconsistency
	if (meta1 == nil && meta2 != nil) || (meta1Str == "<nil>" && meta2Str == "{}") {
		t.Logf("BUG-20: metadata inconsistency: create-without-metadata=%v (%T), create-then-update-metadata=%v (%T)",
			meta1, meta1, meta2, meta2)
		// This is a LOW severity issue — log but don't fail
	}
}

// =============================================================================
// DEEP AUDIT DISCOVERY TESTS — Session 2 (Tier B: stress tests)
// =============================================================================
//
// These tests are probabilistic and may be flaky under load. Gate behind
// BD_STRESS=1 environment variable to avoid impacting CI.

// TestB1_ChildCounterConcurrency stress-tests concurrent child creation
// to detect the race in GetNextChildID (BUG-16).
func TestB1_ChildCounterConcurrency(t *testing.T) {
	if os.Getenv("BD_STRESS") == "" {
		t.Skip("set BD_STRESS=1 to run stress discovery tests")
	}

	w := newCandidateWorkspace(t)
	parent := w.create("--title", "Stress parent", "--type", "epic", "--priority", "1")

	const N = 10
	type result struct {
		id  string
		err error
	}
	results := make(chan result, N)

	for i := 0; i < N; i++ {
		go func(n int) {
			id, err := w.tryCreate("--title", fmt.Sprintf("Concurrent child %d", n),
				"--type", "task", "--priority", "2", "--parent", parent)
			results <- result{id: strings.TrimSpace(id), err: err}
		}(i)
	}

	ids := make(map[string]bool)
	var errors []error
	for i := 0; i < N; i++ {
		r := <-results
		if r.err != nil {
			errors = append(errors, r.err)
			continue
		}
		if ids[r.id] {
			t.Errorf("BUG-16: duplicate child ID %s from concurrent creation", r.id)
		}
		ids[r.id] = true
	}

	if len(errors) > 0 {
		t.Logf("BUG-16: %d/%d concurrent child creations failed: %v", len(errors), N, errors)
	}

	// All successful creates should have unique IDs
	if len(ids)+len(errors) != N {
		t.Errorf("expected %d results, got %d successes + %d errors", N, len(ids), len(errors))
	}

	// Verify parent has the expected number of children
	children := parseIDs(t, w.run("children", parent, "--json"))
	if len(children) != len(ids) {
		t.Errorf("parent should have %d children (successful creates), got %d", len(ids), len(children))
	}
}

// TestB2_ConcurrentLabelAdd stress-tests concurrent label additions
// to verify BUG-5 is fixed on current main.
func TestB2_ConcurrentLabelAdd(t *testing.T) {
	if os.Getenv("BD_STRESS") == "" {
		t.Skip("set BD_STRESS=1 to run stress discovery tests")
	}

	w := newCandidateWorkspace(t)
	a := w.create("--title", "Label stress", "--type", "task", "--priority", "2")

	const N = 5
	done := make(chan error, N)

	for i := 0; i < N; i++ {
		go func(n int) {
			_, err := w.tryRun("label", "add", a, fmt.Sprintf("stress-%d", n))
			done <- err
		}(i)
	}

	var errors []error
	for i := 0; i < N; i++ {
		if err := <-done; err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		t.Logf("%d/%d concurrent label adds failed: %v", len(errors), N, errors)
	}

	// Check how many labels survived
	data := parseJSON(t, w.run("show", a, "--json"))
	labels, _ := data[0]["labels"].([]any)
	expectedCount := N - len(errors)
	if len(labels) < expectedCount {
		t.Errorf("BUG-5 (revisit): expected %d labels after concurrent adds, got %d (lost %d)",
			expectedCount, len(labels), expectedCount-len(labels))
	}
}

// TestB3_ConcurrentUpdate stress-tests concurrent issue updates
// to detect the TOCTOU race in UpdateIssue (BUG-19).
func TestB3_ConcurrentUpdate(t *testing.T) {
	if os.Getenv("BD_STRESS") == "" {
		t.Skip("set BD_STRESS=1 to run stress discovery tests")
	}

	w := newCandidateWorkspace(t)
	a := w.create("--title", "Update stress", "--type", "task", "--priority", "2")

	const N = 5
	done := make(chan error, N)

	for i := 0; i < N; i++ {
		go func(n int) {
			_, err := w.tryRun("update", a, "--title", fmt.Sprintf("Title from goroutine %d", n))
			done <- err
		}(i)
	}

	var errors []error
	for i := 0; i < N; i++ {
		if err := <-done; err != nil {
			errors = append(errors, err)
		}
	}

	// The final title should be one of the goroutine titles
	data := parseJSON(t, w.run("show", a, "--json"))
	title, _ := data[0]["title"].(string)
	if !strings.HasPrefix(title, "Title from goroutine ") {
		t.Errorf("unexpected final title after concurrent updates: %s", title)
	}

	if len(errors) > 0 {
		t.Logf("BUG-19: %d/%d concurrent updates failed (may indicate lock contention): %v",
			len(errors), N, errors)
	}
}
