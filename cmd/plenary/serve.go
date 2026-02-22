package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"

	plenary "github.com/keetonmartin/plenary/internal/plenary"
)

// sseHub manages Server-Sent Events subscribers
type sseHub struct {
	mu          sync.Mutex
	subscribers map[chan []byte]string // chan -> plenaryID filter ("" = all)
}

func newSSEHub() *sseHub {
	return &sseHub{subscribers: make(map[chan []byte]string)}
}

func (h *sseHub) subscribe(plenaryID string) chan []byte {
	ch := make(chan []byte, 64)
	h.mu.Lock()
	h.subscribers[ch] = plenaryID
	h.mu.Unlock()
	return ch
}

func (h *sseHub) unsubscribe(ch chan []byte) {
	h.mu.Lock()
	delete(h.subscribers, ch)
	h.mu.Unlock()
	close(ch)
}

func (h *sseHub) broadcast(evt plenary.Event) {
	b, err := json.Marshal(evt)
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch, filter := range h.subscribers {
		if filter != "" && filter != evt.PlenaryID {
			continue
		}
		select {
		case ch <- b:
		default:
			// slow subscriber, skip
		}
	}
}

// appendAndBroadcast appends an event, validates, and broadcasts to SSE subscribers
func appendAndBroadcast(store *plenary.JSONLStore, hub *sseHub, evt plenary.Event) error {
	events, err := store.ListByPlenary(evt.PlenaryID)
	if err != nil {
		return err
	}
	_, err = plenary.ReduceWithValidation(events, evt)
	if err != nil {
		return err
	}
	if err := store.Append(evt); err != nil {
		return err
	}
	hub.broadcast(evt)
	return nil
}

