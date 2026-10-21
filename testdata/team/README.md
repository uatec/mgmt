# team fixture

A small team with weekly notes from 2026-W30 to 2026-W43, used for the report
goldens (`prep`, and later `review`, `status`, `explain`) at `--now 2026-10-21`.
`mgmt lint` gives 0 errors and only warnings.

Framework `eng`: the spec's three competencies plus `collab.feedback`, which
L2 expects alongside `delivery.ownership`. L3 inherits L2 and adds
`collab.planning` and `scope.crossteam`.

Cycles:

- `ensek-2026-h2` (the spec's cycle): period 2026-04-01..2026-09-30, active on
  2026-10-21 through its milestones; next milestone `peer-nominations` due
  2026-10-27. Selects sam, priya and jo.
- `retro-2026-h1`: period 2026-01-01..2026-03-31, no milestones, one required
  input `self-assessment`; selects sam and priya; not active on 2026-10-21 and
  has no notes in its window.

Notes: sam's 1:1s are in even weeks W30–W42, priya's in odd weeks W31–W43.
`notes/2026-W40.md` and `notes/2026-W42.md` end with a `## 1:1 Alex` section.
`notes/2026-W43.md` also has a `## Platform sync (Tue 20 Oct)` section dated
2026-10-20 with `@sam` and `@priya` lines.

## sam (L3, since 2026-01-05, cadence 7d)

Window 2026-04-01..2026-09-30 (ensek-2026-h2).

| Competency | Status | Why |
| --- | --- | --- |
| delivery.ownership | proven | +W30:4, +W36:5; W43:8 is outside the window |
| collab.planning | disproven | -W32:4, +W34:4, -W38:4 (decides); +W42:4 is outside the window |
| scope.crossteam | unproven | touch only (~W32:5, 2026-08-03) |
| collab.feedback | proven | +W32:6, +W40:4 |

- Open actions, mine: W42:5 `send him the L4 section` due 2026-10-20 (overdue),
  W42:6 `book a planning pairing session` due 2026-10-30, addressing
  collab.planning.
- Open actions, theirs: W43:10 `draft the CDN provisioning plan` due
  2026-10-23, W38:5 `tidy the runbook` with no due date.
- Closed actions: W30:6 (mine), W34:5 (theirs).
- Last 1:1 section before now: W42 (2026-10-12). Since then: W43:8
  (+ownership) and W43:9 (~planning), both 2026-10-20.
- Gaps: collab.planning disproven, addressed by W42:6; scope.crossteam
  unproven, not addressed.
- Suggested focus: scope.crossteam, last touched 2026-08-03.
- Inputs for ensek-2026-h2: self-assessment (W42:7), peer-feedback 1 of 3
  (W43:11), okrs missing.

## priya (L2, since 2026-01-05, cadence 14d)

Everything required is proven: delivery.ownership (+W31:4, +W37:4) and
collab.feedback (+W33:4, +W39:4). W43:4 (+ownership) is outside the window.

- Open actions: mine W43:5 due 2026-10-23; theirs W41:7 due 2026-11-06
  (addresses delivery.ownership). Closed: W31:5, W41:6.
- Last 1:1 section before now: W43 (2026-10-19). Since then: W43:12
  (+crossteam, 2026-10-20; crossteam is not required at L2).
- Suggested focus: collab.feedback, last touched 2026-09-21.
- Inputs: okrs (W35:4), self-assessment (W39:5), peer-feedback 3 of 3 (W39:6,
  W41:4, W41:5); nothing missing.

## jo (L3, since 2026-05-01)

No notes at all: no actions, no previous 1:1, every required competency
unproven and not addressed, focus on `collab.feedback` (first required id,
never touched), and every ensek-2026-h2 input missing.

## alex (L2, since 2026-09-01, cadence 14d, in no cycle)

No ReviewCycle selects alex, so the rolling window applies (Decision L):
2026-09-01 (since) to 2026-10-21. `review` is titled `last 90d` and has no
inputs section; `prep` shows `(rolling)` and no Cycle section. alex adds no
lint warnings.

- delivery.ownership proven (+W40:8, 2026-09-28).
- collab.feedback proven: -W40:9 then +W42:10 (latest positive decides).
- Actions: closed W42:11 (mine, addresses collab.feedback); open W42:12
  (theirs) due 2026-10-23.
- Last 1:1 section: W42 (2026-10-12), nothing since. No gaps. Focus:
  delivery.ownership, last touched 2026-09-28.

## review actions (Ruling R14, amended)

The review pack lists actions dated from the window start to the cycle's
active end (the later of period.to and the last milestone), or to the window
end without a cycle. For ensek-2026-h2 that runs to 2026-11-28, so sam's W42:6
and priya's W41:7 show. For retro-2026-h1 (no milestones) it ends 2026-03-31,
so `review sam --cycle retro-2026-h1` shows no actions.

Note text avoids the words rating, below, meets and exceeds (Ruling R5).
