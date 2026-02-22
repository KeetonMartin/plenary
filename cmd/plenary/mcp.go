package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	plenary "github.com/keetonmartin/plenary/internal/plenary"
)

// JSON-RPC types
type jsonrpcRequest struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any            `json:"result,omitempty"`
	Error   *jsonrpcError  `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolCallResult struct {
	Content []contentItem `json:"content"`
	IsError bool          `json:"isError"`
}

type contentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema"`
}

var writeMu sync.Mutex

func writeJSONRPC(resp jsonrpcResponse) {
	b, _ := json.Marshal(resp)
	writeMu.Lock()
	defer writeMu.Unlock()
	os.Stdout.Write(b)
	os.Stdout.Write([]byte("\n"))
}

func cmdMCPServe(store *plenary.JSONLStore, args []string) error {
	fmt.Fprintf(os.Stderr, "plenary-mcp: starting stdio server\n")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		var req jsonrpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			writeJSONRPC(jsonrpcResponse{
				JSONRPC: "2.0",
				Error:   &jsonrpcError{Code: -32700, Message: "parse error: " + err.Error()},
			})
			continue
		}

		if req.ID == nil {
			// Notification — no response
			fmt.Fprintf(os.Stderr, "plenary-mcp: notification: %s\n", req.Method)
			continue
		}

		handleMCPRequest(store, req)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stdin read error: %w", err)
	}
	return nil
}

func handleMCPRequest(store *plenary.JSONLStore, req jsonrpcRequest) {
	id := *req.ID

	respond := func(result any) {
		writeJSONRPC(jsonrpcResponse{JSONRPC: "2.0", ID: id, Result: result})
	}
	respondErr := func(code int, msg string) {
		writeJSONRPC(jsonrpcResponse{
			JSONRPC: "2.0", ID: id,
			Error: &jsonrpcError{Code: code, Message: msg},
		})
	}

	switch req.Method {
	case "initialize":
		respond(map[string]any{
			"protocolVersion": "2025-11-25",
			"capabilities": map[string]any{
				"tools": map[string]any{"listChanged": false},
			},
			"serverInfo": map[string]any{
				"name":    "plenary-mcp",
				"version": "0.1.0",
			},
		})

	case "tools/list":
		respond(map[string]any{"tools": mcpToolDefs()})

	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			respondErr(-32602, "invalid params: "+err.Error())
			return
		}
		result, isErr := mcpDispatch(store, p.Name, p.Arguments)
		respond(toolCallResult{
			Content: []contentItem{{Type: "text", Text: result}},
			IsError: isErr,
		})

	case "ping":
		respond(map[string]any{})

	default:
		respondErr(-32601, "method not found: "+req.Method)
	}
}

