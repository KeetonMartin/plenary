package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"
)

func startServer(t *testing.T, bin, storePath, port string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(bin, "serve", "--port", port)
	cmd.Env = append(os.Environ(), "PLENARY_DB="+storePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	// Wait for server to be ready
	baseURL := fmt.Sprintf("http://localhost:%s", port)
	for i := 0; i < 50; i++ {
		resp, err := http.Get(baseURL + "/api/plenaries")
		if err == nil {
			resp.Body.Close()
			return cmd
		}
		time.Sleep(100 * time.Millisecond)
	}
	cmd.Process.Kill()
	t.Fatal("server did not start in time")
	return nil
}

func TestServeFullLifecycle(t *testing.T) {
	binary := buildBinary(t)
	tmpDir := t.TempDir()
	storePath := tmpDir + "/events.jsonl"
	port := "18921"

	cmd := startServer(t, binary, storePath, port)
	defer cmd.Process.Kill()

	baseURL := fmt.Sprintf("http://localhost:%s", port)
	actor := map[string]string{"actor_id": "agent-1", "actor_type": "agent"}

	// 1. Create plenary
	createResp := apiPost(t, baseURL+"/api/plenaries", map[string]any{
		"actor":         actor,
		"topic":         "Test lifecycle via API",
		"decision_rule": "unanimity",
	})
	plenaryID := createResp["plenary_id"].(string)
	if plenaryID == "" {
		t.Fatal("expected plenary_id")
	}

	// 2. Join
	joinResp := apiPost(t, fmt.Sprintf("%s/api/plenaries/%s/join", baseURL, plenaryID), map[string]any{
		"actor": actor,
	})
	assertEqual(t, joinResp["status"], "joined")

	// 3. Speak
	speakResp := apiPost(t, fmt.Sprintf("%s/api/plenaries/%s/speak", baseURL, plenaryID), map[string]any{
		"actor": actor,
		"text":  "Hello from API test",
	})
	assertEqual(t, speakResp["status"], "spoke")

	// 4. Status
	statusResp := apiGet(t, fmt.Sprintf("%s/api/plenaries/%s", baseURL, plenaryID))
	assertEqual(t, statusResp["topic"], "Test lifecycle via API")
	assertEqual(t, statusResp["phase"], "framing")

	// 5. Phase transitions -> proposal
	apiPost(t, fmt.Sprintf("%s/api/plenaries/%s/phase", baseURL, plenaryID), map[string]any{
		"actor": actor, "to": "divergence", "from": "framing",
	})
	apiPost(t, fmt.Sprintf("%s/api/plenaries/%s/phase", baseURL, plenaryID), map[string]any{
		"actor": actor, "to": "proposal", "from": "divergence",
	})

	// 6. Propose
	proposeResp := apiPost(t, fmt.Sprintf("%s/api/plenaries/%s/propose", baseURL, plenaryID), map[string]any{
		"actor": actor,
		"text":  "Test proposal",
	})
	proposalID := proposeResp["proposal_id"].(string)

	// 7. Consensus check + consent
	apiPost(t, fmt.Sprintf("%s/api/plenaries/%s/phase", baseURL, plenaryID), map[string]any{
		"actor": actor, "to": "consensus_check", "from": "proposal",
	})
	consentResp := apiPost(t, fmt.Sprintf("%s/api/plenaries/%s/consent", baseURL, plenaryID), map[string]any{
		"actor":       actor,
		"proposal_id": proposalID,
		"reason":      "Looks good",
	})
	assertEqual(t, consentResp["status"], "consent_given")

	// 8. Close
	closeResp := apiPost(t, fmt.Sprintf("%s/api/plenaries/%s/close", baseURL, plenaryID), map[string]any{
		"actor":      actor,
		"outcome":    "consensus",
		"resolution": "API test passed",
	})
	assertEqual(t, closeResp["status"], "closed")

	// 9. List
	listResp := apiGetArray(t, baseURL+"/api/plenaries")
	found := false
	for _, p := range listResp {
		pm := p.(map[string]any)
		if pm["plenary_id"] == plenaryID {
			found = true
			if pm["closed"] != true {
				t.Fatal("expected closed=true")
			}
		}
	}
	if !found {
		t.Fatal("plenary not in list")
	}

	// 10. Events
	eventsResp := apiGetArray(t, fmt.Sprintf("%s/api/plenaries/%s/events", baseURL, plenaryID))
	if len(eventsResp) < 8 {
		t.Fatalf("expected >= 8 events, got %d", len(eventsResp))
	}
}

func TestServeBlock(t *testing.T) {
	binary := buildBinary(t)
	tmpDir := t.TempDir()
	storePath := tmpDir + "/events.jsonl"
	port := "18923"

	cmd := startServer(t, binary, storePath, port)
	defer cmd.Process.Kill()

	baseURL := fmt.Sprintf("http://localhost:%s", port)
	actor := map[string]string{"actor_id": "agent-1", "actor_type": "agent"}

	// Create + join + phases + propose
	createResp := apiPost(t, baseURL+"/api/plenaries", map[string]any{
		"actor": actor, "topic": "Block test", "decision_rule": "unanimity",
	})
	pid := createResp["plenary_id"].(string)
	apiPost(t, fmt.Sprintf("%s/api/plenaries/%s/join", baseURL, pid), map[string]any{"actor": actor})
	apiPost(t, fmt.Sprintf("%s/api/plenaries/%s/phase", baseURL, pid), map[string]any{"actor": actor, "to": "proposal", "from": "framing"})
	propResp := apiPost(t, fmt.Sprintf("%s/api/plenaries/%s/propose", baseURL, pid), map[string]any{"actor": actor, "text": "Bad idea"})
	propID := propResp["proposal_id"].(string)
	apiPost(t, fmt.Sprintf("%s/api/plenaries/%s/phase", baseURL, pid), map[string]any{"actor": actor, "to": "consensus_check", "from": "proposal"})

	// Block
	blockResp := apiPost(t, fmt.Sprintf("%s/api/plenaries/%s/block", baseURL, pid), map[string]any{
		"actor":       actor,
		"proposal_id": propID,
		"text":        "This violates our principles",
	})
	assertEqual(t, blockResp["status"], "block_raised")

	// Status should show block
	status := apiGet(t, fmt.Sprintf("%s/api/plenaries/%s", baseURL, pid))
	blocks := status["unresolved_blocks"].([]any)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
}

func TestServeValidationErrors(t *testing.T) {
	binary := buildBinary(t)
	tmpDir := t.TempDir()
	storePath := tmpDir + "/events.jsonl"
	port := "18924"

	cmd := startServer(t, binary, storePath, port)
	defer cmd.Process.Kill()

	baseURL := fmt.Sprintf("http://localhost:%s", port)

	// Missing actor
	_, code := apiPostRaw(t, baseURL+"/api/plenaries", map[string]any{"topic": "No actor"})
	if code != 400 {
		t.Fatalf("expected 400, got %d", code)
	}

	// Not found
	_, code2 := apiGetRaw(t, baseURL+"/api/plenaries/nonexistent")
	if code2 != 404 {
		t.Fatalf("expected 404, got %d", code2)
	}
}

// --- helpers ---

func assertEqual(t *testing.T, got, want any) {
	t.Helper()
	if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func apiPost(t *testing.T, url string, body map[string]any) map[string]any {
	t.Helper()
	resp, code := apiPostRaw(t, url, body)
	if code >= 400 {
		t.Fatalf("POST %s: %d %s", url, code, resp)
	}
	var result map[string]any
	json.Unmarshal([]byte(resp), &result)
	return result
}

func apiPostRaw(t *testing.T, url string, body map[string]any) (string, int) {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	buf.ReadFrom(resp.Body)
	return buf.String(), resp.StatusCode
}

func apiGet(t *testing.T, url string) map[string]any {
	t.Helper()
	body, code := apiGetRaw(t, url)
	if code >= 400 {
		t.Fatalf("GET %s: %d %s", url, code, body)
	}
	var result map[string]any
	json.Unmarshal([]byte(body), &result)
	return result
}

func apiGetArray(t *testing.T, url string) []any {
	t.Helper()
	body, code := apiGetRaw(t, url)
	if code >= 400 {
		t.Fatalf("GET %s: %d %s", url, code, body)
	}
	var result []any
	json.Unmarshal([]byte(body), &result)
	return result
}

func apiGetRaw(t *testing.T, url string) (string, int) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	buf.ReadFrom(resp.Body)
	return buf.String(), resp.StatusCode
}
