package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func startServeForSSETest(t *testing.T, bin, storePath, port string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(bin, "serve", "--port", port)
	cmd.Env = append(os.Environ(), "PLENARY_DB="+storePath)
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start serve: %v", err)
	}

	baseURL := fmt.Sprintf("http://127.0.0.1:%s", port)
	for i := 0; i < 50; i++ {
		resp, err := http.Get(baseURL + "/api/plenaries")
		if err == nil {
			resp.Body.Close()
			return cmd
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = cmd.Process.Kill()
	t.Fatal("serve did not start in time")
	return nil
}

func postJSONForSSETest(t *testing.T, url string, body map[string]any) map[string]any {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		t.Fatalf("POST %s: status %d", url, resp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode POST %s: %v", url, err)
	}
	return out
}

func openSSEForSSETest(t *testing.T, url string) (*http.Response, *bufio.Reader) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	if resp.StatusCode >= 400 {
		t.Fatalf("GET %s: status %d", url, resp.StatusCode)
	}
	return resp, bufio.NewReader(resp.Body)
}

func readNextSSEDataForSSETest(t *testing.T, br *bufio.Reader, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_ = br
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatalf("read SSE line: %v", err)
		}
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "event:") {
			continue
		}
		if strings.HasPrefix(line, "data: ") {
			return strings.TrimPrefix(line, "data: ")
		}
	}
	t.Fatal("timed out waiting for SSE data")
	return ""
}

func TestServeGlobalSSEStreamsNewEvents(t *testing.T) {
	bin := buildBinary(t)
	storePath := t.TempDir() + "/events.jsonl"
	port := "18925"
	cmd := startServeForSSETest(t, bin, storePath, port)
	defer func() { _ = cmd.Process.Kill() }()

	baseURL := fmt.Sprintf("http://127.0.0.1:%s", port)
	actor := map[string]string{"actor_id": "sse-agent", "actor_type": "agent"}

	resp, br := openSSEForSSETest(t, baseURL+"/api/stream")
	defer resp.Body.Close()

	// First message is the "connected" event payload.
	_ = readNextSSEDataForSSETest(t, br, 2*time.Second)

	create := postJSONForSSETest(t, baseURL+"/api/plenaries", map[string]any{
		"actor": actor,
		"topic": "Global SSE test",
	})
	pid := create["plenary_id"].(string)

	data := readNextSSEDataForSSETest(t, br, 2*time.Second)
	var evt map[string]any
	if err := json.Unmarshal([]byte(data), &evt); err != nil {
		t.Fatalf("unmarshal event: %v; data=%s", err, data)
	}
	if evt["plenary_id"] != pid {
		t.Fatalf("unexpected plenary_id: got %v want %s", evt["plenary_id"], pid)
	}
	if evt["event_type"] != "plenary.created" {
		t.Fatalf("unexpected event_type: got %v", evt["event_type"])
	}
}

func TestServePerPlenarySSEReplaysAndFilters(t *testing.T) {
	bin := buildBinary(t)
	storePath := t.TempDir() + "/events.jsonl"
	port := "18926"
	cmd := startServeForSSETest(t, bin, storePath, port)
	defer func() { _ = cmd.Process.Kill() }()

	baseURL := fmt.Sprintf("http://127.0.0.1:%s", port)
	actor := map[string]string{"actor_id": "sse-agent", "actor_type": "agent"}

	createA := postJSONForSSETest(t, baseURL+"/api/plenaries", map[string]any{
		"actor": actor,
		"topic": "A",
	})
	pidA := createA["plenary_id"].(string)
	_ = postJSONForSSETest(t, fmt.Sprintf("%s/api/plenaries/%s/join", baseURL, pidA), map[string]any{
		"actor": actor,
	})

	createB := postJSONForSSETest(t, baseURL+"/api/plenaries", map[string]any{
		"actor": actor,
		"topic": "B",
	})
	pidB := createB["plenary_id"].(string)

	resp, br := openSSEForSSETest(t, fmt.Sprintf("%s/api/plenaries/%s/stream", baseURL, pidA))
	defer resp.Body.Close()

	// Initial replay should include plenary A events before any new ones.
	first := readNextSSEDataForSSETest(t, br, 2*time.Second)
	second := readNextSSEDataForSSETest(t, br, 2*time.Second)

	var evt1, evt2 map[string]any
	if err := json.Unmarshal([]byte(first), &evt1); err != nil {
		t.Fatalf("unmarshal first replay event: %v", err)
	}
	if err := json.Unmarshal([]byte(second), &evt2); err != nil {
		t.Fatalf("unmarshal second replay event: %v", err)
	}
	if evt1["plenary_id"] != pidA || evt2["plenary_id"] != pidA {
		t.Fatalf("replay events should be filtered to pidA; got %v and %v", evt1["plenary_id"], evt2["plenary_id"])
	}

	// New event for another plenary should NOT appear on pidA stream.
	_ = postJSONForSSETest(t, fmt.Sprintf("%s/api/plenaries/%s/join", baseURL, pidB), map[string]any{
		"actor": map[string]string{"actor_id": "other-agent", "actor_type": "agent"},
	})

	// New event for pidA should appear.
	_ = postJSONForSSETest(t, fmt.Sprintf("%s/api/plenaries/%s/speak", baseURL, pidA), map[string]any{
		"actor": actor,
		"text":  "hello stream",
	})
	data := readNextSSEDataForSSETest(t, br, 2*time.Second)
	var evt3 map[string]any
	if err := json.Unmarshal([]byte(data), &evt3); err != nil {
		t.Fatalf("unmarshal streamed event: %v", err)
	}
	if evt3["plenary_id"] != pidA {
		t.Fatalf("expected pidA streamed event, got %v", evt3["plenary_id"])
	}
	if evt3["event_type"] != "speak" {
		t.Fatalf("expected speak event, got %v", evt3["event_type"])
	}
}

