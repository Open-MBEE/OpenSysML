---
name: testing-validation-census
description: How to verify the validation-constraint census gate (cmd/validation-census) — re-extracting the pilot's constraint names from the pinned jar, checking the census document and its probes against the committed baseline, and the mutations the gate must catch.
---

# Testing the validation-constraint census (`cmd/validation-census`)

The census (`docs/project/validation-constraints.md`) answers "which of the pilot's named
validation constraints does OpenSysML report?". Its denominator is read from the pinned jar's
two validator classes and committed as `docs/project/validation-constraints-baseline.json`
together with each name's census status; the evidence for every ✅/⚠️ row is a violating model
under `cmd/validation-census/testdata/probes/`.

## Prerequisites

- Go on PATH. Nothing else for `-check` without the jar: it reads only committed files.
- The jar comparison needs `./scripts/download-pilot-validator.sh` (Java 21, Maven; minutes),
  which writes `build/pilot-validator/target/sysml-download/sysml/jupyter-sysml-kernel-<v>-all.jar`.
  `-check` compares against it when present and skips the comparison (saying so) when absent;
  `-check -require-jar` fails when it is absent. The scheduled oracle-reproduction workflow runs
  the latter; CircleCI and `make docs-counts` run the former.

## The core checks

```bash
go run ./cmd/validation-census -check            # baseline ↔ document ↔ probes (↔ jar if present)
go run ./cmd/validation-census -check -require-jar
go test -count=1 ./cmd/validation-census         # the same gate plus every probe run through the workspace
```

Reproduction of the baseline: `-update` re-extracts the names and keeps every recorded status,
so on a current tree it must leave the file byte-identical (`git diff --exit-code docs/project/validation-constraints-baseline.json`).
`go run ./cmd/validation-census` (no flag) rewrites the `**Pilot:**`, `**Jar:**` and `**Census:**`
lines from the baseline and must likewise be a no-op on a current tree.

## Mutations the gate must catch

Each of these must make `-check` exit non-zero with a message naming the drift; restore the file
afterwards (`git checkout -- <file>` on a clean tree, or keep a copy).

- Delete one `constraints` entry from the baseline: the summary line is reported stale and, with
  the jar present, the name is reported as "in the jar but not in the baseline".
- Insert a table row for a name the baseline lacks (inside the table, not after its blank line):
  "`<name>` is in the table but not in docs/project/validation-constraints-baseline.json".
- Delete a table row: "`<name>` is in the baseline but has no row".
- Change a digit of the `**Census:**` line, or the release, commit, artifact, jar name or digest
  on the `**Pilot:**`/`**Jar:**` lines: "a derived line is stale".
- Change a row's status marker or language, or append anything to a status cell (`✅ faithfulish`):
  the row is reported as disagreeing with the baseline.
- Drop or double a backtick around a constraint name or a negative-case path (`` ``validateX` ``):
  the cell is rejected as not a backticked name/path, not read as the normalized name.
- Blank or misspell the baseline's `recorded` date, or change its `jar.name` away from the pinned
  artifact's filename: the baseline is rejected before any jar is read.
- Point a row's *Negative case* at a file that is not under `tools/referee/reject/testdata/negative/`.
- Delete a probe of a ✅/⚠️ row, or add a probe for a ❌/❔ row.
- Delete the `.sysml` (or `.kerml`) probe of a row whose language is `both`: each notation needs one.

## Reading a probe

```text
// Census: validateAssociationEndTypes
// Expect: error: An association end must have exactly one type
```

The header names the row and the severity plus a fragment of the message OpenSysML must report;
`TestProbesReportTheirConstraint` opens the model with `model.NewWorkspace` and fails if no
diagnostic matches. A probe is evidence for the *mapping*, not a corpus case: it is not run by
`tools/referee/reject`, and adding one does not change any oracle baseline.

## Adjudicating a status change

Statuses are edited by hand in the baseline (the names are not). Moving a row to ✅/⚠️ needs a
probe that passes the test above and, ideally, a run through the pinned pilot validators
(`build/pilot-sysml-validator/validate-sysml-batch`, `build/pilot-kerml-validator/validate-kerml`)
confirming the pilot reports the same constraint on that model. Then rerun
`go run ./cmd/validation-census` so the summary line follows, and edit the row's status cell to
match — `-check` fails until all three agree.
