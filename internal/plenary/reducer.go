package plenary

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

// deadlineReached checks if a deadline string has passed. Exported for testing.
func deadlineReached(deadline *string) bool {
	if deadline == nil {
		return false
	}
	t, err := time.Parse(time.RFC3339, *deadline)
	if err != nil {
		return false
	}
	return time.Now().After(t)
}

type ParticipantSnapshot struct {
	ActorID     string  `json:"actor_id"`
	ActorType   string  `json:"actor_type"`
	Role        *string `json:"role,omitempty"`
	Lens        *string `json:"lens,omitempty"`
	Stance      Stance  `json:"stance"`
	LastEventAt string  `json:"last_event_at,omitempty"`
	FinalReason *string `json:"final_reason,omitempty"`
}

type ProposalSnapshot struct {
	ProposalID         string  `json:"proposal_id"`
	Text               string  `json:"text"`
	AcceptanceCriteria *string `json:"acceptance_criteria,omitempty"`
}

type BlockSnapshot struct {
	ActorID     string  `json:"actor_id"`
	Text        string  `json:"text"`
	ProposalID  string  `json:"proposal_id"`
	Principle   *string `json:"principle,omitempty"`
	FailureMode *string `json:"failure_mode,omitempty"`
	Status      string  `json:"status"`
}

type Snapshot struct {
	PlenaryID           string                `json:"plenary_id"`
	Topic               string                `json:"topic,omitempty"`
	Context             string                `json:"context,omitempty"`
	Phase               Phase                 `json:"phase"`
	DecisionRule        DecisionRule          `json:"decision_rule"`
	Deadline            *string               `json:"deadline,omitempty"`
	QuorumThreshold     *int                  `json:"quorum_threshold,omitempty"`
	Participants        []ParticipantSnapshot `json:"participants"`
	Proposals           []ProposalSnapshot    `json:"proposals,omitempty"`
	ActiveProposal      *ProposalSnapshot     `json:"active_proposal,omitempty"`
	UnresolvedBlocks    []BlockSnapshot       `json:"unresolved_blocks"`
	OpenQuestions       []string              `json:"open_questions,omitempty"`
	ReadyToClose        bool                  `json:"ready_to_close"`
	NextRequiredActions []string              `json:"next_required_actions"`
	Closed              bool                  `json:"closed"`
	Outcome             *Outcome              `json:"outcome,omitempty"`
	DecisionRecord      *DecisionRecord       `json:"decision_record,omitempty"`
	EventCount          int                   `json:"event_count"`
}

type reducerState struct {
	snapshot      Snapshot
	participants  map[string]*ParticipantSnapshot
	proposals     map[string]ProposalSnapshot
	proposalOrder []string
	activeID      string
	stances       map[string]map[string]Stance
	reasons       map[string]map[string]*string
	blocks        map[string]map[string]BlockSnapshot
	openQuestions []string
}

