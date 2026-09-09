---
name: testing-pilot-corpora-gate
description: How to verify the four OMG corpus gates (internal/core/model/corpus_gate_test.go + pilot_corpora_test.go + training_examples_test.go and their testdata expectations) end to end on Linux — running both gates, proving the baselines are reproducible/machine-independent, the adversarial mutations that must fail, absence handling per root, and exercising the shared pilot-pin.sh downloader.
---

# Testing the OMG corpus gates (training assertion + pilot-corpora ratchet)

Shell-only; no GUI or recording needed. One full `./internal/core/model` run is ~60s; the two
corpus tests alone are ~4s, so iterate with `-run` and only do the full run at the start/end.

Four pinned OMG model roots, **one mechanism, two policies** (`internal/core/model/corpus_gate_test.go`):

| root | dir | policy | expectation file |
|---|---|---|---|
| training | `examples/sysml-v2-training` | **assertion** — errors only, `.sysml` only, must be 100% clean, no per-file counts allowed | `testdata/training_examples_expected.txt` |
| sysml-examples | `examples/pilot-corpora/sysml-examples` | per-file **ratchet**, every severity | `testdata/pilot_corpora_expected.txt` |
| sysml-validation | `examples/pilot-corpora/sysml-validation` | ratchet | same |
| kerml-examples | `examples/pilot-corpora/kerml-examples` | ratchet | same |

## Provision + run

```bash
./scripts/download-training-examples.sh   # examples/sysml-v2-training  (untracked/gitignored)
./scripts/download-pilot-corpora.sh       # examples/pilot-corpora      (untracked/gitignored)
OPENSYSML_REQUIRE_TRAINING_CORPUS=1 OPENSYSML_REQUIRE_PILOT_CORPORA=1 \
  go test -count=1 -v ./internal/core/model
```

The corpora are gitignored, so **copy them aside first** (`cp -a examples/pilot-corpora
examples/sysml-v2-training /tmp/pristine/`) — every absence/mutation test moves or edits them and
`git checkout --` cannot restore them. `diff -r` against the pristine copy after each case.

Expect `--- PASS` for `TestTrainingExamplesSemanticErrors`, `TestPilotCorporaDiagnostics` and
`TestCorpusGatesCacheStateIndependent` (subtests `training` and `pilot-corpora` — the older
`TestTrainingExamplesCacheStateIndependent` / `TestPilotCorporaCacheStateIndependent` no longer
exist), plus `100/100 training files clean` and three `N/M pilot corpus files clean` lines
(observed 2026-08: sysml-examples 76/98, sysml-validation 52/56, kerml-examples 51/58 — these move
whenever diagnostics change, so read the current numbers from `docs/project/pilot-corpora.md`
rather than hard-coding them).

Note the useful verdict lines are `t.Logf`, so they only appear with `-v`, and the ratchet test
logs one `pilot_corpora_test.go:37:` line per reporting file — filter those out
(`grep -vE 'pilot_corpora_test\.go:37:'`) or the real assertion messages scroll off. Filtering test
output with `grep -E "a|b"` breaks when a pattern starts with `-`; use `grep -E -e "..." -e "..."`.

## Reproducibility of the baselines

```bash
go test -count=1 ./internal/core/model -run TestPilotCorporaDiagnostics    -update-pilot-corpora
go test -count=1 ./internal/core/model -run TestTrainingExamplesSemanticErrors -update-training
git diff --exit-code -- internal/core/model/testdata/
```

Regeneration must be byte-identical across runs, from a different cwd
(`cd /tmp && XDG_CACHE_HOME=$(mktemp -d) go test -C <repo> ...`) and with a fresh `XDG_CACHE_HOME` —
each test sets `XDG_CACHE_HOME` to a temp dir itself, which is what makes it machine-independent.
Also check no absolute paths leak:
`grep -nE '(^|[[:space:]])/(home|tmp|Users)' internal/core/model/testdata/*_expected.txt` must find nothing.

## Adversarial mutations that must each fail (restore with `git checkout --` after each)

Mutate `internal/core/model/testdata/pilot_corpora_expected.txt` and re-run
`OPENSYSML_REQUIRE_PILOT_CORPORA=1 go test -count=1 ./internal/core/model -run TestPilotCorporaDiagnostics`:

