# mgmt

A read-only CLI that joins a line manager's weekly markdown notes with
Kubernetes-style YAML resources (people, competency frameworks, review cycles).
It extracts evidence, actions and cycle inputs from the notes, validates the
resources (CUE), derives statuses and findings (Rego, via the OPA Go SDK) and
prints reports (Go templates): 1:1 prep sheets, review evidence packs, team
status. It never modifies your notes.

## Install

You need Go 1.26 or later. From a clone of this repo:

```sh
go install ./cmd/mgmt          # puts mgmt in $(go env GOPATH)/bin
```

Or build static binaries for Linux and macOS (amd64 and arm64) with
`make build`; they land in `dist/mgmt-<os>-<arch>`, and you copy the one you
need onto your `PATH`. `make test` runs the tests.

mgmt makes no network calls and never writes to your notes or resources.

## Quick start

Keep your management data in its own directory (ideally its own private git
repo), separate from this tool:

```sh
mkdir my-team && cd my-team
mgmt init          # writes example resources/, notes/, policy/ and reports/ (never overwrites)
mgmt lint          # 0 errors on the scaffold
mgmt prep alex     # 1:1 prep sheet for the example person
```

Then replace the example people, framework and cycle with your own (see
[Resources](#resources)) and start writing weekly notes.

## A workspace

```
my-team/
  resources/workspace.yaml      where notes live, defaults
  resources/framework.yaml      competencies and their descriptor at each level
  resources/people.yaml         your reports
  resources/cycles/*.yaml       review cycles (optional)
  notes/2026-W43.md             one markdown file per week (or per day)
  policy/                       your own Rego policies (optional)
  reports/                      your own report templates (optional)
```

Run every command from the workspace root. Files under `resources/` may hold
any number of YAML documents separated by `---`.

## Resources

All resources share the Kubernetes shape: `apiVersion: mgmt/v1alpha1`, `kind`,
`metadata.name` (lower-case, used to refer to it) and `spec`. Dates are
`YYYY-MM-DD`; durations are `<n>d` or `<n>w`.

**Workspace** (exactly one):

```yaml
apiVersion: mgmt/v1alpha1
kind: Workspace
metadata: {name: default}
spec:
  notes: notes/**/*.md          # glob: relative to the workspace root, absolute, or ~/...
  me: [me]                      # tokens meaning you in actions (@me)
  defaultWindow: 90d            # evidence window when no review cycle applies
  oneToOneCadence: 14d          # how often you expect a 1:1
```

`spec.notes` can point outside the workspace, so you can read an existing
journal in place, for example an Obsidian vault:

```yaml
  notes: ~/Obsidian/MyVault/Journal/**/*.md
```

Globs starting with `~/` expand to your home directory, and absolute globs
(`/home/me/Vault/Journal/**/*.md`) work too. Files outside the workspace show
up in `file:line` references with their absolute path. mgmt only reads them. Relative globs may use `..` (`../journal/**/*.md`).
The folder part of the glob must not contain glob characters (`[ ] { } * ?`),
so a vault called `Work [2026]` needs a symlink or a renamed folder.

**CompetencyFramework**: a competency × level matrix. `levels` is the ordered
ladder, lowest first. Each competency has an id, a title, the short tokens you
write in notes, an optional `summary`, and a descriptor per level: the bar a
person at that level is judged against. A competency is **required** for a
person when it has a descriptor at their level; leave a level out where the
competency doesn't apply. An id's prefix (`core.`) is its theme.

Shared competencies live in one framework that others `includes`, rather than
being copied. A framework's competencies are its own plus everything it
includes, transitively, and its ladder must match each included framework's.

```yaml
apiVersion: mgmt/v1alpha1
kind: CompetencyFramework
metadata: {name: core}
spec:
  levels: [IC1, IC2, IC3]
  competencies:
    - id: core.ownership
      title: Owns outcomes end to end
      tokens: [ownership, own]
      summary: Takes responsibility for results, not just tasks.
      levels:
        IC1: Finishes the tasks they pick up and says early when they are stuck.
        IC2: Sees their own work through to done, including the follow-up.
        IC3: Owns a whole piece of work from idea to outcome without being chased.
---
apiVersion: mgmt/v1alpha1
kind: CompetencyFramework
metadata: {name: software-engineering}
spec:
  includes: [core]
  levels: [IC1, IC2, IC3]
  competencies:
    - id: eng.planning
      title: Contributes to planning and refinement
      tokens: [planning, plan]
      levels:                   # not required at IC1
        IC2: Breaks their own work into clear, sized steps.
        IC3: Shapes the plan for their area and names risks up front.
```

**Person**: one per report. Gaps and statuses are never written here; they
are derived from your notes.

```yaml
apiVersion: mgmt/v1alpha1
kind: Person
metadata: {name: alex}
spec:
  displayName: Alex Example
  aliases: [Alex]               # how you write their name in headings
  framework: software-engineering
  level: IC2                    # a level on the framework's ladder
  since: 2026-09-01             # evidence before this date is ignored
  levelSince: 2026-10-01        # optional: reached this level; evidence before it is ignored (default: since)
  oneToOneCadence: 7d           # optional override
```

**ReviewCycle** (optional): a formal review period. While a cycle is active
(from `period.from` to its last milestone), its `period` is the evidence
window for the people it selects; otherwise the rolling `defaultWindow` is used.

```yaml
apiVersion: mgmt/v1alpha1
kind: ReviewCycle
metadata: {name: h2-2026}
spec:
  period: {from: 2026-07-01, to: 2026-12-31}
  selector: {people: [alex]}    # or matchFrameworks: [core] for everyone whose framework is or includes core
  evidence: {minPerCompetency: 1, negativeOverrides: true}
  inputs:                       # things to collect; record receipt with !name in notes
    - {name: self-assessment, required: true}
    - {name: peer-feedback, minCount: 3}
  milestones:
    - {name: manager-draft, due: 2026-12-01}
```

A competency is **proven** when it has at least `minPerCompetency` positive
marks in the window and the latest mark is positive, **disproven** when the
latest mark is negative, and **unproven** otherwise. With
`negativeOverrides: false` it is proven when positives outnumber negatives
instead. A required competency that is unproven or disproven is a **gap**.

## A typical week

1. Write your notes as usual in `notes/<year>-W<week>.md`, adding a `+`, `-`
   or `~` token wherever something shows a competency (see below).
2. `mgmt lint` to catch typos (it suggests the closest token or name) and to
   see what needs attention: overdue actions, unaddressed negatives, people
   you haven't had a 1:1 with.
3. Before a 1:1: `mgmt prep <person>` for open actions, the `++`/`--` marks
   to raise, what happened since last time, gaps and a suggested focus.
4. At review time: `mgmt review <person> --out <person>-review.md` for the
   evidence pack (each competency with the bar at the person's level), and
   `mgmt status` for the team picture. `mgmt explain <person> <competency>`
   shows exactly which notes decided a status.

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
| `mgmt lint [--strict] [--format text\|flat\|json]` | print errors and warnings; exit 2 on errors, 1 on warnings with `--strict`, else 0. `text` (default) is markdown: errors grouped by id first, then each person's warnings grouped by id, each with its `file:line`; `flat` is one `file:line: severity id [person]: message` line per finding, for editors and grep |
| `mgmt prep <person> [--cycle <name>]` | 1:1 prep sheet |
| `mgmt review <person> [--cycle <name>]` | review evidence pack |
| `mgmt promo <person>` | draft promotion packet: for each next-level competency, both descriptors and the `++` evidence since the person reached their level, then the competencies with no `++` yet; collected evidence, never a recommendation |
| `mgmt status [--cycle <name>] [--format markdown\|yaml]` | team matrix, then per active cycle the next milestone and three checks (evidence gaps, missing inputs, overdue actions) listing who and what; `yaml` prints the checks as Kubernetes-style `status.conditions` for scripts |
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

- The file name (`YYYY-Www.md`, `YYYY-MM-DD.md` or `YYYY-MM-DD anything.md`,
  as Obsidian daily and weekly notes are named) or the H1 dates the file.
  A heading can carry its own date (`## Sam 2026-10-22`, `## Sam (Wed 22 Oct)`).
- YAML front matter (a leading `---` block closed by `---` or `...`) is ignored.
  Obsidian links work in headings: `## 1:1 [[Sam]]` sets Sam.
- A heading that names a Person (by name or alias) sets the person for the lines
  under it. `@person` on a line attributes that line to someone else.
- `+x` is positive evidence, `-x` negative evidence and `~x` a neutral "discussed"
  touch, where `x` is a competency id or token.
- `++x` and `--x` are strong evidence: praise worth raising and a serious
  concern. A `++` also shows work at the next level up; a `--` counts as a
  negative mark and needs addressing like any other. Each still counts as one
  mark in status. They are not allowed on action lines.
- `- [ ]` is an open action and `- [x]` a closed one. On an action, `@date` sets
  the due date, `@person` the owner (default: you) and `~x` the competency it
  addresses. `@date` accepts `2026-10-28`, `28oct`, `28 Oct` and weekday names
  (`@fri`).
- `!name` records a ReviewCycle input as received.
- A token must start a word (after a space, `(` or the line start), so
  `+1`, `-5`, `C++`, `a--b` and hyphenated words are never tokens, and nothing inside code
  spans, code blocks or links counts.

## Errors and warnings

Errors block reports; warnings never do.

Errors: `schema-invalid`, `ref-unknown`, `token-duplicate`, `alias-duplicate`,
`token-unknown`, `person-unknown`, `person-missing`, `line-multi-person`,
`heading-multi-person`, `evidence-in-action`, `date-unresolved`,
`input-unknown`.

Warnings: `section-unmarked`, `negative-unaddressed`, `gap-unaddressed`,
`competency-untouched` (not raised for a competency that already has
`gap-unaddressed`), `action-overdue`, `action-no-due`, `action-duplicate`,
`oneonone-overdue`, `cycle-input-missing`, `cycle-evidence-thin`,
`no-next-level` (a `++` mark for a person already at the top level of their
framework's ladder).

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
- **Reports:** a template in `reports/` named like a core report (`prep`, `review`, `promo`, `explain`, `status`, and `status-yaml` for `status --format yaml`) replaces it; any other `reports/<name>.tmpl` runs with `mgmt report <name> [<person>]`.
- **Thresholds:** override warning thresholds in `Workspace.spec.policy.<id>` (see [Errors and warnings](#errors-and-warnings)).

### Facts document

`mgmt facts` prints the input every policy and report sees. Two parts matter
most for policy authors:

- `frameworks.<name>` is the effective framework: `levels` (the ladder) and
  `competencies.<id>` with `id`, `title`, `theme`, `summary` and `levels`
  (`<level>: <descriptor>`), including everything pulled in by `includes`.
  Read competency data from here, not from `resources.CompetencyFramework`.
- `evidence[]` records carry `polarity` (`1` or `-1`) and `strength` (`1`, or
  `2` for `++`/`--`).

Escape hatch (best effort): policies follow Conftest conventions, so
`mgmt facts | conftest test --policy policy/ -` works.
