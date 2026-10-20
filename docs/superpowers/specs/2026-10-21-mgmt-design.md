# mgmt — Line Management Evidence Tool Spec

Oct 21, 2026 · @Robert

## Overview

`mgmt` is a local, plain-text tool for line managers: it maps each report against a competency framework, finds evidence for those competencies in the manager's existing weekly notes, and reports gaps, open actions and review readiness.

The manager keeps writing freeform weekly markdown notes. Structured facts (people, framework, review cycles) live in Kubernetes-style YAML resources. A linter and report generator join the two.

**Goals**

- Discover gaps rather than declare them: a gap is a required competency that is unproven or disproven by evidence.
- Near-zero syntax in notes: context (person, date) is inherited, so markers are a single token.
- Catch forgetting: warn when notes about a person carry no evidence at all.
- Produce a 1:1 prep sheet and a review evidence pack per person on demand.
- Be extensible to a specific company's review process without changing the core.

**Non-goals**

- No GUI, server, database or sync. Files in, text out.
- No integration with Leapsome or other HR systems in v1 (data is entered as resources or notes).
- No automatic rating or promotion decisions. The tool surfaces evidence; the manager judges.

## Core model

Gaps are derived, never written. Each person's required competencies come from their role and level; evidence in notes proves or disproves each one within an evidence window.

1. **Person** = name, role, level (plus aliases).
2. **Required competencies** = the framework's expectations for that role at that level, including those inherited from lower levels unless a level overrides them.
3. **Evidence** = a positive (`+`) or negative (`-`) mark against one competency, with a person, a date and a source location (file:line).
4. **Evidence window** = the active review cycle's period, or a rolling 90 days when no cycle applies (configurable).
5. **Competency status** per person, computed over the window:

| Status | Rule |
| --- | --- |
| proven | at least `minPerCompetency` positive marks, and the most recent mark is positive |
| disproven | the most recent mark is negative |
| unproven | no marks in the window |

6. **Gap** = a required competency that is unproven or disproven.
7. **Action** = an open markdown checkbox under a person. An action that contains a competency token *addresses* that competency.

The most-recent-mark rule is the default (`negativeOverrides: true`). A cycle may switch to counting instead: proven when positives exceed negatives.

## Resources

All structured data is YAML in the Kubernetes manifest shape (`apiVersion`, `kind`, `metadata.name`, `spec`). Files live under `resources/`, any number of documents per file, separated by `---`. `apiVersion` for v1 is `mgmt/v1alpha1`.

**Workspace** (exactly one): where notes live and global defaults.

```yaml
apiVersion: mgmt/v1alpha1
kind: Workspace
metadata:
  name: default
spec:
  notes: notes/**/*.md          # glob, relative to repo root
  me: [me, robert]              # tokens meaning the manager in actions
  defaultWindow: 90d            # used when no ReviewCycle is active
  oneToOneCadence: 14d          # default, overridable per Person
```

**CompetencyFramework**: competencies, their note tokens, and what each role expects at each level. A competency id's prefix (`collab.`, `delivery.`) is its theme.

```yaml
apiVersion: mgmt/v1alpha1
kind: CompetencyFramework
metadata:
  name: eng
spec:
  competencies:
    - id: delivery.ownership
      title: Owns outcomes end to end
      tokens: [ownership, own]
    - id: collab.planning
      title: Contributes actively to planning and refinement
      tokens: [planning, plan]
    - id: scope.crossteam
      title: Delivers work that spans teams
      tokens: [crossteam, xteam]
  roles:
    - name: platform-engineer
      levels:
        - name: L2
          expects: [delivery.ownership]
        - name: L3
          inherits: L2
          expects: [collab.planning, scope.crossteam]
          removes: []           # optional: drop an inherited expectation
```

**Person**: who they are. No gaps, no ratings: those are derived.

```yaml
apiVersion: mgmt/v1alpha1
kind: Person
metadata:
  name: sam
spec:
  displayName: Sam Example
  aliases: [Sam, Samuel]        # header names that resolve to this person
  framework: eng
  role: platform-engineer
  level: L3
  since: 2026-05-01             # start of managing them; evidence before this is ignored
  oneToOneCadence: 7d           # optional override
```

