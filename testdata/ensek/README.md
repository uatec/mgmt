# ensek fixture

The `team` fixture (same notes and resources; see `../team/README.md`) plus
the spec's worked example of a company extension, with no Go changes:

- `schema/ensek/okr.cue` registers `kind: OKRSet`.
- `resources/okrs.yaml`: OKRSet `sam-2026-h2` for sam in `ensek-2026-h2`.
  Objective 1 links `scope.crossteam` (unproven for sam), objective 2 links
  `delivery.ownership` (proven); objective 3 omits `keyResults` and
  `competencies` (both optional), so `report okrs` shows its `none` lines.
- `policy/ensek/okr.rego` (`package mgmt.policy.ensek`) warns
  `okr-no-evidence` at the OKRSet's source for each linked competency that is
  unproven while `now` is within 14 days before `manager-draft` (due
  2026-11-14, so 2026-10-31..2026-11-14). `okr_test.rego` is its unit test.
- `reports/prep.tmpl` overrides the core prep sheet (first line
  `# Ensek 1:1 prep`); `reports/okrs.tmpl` is an extra report run with
  `mgmt report okrs [<person>]`.

Goldens: `lint` at 2026-10-21 (no `okr-no-evidence`) and 2026-11-01 (one,
for `scope.crossteam`), `prep sam`, `report okrs` (all, sam, and priya with
none) and the report error paths.
