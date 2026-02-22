import { useEffect, useState } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

interface PlenarySummary {
  plenary_id: string;
  topic: string;
  phase: string;
  decision_rule: string;
  closed: boolean;
  event_count: number;
}

interface Participant {
  actor_id: string;
  actor_type: string;
  role?: string;
  stance: string;
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

  return (
    <Card>
      <CardHeader>
        <CardTitle>Plenaries</CardTitle>
        <CardDescription>All deliberations in this store</CardDescription>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Topic</TableHead>
              <TableHead>Phase</TableHead>
              <TableHead>Rule</TableHead>
              <TableHead>Events</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {plenaries.map((p) => (
              <TableRow
                key={p.plenary_id}
                className="cursor-pointer hover:bg-muted/50"
                onClick={() => onSelect(p.plenary_id)}
              >
                <TableCell className="font-medium">{p.topic}</TableCell>
                <TableCell>
                  <Badge variant={phaseBadgeVariant(p.phase)}>{p.phase}</Badge>
                </TableCell>
                <TableCell>
                  <Badge variant="outline">{p.decision_rule}</Badge>
                </TableCell>
                <TableCell>{p.event_count}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}

function PlenaryDetail({
  snapshot,
  events,
  onBack,
}: {
  snapshot: Snapshot;
  events: PlenaryEvent[];
  onBack: () => void;
}) {
  return (
    <div className="space-y-4">
      <button
        onClick={onBack}
        className="text-sm text-muted-foreground hover:text-foreground"
      >
        &larr; Back to list
      </button>

      {/* Header */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>{snapshot.topic}</CardTitle>
            <div className="flex gap-2">
              <Badge variant={phaseBadgeVariant(snapshot.phase)}>
                {snapshot.phase}
              </Badge>
              <Badge variant="outline">{snapshot.decision_rule}</Badge>
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
          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <span className="text-muted-foreground">Events:</span>{" "}
              {snapshot.event_count}
            </div>
            <div>
              <span className="text-muted-foreground">Ready to close:</span>{" "}
              {snapshot.ready_to_close ? "Yes" : "No"}
            </div>
          </div>
          {snapshot.next_required_actions &&
            snapshot.next_required_actions.length > 0 && (
              <div className="mt-3">
                <span className="text-sm text-muted-foreground">
                  Next actions:
                </span>
                <ul className="list-disc list-inside text-sm mt-1">
                  {snapshot.next_required_actions.map((a, i) => (
                    <li key={i}>{a}</li>
                  ))}
                </ul>
              </div>
            )}
        </CardContent>
      </Card>

      {/* Decision Record */}
      {snapshot.decision_record && (
        <Card>
          <CardHeader>
            <CardTitle className="text-lg">Decision Record</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="font-medium">{snapshot.decision_record.resolution}</p>
            {snapshot.decision_record.rationale_bullets && (
              <ul className="list-disc list-inside text-sm mt-2 text-muted-foreground">
                {snapshot.decision_record.rationale_bullets.map((b, i) => (
                  <li key={i}>{b}</li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>
      )}

      {/* Active Proposal */}
      {snapshot.active_proposal && (
        <Card>
          <CardHeader>
            <CardTitle className="text-lg">Active Proposal</CardTitle>
          </CardHeader>
          <CardContent>
            <p>{snapshot.active_proposal.text}</p>
            {snapshot.active_proposal.acceptance_criteria && (
              <p className="text-sm text-muted-foreground mt-1">
                Criteria: {snapshot.active_proposal.acceptance_criteria}
              </p>
            )}
          </CardContent>
        </Card>
      )}

      {/* Participants */}
      <Card>
        <CardHeader>
          <CardTitle className="text-lg">Participants</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Actor</TableHead>
                <TableHead>Type</TableHead>
                <TableHead>Stance</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {snapshot.participants.map((p) => (
                <TableRow key={p.actor_id}>
                  <TableCell className="font-medium">{p.actor_id}</TableCell>
                  <TableCell>
                    <Badge variant="outline">{p.actor_type}</Badge>
                  </TableCell>
                  <TableCell>
                    <Badge variant={stanceBadgeVariant(p.stance)}>
                      {p.stance}
                    </Badge>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {/* Unresolved Blocks */}
      {snapshot.unresolved_blocks.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="text-lg">Unresolved Blocks</CardTitle>
          </CardHeader>
          <CardContent>
            {snapshot.unresolved_blocks.map((b, i) => (
              <div key={i} className="mb-2 p-2 bg-destructive/10 rounded">
                <span className="font-medium">{b.actor_id}:</span> {b.text}
                {b.principle && (
                  <span className="text-sm text-muted-foreground ml-2">
                    (principle: {b.principle})
                  </span>
                )}
              </div>
            ))}
          </CardContent>
        </Card>
      )}

      {/* Event Timeline */}
      <Card>
        <CardHeader>
          <CardTitle className="text-lg">Event Timeline</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-2">
            {events.map((evt) => (
              <div
                key={evt.event_id}
                className="flex items-start gap-3 text-sm border-l-2 border-muted pl-3 py-1"
              >
                <span className="text-muted-foreground font-mono text-xs whitespace-nowrap">
                  {new Date(evt.ts).toLocaleTimeString()}
                </span>
                <Badge variant="outline" className="text-xs shrink-0">
                  {evt.event_type}
                </Badge>
                <span className="font-medium shrink-0">
                  {evt.actor.actor_id}
                </span>
                <span className="text-muted-foreground truncate">
                  {payloadSummary(evt)}
                </span>
              </div>
            ))}
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
      return `${p.expected_phase} -> ${p.phase}`;
    case "proposal.created":
      return `"${p.text}"`;
    case "consent.given":
      return p.reason ? `reason: "${p.reason}"` : "consented";
    case "stand_aside.given":
      return `"${p.reason}"`;
    case "block.raised":
      return `"${p.text}"`;
    case "block.withdrawn":
      return `"${p.reason}"`;
    case "speak":
      return `"${p.text}"`;
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

  useEffect(() => {
    fetch("/api/plenaries")
      .then((r) => r.json())
      .then((data) => {
        setPlenaries(data || []);
        setLoading(false);
      })
      .catch(() => setLoading(false));
  }, []);

  useEffect(() => {
    if (!selectedId) return;
    Promise.all([
      fetch(`/api/plenaries/${selectedId}`).then((r) => r.json()),
      fetch(`/api/plenaries/${selectedId}/events`).then((r) => r.json()),
    ]).then(([snap, evts]) => {
      setSnapshot(snap);
      setEvents(evts || []);
    });
  }, [selectedId]);

  return (
    <div className="min-h-screen bg-background">
      <header className="border-b">
        <div className="max-w-4xl mx-auto px-4 py-3 flex items-center gap-3">
          <h1 className="text-lg font-semibold">Plenary</h1>
          <span className="text-sm text-muted-foreground">
            Consensus protocol for agents
          </span>
        </div>
      </header>
      <main className="max-w-4xl mx-auto px-4 py-6">
        {loading ? (
          <p className="text-muted-foreground">Loading...</p>
        ) : selectedId && snapshot ? (
          <PlenaryDetail
            snapshot={snapshot}
            events={events}
            onBack={() => {
              setSelectedId(null);
              setSnapshot(null);
              setEvents([]);
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
