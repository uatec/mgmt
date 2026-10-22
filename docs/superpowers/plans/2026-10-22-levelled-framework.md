# Levelled framework, ++/-- marks, promotion packet: implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace mgmt's role/expects framework model with a competency × level matrix (shared core via `includes`), add `++`/`--` marks, and add a draft promotion packet. Then load Robert's real Software Engineering framework into the demo workspace.

**Architecture:** Go resolves framework `includes` once, during validation and extraction, and passes each framework's *effective* ladder and competencies to Rego as a new `frameworks` key in the facts document. Rego derives required competencies from "has a descriptor at the person's level" and exposes descriptors to the report view-models. Strength (`++`/`--`) is a new field on evidence records; status rules are unchanged.

**Tech stack:** as v1 (Go, CUE, OPA/Rego v1, text/template + Sprig, goldmark).

**Spec:** `docs/superpowers/specs/2026-10-22-levelled-framework-design.md`. This plan builds on the v1 spec `docs/superpowers/specs/2026-10-21-mgmt-design.md` and the v1 plan's Global Constraints, Decisions A–M and controller rulings R1–R18, which all still bind unless this plan changes them.

## Global Constraints

- All v1 constraints: read-only, deterministic, no network, `file:line` on printed evidence and actions, lint < 1 s on 52 weekly files × 10 people, rules total over schema-valid facts (R8), `go 1.26`.
- **No company framework text in the tool repo** (public). Tool-repo frameworks, fixtures, goldens and README use invented, generic descriptors. The real text lives only in `~/code/mgmt-demo` (not in git). Source: `~/code/mgmt-demo/framework-source/software-engineering.txt`.
- The v1 framework schema is replaced outright (no compatibility layer). `roles`, `expects`, `inherits`, `removes`, `Person.spec.role`, `ReviewCycle.spec.framework` and `selector.matchRoles` are removed. Every tool-repo fixture, the scaffold and the README are migrated.
- The level ladder is ordered. "Next level" is the next entry in the ladder.

## Decisions (this plan)

- **N. Facts `frameworks`.** `facts.frameworks[<name>] = {levels: [..], competencies: {<id>: {id, title, theme, summary, levels: {<level>: <descriptor>}}}}`. This is the framework's own competencies plus everything it includes, transitively, resolved in Go. `resources.CompetencyFramework` keeps the raw objects. Rego reads only `frameworks` for competency data.
- **O. Strength.** Evidence records gain `"strength": 1 | 2` (`++`/`--` = 2). `polarity` stays ±1. Touch records have no strength.
- **P. Validation**, all `schema-invalid` at the framework's `file:line`, done in Go next to `frameworkDuplicates`:
  - an `includes` entry that is unknown;
  - an include cycle;
  - a ladder that differs from an included framework's ladder;
  - a descriptor keyed by a level not on the ladder;
  - a duplicate competency id across a framework and its includes.

  Kept as Rego `ref-unknown`: a Person `level` not on their framework's ladder; an unknown framework; an unknown `matchFrameworks` entry. Token collisions across includes are reported as `token-duplicate`.
- **Q. `levelSince`.** The optional `Person.spec.levelSince` (a date) defaults to `since`. Every evidence window's `from` is the latest of (cycle or rolling start, `since`, `levelSince`).
- **R. Selector.** `matchFrameworks` selects a person whose framework is listed, or whose framework includes (transitively) a listed framework.
- **S. New warning** `no-next-level`: a `++` mark for a person at the top of their ladder. It is suppressible and anchored at the mark's line.
- **T. Prep "To raise".** A new section placed after "Open actions". It lists the `++` and `--` marks dated after the last 1:1 (the same bounds as "Since last 1:1"), newest first. Each line reads `- 2026-10-19 ++ core.communication · <text> (file:line)`, with `++` items meaning praise and `--` items meaning concern. "Since last 1:1" also tags these marks as `++`/`--`.
- **U. Promotion packet.** The command is `mgmt promo <person>`, with core template `reports/promo.tmpl`, and `promo` is added to the built-in report names (R16).
  - Evidence: `++` marks with `levelSince ≤ date ≤ now`, regardless of cycle.
  - Layout: one section per competency that has a descriptor at the next level, grouped by theme. Each section shows the current-level descriptor (or "not at this level"), the next-level descriptor, and the `++` evidence newest first with `file:line`.
  - It closes with `## No next-level evidence yet`, a list of those competencies with zero `++`.
  - At the top of the ladder it prints `No next level above <level>.`
  - It never says "ready".

## Review Focus

