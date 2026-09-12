---
name: testing-pssm-referee
description: How to verify the PSSM state-machine referee (cmd/pssm-referee + scripts/download-pssm-suite.sh + internal/pssm) end to end on Linux — provisioning the pinned OMG test suite, reproducing the committed bucket counts, proving the determinism, count and trace-disagreement detectors are live, and the adversarial paths (bad checksum, missing suite, a hand-broken translation) that distinguish working from broken.
---

# Testing the PSSM referee

The referee reads the OMG PSSM 1.0 test suite (`ptc/18-11-06`, `PSSM_TestSuite.xmi`, 103
tests), translates each state-machine test that has a SysML v2 spelling into textual notation
in memory, runs it under the runtime's own state-machine driver with the `explore` schedule,
and compares the set of `log` values reachable against the suite's expected traces. It files
each test as `pass`, `fail`, `not-expressible`, `terminate-gap` or `differs-by-design`.

A pass checks that the runtime reproduces UML behavior where the model has a defensible
SysML v2 mapping, provides a second opinion on the tool-choice rows of
`docs/internals/design/precise-semantics-alignment.md`, and is never evidence of SysML v2
conformance. Nothing in this skill changes that: if a run looks like it proves conformance,
you have misread it. See [pssm-referee.md](../../../docs/project/pssm-referee.md).

## Provision

```sh
./scripts/download-pssm-suite.sh          # → build/pssm/PSSM_TestSuite.xmi (~19 MB)
```

The pin (document, version, URL, sha256) lives in `scripts/pssm-pin.sh`; the script fetches
over HTTPS only, checks the sha256 in a staging directory and only then moves the file into
`build/pssm/`, writing `build/pssm/.pssm-pin` beside it. Facts worth knowing before you test it:

- A second run is an early exit: `Already present at .../PSSM_TestSuite.xmi (ptc/18-11-06,
  sha256 c355b249…)`, exit 0, nothing rewritten. `--force` re-downloads. Any other argument
  is `error: unknown option`, exit 1.
- Editing `build/pssm/.pssm-pin` (or changing `PSSM_SUITE_SHA256` in the environment) makes
  the next run print `Stale pin at ...; re-downloading.` and fetch again.
- `PSSM_SUITE_SHA256=0000… ./scripts/download-pssm-suite.sh --force` → exit 1, `error:
  PSSM_TestSuite.xmi from ... has sha256 c355b2…, scripts/pssm-pin.sh pins 0000…`, and
  `build/pssm/` is left exactly as it was (the download is staged in a `mktemp -d` that the
  trap removes). Verify with `sha256sum build/pssm/PSSM_TestSuite.xmi` before and after.
- `PSSM_SUITE_URL=https://www.omg.org/spec/PSSM/20181101/does-not-exist.xmi` → exit 1 with
  curl's error indented and the fallback instructions (fetch elsewhere, verify the sha256,
  point `PSSM_SUITE_ROOT` at it). A plain `http://` URL is refused by `--proto '=https'`.
- `PSSM_SUITE_ROOT=/tmp/elsewhere ./scripts/download-pssm-suite.sh` writes there instead;
  the referee then needs `-suite /tmp/elsewhere`.

Nothing from the suite is committed: `git status --porcelain` must stay empty after
provisioning (`/build/` is ignored), and `git ls-files | grep -i pssm` lists only the
scripts, the Go package, the docs and the baseline.

## Run

```sh
go run ./cmd/pssm-referee                 # < 1 s wall for the whole suite
go run ./cmd/pssm-referee -check          # exit 0 iff the bucket counts match the baseline
go run ./cmd/pssm-referee -json           # the full report, byte-stable
go run ./cmd/pssm-referee -filter "Deferred 006"
go run ./cmd/pssm-referee -keep /tmp/pssm-models   # writes every translated .sysml
```

`-h` must open with the sentence above, verbatim (`TestHelpOpensWithTheMeaningOfAPass`
pins it). Read the reference counts from `docs/project/pssm-referee-baseline.json`
(`.counts`), not from this file; each runtime change moves them.

The summary prints the counts, then every non-passing test under its bucket with its
reasons: `reached a trace the suite does not admit: …` and `admitted trace not reached: …`
for a trace-set disagreement, `run error: …` for a typed runtime error or an exhausted
budget, and `reports on SM<n> (<title>): <verdict>` when the committed row table
(`internal/pssm/rows.go`) maps the test to an alignment-note row.

## Checks that actually distinguish working from broken

- **Determinism.** `go run ./cmd/pssm-referee -json -jobs 1 | sha256sum` equals the same
  with `-jobs 8`, and two `-jobs 8` runs agree. `go run -race ./cmd/pssm-referee -jobs 8`
  prints no `DATA RACE`. `TestRefereeDeterministic` pins the JSON across job counts in-process.
- **The count gate is live.** Copy the baseline aside, edit one count (`"pass": 25` → `24`
  and `"fail": 34` → `35`, keeping the total), run `-check`: exit 1 with `does not
  reproduce:` / `pass: baseline 24, this run 25` / `fail: baseline 35, this run 34` and the
  line `the provenance matches, so this is a movement of the runtime or the translation:
  adjudicate it, then re-record with -update`. Restore the file. Changing a per-test bucket
  in the JSON without changing the counts does **not** fail `-check` — that is the design
  (the per-test rows are for the adjudicator; the count is the gate), and
  `TestReproducesByCount` pins both halves.
