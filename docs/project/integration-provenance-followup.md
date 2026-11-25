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

### Structure differential (re-recorded here)

`TestSymbolStructureMatchesGraphStructure` sweeps both roots with no pilot tool
and no Java, so its expectation is re-recorded here with
`-update-structure-differential` and reviewed cell by cell:

- `examples`: `models 69` unchanged, `matched 54061 → 54071`. The self-model now
  declares one more constraint pass (`passCount 66 → 67` and a
  `StrictRedefinitionPass` part), so the graph and symbol structures compare ten
  more elements. All 63 `equal` movements are increases; the only disagreement
  movements are one `different` on `Feature::type` and `symbol-extra` on
  `Feature::isConstant` (+4) and `Usage::mayTimeVary` (+2). No property was
  added or removed.
- `tests/testdata`: `models 42 → 44`, `matched 2631 → 2747`. The two new fixtures
  account for every disagreement movement: their isolated differential counts,
  summed, reproduce all 13 changed disagreement bucket lines (seven `different`
  and six `symbol-extra` across ten properties) exactly. The other 56 moved
  buckets are new agreeing comparisons. No property was added or removed and no
  `symbol-missing` bucket moved.

The movement is a larger compared input, not a change in how the two structures
agree on existing models: no disagreement count fell, and no new disagreeing
property appeared. The gate passes with the re-recorded expectation; the pilot
baselines below remain blocked. After the later reviewer fixes were ported into
this branch, the same test reproduced the re-recorded figures exactly, so the
expectation needed no second re-record.

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
