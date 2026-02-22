package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "plenary")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return bin
}

func run(t *testing.T, bin, storePath, actorID, actorType string, args ...string) map[string]string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(),
		"PLENARY_DB="+storePath,
		"PLENARY_ACTOR_ID="+actorID,
		"PLENARY_ACTOR_TYPE="+actorType,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command %v failed: %v\n%s", args, err, out)
	}
	var result map[string]string
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("failed to parse output: %v\n%s", err, out)
	}
	return result
}

func runStatus(t *testing.T, bin, storePath, plenaryID string) map[string]any {
	t.Helper()
	cmd := exec.Command(bin, "status", "--plenary", plenaryID)
	cmd.Env = append(os.Environ(), "PLENARY_DB="+storePath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("status failed: %v\n%s", err, out)
	}
	var result map[string]any
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("failed to parse status: %v\n%s", err, out)
	}
	return result
}

func runExpectFail(t *testing.T, bin, storePath, actorID, actorType string, expectedExit int, args ...string) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(),
		"PLENARY_DB="+storePath,
		"PLENARY_ACTOR_ID="+actorID,
		"PLENARY_ACTOR_TYPE="+actorType,
	)
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected failure for %v but got success", args)
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() != expectedExit {
			t.Fatalf("expected exit code %d, got %d for %v", expectedExit, exitErr.ExitCode(), args)
		}
	}
}

func TestCLIFullLifecycle(t *testing.T) {
	bin := buildBinary(t)
	store := filepath.Join(t.TempDir(), "events.jsonl")

	// Create
	result := run(t, bin, store, "keeton", "human",
		"create", "--topic", "Which database?", "--context", "Need to pick a DB", "--rule", "unanimity")
	pid := result["plenary_id"]
	if pid == "" {
		t.Fatal("no plenary_id returned")
	}

	// Join
	run(t, bin, store, "claude", "agent", "join", "--plenary", pid)
	run(t, bin, store, "codex", "agent", "join", "--plenary", pid)

	// Phase transitions
	run(t, bin, store, "keeton", "human", "phase", "--plenary", pid, "--from", "framing", "--to", "divergence")
	run(t, bin, store, "claude", "agent", "speak", "--plenary", pid, "--message", "I think SQLite is fine for v0")
	run(t, bin, store, "keeton", "human", "phase", "--plenary", pid, "--from", "divergence", "--to", "proposal")

	// Propose
	propResult := run(t, bin, store, "claude", "agent", "propose", "--plenary", pid, "--text", "Use SQLite for v0")
	propID := propResult["proposal_id"]
	if propID == "" {
		t.Fatal("no proposal_id returned")
	}

	// Move to consensus check
	run(t, bin, store, "keeton", "human", "phase", "--plenary", pid, "--from", "proposal", "--to", "consensus_check")

	// Consent
	run(t, bin, store, "claude", "agent", "consent", "--plenary", pid, "--proposal", propID)
	run(t, bin, store, "codex", "agent", "consent", "--plenary", pid, "--proposal", propID)

	// Status should show ready_to_close
	status := runStatus(t, bin, store, pid)
	if status["ready_to_close"] != true {
		t.Errorf("expected ready_to_close=true, got %v", status["ready_to_close"])
	}

	// Close
	run(t, bin, store, "keeton", "human", "close", "--plenary", pid, "--resolution", "SQLite for v0", "--outcome", "consensus")

	// Final status
	finalStatus := runStatus(t, bin, store, pid)
	if finalStatus["closed"] != true {
		t.Error("expected closed=true")
	}
	if finalStatus["outcome"] != "consensus" {
		t.Errorf("expected outcome=consensus, got %v", finalStatus["outcome"])
	}
}

func TestCLIBlockAndWithdraw(t *testing.T) {
	bin := buildBinary(t)
	store := filepath.Join(t.TempDir(), "events.jsonl")

	result := run(t, bin, store, "keeton", "human", "create", "--topic", "Test blocking")
	pid := result["plenary_id"]

	run(t, bin, store, "claude", "agent", "join", "--plenary", pid)
	run(t, bin, store, "codex", "agent", "join", "--plenary", pid)
	run(t, bin, store, "keeton", "human", "phase", "--plenary", pid, "--from", "framing", "--to", "divergence")
	run(t, bin, store, "keeton", "human", "phase", "--plenary", pid, "--from", "divergence", "--to", "proposal")

	propResult := run(t, bin, store, "claude", "agent", "propose", "--plenary", pid, "--text", "Proposal A")
	propID := propResult["proposal_id"]

	// Block
	run(t, bin, store, "codex", "agent", "block", "--plenary", pid, "--proposal", propID, "--reason", "Not convinced")

	run(t, bin, store, "keeton", "human", "phase", "--plenary", pid, "--from", "proposal", "--to", "consensus_check")

	// Should not be ready to close
	status := runStatus(t, bin, store, pid)
	if status["ready_to_close"] == true {
		t.Error("should NOT be ready to close with an unresolved block")
	}
}

