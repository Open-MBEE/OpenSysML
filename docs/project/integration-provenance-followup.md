# Combined differential-provenance follow-up

Two merged changes each add a regression fixture to `tests/testdata`: the strict
redefinition case (`tests/testdata/passes/strict_redefinition_resolves_to_nothing.sysml`)
and the bound-subject case (`tests/testdata/bound-subject/verification_subjects.sysml`).
`tests/testdata` is an input root of the advisory differential baselines, whose
committed provenance still records the pre-merge tree.

| Baseline | Committed `testdata` provenance | Merged tree |
|---|---|---|
| `docs/project/pilot-differential-baseline.json` | `files: 42`, digest of the old root | two new files, both digests moved |
| `docs/project/api-json-differential/index.json` | `files: 42`, digest of the old root | two new files, both digests moved |
| `docs/project/grammar-coverage-baseline.json` | `files: 40`, `lines: 623` for the same root | the input root grew; its figures need re-extraction |

Each source change recorded its own follow-up when one fixture was added:
[the strict redefinition record](strict-redefinition-validation.md) and
[the bound-subject record](case-bound-subject-query.md). Together they take the
recorded `testdata` count from 42 to 44, not 43; neither branch's figure applies
to the merged tree. `TestCommittedBaselineStatesThisRepositorysProvenance` in
`tools/referee/diff` and `tools/referee/apijson`, and the grammar-coverage
provenance guard, therefore fail on provenance until the baselines are
re-recorded. No baseline was edited by hand here.

Re-recording needs the pinned pilot validators and JSON exporter, which need a
Java 21 JDK; this environment has no JDK 21 and the pinned tools were not
available, so neither recorder was run and no provenance gate is claimed to
pass. A maintainer with the toolchain must provision it with
`./scripts/download-pilot-sysml-validator.sh`,
`./scripts/download-pilot-kerml-validator.sh` and
`./scripts/download-pilot-json-exporter.sh`, then run
`go run -C tools ./cmd/pilot-diff -update`,
`go run -C tools ./cmd/pilot-apijson-diff -update` and the grammar-coverage
recorder, adjudicate every moved bucket in
[the pilot differential record](pilot-differential.md) and
[the api-json differential record](api-json-differential.md), and re-run the
provenance tests and `make docs-counts`. A merged baseline is a measurement only
once its `-check` reproduces it. Both new models are clean in the default mode,
so no compared-root verdict is expected to move, but that must be confirmed by
the re-record rather than assumed.