func cmdServe(store *plenary.JSONLStore, args []string) error {
	port, _ := getFlag(args, "--port")
	if port == "" {
		port = "8080"
	}

	hub := newSSEHub()
	mux := http.NewServeMux()

	// --- List plenaries ---
	mux.HandleFunc("GET /api/plenaries", func(w http.ResponseWriter, r *http.Request) {
		corsHeaders(w)
		events, err := store.ListAll()
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		grouped := map[string][]plenary.Event{}
		for _, evt := range events {
			grouped[evt.PlenaryID] = append(grouped[evt.PlenaryID], evt)
		}
		type summary struct {
			PlenaryID   string `json:"plenary_id"`
			Topic       string `json:"topic"`
			Phase       string `json:"phase"`
			Rule        string `json:"decision_rule"`
			Closed      bool   `json:"closed"`
			Events      int    `json:"event_count"`
			LastEventAt string `json:"last_event_at,omitempty"`
		}
		summaries := make([]summary, 0, len(grouped))
		for pid, evts := range grouped {
			snap, err := plenary.Reduce(evts)
			if err != nil {
				continue
			}
			lastEventAt := ""
			if n := len(evts); n > 0 {
				lastEventAt = evts[n-1].TS
			}
			summaries = append(summaries, summary{
				PlenaryID:   pid,
				Topic:       snap.Topic,
				Phase:       string(snap.Phase),
				Rule:        string(snap.DecisionRule),
				Closed:      snap.Closed,
				Events:      snap.EventCount,
				LastEventAt: lastEventAt,
			})
		}
		sort.Slice(summaries, func(i, j int) bool {
			if summaries[i].Closed != summaries[j].Closed {
				return !summaries[i].Closed
			}
			if summaries[i].LastEventAt != summaries[j].LastEventAt {
				return summaries[i].LastEventAt > summaries[j].LastEventAt
			}
			return summaries[i].PlenaryID < summaries[j].PlenaryID
		})
		jsonResponse(w, summaries)
	})

	// --- Create plenary ---
	mux.HandleFunc("POST /api/plenaries", func(w http.ResponseWriter, r *http.Request) {
		corsHeaders(w)
		var req struct {
			Actor           plenary.Actor        `json:"actor"`
			Topic           string               `json:"topic"`
			Context         string               `json:"context,omitempty"`
			DecisionRule    plenary.DecisionRule  `json:"decision_rule"`
			Deadline        *string              `json:"deadline,omitempty"`
			QuorumThreshold *int                 `json:"quorum_threshold,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "invalid JSON: "+err.Error(), 400)
			return
		}
		if req.Actor.ActorID == "" || req.Topic == "" {
			jsonError(w, "actor.actor_id and topic are required", 400)
			return
		}
		if req.DecisionRule == "" {
			req.DecisionRule = plenary.RuleUnanimity
		}
		if req.Actor.ActorType == "" {
			req.Actor.ActorType = "agent"
		}

		plenaryID := plenary.NewUUIDLike()
		payload := plenary.PlenaryCreatedPayload{
			Topic:           req.Topic,
			Context:         req.Context,
			DecisionRule:    req.DecisionRule,
			Deadline:        req.Deadline,
			QuorumThreshold: req.QuorumThreshold,
		}
		evt, err := plenary.NewEvent(plenaryID, req.Actor, "plenary.created", payload)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		if err := store.Append(evt); err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		hub.broadcast(evt)
		w.WriteHeader(201)
		jsonResponse(w, map[string]string{"plenary_id": plenaryID, "status": "created"})
	})

	// --- Get plenary status ---
	mux.HandleFunc("GET /api/plenaries/{id}", func(w http.ResponseWriter, r *http.Request) {
		corsHeaders(w)
		pid := r.PathValue("id")
		events, err := store.ListByPlenary(pid)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		if len(events) == 0 {
			jsonError(w, "not found", 404)
			return
		}
		snap, err := plenary.Reduce(events)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		jsonResponse(w, snap)
	})

	// --- Get plenary events ---
	mux.HandleFunc("GET /api/plenaries/{id}/events", func(w http.ResponseWriter, r *http.Request) {
		corsHeaders(w)
		pid := r.PathValue("id")
		events, err := store.ListByPlenary(pid)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		if len(events) == 0 {
			jsonError(w, "not found", 404)
			return
		}
		jsonResponse(w, events)
	})

	// --- SSE stream ---
	mux.HandleFunc("GET /api/plenaries/{id}/stream", func(w http.ResponseWriter, r *http.Request) {
		corsHeaders(w)
		pid := r.PathValue("id")

		flusher, ok := w.(http.Flusher)
		if !ok {
			jsonError(w, "streaming not supported", 500)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		ch := hub.subscribe(pid)
		defer hub.unsubscribe(ch)

		// Send existing events as initial state
		events, _ := store.ListByPlenary(pid)
		for _, evt := range events {
			b, _ := json.Marshal(evt)
			fmt.Fprintf(w, "data: %s\n\n", b)
		}
		flusher.Flush()

		for {
			select {
			case data, ok := <-ch:
				if !ok {
					return
				}
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	})

	// --- Global SSE stream (all plenaries) ---
	mux.HandleFunc("GET /api/stream", func(w http.ResponseWriter, r *http.Request) {
		corsHeaders(w)
		flusher, ok := w.(http.Flusher)
		if !ok {
			jsonError(w, "streaming not supported", 500)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		ch := hub.subscribe("") // empty = all plenaries

		// Send a connected event
		fmt.Fprintf(w, "event: connected\ndata: {\"ts\":%q}\n\n", time.Now().UTC().Format(time.RFC3339))
		flusher.Flush()

		defer hub.unsubscribe(ch)
		for {
			select {
			case data, ok := <-ch:
				if !ok {
					return
				}
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	})

	// --- Generic action endpoint ---
	// POST /api/plenaries/:id/join
	// POST /api/plenaries/:id/speak
	// POST /api/plenaries/:id/propose
	// POST /api/plenaries/:id/consent
	// POST /api/plenaries/:id/block
	// POST /api/plenaries/:id/stand-aside
	// POST /api/plenaries/:id/phase
	// POST /api/plenaries/:id/close

	mux.HandleFunc("POST /api/plenaries/{id}/join", func(w http.ResponseWriter, r *http.Request) {
		corsHeaders(w)
		pid := r.PathValue("id")
		var req struct {
			Actor plenary.Actor `json:"actor"`
			Role  *string       `json:"role,omitempty"`
			Lens  *string       `json:"lens,omitempty"`
		}
		if err := readJSON(r, &req); err != nil {
			jsonError(w, err.Error(), 400)
			return
		}
		if req.Actor.ActorID == "" {
			jsonError(w, "actor.actor_id is required", 400)
			return
		}
		payload := plenary.ParticipantJoinedPayload{Role: req.Role, Lens: req.Lens}
		evt, err := plenary.NewEvent(pid, req.Actor, "participant.joined", payload)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		if err := appendAndBroadcast(store, hub, evt); err != nil {
			handlePlenaryError(w, err)
			return
		}
		jsonResponse(w, map[string]string{"plenary_id": pid, "actor_id": req.Actor.ActorID, "status": "joined"})
	})

	mux.HandleFunc("POST /api/plenaries/{id}/speak", func(w http.ResponseWriter, r *http.Request) {
		corsHeaders(w)
		pid := r.PathValue("id")
		var req struct {
			Actor plenary.Actor `json:"actor"`
			Text  string        `json:"text"`
		}
		if err := readJSON(r, &req); err != nil {
			jsonError(w, err.Error(), 400)
			return
		}
		if req.Actor.ActorID == "" || req.Text == "" {
			jsonError(w, "actor.actor_id and text are required", 400)
			return
		}
		payload := plenary.TextPayload{Text: req.Text}
		evt, err := plenary.NewEvent(pid, req.Actor, "speak", payload)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		if err := appendAndBroadcast(store, hub, evt); err != nil {
			handlePlenaryError(w, err)
			return
		}
		jsonResponse(w, map[string]string{"plenary_id": pid, "actor_id": req.Actor.ActorID, "status": "spoke"})
	})

	mux.HandleFunc("POST /api/plenaries/{id}/propose", func(w http.ResponseWriter, r *http.Request) {
		corsHeaders(w)
		pid := r.PathValue("id")
		var req struct {
			Actor              plenary.Actor `json:"actor"`
			Text               string        `json:"text"`
			AcceptanceCriteria *string       `json:"acceptance_criteria,omitempty"`
		}
		if err := readJSON(r, &req); err != nil {
			jsonError(w, err.Error(), 400)
			return
		}
		if req.Actor.ActorID == "" || req.Text == "" {
			jsonError(w, "actor.actor_id and text are required", 400)
			return
		}
		proposalID := plenary.NewUUIDLike()
		payload := plenary.ProposalCreatedPayload{
			ProposalID:         proposalID,
			Text:               req.Text,
			AcceptanceCriteria: req.AcceptanceCriteria,
		}
		evt, err := plenary.NewEvent(pid, req.Actor, "proposal.created", payload)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		if err := appendAndBroadcast(store, hub, evt); err != nil {
			handlePlenaryError(w, err)
			return
		}
		jsonResponse(w, map[string]string{"plenary_id": pid, "proposal_id": proposalID, "status": "proposed"})
	})

	mux.HandleFunc("POST /api/plenaries/{id}/consent", func(w http.ResponseWriter, r *http.Request) {
		corsHeaders(w)
		pid := r.PathValue("id")
		var req struct {
			Actor      plenary.Actor `json:"actor"`
			ProposalID string        `json:"proposal_id"`
			Reason     *string       `json:"reason,omitempty"`
		}
		if err := readJSON(r, &req); err != nil {
			jsonError(w, err.Error(), 400)
			return
		}
		if req.Actor.ActorID == "" || req.ProposalID == "" {
			jsonError(w, "actor.actor_id and proposal_id are required", 400)
			return
		}
		payload := plenary.ConsentPayload{ProposalID: req.ProposalID, Reason: req.Reason}
		evt, err := plenary.NewEvent(pid, req.Actor, "consent.given", payload)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		if err := appendAndBroadcast(store, hub, evt); err != nil {
			handlePlenaryError(w, err)
			return
		}
		jsonResponse(w, map[string]string{"plenary_id": pid, "actor_id": req.Actor.ActorID, "status": "consent_given"})
	})

	mux.HandleFunc("POST /api/plenaries/{id}/block", func(w http.ResponseWriter, r *http.Request) {
		corsHeaders(w)
		pid := r.PathValue("id")
		var req struct {
			Actor       plenary.Actor `json:"actor"`
			ProposalID  string        `json:"proposal_id"`
			Text        string        `json:"text"`
			Principle   *string       `json:"principle,omitempty"`
			FailureMode *string       `json:"failure_mode,omitempty"`
		}
		if err := readJSON(r, &req); err != nil {
			jsonError(w, err.Error(), 400)
			return
		}
		if req.Actor.ActorID == "" || req.ProposalID == "" || req.Text == "" {
			jsonError(w, "actor.actor_id, proposal_id, and text are required", 400)
			return
		}
		payload := plenary.ProposalRefTextPayload{
			ProposalID:  req.ProposalID,
			Text:        req.Text,
			Principle:   req.Principle,
			FailureMode: req.FailureMode,
		}
		evt, err := plenary.NewEvent(pid, req.Actor, "block.raised", payload)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		if err := appendAndBroadcast(store, hub, evt); err != nil {
			handlePlenaryError(w, err)
			return
		}
		jsonResponse(w, map[string]string{"plenary_id": pid, "actor_id": req.Actor.ActorID, "status": "block_raised"})
	})

	mux.HandleFunc("POST /api/plenaries/{id}/stand-aside", func(w http.ResponseWriter, r *http.Request) {
		corsHeaders(w)
		pid := r.PathValue("id")
		var req struct {
			Actor      plenary.Actor `json:"actor"`
			ProposalID string        `json:"proposal_id"`
			Reason     string        `json:"reason"`
		}
		if err := readJSON(r, &req); err != nil {
			jsonError(w, err.Error(), 400)
			return
		}
		if req.Actor.ActorID == "" || req.ProposalID == "" || req.Reason == "" {
			jsonError(w, "actor.actor_id, proposal_id, and reason are required", 400)
			return
		}
		payload := plenary.StandAsidePayload{ProposalID: req.ProposalID, Reason: req.Reason}
		evt, err := plenary.NewEvent(pid, req.Actor, "stand_aside.given", payload)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		if err := appendAndBroadcast(store, hub, evt); err != nil {
			handlePlenaryError(w, err)
			return
		}
		jsonResponse(w, map[string]string{"plenary_id": pid, "actor_id": req.Actor.ActorID, "status": "stand_aside_given"})
	})

	mux.HandleFunc("POST /api/plenaries/{id}/phase", func(w http.ResponseWriter, r *http.Request) {
		corsHeaders(w)
		pid := r.PathValue("id")
		var req struct {
			Actor         plenary.Actor `json:"actor"`
			To            plenary.Phase `json:"to"`
			ExpectedPhase plenary.Phase `json:"from"`
		}
		if err := readJSON(r, &req); err != nil {
			jsonError(w, err.Error(), 400)
			return
		}
		if req.Actor.ActorID == "" || req.To == "" || req.ExpectedPhase == "" {
			jsonError(w, "actor.actor_id, to, and from are required", 400)
			return
		}
		payload := plenary.PhaseSetPayload{Phase: req.To, ExpectedPhase: req.ExpectedPhase}
		evt, err := plenary.NewEvent(pid, req.Actor, "phase.set", payload)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		if err := appendAndBroadcast(store, hub, evt); err != nil {
			handlePlenaryError(w, err)
			return
		}
		jsonResponse(w, map[string]string{"plenary_id": pid, "phase": string(req.To), "status": "phase_set"})
	})

	mux.HandleFunc("POST /api/plenaries/{id}/close", func(w http.ResponseWriter, r *http.Request) {
		corsHeaders(w)
		pid := r.PathValue("id")
		var req struct {
			Actor      plenary.Actor   `json:"actor"`
			Outcome    plenary.Outcome `json:"outcome"`
			Resolution string          `json:"resolution"`
		}
		if err := readJSON(r, &req); err != nil {
			jsonError(w, err.Error(), 400)
			return
		}
		if req.Actor.ActorID == "" || req.Resolution == "" {
			jsonError(w, "actor.actor_id and resolution are required", 400)
			return
		}
		if req.Outcome == "" {
			req.Outcome = plenary.OutcomeConsensus
		}

		// Build decision record from current state
		events, err := store.ListByPlenary(pid)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		snap, err := plenary.Reduce(events)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
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
			Outcome: req.Outcome,
			DecisionRecord: plenary.DecisionRecord{
				Resolution:   req.Resolution,
				Participants: participants,
			},
		}
		evt, err := plenary.NewEvent(pid, req.Actor, "decision.closed", payload)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		if err := appendAndBroadcast(store, hub, evt); err != nil {
			handlePlenaryError(w, err)
			return
		}
		jsonResponse(w, map[string]string{"plenary_id": pid, "outcome": string(req.Outcome), "status": "closed"})
	})

	// --- CORS preflight ---
	mux.HandleFunc("OPTIONS /", func(w http.ResponseWriter, r *http.Request) {
		corsHeaders(w)
		w.WriteHeader(204)
	})

	addr := "0.0.0.0:" + port
	fmt.Printf("Plenary API server: http://%s\n", addr)
	fmt.Printf("SSE stream: http://%s/api/stream\n", addr)
	return http.ListenAndServe(addr, mux)
}

// --- HTTP helpers ---

func corsHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func jsonResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func readJSON(r *http.Request, v any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}

func handlePlenaryError(w http.ResponseWriter, err error) {
	msg := err.Error()
	switch {
	case plenary.Is(err, plenary.ErrValidation):
		jsonError(w, msg, 400)
	case plenary.Is(err, plenary.ErrConflict):
		jsonError(w, msg, 409)
	case plenary.Is(err, plenary.ErrNotFound):
		jsonError(w, msg, 404)
	default:
		jsonError(w, msg, 500)
	}
}
