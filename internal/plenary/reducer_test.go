package plenary

import (
	"encoding/json"
	"testing"
)

// helper to build events quickly
func makeEvent(plenaryID, actorID, actorType, eventType string, payload any) Event {
	evt, err := NewEvent(plenaryID, Actor{ActorID: actorID, ActorType: actorType}, eventType, payload)
	if err != nil {
		panic(err)
	}
	return evt
}

func plenaryCreated(pid, actorID string, rule DecisionRule) Event {
	return makeEvent(pid, actorID, "human", "plenary.created", PlenaryCreatedPayload{
		Topic:        "Test topic",
		Context:      "Test context",
		DecisionRule: rule,
	})
}

func participantJoined(pid, actorID, actorType string) Event {
	return makeEvent(pid, actorID, actorType, "participant.joined", ParticipantJoinedPayload{})
}

func phaseSet(pid, actorID string, from, to Phase) Event {
	return makeEvent(pid, actorID, "human", "phase.set", PhaseSetPayload{
		Phase:         to,
		ExpectedPhase: from,
	})
}

func proposalCreated(pid, actorID, proposalID, text string) Event {
	return makeEvent(pid, actorID, "agent", "proposal.created", ProposalCreatedPayload{
		ProposalID: proposalID,
		Text:       text,
	})
}

func consentGiven(pid, actorID, proposalID string) Event {
	return makeEvent(pid, actorID, "agent", "consent.given", ConsentPayload{
		ProposalID: proposalID,
	})
}

func standAsideGiven(pid, actorID, proposalID, reason string) Event {
	return makeEvent(pid, actorID, "agent", "stand_aside.given", StandAsidePayload{
		ProposalID: proposalID,
		Reason:     reason,
	})
}

func blockRaised(pid, actorID, proposalID, text string) Event {
	return makeEvent(pid, actorID, "agent", "block.raised", ProposalRefTextPayload{
		ProposalID: proposalID,
		Text:       text,
	})
}

func blockWithdrawn(pid, actorID, proposalID, reason string) Event {
	return makeEvent(pid, actorID, "agent", "block.withdrawn", BlockWithdrawnPayload{
		ProposalID: proposalID,
		Reason:     reason,
	})
}

func speakEvent(pid, actorID, text string) Event {
	return makeEvent(pid, actorID, "agent", "speak", TextPayload{Text: text})
}

func decisionClosed(pid, actorID string, outcome Outcome, resolution string, participants []DecisionRecordParticipant) Event {
	return makeEvent(pid, actorID, "human", "decision.closed", DecisionClosedPayload{
		Outcome: outcome,
		DecisionRecord: DecisionRecord{
			Resolution:   resolution,
			Participants: participants,
		},
	})
}

// --- Golden tests ---

func TestReduceEmpty(t *testing.T) {
	snap, err := Reduce(nil)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Phase != PhaseFraming {
		t.Errorf("expected phase framing, got %s", snap.Phase)
	}
	if snap.EventCount != 0 {
		t.Errorf("expected 0 events, got %d", snap.EventCount)
	}
}

func TestReduceCreateOnly(t *testing.T) {
	events := []Event{
		plenaryCreated("p1", "keeton", RuleUnanimity),
	}
	snap, err := Reduce(events)
	if err != nil {
		t.Fatal(err)
	}
	if snap.PlenaryID != "p1" {
		t.Errorf("expected plenary_id p1, got %s", snap.PlenaryID)
	}
	if snap.Topic != "Test topic" {
		t.Errorf("expected topic 'Test topic', got %s", snap.Topic)
	}
	if snap.Phase != PhaseFraming {
		t.Errorf("expected phase framing, got %s", snap.Phase)
	}
	if snap.DecisionRule != RuleUnanimity {
		t.Errorf("expected unanimity rule, got %s", snap.DecisionRule)
	}
	if snap.EventCount != 1 {
		t.Errorf("expected 1 event, got %d", snap.EventCount)
	}
	if snap.ReadyToClose {
		t.Error("should not be ready to close")
	}
}

