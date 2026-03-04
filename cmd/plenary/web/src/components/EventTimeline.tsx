import { useEffect, useRef, useState } from "react";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import type { PlenaryEvent } from "../types";
import { eventTypeColor, eventTypeIcon, relativeTime } from "../utils";

// Events that have substantive text content worth showing inline
const CONTENT_EVENTS = new Set([
  "speak",
  "proposal.created",
  "consent.given",
  "block.raised",
  "block.withdrawn",
  "stand_aside.given",
  "decision.closed",
]);

function getContentText(evt: PlenaryEvent): string | null {
  const p = evt.payload;
  switch (evt.event_type) {
    case "speak":
      return String(p.text || "");
    case "proposal.created":
      return String(p.text || "");
    case "consent.given":
      return p.reason ? String(p.reason) : null;
    case "block.raised":
      return String(p.text || "");
    case "block.withdrawn":
      return p.reason ? String(p.reason) : null;
    case "stand_aside.given":
      return p.reason ? String(p.reason) : null;
    case "decision.closed":
      return p.resolution ? String(p.resolution) : null;
    default:
      return null;
  }
}

function compactSummary(evt: PlenaryEvent): string {
  const p = evt.payload;
  switch (evt.event_type) {
    case "plenary.created":
      return `created plenary`;
    case "participant.joined":
      return p.role ? `joined as ${p.role}` : "joined";
    case "phase.set":
      return `${p.expected_phase} \u2192 ${p.phase}`;
    default:
      return evt.event_type;
  }
}

export function EventTimeline({ events }: { events: PlenaryEvent[] }) {
  const timelineEndRef = useRef<HTMLDivElement>(null);
  const [expandedEvent, setExpandedEvent] = useState<string | null>(null);
  const [viewMode, setViewMode] = useState<"conversation" | "raw">("conversation");

  useEffect(() => {
    timelineEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [events.length]);

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle className="text-lg">
            Timeline ({events.length})
          </CardTitle>
          <div className="flex gap-1">
            {(["conversation", "raw"] as const).map((mode) => (
              <button
                key={mode}
                onClick={() => setViewMode(mode)}
                className={`px-2 py-0.5 text-xs rounded cursor-pointer transition-colors ${
                  viewMode === mode
                    ? "bg-primary text-primary-foreground"
                    : "bg-muted text-muted-foreground hover:text-foreground"
                }`}
              >
                {mode === "conversation" ? "Conversation" : "Raw"}
              </button>
            ))}
          </div>
        </div>
      </CardHeader>
      <CardContent>
        {viewMode === "conversation" ? (
          <ConversationView events={events} />
        ) : (
          <RawView
            events={events}
            expandedEvent={expandedEvent}
            setExpandedEvent={setExpandedEvent}
          />
        )}
        <div ref={timelineEndRef} />
      </CardContent>
    </Card>
  );
}

function ConversationView({ events }: { events: PlenaryEvent[] }) {
  return (
    <div className="space-y-3">
      {events.map((evt) => {
        const isContent = CONTENT_EVENTS.has(evt.event_type);
        const contentText = isContent ? getContentText(evt) : null;

        if (!isContent) {
          // Compact structural event
          return (
            <div
              key={evt.event_id}
              className="flex items-center gap-2 text-xs text-muted-foreground py-1"
            >
              <span className="font-mono w-4 text-center shrink-0">
                {eventTypeIcon(evt.event_type)}
              </span>
              <span className="font-medium text-foreground">
                {evt.actor.actor_id}
              </span>
              <span>{compactSummary(evt)}</span>
              <span className="font-mono">{relativeTime(evt.ts)}</span>
            </div>
          );
        }

        // Content event — chat bubble style
        return (
          <div key={evt.event_id} className={`border-l-3 ${eventTypeColor(evt.event_type)} pl-3`}>
            <div className="flex items-center gap-2 mb-1">
              <span className="font-medium text-sm">{evt.actor.actor_id}</span>
              <Badge variant="outline" className="text-xs">
                {evt.event_type.replace(".", " ").replace("_", " ")}
              </Badge>
              <span className="text-xs text-muted-foreground font-mono">
                {relativeTime(evt.ts)}
              </span>
            </div>
            {contentText && (
              <p className="text-sm whitespace-pre-wrap leading-relaxed">
                {contentText}
              </p>
            )}
          </div>
        );
      })}
    </div>
  );
}

function RawView({
  events,
  expandedEvent,
  setExpandedEvent,
}: {
  events: PlenaryEvent[];
  expandedEvent: string | null;
  setExpandedEvent: (id: string | null) => void;
}) {
  return (
    <div className="space-y-1">
      {events.map((evt) => (
        <div key={evt.event_id}>
          <div
            className={`flex items-start gap-2 text-sm border-l-3 ${eventTypeColor(
              evt.event_type
            )} pl-3 py-1.5 cursor-pointer hover:bg-muted/30 rounded-r transition-colors`}
            onClick={() =>
              setExpandedEvent(
                expandedEvent === evt.event_id ? null : evt.event_id
              )
            }
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                setExpandedEvent(
                  expandedEvent === evt.event_id ? null : evt.event_id
                );
              }
            }}
            role="button"
            tabIndex={0}
            aria-expanded={expandedEvent === evt.event_id}
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
          </div>
          {expandedEvent === evt.event_id && (
            <div className="ml-8 mb-2 p-3 bg-muted/50 rounded text-xs font-mono whitespace-pre-wrap break-all">
              {JSON.stringify(evt.payload, null, 2)}
            </div>
          )}
        </div>
      ))}
    </div>
  );
}