**ReviewCycle**: one formal review period and the company's process for it. Ensek-specific behaviour lives here and in policies keyed off it, not in the core.

```yaml
apiVersion: mgmt/v1alpha1
kind: ReviewCycle
metadata:
  name: ensek-2026-h2
spec:
  framework: eng
  period: { from: 2026-04-01, to: 2026-09-30 }    # evidence window
  selector:
    people: [sam, priya, jo]     # or matchRoles: [platform-engineer]
  evidence:
    minPerCompetency: 1
    negativeOverrides: true
  inputs:                        # external inputs, ticked off as received
    - name: self-assessment
      source: leapsome
      required: true
    - name: peer-feedback
      source: leapsome
      minCount: 3
    - name: okrs
      required: true
  milestones:
    - { name: peer-nominations, due: 2026-10-27 }
    - { name: peer-feedback,    due: 2026-11-07 }
    - { name: manager-draft,    due: 2026-11-14 }
    - { name: calibration,      due: 2026-11-21 }
    - { name: conversations,    due: 2026-11-28 }
  outcomes:
    ratings: [below, meets, exceeds]
    promotionNomination: true
```

Receipt of cycle inputs is recorded in notes with an input token (see Notes format), so the YAML stays declarative.

**Status is output only.** Commands print a Kubernetes-style `status` block (conditions with `type`, `status`, `reason`, `message`). The tool never writes to resource files.

## Notes format

Notes stay freeform weekly markdown. Person and date are inherited from context, so a marker is one token. The manager must never have to write a person or date on a marked line.

```markdown
# 2026-W43

## 1:1 Sam
- talked through the Cloudflare work, keen on it
- +ownership led the incident review well
- -planning quiet in refinement again
- ~crossteam discussed the Zoa access ask
- [ ] send him the L4 section @28oct ~planning
- [ ] @sam draft the CDN provisioning plan @fri
- !self-assessment submitted in Leapsome

## Platform sync
- @priya +crossteam unblocked the SRE handover
```

**Date**

- From the file name: `YYYY-Www.md` (ISO week, dated to its Monday) or `YYYY-MM-DD*.md`.
- Overridden by a date in the H1 or in a section heading (`## Sam 2026-10-22`, `## Sam (Wed 22 Oct)`), which applies to that section.
- A file with no resolvable date is an error.

**Person context**

- A heading at any level sets the current person when it contains a Person's name or alias as a whole word, case-insensitive (`## 1:1 Sam`, `### Sam`).
- Context lasts until the next heading at the same or a higher level. A heading that names no person clears it.
- `@<person>` on a line attributes that line to that person, overriding context. This allows marks in team sections.
- A heading naming two people is an error unless every marked line under it carries `@<person>`.

**Tokens**

| Token | Meaning | Counts as evidence |
| --- | --- | --- |
| `+<competency>` | positive evidence | yes |
| `-<competency>` | negative evidence | yes |
| `~<competency>` | discussed or addressed, neutral | no |
| `- [ ] …` | open action | no |
| `- [x] …` | closed action | no |
| `@<person>` | attribute line, or set action owner | no |
| `@<date>` | action due date | no |
| `!<input>` | ReviewCycle input received | no |

- A token must be preceded by start of line, whitespace or `(`, and the sigil must be followed by a letter. This excludes list bullets, `+1`, `-5` and hyphenated words.
- `<competency>` resolves against a framework's `tokens`, its full id (`collab.planning`) or its id suffix (`planning`). Matching is case-insensitive. Unresolved tokens are errors with a closest-match suggestion.
- Several tokens on one line create one record each, sharing the line's text.
- Inside an action line, `+` and `-` tokens are an error. Use `~` to mark the competency the action addresses.
- Action owner is the manager (`Workspace.spec.me`) unless the line carries `@<person>`.
- `@<date>` accepts `2026-10-28`, `28oct`, `28 Oct` and weekday names (`@fri`), resolved forward from the entry date. Whether `@x` is a date or a person is decided by trying the date grammar first.
- Actions are identified by file and line text. They are closed by ticking them in place. Carrying an action forward by rewriting it in a later week creates a duplicate, which a policy flags.

