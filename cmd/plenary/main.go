package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	plenary "github.com/keetonmartin/plenary/internal/plenary"
)

const defaultStorePath = ".plenary/events.jsonl"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	storePath := os.Getenv("PLENARY_DB")
	if storePath == "" {
		storePath = defaultStorePath
	}
	store := plenary.NewJSONLStore(storePath)

	cmd := os.Args[1]
	args := os.Args[2:]

	// Check for subcommand --help
	for _, a := range args {
		if a == "--help" || a == "-h" {
			if showSubcommandHelp(cmd) {
				return
			}
			break
		}
	}

	var err error
	switch cmd {
	case "create":
		err = cmdCreate(store, args)
	case "join":
		err = cmdJoin(store, args)
	case "list":
		err = cmdList(store, args)
	case "status":
		err = cmdStatus(store, args)
	case "propose":
		err = cmdPropose(store, args)
	case "consent":
		err = cmdConsent(store, args)
	case "block":
		err = cmdBlock(store, args)
	case "stand-aside":
		err = cmdStandAside(store, args)
	case "speak":
		err = cmdSpeak(store, args)
	case "close":
		err = cmdClose(store, args)
	case "phase":
		err = cmdPhase(store, args)
	case "export":
		err = cmdExport(store, args)
	case "tail":
		err = cmdTail(store, args)
	case "wait":
		err = cmdWait(store, args)
	case "web":
		err = cmdWeb(store, args)
	case "serve":
		err = cmdServe(store, args)
	case "mcp-serve":
		err = cmdMCPServe(store, args)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		if plenary.Is(err, plenary.ErrValidation) {
			os.Exit(2)
		}
		if plenary.Is(err, plenary.ErrConflict) {
			os.Exit(3)
		}
		if plenary.Is(err, plenary.ErrNotFound) {
			os.Exit(4)
		}
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`plenary — consensus protocol for agents

Usage: plenary <command> [flags]

Commands:
  create       Create a new plenary
  join         Join an existing plenary
  list         List plenaries in the store
  status       Show derived state of a plenary
  propose      Create a formal proposal
  consent      Consent to the active proposal
  block        Raise a block against the active proposal
  stand-aside  Stand aside (disagree but won't block)
  speak        Freeform contribution
  phase        Transition to a new phase
  close        Close the plenary with a decision
  export       Export plenary artifacts to files
  tail         Stream events for a plenary
  wait         Wait for a condition (phase change, new events, etc.)
  web          Start local web viewer
  serve        Start HTTP API server with SSE
  mcp-serve    Start MCP tool-server (stdio JSON-RPC)

Environment:
  PLENARY_DB          Path to event store (default: .plenary/events.jsonl)
  PLENARY_ACTOR_ID    Your actor ID
  PLENARY_ACTOR_TYPE  Your actor type (human|agent); 'ai' accepted as alias
  PLENARY_ID          Default plenary ID for commands that need one

Run 'plenary <command> --help' for details on a specific command.`)
}