| mutation | expected message |
|---|---|
| count up | `<path>: 2 diagnostic(s), expected 3 (fewer than recorded); adjudicate ...` |
| count down | `... (more than recorded) ...` |
| delete an entry | `<path>: N new diagnostic(s), previously clean; adjudicate ...` |
| add an entry for a currently-clean file | `<path>: expected 1 diagnostic(s) but the file is now clean` |
| change a `# files: <root> <n>` total | `<root> holds 98 model file(s), expectations were recorded against 97` |
| tab → space on a data line | `pilot_corpora_expected.txt:<line>: want "<count>\t<path>", got ...` |
| non-numeric count | `pilot_corpora_expected.txt:<line>: bad count: strconv.Atoi ...` |
| drop `<n>` from a `# files:` line | `pilot_corpora_expected.txt:<line>: want "# files: <root> <n>", got ...` |
| stray extra `.sysml` copied into a root | the `# files:` total check (`holds 99 ... recorded against 98`) |

Pick a stable victim entry with a literal `sed` match (e.g.
`2\tkerml-examples/Simple Tests/Associations.kerml`); to find a *clean* file for the "added entry"
case, `comm -23` the root's `find`-listing against the paths already in the expectation file.

## The training gate is an assertion, not a ratchet — prove it

Two distinct properties, both worth testing:

1. Adding any `<count>\tpath` line to `training_examples_expected.txt` (which is otherwise
   comment-only plus `# files: 100`) must fail with
   `... records 1 per-file count(s) ([...]); the training corpus is asserted clean, not ratcheted, ...`
2. With a real error injected into a training file (append
   `package BogusForTest { part x : NoSuchTypeAnywhere; }` to an existing `.sysml`), the plain gate
   fails with `<path>: 1 semantic error(s); the training corpus must stay clean`, **and**
   `-update-training` must also fail with
   `1 file(s) report errors ([...]); the training gate asserts a clean corpus, so -update-training cannot record them`
   leaving the expectation file byte-identical.

Pitfall: `printf ... >> "examples/sysml-v2-training/<guessed name>.sysml"` silently *creates* a new
file if the name is wrong, which trips the `# files: 100` total check instead of the error check and
leaves an untracked corpus file behind. `ls` the directory and confirm the file exists first
(e.g. `01. Packages/Package Example.sysml` is real; `01. Packages/Packages.sysml` is not).

## Absence handling — test all four roots separately

For each of the four roots, in three shapes (moved aside / empty-but-present / only a `README.txt`):

- with the root's require-env (`OPENSYSML_REQUIRE_TRAINING_CORPUS` or
  `OPENSYSML_REQUIRE_PILOT_CORPORA`) → FAIL, `<env>=1 but <dir> is missing: ...` or
  `... holds no model files: ...`
- without it → exit 0 with `--- SKIP` plus the `!!! GATE NOT RUN` stderr banner naming the right
  fetch script for that gate.
- CI catch: `go test -count=1 -v ./internal/core/model -run 'TestTrainingExamples|TestCorpusGates' | tee corpus-gate.log`
  then `grep -qE '^\s*--- SKIP' corpus-gate.log`. Note an absent **pilot** root surfaces in that CI
  command only as `--- SKIP: TestCorpusGatesCacheStateIndependent/pilot-corpora` (the ratchet test
  itself is not in the `-run` pattern) — the grep still catches it, and that is worth verifying.

## Shared downloader (`scripts/pilot-pin.sh`)

`pilot_fetch_subtrees "<path in pilot repo>:<dest>" ...` is the single fetch behind
`download-training-examples.sh` (1 entry), `download-pilot-corpora.sh` (3 entries) and
`download-pilot-xpect.sh` (2 entries, counting `*.xt`); `download-pilot-grammars.sh` uses the
same `pilot_clone`. The pin is `PILOT_TAG` **and** `PILOT_COMMIT`: the clone is by tag, then
`pilot_clone` fails unless `HEAD` is the pinned commit, because a tag is a mutable ref.

- Every destination is stamped with `.pilot-pin` (`<tag> <commit> <repo>`). A destination whose
  stamp is the current pin prints `Already present at <dest> (pin 2026-07 c7fc737d…)` and is
  skipped; if *all* are present no clone happens at all. A stamp that differs prints `Stale pin at
  <dest>: fetched from …; re-downloading.` and an unstamped directory prints `No pin recorded at
  <dest>: …; re-downloading.` — both re-fetch, and the old copy is only replaced after the clone
  succeeded. This is what heals a checkout provisioned at an earlier pin (the 98-example /
  125-`.xt` state that made every provenance test fail was exactly such a stale, unstamped copy).
- Re-fetch a moved-aside root and `diff -r` it against the pristine copy — must be identical
  (58 kerml-examples, 99 sysml-examples, 56 sysml-validation, 100 training model files) apart
  from the `.pilot-pin` stamp if the old copy predates it.