## Architecture and toolchain

`mgmt` is a single static Go binary that embeds three declarative engines: CUE for resource schemas, OPA/Rego for derivation and policy, and Go templates for reports. Go code is limited to loading files, extracting facts from markdown, and wiring the stages together.

| Stage | Implementation | Output |
| --- | --- | --- |
| Load | Go: YAML via `sigs.k8s.io/yaml`, markdown via `goldmark` with the GFM task-list extension | raw resources, markdown AST |
| Validate | CUE definitions in `schema/*.cue`, run through the `cuelang.org/go` API | schema errors |
| Extract | Go: walk the AST, apply context rules and the token grammar | facts document (JSON) |
| Derive | Rego package `mgmt.derive`, run through the OPA Go SDK | required competencies, statuses, gaps, coverage |
| Policy | Rego packages `mgmt.policy.*` with `deny` and `warn` rules | errors and warnings |
| Render | Go `text/template` plus Sprig functions, templates in `reports/*.tmpl` | markdown reports |

**Facts document** (the contract between Go and Rego):

```json
{
  "now": "2026-10-21",
  "resources": { "Workspace": {}, "CompetencyFramework": {}, "Person": {}, "ReviewCycle": {} },
  "evidence": [ { "person": "sam", "competency": "collab.planning", "polarity": -1,
                   "date": "2026-10-19", "file": "notes/2026-W43.md", "line": 6, "text": "quiet in refinement again" } ],
  "touches":  [ { "person": "sam", "competency": "scope.crossteam", "date": "…", "file": "…", "line": 7 } ],
  "actions":  [ { "id": "sha1(file+text)", "person": "sam", "owner": "me", "due": "2026-10-28",
                   "done": false, "addresses": ["collab.planning"], "date": "…", "file": "…", "line": 8, "text": "…" } ],
  "inputs":   [ { "person": "sam", "input": "self-assessment", "date": "…", "file": "…", "line": 10 } ],
  "sections": [ { "person": "sam", "date": "2026-10-19", "file": "…", "line": 3, "tokenCount": 6 } ],
  "diagnostics": [ { "severity": "error", "code": "unknown-token", "file": "…", "line": 12, "message": "…", "suggest": "planning" } ]
}
```

**Why this split**

- Gap discovery is set logic (required minus proven). Rego expresses it in a few rules and supports `deny` and `warn` natively.
- CUE gives typed resources with constraints (valid levels per role, unique tokens, ISO dates) without hand-written validation.
- Policies follow Conftest conventions, so `mgmt facts | conftest test --policy policy/ -` works as an escape hatch.
- Templates keep report layout editable without recompiling.

Extraction diagnostics, CUE errors and Rego `deny` results all surface as errors; Rego `warn` results as warnings.

## Policies

Errors mean the data is wrong and block reports. Warnings mean the data is valid but something needs the manager's attention. Thresholds have defaults and are overridable in `Workspace.spec.policy.<id>`.

**Errors**

| Id | Fires when |
| --- | --- |
| `schema-invalid` | a resource fails its CUE definition |
| `ref-unknown` | a Person names a framework, role or level that does not exist |
| `token-duplicate` | one token maps to two competencies |
| `token-unknown` | a `+`, `-` or `~` token resolves to no competency (with suggestion) |
| `person-unknown` | `@name` matches no Person alias and no date |
| `date-unresolved` | a notes file or section has no resolvable date |
| `heading-multi-person` | a heading names two people and a marked line under it has no `@person` |
| `evidence-in-action` | an action line contains a `+` or `-` token |
| `input-unknown` | `!name` matches no input in the active ReviewCycle |

**Warnings**