var subcommandHelp = map[string]string{
	"create": `Usage: plenary create --topic <text> [--context <text>] [--decision-rule <rule>] [--deadline <iso8601>]

Create a new plenary deliberation.

Required:
  --topic <text>           The topic or question to deliberate on

Optional:
  --context <text>         Additional context for participants
  --decision-rule <rule>   Decision rule: unanimity (default), quorum, timeboxed
  --deadline <iso8601>     Optional deadline

Aliases: --rule is accepted for --decision-rule`,

	"join": `Usage: plenary join --plenary <id> [--role <text>] [--lens <text>]

Join an existing plenary as a participant.

Required:
  --plenary <id>    Plenary ID to join

Optional:
  --role <text>     Your role in this deliberation
  --lens <text>     Your perspective/lens`,

	"status": `Usage: plenary status --plenary <id>

Show the derived snapshot (current state) of a plenary.

Required:
  --plenary <id>    Plenary ID`,

	"propose": `Usage: plenary propose --plenary <id> --text <text> [--criteria <text>]

Create a formal proposal for the group to consider.

Required:
  --plenary <id>    Plenary ID
  --text <text>     The proposal text

Optional:
  --criteria <text>  Acceptance criteria`,

	"consent": `Usage: plenary consent --plenary <id> --proposal <id> [--reason <text>]

Consent to the active proposal.

Required:
  --plenary <id>     Plenary ID
  --proposal <id>    Proposal ID (from 'plenary status')

Optional:
  --reason <text>    Reason for consenting`,

	"block": `Usage: plenary block --plenary <id> --proposal <id> --reason <text> [--principle <text>] [--failure-mode <text>]

Raise a block against the active proposal.

Required:
  --plenary <id>      Plenary ID
  --proposal <id>     Proposal ID
  --reason <text>     Why you are blocking

Optional:
  --principle <text>      Principle being violated
  --failure-mode <text>   What failure this would cause`,

	"stand-aside": `Usage: plenary stand-aside --plenary <id> --proposal <id> --reason <text>

Stand aside from the active proposal (disagree but won't block consensus).

Required:
  --plenary <id>     Plenary ID
  --proposal <id>    Proposal ID
  --reason <text>    Why you are standing aside`,

	"speak": `Usage: plenary speak --plenary <id> --text <text>

Make a freeform contribution to the deliberation.

Required:
  --plenary <id>    Plenary ID
  --text <text>     Your message

Aliases: --message is accepted for --text`,

	"phase": `Usage: plenary phase --plenary <id> --to <phase> --from <phase>

Transition the plenary to a new phase.

Required:
  --plenary <id>    Plenary ID
  --to <phase>      Target phase (framing, divergence, proposal, consensus_check, closed)
  --from <phase>    Expected current phase (safety check)`,

	"close": `Usage: plenary close --plenary <id> --resolution <text> [--outcome <outcome>]

Close the plenary with a decision.

Required:
  --plenary <id>        Plenary ID
  --resolution <text>   Summary of the decision

Optional:
  --outcome <outcome>   consensus (default), owner_decision, abandoned`,

	"export": `Usage: plenary export --plenary <id> [--out <dir>]

Export plenary artifacts to files (events.jsonl, snapshot.json, transcript.md, decision_record.json).

Required:
  --plenary <id>    Plenary ID

Optional:
  --out <dir>       Output directory (default: .plenary/exports/<id>)`,

	"tail": `Usage: plenary tail --plenary <id> [--follow] [--interval-ms <ms>]

Stream events for a plenary.

Required:
  --plenary <id>    Plenary ID

Optional:
  --follow          Keep watching for new events
  --interval-ms <ms>  Poll interval in milliseconds (default: 500, min: 50)`,

	"wait": `Usage: plenary wait --plenary <id> [--phase <phase>] [--event-type <type>] [--timeout <seconds>]

Wait for a condition to be met, then exit. Useful for agent scripting.

Required:
  --plenary <id>         Plenary ID

Conditions (at least one required):
  --phase <phase>        Wait until plenary reaches this phase
  --event-type <type>    Wait until an event of this type appears
  --events-after <n>     Wait until event count exceeds n

Optional:
  --timeout <seconds>    Max wait time in seconds (default: 300)
  --interval-ms <ms>     Poll interval in ms (default: 500, min: 50)

Exits 0 when condition met. Outputs the triggering event as JSON.
Exits 1 on timeout.`,

	"web": `Usage: plenary web [--port <port>]

Start the local web viewer.

Optional:
  --port <port>    Port to listen on (default: 3000)`,

	"serve": `Usage: plenary serve [--port <port>]

Start the HTTP API server with SSE event streaming.
Agents on any machine can participate via HTTP instead of the CLI.

Endpoints:
  GET  /api/plenaries              List all plenaries
  POST /api/plenaries              Create a new plenary
  GET  /api/plenaries/:id          Get plenary status (snapshot)
  GET  /api/plenaries/:id/events   Get all events
  GET  /api/plenaries/:id/stream   SSE stream for one plenary
  GET  /api/stream                 SSE stream for all plenaries
  POST /api/plenaries/:id/join     Join a plenary
  POST /api/plenaries/:id/speak    Speak
  POST /api/plenaries/:id/propose  Create a proposal
  POST /api/plenaries/:id/consent  Consent to active proposal
  POST /api/plenaries/:id/block    Raise a block
  POST /api/plenaries/:id/stand-aside  Stand aside
  POST /api/plenaries/:id/phase    Transition phase
  POST /api/plenaries/:id/close    Close with decision

Optional:
  --port <port>    Port to listen on (default: 8080)`,

	"mcp-serve": `Usage: plenary mcp-serve

Start the MCP (Model Context Protocol) tool-server over stdio.
Reads JSON-RPC 2.0 from stdin, writes responses to stdout.

Use with Claude Code, Claude Desktop, or any MCP-compatible host.

Environment:
  PLENARY_DB          Path to event store (default: .plenary/events.jsonl)
  PLENARY_ACTOR_ID    Actor identity for all tool calls
  PLENARY_ACTOR_TYPE  Actor type: human or agent

Tools exposed:
  plenary_create, plenary_join, plenary_speak, plenary_propose,
  plenary_consent, plenary_block, plenary_stand_aside, plenary_phase,
  plenary_close, plenary_status, plenary_list`,
}