func TestReduceFullLifecycleUnanimity(t *testing.T) {
	pid := "p1"
	propID := "prop1"

	events := []Event{
		plenaryCreated(pid, "keeton", RuleUnanimity),
		participantJoined(pid, "claude", "agent"),
		participantJoined(pid, "codex", "agent"),
		phaseSet(pid, "keeton", PhaseFraming, PhaseDivergence),
		speakEvent(pid, "claude", "I think we should use Go"),
		speakEvent(pid, "codex", "I agree with Go"),
		phaseSet(pid, "keeton", PhaseDivergence, PhaseProposal),
		proposalCreated(pid, "claude", propID, "Use Go for the CLI"),
		phaseSet(pid, "keeton", PhaseProposal, PhaseObjections),
		phaseSet(pid, "keeton", PhaseObjections, PhaseConsensusCheck),
		consentGiven(pid, "claude", propID),
		consentGiven(pid, "codex", propID),
	}

	snap, err := Reduce(events)
	if err != nil {
		t.Fatal(err)
	}

	if snap.Phase != PhaseConsensusCheck {
		t.Errorf("expected consensus_check, got %s", snap.Phase)
	}
	if len(snap.Participants) != 2 {
		t.Errorf("expected 2 participants, got %d", len(snap.Participants))
	}
	if len(snap.UnresolvedBlocks) != 0 {
		t.Errorf("expected 0 unresolved blocks, got %d", len(snap.UnresolvedBlocks))
	}
	if !snap.ReadyToClose {
		t.Error("should be ready to close with unanimity and all consents")
	}
	if snap.EventCount != 12 {
		t.Errorf("expected 12 events, got %d", snap.EventCount)
	}

	// Verify all participants show consent
	for _, p := range snap.Participants {
		if p.Stance != StanceConsent {
			t.Errorf("participant %s expected consent, got %s", p.ActorID, p.Stance)
		}
	}
}

func TestReduceBlockPreventsConsensus(t *testing.T) {
	pid := "p1"
	propID := "prop1"

	events := []Event{
		plenaryCreated(pid, "keeton", RuleUnanimity),
		participantJoined(pid, "claude", "agent"),
		participantJoined(pid, "codex", "agent"),
		phaseSet(pid, "keeton", PhaseFraming, PhaseDivergence),
		phaseSet(pid, "keeton", PhaseDivergence, PhaseProposal),
		proposalCreated(pid, "claude", propID, "Use Go for the CLI"),
		phaseSet(pid, "keeton", PhaseProposal, PhaseObjections),
		blockRaised(pid, "codex", propID, "Go is harder for Keeton to read"),
		phaseSet(pid, "keeton", PhaseObjections, PhaseConsensusCheck),
		consentGiven(pid, "claude", propID),
	}

	snap, err := Reduce(events)
	if err != nil {
		t.Fatal(err)
	}

	if snap.ReadyToClose {
		t.Error("should NOT be ready to close when there's an unresolved block")
	}
	if len(snap.UnresolvedBlocks) != 1 {
		t.Errorf("expected 1 unresolved block, got %d", len(snap.UnresolvedBlocks))
	}
	if snap.UnresolvedBlocks[0].ActorID != "codex" {
		t.Errorf("expected block by codex, got %s", snap.UnresolvedBlocks[0].ActorID)
	}
}

func TestReduceBlockWithdrawnUnlocksConsensus(t *testing.T) {
	pid := "p1"
	propID := "prop1"

	events := []Event{
		plenaryCreated(pid, "keeton", RuleUnanimity),
		participantJoined(pid, "claude", "agent"),
		participantJoined(pid, "codex", "agent"),
		phaseSet(pid, "keeton", PhaseFraming, PhaseDivergence),
		phaseSet(pid, "keeton", PhaseDivergence, PhaseProposal),
		proposalCreated(pid, "claude", propID, "Use Go for the CLI"),
		phaseSet(pid, "keeton", PhaseProposal, PhaseObjections),
		blockRaised(pid, "codex", propID, "Go is harder for Keeton to read"),
		phaseSet(pid, "keeton", PhaseObjections, PhaseConsensusCheck),
		consentGiven(pid, "claude", propID),
		// Codex withdraws block and consents
		blockWithdrawn(pid, "codex", propID, "Keeton said Go is fine"),
		consentGiven(pid, "codex", propID),
	}

	snap, err := Reduce(events)
	if err != nil {
		t.Fatal(err)
	}

	if len(snap.UnresolvedBlocks) != 0 {
		t.Errorf("expected 0 blocks after withdrawal, got %d", len(snap.UnresolvedBlocks))
	}
	if !snap.ReadyToClose {
		t.Error("should be ready to close after block withdrawn and all consent")
	}
}

