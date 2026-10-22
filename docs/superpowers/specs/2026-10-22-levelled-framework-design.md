# Levelled competency framework, `++` / `--` marks and draft promotion packet

Oct 22, 2026 · design for mgmt (follows the v1 spec `2026-10-21-mgmt-design.md`)

## Why

Robert's real framework, a career-progression framework for engineering, is a **matrix**. Each competency (Communication, Ownership, Code, …) exists at every level, with a **different descriptor per level**: IC1 communication is about keeping your own team informed, and IC7 communication is about shaping how a whole department communicates.

mgmt v1 models a framework the other way round. Each role-level has a list of distinct competencies it `expects`, with `inherits` and `removes` between levels. That forces either one token per (competency, level), which is unusable in a meeting, or one token for every level, which loses the level.

The note-taking workflow:
1. In a meeting, Robert sees something and writes a single token: `+comms`.
2. Later he reads the person's review pack with all the evidence.
3. If something was clearly above their level, it should flow into a promotion case without him having to dig for it.

## Goals

- One token per competency (`+comms`). The level is never written in notes. Evidence is judged against the descriptor for the **person's current level**.
- Two strong marks that are still a single token:
  - `++comms`: clearly at the **next** level up.
  - `--comms`: clearly and badly below the bar. This is more than a missed opportunity.
- `++` and `--` since the last 1:1 stand out in `prep`, so Robert can praise or address them in the 1:1.
- A **draft promotion packet** built automatically from `++` evidence, shown against the next level's descriptors.
- A framework schema that matches the real framework's shape: a competency × level matrix. Core competencies are shared by several disciplines, and each discipline adds its own functional competencies.
- Robert's Software Engineering framework (IC1–IC7) entered as YAML from the text he pasted. Robert has no spreadsheet or export, so the builder transcribes it.

## Non-goals

- **Manager (M) and Exec tracks.** Only the IC track is modelled. M/Exec descriptors and N/A cells are not entered.
- **Sideways moves** (for example IC → M). The promotion ladder is linear: IC5 → IC6.
- **HR processes.** `--` does not feed an HR case. If Robert takes something to HR, he does that outside the tool.
- **What a "real" promotion packet must contain.** Robert has no company template, so the packet is a sensible draft for him to edit.
- **Importing a framework from a website or spreadsheet.**
- **The QA and Platform disciplines.** Their functional rows aren't available yet. The schema must make adding them later a matter of adding one file that reuses core.

## Anti-goals (failure even if it technically works)

- Note-taking gets slower or needs more syntax than one token per mark.
- The tool rates people or declares promotion readiness. The packet collects evidence; Robert judges.
- **Company framework text lands in the public tool repo.** `github.com/uatec/mgmt` is public. The real descriptors belong in Robert's workspace, which is not in the tool repo. The tool repo's scaffold, fixtures and README use made-up or generic descriptors only.

## Decided

### Framework model

