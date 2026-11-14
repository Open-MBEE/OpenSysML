# The cross-MDK conformance suite as a daily advisory ratchet

`tools/cmd/hideo-suite` runs the OpenSysML readers of
[hideo67/sysmlv2-tests](https://github.com/hideo67/sysmlv2-tests), a common test suite for
SysML v2 MDKs, against this tree and holds **every case of every reader** to its committed
outcome in [`hideo-suite-baseline.tsv`](hideo-suite-baseline.tsv). It is advisory: it runs daily
in the [oracle reproduction](../../.github/workflows/oracle-reproduction.yml) workflow and on
demand, never on a pull request and never as a required check, exactly as the
[PSSM referee](pssm-referee.md) and the [pilot differential](pilot-differential.md) are
re-run there.

**What a pass means.** The suite's expectations are the answers of the OMG pilot implementation,
recorded by the suite's author. A pass is agreement with the pilot on one read of one model
through one API; a fail is a disagreement to adjudicate, which may be this implementation's gap,
the pilot's, or the suite's. It is never evidence of SysML v2 conformance.

## The suite, pinned

| | |
|---|---|
| Repository | <https://github.com/hideo67/sysmlv2-tests> |
| Commit | `99eae49af071a79c81bf4262eb0c73d014fbb983`, the head of its default branch on 2026-11-11; the suite publishes no tags |
| Pin | `scripts/hideo-suite-pin.sh` (`HIDEO_SUITE_REPO`, `HIDEO_SUITE_COMMIT`) |
| Download | `./scripts/download-hideo-suite.sh`, into the git-ignored `build/hideo-suite/`, idempotently; a checkout that is not the pinned commit, or that holds no case file, is refused, and the ratchet refuses to run a checkout whose commit or tracked files are not the pinned ones |
| Contents | 23 matrices (`A01` … `F10`) under `test-suite/data/matrices/`, the cases generated from them under `test-suite/data/cases/` (1,813 at this pin), and the models they read under `test-suite/data/` |

The suite is not vendored. It is its author's and states no licence, so nothing of it is
committed here: only this implementation's outcome per case id.

The suite reads the implementation under test from a directory named `OpenSysML` beside its
`test-suite/`: its Python readers install `OpenSysML/client/python` and drive
`OpenSysML/bin/sysml-grpc`, and its Go readers `replace` this module with that directory. The
download script makes that directory a symbolic link to the repository it is run from, so the
suite measures the working tree, and the ratchet refuses a link that points anywhere else.

Four of the suite's readers are run, in the baseline's column order:

| Suite | API | Client |
|---|---|---|
| `opensysml-py-props` | element properties (`GetElementProperties`) | Python, over `sysml-grpc` |
| `opensysml-go-props` | element properties | Go, in process |
| `opensysml-py` | Symbol API | Python, over `sysml-grpc` |
| `opensysml-go` | Symbol API | Go, in process |

The suite's Rust, TypeScript and Java readers of this implementation, and its readers of other
MDKs, are not run. Its cross-MDK report needs a pilot checkout and Java; the ratchet reads the
per-case CSV files each reader writes and needs neither, so the report's failure at the end of
the suite's own log is expected.

## Running it

```bash
./scripts/download-hideo-suite.sh             # once; needs git
go run -C tools ./cmd/hideo-suite             # run and print the movement, exit 0
go run -C tools ./cmd/hideo-suite -check      # the same, failing on any movement
```

It needs Go, Python 3.12 and [uv](https://docs.astral.sh/uv/) on `PATH`. Each run builds
`bin/sysml-grpc` with `make build-grpc`, provisions the suite's Python environment with
`uv sync`, and runs the four readers through the suite's `run-all.sh` with Go's test cache off;
a run takes about a minute and a half. The results are left in
`build/hideo-suite/test-suite/results/`: one CSV per reader (`suite,matrix,id,outcome,detail`)
and its log under `logs/`. `-no-run` compares the results already there instead of running.
The readers are given this checkout's `bin/sysml-grpc` as `OPENSYSML_BINARY` and no
`OPENSYSML_SERVICE`, whatever the caller's environment names, so the outcomes are this tree's.

The checkout holds the suite's own Go sources, one of which is not `gofmt`-formatted, so
`gofmt -l .` names it while the checkout is under `build/`. Set `HIDEO_SUITE_ROOT` to a
directory outside the repository for the download and pass the same directory as `-suite` to
keep the tree's own gates quiet.

When the checkout is absent the command says so and exits 0; the workflow sets
`OPENSYSML_REQUIRE_HIDEO_SUITE=1`, under which an absent checkout fails.

## What the ratchet compares

An outcome is `pass`, `fail`, `unsupported` (the reader does not implement the case's read),
`error` (the reader could not complete it) or `skip`. They are ordered for the ratchet in three
ranks: `pass`, then `unsupported` and `skip`, then `fail` and `error`.

| Movement | Example | Without `-check` | With `-check` |
|---|---|---|---|
| Worse | `pass` → anything else, `unsupported` → `fail` | named | fails |
| Better | `fail` → `pass`, `unsupported` → `pass` | named | fails until recorded with `-update` |
| Changed within a rank | `fail` → `error` | named | fails until recorded with `-update` |
| Provisioning failure | a reader wrote no results, wrote `error` for every case, or wrote a different set of case ids | named per reader | fails |

A provisioning failure is reported once per reader and its cases are not compared, so a service
that did not start is never read as hundreds of regressions; `-update` refuses to record such a
run, including one in which every reader lost the same cases while the pin did not move. The outcomes are deterministic: two runs of the same tree give the same outcome for every
case.

## Adjudicating a movement

The daily run fails when `develop` no longer reproduces the baseline. The artifact
`hideo-suite-results` of that run holds the CSVs and logs; the `detail` column of a moved case
says what the reader got and what the pilot answered.

- **A case got worse.** Find the change that moved it and fix it, or, when the new answer is
  the right one and the suite's expectation is not, say so in the pull request and record it
  with `-update -accept-regressions`. `-update` alone refuses a run in which a case got worse.
- **A case got better.** Confirm the change that moved it meant to, then record it with
  `-update` in a pull request that says which change it was.
- **A provisioning failure.** Read the reader's log; nothing is recorded.
- **The pin moves.** Change `HIDEO_SUITE_COMMIT`, the commit and date in the table above and in
  the pin script's comment, re-download, and re-record with `-update`; a baseline measured at
  another commit is refused by `-check`, and `go test -C tools ./referee/hideo` fails until the
  baseline and this record name the pinned commit.

```bash
go run -C tools ./cmd/hideo-suite -update     # re-record the baseline and the table below
```

The baseline and the table below are written only by `-update`; neither is edited by hand, and
the tool's tests fail when the table is not the one the baseline gives.

## Current outcomes

Each cell is `pass / fail / unsupported` over the matrix's cases; an `error` or `skip` count is
named when there is one. The matrices themselves are described in the suite's
`test-suite/data/matrices/`.

<!-- hideo-suite:begin counts -->
| Matrix | Cases | `opensysml-py-props` | `opensysml-go-props` | `opensysml-py` | `opensysml-go` |
|---|---:|---:|---:|---:|---:|
| A01 | 117 | 117 / 0 / 0 | 117 / 0 / 0 | 87 / 0 / 30 | 87 / 0 / 30 |
| A02 | 50 | 40 / 0 / 10 | 40 / 0 / 10 | 30 / 10 / 10 | 30 / 10 / 10 |
| A03 | 128 | 128 / 0 / 0 | 128 / 0 / 0 | 96 / 0 / 32 | 96 / 0 / 32 |
| A10 | 36 | 36 / 0 / 0 | 36 / 0 / 0 | 20 / 0 / 16 | 18 / 2 / 16 |
| A20 | 100 | 100 / 0 / 0 | 100 / 0 / 0 | 80 / 0 / 20 | 80 / 0 / 20 |
| A30 | 44 | 44 / 0 / 0 | 44 / 0 / 0 | 34 / 10 / 0 | 34 / 10 / 0 |
| A40 | 45 | 42 / 3 / 0 | 42 / 3 / 0 | 45 / 0 / 0 | 45 / 0 / 0 |
| A50 | 54 | 54 / 0 / 0 | 54 / 0 / 0 | 36 / 0 / 18 | 24 / 12 / 18 |
| A60 | 32 | 32 / 0 / 0 | 32 / 0 / 0 | 32 / 0 / 0 | 32 / 0 / 0 |
| B10 | 98 | 90 / 8 / 0 | 90 / 8 / 0 | 0 / 36 / 62 | 0 / 36 / 62 |
| B11 | 90 | 90 / 0 / 0 | 90 / 0 / 0 | 0 / 36 / 54 | 0 / 36 / 54 |
| B12 | 108 | 108 / 0 / 0 | 108 / 0 / 0 | 42 / 12 / 54 | 42 / 12 / 54 |
| B13 | 165 | 50 / 115 / 0 | 50 / 115 / 0 | 0 / 66 / 99 | 0 / 66 / 99 |
| B20 | 75 | 48 / 27 / 0 | 48 / 27 / 0 | 15 / 15 / 45 | 15 / 15 / 45 |
| C01 | 114 | 99 / 15 / 0 | 99 / 15 / 0 | 76 / 0 / 38 | 76 / 0 / 38 |
| C10 | 116 | 92 / 4 / 20 | 92 / 4 / 20 | 36 / 0 / 80 | 36 / 0 / 80 |
| C20 | 120 | 117 / 0 / 3 | 117 / 0 / 3 | 24 / 0 / 96 | 24 / 0 / 96 |
| C21 | 72 | 48 / 24 / 0 | 48 / 24 / 0 | 0 / 0 / 72 | 0 / 0 / 72 |
| C30 | 78 | 78 / 0 / 0 | 78 / 0 / 0 | 6 / 0 / 72 | 6 / 0 / 72 |
| D10 | 45 | 45 / 0 / 0 | 45 / 0 / 0 | 20 / 10 / 15 | 20 / 10 / 15 |
| D20 | 48 | 48 / 0 / 0 | 48 / 0 / 0 | 0 / 0 / 48 | 0 / 0 / 48 |
| E10 | 48 | 48 / 0 / 0 | 48 / 0 / 0 | 48 / 0 / 0 | 48 / 0 / 0 |
| F10 | 30 | 30 / 0 / 0 | 30 / 0 / 0 | 30 / 0 / 0 | 30 / 0 / 0 |
| **All** | 1813 | 1584 / 196 / 33 | 1584 / 196 / 33 | 757 / 195 / 861 | 743 / 209 / 861 |
<!-- hideo-suite:end counts -->

The two element-properties readers agree on every case. The two Symbol readers differ on 14:
12 cases of `A10` and `A50` pass from Python and fail from Go, whose reader reports an unnamed
member as not listed, and 2 cases of `A60` fail from Python and pass from Go. The Symbol API
covers fewer of the suite's reads than the element-properties API does, which is what its
`unsupported` column counts.