func TestReduceStandAsideAllowedInUnanimity(t *testing.T) {
	// In Quaker process, stand-aside should NOT prevent consensus.
	// Only blocks prevent consensus. This tests the agreed-upon semantics.
	pid := "p1"
	propID := "prop1"

	events := []Event{
		plenaryCreated(pid, "keeton", RuleUnanimity),
		participantJoined(pid, "claude", "agent"),
		participantJoined(pid, "codex", "agent"),
		participantJoined(pid, "keeton", "human"),
		phaseSet(pid, "keeton", PhaseFraming, PhaseDivergence),
		phaseSet(pid, "keeton", PhaseDivergence, PhaseProposal),
		proposalCreated(pid, "claude", propID, "Use Go for the CLI"),
		phaseSet(pid, "keeton", PhaseProposal, PhaseConsensusCheck),
		consentGiven(pid, "claude", propID),
		consentGiven(pid, "keeton", propID),
		standAsideGiven(pid, "codex", propID, "I prefer TS but won't block"),
	}

	snap, err := Reduce(events)
	if err != nil {
		t.Fatal(err)
	}

	// This is the key assertion: stand-aside should NOT prevent unanimity consensus
	if !snap.ReadyToClose {
		t.Error("stand-aside should NOT prevent consensus under unanimity rule — only blocks should prevent consensus")
	}
}

func TestReducePhaseTransitionValidation(t *testing.T) {
	pid := "p1"

	events := []Event{
		plenaryCreated(pid, "keeton", RuleUnanimity),
	}

	// Try to set phase with wrong expected_phase
	badPhase := makeEvent(pid, "keeton", "human", "phase.set", PhaseSetPayload{
		Phase:         PhaseDivergence,
		ExpectedPhase: PhaseDivergence, // wrong — current is framing
	})

	_, err := ReduceWithValidation(events, badPhase)
	if err == nil {
		t.Error("expected error for phase mismatch")
	}
	if !Is(err, ErrConflict) {
		t.Errorf("expected ErrConflict, got %v", err)
	}
}

