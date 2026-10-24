---
name: testing-validation-census
description: How to verify the validation-constraint census gate (tools/census/validation) — re-extracting the pilot's constraint names from the pinned jar, checking the census document and its probes against the committed constraint manifest, and the mutations the gate must catch.
---

# Testing the validation-constraint census (`tools/census/validation`)

The census (`docs/project/validation-constraints.md`) answers "which of the pilot's named
validation constraints does OpenSysML report?". Its denominator is read from the pinned jar's
two validator classes and committed as `docs/project/constraint-manifest.json` together with
each name's row (metaclass, spec clause, implementation, diagnostic codes, negative case, status,
rationale); the page's table is rendered from it by `doc-counts` and by `validation-census`. The
evidence for a ✅/⚠️ `validate*` row is a violating model under `tools/census/validation/testdata/probes/`;
a ✅/⚠️ `check*` or `derive*` row (an implied specialization or a derived property, which reports no
diagnostic) is held to the declared function its `implementation` cites, with its unit test beside it.
The manifest also carries the spec-sourced rows (`"source": "spec"`): the `check*`/`derive*`/`validate*`
constraints the pinned formal documents state and the pilot does not name, read from the PDFs by
`tools/census/formal`. They follow the pilot rows, are outside the census table and counts, and are
tallied by the `validation-spec-coverage` block of the page instead.

## Prerequisites

- Go on PATH. Nothing else for `-check` without the jar: it reads only committed files.
- The jar comparison needs `./scripts/download-pilot-validator.sh` (Java 21, Maven; minutes),
  which writes `build/pilot-validator/target/sysml-download/sysml/jupyter-sysml-kernel-<v>-all.jar`.
  `-check` compares against it when present and skips the comparison (saying so) when absent;
  `-check -require-jar` fails when it is absent. The scheduled oracle-reproduction workflow runs
  the latter; CircleCI and `make docs-counts` run the former.

## The core checks

```bash
go run -C tools ./cmd/validation-census -check            # manifest ↔ document ↔ probes (↔ jar if present)
go run -C tools ./cmd/doc-counts -check                   # the rendered table and the census summary in spec-compliance.md
go run -C tools ./cmd/validation-census -check -require-jar
go test -C tools -count=1 ./census/validation         # the same gate plus every probe run through the workspace
```

Reproduction of the manifest: `-update` re-extracts the names and keeps every recorded row,
so on a current tree it must leave the file byte-identical (`git diff --exit-code docs/project/constraint-manifest.json`).
`go run -C tools ./cmd/validation-census` (no flag) rewrites the `**Pilot:**`, `**Jar:**` and `**Census:**`
lines and the census table from the manifest and must likewise be a no-op on a current tree.

## Mutations the gate must catch

Each of these must make `-check` exit non-zero with a message naming the drift; restore the file
afterwards (`git checkout -- <file>` on a clean tree, or keep a copy).

- Delete one `constraints` entry from the manifest: the summary line and the table are reported
  stale and, with the jar present, the name is reported as "in the jar but not in the manifest".
- Edit a manifest row's `status` (with a `rationale`) without regenerating: both
  `validation-census -check` ("the census table is stale") and `doc-counts -check` (the page and
  spec-compliance.md stale) fail; regenerating moves exactly that row and the census lines.
- Insert, delete or edit a table row in the page (status marker, language, `✅ faithfulish`,
  a dropped rationale): "the census table is stale".
- Change a digit of the `**Census:**` line, or the release, commit, artifact, jar name or digest
  on the `**Pilot:**`/`**Jar:**` lines: "a derived line or the census table is stale".
- Blank a non-✅ row's `rationale`, or double a backtick in its `negativeCase`
  (`` ``kerml/x.kerml` ``): the manifest is rejected before anything is compared.
- Blank or misspell the manifest's `recorded` date, or change its `jar.name` away from the pinned
  artifact's filename: the manifest is rejected before any jar is read.
- Rename a diagnostic code in a pass, or edit a row's `codes`: `go test -C tools ./census/validation`
  reports the probe's code against the row's; delete an entry of `extensions`: the pass file is
  reported as neither cited nor listed.
- Point a row's *Negative case* at a file that is not under `tools/referee/reject/testdata/negative/`.
- Delete a probe of a ✅/⚠️ row, or add a probe for a ❌/❔ row.
- Delete the `.sysml` (or `.kerml`) probe of a row whose language is `both`: each notation needs one.
- Delete a spec-sourced row, or move one above the last pilot row: the manifest is rejected
  ("out of order") or, for the deletion, `doc-counts -check` reports the coverage block stale while
  the `**Census:**` line and the census table stay current — the census must not move.
- Give a pilot row an `ocl`, or a spec-sourced row a `raw` identifier, a blank `checks`, or a
  `spec.name` other than its `name`: the manifest is rejected before anything is compared.

## The formal-document gate

`go run -C tools ./cmd/formal-constraints -kerml <KerML.pdf> -sysml <SysML.pdf> -check` needs the
two pinned PDFs (the `url`s and `sha256`s are in the manifest's `specifications`) and poppler's
`pdftotext`; it refuses a PDF whose digest is not the pinned one before reading it. `-list` printed
twice must be byte-identical; `-update` on a current tree must leave the manifest byte-identical.
Mutations it must catch: edit a spec-sourced row's `checks`, `ocl`, `metaclass` or `spec.clause`
("recorded with another text, clause or metaclass"); delete one ("stated by the documents but not
recorded"); add one the documents do not state; change a pilot row's `spec.name` to a name the
documents do not state, or its `spec.clause` to another clause ("cites … the documents state …").
The package's own tests (`go test -C tools ./census/formal`) drive the same paths over a fake
`pdftotext`, so they run without the PDFs.

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

Statuses live in the manifest row, never in the page (the names are not edited at all: `-update`
re-extracts them from the jar). To move a row to ✅/⚠️:

1. Add a probe under `tools/census/validation/testdata/probes/` (one per notation for a `both`
   row) and, ideally, run the model through the pinned pilot validators
   (`build/pilot-sysml-validator/validate-sysml-batch`, `build/pilot-kerml-validator/validate-kerml`)
   to confirm the pilot reports the same constraint on it.
2. Edit the row in `docs/project/constraint-manifest.json`: `status`, `implementation`
   (`internal/<file>.go:<func>` or `<Type>.<method>`), `codes` and `passes` (the diagnostic code and
   pass the probe observes), `message`, `negativeCase`, and a `rationale` unless the status is ✅.
3. Regenerate: `go run -C tools ./cmd/validation-census` (census lines and table) and
   `make docs-counts` (the same table and the summary in spec-compliance.md).
4. Gate: `go run -C tools ./cmd/validation-census -check`, `go run -C tools ./cmd/doc-counts -check`
   and `go test -C tools -count=1 ./census/validation` — the last compares the probe's diagnostic
   with the row's `codes`/`passes` and fails until all three agree.

Never edit the generated table by hand: `-check` reports it stale and regeneration overwrites it.