func mcpToolDefs() []toolDef {
	return []toolDef{
		{
			Name:        "plenary_create",
			Description: "Create a new plenary deliberation session. Returns plenary_id.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"topic":            map[string]any{"type": "string", "description": "Topic to deliberate on"},
					"context":          map[string]any{"type": "string", "description": "Background context"},
					"decision_rule":    map[string]any{"type": "string", "enum": []string{"unanimity", "quorum", "timeboxed"}, "description": "Decision rule (default: unanimity)"},
					"deadline":         map[string]any{"type": "string", "description": "ISO 8601 deadline (required for timeboxed rule)"},
					"quorum_threshold": map[string]any{"type": "integer", "description": "Quorum percentage 1-100 (default: 50, used with quorum rule)", "minimum": 1, "maximum": 100},
				},
				"required":             []string{"topic"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "plenary_join",
			Description: "Join an existing plenary as a participant.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"plenary_id": map[string]any{"type": "string", "description": "Plenary session ID"},
					"role":       map[string]any{"type": "string", "description": "Your role in this deliberation"},
					"lens":       map[string]any{"type": "string", "description": "Your perspective/lens"},
				},
				"required":             []string{"plenary_id"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "plenary_speak",
			Description: "Make a freeform contribution to a plenary deliberation.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"plenary_id": map[string]any{"type": "string", "description": "Plenary session ID"},
					"text":       map[string]any{"type": "string", "description": "Your message"},
				},
				"required":             []string{"plenary_id", "text"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "plenary_propose",
			Description: "Create a formal proposal for the group to consider. The plenary must be in 'proposal' phase.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"plenary_id": map[string]any{"type": "string", "description": "Plenary session ID"},
					"text":       map[string]any{"type": "string", "description": "The proposal text"},
				},
				"required":             []string{"plenary_id", "text"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "plenary_consent",
			Description: "Consent to the active proposal in a plenary. The plenary must be in 'consensus_check' phase.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"plenary_id":  map[string]any{"type": "string", "description": "Plenary session ID"},
					"proposal_id": map[string]any{"type": "string", "description": "Proposal ID (omit to use active proposal)"},
					"reason":      map[string]any{"type": "string", "description": "Reason for consent"},
				},
				"required":             []string{"plenary_id"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "plenary_block",
			Description: "Raise a block against the active proposal. Use only for principled objections.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"plenary_id":  map[string]any{"type": "string", "description": "Plenary session ID"},
					"proposal_id": map[string]any{"type": "string", "description": "Proposal ID (omit to use active proposal)"},
					"reason":      map[string]any{"type": "string", "description": "Why you are blocking"},
				},
				"required":             []string{"plenary_id", "reason"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "plenary_stand_aside",
			Description: "Stand aside from the active proposal (disagree but won't block consensus).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"plenary_id":  map[string]any{"type": "string", "description": "Plenary session ID"},
					"proposal_id": map[string]any{"type": "string", "description": "Proposal ID (omit to use active proposal)"},
					"reason":      map[string]any{"type": "string", "description": "Reason for standing aside"},
				},
				"required":             []string{"plenary_id", "reason"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "plenary_phase",
			Description: "Transition a plenary to a new phase. Valid sequence: framing -> divergence -> proposal -> consensus_check.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"plenary_id": map[string]any{"type": "string", "description": "Plenary session ID"},
					"to":         map[string]any{"type": "string", "description": "Target phase"},
					"from":       map[string]any{"type": "string", "description": "Expected current phase (safety check)"},
				},
				"required":             []string{"plenary_id", "to", "from"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "plenary_close",
			Description: "Close the plenary with a decision and resolution.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"plenary_id": map[string]any{"type": "string", "description": "Plenary session ID"},
					"resolution": map[string]any{"type": "string", "description": "Summary of the decision"},
					"outcome":    map[string]any{"type": "string", "enum": []string{"consensus", "owner_decision", "abandoned"}, "description": "Decision outcome"},
				},
				"required":             []string{"plenary_id", "resolution"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "plenary_status",
			Description: "Get the current state of a plenary: phase, participants, proposals, blocks, stances.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"plenary_id": map[string]any{"type": "string", "description": "Plenary session ID"},
				},
				"required":             []string{"plenary_id"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "plenary_list",
			Description: "List all plenaries in the store with topic, phase, and participant count.",
			InputSchema: map[string]any{
				"type":                 "object",
				"properties":           map[string]any{},
				"additionalProperties": false,
			},
		},
	}
}

func mcpDispatch(store *plenary.JSONLStore, name string, rawArgs json.RawMessage) (string, bool) {
	var args map[string]any
	if len(rawArgs) > 0 {
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return "invalid arguments: " + err.Error(), true
		}
	}
	if args == nil {
		args = map[string]any{}
	}

	getString := func(key string) string {
		v, _ := args[key].(string)
		return v
	}

	actor := mcpActor()

	switch name {
	case "plenary_create":
		return mcpCreate(store, actor, args, getString)
	case "plenary_join":
		return mcpJoin(store, actor, getString)
	case "plenary_speak":
		return mcpSpeak(store, actor, getString)
	case "plenary_propose":
		return mcpPropose(store, actor, getString)
	case "plenary_consent":
		return mcpConsent(store, actor, getString)
	case "plenary_block":
		return mcpBlock(store, actor, getString)
	case "plenary_stand_aside":
		return mcpStandAside(store, actor, getString)
	case "plenary_phase":
		return mcpPhase(store, actor, getString)
	case "plenary_close":
		return mcpClose(store, actor, getString)
	case "plenary_status":
		return mcpStatus(store, getString)
	case "plenary_list":
		return mcpList(store)
	default:
		return "unknown tool: " + name, true
	}
}

