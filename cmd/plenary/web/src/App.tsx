import { useCallback, useEffect, useRef, useState } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

interface PlenarySummary {
  plenary_id: string;
  topic: string;
  phase: string;
  decision_rule: string;
  closed: boolean;
  event_count: number;
  last_event_at?: string;
  outcome?: string;
}

interface Participant {
  actor_id: string;
  actor_type: string;
  role?: string;
  lens?: string;
  stance: string;
  final_reason?: string;
  last_event_at: string;
}

interface Proposal {
  proposal_id: string;
  text: string;
  acceptance_criteria?: string;
}

interface Block {
  actor_id: string;
  text: string;
  principle?: string;
  failure_mode?: string;
  status: string;
}

interface DecisionRecord {
  resolution: string;
  rationale_bullets?: string[];
  participants: Array<{
    actor_id: string;
    actor_type: string;
    final_stance: string;
    final_reason?: string;
  }>;
}

interface Snapshot {
  plenary_id: string;
  topic: string;
  context?: string;
  phase: string;
  decision_rule: string;
  deadline?: string;
  participants: Participant[];
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

interface PlenaryEvent {
  event_id: string;
  plenary_id: string;
  ts: string;
  actor: { actor_id: string; actor_type: string };
  event_type: string;
  payload: Record<string, unknown>;
}

const PHASES = ["framing", "divergence", "proposal", "consensus_check", "closed"];

function phaseIndex(phase: string): number {
  return PHASES.indexOf(phase);
}

function phaseBadgeVariant(
  phase: string
): "default" | "secondary" | "destructive" | "outline" {
  if (phase === "closed") return "default";
  if (phase === "consensus_check") return "destructive";
  return "secondary";
}

function stanceBadgeVariant(
  stance: string
): "default" | "secondary" | "destructive" | "outline" {
  if (stance === "consent") return "default";
  if (stance === "block") return "destructive";
  if (stance === "stand_aside") return "outline";
  return "secondary";
}

function eventTypeColor(eventType: string): string {
  switch (eventType) {
    case "plenary.created":
      return "border-blue-400";
    case "participant.joined":
      return "border-green-400";
    case "phase.set":
      return "border-purple-400";
    case "speak":
    case "position.submitted":
    case "concern.raised":
      return "border-gray-400";
    case "proposal.created":
      return "border-yellow-400";
    case "consent.given":
      return "border-green-500";
    case "block.raised":
      return "border-red-500";
    case "block.withdrawn":
      return "border-orange-400";
    case "stand_aside.given":
      return "border-orange-300";
    case "decision.closed":
      return "border-blue-600";
    default:
      return "border-muted";
  }
}

function eventTypeIcon(eventType: string): string {
  switch (eventType) {
    case "plenary.created": return "+";
    case "participant.joined": return ">";
    case "phase.set": return ">>";
    case "speak": return '"';
    case "proposal.created": return "P";
    case "consent.given": return "\u2713";
    case "block.raised": return "\u2717";
    case "block.withdrawn": return "\u21B6";
    case "stand_aside.given": return "\u2192";
    case "decision.closed": return "\u2605";
    default: return "\u00B7";
  }
}

function relativeTime(ts: string): string {
  const diff = Date.now() - new Date(ts).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

// --- Phase Stepper ---
function PhaseStepper({ currentPhase }: { currentPhase: string }) {
  const current = phaseIndex(currentPhase);
  return (
    <div className="flex items-center gap-1 w-full">
      {PHASES.map((phase, i) => {
        const isActive = i === current;
        const isComplete = i < current;
        return (
          <div key={phase} className="flex items-center gap-1 flex-1">
            <div
              className={`flex items-center justify-center rounded-full text-xs font-medium h-7 w-7 shrink-0 ${
                isActive
                  ? "bg-primary text-primary-foreground"
                  : isComplete
                  ? "bg-primary/20 text-primary"
                  : "bg-muted text-muted-foreground"
              }`}
            >
              {isComplete ? "\u2713" : i + 1}
            </div>
            <span
              className={`text-xs truncate ${
                isActive ? "font-semibold" : "text-muted-foreground"
              }`}
            >
              {phase.replace("_", " ")}
            </span>
            {i < PHASES.length - 1 && (
              <div
                className={`h-px flex-1 ${
                  isComplete ? "bg-primary/40" : "bg-muted"
                }`}
              />
            )}
          </div>
        );
      })}
    </div>
  );
}

// --- Stance Summary ---
function StanceSummary({ participants }: { participants: Participant[] }) {
  const counts = { consent: 0, block: 0, stand_aside: 0, undeclared: 0 };
  for (const p of participants) {
    if (p.stance in counts) {
      counts[p.stance as keyof typeof counts]++;
    } else {
      counts.undeclared++;
    }
  }
  return (
    <div className="flex gap-3 text-sm">
      {counts.consent > 0 && (
        <span className="text-green-600 font-medium">{counts.consent} consent</span>
      )}
      {counts.block > 0 && (
        <span className="text-red-600 font-medium">{counts.block} block</span>
      )}
      {counts.stand_aside > 0 && (
        <span className="text-orange-500 font-medium">{counts.stand_aside} stand aside</span>
      )}
      {counts.undeclared > 0 && (
        <span className="text-muted-foreground">{counts.undeclared} undeclared</span>
      )}
    </div>
  );
}

// --- Plenary List ---
function PlenaryList({
  plenaries,
  onSelect,
}: {
  plenaries: PlenarySummary[];
  onSelect: (id: string) => void;
}) {
  if (plenaries.length === 0) {
    return (
      <Card>
        <CardContent className="p-6 text-center text-muted-foreground">
          No plenaries found. Create one with{" "}
          <code className="text-sm bg-muted px-1 py-0.5 rounded">
            plenary create --topic "..."
          </code>
        </CardContent>
      </Card>
    );
  }

  // Sort: open first, then by last event
  const sorted = [...plenaries].sort((a, b) => {
    if (a.closed !== b.closed) return a.closed ? 1 : -1;
    const ta = a.last_event_at || "";
    const tb = b.last_event_at || "";
    return tb.localeCompare(ta);
  });

  return (
    <div className="space-y-3">
      {sorted.map((p) => (
        <Card
          key={p.plenary_id}
          className={`cursor-pointer transition-colors hover:border-primary/50 ${
            p.closed ? "opacity-60" : ""
          }`}
          onClick={() => onSelect(p.plenary_id)}
        >
          <CardContent className="p-4">
            <div className="flex items-start justify-between gap-3">
              <div className="flex-1 min-w-0">
                <p className="font-medium text-sm leading-tight truncate">
                  {p.topic}
                </p>
                <div className="flex items-center gap-2 mt-1.5">
                  <Badge variant={phaseBadgeVariant(p.phase)} className="text-xs">
                    {p.phase.replace("_", " ")}
                  </Badge>
                  <Badge variant="outline" className="text-xs">
                    {p.decision_rule}
                  </Badge>
                  {p.outcome && (
                    <Badge variant="default" className="text-xs">{p.outcome}</Badge>
                  )}
                  <span className="text-xs text-muted-foreground">
                    {p.event_count} events
                  </span>
                </div>
              </div>
              {p.last_event_at && (
                <span className="text-xs text-muted-foreground whitespace-nowrap">
                  {relativeTime(p.last_event_at)}
                </span>
              )}
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}

// --- Plenary Detail ---
function PlenaryDetail({
  snapshot,
  events,
  onBack,
  liveIndicator,
}: {
  snapshot: Snapshot;
  events: PlenaryEvent[];
  onBack: () => void;
  liveIndicator: boolean;
}) {
  const timelineEndRef = useRef<HTMLDivElement>(null);
  const [expandedEvent, setExpandedEvent] = useState<string | null>(null);

  useEffect(() => {
    timelineEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [events.length]);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <button
          onClick={onBack}
          className="text-sm text-muted-foreground hover:text-foreground"
        >
          &larr; Back to list
        </button>
        {liveIndicator && (
          <div className="flex items-center gap-1.5 text-xs text-green-600">
            <span className="relative flex h-2 w-2">
              <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-75"></span>
              <span className="relative inline-flex rounded-full h-2 w-2 bg-green-500"></span>
            </span>
            Live
          </div>
        )}
      </div>

      {/* Header + Phase Stepper */}
      <Card>
        <CardHeader>
          <div className="flex items-start justify-between gap-3">
            <CardTitle className="text-base">{snapshot.topic}</CardTitle>
            <div className="flex gap-2 shrink-0">
              {snapshot.outcome && (
                <Badge variant="default">{snapshot.outcome}</Badge>
              )}
            </div>
          </div>
          {snapshot.context && (
            <CardDescription>{snapshot.context}</CardDescription>
          )}
        </CardHeader>
        <CardContent>
          <PhaseStepper currentPhase={snapshot.phase} />
          <div className="grid grid-cols-3 gap-4 text-sm mt-4">
            <div>
              <span className="text-muted-foreground">Rule:</span>{" "}
              <span className="font-medium">{snapshot.decision_rule}</span>
            </div>
            <div>
              <span className="text-muted-foreground">Events:</span>{" "}
              {snapshot.event_count}
            </div>
            <div>
              <span className="text-muted-foreground">Ready to close:</span>{" "}
              <span className={snapshot.ready_to_close ? "text-green-600 font-medium" : ""}>
                {snapshot.ready_to_close ? "Yes" : "No"}
              </span>
            </div>
          </div>
          {snapshot.deadline && (
            <div className="text-sm mt-2">
              <span className="text-muted-foreground">Deadline:</span>{" "}
              {new Date(snapshot.deadline).toLocaleString()}
            </div>
          )}
          {snapshot.next_required_actions &&
            snapshot.next_required_actions.length > 0 && (
              <div className="mt-3 p-2 bg-muted/50 rounded text-sm">
                <span className="font-medium">Next:</span>{" "}
                {snapshot.next_required_actions.join(", ")}
              </div>
            )}
        </CardContent>
      </Card>

      {/* Decision Record */}
      {snapshot.decision_record && (
        <Card className="border-blue-200 bg-blue-50/30">
          <CardHeader>
            <CardTitle className="text-lg">Decision Record</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="font-medium">{snapshot.decision_record.resolution}</p>
            {snapshot.decision_record.rationale_bullets &&
              snapshot.decision_record.rationale_bullets.length > 0 && (
                <ul className="list-disc list-inside text-sm mt-2 text-muted-foreground">
                  {snapshot.decision_record.rationale_bullets.map((b, i) => (
                    <li key={i}>{b}</li>
                  ))}
                </ul>
              )}
            {snapshot.decision_record.participants.length > 0 && (
              <div className="mt-3">
                <p className="text-sm text-muted-foreground mb-1">Final stances:</p>
                <div className="flex flex-wrap gap-2">
                  {snapshot.decision_record.participants.map((p) => (
                    <div key={p.actor_id} className="flex items-center gap-1">
                      <span className="text-sm font-medium">{p.actor_id}</span>
                      <Badge variant={stanceBadgeVariant(p.final_stance)} className="text-xs">
                        {p.final_stance}
                      </Badge>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {/* Active Proposal */}
      {snapshot.active_proposal && (
        <Card className="border-yellow-200 bg-yellow-50/30">
          <CardHeader>
            <CardTitle className="text-lg">Active Proposal</CardTitle>
            <CardDescription className="font-mono text-xs">
              {snapshot.active_proposal.proposal_id}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <p className="whitespace-pre-wrap">{snapshot.active_proposal.text}</p>
            {snapshot.active_proposal.acceptance_criteria && (
              <p className="text-sm text-muted-foreground mt-2">
                <span className="font-medium">Criteria:</span>{" "}
                {snapshot.active_proposal.acceptance_criteria}
              </p>
            )}
            {snapshot.participants.length > 0 && (
              <div className="mt-3">
                <StanceSummary participants={snapshot.participants} />
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {/* Participants */}
      <Card>
        <CardHeader>
          <CardTitle className="text-lg">
            Participants ({snapshot.participants.length})
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid gap-2">
            {snapshot.participants.map((p) => (
              <div
                key={p.actor_id}
                className="flex items-center justify-between p-2 rounded bg-muted/30"
              >
                <div className="flex items-center gap-2">
                  <span className="font-medium text-sm">{p.actor_id}</span>
                  <Badge variant="outline" className="text-xs">
                    {p.actor_type}
                  </Badge>
                  {p.role && (
                    <span className="text-xs text-muted-foreground">({p.role})</span>
                  )}
                </div>
                <div className="flex items-center gap-2">
                  <Badge variant={stanceBadgeVariant(p.stance)} className="text-xs">
                    {p.stance}
                  </Badge>
                  {p.final_reason && (
                    <span className="text-xs text-muted-foreground max-w-48 truncate">
                      {p.final_reason}
                    </span>
                  )}
                </div>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      {/* Unresolved Blocks */}
      {snapshot.unresolved_blocks.length > 0 && (
        <Card className="border-red-200">
          <CardHeader>
            <CardTitle className="text-lg text-red-700">
              Unresolved Blocks ({snapshot.unresolved_blocks.length})
            </CardTitle>
          </CardHeader>
          <CardContent>
            {snapshot.unresolved_blocks.map((b, i) => (
              <div key={i} className="mb-3 last:mb-0 p-3 bg-red-50 border border-red-200 rounded">
                <div className="flex items-center gap-2 mb-1">
                  <span className="font-medium text-sm">{b.actor_id}</span>
                  <Badge variant="destructive" className="text-xs">block</Badge>
                </div>
                <p className="text-sm">{b.text}</p>
                {b.principle && (
                  <p className="text-xs text-muted-foreground mt-1">
                    Principle: {b.principle}
                  </p>
                )}
                {b.failure_mode && (
                  <p className="text-xs text-muted-foreground">
                    Failure mode: {b.failure_mode}
                  </p>
                )}
              </div>
            ))}
          </CardContent>
        </Card>
      )}

      {/* Event Timeline */}
      <Card>
        <CardHeader>
          <CardTitle className="text-lg">
            Event Timeline ({events.length})
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-1">
            {events.map((evt) => (
              <div
                key={evt.event_id}
                className={`flex items-start gap-2 text-sm border-l-3 ${eventTypeColor(
                  evt.event_type
                )} pl-3 py-1.5 cursor-pointer hover:bg-muted/30 rounded-r transition-colors`}
                onClick={() =>
                  setExpandedEvent(
                    expandedEvent === evt.event_id ? null : evt.event_id
                  )
                }
              >
                <span className="w-5 text-center shrink-0 font-mono text-xs mt-0.5">
                  {eventTypeIcon(evt.event_type)}
                </span>
                <span className="text-muted-foreground font-mono text-xs whitespace-nowrap mt-0.5">
                  {new Date(evt.ts).toLocaleTimeString()}
                </span>
                <Badge variant="outline" className="text-xs shrink-0">
                  {evt.event_type}
                </Badge>
                <span className="font-medium shrink-0 text-xs mt-0.5">
                  {evt.actor.actor_id}
                </span>
                {expandedEvent !== evt.event_id && (
                  <span className="text-muted-foreground truncate text-xs mt-0.5">
                    {payloadSummary(evt)}
                  </span>
                )}
              </div>
            ))}
            {events.map(
              (evt) =>
                expandedEvent === evt.event_id && (
                  <div
                    key={`${evt.event_id}-detail`}
                    className="ml-8 mb-2 p-3 bg-muted/50 rounded text-xs font-mono whitespace-pre-wrap break-all"
                  >
                    {JSON.stringify(evt.payload, null, 2)}
                  </div>
                )
            )}
            <div ref={timelineEndRef} />
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

function payloadSummary(evt: PlenaryEvent): string {
  const p = evt.payload;
  switch (evt.event_type) {
    case "plenary.created":
      return `topic="${p.topic}"`;
    case "participant.joined":
      return p.role ? `role="${p.role}"` : "joined";
    case "phase.set":
      return `${p.expected_phase} \u2192 ${p.phase}`;
    case "proposal.created":
      return `"${String(p.text).slice(0, 100)}"`;
    case "consent.given":
      return p.reason ? `reason: "${p.reason}"` : "consented";
    case "stand_aside.given":
      return `"${p.reason}"`;
    case "block.raised":
      return `"${String(p.text).slice(0, 100)}"`;
    case "block.withdrawn":
      return `"${p.reason}"`;
    case "speak":
      return `"${String(p.text).slice(0, 120)}"`;
    case "decision.closed":
      return `outcome=${p.outcome}`;
    default:
      return JSON.stringify(p).slice(0, 80);
  }
}

function App() {
  const [plenaries, setPlenaries] = useState<PlenarySummary[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null);
  const [events, setEvents] = useState<PlenaryEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [sseConnected, setSSEConnected] = useState(false);
  const sseRef = useRef<EventSource | null>(null);

  // Fetch plenaries list
  const fetchPlenaries = useCallback(() => {
    fetch("/api/plenaries")
      .then((r) => r.json())
      .then((data) => {
        setPlenaries(data || []);
        setLoading(false);
      })
      .catch(() => setLoading(false));
  }, []);

  useEffect(() => {
    fetchPlenaries();
    // Auto-refresh list every 5s
    const interval = setInterval(fetchPlenaries, 5000);
    return () => clearInterval(interval);
  }, [fetchPlenaries]);

  // Fetch detail + connect SSE when plenary is selected
  useEffect(() => {
    if (!selectedId) {
      // Cleanup SSE
      if (sseRef.current) {
        sseRef.current.close();
        sseRef.current = null;
        setSSEConnected(false);
      }
      return;
    }

    // Fetch initial data
    Promise.all([
      fetch(`/api/plenaries/${selectedId}`).then((r) => r.json()),
      fetch(`/api/plenaries/${selectedId}/events`).then((r) => r.json()),
    ]).then(([snap, evts]) => {
      setSnapshot(snap);
      setEvents(evts || []);
    });

    // Connect SSE for live updates
    const es = new EventSource(`/api/plenaries/${selectedId}/stream`);
    sseRef.current = es;

    const seenIds = new Set<string>();

    es.onopen = () => setSSEConnected(true);
    es.onerror = () => setSSEConnected(false);
    es.onmessage = (msg) => {
      try {
        const evt: PlenaryEvent = JSON.parse(msg.data);
        if (seenIds.has(evt.event_id)) return;
        seenIds.add(evt.event_id);

        setEvents((prev) => {
          // Deduplicate
          if (prev.some((e) => e.event_id === evt.event_id)) return prev;
          return [...prev, evt];
        });

        // Re-fetch snapshot to get updated derived state
        fetch(`/api/plenaries/${selectedId}`)
          .then((r) => r.json())
          .then(setSnapshot);
      } catch {
        // ignore parse errors
      }
    };

    return () => {
      es.close();
      sseRef.current = null;
      setSSEConnected(false);
    };
  }, [selectedId]);

  return (
    <div className="min-h-screen bg-background">
      <header className="border-b sticky top-0 bg-background/95 backdrop-blur z-10">
        <div className="max-w-4xl mx-auto px-4 py-3 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <h1 className="text-lg font-semibold">Plenary</h1>
            <span className="text-sm text-muted-foreground">
              Consensus protocol for agents
            </span>
          </div>
          <div className="flex items-center gap-3 text-xs text-muted-foreground">
            <span>{plenaries.length} plenaries</span>
            <span>{plenaries.filter((p) => !p.closed).length} open</span>
          </div>
        </div>
      </header>
      <main className="max-w-4xl mx-auto px-4 py-6">
        {loading ? (
          <div className="flex items-center justify-center py-12">
            <p className="text-muted-foreground">Loading...</p>
          </div>
        ) : selectedId && snapshot ? (
          <PlenaryDetail
            snapshot={snapshot}
            events={events}
            liveIndicator={sseConnected}
            onBack={() => {
              setSelectedId(null);
              setSnapshot(null);
              setEvents([]);
              fetchPlenaries(); // Refresh list on back
            }}
          />
        ) : (
          <PlenaryList
            plenaries={plenaries}
            onSelect={(id) => setSelectedId(id)}
          />
        )}
      </main>
    </div>
  );
}

export default App;
