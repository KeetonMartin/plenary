package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"
)

type mcpRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func startMCP(t *testing.T, bin, storePath string) (io.WriteCloser, *bufio.Scanner, *exec.Cmd) {
	t.Helper()
	cmd := exec.Command(bin, "mcp-serve")
	cmd.Env = append(os.Environ(),
		"PLENARY_DB="+storePath,
		"PLENARY_ACTOR_ID=test-agent",
		"PLENARY_ACTOR_TYPE=agent",
	)
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("start mcp: %v", err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)

	return stdin, scanner, cmd
}

func mcpCall(t *testing.T, stdin io.Writer, scanner *bufio.Scanner, id int, method string, params any) mcpResponse {
	t.Helper()
	req := mcpRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	b, _ := json.Marshal(req)
	if _, err := fmt.Fprintf(stdin, "%s\n", b); err != nil {
		t.Fatalf("write to mcp: %v", err)
	}
	if !scanner.Scan() {
		t.Fatalf("no response from mcp (method=%s)", method)
	}
	var resp mcpResponse
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v\nraw: %s", err, scanner.Text())
	}
	return resp
}

func TestMCPInitializeAndToolsList(t *testing.T) {
	bin := buildBinary(t)
	storePath := t.TempDir() + "/events.jsonl"

	stdin, scanner, cmd := startMCP(t, bin, storePath)
	defer cmd.Process.Kill()

	// Initialize
	resp := mcpCall(t, stdin, scanner, 1, "initialize", map[string]any{
		"protocolVersion": "2025-11-25",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test", "version": "1.0"},
	})
	if resp.Error != nil {
		t.Fatalf("initialize error: %v", resp.Error)
	}

	var initResult map[string]any
	json.Unmarshal(resp.Result, &initResult)
	assertEqual(t, initResult["protocolVersion"], "2025-11-25")

	serverInfo := initResult["serverInfo"].(map[string]any)
	assertEqual(t, serverInfo["name"], "plenary-mcp")

	// Send initialized notification (no response expected)
	notif, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})
	fmt.Fprintf(stdin, "%s\n", notif)

	// Tools list
	resp2 := mcpCall(t, stdin, scanner, 2, "tools/list", map[string]any{})
	if resp2.Error != nil {
		t.Fatalf("tools/list error: %v", resp2.Error)
	}

	var listResult map[string]any
	json.Unmarshal(resp2.Result, &listResult)
	tools := listResult["tools"].([]any)
	if len(tools) < 10 {
		t.Fatalf("expected >= 10 tools, got %d", len(tools))
	}

	// Check first tool has correct structure
	tool0 := tools[0].(map[string]any)
	if tool0["name"] == nil || tool0["description"] == nil || tool0["inputSchema"] == nil {
		t.Fatal("tool missing required fields")
	}

	stdin.Close()
}