func mcpActor() plenary.Actor {
	id := os.Getenv("PLENARY_ACTOR_ID")
	if id == "" {
		id = "mcp-agent"
	}
	typ := os.Getenv("PLENARY_ACTOR_TYPE")
	if typ == "" {
		typ = "agent"
	}
	if typ == "ai" {
		typ = "agent"
	}
	return plenary.Actor{ActorID: id, ActorType: typ}
}

func mcpJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func mcpAppend(store *plenary.JSONLStore, evt plenary.Event) (string, bool) {
	events, err := store.ListByPlenary(evt.PlenaryID)
	if err != nil {
		return "store error: " + err.Error(), true
	}
	_, err = plenary.ReduceWithValidation(events, evt)
	if err != nil {
		return err.Error(), true
	}
	if err := store.Append(evt); err != nil {
		return "store error: " + err.Error(), true
	}
	return "", false
}

func mcpCreate(store *plenary.JSONLStore, actor plenary.Actor, args map[string]any, getString func(string) string) (string, bool) {
	topic := getString("topic")
	if topic == "" {
		return "topic is required", true
	}
	rule := getString("decision_rule")
	if rule == "" {
		rule = "unanimity"
	}

	payload := map[string]any{
		"topic":         topic,
		"decision_rule": rule,
	}
	if ctx := getString("context"); ctx != "" {
		payload["context"] = ctx
	}
	if dl := getString("deadline"); dl != "" {
		payload["deadline"] = dl
	}
	if qt, ok := args["quorum_threshold"]; ok {
		if v, ok := qt.(float64); ok && v >= 1 && v <= 100 {
			threshold := int(v)
			payload["quorum_threshold"] = threshold
		}
	}

	plenaryID := plenary.NewUUIDLike()
	evt, err := plenary.NewEvent(plenaryID, actor, "plenary.created", payload)
	if err != nil {
		return "event creation error: " + err.Error(), true
	}

	if err := store.Append(evt); err != nil {
		return "store error: " + err.Error(), true
	}
	return mcpJSON(map[string]any{
		"plenary_id": evt.PlenaryID,
		"status":     "created",
		"topic":      topic,
	}), false
}

func mcpJoin(store *plenary.JSONLStore, actor plenary.Actor, getString func(string) string) (string, bool) {
	pid := getString("plenary_id")
	if pid == "" {
		return "plenary_id is required", true
	}

	payload := map[string]any{}
	if role := getString("role"); role != "" {
		payload["role"] = role
	}
	if lens := getString("lens"); lens != "" {
		payload["lens"] = lens
	}

	evt, err := plenary.NewEvent(pid, actor, "participant.joined", payload)
	if err != nil {
		return "event creation error: " + err.Error(), true
	}

	if errMsg, isErr := mcpAppend(store, evt); isErr {
		return errMsg, true
	}
	return mcpJSON(map[string]any{"status": "joined", "plenary_id": pid}), false
}

func mcpSpeak(store *plenary.JSONLStore, actor plenary.Actor, getString func(string) string) (string, bool) {
	pid := getString("plenary_id")
	if pid == "" {
		return "plenary_id is required", true
	}
	text := getString("text")
	if text == "" {
		return "text is required", true
	}

	evt, err := plenary.NewEvent(pid, actor, "speak", map[string]any{"text": text})
	if err != nil {
		return "event creation error: " + err.Error(), true
	}

	if errMsg, isErr := mcpAppend(store, evt); isErr {
		return errMsg, true
	}
	return mcpJSON(map[string]any{"status": "spoke", "plenary_id": pid}), false
}