- A framework has an **ordered level ladder**: `[IC1, IC2, IC3, IC4, IC5, IC6, IC7]`.
- Each competency has:
  - an `id` (prefix = theme, as in v1: `core.communication`, `eng.code`);
  - a `title`;
  - `tokens`;
  - an optional `summary` (the framework's "default description" or values text);
  - `levels`, a map of level → descriptor.
- A competency is **required** for a person when it has a descriptor at the person's level. A missing level means the competency doesn't exist there (the framework's N/A), so it is not required.
- Core competencies are shared by including them, not copying them. A discipline framework (for example `software-engineering`) **includes** a core framework (for example `core`). The person's effective competencies are the discipline's own plus everything included. Ladders must match across includes; a mismatch is a validation error.
- `inherits`, `expects`, `removes` and `roles` are removed. A Person has `framework` (the discipline framework) and `level` (an IC level on that ladder). A Person no longer has a `role`.
- **This replaces the v1 framework schema outright.** There is no compatibility layer: it's `v1alpha1` with one user. Everything in the tool repo is migrated: the scaffold, `testdata` fixtures, README and demo workspace.
- `ReviewCycle.spec.selector.matchRoles` is replaced by `matchFrameworks` (a list of framework names). `people` is unchanged.

Illustrative shape. The builder owns exact field names, as long as the `levels` map and `includes` concepts survive:

```yaml
kind: CompetencyFramework
metadata: {name: core}
spec:
  levels: [IC1, IC2, IC3, IC4, IC5, IC6, IC7]
  competencies:
    - id: core.communication
      title: Communication
      tokens: [comms, communication]
      summary: <default description>
      levels:
        IC1: <descriptor>
        IC2: <descriptor>
        # ... IC7
---
kind: CompetencyFramework
metadata: {name: software-engineering}
spec:
  includes: [core]
  levels: [IC1, IC2, IC3, IC4, IC5, IC6, IC7]
  competencies:
    - id: eng.security
      title: Security and Compliance
      tokens: [security, sec]
      levels: {IC1: <descriptor>, ...}
```

### Marks

| Token | Meaning | Status effect |
| --- | --- | --- |
| `+x` | meets the bar at their level | positive (as v1) |
| `-x` | misses the bar at their level | negative (as v1) |
| `++x` | clearly at the next level | positive; also promotion evidence |
| `--x` | clearly and badly below the bar | negative; flagged as a concern |
| `~x` | discussed (unchanged) | none |

- Grammar is the v1 rule extended: the sigil (`+`, `-`, `++` or `--`) is preceded by start of line, whitespace or `(`, and followed by a letter. So `C++`, `--verbose` written after a word, `+1` and hyphenated words remain non-tokens. `-- text` (a dash, a dash, a space) is not a token.
- `++` and `--` are not allowed on action lines, the same as `+` and `-` (`evidence-in-action`).
- Status rules from v1 are unchanged. A `++` counts as one positive mark and a `--` as one negative. Strength does not change proven, disproven or unproven.
- `++` for a person at the top of the ladder (IC7) has no next level. It still counts as positive, and lint warns (new warning, suppressible) that there is no next level. `--` at IC1 is just a strong negative.

### Where it shows up

- **`prep`:** a new section near the top, "To raise", listing `++` (praise) and `--` (concern) marks dated since the last 1:1, newest first, with `file:line`. They still appear in "Since last 1:1" too. They are tagged `++`/`--` there, not shown as `+`/`-`.
- **`review`:** each competency shows the descriptor for the person's level (the bar they're judged against). `++`/`--` evidence is visibly tagged.
- **`explain`:** shows the level descriptor and tags `++`/`--`.
- **`status`:** the matrix is unchanged in meaning. Columns are the competencies required for at least one listed person.
- **Promotion packet**, as a new command (`mgmt promo <person>`; the builder may pick the name). For the person's next level, per competency required at that level:
  - the current-level descriptor and the next-level descriptor;
  - the `++` evidence, newest first, with dates and `file:line`;
  - a closing list of next-level competencies with **no** `++` evidence yet: the gaps in the promotion case.

  The packet never says "ready" or "not ready". At the top of the ladder it says there is no next level.
- **Packet evidence window:** promotion evidence spans more than one review cycle. The packet uses all `++` evidence since the person reached their current level. That date is a new optional `Person.spec.levelSince`, defaulting to `since`. Evidence from before a promotion was judged against the old next level, so it must not count.
- **Warnings:** `negative-unaddressed` covers `--` as well as `-`. No other warning changes.

### Robert's framework data

- The builder transcribes the pasted Software Engineering framework (core plus functional, IC track only) into YAML **in Robert's workspace**, not in the tool repo. For now that's the demo at `~/code/mgmt-demo/resources/`, replacing its toy framework. The demo's people move to IC levels and the `software-engineering` framework. Their notes are re-tokenised to the new tokens where needed, and a few `++`/`--` are added so prep, review and the packet have something to show.
- **Column mapping, as read from the paste.** The paste interleaves IC and M columns: `IC1, IC2, IC3, M3, IC4, M4, IC5, M5, IC6, M6, IC7, M7, M8, Exec`.
  - **Core rows:** 13 level descriptors (IC1 to M8; Exec blank), followed by the default description. Confirmed by Robert, 2026-10-22.
  - **Functional rows:** 14 descriptors (IC1 to Exec) with N/A cells and no default description.
  - **IC levels** are positions 1, 2, 3, 5, 7, 9 and 11 in both cases.
  - **Check:** the Communication descriptor Robert remembered as "level 7" sits in the M6 column by position. The mapping is confirmed: IC7 is the cell at position 11.
- **Tokens** (confirmed by Robert, 2026-10-22):

  | Competency | Tokens |
  | --- | --- |
  | Communication | `comms` |
  | Growth Mindset | `growth` |
  | Collaboration | `collab` |
  | Stakeholder Management | `stakeholders` |
  | Ownership | `ownership` |
  | Problem solving | `problems` |
  | Security and Compliance | `security` |
  | Operations | `ops` |
  | Design | `design` |
  | Code | `code` |

  Ids use the `core.` and `eng.` prefixes as themes.

## Builder-check answers (2026-10-22)

- **Source text.** Robert's paste is saved privately at `~/code/mgmt-demo/framework-source/software-engineering.txt`. It is outside every git repo. Transcribe from that file. Never copy it into the tool repo.
- **`levelSince` applies everywhere** *(builder's call)*. Every evidence window starts at the latest of the cycle or rolling start, `since` and `levelSince`, not just the promotion packet's. After a promotion, the person is judged only against the new bar. This settles the v1 open question about level changes inside a window.
- **Where the real framework lives** *(builder's call)*. The demo at `~/code/mgmt-demo` is local and not in git. It gets the real Software Engineering framework, while its people and notes stay fictional. Update its README to say so, and add a warning not to publish the folder. All five demo people use `software-engineering`, including the two SREs: Platform's functional rows don't exist yet.
- **Demo promotion policy** *(builder's call)*. Delete `policy/team/promotion.rego`: the promotion packet replaces it, and it leans towards "readiness". Replace it with the scaffold's `example-me-backlog` policy, so the demo still shows a custom policy.
- **Cycle selection** *(builder's call)*. `matchFrameworks` matches a person whose framework is listed or includes a listed framework, so `[core]` selects everyone. `ReviewCycle.spec.framework` is removed.
- **Tool-repo hygiene.** The scaffold, fixtures, goldens and README use generic, invented descriptors. A test asserts that no tracked file contains a distinctive phrase from the private source file. It is skipped when the file doesn't exist.

## Left to the builder

- Exact YAML field names; the facts-document representation of strength (for example a `strength` field or polarity ±2), as long as the facts stay a documented contract and the existing Rego and policies are updated consistently.
- How `includes` resolves (CUE or Go) and the error codes for bad includes or ladder mismatches (`ref-unknown` or `schema-invalid` are fine).
- The new warning's id and wording; the packet command name and template layout (markdown, following the existing report style).
- Generic, made-up descriptors for the scaffold, fixtures and README.
- Migration order and test strategy, within the v1 constraints: deterministic, read-only, no network, `file:line` on every printed evidence item, lint < 1 s on 52 files × 10 people.
