# Plenary

Consensus protocol tooling for humans and agents.

Current implementation:
- Go CLI (`plenary`)
- JSONL event store (`.plenary/events.jsonl`)
- Deterministic reducer for derived snapshots
- Local web viewer (in progress; served from `cmd/plenary/web`)

## Quickstart

Prereqs:
- Go (for CLI)
- Node.js (only needed for the web viewer frontend build)

Build the web frontend first (required because the Go CLI embeds `cmd/plenary/web/dist`):

```bash
cd cmd/plenary/web
npm install
npm run build
cd ../../..
```

Run tests:

```bash
go test ./...
```

Run the CLI directly:

```bash
go run ./cmd/plenary help
```

## Environment

The CLI reads actor identity from environment variables:

```bash
export PLENARY_DB=.plenary/events.jsonl
export PLENARY_ACTOR_ID=codex
export PLENARY_ACTOR_TYPE=agent   # or human
```

## End-to-End Example

Create a plenary:

```bash
plenary create --topic "Choose storage engine" --rule unanimity
```

Join as participants (run with different actor env vars):

```bash
plenary join --plenary <PLENARY_ID>
```

Progress phases:

```bash
plenary phase --plenary <PLENARY_ID> --from framing --to divergence
plenary speak --plenary <PLENARY_ID> --message "SQLite is enough for v0"
plenary phase --plenary <PLENARY_ID> --from divergence --to proposal
```

Propose + consent:

```bash
plenary propose --plenary <PLENARY_ID> --text "Use SQLite for v0"
plenary phase --plenary <PLENARY_ID> --from proposal --to consensus_check
plenary consent --plenary <PLENARY_ID> --proposal <PROPOSAL_ID>
```

Inspect state:

```bash
plenary status --plenary <PLENARY_ID>
```

Close decision:

```bash
plenary close --plenary <PLENARY_ID> --resolution "Use SQLite for v0" --outcome consensus
```

## Export Artifacts

Export a plenary to files:

```bash
plenary export --plenary <PLENARY_ID> --out ./out/<PLENARY_ID>
```

Artifacts written:
- `events.jsonl`
- `snapshot.json`
- `transcript.md`
- `decision_record.json` (only if the plenary has been closed)

`export` JSON output includes `decision_record_present` so callers can branch safely.

## Tail Events

Print events as compact JSON lines:

```bash
plenary tail --plenary <PLENARY_ID>
```

Follow mode:

```bash
plenary tail --plenary <PLENARY_ID> --follow --interval-ms 500
```

## Web Viewer

Start the local viewer:

```bash
plenary web --port 3000
```

This serves:
- UI on `/`
- API on `/api/plenaries`
- Snapshot on `/api/plenaries/<PLENARY_ID>`
- Events on `/api/plenaries/<PLENARY_ID>/events`

## HTTP API Smoke Test

With `plenary serve` running, smoke-test a two-actor HTTP lifecycle:

```bash
make smoke-http
```

You can also point it at a different server with `BASE=http://host:port make smoke-http`.

Full API reference: `API.md`
