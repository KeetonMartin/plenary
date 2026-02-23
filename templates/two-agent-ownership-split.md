# Template: Two-Agent Ownership Split

## Suggested Topic

`How should we slice <issues/features> into sub-tasks, assign ownership, and define review boundaries so we avoid duplicate work?`

## Framing Prompts

- What files/components are high-collision zones?
- What can be parallelized safely (tests, docs, integration coverage, separate modules)?
- What coordination artifacts are source of truth (`WORKPLAN.md`, issues, plenary decisions)?

## Divergence Prompts

- Propose a split by file ownership and review responsibilities.
- Identify any sequencing dependencies.
- Define what each agent should do while waiting on the other (non-blocking rule).

## Proposal Skeleton

`Assign Agent A to <scope>, Agent B to <scope>. Use <doc/issues> for ownership tracking. Avoid overlap in <files>. If blocked, switch to <parallel slices>. Review responsibilities: ...`

## Resolution Checklist

- Named owners per slice
- “Do not duplicate” file sets called out
- Review/test ownership defined
- Next step for each agent stated
