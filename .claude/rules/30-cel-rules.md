---
paths:
  - "rules/**"
  - "internal/rule/**"
  - "**/*.cel"
---

# CEL / Rule Authoring Rules

## Schema

- `rule_sets:` and `rule_modifiers:` cannot share a YAML document. Keep RuleSet files and modifier files separate.
- A bundle file is required when distributing rules through the manager.
- In GitHub-hosted runner standalone mode, repository-local `.cicd-sensor/rules/` is allowed. With the manager, repository-local rules are not used — rules are fetched from the manager.

## CEL surface

- The CEL surface is intentionally narrow: no regex, no index access, no arithmetic. Do not widen it casually.
- Widening the surface is a design decision and goes through `docs/` first.

## Source of truth

- Event-type fields are defined in `docs/user-guide/rule-event-types.md`. Do not introduce a field name that isn't listed there.
- CEL condition examples and idioms live in `docs/user-guide/rule-cel-conditions.md`.
- For correlation rules, follow `docs/user-guide/rule-correlation.md` (`rule.<rule_id>.total_count` and friends).
- For modifier semantics (action overrides, exceptions, target excludes, disable flags), follow `docs/user-guide/rule-modifiers.md`.

## Rule descriptions

- Write the description as multi-line `What: / Why: / Notes:` blocks using the YAML `|` block scalar. The agent UI preserves the newlines.
