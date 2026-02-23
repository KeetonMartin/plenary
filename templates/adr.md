# Template: Architecture Decision Record (ADR)

## Suggested Topic

`What architecture should we use for <capability>, and why?`

## Framing Prompts

- What problem is this architecture decision solving?
- What are the hard constraints (runtime, deployment, compatibility, latency, cost)?
- What decisions are explicitly out of scope?

## Divergence Prompts

- Present 2-3 viable options (not just one).
- What fails or becomes expensive with each option?
- What migration path exists if we choose wrong?

## Proposal Skeleton

`Decision: Adopt <option> for <scope>. Context: ... Alternatives considered: ... Consequences: ... Migration/rollback: ...`

## Resolution Checklist

- Context summarized
- Chosen option and scope
- Alternatives named
- Consequences/tradeoffs stated
- Revisit trigger (what evidence would cause re-evaluation)