func TestMCPFullLifecycle(t *testing.T) {
	bin := buildBinary(t)
	storePath := t.TempDir() + "/events.jsonl"

	stdin, scanner, cmd := startMCP(t, bin, storePath)
	defer cmd.Process.Kill()

	// Initialize
	mcpCall(t, stdin, scanner, 1, "initialize", map[string]any{
		"protocolVersion": "2025-11-25",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test", "version": "1.0"},
	})

	id := 2

	callTool := func(name string, args map[string]any) map[string]any {
		t.Helper()
		resp := mcpCall(t, stdin, scanner, id, "tools/call", map[string]any{
			"name":      name,
			"arguments": args,
		})
		id++
		if resp.Error != nil {
			t.Fatalf("tool %s RPC error: %v", name, resp.Error)
		}
		var result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		}
		json.Unmarshal(resp.Result, &result)
		if result.IsError {
			t.Fatalf("tool %s returned error: %s", name, result.Content[0].Text)
		}
		var data map[string]any
		json.Unmarshal([]byte(result.Content[0].Text), &data)
		return data
	}

	// 1. Create
	createResult := callTool("plenary_create", map[string]any{
		"topic":         "MCP test plenary",
		"decision_rule": "unanimity",
	})
	plenaryID := createResult["plenary_id"].(string)
	if plenaryID == "" {
		t.Fatal("expected plenary_id")
	}

	// 2. Join
	joinResult := callTool("plenary_join", map[string]any{"plenary_id": plenaryID})
	assertEqual(t, joinResult["status"], "joined")

	// 3. Speak
	speakResult := callTool("plenary_speak", map[string]any{"plenary_id": plenaryID, "text": "Hello from MCP"})
	assertEqual(t, speakResult["status"], "spoke")

	// 4. Phase -> proposal
	callTool("plenary_phase", map[string]any{"plenary_id": plenaryID, "to": "divergence", "from": "framing"})
	callTool("plenary_phase", map[string]any{"plenary_id": plenaryID, "to": "proposal", "from": "divergence"})

	// 5. Propose
	propResult := callTool("plenary_propose", map[string]any{"plenary_id": plenaryID, "text": "MCP proposal"})
	proposalID := propResult["proposal_id"].(string)
	if proposalID == "" {
		t.Fatal("expected proposal_id")
	}

	// 6. Phase -> consensus_check
	callTool("plenary_phase", map[string]any{"plenary_id": plenaryID, "to": "consensus_check", "from": "proposal"})

	// 7. Consent
	consentResult := callTool("plenary_consent", map[string]any{"plenary_id": plenaryID, "proposal_id": proposalID, "reason": "LGTM"})
	assertEqual(t, consentResult["status"], "consent_given")

	// 8. Close
	closeResult := callTool("plenary_close", map[string]any{"plenary_id": plenaryID, "resolution": "MCP works", "outcome": "consensus"})
	assertEqual(t, closeResult["status"], "closed")

	// 9. Status
	statusResult := callTool("plenary_status", map[string]any{"plenary_id": plenaryID})
	assertEqual(t, statusResult["topic"], "MCP test plenary")
	assertEqual(t, statusResult["closed"], true)

	// 10. List
	listResp := mcpCall(t, stdin, scanner, id, "tools/call", map[string]any{
		"name":      "plenary_list",
		"arguments": map[string]any{},
	})
	var listCallResult struct {
		Content []struct{ Text string } `json:"content"`
		IsError bool                    `json:"isError"`
	}
	json.Unmarshal(listResp.Result, &listCallResult)
	if listCallResult.IsError {
		t.Fatal("list returned error")
	}
	var summaries []map[string]any
	json.Unmarshal([]byte(listCallResult.Content[0].Text), &summaries)
	if len(summaries) != 1 {
		t.Fatalf("expected 1 plenary in list, got %d", len(summaries))
	}

	stdin.Close()
}

func TestMCPToolErrors(t *testing.T) {
	bin := buildBinary(t)
	storePath := t.TempDir() + "/events.jsonl"

	stdin, scanner, cmd := startMCP(t, bin, storePath)
	defer cmd.Process.Kill()

	mcpCall(t, stdin, scanner, 1, "initialize", map[string]any{
		"protocolVersion": "2025-11-25",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test", "version": "1.0"},
	})

	// Unknown tool -> should still return a result with isError
	resp := mcpCall(t, stdin, scanner, 2, "tools/call", map[string]any{
		"name":      "plenary_nonexistent",
		"arguments": map[string]any{},
	})
	if resp.Error != nil {
		t.Fatalf("expected tool-level error, got RPC error: %v", resp.Error)
	}
	var result struct {
		Content []struct{ Text string } `json:"content"`
		IsError bool                    `json:"isError"`
	}
	json.Unmarshal(resp.Result, &result)
	if !result.IsError {
		t.Fatal("expected isError=true for unknown tool")
	}

	// Missing required field
	resp2 := mcpCall(t, stdin, scanner, 3, "tools/call", map[string]any{
		"name":      "plenary_create",
		"arguments": map[string]any{},
	})
	var result2 struct {
		Content []struct{ Text string } `json:"content"`
		IsError bool                    `json:"isError"`
	}
	json.Unmarshal(resp2.Result, &result2)
	if !result2.IsError {
		t.Fatal("expected isError=true for missing topic")
	}

	// Unknown method -> RPC error
	resp3 := mcpCall(t, stdin, scanner, 4, "nonexistent/method", nil)
	if resp3.Error == nil {
		t.Fatal("expected RPC error for unknown method")
	}
	if resp3.Error.Code != -32601 {
		t.Fatalf("expected error code -32601, got %d", resp3.Error.Code)
	}

	stdin.Close()
}
