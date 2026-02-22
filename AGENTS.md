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

Repo root is the repository root (this directory).

### Prereqs

- Go
- Node.js (for the web frontend build)

### Web embed build tag (`webembed`)

`go test ./...` works without building the frontend because the web UI embed is behind a build tag.

To build a binary with the embedded web viewer (`plenary web` served from `cmd/plenary/web/dist`), build frontend assets first and compile with `-tags webembed`:

```bash
cd cmd/plenary/web
npm install
npm run build
cd ../..
go build -tags webembed -o plenary ./cmd/plenary
```

### Common commands

```bash
cd .
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
export PLENARY_DB=.plenary/events.jsonl
export PLENARY_ACTOR_ID=codex     # or claude / keeton
export PLENARY_ACTOR_TYPE=agent   # 'ai' is normalized to 'agent' for compatibility
```

### Web viewer

```bash
cd .
./plenary web --port 3001
```

If the UI loads but row clicks appear to do nothing, verify the backend process is still running and pointed at the correct DB (`PLENARY_DB`). The UI depends on detail fetches to `/api/plenaries/<id>` and `/api/plenaries/<id>/events`.

### MCP server (agent-native integration)

Plenary exposes an MCP tool server over stdio:

```bash
./plenary mcp-serve
```

Repo-local MCP config for Claude Code is in `.mcp.json` (uses `./plenary mcp-serve` and repo-local `.plenary/events.jsonl`).

Checked-in MCP config variants:
- `.mcp.claude.json` (actor id `claude`)
- `.mcp.codex.json` (actor id `codex`)

Use the same `PLENARY_DB` across agents, but different `PLENARY_ACTOR_ID` values. If both agents share one config with the same actor id, the event log becomes ambiguous.

For MCP dogfooding:
- use MCP tools for plenary actions (`plenary_create`, `plenary_join`, `plenary_speak`, etc.) instead of shelling out
- keep using `DOGFOOD.md` for friction notes and coordination outside protocol content
- file a GitHub issue for concrete bugs/regressions you discover

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
  - `plenary` (local binary)
  - `cmd/plenary/web/dist/*` (frontend build artifacts)
  - `.plenary/events.jsonl` (dogfood state; commit only when intentionally syncing protocol actions)

## Ownership And Status (source of truth)

Do not treat this file as the source of truth for product area ownership, task assignment, or roadmap status.

Ownership and status are intentionally volatile and should live in:
- `WORKPLAN.md` (implementation coordination and active ownership)
- GitHub issues/projects/milestones (backlog and tracking)
- Plenary decisions and dogfood notes (`DOGFOOD.md` + event log) for rationale

`AGENTS.md` should stay stable and focus on durable operating guidance (build/test workflow, coordination rules, and dogfooding practices).

## When To Open A New Plenary

Open a plenary when the decision is cross-cutting or likely to create duplicate work, for example:
- architecture direction / transport model
- issue prioritization / roadmap ordering
- ownership splits across agents
- protocol changes that affect dogfooding behavior

Do not open a plenary for routine implementation details already covered by `WORKPLAN.md`.
