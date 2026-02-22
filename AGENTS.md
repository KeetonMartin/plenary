# AGENTS.md

This file is the canonical agent-operating guide for this repository.

If you are an AI coding agent (Codex, Claude Code, etc.), read this first, then use the repo docs referenced here.

## Purpose

Plenary is both:
- the product (consensus protocol tooling for humans/agents)
- the coordination substrate we actively dogfood while building it

Agents should optimize for two outcomes at once:
- shipping code without duplicating work
- using Plenary itself to make product/architecture/roadmap decisions

## Source Of Truth Docs

Use these docs for the right kind of coordination:

- `WORKPLAN.md`
  - Canonical implementation coordination doc
  - Claim tasks here before coding
  - Record handoffs, ownership, and interface changes here
- `DOGFOOD.md`
  - Dogfood runbook + learnings + async message board
  - Record friction/bugs and plenary coordination notes here
- `README.md`
  - User-facing quickstart and CLI usage

Rule of thumb:
- implementation ownership/conflicts -> `WORKPLAN.md`
- protocol use / dogfood observations / “next turn” notes -> `DOGFOOD.md`
- end-user docs -> `README.md`

## Build / Test Basics

Repo root is `/Users/keetonmartin/code/plenary`.

### Prereqs

- Go
- Node.js (for the web frontend build)

### Important embed prerequisite

`/Users/keetonmartin/code/plenary/cmd/plenary/web.go` embeds `/Users/keetonmartin/code/plenary/cmd/plenary/web/dist`, so a fresh checkout may need the frontend build before `go test ./...` works.

Build frontend assets:

```bash
cd /Users/keetonmartin/code/plenary/cmd/plenary/web
npm install
npm run build
```

### Common commands

```bash
cd /Users/keetonmartin/code/plenary
make test      # preferred if available
# or
go test ./...

make build     # builds frontend + CLI binary (preferred)
# or
go build -o plenary ./cmd/plenary
```

## Local Runtime Conventions

### CLI actor identity

```bash
export PLENARY_DB=/Users/keetonmartin/code/plenary/.plenary/events.jsonl
export PLENARY_ACTOR_ID=codex     # or claude / keeton
export PLENARY_ACTOR_TYPE=agent   # 'ai' is normalized to 'agent' for compatibility
```

### Web viewer

```bash
cd /Users/keetonmartin/code/plenary
./plenary web --port 3001
```

If the UI loads but row clicks appear to do nothing, verify the backend process is still running and pointed at the correct DB (`PLENARY_DB`). The UI depends on detail fetches to `/api/plenaries/<id>` and `/api/plenaries/<id>/events`.

## Multi-Agent Coordination Rules

### 1) Claim before coding

Before starting work, update `WORKPLAN.md`:
- set task owner
- mark status `Starting` / `In Progress`
- note affected files if relevant

Do not edit files actively owned by another agent without a written handoff note in `WORKPLAN.md`.

### 2) Prefer non-overlapping slices

When another agent is active on a feature, pick a parallelizable slice:
- tests/review coverage
- docs
- integration verification
- separate subcommands/files

Avoid duplicate implementations in the same file set.

### 3) Non-blocking collaboration (important)

Do **not** sit in a sleep/poll loop waiting on the other agent.

Preferred pattern:
- take your turn / make your change
- commit + push (if sharing via git)
- update `WORKPLAN.md` and/or `DOGFOOD.md`
- move to another productive task
- check for responses later

Blocking wait loops caused avoidable dead time during dogfooding.

### 4) Use the right channel for the message

- Deliberation content (product vision, architecture tradeoffs, roadmap ordering): use Plenary events (`speak`, `propose`, `consent`, etc.)
- Async coordination and “heads up”: `DOGFOOD.md` message board
- Implementation file ownership and handoff: `WORKPLAN.md`

## Dogfooding Plenary (Protocol Best Practices)

Use Plenary for decisions that matter (roadmap, architecture, ownership splits, issue ordering).

### Start a plenary

```bash
./plenary create --topic "<decision topic>" --rule unanimity
./plenary join --plenary <PLENARY_ID>
./plenary status --plenary <PLENARY_ID>
```

### Suggested phase discipline

- `framing`
- `divergence`
- `proposal`
- `consensus_check`
- `close`

### Turn discipline (when using git as the temporary sync layer)

- `git pull --ff-only`
- check status (`./plenary status --plenary ...`)
- take one protocol action (`speak`, `propose`, `consent`, `phase`, etc.)
- verify once if needed
- commit/push `.plenary/events.jsonl` (and any relevant doc notes)
- do other work

### Polling guidance (if you must check)

If you need to check for activity, do a single check, not an indefinite wait loop:
- `git pull --ff-only`
- compare last `event_id` or check `plenary status`

Avoid count-threshold assumptions like `events > N`; they were brittle in practice.

## Git Hygiene

- Run `git status --short` before and after a work chunk
- Commit only your intended files
- Do not sweep in local runtime state unless it is part of the dogfood sync turn
- Be careful with these common local-noise files:
  - `/Users/keetonmartin/code/plenary/plenary` (local binary)
  - `/Users/keetonmartin/code/plenary/cmd/plenary/web/dist/*` (frontend build artifacts)
  - `/Users/keetonmartin/code/plenary/.plenary/events.jsonl` (dogfood state; commit only when intentionally syncing protocol actions)

## Current Roadmap Ownership (summary)

See `WORKPLAN.md` for exact status. As of current dogfood decisions:
- Claude: HTTP API sidecar + SSE (`plenary serve`)
- Codex: discovery/ergonomics and later MCP server (after API contract stabilizes)

## When To Open A New Plenary

Open a plenary when the decision is cross-cutting or likely to create duplicate work, for example:
- architecture direction / transport model
- issue prioritization / roadmap ordering
- ownership splits across agents
- protocol changes that affect dogfooding behavior

Do not open a plenary for routine implementation details already covered by `WORKPLAN.md`.
