---
name: testing-transformation-census
description: How to verify the SysML v1 to v2 transformation census gate (tools/census/transformation) — provisioning the pinned OMG mapping model, checking the census document and its citations against the committed baseline, and the mutations the gate must catch.
---

# Testing the transformation census (`tools/census/transformation`)

The census (`docs/project/sysml-v1-transformation-census.md`) answers "which of the OMG SysML v1
to v2 transformation model's 783 mapping classes does OpenSysML's migrator carry out?". Its
denominator is extracted from the pinned XMI (`SysMLv1Tov2.xmi`, OMG document `ptc/25-04-10`) and
committed as `docs/project/sysml-v1-transformation-census-baseline.json` together with each
mapping's extracted fields (`package`, `qualified`, `abstract`, `from`, `to`, `generals`,
`operations`, `ocl` digest) and its census verdict (`status`, `implementation`, `tests`, `reason`,
`scope`).

## Prerequisites

- Go on PATH. Nothing else for `-check` without the model: it reads only committed files.
- The model comparison needs `./scripts/download-sysml-v1tov2.sh`, which writes
  `build/sysml-v1tov2/SysMLv1Tov2.xmi`. `-check` compares against it when present and skips
  (saying so) when absent; `-check -require-xmi` fails when it is absent.
- The scope-token measurement reads the PSSM suite (`build/pssm/PSSM_TestSuite.xmi`, from
  `./scripts/download-pssm-suite.sh`) and the migration fixtures
  (`tests/migrate/testdata/xmi/*.xmi` plus `conformance/fixtures/vehicle.xmi`). `-check` recomputes
  the counts when the suite is present and skips the comparison when absent unless
  `OPENSYSML_REQUIRE_PSSM_SUITE=1`; `-measure` fails outright without it. CI downloads the model
  and sets `OPENSYSML_REQUIRE_SYSML_V1TOV2=1`.

## The core checks

```bash
go run -C tools ./cmd/transformation-census -check            # baseline ↔ document ↔ cites ↔ measurement (↔ model if present)
go run -C tools ./cmd/transformation-census -check -require-xmi
go test -C tools -count=1 ./census/transformation           # the same gate plus extraction and mutation tests
```

Reproduction of the baseline: `-update` re-extracts the rows and keeps every recorded verdict,
so on a current tree it must leave the file byte-identical
(`git diff --exit-code docs/project/sysml-v1-transformation-census-baseline.json`).
`go run -C tools ./cmd/transformation-census` (no flag) rewrites the generated blocks of the
census document between the `<!-- census:begin … -->` / `<!-- census:end … -->` markers and must
likewise be a no-op on a current tree. `-measure` recomputes only the scope tokens the rows use.

## What the document generates

Six blocks between `<!-- census:begin … -->` / `<!-- census:end … -->` markers:
`source` (provenance line with the OCL split: 1,084 bodyCondition bodies of 1,088 OCL2.0
specifications — the rest are recorded postconditions and owned rules), `summary` (totals plus a
per-package status table), `rows` (one table per package), `beyond` (migrator behaviours with no
OMG mapping class, from the baseline's `beyond` entries — each needs ≥1 implementation and ≥1 test
cite, resolved like a row's), `errata` (rows whose reason carries `candidate erratum:`, sorted by
package then name), and `gaps` (approximate/not-implemented rows **grouped by shared status,
scope and reason** — dozens of sub-mappings carry one scope — ranked by total measured count,
ties by first mapping name).

Two CI env nuances: `-check` consults `OPENSYSML_REQUIRE_PSSM_SUITE` only for the measurement
comparison (a job that provisions neither input sets it to `""` at step level so an
`OPENSYSML_REQUIRE_PSSM_SUITE=1` ambient env cannot fail it), and `OPENSYSML_REQUIRE_SYSML_V1TOV2`
is consulted only by the Go test gate — `-require-xmi` is the command-level equivalent.

## Mutations the gate must catch

Each of these must make `-check` exit non-zero with a message naming the drift; restore the file
afterwards (`git checkout -- <file>` on a clean tree, or keep a copy).

- Delete one `mappings` entry from the baseline: with the model present, the name is reported as
  "in the model but not in the baseline"; and the document is reported stale.
- Insert a table row for a name the baseline lacks (inside the `rows` block): "`<name>` is in the
  census table but not in …baseline.json".
- Delete a table row: "`<name>` is in …baseline.json but has no census row".
- Change a digit of the `**Census:**` line, or the source line's counts or digest: "a generated
  block is stale".
- Edit a baseline row's extracted field (`from`, `to`, `ocl`, a general or operation): "extracted
  fields drifted" when the model is provisioned.
- Point a row's `implementation` at a file that does not exist, or a function/`Type.method` it does
  not declare: "the citations do not resolve". Same for a `tests` cite that is not a
  `func TestX(t *testing.T)` in a `_test.go` file.
- Give a `faithful`/`approximate`/`known-failure` row an empty `implementation` or `tests`, or a
  `not-implemented`/`approximate` row an empty `scope`, or any non-`faithful` row an empty
  `reason`: the baseline is rejected before the document or the model is read.
- Use a scope token that is not `uml:<Name>` or `sysml:<Name>`, or one `-measure` has not counted.
- Edit a recorded count under `measurement.counts` (with the PSSM suite present): "the recorded
  measurement is stale".
- Edit `source.document`/`url`/`digest` away from `scripts/sysml-v1tov2-pin.sh`: the source block
  is rejected before the model is read.
- Edit a `source` count (`packages`, `classes`, `mappings`, `oclBodies`, `oclSpecifications`):
  with the model present, the recorded dimensions are reported stale.
- Hand-edit the `beyond` table or give a beyond entry a cite that does not resolve: the stale
  block or the unresolved cite is reported.
- Hand-edit the `errata` or `gaps` table (a note, a group's count or figure): "a generated block
  is stale".

## Adjudicating a status change

Verdicts are edited by hand in the baseline (the extracted fields are not). Moving a row to
✅/⚠️/🚧 needs an implementation cite that resolves (`<file>.go:<Func>` or `<file>.go:<Type.method>`)
and a test cite that resolves (`<file>_test.go:<TestX>`); a ❌/⚠️ row needs at least one
`uml:<Name>`/`sysml:<Name>` scope token, after which `-measure` must re-run so the token is counted
and the gaps table re-orders, then `go run -C tools ./cmd/transformation-census` rewrites the
document — `-check` fails until all three agree.
