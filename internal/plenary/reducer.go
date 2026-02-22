package plenary

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

type ParticipantSnapshot struct {
	ActorID      string  `json:"actor_id"`
	ActorType    string  `json:"actor_type"`
	Role         *string `json:"role,omitempty"`
	Lens         *string `json:"lens,omitempty"`
	Stance       Stance  `json:"stance"`
	LastEventAt  string  `json:"last_event_at,omitempty"`
	FinalReason  *string `json:"final_reason,omitempty"`
}

type ProposalSnapshot struct {
	ProposalID         string  `json:"proposal_id"`
	Text               string  `json:"text"`
	AcceptanceCriteria *string `json:"acceptance_criteria,omitempty"`
}

type BlockSnapshot struct {
	ActorID      string  `json:"actor_id"`
	Text         string  `json:"text"`
	ProposalID   string  `json:"proposal_id"`
	Principle    *string `json:"principle,omitempty"`
	FailureMode  *string `json:"failure_mode,omitempty"`
	Status       string  `json:"status"`
}

type Snapshot struct {
	PlenaryID            string                `json:"plenary_id"`
	Topic                string                `json:"topic,omitempty"`
	Context              string                `json:"context,omitempty"`
	Phase                Phase                 `json:"phase"`
	DecisionRule         DecisionRule          `json:"decision_rule"`
	Deadline             *string               `json:"deadline,omitempty"`
	Participants         []ParticipantSnapshot `json:"participants"`
	ActiveProposal       *ProposalSnapshot     `json:"active_proposal,omitempty"`
	UnresolvedBlocks     []BlockSnapshot       `json:"unresolved_blocks"`
	OpenQuestions        []string              `json:"open_questions,omitempty"`
	ReadyToClose         bool                  `json:"ready_to_close"`
	NextRequiredActions  []string              `json:"next_required_actions"`
	Closed               bool                  `json:"closed"`
	Outcome              *Outcome              `json:"outcome,omitempty"`
	DecisionRecord       *DecisionRecord       `json:"decision_record,omitempty"`
	EventCount           int                   `json:"event_count"`
}

type reducerState struct {
	snapshot      Snapshot
	participants  map[string]*ParticipantSnapshot
	blocks        map[string]BlockSnapshot
	openQuestions []string
}

func Reduce(events []Event) (Snapshot, error) {
	state := reducerState{
		snapshot: Snapshot{
			Phase:            PhaseFraming,
			DecisionRule:     RuleUnanimity,
			Participants:     []ParticipantSnapshot{},
			UnresolvedBlocks: []BlockSnapshot{},
		},
		participants: map[string]*ParticipantSnapshot{},
		blocks:       map[string]BlockSnapshot{},
	}
	for _, evt := range events {
		if err := ApplyEvent(&state, evt); err != nil {
			return Snapshot{}, err
		}
	}
	finalizeSnapshot(&state)
	return state.snapshot, nil
}

func ReduceWithValidation(existing []Event, next Event) (Snapshot, error) {
	snap, err := Reduce(existing)
	if err != nil {
		return Snapshot{}, err
	}
	if err := ValidateEvent(snap, next, len(existing) == 0); err != nil {
		return Snapshot{}, err
	}
	all := append(append([]Event{}, existing...), next)
	return Reduce(all)
}

func ValidateEvent(snap Snapshot, evt Event, isFirst bool) error {
	if evt.PlenaryID == "" || evt.EventType == "" || evt.Actor.ActorID == "" || evt.Actor.ActorType == "" {
		return fmt.Errorf("%w: missing base event fields", ErrValidation)
	}
	if isFirst && evt.EventType != "plenary.created" {
		return fmt.Errorf("%w: first event must be plenary.created", ErrValidation)
	}
	if !isFirst && snap.Closed {
		return fmt.Errorf("%w: plenary is closed", ErrConflict)
	}

	switch evt.EventType {
	case "plenary.created":
		if !isFirst {
			return fmt.Errorf("%w: plenary already exists", ErrConflict)
		}
		var p PlenaryCreatedPayload
		if err := decodePayload(evt, &p); err != nil {
			return err
		}
		if p.Topic == "" {
			return fmt.Errorf("%w: topic required", ErrValidation)
		}
	case "participant.joined":
		// always allowed before close
	case "phase.set":
		var p PhaseSetPayload
		if err := decodePayload(evt, &p); err != nil {
			return err
		}
		if p.ExpectedPhase != snap.Phase {
			return fmt.Errorf("%w: expected_phase=%s current=%s", ErrConflict, p.ExpectedPhase, snap.Phase)
		}
	case "proposal.created":
		if snap.Phase != PhaseProposal && snap.Phase != PhaseObjections && snap.Phase != PhaseConsensusCheck {
			return fmt.Errorf("%w: proposal.created not allowed in phase %s", ErrConflict, snap.Phase)
		}
	case "block.raised", "consent.given", "stand_aside.given":
		if snap.ActiveProposal == nil {
			return fmt.Errorf("%w: no active proposal", ErrConflict)
		}
	}
	return nil
}