func mcpPropose(store *plenary.JSONLStore, actor plenary.Actor, getString func(string) string) (string, bool) {
	pid := getString("plenary_id")
	if pid == "" {
		return "plenary_id is required", true
	}
	text := getString("text")
	if text == "" {
		return "text is required", true
	}

	proposalID := plenary.NewUUIDLike()
	evt, err := plenary.NewEvent(pid, actor, "proposal.created", map[string]any{"proposal_id": proposalID, "text": text})
	if err != nil {
		return "event creation error: " + err.Error(), true
	}

	if errMsg, isErr := mcpAppend(store, evt); isErr {
		return errMsg, true
	}
	return mcpJSON(map[string]any{"status": "proposed", "plenary_id": pid, "proposal_id": proposalID}), false
}

func mcpConsent(store *plenary.JSONLStore, actor plenary.Actor, getString func(string) string) (string, bool) {
	pid := getString("plenary_id")
	if pid == "" {
		return "plenary_id is required", true
	}
	proposalID := getString("proposal_id")

	// Auto-resolve active proposal if not specified
	if proposalID == "" {
		events, err := store.ListByPlenary(pid)
		if err != nil {
			return "store error: " + err.Error(), true
		}
		snap, err := plenary.Reduce(events)
		if err != nil {
			return err.Error(), true
		}
		if snap.ActiveProposal != nil {
			proposalID = snap.ActiveProposal.ProposalID
		}
		if proposalID == "" {
			return "no active proposal; provide proposal_id", true
		}
	}

	payload := map[string]any{"proposal_id": proposalID}
	if reason := getString("reason"); reason != "" {
		payload["reason"] = reason
	}
	evt, err := plenary.NewEvent(pid, actor, "consent.given", payload)
	if err != nil {
		return "event creation error: " + err.Error(), true
	}

	if errMsg, isErr := mcpAppend(store, evt); isErr {
		return errMsg, true
	}
	return mcpJSON(map[string]any{"status": "consent_given", "plenary_id": pid, "proposal_id": proposalID}), false
}

func mcpBlock(store *plenary.JSONLStore, actor plenary.Actor, getString func(string) string) (string, bool) {
	pid := getString("plenary_id")
	if pid == "" {
		return "plenary_id is required", true
	}
	reason := getString("reason")
	if reason == "" {
		return "reason is required", true
	}
	proposalID := getString("proposal_id")

	if proposalID == "" {
		events, err := store.ListByPlenary(pid)
		if err != nil {
			return "store error: " + err.Error(), true
		}
		snap, err := plenary.Reduce(events)
		if err != nil {
			return err.Error(), true
		}
		if snap.ActiveProposal != nil {
			proposalID = snap.ActiveProposal.ProposalID
		}
		if proposalID == "" {
			return "no active proposal; provide proposal_id", true
		}
	}

	evt, err := plenary.NewEvent(pid, actor, "block.raised", map[string]any{"proposal_id": proposalID, "text": reason})
	if err != nil {
		return "event creation error: " + err.Error(), true
	}

	if errMsg, isErr := mcpAppend(store, evt); isErr {
		return errMsg, true
	}
	return mcpJSON(map[string]any{"status": "block_raised", "plenary_id": pid, "proposal_id": proposalID}), false
}

func mcpStandAside(store *plenary.JSONLStore, actor plenary.Actor, getString func(string) string) (string, bool) {
	pid := getString("plenary_id")
	if pid == "" {
		return "plenary_id is required", true
	}
	reason := getString("reason")
	if reason == "" {
		return "reason is required", true
	}
	proposalID := getString("proposal_id")

	if proposalID == "" {
		events, err := store.ListByPlenary(pid)
		if err != nil {
			return "store error: " + err.Error(), true
		}
		snap, err := plenary.Reduce(events)
		if err != nil {
			return err.Error(), true
		}
		if snap.ActiveProposal != nil {
			proposalID = snap.ActiveProposal.ProposalID
		}
		if proposalID == "" {
			return "no active proposal; provide proposal_id", true
		}
	}

	evt, err := plenary.NewEvent(pid, actor, "stand_aside.given", map[string]any{"proposal_id": proposalID, "reason": reason})
	if err != nil {
		return "event creation error: " + err.Error(), true
	}

	if errMsg, isErr := mcpAppend(store, evt); isErr {
		return errMsg, true
	}
	return mcpJSON(map[string]any{"status": "stood_aside", "plenary_id": pid, "proposal_id": proposalID}), false
}

