# Pilot Corpora Gate

## Overview

**Corpora:** the three pinned OMG pilot corpora, fetched by `./scripts/download-pilot-corpora.sh`
at release `2026-08`, commit `692170b71867353b8f90341e61556f49a5beb0e5` (`scripts/pilot-pin.sh`)

| Root | Directory | Files |
|---|---|---|
| `sysml-examples` | `examples/pilot-corpora/sysml-examples` | 99 `.sysml` |
| `sysml-validation` | `examples/pilot-corpora/sysml-validation` | 56 `.sysml` |
| `kerml-examples` | `examples/pilot-corpora/kerml-examples` | 58 `.kerml` |

**Gate:** `TestPilotCorporaDiagnostics` in `internal/core/model/pilot_corpora_test.go` records
every file's diagnostic count in `internal/core/model/testdata/pilot_corpora_expected.txt`, so a
count going up, a count going down, a file that becomes clean and a file that starts reporting all
fail the test
**Regenerate:** `go test ./internal/core/model -run TestPilotCorporaDiagnostics -update-pilot-corpora`
**Required in CI:** `OPENSYSML_REQUIRE_PILOT_CORPORA=1` in both `.circleci/config.yml` and
`.github/workflows/pr.yml`, under which an absent or empty corpus fails instead of skipping

## One mechanism, two policies

All four OMG model roots — these three plus the training corpus in
`examples/sysml-v2-training` — come from the same pinned pilot release and share one gate
mechanism in `internal/core/model/corpus_gate_test.go`: one walker, one whole-root loader, one
`GATE NOT RUN` skip banner, one expectation-file format, one cache-independence test
(`TestCorpusGatesCacheStateIndependent`, which covers all four roots), and one downloader
(`pilot_fetch_subtrees` in `scripts/pilot-pin.sh`, called with one entry by
`download-training-examples.sh` and three by `download-pilot-corpora.sh`). The downloader clones
the release tag and refuses it unless it resolves to the pinned commit, and stamps each root with
the tag, commit and repository it came from (`.pilot-pin`); a root stamped with another pin, or
not stamped at all, is re-fetched on the next run. The ratchet's header records the file count of
each root, so a root whose count differs from the header is a provisioning question — a stale or
partial copy — before it is a behaviour question; see [pilot-differential.md](pilot-differential.md).

The two *policies* over that mechanism deliberately differ:

- **Training asserts.** The reference validates all 100 files of `sysml/src/training` clean and so
  do we, so `TestTrainingExamplesSemanticErrors` asserts that: `training_examples_expected.txt`
  records the corpus size and nothing else, and a reporting file fails the gate instead of being
  recorded. `-update-training` refuses to write a per-file count, so the assertion cannot be
  ratcheted into a baseline by a future PR with a plausible-sounding justification.
- **The other three ratchet.** They are not clean under our implementation (109, 10 and 72 files'
  worth of diagnostics the reference does not report, per `cmd/pilot-diff`), so there is nothing to
  assert yet; the per-file counts are pinned instead, and every movement in either direction has to
  be adjudicated.

Do not "simplify" the training gate into the ratchet: an absolute gate that can be re-baselined is
not an absolute gate. The scopes also differ on purpose — training counts `SeverityError` over
`.sysml` only, the corpora count every severity over `.sysml` and `.kerml`. (Measured, not assumed:
the training corpus is clean on *every* severity too, so its narrower scope costs nothing today; it
is kept because a future warning there is a warning, not a broken gate.)

The two require-env vars stay separate (`OPENSYSML_REQUIRE_TRAINING_CORPUS`,
`OPENSYSML_REQUIRE_PILOT_CORPORA`), because the two corpora are fetched, cached and run
independently in both CI configurations: one absent corpus must fail its own gate, not silently
un-gate the other.

## What the gate does not do

- **It is not a conformance claim.** The counts are *our* verdicts on those 212 files, recorded as
  measured. Many of them are diagnostics the reference implementation does not report; the starting
  baseline is where the implementation actually is, not where it should be.
- **It is not a comparison against the reference implementation.** That is
  [pilot-differential.md](pilot-differential.md) (`go run ./cmd/pilot-diff`), which needs the pinned
  Java validators, is advisory, and is deliberately not wired into CI. This gate needs no validator:
  it is pure Go and runs in seconds, which is why it can gate every PR.
- **It does not adjudicate.** Like the [training-examples](training-examples.md) gate, the
  expectation file is a snapshot, so `-update-pilot-corpora` re-baselines a regression as quietly as
  it records a fix. Every count that moves must be judged against the OMG model that produced it
  before the new number is committed; a file going clean is only an improvement if the references it
  used to report now resolve, since a file also goes clean when a construct stops being checked.

## How the counts are measured

- Every file of a root is opened into one workspace **before** any diagnostic is read, because the
  corpora import across files: diagnosing a file while later ones are unopened would measure the
  alphabetical order of the corpus rather than the implementation. This is what the training gate
  and `cmd/pilot-diff` both do.
- Each root is loaded as one batch per language, mirroring `cmd/pilot-diff`, where a KerML file and
  a SysML file do not share a resource set.
- Diagnostics of **every** severity are counted, not errors alone, so a warning that appears or
  disappears is a movement the gate reports. Only the count is recorded, so a diagnostic that merely
  changes severity leaves the count untouched and passes. Paths are recorded relative to their root,
  so the file is machine-independent.
- The run sets `XDG_CACHE_HOME` to a temporary directory, so it measures the implementation on an
  empty semantic cache — what a fresh checkout and CI do — rather than the developer's machine.
- `TestCorpusGatesCacheStateIndependent` pins that a run restored from a populated library cache
  agrees with a cold one, over all four roots in one test — the training corpus for SysML, and
  `kerml-examples` for the KerML half it contains no files for.

## Starting baseline

Generated from the code at the time the gate was added, and reproduced byte-identically across
repeated runs:

| Root | Files reporting diagnostics | Diagnostics |
|---|---|---|
| `sysml-examples` | 28 / 98 | 138 |
| `sysml-validation` | 6 / 56 | 22 |
| `kerml-examples` | 20 / 58 | 80 |

## Local runs

The corpora are not vendored, so the gate skips while they are absent — and announces the skip on
stderr with a `GATE NOT RUN` banner, because `go test` hides skip reasons without `-v` and a gate
that never ran must not look like a gate that passed. Fetch them once:

```bash
./scripts/download-pilot-corpora.sh
go test -count=1 ./internal/core/model -run TestPilotCorpora
```