func Reduce(events []Event) (Snapshot, error) {
	state := reducerState{
		snapshot: Snapshot{
			Phase:            PhaseFraming,
			DecisionRule:     RuleUnanimity,
			Participants:     []ParticipantSnapshot{},
			Proposals:        []ProposalSnapshot{},
			UnresolvedBlocks: []BlockSnapshot{},
		},
		participants: map[string]*ParticipantSnapshot{},
		proposals:    map[string]ProposalSnapshot{},
		stances:      map[string]map[string]Stance{},
		reasons:      map[string]map[string]*string{},
		blocks:       map[string]map[string]BlockSnapshot{},
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
	switch evt.Actor.ActorType {
	case "human", "agent", "ai":
		// 'ai' accepted for backward compatibility with dogfood logs;
		// CLI normalizes it to 'agent' for newly created events.
	default:
		return fmt.Errorf("%w: invalid actor_type %q", ErrValidation, evt.Actor.ActorType)
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
		if p.DecisionRule == RuleTimeboxed && p.Deadline == nil {
			return fmt.Errorf("%w: deadline required for timeboxed decision rule", ErrValidation)
		}
		if p.DecisionRule == RuleTimeboxed && p.Deadline != nil {
			if _, err := time.Parse(time.RFC3339, *p.Deadline); err != nil {
				return fmt.Errorf("%w: deadline must be valid RFC3339", ErrValidation)
			}
		}
		if p.QuorumThreshold != nil && (*p.QuorumThreshold < 1 || *p.QuorumThreshold > 100) {
			return fmt.Errorf("%w: quorum_threshold must be 1-100", ErrValidation)
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
	case "proposal.selected":
		var p ProposalSelectedPayload
		if err := decodePayload(evt, &p); err != nil {
			return err
		}
		if !proposalExists(snap.Proposals, p.ProposalID) {
			return fmt.Errorf("%w: unknown proposal_id %s", ErrValidation, p.ProposalID)
		}
	case "block.raised":
		var p ProposalRefTextPayload
		if err := decodePayload(evt, &p); err != nil {
			return err
		}
		if !proposalExists(snap.Proposals, p.ProposalID) {
			return fmt.Errorf("%w: unknown proposal_id %s", ErrValidation, p.ProposalID)
		}
	case "consent.given":
		var p ConsentPayload
		if err := decodePayload(evt, &p); err != nil {
			return err
		}
		if !proposalExists(snap.Proposals, p.ProposalID) {
			return fmt.Errorf("%w: unknown proposal_id %s", ErrValidation, p.ProposalID)
		}
	case "stand_aside.given":
		var p StandAsidePayload
		if err := decodePayload(evt, &p); err != nil {
			return err
		}
		if !proposalExists(snap.Proposals, p.ProposalID) {
			return fmt.Errorf("%w: unknown proposal_id %s", ErrValidation, p.ProposalID)
		}
	case "block.withdrawn":
		var p BlockWithdrawnPayload
		if err := decodePayload(evt, &p); err != nil {
			return err
		}
		if !proposalExists(snap.Proposals, p.ProposalID) {
			return fmt.Errorf("%w: unknown proposal_id %s", ErrValidation, p.ProposalID)
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
		state.snapshot.QuorumThreshold = p.QuorumThreshold
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
		prop := ProposalSnapshot{
			ProposalID:         p.ProposalID,
			Text:               p.Text,
			AcceptanceCriteria: p.AcceptanceCriteria,
		}
		if _, exists := state.proposals[p.ProposalID]; !exists {
			state.proposalOrder = append(state.proposalOrder, p.ProposalID)
		}
		state.proposals[p.ProposalID] = prop
		state.activeID = p.ProposalID
		ensureProposalState(state, p.ProposalID)
		touchParticipant(state, evt, nil)
	case "proposal.selected":
		var p ProposalSelectedPayload
		if err := decodePayload(evt, &p); err != nil {
			return err
		}
		if _, exists := state.proposals[p.ProposalID]; exists {
			state.activeID = p.ProposalID
		}
		touchParticipant(state, evt, nil)
	case "proposal.withdrawn":
		var p ProposalWithdrawnPayload
		if err := decodePayload(evt, &p); err != nil {
			return err
		}
		delete(state.proposals, p.ProposalID)
		delete(state.stances, p.ProposalID)
		delete(state.reasons, p.ProposalID)
		delete(state.blocks, p.ProposalID)
		newOrder := make([]string, 0, len(state.proposalOrder))
		for _, id := range state.proposalOrder {
			if id != p.ProposalID {
				newOrder = append(newOrder, id)
			}
		}
		state.proposalOrder = newOrder
		if state.activeID == p.ProposalID {
			state.activeID = ""
			if len(state.proposalOrder) > 0 {
				state.activeID = state.proposalOrder[len(state.proposalOrder)-1]
			}
		}
		touchParticipant(state, evt, nil)
	case "amendment.applied":
		var p struct {
			ProposalID string `json:"proposal_id"`
			NewText    string `json:"new_text"`
		}
		if err := decodePayload(evt, &p); err != nil {
			return err
		}
		if prop, ok := state.proposals[p.ProposalID]; ok {
			prop.Text = p.NewText
			state.proposals[p.ProposalID] = prop
			// Text changes invalidate prior stances for that proposal.
			delete(state.stances, p.ProposalID)
			delete(state.reasons, p.ProposalID)
			delete(state.blocks, p.ProposalID)
		}
		touchParticipant(state, evt, nil)
	case "block.raised":
		var p ProposalRefTextPayload
		if err := decodePayload(evt, &p); err != nil {
			return err
		}
		ensureProposalState(state, p.ProposalID)
		block := BlockSnapshot{
			ActorID:     evt.Actor.ActorID,
			Text:        p.Text,
			ProposalID:  p.ProposalID,
			Principle:   p.Principle,
			FailureMode: p.FailureMode,
			Status:      "open",
		}
		state.blocks[p.ProposalID][evt.Actor.ActorID] = block
		state.stances[p.ProposalID][evt.Actor.ActorID] = StanceBlock
		state.reasons[p.ProposalID][evt.Actor.ActorID] = nil
		touchParticipant(state, evt, nil)
	case "block.withdrawn":
		var p BlockWithdrawnPayload
		if err := decodePayload(evt, &p); err != nil {
			return err
		}
		ensureProposalState(state, p.ProposalID)
		delete(state.blocks[p.ProposalID], evt.Actor.ActorID)
		state.stances[p.ProposalID][evt.Actor.ActorID] = StanceUndeclared
		reason := p.Reason
		state.reasons[p.ProposalID][evt.Actor.ActorID] = &reason
		touchParticipant(state, evt, nil)
	case "consent.given":
		var p ConsentPayload
		if err := decodePayload(evt, &p); err != nil {
			return err
		}
		ensureProposalState(state, p.ProposalID)
		state.stances[p.ProposalID][evt.Actor.ActorID] = StanceConsent
		state.reasons[p.ProposalID][evt.Actor.ActorID] = p.Reason
		touchParticipant(state, evt, nil)
	case "stand_aside.given":
		var p StandAsidePayload
		if err := decodePayload(evt, &p); err != nil {
			return err
		}
		ensureProposalState(state, p.ProposalID)
		state.stances[p.ProposalID][evt.Actor.ActorID] = StanceStandAside
		reason := p.Reason
		state.reasons[p.ProposalID][evt.Actor.ActorID] = &reason
		touchParticipant(state, evt, nil)
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
	proposals := make([]ProposalSnapshot, 0, len(state.proposalOrder))
	for _, id := range state.proposalOrder {
		if p, ok := state.proposals[id]; ok {
			proposals = append(proposals, p)
		}
	}
	state.snapshot.Proposals = proposals

	if state.activeID == "" && len(proposals) > 0 {
		state.activeID = proposals[len(proposals)-1].ProposalID
	}
	if state.activeID != "" {
		if p, ok := state.proposals[state.activeID]; ok {
			cp := p
			state.snapshot.ActiveProposal = &cp
		} else {
			state.snapshot.ActiveProposal = nil
		}
	} else {
		state.snapshot.ActiveProposal = nil
	}

	participants := make([]ParticipantSnapshot, 0, len(state.participants))
	for _, p := range state.participants {
		cp := *p
		cp.Stance = StanceUndeclared
		cp.FinalReason = nil
		if state.activeID != "" {
			if perProposal, ok := state.stances[state.activeID]; ok {
				if stance, ok := perProposal[cp.ActorID]; ok {
					cp.Stance = stance
				}
			}
			if perProposalReasons, ok := state.reasons[state.activeID]; ok {
				if reason, ok := perProposalReasons[cp.ActorID]; ok {
					cp.FinalReason = reason
				}
			}
		}
		participants = append(participants, cp)
	}
	sort.Slice(participants, func(i, j int) bool { return participants[i].ActorID < participants[j].ActorID })
	state.snapshot.Participants = participants

	blocks := make([]BlockSnapshot, 0)
	if state.activeID != "" {
		for _, b := range state.blocks[state.activeID] {
			blocks = append(blocks, b)
		}
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
	undeclared := 0
	for _, p := range s.Participants {
		switch p.Stance {
		case StanceConsent:
			consents++
		case StanceStandAside:
			// stand-asides don't block closure
		default:
			undeclared++
		}
	}

	total := len(s.Participants)

	switch s.DecisionRule {
	case RuleUnanimity:
		// Quaker unanimity: all must declare (consent or stand-aside),
		// no undeclared participants. At least one consent required.
		return consents > 0 && undeclared == 0

	case RuleQuorum:
		// Quorum: consent percentage must meet threshold.
		// Default threshold is 50% if not specified.
		threshold := 50
		if s.QuorumThreshold != nil {
			threshold = *s.QuorumThreshold
		}
		return consents*100 >= threshold*total

	case RuleTimeboxed:
		// Timeboxed: if deadline has passed, ready with at least 1 consent.
		// Before deadline, behaves like unanimity.
		if deadlineReached(s.Deadline) {
			return consents > 0
		}
		return consents > 0 && undeclared == 0

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

func ensureProposalState(state *reducerState, proposalID string) {
	if proposalID == "" {
		return
	}
	if _, ok := state.stances[proposalID]; !ok {
		state.stances[proposalID] = map[string]Stance{}
	}
	if _, ok := state.reasons[proposalID]; !ok {
		state.reasons[proposalID] = map[string]*string{}
	}
	if _, ok := state.blocks[proposalID]; !ok {
		state.blocks[proposalID] = map[string]BlockSnapshot{}
	}
}

func proposalExists(proposals []ProposalSnapshot, proposalID string) bool {
	for _, p := range proposals {
		if p.ProposalID == proposalID {
			return true
		}
	}
	return false
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