func TestCLIStandAsideDoesNotBlock(t *testing.T) {
	bin := buildBinary(t)
	store := filepath.Join(t.TempDir(), "events.jsonl")

	result := run(t, bin, store, "keeton", "human", "create", "--topic", "Test stand-aside")
	pid := result["plenary_id"]

	run(t, bin, store, "claude", "agent", "join", "--plenary", pid)
	run(t, bin, store, "codex", "agent", "join", "--plenary", pid)
	run(t, bin, store, "keeton", "human", "phase", "--plenary", pid, "--from", "framing", "--to", "divergence")
	run(t, bin, store, "keeton", "human", "phase", "--plenary", pid, "--from", "divergence", "--to", "proposal")

	propResult := run(t, bin, store, "claude", "agent", "propose", "--plenary", pid, "--text", "Proposal B")
	propID := propResult["proposal_id"]

	run(t, bin, store, "keeton", "human", "phase", "--plenary", pid, "--from", "proposal", "--to", "consensus_check")

	run(t, bin, store, "claude", "agent", "consent", "--plenary", pid, "--proposal", propID)
	run(t, bin, store, "codex", "agent", "stand-aside", "--plenary", pid, "--proposal", propID, "--reason", "I disagree but won't block")

	status := runStatus(t, bin, store, pid)
	if status["ready_to_close"] != true {
		t.Error("stand-aside should NOT prevent consensus")
	}
}

func TestCLIPhaseConflict(t *testing.T) {
	bin := buildBinary(t)
	store := filepath.Join(t.TempDir(), "events.jsonl")

	result := run(t, bin, store, "keeton", "human", "create", "--topic", "Test phase conflict")
	pid := result["plenary_id"]

	// Try to set phase with wrong expected
	runExpectFail(t, bin, store, "keeton", "human", 3,
		"phase", "--plenary", pid, "--from", "divergence", "--to", "proposal")
}

func TestCLIExport(t *testing.T) {
	bin := buildBinary(t)
	store := filepath.Join(t.TempDir(), "events.jsonl")
	exportDir := filepath.Join(t.TempDir(), "export")

	result := run(t, bin, store, "keeton", "human", "create", "--topic", "Export test")
	pid := result["plenary_id"]

	run(t, bin, store, "claude", "agent", "join", "--plenary", pid)

	// Export may return mixed types (bool for decision_record_present), so parse as map[string]any
	cmd := exec.Command(bin, "export", "--plenary", pid, "--out", exportDir)
	cmd.Env = append(os.Environ(), "PLENARY_DB="+store, "PLENARY_ACTOR_ID=keeton", "PLENARY_ACTOR_TYPE=human")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("export failed: %v\n%s", err, out)
	}
	var exportResult map[string]any
	if err := json.Unmarshal(out, &exportResult); err != nil {
		t.Fatalf("failed to parse export output: %v\n%s", err, out)
	}
	if exportResult["status"] != "exported" {
		t.Errorf("expected status=exported, got %v", exportResult["status"])
	}

	// Check files exist
	for _, f := range []string{"events.jsonl", "snapshot.json", "transcript.md"} {
		path := filepath.Join(exportDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected %s to exist", path)
		}
	}
}

