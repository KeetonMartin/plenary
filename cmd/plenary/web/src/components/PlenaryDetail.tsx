import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import type { Snapshot, PlenaryEvent } from "../types";
import { stanceBadgeVariant } from "../utils";
import { PhaseStepper } from "./PhaseStepper";
import { EventTimeline } from "./EventTimeline";
import { CopyButton } from "./CopyButton";
import { ProposalCard } from "./ProposalCard";

export function PlenaryDetail({
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
  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <button
          onClick={onBack}
          className="text-sm text-muted-foreground hover:text-foreground cursor-pointer"
        >
          &larr; Back to list
        </button>
        <div className="flex items-center gap-3">
          <CopyButton text={snapshot.plenary_id} label="plenary ID" />
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

      {/* Proposals */}
      {(snapshot.proposals && snapshot.proposals.length > 0
        ? snapshot.proposals
        : snapshot.active_proposal
        ? [snapshot.active_proposal]
        : []
      ).map((proposal) => (
        <ProposalCard
          key={proposal.proposal_id}
          proposal={proposal}
          isActive={proposal.proposal_id === snapshot.active_proposal?.proposal_id}
          participants={
            proposal.proposal_id === snapshot.active_proposal?.proposal_id
              ? snapshot.participants
              : undefined
          }
        />
      ))}

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
      <EventTimeline events={events} />
    </div>
  );
}
