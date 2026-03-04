import { useState } from "react";
import {
  Card,
  CardContent,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import type { PlenarySummary } from "../types";
import { phaseBadgeVariant, relativeTime } from "../utils";

export function PlenaryList({
  plenaries,
  onSelect,
}: {
  plenaries: PlenarySummary[];
  onSelect: (id: string) => void;
}) {
  const [search, setSearch] = useState("");
  const [filter, setFilter] = useState<"all" | "open" | "closed">("all");

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

  const filtered = plenaries.filter((p) => {
    if (filter === "open" && p.closed) return false;
    if (filter === "closed" && !p.closed) return false;
    if (search && !p.topic.toLowerCase().includes(search.toLowerCase())) return false;
    return true;
  });

  // Sort: open first, then by last event
  const sorted = [...filtered].sort((a, b) => {
    if (a.closed !== b.closed) return a.closed ? 1 : -1;
    const ta = a.last_event_at || "";
    const tb = b.last_event_at || "";
    return tb.localeCompare(ta);
  });

  const openCount = plenaries.filter((p) => !p.closed).length;
  const closedCount = plenaries.filter((p) => p.closed).length;

  return (
    <div className="space-y-3">
      {/* Search + Filter bar */}
      <div className="flex items-center gap-2">
        <input
          type="text"
          placeholder="Search topics..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="flex-1 px-3 py-1.5 text-sm rounded-md border bg-background placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring/50"
        />
        <div className="flex gap-1">
          {(["all", "open", "closed"] as const).map((f) => (
            <button
              key={f}
              onClick={() => setFilter(f)}
              className={`px-2.5 py-1 text-xs rounded-md transition-colors cursor-pointer ${
                filter === f
                  ? "bg-primary text-primary-foreground"
                  : "bg-muted text-muted-foreground hover:text-foreground"
              }`}
            >
              {f === "all" ? `All (${plenaries.length})` : f === "open" ? `Open (${openCount})` : `Closed (${closedCount})`}
            </button>
          ))}
        </div>
      </div>

      {sorted.length === 0 ? (
        <Card>
          <CardContent className="p-6 text-center text-muted-foreground">
            No plenaries match your search.
          </CardContent>
        </Card>
      ) : (
        sorted.map((p) => (
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
        ))
      )}
    </div>
  );
}
