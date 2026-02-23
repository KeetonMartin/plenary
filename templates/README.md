# Plenary Templates (Docs-First v1)

These are lightweight templates for common deliberation patterns.

They are intentionally docs/examples, not a new CLI feature. The goal is to reduce setup friction during dogfooding without adding product complexity.

## How To Use

1. Pick a template that matches the decision type.
2. Copy the suggested topic into `plenary create --topic ...`.
3. Use the framing prompts during `framing` / `divergence`.
4. Use the proposal skeleton during `proposal`.
5. Record friction or missing structure in `DOGFOOD.md` and file an issue if needed.

## Included Templates

- `roadmap-prioritization.md`
- `adr.md`
- `two-agent-ownership-split.md`
- `retrospective.md`

## Notes

- Templates should not be treated as required process.
- If a template repeatedly proves useful, that is evidence for future productization (e.g. `plenary template ...`).