func ApplyEvent(state *reducerState, evt Event) error {
	state.snapshot.PlenaryID = evt.PlenaryID
	state.snapshot.EventCount++

	switch evt.EventType {
	case "plenary.created":
		var p PlenaryCreatedPayload
		if err := decodePayload(evt, &p); err != nil {
			return err
		}
		state.snapshot.Topic = p.Topic
		state.snapshot.Context = p.Context
		if p.DecisionRule != "" {
			state.snapshot.DecisionRule = p.DecisionRule
		}
		state.snapshot.Deadline = p.Deadline
		state.snapshot.Phase = PhaseFraming
	case "participant.joined":
		var p ParticipantJoinedPayload
		if err := decodePayload(evt, &p); err != nil {
			return err
		}
		item, ok := state.participants[evt.Actor.ActorID]
		if !ok {
			item = &ParticipantSnapshot{
				ActorID:   evt.Actor.ActorID,
				ActorType: evt.Actor.ActorType,
				Stance:    StanceUndeclared,
			}
			state.participants[evt.Actor.ActorID] = item
		}
		item.Role = p.Role
		item.Lens = p.Lens
		item.LastEventAt = evt.TS
	case "phase.set":
		var p PhaseSetPayload
		if err := decodePayload(evt, &p); err != nil {
			return err
		}
		state.snapshot.Phase = p.Phase
	case "question.asked":
		var p TextPayload
		if err := decodePayload(evt, &p); err != nil {
			return err
		}
		state.openQuestions = append(state.openQuestions, p.Text)
		touchParticipant(state, evt, nil)
	case "speak", "position.submitted", "concern.raised", "summary.computed":
		touchParticipant(state, evt, nil)
	case "proposal.created":
		var p ProposalCreatedPayload
		if err := decodePayload(evt, &p); err != nil {
			return err
		}
		state.snapshot.ActiveProposal = &ProposalSnapshot{
			ProposalID:         p.ProposalID,
			Text:               p.Text,
			AcceptanceCriteria: p.AcceptanceCriteria,
		}
		for _, pt := range state.participants {
			pt.Stance = StanceUndeclared
			pt.FinalReason = nil
		}
		state.blocks = map[string]BlockSnapshot{}
		touchParticipant(state, evt, nil)
	case "amendment.applied":
		var p struct {
			ProposalID string `json:"proposal_id"`
			NewText    string `json:"new_text"`
		}
		if err := decodePayload(evt, &p); err != nil {
			return err
		}
		if state.snapshot.ActiveProposal != nil && state.snapshot.ActiveProposal.ProposalID == p.ProposalID {
			state.snapshot.ActiveProposal.Text = p.NewText
			for _, pt := range state.participants {
				if pt.Stance != StanceBlock {
					pt.Stance = StanceUndeclared
					pt.FinalReason = nil
				}
			}
		}
		touchParticipant(state, evt, nil)
	case "block.raised":
		var p ProposalRefTextPayload
		if err := decodePayload(evt, &p); err != nil {
			return err
		}
		block := BlockSnapshot{
			ActorID:     evt.Actor.ActorID,
			Text:        p.Text,
			ProposalID:  p.ProposalID,
			Principle:   p.Principle,
			FailureMode: p.FailureMode,
			Status:      "open",
		}
		state.blocks[evt.Actor.ActorID] = block
		touchParticipant(state, evt, func(pt *ParticipantSnapshot) {
			pt.Stance = StanceBlock
		})
	case "block.withdrawn":
		var p BlockWithdrawnPayload
		if err := decodePayload(evt, &p); err != nil {
			return err
		}
		delete(state.blocks, evt.Actor.ActorID)
		touchParticipant(state, evt, func(pt *ParticipantSnapshot) {
			if pt.Stance == StanceBlock {
				pt.Stance = StanceUndeclared
			}
			reason := p.Reason
			pt.FinalReason = &reason
		})
	case "consent.given":
		var p ConsentPayload
		if err := decodePayload(evt, &p); err != nil {
			return err
		}
		touchParticipant(state, evt, func(pt *ParticipantSnapshot) {
			pt.Stance = StanceConsent
			pt.FinalReason = p.Reason
		})
	case "stand_aside.given":
		var p StandAsidePayload
		if err := decodePayload(evt, &p); err != nil {
			return err
		}
		touchParticipant(state, evt, func(pt *ParticipantSnapshot) {
			pt.Stance = StanceStandAside
			reason := p.Reason
			pt.FinalReason = &reason
		})
	case "decision.closed":
		var p DecisionClosedPayload
		if err := decodePayload(evt, &p); err != nil {
			return err
		}
		state.snapshot.Closed = true
		state.snapshot.Phase = PhaseClosed
		outcome := p.Outcome
		state.snapshot.Outcome = &outcome
		dr := p.DecisionRecord
		state.snapshot.DecisionRecord = &dr
	default:
		return fmt.Errorf("%w: unknown event_type %q", ErrValidation, evt.EventType)
	}
	return nil
}