- To prove pinning/sparseness without touching the script, put a logging `git` wrapper first on
  `PATH` (`echo "$*" >> log; /usr/bin/git "$@"`, and after `sparse-checkout` also dump
  `git -C <dir> sparse-checkout list`, `git -C <dir> log -1 --decorate` and `ls <dir>`). Expect
  `clone --quiet --filter=blob:none --sparse --depth 1 --branch 2026-07 ...`,
  `rev-parse HEAD^{commit}`, `sparse-checkout set <only the requested paths>`, HEAD
  `c7fc737d56da9e2d78f9d7df6d38efbec2e7e965` decorated `tag: 2026-07`, and no unrequested subtree
  on disk.
- Error paths, all exit 1 and leave the existing destinations untouched: a bogus source path →
  `error: <path> is missing from <repo> at <tag>`; `PILOT_TAG=9999-99` → git's
  `fatal: Remote branch 9999-99 not found in upstream origin` then `error: could not clone <repo>
  at 9999-99, the tag scripts/pilot-pin.sh pins`; `PILOT_COMMIT=<other sha>` (simulates the tag
  moving upstream) → `error: <repo> tag 2026-07 resolves to c7fc737d…, scripts/pilot-pin.sh pins
  <other sha>` and nothing is provisioned.

## The gate runs cold: never compare it against an ad-hoc harness or the CLI

`TestPilotCorporaDiagnostics` does `t.Setenv("XDG_CACHE_HOME", t.TempDir())` before counting, so it
always indexes the standard library by parsing it. A hand-rolled harness (or `go run ./cmd/sysml
-validate`, or the LSP) uses the persistent library index under `$XDG_CACHE_HOME/sysml-ls/libs`
instead, and that index is keyed only by the library content hash plus a schema version (`-v22`) —
**not** by the OpenSysML build. A change to symbol classification or anything else the index stores
therefore leaves a stale cache in place, and the same tree can report different diagnostics through
the two paths (observed: a corpus file clean under the gate but reporting three
`Must be a valid feature` errors through a warm pre-change cache). When reproducing or refuting a
gate verdict outside the gate, always `XDG_CACHE_HOME=$(mktemp -d)` first; if the two disagree,
suspect a stale index before suspecting the gate.

## Attributing a corpus movement to a commit

Expectation lines move because of *some* commit, and the commit message's guess is not evidence.
Bisect it mechanically in a throwaway worktree (`git worktree add -f /tmp/wt-main <base>`, then
symlink the gitignored `examples/pilot-corpora` and `examples/sysml-v2-training` in from the main
checkout so the gate does not skip):

1. Confirm the gate is RED/GREEN there as claimed.
2. `git revert --no-commit <suspect>` and re-run the gate — if the count does not come back, the
   suspect is not the cause. Use `git reset --hard` between attempts: a failed multi-pathspec
   `git checkout --` restores nothing, and `git checkout <sha> -- <files>` leaves the index staged.
3. Walk the other candidates on the merged side (`git log --oneline <merge>^2 --not <merge>^1`) the
   same way until the revert reproduces the old count exactly.
4. Boil the finding down to a two-line model that reproduces the old diagnostic and is clean now
   (e.g. a `:>>` binding to a stdlib `bool` such as `Occurrences::earlierFirstIncomingTransferSort`),
   and keep a genuine violation of the same check nearby as the negative control, so "fixed" is
   distinguished from "check dropped".

Print spans for one corpus file by dropping a scratch `*_test.go` into `internal/core/model`: the
gate's own helpers are package-private but reusable (`pilotCorporaGate.files(t)` /
`.counts(t, files)`), diagnostics carry byte offsets only, so map them with
`source.New(name, content).Lines().PosAt(d.Span.Offset)`.

## Tooling on this box

`actionlint`, `shellcheck`, `python3 scripts/check-doc-links.py`, `gofmt`, `go vet`,
`go run ./cmd/pilot-diff` (validators pre-downloaded; ~4min, prints e.g.
the headline the committed baseline holds — `368 file(s), 338 fully agreeing; 34 agreed
diagnostic(s), 21 only ours, 596 only the pilot's` after the unbound-parameter round, so read it from
`docs/project/pilot-differential-baseline.json` rather than from this line)
and `make lint` (staticcheck+gosec, ~2min) all work. There is **no** `yamllint` and **no**
`circleci` CLI, so `.circleci/config.yml` can only be parsed as YAML, not schema-validated — say so
in reports rather than implying it was validated.

## Devin Secrets Needed

None. Network access to github.com is required for the downloader tests.