| Id | Fires when | Default |
| --- | --- | --- |
| `section-unmarked` | a person section has no tokens of any kind | — |
| `negative-unaddressed` | a `-` mark has no open or closed action with `~` for that competency dated on or after it | 7 days grace |
| `gap-unaddressed` | a gap (unproven or disproven) has no open action addressing it | — |
| `competency-untouched` | a required competency has no mark or touch for a person within the window | 42 days |
| `action-overdue` | an open action is past its due date | — |
| `action-no-due` | an open action has no due date | — |
| `action-duplicate` | the same open action text appears in more than one place | — |
| `oneonone-overdue` | no section for a person within their cadence | Person or Workspace cadence |
| `cycle-input-missing` | a required input is not recorded and the next milestone is near | 7 days before |
| `cycle-evidence-thin` | gaps remain and the `manager-draft` milestone is near | 14 days before |

A line ending in `<!-- mgmt:ignore <id>[,<id>] -->` suppresses those warnings for that line. Errors cannot be suppressed.

Company-specific policies (for example, an OKR rule for Ensek) are additional Rego packages under `policy/` and need no core change.

## CLI and reports

All commands run from the workspace root, read only, and print markdown to stdout (`--out <file>` to write a file). Every evidence item and action is printed with a `file:line` reference so the manager can jump to it.

| Command | Purpose |
| --- | --- |
| `mgmt init` | scaffold a workspace with example resources, notes and policies |
| `mgmt lint [--strict] [--format text\|json]` | print errors and warnings; exit 2 on errors, 1 on warnings with `--strict`, else 0 |
| `mgmt prep <person>` | 1:1 prep sheet |
| `mgmt review <person> [--cycle <name>]` | review evidence pack |
| `mgmt status [--cycle <name>]` | team overview and cycle conditions |
| `mgmt explain <person> <competency>` | why a competency has its status |
| `mgmt facts` | print the facts document as JSON |

`<person>` accepts a Person name or alias. `--cycle` defaults to the ReviewCycle whose period or milestones include today.

**`prep` sheet sections**, in order:

1. Open actions: overdue first, then by due date; mine and theirs listed separately.
2. Since last 1:1: marks and touches dated after the person's previous section.
3. Gaps: disproven first, then unproven, each showing whether an action addresses it.
4. Suggested focus: the required competency touched least recently.
5. Cycle: next milestone and any missing inputs for this person.

**`review` pack**: one section per required competency, grouped by theme, each with its status, positive and negative evidence (newest first, with dates and sources), and actions that addressed it. It ends with cycle inputs received and missing. It never states a rating.

**`status`**: a person × competency matrix (proven, disproven, unproven), then a `status` block per active ReviewCycle with conditions `EvidenceComplete`, `InputsComplete` and `ActionsCurrent`.

**`explain`**: the rule applied, the window, and every evidence item considered, with which one decided the status.

## Repo layout and extensibility

The tool and the manager's data are separate repos. Core schemas, policies and templates are embedded in the binary; a workspace adds or overrides them without rebuilding.

**Tool repo**

```
cmd/mgmt/            CLI entry point
internal/load/       YAML and markdown loading
internal/extract/    context rules, token grammar, facts builder
internal/engine/     CUE validation, OPA evaluation
internal/render/     template rendering
schema/              core CUE definitions (embedded)
policy/core/         derive.rego + core policies (embedded)
reports/             prep, review, status, explain templates (embedded)
testdata/            fixture workspaces with golden outputs
```