1. **Token boundaries for `++`/`--`.** `C++`, `a--b`, `x++`, `--` followed by a space, and `++1` are not tokens. `(++comms)` and `++comms,` are. Tested in Task 2.
2. **Ladder edges.** `++` at the top level gives `no-next-level` but still counts as positive. `--` at the bottom is a plain strong negative. The packet at the top level prints its message. Tested in Tasks 2 and 4.
3. **Includes.** A token defined in core resolves for a person whose framework is `software-engineering`, and a typo suggests from the merged token set. Tested in Task 1.
4. **Level not on ladder, or missing descriptors.** A person at a level where a competency has no descriptor isn't required to meet it. That competency's `explain` says "not required at <level>", and the run doesn't crash. Tested in Tasks 1 and 3.
5. **Promotion mid-window.** Marks before `levelSince` are excluded from status and from the packet, and `explain` lists them under "outside window". Tested in Task 3.

---

### Task 1: Framework schema, includes, Person level, selector; migrate the tool repo

**Files:**
- Modify: `schema/core.cue`, `internal/engine/cue.go` (Decision P checks; remove the role and inherits validation), `internal/engine/run.go` (build `facts.frameworks`), `internal/facts/facts.go` (`Frameworks` field), `internal/extract/index.go` (token index from the effective framework; `matchFrameworks`), `policy/core/derive.rego` (`competencies`, `required`, `window` with `levelSince`, `cycle_people`), `policy/core/errors.rego` (`ref-unknown` for level, framework and matchFrameworks; drop the role and inherits rules), all affected `*_test.rego`.
- Migrate every framework, person and cycle YAML under `testdata/*/resources` and `scaffold/resources` to the new shape. Use invented descriptors, and choose ladders and descriptors so that each fixture's *required sets* stay what they were (`L2`/`L3` become an `IC` ladder). This keeps the goldens' meaning stable.
- Update the README "Resources" section.
- Test: `internal/engine/cue_test.go`, `internal/extract/*_test.go`, Rego tests, regenerated goldens.

**Interfaces:**
- Produces: `facts.Facts.Frameworks map[string]Framework` (JSON `frameworks`), with `Framework{Levels []string; Competencies map[string]Competency}` and `Competency{ID, Title, Theme, Summary string; Levels map[string]string}`. It also produces `derive.ladder[p]` (array), `derive.level[p]`, `derive.next_level[p]` (undefined at the top), and `derive.descriptor[p][c]` (the current-level text, undefined if none).
- The `derive` names `competencies[p]`, `required[p]`, `status[p][c]`, `gaps[p]`, `window[p]` and `cycle_people[cy]` keep their v1 shapes (later tasks and workspace policies depend on them). `competencies[p][c]` gains `summary` and `levels`.

- [ ] **Step 1: Write failing tests.**
  - CUE: a valid new-shape framework passes. Each Decision P case gives `schema-invalid` with a message substring.
  - Index: a core token resolves for a `software-engineering` person, and a typo suggests from merged tokens.
  - Rego:
    - `required` for IC3 = the competencies with an IC3 descriptor.
    - The window is clamped by `levelSince`.
    - `matchFrameworks: [core]` selects an SE person.
    - `ref-unknown` fires for a level that isn't on the ladder.
- [ ] **Step 2: Run** `go test ./...`. Expected: the new tests FAIL.
- [ ] **Step 3: Implement**, then migrate the fixtures, scaffold and README.
- [ ] **Step 4: Run** `go test ./... -update` once. Hand-check that every changed golden still says the same thing: the same statuses, gaps and warnings, with only ids, levels or wording changing. Then run `go test ./... && go vet ./...`. Expected: PASS.
- [ ] **Step 5: Commit** `feat!: levelled competency frameworks with includes`.

### Task 2: `++` / `--` marks

**Files:**
- Modify: `internal/extract/tokens.go` (the sigil may be `++` or `--`; `RawToken` gains `Strong bool`), `internal/extract/extract.go` (Evidence `Strength`; `evidence-in-action` covers strong marks), `internal/facts/facts.go`, `policy/core/warnings.rego` (`negative-unaddressed` covers `--`; new `no-next-level`), and the tests.
- Update the README notes syntax and the error and warning lists.

- [ ] **Step 1: Write failing tests.**
  - `ScanLine`: every Review Focus 1 case.
  - Extract: `++comms` gives polarity 1 and strength 2; `--comms` gives −1 and 2. `- [ ] ++comms x` gives `evidence-in-action`.
  - Rego: `--` triggers `negative-unaddressed`. `++` at the top level triggers `no-next-level`, and at a lower level it doesn't. Status counts a `++` as one positive.