func TestCLIListAndLastAndActiveProposalShorthand(t *testing.T) {
	bin := buildBinary(t)
	store := filepath.Join(t.TempDir(), "events.jsonl")

	// Create two plenaries so --last and list ordering have meaning.
	p1 := run(t, bin, store, "alice", "human", "create", "--topic", "First")["plenary_id"]
	if p1 == "" {
		t.Fatal("missing plenary_id for p1")
	}
	p2 := run(t, bin, store, "bob", "agent", "create", "--topic", "Second")["plenary_id"]
	if p2 == "" {
		t.Fatal("missing plenary_id for p2")
	}

	// list should include both plenaries and newest first (p2).
	cmd := exec.Command(bin, "list")
	cmd.Env = append(os.Environ(), "PLENARY_DB="+store)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("list failed: %v\n%s", err, out)
	}
	var list []map[string]any
	if err := json.Unmarshal(out, &list); err != nil {
		t.Fatalf("list parse failed: %v\n%s", err, out)
	}
	if len(list) < 2 {
		t.Fatalf("expected at least 2 plenaries in list, got %d", len(list))
	}
	if list[0]["plenary_id"] != p2 {
		t.Fatalf("expected most recent plenary first (%s), got %v", p2, list[0]["plenary_id"])
	}

	// Build a proposal in p2 and use PLENARY_ID + implicit active proposal for consent.
	run(t, bin, store, "claude", "agent", "join", "--plenary", p2)
	run(t, bin, store, "keeton", "human", "phase", "--plenary", p2, "--from", "framing", "--to", "divergence")
	run(t, bin, store, "keeton", "human", "phase", "--plenary", p2, "--from", "divergence", "--to", "proposal")
	run(t, bin, store, "claude", "agent", "propose", "--plenary", p2, "--text", "Proposal X")
	run(t, bin, store, "keeton", "human", "phase", "--plenary", p2, "--from", "proposal", "--to", "consensus_check")

	consent := exec.Command(bin, "consent", "--reason", "works")
	consent.Env = append(os.Environ(),
		"PLENARY_DB="+store,
		"PLENARY_ACTOR_ID=codex",
		"PLENARY_ACTOR_TYPE=agent",
		"PLENARY_ID="+p2,
	)
	out, err = consent.CombinedOutput()
	if err != nil {
		t.Fatalf("consent with PLENARY_ID + implicit active proposal failed: %v\n%s", err, out)
	}

	// status --last should resolve to p2 and show codex consent.
	statusCmd := exec.Command(bin, "status", "--last")
	statusCmd.Env = append(os.Environ(), "PLENARY_DB="+store)
	out, err = statusCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("status --last failed: %v\n%s", err, out)
	}
	var status map[string]any
	if err := json.Unmarshal(out, &status); err != nil {
		t.Fatalf("status --last parse failed: %v\n%s", err, out)
	}
	if status["plenary_id"] != p2 {
		t.Fatalf("status --last expected plenary %s, got %v", p2, status["plenary_id"])
	}
}

func TestCLIActorTypeNormalizationAndValidation(t *testing.T) {
	bin := buildBinary(t)
	store := filepath.Join(t.TempDir(), "events.jsonl")

	// 'ai' should be accepted and normalized to 'agent'
	pid := run(t, bin, store, "keeton", "human", "create", "--topic", "Actor type")["plenary_id"]
	run(t, bin, store, "codex", "ai", "join", "--plenary", pid)
	status := runStatus(t, bin, store, pid)
	participants, ok := status["participants"].([]any)
	if !ok {
		t.Fatalf("participants missing or wrong type: %T", status["participants"])
	}
	found := false
	for _, raw := range participants {
		p, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if p["actor_id"] == "codex" {
			found = true
			if p["actor_type"] != "agent" {
				t.Fatalf("expected actor_type normalized to agent, got %v", p["actor_type"])
			}
		}
	}
	if !found {
		t.Fatal("expected codex participant")
	}

	// Invalid actor type should fail with validation exit code.
	cmd := exec.Command(bin, "join", "--plenary", pid)
	cmd.Env = append(os.Environ(),
		"PLENARY_DB="+store,
		"PLENARY_ACTOR_ID=bad",
		"PLENARY_ACTOR_TYPE=robot",
	)
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected invalid actor type to fail")
	}
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() != 2 {
		t.Fatalf("expected exit 2 for invalid actor type, got %d", ee.ExitCode())
	}
}

func TestCLIStatusWithoutPlenaryIDFails(t *testing.T) {
	bin := buildBinary(t)
	store := filepath.Join(t.TempDir(), "events.jsonl")

	cmd := exec.Command(bin, "status")
	cmd.Env = append(os.Environ(), "PLENARY_DB="+store)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected status without plenary ID to fail")
	}
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %d", ee.ExitCode())
	}
	if !strings.Contains(string(out), "PLENARY_ID") && !strings.Contains(string(out), "--last") {
		t.Fatalf("expected error message to mention PLENARY_ID/--last, got: %s", out)
	}
}