**Workspace repo** (the manager's data)

```
resources/workspace.yaml
resources/framework.yaml
resources/people.yaml
resources/cycles/ensek-2026-h2.yaml
notes/2026-W43.md
policy/ensek/        company-specific Rego (optional)
schema/ensek/        company-specific CUE kinds (optional)
reports/             template overrides and extra reports (optional)
```

**Extension points**

- **Policies:** every `.rego` under the workspace `policy/` loads alongside core. Packages are `mgmt.policy.<name>`, may read `data.mgmt.derive`, and emit `deny` and `warn` sets of `{id, person, file, line, message}`.
- **Kinds:** a CUE definition under the workspace `schema/` registers a new `kind`. Its resources appear in facts under `resources.<Kind>`. A resource whose kind has no schema is an error.
- **Reports:** a workspace template with a core template's name replaces it. Any other template runs via `mgmt report <name> [<person>]`.
- **Token grammar** is fixed in v1.

**Worked example: Ensek OKRs.** Add `kind: OKRSet` (person, cycle, objectives with key results and linked competencies) under `schema/ensek/`, and a policy `okr-no-evidence` under `policy/ensek/` that warns when a linked competency is unproven at `manager-draft` minus 14 days. No Go changes.

## Constraints, testing and acceptance

**Constraints**

- No network access, telemetry or external calls at runtime. Notes contain personal data about employees.
- Read only: the tool never modifies notes or resources.
- Single static binary for Linux and macOS; Go 1.23 or later.
- Deterministic output: same inputs and `--now` give byte-identical reports. `--now <date>` overrides today for tests.
- `mgmt lint` on a workspace of 52 weekly files and 10 people completes in under 1 second.

**Testing**

- Unit tests for the token grammar, date resolution and person-context rules, including every exclusion case (`+1`, `-5`, list bullets, hyphenated words).
- Rego unit tests (`opa test`) for every derive rule and policy, firing and not firing.
- Golden tests: fixture workspaces under `testdata/`, each with expected `lint`, `prep`, `review` and `status` output at a fixed `--now`.

**Milestones and acceptance**

1. Load, validate, extract
   - [ ] Core CUE schemas for all four kinds; `schema-invalid` and `ref-unknown` fire on fixtures
   - [ ] Extractor produces the facts document for the Notes format example exactly
   - [ ] All extraction errors in the Policies table fire with file, line and suggestion
   - [ ] `mgmt facts` and `mgmt lint` (errors only) work
2. Derive and warn
   - [ ] Required competencies resolve with `inherits` and `removes`
   - [ ] Statuses follow the Core model table under both `negativeOverrides` settings
   - [ ] Every warning in the Policies table fires on a fixture; `mgmt:ignore` suppresses it
3. Reports
   - [ ] `prep`, `review`, `status` and `explain` match golden output
   - [ ] Every printed item carries a `file:line` reference
4. Extensions
   - [ ] Workspace policies, kinds and templates load without rebuilding
   - [ ] The Ensek OKR example works end to end from a fixture
   - [ ] `mgmt init` scaffolds a workspace that passes `lint` with zero errors

**Open questions**

- Level changes inside a window: v1 uses the level at the window's end. Should earlier evidence be judged against the earlier level?
- Importing Leapsome peer feedback as evidence is out of scope for v1. Decide whether a later version reads an export file.

## Clarifications (2026-10-21)

Settled during planning. Items marked *builder's call* were not stated by Robert and may be overridden.

- **Facts example dates.** `notes/2026-W43.md` is dated to its Monday, 2026-10-19. The facts example above was corrected from 2026-10-20.
- **Too few positives** *(builder's call)*. Under `negativeOverrides: true`, a competency whose most recent mark is positive but has fewer than `minPerCompetency` positives is **unproven**. Under counting (`negativeOverrides: false`): no marks → unproven; positives > negatives and positives ≥ `minPerCompetency` → proven; positives > negatives but below the minimum → unproven; otherwise → disproven.
- **Example window.** In the worked example, `ensek-2026-h2`'s window ends 2026-09-30, so the W43 marks fall outside it. The cycle is still active on 2026-10-21 (its active range runs from `period.from` to the later of `period.to` and the last milestone), so `!self-assessment` resolves.
- **`tokenCount`** *(builder's call)* counts the records a section produces: marks, touches, actions and inputs. `@` tokens and `~` tokens inside actions are not counted separately. The worked example's Sam section therefore has 6.
- **Go floor** *(builder's call)*. Current OPA (v1.21) needs Go 1.26 and current CUE needs 1.25, so `go.mod` declares `go 1.26`. This replaces "Go 1.23 or later".
