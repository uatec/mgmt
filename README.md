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

All commands run from the workspace root and print to stdout. Two global flags
work before or after the command: `--out FILE` writes the output to FILE
instead (created with mode 0600, since reports carry personal data) and
`--now YYYY-MM-DD` fixes today's date. `mgmt --now 2026-10-21 lint` and
`mgmt lint --now 2026-10-21` are the same; so are `mgmt --out sam.md review sam`
and `mgmt review sam --out sam.md`.

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
# 2026-W43

## 1:1 Sam
- +ownership led the review
- -planning quiet in refinement
- ~crossteam discussed Zoa
- [ ] send the L4 section @28oct ~planning
- [x] done thing
- !self-assessment submitted

## Platform sync
- @priya +crossteam unblocked the handover
```

- The file name (`YYYY-Www.md` or `YYYY-MM-DD*.md`) or the H1 dates the file.
  A heading can carry its own date (`## Sam 2026-10-22`, `## Sam (Wed 22 Oct)`).
- A heading that names a Person (by name or alias) sets the person for the lines
  under it. `@person` on a line attributes that line to someone else.
- `+x` is positive evidence, `-x` negative evidence and `~x` a neutral "discussed"
  touch, where `x` is a competency id or token.
- `- [ ]` is an open action and `- [x]` a closed one. On an action, `@date` sets
  the due date, `@person` the owner (default: you) and `~x` the competency it
  addresses. `@date` accepts `2026-10-28`, `28oct`, `28 Oct` and weekday names
  (`@fri`).
- `!name` records a ReviewCycle input as received.

## Errors and warnings

Errors block reports; warnings never do.

Errors: `schema-invalid`, `ref-unknown`, `token-duplicate`, `alias-duplicate`,
`token-unknown`, `person-unknown`, `person-missing`, `line-multi-person`,
`heading-multi-person`, `evidence-in-action`, `date-unresolved`,
`input-unknown`.

Warnings: `section-unmarked`, `negative-unaddressed`, `gap-unaddressed`,
`competency-untouched`, `action-overdue`, `action-no-due`, `action-duplicate`,
`oneonone-overdue`, `cycle-input-missing`, `cycle-evidence-thin`.

Warning thresholds are set in the Workspace as
`spec.policy.<id>.<param>` (a whole number of days, or a duration such as `2w`):

| Parameter | Default |
| --- | --- |
| `negative-unaddressed.graceDays` | 7 |
| `competency-untouched.days` | 42 |
| `cycle-input-missing.daysBefore` | 7 |
| `cycle-evidence-thin.daysBefore` | 14 |

```yaml
spec:
  policy:
    competency-untouched: {days: 56}
```

`oneonone-overdue` uses the Person's `oneToOneCadence`, else the Workspace's.

Silencing a warning (errors are never silenced):

- In notes, put `<!-- mgmt:ignore <id>[,<id>] -->` on the line the warning points at.
- For a warning that points at a resource (for example `oneonone-overdue`,
  `gap-unaddressed` or `competency-untouched` on a Person, or
  `cycle-input-missing` on a ReviewCycle), add the YAML comment
  `# mgmt:ignore <id>[,<id>]` anywhere in that resource's document.

## Extension points

No rebuild needed for any of these.

- **Policies:** every `.rego` under `policy/` loads alongside the core policies. Packages are `mgmt.policy.<name>`, may read `data.mgmt.derive`, and emit `deny` and `warn` sets of `{id, person, file, line, message}`. `mgmt init` writes a commented example, `policy/example/example.rego`.
- **Kinds:** a CUE definition under `schema/` registers a new `kind`; its resources appear in facts under `resources.<Kind>`.
- **Reports:** a template in `reports/` named like a core report (`prep`, `review`, `explain`, `status`) replaces it; any other `reports/<name>.tmpl` runs with `mgmt report <name> [<person>]`.
- **Thresholds:** override warning thresholds in `Workspace.spec.policy.<id>` (see [Errors and warnings](#errors-and-warnings)).

Escape hatch (best effort): policies follow Conftest conventions, so
`mgmt facts | conftest test --policy policy/ -` works.
