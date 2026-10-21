# mgmt

A read-only CLI that joins a line manager's weekly markdown notes with
Kubernetes-style YAML resources (people, competency frameworks, review cycles).
It extracts evidence, actions and cycle inputs from the notes, validates the
resources (CUE), derives statuses and findings (Rego, via the OPA Go SDK) and
prints reports (Go templates): 1:1 prep sheets, review evidence packs, team
status. It never modifies your notes.

## Quick start

```sh
mkdir my-team && cd my-team
mgmt init          # writes example resources/, notes/, policy/ and reports/ (never overwrites)
mgmt lint          # 0 errors on the scaffold
mgmt prep alex
```

Build static binaries with `make build` (output in `dist/`); `make test` runs the tests.

## Commands

All commands run from the workspace root and print to stdout (`--out FILE` writes a file; `--now YYYY-MM-DD` fixes today's date).

| Command | Purpose |
| --- | --- |
| `mgmt init` | scaffold a workspace with example resources, notes and policies; refuses to overwrite |
| `mgmt lint [--strict] [--format text\|json]` | print errors and warnings; exit 2 on errors, 1 on warnings with `--strict`, else 0 |
| `mgmt prep <person> [--cycle <name>]` | 1:1 prep sheet |
| `mgmt review <person> [--cycle <name>]` | review evidence pack |
| `mgmt status [--cycle <name>]` | team overview and cycle conditions |
| `mgmt explain <person> <competency> [--cycle <name>]` | why a competency has its status |
| `mgmt report <name> [<person>]` | run a workspace report template (`reports/<name>.tmpl`) |
| `mgmt facts` | print the facts document as JSON |

`<person>` accepts a Person name or alias.

## Notes syntax

Person and date are inherited from context, so a mark is a single token.

```markdown
# 2026-W43                      <!-- date: file name YYYY-Www.md or YYYY-MM-DD*.md -->

## 1:1 Sam                      <!-- heading names the person (name or alias) -->
- +ownership led the review     <!-- positive evidence -->
- -planning quiet in refinement <!-- negative evidence -->
- ~crossteam discussed Zoa      <!-- discussed, neutral -->
- [ ] send the L4 section @28oct ~planning   <!-- open action, due date, addresses a competency -->
- [x] done thing                <!-- closed action -->
- !self-assessment submitted    <!-- ReviewCycle input received -->

## Platform sync
- @priya +crossteam unblocked the handover   <!-- @person attributes the line -->
```

`@date` accepts `2026-10-28`, `28oct`, `28 Oct` and weekday names (`@fri`). A heading can carry its own date (`## Sam 2026-10-22`). Add `<!-- mgmt:ignore <id> -->` to a line to silence a warning there.

## Extension points

No rebuild needed for any of these.

- **Policies:** every `.rego` under `policy/` loads alongside the core policies. Packages are `mgmt.policy.<name>`, may read `data.mgmt.derive`, and emit `deny` and `warn` sets of `{id, person, file, line, message}`.
- **Kinds:** a CUE definition under `schema/` registers a new `kind`; its resources appear in facts under `resources.<Kind>`.
- **Reports:** a template in `reports/` named like a core report (`prep`, `review`, `explain`, `status`) replaces it; any other `reports/<name>.tmpl` runs with `mgmt report <name> [<person>]`.
- **Thresholds:** override warning thresholds in `Workspace.spec.policy.<id>`.

Escape hatch (best effort): policies follow Conftest conventions, so
`mgmt facts | conftest test --policy policy/ -` works.
