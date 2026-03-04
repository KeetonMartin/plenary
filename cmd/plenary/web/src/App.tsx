import { useCallback, useEffect, useRef, useState } from "react";
import type { PlenarySummary, Snapshot, PlenaryEvent } from "./types";
import { PlenaryList } from "./components/PlenaryList";
import { PlenaryDetail } from "./components/PlenaryDetail";

function App() {
  const [plenaries, setPlenaries] = useState<PlenarySummary[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null);
  const [events, setEvents] = useState<PlenaryEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [sseConnected, setSSEConnected] = useState(false);
  const sseRef = useRef<EventSource | null>(null);
  const globalSseRef = useRef<EventSource | null>(null);

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

  // Global SSE stream — triggers list refresh on any event across all plenaries
  useEffect(() => {
    fetchPlenaries();

    // Fallback polling every 10s in case SSE drops
    const interval = setInterval(fetchPlenaries, 10000);

    const globalEs = new EventSource("/api/stream");
    globalSseRef.current = globalEs;

    let refreshTimer: ReturnType<typeof setTimeout> | null = null;
    globalEs.onmessage = () => {
      // Debounce list refreshes — multiple events may arrive in a burst
      if (refreshTimer) clearTimeout(refreshTimer);
      refreshTimer = setTimeout(fetchPlenaries, 500);
    };

    return () => {
      clearInterval(interval);
      globalEs.close();
      globalSseRef.current = null;
      if (refreshTimer) clearTimeout(refreshTimer);
    };
  }, [fetchPlenaries]);

  // Fetch detail + connect SSE when plenary is selected
  useEffect(() => {
    if (!selectedId) {
      if (sseRef.current) {
        sseRef.current.close();
        sseRef.current = null;
        setSSEConnected(false);
      }
      return;
    }

    let initialEventCount = 0;
    Promise.all([
      fetch(`/api/plenaries/${selectedId}`).then((r) => r.json()),
      fetch(`/api/plenaries/${selectedId}/events`).then((r) => r.json()),
    ]).then(([snap, evts]) => {
      setSnapshot(snap);
      const eventList = evts || [];
      setEvents(eventList);
      initialEventCount = eventList.length;
    });

    // Connect SSE for live updates
    const es = new EventSource(`/api/plenaries/${selectedId}/stream`);
    sseRef.current = es;

    const seenIds = new Set<string>();
    let sseEventCount = 0;
    let snapshotRefreshTimer: ReturnType<typeof setTimeout> | null = null;

    es.onopen = () => setSSEConnected(true);
    es.onerror = () => setSSEConnected(false);
    es.onmessage = (msg) => {
      try {
        const evt: PlenaryEvent = JSON.parse(msg.data);
        if (seenIds.has(evt.event_id)) return;
        seenIds.add(evt.event_id);
        sseEventCount++;

        setEvents((prev) => {
          if (prev.some((e) => e.event_id === evt.event_id)) return prev;
          return [...prev, evt];
        });

        // Only refetch snapshot for genuinely new events (not SSE replay).
        if (sseEventCount > initialEventCount) {
          if (snapshotRefreshTimer) clearTimeout(snapshotRefreshTimer);
          snapshotRefreshTimer = setTimeout(() => {
            fetch(`/api/plenaries/${selectedId}`)
              .then((r) => r.json())
              .then(setSnapshot);
          }, 300);
        }
      } catch {
        // ignore parse errors
      }
    };

    return () => {
      es.close();
      sseRef.current = null;
      setSSEConnected(false);
      if (snapshotRefreshTimer) clearTimeout(snapshotRefreshTimer);
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
              fetchPlenaries();
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
