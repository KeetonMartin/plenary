export const PHASES = ["framing", "divergence", "proposal", "consensus_check", "closed"];

export function phaseIndex(phase: string): number {
  return PHASES.indexOf(phase);
}

export function phaseBadgeVariant(
  phase: string
): "default" | "secondary" | "destructive" | "outline" {
  if (phase === "closed") return "default";
  if (phase === "consensus_check") return "destructive";
  return "secondary";
}

export function stanceBadgeVariant(
  stance: string
): "default" | "secondary" | "destructive" | "outline" {
  if (stance === "consent") return "default";
  if (stance === "block") return "destructive";
  if (stance === "stand_aside") return "outline";
  return "secondary";
}

export function relativeTime(ts: string): string {
  const diff = Date.now() - new Date(ts).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

export function eventTypeColor(eventType: string): string {
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

export function eventTypeIcon(eventType: string): string {
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

export function payloadSummary(evt: { event_type: string; payload: Record<string, unknown> }): string {
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

export async function copyToClipboard(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    return false;
  }
}