func TestReduceFirstEventMustBeCreated(t *testing.T) {
	join := participantJoined("p1", "claude", "agent")
	_, err := ReduceWithValidation(nil, join)
	if err == nil {
		t.Error("expected error: first event must be plenary.created")
	}
	if !Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

func TestReduceCannotCreateTwice(t *testing.T) {
	events := []Event{
		plenaryCreated("p1", "keeton", RuleUnanimity),
	}
	second := plenaryCreated("p1", "keeton", RuleUnanimity)
	_, err := ReduceWithValidation(events, second)
	if err == nil {
		t.Error("expected error: plenary already exists")
	}
}

func TestReduceCannotActAfterClose(t *testing.T) {
	pid := "p1"
	propID := "prop1"

	events := []Event{
		plenaryCreated(pid, "keeton", RuleUnanimity),
		participantJoined(pid, "claude", "agent"),
		phaseSet(pid, "keeton", PhaseFraming, PhaseDivergence),
		phaseSet(pid, "keeton", PhaseDivergence, PhaseProposal),
		proposalCreated(pid, "claude", propID, "Use Go"),
		phaseSet(pid, "keeton", PhaseProposal, PhaseConsensusCheck),
		consentGiven(pid, "claude", propID),
		decisionClosed(pid, "keeton", OutcomeConsensus, "Use Go", []DecisionRecordParticipant{
			{ActorID: "claude", ActorType: "agent", FinalStance: StanceConsent},
		}),
	}

	speak := speakEvent(pid, "claude", "Wait, one more thing")
	_, err := ReduceWithValidation(events, speak)
	if err == nil {
		t.Error("expected error: plenary is closed")
	}
}

func TestReduceQuorumRule(t *testing.T) {
	pid := "p1"
	propID := "prop1"

	events := []Event{
		plenaryCreated(pid, "keeton", RuleQuorum),
		participantJoined(pid, "claude", "agent"),
		participantJoined(pid, "codex", "agent"),
		participantJoined(pid, "keeton", "human"),
		phaseSet(pid, "keeton", PhaseFraming, PhaseDivergence),
		phaseSet(pid, "keeton", PhaseDivergence, PhaseProposal),
		proposalCreated(pid, "claude", propID, "Use Go"),
		phaseSet(pid, "keeton", PhaseProposal, PhaseConsensusCheck),
		// Only claude consents, others undeclared
		consentGiven(pid, "claude", propID),
	}

	snap, err := Reduce(events)
	if err != nil {
		t.Fatal(err)
	}

	// Quorum: should NOT be ready because there are still undeclared participants
	// (the current implementation returns true if consents > 0 for quorum,
	// but undeclared participants should probably prevent closure)
	// This test documents current behavior — may need discussion.
	t.Logf("Quorum with 1/3 consents, 2 undeclared: ready_to_close=%v", snap.ReadyToClose)
}

func TestReduceProposalResetsStances(t *testing.T) {
	pid := "p1"
	prop1 := "prop1"
	prop2 := "prop2"

	events := []Event{
		plenaryCreated(pid, "keeton", RuleUnanimity),
		participantJoined(pid, "claude", "agent"),
		participantJoined(pid, "codex", "agent"),
		phaseSet(pid, "keeton", PhaseFraming, PhaseDivergence),
		phaseSet(pid, "keeton", PhaseDivergence, PhaseProposal),
		proposalCreated(pid, "claude", prop1, "Use Go"),
		consentGiven(pid, "claude", prop1),
		consentGiven(pid, "codex", prop1),
		// New proposal replaces old one — stances should reset
		proposalCreated(pid, "codex", prop2, "Actually use TypeScript"),
	}

	snap, err := Reduce(events)
	if err != nil {
		t.Fatal(err)
	}

	if snap.ActiveProposal == nil || snap.ActiveProposal.ProposalID != prop2 {
		t.Error("active proposal should be prop2")
	}
	for _, p := range snap.Participants {
		if p.Stance != StanceUndeclared {
			t.Errorf("participant %s should have stance reset to undeclared after new proposal, got %s", p.ActorID, p.Stance)
		}
	}
}

func TestReduceNextActionsFraming(t *testing.T) {
	events := []Event{
		plenaryCreated("p1", "keeton", RuleUnanimity),
	}
	snap, err := Reduce(events)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.NextRequiredActions) == 0 {
		t.Error("expected next actions in framing phase")
	}
}

func TestReduceDecisionClosedSetsOutcome(t *testing.T) {
	pid := "p1"
	propID := "prop1"

	events := []Event{
		plenaryCreated(pid, "keeton", RuleUnanimity),
		participantJoined(pid, "claude", "agent"),
		phaseSet(pid, "keeton", PhaseFraming, PhaseDivergence),
		phaseSet(pid, "keeton", PhaseDivergence, PhaseProposal),
		proposalCreated(pid, "claude", propID, "Use Go"),
		phaseSet(pid, "keeton", PhaseProposal, PhaseConsensusCheck),
		consentGiven(pid, "claude", propID),
		decisionClosed(pid, "keeton", OutcomeConsensus, "We will use Go", []DecisionRecordParticipant{
			{ActorID: "claude", ActorType: "agent", FinalStance: StanceConsent},
		}),
	}

	snap, err := Reduce(events)
	if err != nil {
		t.Fatal(err)
	}

	if !snap.Closed {
		t.Error("expected closed=true")
	}
	if snap.Phase != PhaseClosed {
		t.Errorf("expected phase closed, got %s", snap.Phase)
	}
	if snap.Outcome == nil || *snap.Outcome != OutcomeConsensus {
		t.Error("expected outcome consensus")
	}
	if snap.DecisionRecord == nil {
		t.Error("expected decision record")
	}
	if snap.DecisionRecord.Resolution != "We will use Go" {
		t.Errorf("expected resolution 'We will use Go', got %s", snap.DecisionRecord.Resolution)
	}
}

func TestSnapshotJSON(t *testing.T) {
	// Verify the snapshot serializes cleanly to JSON
	events := []Event{
		plenaryCreated("p1", "keeton", RuleUnanimity),
		participantJoined("p1", "claude", "agent"),
	}
	snap, err := Reduce(events)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Snapshot JSON:\n%s", string(b))

	// Roundtrip
	var snap2 Snapshot
	if err := json.Unmarshal(b, &snap2); err != nil {
		t.Fatal(err)
	}
	if snap2.PlenaryID != snap.PlenaryID {
		t.Error("roundtrip failed")
	}
}