- **Provenance is compared before counts.** Edit `"suiteDigest"` in the copy: `-check`
  reports `provenance: baseline measured …, this run …` and does not attribute the movement
  to the runtime.
- **`-update` is reproducible.** `go run ./cmd/pssm-referee -update -develop <sha>` followed
  by `git diff --stat docs/project/pssm-referee-baseline.json` changes only the `recorded`
  date (and `develop` if you passed a different sha). `-update` refuses `-filter`;
  `-develop` without `-update` is refused too (`TestRefusedOptions`).
- **Trace disagreement is reachable, both directions.** `TestRefereeTraceSetDisagreement`
  builds a suite fragment whose expected set is missing one reachable interleaving and lists
  one unreachable trace, and asserts both reasons name the trace. Reproduce by hand from the
  real suite: `-filter "Transition 011 C"` shows one extra and one missing trace — a
  `reached a trace the suite does not admit` **and** an `admitted trace not reached` line.
- **Runtime errors are `fail`, never `differs-by-design`.** `-filter "History 001-A"` files
  a `run error: … history … has no default transition` under `fail` with `reports on SM28`;
  the SM28 row is `differs, v2 silent` (a tool choice), so the bucket stays `fail`.
  `TestRefereeDiffersByDesignIsNotInferred` pins that a failing test with no row, or with a
  tool-choice row, is `fail`, and only an explicit `differs because v2 differs` row moves it.
  `TestRefereeRowsAreWellFormed` pins that every test in `TestRows` names a row in `Rows`
  and is a test the suite contains (so a typo cannot silently drop a mapping).
- **Budget exhaustion is a `fail` that names the budget.** `TestRefereeBudgetExhausted`
  runs with a one-step budget and asserts the reason.

## Adversarial paths

- **Missing suite.** `go run ./cmd/pssm-referee -suite /tmp/empty` (an existing directory
  with no XMI) → exit **0**, prints `PSSM test suite is absent at /tmp/empty/PSSM_TestSuite.xmi;
  run ./scripts/download-pssm-suite.sh to provision it`, and writes nothing. With
  `OPENSYSML_REQUIRE_PSSM_SUITE=1` the same command exits **1** with `OPENSYSML_REQUIRE_PSSM_SUITE
  is set: PSSM test suite is absent …`. The Go gates (`TestSuiteRead`,
  `TestSuiteClassification`, `TestEmitSuite`) skip with the same message and fail with the
  variable set: `OPENSYSML_REQUIRE_PSSM_SUITE=1 go test ./internal/pssm -run TestEmitSuite`
  against an absent `build/pssm` must fail, not skip. Use a fresh `-suite` path so an existing
  `build/pssm` cannot make the assertion vacuous.
- **Bad checksum.** Copy the suite to a scratch directory and append one byte
  (`printf '\n' >> /tmp/bad/PSSM_TestSuite.xmi`); `-suite /tmp/bad` → exit 1,
  `<path> has sha256 88ad09…, not the pinned c355b2…; re-run ./scripts/download-pssm-suite.sh`.
  The digest is verified before the XMI is parsed, so a truncated or edited suite is never
  read (`TestBadChecksum`). A malformed XMI with the right digest cannot exist, which is why
  the reader's diagnostics are tested on fragments (`TestReadDiagnostics`,
  `TestReadUnsupportedNodes`) rather than on the suite.
- **A hand-broken translation.** The emitter runs in memory, so break it at the source, then
  put it back:
  1. In `internal/pssm/emit.go`, change the `"::"` separator in the `trace` translation to
     `"--"`. `OPENSYSML_REQUIRE_PSSM_SUITE=1 go test ./internal/pssm -run TestEmitSuite` still
     passes (the model is syntactically fine), and that is the point: the parse gate cannot
     see it. `go run ./cmd/pssm-referee` then moves every multi-segment `pass` to `fail`
     with `reached a trace the suite does not admit: S1(entry)--…` — `-check` exits 1 with
     `pass: baseline 25, this run <n>`. Restore the file; `-check` is green again.
  2. Write a model the front end rejects: in `emit.go` change `attribute log : String = ""`
     to `attribute log : Strin = ""`. `TestEmitSuite` now fails for every expressible test
     with the validator's diagnostic and the model text, and `TestEmitStandard` (the
     fixture-level test) fails first. Restore the file.
  3. Break a fixture instead of the emitter: `TestEmitRejects` lists the shapes the emitter
     refuses (a structured payload, a call with arguments, a behavior with parameters); add
     one to a fixture in `emit_test.go` and confirm the error names the construct and the
     place. Revert.
- **Reclassification is pinned.** `TestSuiteClassification` pins the per-area and total
  counts (31 standard / 30 extension / 3 terminate-gap / 39 not-expressible). Moving one
  construct between buckets in `classify.go` fails it with the area that moved; the note's
  test-suite section and `docs/project/pssm-referee.md` must move with it — they are the
  record of every move and its reason.

## CI

The pull-request workflow provisions the suite (`./scripts/download-pssm-suite.sh`, cached on
the pin), sets `OPENSYSML_REQUIRE_PSSM_SUITE=1` so the Go gates cannot skip, and runs
`go run ./cmd/pssm-referee -check` as its own step so the counts are legible in the log. A
movement in any bucket fails that step until the baseline is regenerated with `-update` and
the movement adjudicated in the pull-request body. It never gates on all-pass.

## Devin Secrets Needed

None. Network access is required only to provision the suite from omg.org.

## Recording

CLI-only work; per the testing-mode guidance this needs no screen recording — collect verbatim
stdout/stderr and exit codes instead.
