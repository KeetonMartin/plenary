import type { Participant } from "../types";

export function StanceSummary({ participants }: { participants: Participant[] }) {
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
