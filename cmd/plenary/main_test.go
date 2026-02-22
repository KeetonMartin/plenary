package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
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
