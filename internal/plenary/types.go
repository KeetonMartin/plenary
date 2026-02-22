package plenary

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

type Phase string

const (
	PhaseFraming        Phase = "framing"
	PhaseDivergence     Phase = "divergence"
	PhaseProposal       Phase = "proposal"
	PhaseObjections     Phase = "objections"
	PhaseConsensusCheck Phase = "consensus_check"
	PhaseClosed         Phase = "closed"
)

type DecisionRule string

const (
	RuleUnanimity DecisionRule = "unanimity"
	RuleQuorum    DecisionRule = "quorum"
	RuleTimeboxed DecisionRule = "timeboxed"
)

type Outcome string

const (
	OutcomeConsensus     Outcome = "consensus"
	OutcomeOwnerDecision Outcome = "owner_decision"
	OutcomeAbandoned     Outcome = "abandoned"
)

type Stance string

const (
	StanceUndeclared Stance = "undeclared"
	StanceConsent    Stance = "consent"
	StanceStandAside Stance = "stand_aside"
	StanceBlock      Stance = "block"
)

type Actor struct {
	ActorID     string  `json:"actor_id"`
	ActorType   string  `json:"actor_type"`
	DelegatorID *string `json:"delegator_id,omitempty"`
}

type Integrity struct {
	Hash     string `json:"hash,omitempty"`
	Sig      string `json:"sig,omitempty"`
	PrevHash string `json:"prev_hash,omitempty"`
}

type Event struct {
	EventID   string          `json:"event_id"`
	PlenaryID string          `json:"plenary_id"`
	TS        string          `json:"ts"`
	Actor     Actor           `json:"actor"`
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
	Integrity *Integrity      `json:"integrity,omitempty"`
}

func NewEvent(plenaryID string, actor Actor, eventType string, payload any) (Event, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Event{}, err
	}
	return Event{
		EventID:   NewUUIDLike(),
		PlenaryID: plenaryID,
		TS:        time.Now().UTC().Format(time.RFC3339Nano),
		Actor:     actor,
		EventType: eventType,
		Payload:   raw,
	}, nil
}

func NewUUIDLike() string {
	// RFC4122-ish random UUID (v4 style) without external deps.
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	hexs := hex.EncodeToString(b[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", hexs[0:8], hexs[8:12], hexs[12:16], hexs[16:20], hexs[20:32])
}

type PlenaryCreatedPayload struct {
	Topic        string       `json:"topic"`
	Context      string       `json:"context,omitempty"`
	DecisionRule DecisionRule `json:"decision_rule"`
	Deadline     *string      `json:"deadline,omitempty"`
}

type ParticipantJoinedPayload struct {
	Role *string `json:"role,omitempty"`
	Lens *string `json:"lens,omitempty"`
}

type PhaseSetPayload struct {
	Phase         Phase `json:"phase"`
	ExpectedPhase Phase `json:"expected_phase"`
}

type TextPayload struct {
	Text string `json:"text"`
}

type ProposalCreatedPayload struct {
	ProposalID          string  `json:"proposal_id"`
	Text                string  `json:"text"`
	AcceptanceCriteria  *string `json:"acceptance_criteria,omitempty"`
}

type ProposalRefTextPayload struct {
	ProposalID string  `json:"proposal_id"`
	Text       string  `json:"text"`
	Principle  *string `json:"principle,omitempty"`
	FailureMode *string `json:"failure_mode,omitempty"`
}

type BlockWithdrawnPayload struct {
	ProposalID string `json:"proposal_id"`
	Reason     string `json:"reason"`
}

type ConsentPayload struct {
	ProposalID string  `json:"proposal_id"`
	Reason     *string `json:"reason,omitempty"`
}

type StandAsidePayload struct {
	ProposalID string `json:"proposal_id"`
	Reason     string `json:"reason"`
}

type SummaryPayload struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

type DecisionRecordParticipant struct {
	ActorID     string  `json:"actor_id"`
	ActorType   string  `json:"actor_type"`
	Role        *string `json:"role,omitempty"`
	FinalStance Stance  `json:"final_stance"`
	FinalReason *string `json:"final_reason,omitempty"`
}

type DecisionRecordObjection struct {
	ActorID        string  `json:"actor_id"`
	Text           string  `json:"text"`
	Status         string  `json:"status"`
	ResolutionNote *string `json:"resolution_note,omitempty"`
}

type DecisionRecordActionItem struct {
	Text    string  `json:"text"`
	OwnerID *string `json:"owner_id,omitempty"`
	DueAt   *string `json:"due_at,omitempty"`
}

type DecisionRecord struct {
	Resolution       string                     `json:"resolution"`
	RationaleBullets []string                   `json:"rationale_bullets,omitempty"`
	Participants     []DecisionRecordParticipant `json:"participants"`
	Objections       []DecisionRecordObjection  `json:"objections,omitempty"`
	ActionItems      []DecisionRecordActionItem `json:"action_items,omitempty"`
}

type DecisionClosedPayload struct {
	Outcome        Outcome        `json:"outcome"`
	DecisionRecord DecisionRecord `json:"decision_record"`
}