- [ ] **Step 2: Run the tests.** Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./... && go vet ./...` (goldens should be unchanged, since no fixture uses `++` yet). Expected: PASS.
- [ ] **Step 5: Commit** `feat: ++ and -- marks`.

### Task 3: Descriptors and strong marks in prep, review and explain

**Files:**
- Modify: `policy/core/report.rego`, `reports/prep.tmpl` (the "To raise" section, Decision T; `++`/`--` tags in "Since last 1:1"), `reports/review.tmpl` (a `Bar (<level>): <descriptor>` line under each competency heading; evidence lines tagged `++`/`--` where strength is 2), `reports/explain.tmpl` (the descriptor line; mark tags; "not required at <level>" when there is no descriptor).
- Add `++`/`--` marks to `testdata/team` notes so the goldens exercise them, and update that fixture's README.
- Test: Rego report tests and goldens.

- [ ] **Step 1: Write failing tests.**
  - The prep view `toRaise` (newest first, after the last 1:1).
  - Review and explain carry the descriptor.
  - A `++` dated before `levelSince` is listed under explain's "Outside window".
- [ ] **Step 2: Run the tests.** Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `-update` once, hand-check, then `go test ./... && go vet ./...`. Expected: PASS.
- [ ] **Step 5: Commit** `feat: level descriptors and ++/-- in prep, review and explain`.

### Task 4: Promotion packet (`mgmt promo`)

**Files:**
- Modify: `policy/core/report.rego` (`report.promo[p]`), `internal/cli/cli.go` (`promo <person>`; `builtinReports`), the README command table.
- Create: `reports/promo.tmpl`.
- Test: Rego tests, goldens (`team promo sam`, a top-of-ladder person, an unknown person with exit 2), and `TestPromoNeverRates` (no "ready", "rating" or "meets" in promo goldens).

- [ ] **Step 1: Write failing tests**, per Decision U.
- [ ] **Step 2: Run the tests.** Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `-update` once, hand-check, then `go test ./... && go vet ./...`. Expected: PASS.
- [ ] **Step 5: Commit** `feat: draft promotion packet`.

### Task 5: Repo hygiene guard

**Files:**
- Create: `internal/cli/hygiene_test.go`. It reads `$HOME/code/mgmt-demo/framework-source/software-engineering.txt` if present (otherwise it skips), takes every line longer than 40 characters, and asserts that none appears in any `git ls-files` file.

- [ ] **Step 1:** Write the test.
- [ ] **Step 2:** Run it. Expected: PASS when the repo is clean. Prove it bites by temporarily adding one line to a fixture: it should FAIL. Then revert.
- [ ] **Step 3: Commit** `test: guard against company framework text in the repo`.

### Task 6: Demo workspace on the real framework (outside the repo)

**Files (all in `~/code/mgmt-demo`, not git):**
- `resources/framework.yaml`: replaced by `resources/frameworks/core.yaml` and `resources/frameworks/software-engineering.yaml`, generated from the source file by a throwaway script in `/tmp`.
  - Mapping (confirmed): positions 1, 2, 3, 5, 7, 9 and 11 are IC1 to IC7.
  - For a core row, the text after the 13th level cell is the `summary`.
  - `N/A` cells are omitted.
  - Ids and tokens:

    | Id | Token |
    | --- | --- |
    | `core.communication` | `comms` |
    | `core.growth` | `growth` |
    | `core.collaboration` | `collab` |
    | `core.stakeholders` | `stakeholders` |
    | `core.ownership` | `ownership` |
    | `core.problem-solving` | `problems` |
    | `eng.security` | `security` |
    | `eng.operations` | `ops` |
    | `eng.design` | `design` |
    | `eng.code` | `code` |

  - Normalise the paste's typos only where the meaning is obvious: missing spaces such as "anticipatesobjections" become "anticipates objections". Leave everything else verbatim.
- `resources/people.yaml`: all five people move to `framework: software-engineering`. Levels: sam IC3, priya IC5, jo IC1, marcus IC4, lena IC2. Remove `role`. Give priya a `levelSince`.
- `resources/cycles/*.yaml`: drop `framework`, and switch the selectors to the new shape.
- `notes/*.md`:
  - Re-tokenise the old toy tokens to the new ones: `planning`→`problems` or `ownership`; `quality`→`code`; `feedback`→`comms`; `crossteam`→`collab`; `mentoring`→`growth`. Pick per line so the sentence still makes sense.
  - Add a handful of `++` (several for priya, so her promotion packet is rich) and two `--` (one for marcus, one for sam).
  - `mgmt lint` must give 0 errors.
- `policy/team/promotion.rego`: delete it, and copy the scaffold's `policy/example/example.rego` into place.
- `README.md`: the framework is the real company framework and must not be published; the people and notes stay fictional; the cast table and "things to try" now include `mgmt promo priya`, `++`/`--` and "To raise".

- [ ] **Step 1:** Generate the frameworks. Spot-check that every IC1 and IC7 descriptor in the YAML equals the source cell at positions 1 and 11.
- [ ] **Step 2:** Migrate people, cycles, notes and policy.
- [ ] **Step 3:** Run every command in the demo README with `--now 2026-10-21`. Expected: 0 errors, and the outputs make sense.
- [ ] **Step 4:** No commit (not a repo). Report the demo's `lint`, `prep sam`, `promo priya` and `status` output.
