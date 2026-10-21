# warnings fixture

Fires every core warning at `--now 2026-10-21` with no errors, so
`mgmt lint --strict` exits 1. Cycle `autumn` runs 2026-07-01..2026-10-31 with
`peer-nominations` due 2026-10-27 (6 days) and `manager-draft` due 2026-11-01
(11 days).

| Location | Warning |
| --- | --- |
| notes/2026-W42.md:4 (`-planning`, 2026-10-12, no `~planning` action) | `negative-unaddressed` |
| notes/2026-W42.md:6 (open, due Fri 2026-10-16) | `action-overdue` |
| notes/2026-W42.md:7 (open, no due date) | `action-no-due` |
| notes/2026-W42.md:8 (open, no due date, `<!-- mgmt:ignore action-no-due -->`) | none: `action-no-due` suppressed |
| notes/2026-W42.md:9 and notes/2026-W43.md:6 (same open text, different case and spacing) | `action-duplicate` on both |
| notes/2026-W43.md:8 (`## Priya` section with no tokens) | `section-unmarked` |
| resources/people.yaml:1 (sam: planning disproven, crossteam unproven, no open actions) | `gap-unaddressed` |
| resources/people.yaml:24 (jo: managed since 2026-05-01, no notes at all; `# mgmt:ignore gap-unaddressed` in jo's document) | `competency-untouched`, `oneonone-overdue`; `gap-unaddressed` suppressed |
| resources/cycle.yaml:1 (priya and jo have no `!self-assessment`; sam does, at notes/2026-W43.md:5) | `cycle-input-missing` |
| resources/cycle.yaml:1 (sam and jo have gaps; priya proved ownership at notes/2026-W38.md:4) | `cycle-evidence-thin` |
