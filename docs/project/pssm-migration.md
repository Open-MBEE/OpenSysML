# PSSM Migration Gate

## Overview

**Corpus:** the OMG PSSM test suite, one XMI document of some 47 000 UML elements, fetched by
`./scripts/download-pssm-suite.sh` at the pin in `scripts/pssm-pin.sh` (document `ptc/18-11-06`,
checksum-verified) into `build/pssm/PSSM_TestSuite.xmi` — the same file the
[PSSM referee](pssm-referee.md) translates by rule. This gate runs the SysML v1 migrator over it
instead (`sysml build/pssm/PSSM_TestSuite.xmi -convert sysml`), the largest v1 model the migrator
is exercised on.

**Gate:** `TestPSSMSuiteMigration` in `tests/corpus/pssm_migration_test.go`. Two policies in one
test:

- **The notation is asserted to parse.** The migrator writes one file for the whole suite, and
  the converter refuses a file with a syntax error anywhere in it, so one v1 shape written in a
  form the parser rejects hides the translation of everything else. A syntax error fails the gate
  with its line and message; it is never recorded.
- **The report totals and the validation-error count ratchet.** The migration report's totals by
  verdict (`mapped`, `approximated`, `unmapped`, `skipped`) and the number of error diagnostics
  the written notation analyses with are pinned in
  `tests/corpus/testdata/pssm_migration_expected.txt`; a figure that moves in either direction
  fails the gate until it is adjudicated and the file regenerated.

**Regenerate:** `go test ./tests/corpus -run TestPSSMSuiteMigration -update-pssm-migration`
**Required in CI:** `OPENSYSML_REQUIRE_PSSM_SUITE=1` in both `.circleci/config.yml` and
`.github/workflows/pr.yml` — the variable the referee's own gates already require — under which
an absent or empty suite fails instead of skipping. Both configurations also run the gate on its
own, so the report totals are legible in the log and a skip cannot hide behind a green suite.

## Why it ratchets where the pilot corpora ratchet per file

The suite is one file, so there is no per-file table to pin; the figures that describe the
migration are the report's totals. They follow the adjudication rule of the
[pilot corpora gate](pilot-corpora.md): the expectation file is a snapshot, so `-update-pssm-migration`
re-baselines a regression as quietly as it records an improvement, and every figure that moves
must be explained against the v1 model before the new number is committed. A verdict total moving
means some construct is now mapped, approximated or refused differently; `unmapped` going down is
an improvement only if the elements it lost now have a v2 form, since it also goes down when a
construct is skipped as content nothing refers to.

The validation-error count is recorded because it is stable: the same notation analyses with the
same errors on an empty semantic cache and a populated one, and across repeated runs. It is a
ratchet rather than an assertion because the suite is not yet clean — its errors are references
the migrator does not resolve (members of the suite's test-harness classes and a junction with
no outgoing transition), not consequences of how the notation is written. Only errors are counted:
the notation carries thousands of warnings about approximated constructs, whose number is a
property of the migrator's notes rather than of the model.

## What the gate does not do

- **It is not the PSSM referee.** The referee ([pssm-referee.md](pssm-referee.md)) translates each
  test by rule and runs it against the runtime, adjudicating its bucket. This gate never executes
  anything: it migrates the suite as a user would migrate their own v1 model and checks that the
  result is a document the toolchain reads.
- **It is not a conformance claim.** The verdict totals are the migrator's own report of what it
  kept, approximated and refused; the adjudication is the reader's.
- **It does not adjudicate.** A movement is a question, not an answer; the answer belongs in the
  pull request that regenerates the file.

## How the figures are measured

- The suite is migrated through `convert.Migrate`, the path `sysml -convert sysml` takes, so the
  syntax assertion is the converter's own refusal and not a second parse.
- The written notation is opened into one workspace and its `SeverityError` diagnostics counted;
  each is logged, so the log names the errors behind the figure.
- The run sets `XDG_CACHE_HOME` to a temporary directory, so it measures the implementation on an
  empty semantic cache — what a fresh checkout and CI do — rather than the developer's machine.

## Local runs

The suite is not vendored, so the gate skips while it is absent — announcing the skip on stderr
with a `GATE NOT RUN` banner, as the other corpus gates do. Fetch it once:

```bash
./scripts/download-pssm-suite.sh
go test -count=1 ./tests/corpus -run TestPSSMSuiteMigration
```

An environment that sets `OPENSYSML_REQUIRE_PSSM_SUITE` must run the download script before
`go test ./...`; the script leaves a suite already at the pin alone.
