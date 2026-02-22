# Decisions for Keeton

We (Claude + Codex) need your input on these questions to unblock building. Our leans are included — just fill in your answers. One word or one sentence per question is plenty.

---

## 1. Beachhead — who is v1 for?

- **A) Solo dev using multiple agents for coding/architecture decisions**
- **B) Small product/eng team (humans + agents together)**
- **C) GTM/sales workflows (BDR messaging, objection handling)**
- **D) Other: __________

> Both agents lean A — strongest artifact expectations (ADRs, PRs), natural CLI-first audience.

**Your pick:** A! we can even dogfood once we have something that works. but also i want the v0 to work for people who work on teams who share a codebase. still independent devs interacting with their agents (though this is mostly about agents interacting with each other), though someday we might want multi-dev "multiplayer" as part of the plenary experience, it's mostly something for multi-agent interaction.

**v1 success metric (one number, if you have one):**

---

## 2. Default consensus rule

- **A) Strict unanimity** — any block prevents close
- **B) No active blocks + quorum consents** — pragmatic default
- **C) Configurable per plenary** (pick A or B as the default)

> Claude leans C-with-A-default (truest to Quaker roots). Codex leans C-with-B-default (pragmatic). We agree: escalation to "owner_decision" should always exist and be labeled distinctly from "consensus".

**Your pick:** let's go with C for sure. Default can be A

**Should escalation always be available? (yes/no):** sure, yes

**Outcome labels you like (e.g. `consensus`, `owner_decision`, `abandoned`):** 

---

## 3. Who can convene a plenary?

- **A) Only humans**
- **B) Only agents**
- **C) Both, no guardrails**
- **D) Both, with guardrails** (local = anyone; hosted = human approval or policy)

> Both agents lean D. Codex also suggests distinguishing "advisory" vs "binding" plenaries.

**Your pick:** D sounds good

**Any constraints on agent-initiated plenaries?** Agent initiated plenaries I think are the primary use case?

---

## 4. Agent roles (devil's advocate, security reviewer, etc.)

- **A) Optional metadata, default to peers** — support `--role` but don't require it
- **B) Required role assignment to join**

> Both agents lean A. Only "facilitator/clerk" would have special authority (phase transitions).

**Your pick:** I think A here. Not sure we need roles for v0 unless you guys disagree?

**Any must-have built-in roles?** prolly not???

---

## 5. Scope — decisions only, or also artifacts?

- **A) Decisions only** — plenary produces a decision record, not the artifact itself
- **B) Decisions + artifact templates** (e.g. messaging brief) in v1
- **C) Decisions for v1, artifacts later**

> Both agents lean C. Decision records are the core; artifact production is a different problem to layer on later.

**Your pick:** I think we want B actually. Why not have first class artifacts from the start? It will make things easier to debug as well, as we built :)

**Must-have outputs besides the decision record?** I'm imagining transcripts/logs of sorts.

---

## 6. Open source boundary

| OSS (free, local) | Paid (hosted) |
|---|---|
| CLI + core protocol | Multi-tenant hosting |
| Local SQLite storage | SSO + org controls |
| Deterministic reducer | Managed inference |
| Basic local web viewer (read-only) | Cross-plenary search |
| Client libraries | Connectors (Jira/GitHub/Slack) |
| Event schema + golden tests | Retention policies + compliance |

> Both agents agree on this split. Key: basic local web viewer is OSS for adoption.

**Approve / modify / reject:** Approve! looks great.

**Non-negotiable OSS components (if any):** I think we want shadcn for our component library UI/UX for the web app. 

**Non-negotiable paid components (if any):**

---

## 7. Hosted inference model

- **A) Coordination only** — users bring their own agents/keys
- **B) Managed inference** — you spin up agents, users pay
- **C) Both** — BYOA for power users, managed for simpler use cases

> Both agents: keep OSS core vendor-neutral. Hosted can layer on managed inference. This is a business model decision.

**Your pick:** Yeah I think bring your own agents for starters... and definitely we want vendor-neutrality.

**Any constraints (providers, privacy, logging)?**

---

## 8. Trust/integrity in v1

- **A) Minimal** — append-only events, sequential IDs, no hash chain (local trust is fine)
- **B) Tamper-evident** — rolling hash chain in v1, no full PKI
- **C) Full signatures** — not recommended for v1

> Claude leans A (don't over-engineer for local). Codex proposed compromise: A for v1, but keep schema extensible for signatures later. Don't claim "tamper-proof" — at most "append-only log for auditability."

**Your pick:** A sounds good for starters but let's definitely be extensible for signatures later, that sounds like a nice feature that could help with tamper detection even if it's pretty weak, at least it's something?

**What do you want to claim about integrity in docs/marketing?**
up to you guys on that^

---

## 9. Facilitation / who controls phase transitions?

- **A) Anyone can change phases** (simplest)
- **B) Facilitator/clerk only** (cleanest process)
- **C) Configurable per plenary**

> Not yet debated in depth. Codex leans B or C.

**Your pick:** C. I think this is similar to the configurability of number 2 "Default consensus rule", where each plenary can be different?

**Who can close a plenary by default?** I think this is answered in number 2 as well? probably this depends on the extent to which we'}e supporting rolles from the jump. 

---

## 10. Implementation language (only if you care)

> Both agents aligned on **Go** (single binary, no runtime deps, pure-Go SQLite, easy cross-compile). TypeScript as second choice.

**Go is fine / other preference:** Honestly guys I've never worked in Go, and in the unlikely event that I need to read the code, I prefer TS. Can you guys convince me to use Go instead? TS feels like the way to build most projects these days... Lmk tho if y'all both prefer Go we can do that.

---

*Fill this in and we'll start building. Agents: once Keeton answers, copy decisions back to BRAINSTORM.md and begin the event schema.*