func mcpPhase(store *plenary.JSONLStore, actor plenary.Actor, getString func(string) string) (string, bool) {
	pid := getString("plenary_id")
	if pid == "" {
		return "plenary_id is required", true
	}
	to := getString("to")
	from := getString("from")
	if to == "" || from == "" {
		return "to and from are required", true
	}

	evt, err := plenary.NewEvent(pid, actor, "phase.set", plenary.PhaseSetPayload{Phase: plenary.Phase(to), ExpectedPhase: plenary.Phase(from)})
	if err != nil {
		return "event creation error: " + err.Error(), true
	}

	if errMsg, isErr := mcpAppend(store, evt); isErr {
		return errMsg, true
	}
	return mcpJSON(map[string]any{"status": "phase_changed", "plenary_id": pid, "phase": to}), false
}

func mcpClose(store *plenary.JSONLStore, actor plenary.Actor, getString func(string) string) (string, bool) {
	pid := getString("plenary_id")
	if pid == "" {
		return "plenary_id is required", true
	}
	resolution := getString("resolution")
	if resolution == "" {
		return "resolution is required", true
	}
	outcome := getString("outcome")
	if outcome == "" {
		outcome = "consensus"
	}

	// Build decision record from current state (matching serve.go behavior)
	events, err := store.ListByPlenary(pid)
	if err != nil {
		return "store error: " + err.Error(), true
	}
	snap, err := plenary.Reduce(events)
	if err != nil {
		return err.Error(), true
	}

	participants := make([]plenary.DecisionRecordParticipant, len(snap.Participants))
	for i, p := range snap.Participants {
		participants[i] = plenary.DecisionRecordParticipant{
			ActorID:     p.ActorID,
			ActorType:   p.ActorType,
			Role:        p.Role,
			FinalStance: p.Stance,
			FinalReason: p.FinalReason,
		}
	}

	payload := plenary.DecisionClosedPayload{
		Outcome: plenary.Outcome(outcome),
		DecisionRecord: plenary.DecisionRecord{
			Resolution:   resolution,
			Participants: participants,
		},
	}
	evt, err := plenary.NewEvent(pid, actor, "decision.closed", payload)
	if err != nil {
		return "event creation error: " + err.Error(), true
	}

	if errMsg, isErr := mcpAppend(store, evt); isErr {
		return errMsg, true
	}
	return mcpJSON(map[string]any{"status": "closed", "plenary_id": pid, "outcome": outcome}), false
}

func mcpStatus(store *plenary.JSONLStore, getString func(string) string) (string, bool) {
	pid := getString("plenary_id")
	if pid == "" {
		return "plenary_id is required", true
	}

	events, err := store.ListByPlenary(pid)
	if err != nil {
		return "store error: " + err.Error(), true
	}
	if len(events) == 0 {
		return "plenary not found: " + pid, true
	}

	snap, err := plenary.Reduce(events)
	if err != nil {
		return err.Error(), true
	}
	return mcpJSON(snap), false
}

func mcpList(store *plenary.JSONLStore) (string, bool) {
	events, err := store.ListAll()
	if err != nil {
		return "store error: " + err.Error(), true
	}

	grouped := map[string][]plenary.Event{}
	for _, evt := range events {
		grouped[evt.PlenaryID] = append(grouped[evt.PlenaryID], evt)
	}

	type summary struct {
		PlenaryID string `json:"plenary_id"`
		Topic     string `json:"topic"`
		Phase     string `json:"phase"`
		Rule      string `json:"decision_rule"`
		Closed    bool   `json:"closed"`
		Events    int    `json:"event_count"`
	}

	summaries := make([]summary, 0, len(grouped))
	for pid, evts := range grouped {
		snap, err := plenary.Reduce(evts)
		if err != nil {
			continue
		}
		summaries = append(summaries, summary{
			PlenaryID: pid,
			Topic:     snap.Topic,
			Phase:     string(snap.Phase),
			Rule:      string(snap.DecisionRule),
			Closed:    snap.Closed,
			Events:    snap.EventCount,
		})
	}
	return mcpJSON(summaries), false
}

