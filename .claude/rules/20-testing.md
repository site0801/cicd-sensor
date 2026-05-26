---
paths:
  - "**/*_test.go"
  - "**/*.go"
---

# Testing Rules

## Baseline

- Table-driven tests with `t.Run` per case. The table documents what the spec guarantees.
- `t.Helper()` in helpers. `t.Parallel()` on independent cases.
- Cover happy path, error returns, and boundary / edge cases. Every `return err` is reached by at least one case.
- Case names answer "why this case is needed", not "what the input is".
- Compare errors with `errors.Is` / `errors.As`. Never string-compare.
- Match the existing test file's package (`package foo` vs `package foo_test`). Don't depend on private internals or re-implement production logic in the test.
- Integration tests and tests with external dependencies are gated by a build tag or env var.
- Fixtures live in `testdata/`. For large or structured output, use golden files with an `-update` flag.
- Run with `-race` for goroutine code.

## Required output when writing or reviewing tests

Whenever you add tests, modify tests, or review a tested function, you must include both of the tables below in your reply. Do not summarise them or omit them — render the full markdown tables so the user can read the case list and the perspective coverage directly.

Trigger conditions:

- Writing new `_test.go` content.
- Modifying existing test cases or test helpers.
- Touching a function that already has tests (re-check that the existing tables still hold).
- Reviewing or auditing tests on request.

### 1. Test case list

One table per target function.

| # | Case name | Input | Expected | Kind | Notes |
| --- | --- | --- | --- | --- | --- |
| 1 | happy: basic input | `x=1, y=2` | `3, nil` | unit | |
| 2 | error: nil pointer | `x=nil` | `_, ErrNilInput` | unit | |

- One row per table-driven entry. `t.Run` subtests: include the subtest name in the case name.
- Kinds: `unit` / `integration` / `bench` / `example` / `fuzz`.
- Functions with no test get a row with an empty body and a note.

### 2. Coverage perspective check

| Perspective | Status | Detail |
| --- | --- | --- |
| Happy path | OK / partial / missing | … |
| Error paths | OK / partial / missing | each `return err` reached |
| Boundary values | OK / partial / missing | empty / single / large; 0 / negative / max |
| Zero / nil | OK / partial / missing | the Go zero value of every argument |
| Concurrency | OK / partial / missing / N/A | `-race` test for goroutine code |
| Context | OK / partial / missing / N/A | cancel / deadline / timeout |
| Interface boundary | OK / partial / missing / N/A | substitution for external dependencies |
| Edge cases | OK / partial / missing | known oddities specific to the function |

Mark `missing` perspectives as proposed cases before declaring the change ready.

## Modern APIs (reference)

- `t.Context()` / `b.Context()` (1.24) — test-scoped context; prefer over `context.Background()`.
- `testing.B.Loop()` (1.24) — benchmark loop driver; prefer over `for i := 0; i < b.N; i++`.
- `testing/synctest.Test(t, fn)` (1.25, stable) — virtualized time for timing-sensitive concurrent code; prefer over real `time.Sleep`.