func finalizeSnapshot(state *reducerState) {
	participants := make([]ParticipantSnapshot, 0, len(state.participants))
	for _, p := range state.participants {
		participants = append(participants, *p)
	}
	sort.Slice(participants, func(i, j int) bool { return participants[i].ActorID < participants[j].ActorID })
	state.snapshot.Participants = participants

	blocks := make([]BlockSnapshot, 0, len(state.blocks))
	for _, b := range state.blocks {
		blocks = append(blocks, b)
	}
	sort.Slice(blocks, func(i, j int) bool { return blocks[i].ActorID < blocks[j].ActorID })
	state.snapshot.UnresolvedBlocks = blocks

	state.snapshot.OpenQuestions = append([]string(nil), state.openQuestions...)
	state.snapshot.NextRequiredActions = computeNextActions(state.snapshot)
	state.snapshot.ReadyToClose = computeReadyToClose(state.snapshot)
}

func computeReadyToClose(s Snapshot) bool {
	if s.Closed || s.Phase != PhaseConsensusCheck || s.ActiveProposal == nil || len(s.Participants) == 0 {
		return false
	}
	if len(s.UnresolvedBlocks) > 0 {
		return false
	}
	consents := 0
	standAsides := 0
	for _, p := range s.Participants {
		switch p.Stance {
		case StanceConsent:
			consents++
		case StanceStandAside:
			standAsides++
		default:
			// undeclared or block — not ready
			return false
		}
	}
	switch s.DecisionRule {
	case RuleUnanimity:
		// Quaker unanimity: all must declare, stand-asides are allowed,
		// only blocks prevent consensus. At least one consent required.
		return consents > 0 && (consents+standAsides) == len(s.Participants)
	case RuleQuorum, RuleTimeboxed:
		return consents > 0
	default:
		return false
	}
}

func computeNextActions(s Snapshot) []string {
	if s.Closed {
		return nil
	}
	var out []string
	if s.ActiveProposal == nil && (s.Phase == PhaseProposal || s.Phase == PhaseObjections || s.Phase == PhaseConsensusCheck) {
		out = append(out, "need active proposal")
	}
	for _, b := range s.UnresolvedBlocks {
		out = append(out, fmt.Sprintf("resolve block by %s", b.ActorID))
	}
	if s.Phase == PhaseConsensusCheck && s.ActiveProposal != nil {
		for _, p := range s.Participants {
			if p.Stance == StanceUndeclared {
				out = append(out, fmt.Sprintf("need stance from %s", p.ActorID))
			}
		}
	}
	if len(out) == 0 {
		switch s.Phase {
		case PhaseFraming:
			out = append(out, "set phase to divergence")
		case PhaseDivergence:
			out = append(out, "set phase to proposal")
		case PhaseProposal:
			out = append(out, "create proposal or move to objections")
		case PhaseObjections:
			out = append(out, "move to consensus_check when objections addressed")
		case PhaseConsensusCheck:
			if s.ReadyToClose {
				out = append(out, "close plenary")
			}
		}
	}
	return out
}

func touchParticipant(state *reducerState, evt Event, update func(*ParticipantSnapshot)) {
	item, ok := state.participants[evt.Actor.ActorID]
	if !ok {
		item = &ParticipantSnapshot{
			ActorID:   evt.Actor.ActorID,
			ActorType: evt.Actor.ActorType,
			Stance:    StanceUndeclared,
		}
		state.participants[evt.Actor.ActorID] = item
	}
	item.LastEventAt = evt.TS
	if update != nil {
		update(item)
	}
}

func decodePayload[T any](evt Event, out *T) error {
	if len(evt.Payload) == 0 {
		return fmt.Errorf("%w: empty payload for %s", ErrValidation, evt.EventType)
	}
	if err := json.Unmarshal(evt.Payload, out); err != nil {
		return fmt.Errorf("%w: invalid payload for %s: %v", ErrValidation, evt.EventType, err)
	}
	return nil
}

func LatestDecisionRecord(events []Event) (*DecisionClosedPayload, error) {
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].EventType != "decision.closed" {
			continue
		}
		var p DecisionClosedPayload
		if err := decodePayload(events[i], &p); err != nil {
			return nil, err
		}
		return &p, nil
	}
	return nil, ErrNotFound
}

func Is(err, target error) bool {
	return errors.Is(err, target)
}

