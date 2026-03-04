import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import type { Proposal, Participant } from "../types";
import { StanceSummary } from "./StanceSummary";
import { CopyButton } from "./CopyButton";

export function ProposalCard({
  proposal,
  isActive,
  participants,
}: {
  proposal: Proposal;
  isActive: boolean;
  participants?: Participant[];
}) {
  return (
    <Card
      className={
        isActive
          ? "border-yellow-200 bg-yellow-50/30"
          : "border-muted bg-muted/10 opacity-75"
      }
    >
      <CardHeader>
        <div className="flex items-center gap-2">
          <CardTitle className="text-lg">
            {isActive ? "Active Proposal" : "Proposal"}
          </CardTitle>
          {isActive && (
            <span className="text-xs bg-yellow-200 text-yellow-800 px-1.5 py-0.5 rounded font-medium">
              active
            </span>
          )}
        </div>
        <CardDescription>
          <CopyButton text={proposal.proposal_id} label="proposal ID" />
        </CardDescription>
      </CardHeader>
      <CardContent>
        <p className="whitespace-pre-wrap">{proposal.text}</p>
        {proposal.acceptance_criteria && (
          <p className="text-sm text-muted-foreground mt-2">
            <span className="font-medium">Criteria:</span>{" "}
            {proposal.acceptance_criteria}
          </p>
        )}
        {isActive && participants && participants.length > 0 && (
          <div className="mt-3">
            <StanceSummary participants={participants} />
          </div>
        )}
      </CardContent>
    </Card>
  );
}