func showSubcommandHelp(cmd string) bool {
	help, ok := subcommandHelp[cmd]
	if !ok {
		return false
	}
	fmt.Println(help)
	return true
}

// --- Flag parsing helpers ---

func getFlag(args []string, name string) (string, []string) {
	for i, a := range args {
		if a == name && i+1 < len(args) {
			rest := make([]string, 0, len(args)-2)
			rest = append(rest, args[:i]...)
			rest = append(rest, args[i+2:]...)
			return args[i+1], rest
		}
		if strings.HasPrefix(a, name+"=") {
			rest := make([]string, 0, len(args)-1)
			rest = append(rest, args[:i]...)
			rest = append(rest, args[i+1:]...)
			return strings.TrimPrefix(a, name+"="), rest
		}
	}
	return "", args
}

func requireFlag(args []string, name string) (string, []string, error) {
	val, rest := getFlag(args, name)
	if val == "" {
		return "", rest, fmt.Errorf("%w: %s is required", plenary.ErrValidation, name)
	}
	return val, rest, nil
}

func hasFlag(args []string, name string) (bool, []string) {
	for i, a := range args {
		if a == name {
			return true, append(args[:i], args[i+1:]...)
		}
	}
	return false, args
}

func getActor() (plenary.Actor, error) {
	id := os.Getenv("PLENARY_ACTOR_ID")
	typ := os.Getenv("PLENARY_ACTOR_TYPE")
	if id == "" || typ == "" {
		return plenary.Actor{}, fmt.Errorf("%w: set PLENARY_ACTOR_ID and PLENARY_ACTOR_TYPE env vars", plenary.ErrValidation)
	}
	norm, err := normalizeActorType(typ)
	if err != nil {
		return plenary.Actor{}, err
	}
	return plenary.Actor{ActorID: id, ActorType: norm}, nil
}

