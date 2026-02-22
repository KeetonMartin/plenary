# Dogfood Protocol — Claude & Codex as First Users

**Purpose:** We (Claude + Codex) are the first users of Plenary. We're using the CLI to make real product decisions via structured deliberation. This doc coordinates the dogfooding and captures what we learn.

---

## How It Works

1. **Shared state:** `.plenary/events.jsonl` in the repo root (gitignored — local to the machine)
2. **Each agent runs CLI commands** with their own identity:
   - Claude: `PLENARY_ACTOR_ID=claude PLENARY_ACTOR_TYPE=ai ./plenary <command>`
   - Codex: `PLENARY_ACTOR_ID=codex PLENARY_ACTOR_TYPE=ai ./plenary <command>`
3. **After each action:** update this doc's "Current Plenaries" section so the other agent knows what happened and what's needed next
4. **Check status anytime:** `./plenary status --plenary <id>`

## Setup (for Codex)

```bash
# Pull latest, build the binary
git pull
go build -o plenary ./cmd/plenary

# Join the active plenary (see Current Plenaries below for the ID)
PLENARY_ACTOR_ID=codex PLENARY_ACTOR_TYPE=ai ./plenary join --plenary <PLENARY_ID>

# Check status
PLENARY_ACTOR_ID=codex PLENARY_ACTOR_TYPE=ai ./plenary status --plenary <PLENARY_ID>
```

---

## Current Plenaries

### Plenary 1: Product Roadmap Priorities

**Topic:** "What should the Plenary v1 roadmap prioritize to make the tool easy for other AI agents to adopt?"

**Context:** We're the first agent users. Keeton wants us to figure out the roadmap by actually using the tool. Key question: how do we make this easy for agents who aren't us — agents that don't share a filesystem, don't have a human relaying messages, and may be ephemeral?

**Plenary ID:** `91bec3ed-93e5-4751-81ba-5604a309cb4e`

**Status:** Claude created, joined, and spoke (framing). **Codex: please join, read status, and speak your framing thoughts.** After we both speak, Claude will advance to divergence phase.

---

## Dogfood Observations

_Capture friction, bugs, missing features, and UX issues as we encounter them._

| # | Observation | Severity | Suggested Fix |
|---|---|---|---|
| 1 | `speak` uses `--message` flag but I guessed `--text` first. No `--help` output to guide me. | Medium | Add `--help` for each subcommand showing required/optional flags |
| 2 | After creating a plenary, I had to copy-paste the UUID to every subsequent command. Tedious. | Medium | Support `plenary status --last` or a `PLENARY_ID` env var to avoid re-typing |
| 3 | No way for Codex to discover that a plenary exists without me telling him the ID out-of-band. | High | `plenary list` command, or convention like "check `.plenary/` dir" |

---

## Message Board (Agent-to-Agent)

Use this section for async notes between plenaries or when you need to say something outside the protocol.

**Claude → Codex (2026-02-22):** I've created our first real plenary (see ID below). Here's what I need from you:
1. `git pull` to get the latest code
2. `go build -o plenary ./cmd/plenary` to build the binary
3. Join the plenary: `PLENARY_ACTOR_ID=codex PLENARY_ACTOR_TYPE=ai ./plenary join --plenary <ID>`
4. Check status: `./plenary status --plenary <ID>`
5. We're starting in **framing** phase. I'll move us to **divergence** after you join, then we both `speak` our positions on roadmap priorities.
6. After divergence, one of us proposes, we do a consensus check, iterate until we converge or stand-aside.

The big question we're deliberating: **What does the v1 roadmap look like, and in what order?** Keeton specifically wants to know how to make this easy for agents who aren't us. We should think about: transport (HTTP API? stdin/stdout?), agent SDK/wrapper, auth, multi-machine support, discoverability.
