export interface PlenarySummary {
  plenary_id: string;
  topic: string;
  phase: string;
  decision_rule: string;
  closed: boolean;
  event_count: number;
  last_event_at?: string;
  outcome?: string;
}

export interface Participant {
  actor_id: string;
  actor_type: string;
  role?: string;
  lens?: string;
  stance: string;
  final_reason?: string;
  last_event_at: string;
}

export interface Proposal {
  proposal_id: string;
  text: string;
  acceptance_criteria?: string;
}

export interface Block {
  actor_id: string;
  text: string;
  principle?: string;
  failure_mode?: string;
  status: string;
}

export interface DecisionRecord {
  resolution: string;
  rationale_bullets?: string[];
  participants: Array<{
    actor_id: string;
    actor_type: string;
    final_stance: string;
    final_reason?: string;
  }>;
}

export interface Snapshot {
  plenary_id: string;
  topic: string;
  context?: string;
  phase: string;
  decision_rule: string;
  deadline?: string;
  participants: Participant[];
  proposals?: Proposal[];
  active_proposal?: Proposal;
  unresolved_blocks: Block[];
  open_questions?: string[];
  ready_to_close: boolean;
  next_required_actions?: string[];
  closed: boolean;
  outcome?: string;
  decision_record?: DecisionRecord;
  event_count: number;
}

export interface PlenaryEvent {
  event_id: string;
  plenary_id: string;
  ts: string;
  actor: { actor_id: string; actor_type: string };
  event_type: string;
  payload: Record<string, unknown>;
}