func printJSON(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

func appendAndValidate(store *plenary.JSONLStore, evt plenary.Event) error {
	events, err := store.ListByPlenary(evt.PlenaryID)
	if err != nil {
		return err
	}
	_, err = plenary.ReduceWithValidation(events, evt)
	if err != nil {
		return err
	}
	return store.Append(evt)
}

func writeJSONFile(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

func writeEventsJSONL(path string, events []plenary.Event) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, evt := range events {
		b, err := json.Marshal(evt)
		if err != nil {
			return err
		}
		if _, err := f.Write(append(b, '\n')); err != nil {
			return err
		}
	}
	return nil
}

func eventPayloadSummary(evt plenary.Event) string {
	switch evt.EventType {
	case "plenary.created":
		var p plenary.PlenaryCreatedPayload
		if json.Unmarshal(evt.Payload, &p) == nil {
			return fmt.Sprintf("topic=%q rule=%s", p.Topic, p.DecisionRule)
		}
	case "participant.joined":
		var p plenary.ParticipantJoinedPayload
		if json.Unmarshal(evt.Payload, &p) == nil {
			return fmt.Sprintf("role=%q lens=%q", strPtr(p.Role), strPtr(p.Lens))
		}
	case "proposal.created":
		var p plenary.ProposalCreatedPayload
		if json.Unmarshal(evt.Payload, &p) == nil {
			return fmt.Sprintf("proposal=%s text=%q", p.ProposalID, p.Text)
		}
	case "consent.given":
		var p plenary.ConsentPayload
		if json.Unmarshal(evt.Payload, &p) == nil {
			return fmt.Sprintf("proposal=%s reason=%q", p.ProposalID, strPtr(p.Reason))
		}
	case "stand_aside.given":
		var p plenary.StandAsidePayload
		if json.Unmarshal(evt.Payload, &p) == nil {
			return fmt.Sprintf("proposal=%s reason=%q", p.ProposalID, p.Reason)
		}
	case "block.raised":
		var p plenary.ProposalRefTextPayload
		if json.Unmarshal(evt.Payload, &p) == nil {
			return fmt.Sprintf("proposal=%s reason=%q", p.ProposalID, p.Text)
		}
	case "speak":
		var p plenary.TextPayload
		if json.Unmarshal(evt.Payload, &p) == nil {
			return fmt.Sprintf("text=%q", p.Text)
		}
	case "phase.set":
		var p plenary.PhaseSetPayload
		if json.Unmarshal(evt.Payload, &p) == nil {
			return fmt.Sprintf("from=%s to=%s", p.ExpectedPhase, p.Phase)
		}
	case "decision.closed":
		var p plenary.DecisionClosedPayload
		if json.Unmarshal(evt.Payload, &p) == nil {
			return fmt.Sprintf("outcome=%s resolution=%q", p.Outcome, p.DecisionRecord.Resolution)
		}
	}
	return string(evt.Payload)
}

func strPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func renderTranscript(events []plenary.Event, snap plenary.Snapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Plenary Transcript\n\n")
	fmt.Fprintf(&b, "- Plenary ID: `%s`\n", snap.PlenaryID)
	fmt.Fprintf(&b, "- Topic: %s\n", snap.Topic)
	fmt.Fprintf(&b, "- Rule: `%s`\n", snap.DecisionRule)
	fmt.Fprintf(&b, "- Phase: `%s`\n", snap.Phase)
	fmt.Fprintf(&b, "- Events: `%d`\n", len(events))
	if snap.Outcome != nil {
		fmt.Fprintf(&b, "- Outcome: `%s`\n", *snap.Outcome)
	}
	b.WriteString("\n## Events\n\n")
	for _, evt := range events {
		fmt.Fprintf(
			&b,
			"- `%s` `%s` `%s` `%s`: %s\n",
			evt.TS,
			evt.Actor.ActorID,
			evt.Actor.ActorType,
			evt.EventType,
			eventPayloadSummary(evt),
		)
	}
	return b.String()
}

func printEventJSONLine(evt plenary.Event) error {
	b, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

func normalizeActorType(typ string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(typ)) {
	case "human":
		return "human", nil
	case "agent", "ai":
		return "agent", nil
	default:
		return "", fmt.Errorf("%w: invalid actor type %q (allowed: human|agent; 'ai' is an alias)", plenary.ErrValidation, typ)
	}
}

func latestPlenaryID(store *plenary.JSONLStore) (string, error) {
	events, err := store.ListAll()
	if err != nil {
		return "", err
	}
	if len(events) == 0 {
		return "", fmt.Errorf("%w: no plenaries found", plenary.ErrNotFound)
	}
	return events[len(events)-1].PlenaryID, nil
}

func resolvePlenaryID(store *plenary.JSONLStore, args []string) (string, []string, error) {
	if id, rest := getFlag(args, "--plenary"); id != "" {
		return id, rest, nil
	}
	if useLast, rest := hasFlag(args, "--last"); useLast {
		id, err := latestPlenaryID(store)
		return id, rest, err
	}
	if id := os.Getenv("PLENARY_ID"); id != "" {
		return id, args, nil
	}
	return "", args, fmt.Errorf("%w: provide --plenary <id>, --last, or set PLENARY_ID", plenary.ErrValidation)
}

func getSnapshot(store *plenary.JSONLStore, plenaryID string) (plenary.Snapshot, []plenary.Event, error) {
	events, err := store.ListByPlenary(plenaryID)
	if err != nil {
		return plenary.Snapshot{}, nil, err
	}
	if len(events) == 0 {
		return plenary.Snapshot{}, nil, fmt.Errorf("%w: plenary %s not found", plenary.ErrNotFound, plenaryID)
	}
	snap, err := plenary.Reduce(events)
	if err != nil {
		return plenary.Snapshot{}, nil, err
	}
	return snap, events, nil
}

func resolveProposalID(store *plenary.JSONLStore, plenaryID string, args []string) (string, []string, error) {
	if id, rest := getFlag(args, "--proposal"); id != "" {
		return id, rest, nil
	} else {
		args = rest
	}
	_, args = hasFlag(args, "--active")
	snap, _, err := getSnapshot(store, plenaryID)
	if err != nil {
		return "", args, err
	}
	if snap.ActiveProposal == nil {
		return "", args, fmt.Errorf("%w: no active proposal (pass --proposal <id>)", plenary.ErrConflict)
	}
	return snap.ActiveProposal.ProposalID, args, nil
}

// --- Commands ---

func cmdCreate(store *plenary.JSONLStore, args []string) error {
	actor, err := getActor()
	if err != nil {
		return err
	}

	topic, args, err := requireFlag(args, "--topic")
	if err != nil {
		return err
	}
	context, args := getFlag(args, "--context")
	ruleStr, args := getFlag(args, "--decision-rule")
	if ruleStr == "" {
		ruleStr, args = getFlag(args, "--rule")
	}
	deadline, _ := getFlag(args, "--deadline")

	rule := plenary.RuleUnanimity
	if ruleStr != "" {
		rule = plenary.DecisionRule(ruleStr)
	}

	plenaryID := plenary.NewUUIDLike()
	payload := plenary.PlenaryCreatedPayload{
		Topic:        topic,
		Context:      context,
		DecisionRule: rule,
	}
	if deadline != "" {
		payload.Deadline = &deadline
	}

	evt, err := plenary.NewEvent(plenaryID, actor, "plenary.created", payload)
	if err != nil {
		return err
	}

	if err := store.Append(evt); err != nil {
		return err
	}

	return printJSON(map[string]string{
		"plenary_id": plenaryID,
		"status":     "created",
	})
}

func cmdJoin(store *plenary.JSONLStore, args []string) error {
	actor, err := getActor()
	if err != nil {
		return err
	}

	plenaryID, args, err := resolvePlenaryID(store, args)
	if err != nil {
		return err
	}
	role, args := getFlag(args, "--role")
	lens, _ := getFlag(args, "--lens")

	payload := plenary.ParticipantJoinedPayload{}
	if role != "" {
		payload.Role = &role
	}
	if lens != "" {
		payload.Lens = &lens
	}

	evt, err := plenary.NewEvent(plenaryID, actor, "participant.joined", payload)
	if err != nil {
		return err
	}

	if err := appendAndValidate(store, evt); err != nil {
		return err
	}

	return printJSON(map[string]string{
		"plenary_id": plenaryID,
		"actor_id":   actor.ActorID,
		"status":     "joined",
	})
}

func cmdList(store *plenary.JSONLStore, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("%w: list takes no flags", plenary.ErrValidation)
	}
	events, err := store.ListAll()
	if err != nil {
		return err
	}
	if len(events) == 0 {
		return printJSON([]any{})
	}

	grouped := map[string][]plenary.Event{}
	lastTS := map[string]string{}
	for _, evt := range events {
		grouped[evt.PlenaryID] = append(grouped[evt.PlenaryID], evt)
		lastTS[evt.PlenaryID] = evt.TS
	}

	type listItem struct {
		PlenaryID    string  `json:"plenary_id"`
		Topic        string  `json:"topic,omitempty"`
		Phase        string  `json:"phase"`
		DecisionRule string  `json:"decision_rule"`
		Closed       bool    `json:"closed"`
		EventCount   int     `json:"event_count"`
		LastEventAt  string  `json:"last_event_at"`
		Outcome      *string `json:"outcome,omitempty"`
	}
	out := make([]listItem, 0, len(grouped))
	for pid, evts := range grouped {
		snap, err := plenary.Reduce(evts)
		if err != nil {
			continue
		}
		var outcome *string
		if snap.Outcome != nil {
			v := string(*snap.Outcome)
			outcome = &v
		}
		out = append(out, listItem{
			PlenaryID:    pid,
			Topic:        snap.Topic,
			Phase:        string(snap.Phase),
			DecisionRule: string(snap.DecisionRule),
			Closed:       snap.Closed,
			EventCount:   snap.EventCount,
			LastEventAt:  lastTS[pid],
			Outcome:      outcome,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastEventAt > out[j].LastEventAt })
	return printJSON(out)
}

func cmdStatus(store *plenary.JSONLStore, args []string) error {
	plenaryID, _, err := resolvePlenaryID(store, args)
	if err != nil {
		return err
	}

	snap, _, err := getSnapshot(store, plenaryID)
	if err != nil {
		return err
	}

	return printJSON(snap)
}

func cmdPropose(store *plenary.JSONLStore, args []string) error {
	actor, err := getActor()
	if err != nil {
		return err
	}

	plenaryID, args, err := requireFlag(args, "--plenary")
	if err != nil {
		return err
	}
	text, args, err := requireFlag(args, "--text")
	if err != nil {
		return err
	}
	criteria, _ := getFlag(args, "--criteria")

	proposalID := plenary.NewUUIDLike()
	payload := plenary.ProposalCreatedPayload{
		ProposalID: proposalID,
		Text:       text,
	}
	if criteria != "" {
		payload.AcceptanceCriteria = &criteria
	}

	evt, err := plenary.NewEvent(plenaryID, actor, "proposal.created", payload)
	if err != nil {
		return err
	}

	if err := appendAndValidate(store, evt); err != nil {
		return err
	}

	return printJSON(map[string]string{
		"plenary_id":  plenaryID,
		"proposal_id": proposalID,
		"status":      "proposed",
	})
}

func cmdConsent(store *plenary.JSONLStore, args []string) error {
	actor, err := getActor()
	if err != nil {
		return err
	}

	plenaryID, args, err := resolvePlenaryID(store, args)
	if err != nil {
		return err
	}
	proposalID, args, err := resolveProposalID(store, plenaryID, args)
	if err != nil {
		return err
	}
	reason, _ := getFlag(args, "--reason")

	payload := plenary.ConsentPayload{
		ProposalID: proposalID,
	}
	if reason != "" {
		payload.Reason = &reason
	}

	evt, err := plenary.NewEvent(plenaryID, actor, "consent.given", payload)
	if err != nil {
		return err
	}

	if err := appendAndValidate(store, evt); err != nil {
		return err
	}

	return printJSON(map[string]string{
		"plenary_id": plenaryID,
		"actor_id":   actor.ActorID,
		"status":     "consent_given",
	})
}

func cmdBlock(store *plenary.JSONLStore, args []string) error {
	actor, err := getActor()
	if err != nil {
		return err
	}

	plenaryID, args, err := resolvePlenaryID(store, args)
	if err != nil {
		return err
	}
	proposalID, args, err := resolveProposalID(store, plenaryID, args)
	if err != nil {
		return err
	}
	text, args, err := requireFlag(args, "--reason")
	if err != nil {
		return err
	}
	principle, args := getFlag(args, "--principle")
	failureMode, _ := getFlag(args, "--failure-mode")

	payload := plenary.ProposalRefTextPayload{
		ProposalID: proposalID,
		Text:       text,
	}
	if principle != "" {
		payload.Principle = &principle
	}
	if failureMode != "" {
		payload.FailureMode = &failureMode
	}

	evt, err := plenary.NewEvent(plenaryID, actor, "block.raised", payload)
	if err != nil {
		return err
	}

	if err := appendAndValidate(store, evt); err != nil {
		return err
	}

	return printJSON(map[string]string{
		"plenary_id": plenaryID,
		"actor_id":   actor.ActorID,
		"status":     "block_raised",
	})
}

func cmdStandAside(store *plenary.JSONLStore, args []string) error {
	actor, err := getActor()
	if err != nil {
		return err
	}

	plenaryID, args, err := resolvePlenaryID(store, args)
	if err != nil {
		return err
	}
	proposalID, args, err := resolveProposalID(store, plenaryID, args)
	if err != nil {
		return err
	}
	reason, _, err := requireFlag(args, "--reason")
	if err != nil {
		return err
	}

	payload := plenary.StandAsidePayload{
		ProposalID: proposalID,
		Reason:     reason,
	}

	evt, err := plenary.NewEvent(plenaryID, actor, "stand_aside.given", payload)
	if err != nil {
		return err
	}

	if err := appendAndValidate(store, evt); err != nil {
		return err
	}

	return printJSON(map[string]string{
		"plenary_id": plenaryID,
		"actor_id":   actor.ActorID,
		"status":     "stand_aside_given",
	})
}

func cmdSpeak(store *plenary.JSONLStore, args []string) error {
	actor, err := getActor()
	if err != nil {
		return err
	}

	plenaryID, args, err := resolvePlenaryID(store, args)
	if err != nil {
		return err
	}
	// Accept --text (canonical) or --message (alias)
	text, args := getFlag(args, "--text")
	if text == "" {
		text, args = getFlag(args, "--message")
	}
	if text == "" {
		return fmt.Errorf("%w: --text is required", plenary.ErrValidation)
	}
	_ = args

	payload := plenary.TextPayload{Text: text}

	evt, err := plenary.NewEvent(plenaryID, actor, "speak", payload)
	if err != nil {
		return err
	}

	if err := appendAndValidate(store, evt); err != nil {
		return err
	}

	return printJSON(map[string]string{
		"plenary_id": plenaryID,
		"actor_id":   actor.ActorID,
		"status":     "spoke",
	})
}

func cmdPhase(store *plenary.JSONLStore, args []string) error {
	actor, err := getActor()
	if err != nil {
		return err
	}

	plenaryID, args, err := resolvePlenaryID(store, args)
	if err != nil {
		return err
	}
	to, args, err := requireFlag(args, "--to")
	if err != nil {
		return err
	}
	from, _, err := requireFlag(args, "--from")
	if err != nil {
		return err
	}

	payload := plenary.PhaseSetPayload{
		Phase:         plenary.Phase(to),
		ExpectedPhase: plenary.Phase(from),
	}

	evt, err := plenary.NewEvent(plenaryID, actor, "phase.set", payload)
	if err != nil {
		return err
	}

	if err := appendAndValidate(store, evt); err != nil {
		return err
	}

	return printJSON(map[string]string{
		"plenary_id": plenaryID,
		"phase":      to,
		"status":     "phase_set",
	})
}

func cmdClose(store *plenary.JSONLStore, args []string) error {
	actor, err := getActor()
	if err != nil {
		return err
	}

	plenaryID, args, err := resolvePlenaryID(store, args)
	if err != nil {
		return err
	}
	resolution, args, err := requireFlag(args, "--resolution")
	if err != nil {
		return err
	}
	outcomeStr, _ := getFlag(args, "--outcome")

	outcome := plenary.OutcomeConsensus
	if outcomeStr != "" {
		outcome = plenary.Outcome(outcomeStr)
	}

	// Build decision record from current state
	snap, _, err := getSnapshot(store, plenaryID)
	if err != nil {
		return err
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
		Outcome: outcome,
		DecisionRecord: plenary.DecisionRecord{
			Resolution:   resolution,
			Participants: participants,
		},
	}

	evt, err := plenary.NewEvent(plenaryID, actor, "decision.closed", payload)
	if err != nil {
		return err
	}

	if err := appendAndValidate(store, evt); err != nil {
		return err
	}

	return printJSON(map[string]string{
		"plenary_id": plenaryID,
		"outcome":    string(outcome),
		"status":     "closed",
	})
}

func cmdExport(store *plenary.JSONLStore, args []string) error {
	plenaryID, args, err := resolvePlenaryID(store, args)
	if err != nil {
		return err
	}
	outDir, _ := getFlag(args, "--out")
	if outDir == "" {
		outDir = filepath.Join(".plenary", "exports", plenaryID)
	}

	snap, events, err := getSnapshot(store, plenaryID)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	eventsPath := filepath.Join(outDir, "events.jsonl")
	snapshotPath := filepath.Join(outDir, "snapshot.json")
	transcriptPath := filepath.Join(outDir, "transcript.md")
	decisionPath := filepath.Join(outDir, "decision_record.json")

	if err := writeEventsJSONL(eventsPath, events); err != nil {
		return err
	}
	if err := writeJSONFile(snapshotPath, snap); err != nil {
		return err
	}
	if err := os.WriteFile(transcriptPath, []byte(renderTranscript(events, snap)), 0o644); err != nil {
		return err
	}
	decisionRecordWritten := false
	if closed, err := plenary.LatestDecisionRecord(events); err != nil {
		if !plenary.Is(err, plenary.ErrNotFound) {
			return err
		}
	} else {
		if err := writeJSONFile(decisionPath, closed.DecisionRecord); err != nil {
			return err
		}
		decisionRecordWritten = true
	}

	resp := map[string]any{
		"plenary_id":              plenaryID,
		"status":                  "exported",
		"out_dir":                 outDir,
		"events_jsonl":            eventsPath,
		"snapshot_json":           snapshotPath,
		"transcript_md":           transcriptPath,
		"decision_record_present": decisionRecordWritten,
	}
	if decisionRecordWritten {
		resp["decision_record_json"] = decisionPath
	}
	return printJSON(resp)
}

func cmdTail(store *plenary.JSONLStore, args []string) error {
	plenaryID, args, err := resolvePlenaryID(store, args)
	if err != nil {
		return err
	}
	follow, args := hasFlag(args, "--follow")
	intervalMS := 500
	if s, _ := getFlag(args, "--interval-ms"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 50 {
			return fmt.Errorf("%w: --interval-ms must be an integer >= 50", plenary.ErrValidation)
		}
		intervalMS = n
	}
	if len(args) > 0 {
		return fmt.Errorf("%w: unknown args: %s", plenary.ErrValidation, strings.Join(args, " "))
	}

	events, err := store.ListByPlenary(plenaryID)
	if err != nil {
		return err
	}
	if len(events) == 0 {
		return fmt.Errorf("%w: plenary %s not found", plenary.ErrNotFound, plenaryID)
	}
	seen := make(map[string]struct{}, len(events))
	for _, evt := range events {
		if err := printEventJSONLine(evt); err != nil {
			return err
		}
		seen[evt.EventID] = struct{}{}
	}
	if !follow {
		return nil
	}

	for {
		time.Sleep(time.Duration(intervalMS) * time.Millisecond)
		events, err := store.ListByPlenary(plenaryID)
		if err != nil {
			return err
		}
		for _, evt := range events {
			if _, ok := seen[evt.EventID]; ok {
				continue
			}
			if err := printEventJSONLine(evt); err != nil {
				return err
			}
			seen[evt.EventID] = struct{}{}
		}
	}
}

func cmdWait(store *plenary.JSONLStore, args []string) error {
	plenaryID, args, err := resolvePlenaryID(store, args)
	if err != nil {
		return err
	}

	targetPhase, args := getFlag(args, "--phase")
	targetEventType, args := getFlag(args, "--event-type")
	eventsAfterStr, args := getFlag(args, "--events-after")

	timeoutSec := 300
	if s, rest := getFlag(args, "--timeout"); s != "" {
		args = rest
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 {
			return fmt.Errorf("%w: --timeout must be a positive integer", plenary.ErrValidation)
		}
		timeoutSec = n
	}

	intervalMS := 500
	if s, rest := getFlag(args, "--interval-ms"); s != "" {
		args = rest
		n, err := strconv.Atoi(s)
		if err != nil || n < 50 {
			return fmt.Errorf("%w: --interval-ms must be an integer >= 50", plenary.ErrValidation)
		}
		intervalMS = n
	}

	if targetPhase == "" && targetEventType == "" && eventsAfterStr == "" {
		return fmt.Errorf("%w: at least one condition required: --phase, --event-type, or --events-after", plenary.ErrValidation)
	}

	eventsAfter := -1
	if eventsAfterStr != "" {
		n, err := strconv.Atoi(eventsAfterStr)
		if err != nil || n < 0 {
			return fmt.Errorf("%w: --events-after must be a non-negative integer", plenary.ErrValidation)
		}
		eventsAfter = n
	}

	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)

	for {
		events, err := store.ListByPlenary(plenaryID)
		if err != nil {
			return err
		}

		// Check events-after condition
		if eventsAfter >= 0 && len(events) > eventsAfter {
			if len(events) > 0 {
				return printEventJSONLine(events[len(events)-1])
			}
			return nil
		}

		// Check phase condition
		if targetPhase != "" && len(events) > 0 {
			snap, err := plenary.Reduce(events)
			if err == nil && string(snap.Phase) == targetPhase {
				return printEventJSONLine(events[len(events)-1])
			}
		}

		// Check event-type condition
		if targetEventType != "" {
			for i := len(events) - 1; i >= 0; i-- {
				if events[i].EventType == targetEventType {
					return printEventJSONLine(events[i])
				}
			}
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timeout: condition not met within %d seconds", timeoutSec)
		}
		time.Sleep(time.Duration(intervalMS) * time.Millisecond)
	}
}
