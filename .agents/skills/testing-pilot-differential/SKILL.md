---
name: testing-pilot-differential
description: How to verify the advisory pilot-implementation differential harness (cmd/pilot-diff + scripts/download-pilot-sysml-validator.sh) end to end on Linux — provisioning the batch SysML/KerML oracles, reproducing the committed baseline, and the adversarial paths (bad pin, missing tools, wrong flags) worth checking.
---

# Testing the pilot differential harness (`cmd/pilot-diff`)

**Since F6 (PR #397) the default SysML oracle is the plain-Java batch bridge
`build/pilot-sysml-validator/validate-sysml-batch`, not the DeciSym CLI
`build/pilot-validator/validate-sysml`.** Provision it with
`./scripts/download-pilot-sysml-validator.sh` (it auto-calls
`download-pilot-validator.sh` when the pinned jar/library are absent, because the bridge
compiles and runs against them). It loads every `.sysml` file of a corpus root into **one**
resource set (`SysMLUtil.readResource`/`addInputResource`) and only then validates, printing
GNU-format diagnostics **relative to `--root`**. Consequences for testing:

- `cmd/pilot-diff/order.go` (`orderByImports`) and `batchByBaseName` are **deleted**; one root
  is exactly one invocation regardless of duplicate base names or import order. Anything in
  this file that still says "topologically sorted" or "split by basename" applies to history
  only. The KerML and SysML sides now share one `pilotDiagnostics`.
- The pin `cmd/pilot-diff` reports comes from `build/pilot-sysml-validator/pilot-pin.txt`
  (written by the new script), not from the DeciSym `pom.xml`.
- `-validator /nonexistent` now says `run ./scripts/download-pilot-sysml-validator.sh`.
- Measured at the `2026-08` re-pin, with a fresh library cache: `371 file(s), 343 fully agreeing; 34 agreed,
  37 only ours, 1105 only the pilot's`, JSON totals `openSysMLDiagnostics 74 / pilotDiagnostics
  1142 / severityMismatch 3`; ~2 min wall, byte-identical across runs *and* after a from-scratch
  rebuild of `build/pilot-validator`. The six `kerml-examples` pilot-only rows the `2026-07` run
  carried (`The opposite features 'owningType' … do not refer to each other`) are gone: the pilot
  fixed its `ownedDisjoining` delegate, and nothing on our side moved. `kerml-examples` carries no `syntax` diagnostic on either
  side. Refresh this paragraph with every rebaseline, and treat a stale one as a finding.
- **`cmd/pilot-diff` has no `-jobs` flag.** Its full flag set is
  `-repo -validator -kerml-validator -syside -out -timeout`; passing `-jobs` exits **2** with
  `flag provided but not defined: -jobs`. Only `cmd/pilot-xpect` is job-parallel. So a PR that
  claims the differential is "deterministic across `-jobs` settings" is claiming something
  untestable — prove differential determinism instead with two or three *independent* fresh-cache
  runs (`rm -rf /tmp/cN && XDG_CACHE_HOME=/tmp/cN go run ./cmd/pilot-diff -out /tmp/pd-runN`) and
  compare each `pilot-diff.json` to the committed baseline with `cmp`, or hash all of them and
  assert a single distinct sha256.
- **Assert the pilot column is non-empty before believing any census (the silent-zero trap).** If
  the pilot launcher cannot find its ~133 MB shaded jar — the common case when you copy only
  `build/pilot-sysml-validator/` into a control worktree, since the wrapper resolves the jar at
  `$SCRIPT_DIR/../pilot-validator/target/sysml-download/sysml/...` — the run prints
  `pilot ... jar not found` per root but still **exits 0** and reports a plausible census with an
  empty pilot side — reported fully-agreeing 322 / agreed 0 / only-ours 175 / only-pilot 0, where the
  truth is 321 / 32 / 92 / 66. The bogus only-ours can coincidentally equal the true
  `openSysMLDiagnostics`, which is
  exactly how such a run gets mistaken for valid. Gate every census on
  `grep -c "jar not found" == 0`, `pilotOnly > 0` and `agreement > 0`. The cheapest fix when
  measuring a base column is to leave the launchers out of the control worktree entirely and pass
  the *main* checkout's absolute `-validator` / `-kerml-validator` paths while running `go run` from
  the control worktree.
- **Regenerating the baseline JSON does not update the narrative docs, and `make docs-counts`
  cannot catch it.** Pages without `<!-- doc-count -->` markers (e.g. a wave's own
  `docs/project/waveNNx-*.md` "What moved" table and its surrounding prose) keep whatever census
  they were written with, so a rebase that restates `pilot-differential.md` plus the JSON can leave
  them contradicting the tree while every gate stays green. Cross-check each such page by hand, and
  use the identities `ourDiagnostics = onlyOurs + agreed + severityMismatch` and
  `pilotDiagnostics = onlyPilot + agreed + severityMismatch` to spot half-updated tables.
- **A parser PR moves these numbers without rebaselining the committed JSON.** Wave 10D (#462)
  measured a live `353 / 310 fully agreeing / 23 agreed / 106 only ours / 85 only the pilot's`
  (`openSysMLDiagnostics 144`) while `docs/project/pilot-differential-baseline.json` still held the
  wave-9 `309 / 119 / 157`. So `diff <(jq -S . docs/…baseline.json) <(jq -S . build/…json)`
  being non-empty is the *expected* state on such a branch, and equality would be the finding.
  Replace that check with (a) the numbers the PR author claims and (b) a parent-commit control
  (`git worktree add /tmp/wt-base origin/main`, then run the parent's harness with
  `-repo <real checkout>` and absolute `-validator`/`-kerml-validator`) — the parent must
  reproduce the committed baseline exactly, which is what proves the delta belongs to the branch.
- **Put the control worktree under `/home/ubuntu`, never `/tmp`.** A control worktree needs the
  downloaded corpora and validators, and the only safe way to supply them is `cp -al`
  hardlink copies of `examples/pilot-corpora`, `examples/sysml-v2-training` and
  `build/pilot-{validator,sysml-validator,kerml-validator,evaluator,grammars,xpect-corpus}`.
  Symlinking the two `examples` roots makes the walker silently see **253** files instead of 355.
  But `/tmp` is a **tmpfs**, a different device, so `cp -al` there fails with
  `Invalid cross-device link` for every file — put the worktree on the same filesystem as the
  repo (`df -P` to check) and the hardlinks are free. Always gate on the control's own
  `355 file(s)` line before believing any control number; 253 means re-provision.
- **Attribute the rejection oracle the same way.** `docs/project/pilot-rejection-baseline.json`
  goes stale exactly like the differential one, and a merge of `main` into the branch can move
  it. At wave 11E both branch and control measured `120 / 116 both reject / 4 pilot-only`
  while the committed baseline still held `114 / 6` — the two moved cases were `main`'s
  metadata-evaluability work, not the branch's. Run `cmd/pilot-reject` on the control before
  crediting a rejection delta to the PR, and compare with
  `diff <(jq -S 'del(.validator,.pilot)' a) <(jq -S 'del(.validator,.pilot)' b)` so the
  volatile pin/validator fields do not mask the real comparison.
- **A net improvement can hide a per-file increase, and that is where the review value is.**
  Diff per-file entries keyed by `(root.name, file.path)` and print each file's summed only-ours
  `count` before/after. At 10D the net was −13 over 3 files, but
  `pilot-examples/Vehicle Example/Annex_A_VehicleViews.sysml` went 7 → 15: 5 recovery `syntax`
  errors disappeared and, because the tiers no longer short-circuit, 6 `unresolved-reference`
  errors + 1 `kind-mismatch` + 8 `unmapped` warnings appeared instead. Tier unblocking is the
  normal consequence of accepting new syntax, so expect it — but each newly revealed only-ours
  *error* is a fresh candidate false positive and should be named in the report even when the
  committed per-file ratchet (`internal/core/model/testdata/pilot_corpora_expected.txt`) already
  records the new number.
- `TestPilotDifferentialDocumentCountsMatchBaseline` reads only the *committed* baseline JSON, so
  it proves doc ↔ baseline consistency and cannot detect a committed baseline that no longer
  reproduces. A live harness run is still mandatory; both checks are needed.
- **Symlinked corpus roots are silently skipped.** The corpus walker does not follow symlinks, so a
  baseline worktree whose `examples/sysml-v2-training` or `examples/pilot-corpora` is a symlink into
  the real checkout drops that whole root from the report without warning — the file count simply
  comes out lower. Run the baseline as `go run ./cmd/pilot-diff -repo <real-checkout>` instead.
- **Silence can be a tier artifact, not a missing rule.** A type-tier error suppresses every
  constraint-tier pass for the whole file, so when a CLI run is silent, prove the rule is live with a
  positive control in the same file shape before concluding anything.
- **Guard scope:** mechanically guarded surfaces are:
  - every count in `docs/project/pilot-differential.md` and `README.md` against
    `pilot-differential-baseline.json` (the existing guard);
  - differential headlines and `openSysMLDiagnostics`/`pilotDiagnostics`/`severityMismatch`
    lines in every `.agents/skills/**/SKILL.md` against the same baseline (the new guard in
    `cmd/pilot-diff/w6f_skill_counts_test.go`);
  - the Totals block, per-kind table, per-suite table, and census prose in
    `docs/project/pilot-xpect.md` against `pilot-xpect-baseline.json`.
  - A live-looking headline is checked by default. `<!-- doc-count:historical -->` (optionally
    `<!-- doc-count:historical: reason -->`) exempts exactly the next matching claim in file order
    and is consumed; a marker with no following claim fails, so it cannot remain after its claim
    is deleted. The marker must sit outside inline code and fenced blocks — marker syntax inside
    them is documentation, not a marker.
  - **Trust by review:** live reproduction of either committed baseline (both guards read committed
    JSON only), the Xpect reconciliation table's `Published` column, and
    `docs/project/grammar-coverage-baseline.json` remain unguarded. Per-file corpus clean counts
    quoted in skills, prose `as of <wave> this is current` statements, tool-availability claims,
    and pinned paths also remain unguarded.
- Useful contrast to demo the change: three files where the importer sorts *before* the
  imported file (`a/Ref.sysml` importing `PkgB` from `b/Model.sysml`). The batch bridge is
  clean, exit 0; `build/pilot-validator/validate-sysml` on the same argv reports
  `Couldn't resolve reference to Namespace 'PkgB'` — order dependence, exit 1.
- `Duplicate of other owned member name` is **not** a wrapper artifact: it reproduces on a
  single file in isolation under both oracles (e.g. `testdata/passes/corpus_notation.sysml`
  lines 33/34, the `timeslice item item1` / `snapshot item item1` inside `item item1`), with
  or without `--root`, and its count does not grow with batch size. 25 warnings (summed `xK`
  multiplicities, over 7 files of `testdata` and `examples`) remain in the F6 report; 23 of
  them are collateral of the reference's own error recovery on notation it rejects, so a file
  with a pilot syntax error is a bad place to test the rule.

The harness compares OpenSysML diagnostics against the OMG SysML v2 Pilot Implementation
(via two pinned plain-Java bridges over the pilot's own validators) over four corpus roots and writes
`build/pilot-diff/pilot-diff.{txt,json}`. `docs/project/pilot-differential-baseline.json` is the
committed result of the *last refreshed* run, so **the harness is testable by reproduction** —
but only while the baseline is current. Check that first. As of the `2026-08` re-pin it **is**
current: a live run gives `371 file(s), 343 fully agreeing; 34 agreed, 37 only ours, 1105 only the
pilot's`, byte-identical to the committed baseline, and `docs/project/pilot-differential.md`'s
"Results" table matches. The rebaseline before it, at the architecture self-model's landing, covered two rounds, because the succession-shorthand
removal before it landed without refreshing the baseline; a control run of its merge commit gives
`32 agreed, 54 only ours, 79 only the pilot's`. When it is stale (it was at `ac4ac4fb`, and again while the F60–F69 fix
PRs were in flight), a non-empty `jq -S` baseline diff is *not* by itself evidence of a
regression — see "Isolating one change's effect" below.

## Prerequisites

- Go on PATH (`export PATH=/usr/local/go/bin:$PATH`), `java` (>= 21), `mvn`, `git`, `jq`.
- The OMG training corpus must be present (`./scripts/download-training-examples.sh`, in the repo
  blueprint's maintenance step) — without it the `training` root prints
  `skipping examples/sysml-v2-training: no .sysml files (corpus not downloaded?)` and the totals
  silently drop from 122 files to 22, which looks like a harness bug but is a missing corpus.
- The pinned jar: `./scripts/download-pilot-validator.sh`. It takes ~2-3 minutes cold (~18 s with
  a warm `~/.m2`; Maven downloads the pilot release ZIP and shades a jar) and needs network access
  to github.com and Maven Central. Once built it no-ops. Both bridges call it when the jar is
  absent, so provisioning either is enough.
- The SysML oracle: `./scripts/download-pilot-sysml-validator.sh` (needs `javac`). It compiles
  `scripts/pilot-sysml-validator/ValidateSysML.java` into
  `build/pilot-sysml-validator/validate-sysml-batch`, which is what `cmd/pilot-diff` runs by
  default. It batch-loads: every file of a root enters one resource set before any is validated.
  The older `build/pilot-validator/validate-sysml` (the DeciSym interactive CLI) is still built by
  the provisioning script and is still the handiest way to ask the reference about a single file.
- Provisioning checks for `download-pilot-sysml-validator.sh` that distinguish working from
  broken (all observed at `86514a44`):
  - no args → `SysML validator already compiled at .../classes/ValidateSysML.class` + `Built ...
    (pilot 2026-08, 0.62.0)`, `.class` mtime unchanged; `--force` → `Compiling ...`, mtime
    advances; `--bogus` → exit 1 `error: unknown option: --bogus (only --force is supported)`.
  - `rm -rf build/pilot-sysml-validator` and re-run → recreated in seconds, and the launcher is
    **byte-identical** with `pilot-pin.txt` = `sysml.release.tag=2026-08`
    / `sysml.artifact.version=0.62.0` and no `__PILOT_ARTIFACT_VERSION__` placeholder left.
  - the script always runs `download-pilot-validator.sh` first (no args → its `already built`
    fast path, ~1 s), so a re-pin that keeps the artifact version but changes the tag, commit,
    repository or wrapper commit still rebuilds the jar before anything compiles against it.
  - bad pin: move `build/pilot-validator` aside, then
    `PILOT_TAG=9999-99 PILOT_ARTIFACT_VERSION=9.9.9 ./scripts/download-pilot-sysml-validator.sh`
    → clones, prints `Downloading the pilot 9999-99 (9.9.9) release ...` (the pin is handed to Maven, so
    the wrapper's own `pom.xml` default never decides the release) and exits 1 when Maven's
    release download 404s; `build/pilot-sysml-validator/pilot-pin.txt` is left untouched.
  - missing tools need `--force` (the already-compiled early return is after the tool guards but
    before the compile): a stripped PATH without `javac` → `error: javac 21+ is required to build
    the SysML validator`; without `java` too → `error: Java 21+ is required ...`. The pinned
    build must be current for these (stamp and versioned jar present), otherwise the delegated
    `download-pilot-validator.sh` reaches its own tool guards first and you get
    `error: git is required to build the pilot validator` instead.
  - **Pitfall:** `mv build/pilot-validator /tmp/pv-aside` twice nests the backup
    (`/tmp/pv-aside/pilot-validator`), and restoring onto an existing directory nests it again as
    `build/pilot-validator/pv-aside`. Verify with `ls build/pilot-validator | tr '\n' ' '` — it
    must contain `target` and no stray backup directory.
- The KerML oracle: `./scripts/download-pilot-kerml-validator.sh` (needs `javac` too). It compiles
  `scripts/pilot-kerml-validator/ValidateKerML.java` against the pilot shaded jar into
  `build/pilot-kerml-validator/`, auto-provisioning the SysML validator first if the jar is
  missing. Seconds, not minutes — safe to `--force` rebuild during testing.

## The core check (fast, ~20 s per run)

```bash
rm -rf build/pilot-diff && go run ./cmd/pilot-diff        # ~19 s wall, ~1 min CPU
diff <(jq -S . docs/project/pilot-differential-baseline.json) \
     <(jq -S . build/pilot-diff/pilot-diff.json)          # must be empty
```

Run it twice and diff the two JSONs against each other to prove determinism (since F6 both sides
load a whole root before validating any of it, so there is no ordering left to perturb — but the
run order of roots and the EMF URI rewriting in the bridges still could). Observed at `3b2d14a3`: byte-identical across runs, and identical
after a full validator rebuild from scratch — that last one is the strongest evidence available,
because it shows the numbers are a property of the pinned pilot release, not of one local build.

Observed at `90da2cad` (KerML root added): <!-- doc-count:historical -->`338 file(s), 196 fully agreeing; 20 agreed, 851 only
ours, 145 only the pilot's`, wall time ~70 s (the KerML batch costs ~50 s), byte-identical to the
committed baseline and across runs.

Observed at `82ff0fac` (F34, per-file language dispatch): <!-- doc-count:historical -->`349 file(s), 222 fully agreeing; 20
agreed diagnostic(s), 564 only ours, 459 only the pilot's`, wall time ~82 s, byte-identical across
runs (both `.json` and `.txt`). The committed baseline is stale against this (338 / 221 / 20 / 560
/ 145), so use the entry-keyed delta below rather than `jq -S`.

When the baseline is stale, audit a change by comparing per-file *entries* keyed by
`(root.name, file.path)` — the whole entry value, not the whole file — so that added roots/files
and aggregate totals do not drown out the question you are asking (did any pre-existing verdict
move?). A run at `82ff0fac` gives 0 changed, 0 removed, 10 added
(`examples/parser_features_demo_*.kerml`). Files clean on both sides appear in no entry map at
all, so a newly-compared clean file shows up only as `filesFullyAgreeing +1`.

## Auditing a docs-only PR's numeric claims against the report

Adjudication PRs (e.g. #356, the S1–S10 / F60–F69 classes) assert per-root, per-category and
per-message counts. Derive every one of them from `build/pilot-diff/pilot-diff.txt`, never from
the doc. The JSON carries no per-diagnostic message, so the txt is the only source for message
and category counts. Parsing rules that matter:

- A root section starts with `<name> (<dir>)`; inside it a file block is `  <path>` and the
  diagnostic buckets are `    only OpenSysML (candidate false positives):`,
  `    only the pilot's ...`, `    agree...`, `    severity...`. Reset the current bucket on every
  new file line, or later files' pilot-only diagnostics leak into your only-ours totals.
- An entry line is `      line N  severity  category  xK` followed by **K** message lines. Count
  the K message lines, not the entry — otherwise multi-message lines (`x2`) undercount.
- Shortcut when a root has `pilotDiagnostics: 0` and `agreement: 0` (true for `pilot-examples`
  and `pilot-validation`): every `        opensysml:` line in that root's section is an only-ours
  diagnostic, so `grep -c` over the sliced section is an independent cross-check of the parser.
- **Agreement and severity-only must be counted by summing the `xK` multiplicities, not the
  message lines.** An agreement entry lists K `opensysml:` *and* K `pilot:` lines, so counting
  messages doubles it; the severity bucket's header is `same line and category, different
  severity:`, which a parser keyed on `only ...` silently mis-buckets.
- **Root file counts must come from `.roots[].totals`, not `len(.roots[].files)`** — the per-file
  array omits files both tools are silent on.
- Cross-check the doc's class table by summing its Files and Diags columns; they must equal the
  measured number of files carrying only-ours diagnostics (root `files` − `filesFullyAgreeing`)
  and the root only-ours totals.
- Cheapest whole-report cross-check: `grep -c '^        opensysml:' pilot-diff.txt` must equal
  `.totals.openSysMLDiagnostics` (317 at `0647cfe5`). If your parser disagrees with that, the
  parser is wrong, not the harness.
- Two claim shapes in these docs are almost always off by the same mistake, so recount them
  explicitly: (a) a per-file claim like "now reports 5 `unresolved reference: …`" is usually the
  number of *entry lines*, while the diagnostic count is the sum of `xK` (measured 6 at
  `0647cfe5` for `Analysis Examples/Vehicle Analysis Demo.sysml:214-218`, because line 214 carries
  `x2`); and (b) a per-root prose count that reads like "it contributed N only-ours **syntax**
  diagnostics … and M now" often quotes the root's only-ours *total* as M (22 for
  `pilot-validation`, whose syntax alone is 20). Quote which of the two you measured.
- An "`unmapped`, our side" tally can mean either the number of `.unmapped[]` *rows* (distinct
  messages) or the sum of their `.count`s — they differ (15 rows / 16 diagnostics at `0647cfe5`).
  The historical columns in `pilot-differential.md` use the diagnostic sum, so use
  `jq '[.unmapped[]|select(.side=="opensysml")|.count]|add'`.
- To attribute a movement claim to specific files, diff per-file entries against the previous
  committed baseline (`git show main:docs/project/pilot-differential-baseline.json`) keyed by
  `(root, path)`. This catches "the two renamed files were added to the `.sysml` side"-style
  causal claims where the renamed files actually contribute 0 and the whole delta is one other
  file. Renames show up as *removed* `.kerml` entries with no matching added `.sysml` entry when
  the renamed file is clean on both sides.
- A claim that a newly implemented rule "adds no only-ours diagnostic to any root" is checkable in
  one command: grep the report for that rule's message text (e.g. `flow end`, `evaluab`, `invoke`)
  under `opensysml:` and expect no hit.

Observed at `75672e91` (PR #356) — useful reference values: totals `338 / 221 / 20 agreed /
560 only ours / 145 only pilot`, byte-identical to the committed baseline; `pilot-examples` 314
and `pilot-validation` 59 = **373** over **73** of 154 files; categories syntax 274,
unresolved-reference 82, kind-mismatch 14, unmapped 2, units 1; and the two generic recovery
messages measured **102** `expected a body member` + **73** `expected a namespace member` = 175
(the doc claimed 74 + 64 = 138, so recovery-vs-finding splits in these docs are the claim most
likely to be stale — always recount).

## Single-file `-validate` is not the harness's loading model

The harness opens each corpus root as **one workspace batch** on both sides (see
`openSysMLDiagnostics` in `opensysml.go` and `pilotDiagnostics` in `pilot.go`), so a corpus file that imports a sibling
file — e.g. `Vehicle Example/VehicleIndividuals.sysml`, which does `private import
VehicleUsages::*` from `VehicleUsages.sysml` in the same directory — reports unresolved-reference
errors under a bare single-file `bin/sysml -validate <f>` even when the differential shows it
fully agreeing. Before calling such a file a failure, either pass the sibling files on the same
`-validate` command line or check the file's entry in `build/pilot-diff/pilot-diff.json`
(`.roots[].files[]` omits only files both tools are silent on; a fully-agreeing file with shared
diagnostics is still listed with empty `openSysMLOnly`/`pilotOnly`/`severityMismatch` buckets, so
treat absence — or all-empty disagreement buckets — as full agreement, confirming by comparing
against the same file's entry in `docs/project/pilot-differential-baseline.json`).

## Running a doc's inline reproducers through both tools

Adjudication rows quote a reproducer instead of committing a fixture, so re-derive them:
write the snippet to a temp `.sysml`, then `go build -o /tmp/sysml ./cmd/sysml &&
/tmp/sysml -validate <f>` for our side and `build/pilot-validator/validate-sysml <f>` for the
reference (capture `$?` without a pipe). A row that claims "ours" needs our message present *and*
the pilot silent; if the pilot also errors, the row is wrong.

Pitfall: the pilot's grammar requires a visibility keyword on imports, so a reproducer with
`import ISQ::*;` fails on the *pilot* side with `mismatched input 'import' expecting '}'` plus
cascading `Couldn't resolve reference to Type` errors — which looks like the reference rejecting
the construct under test. Always write `private import ISQ::*;` (what the corpora do) and put
library-dependent snippets inside a named `package`.

## Isolating one change's effect (works even with a stale baseline)

The reliable control is to run the *parent commit's* harness code over the *same* corpora and
diff the two JSONs:

```bash
git worktree add /tmp/wt-base <parent-sha>
(cd /tmp/wt-base && go run ./cmd/pilot-diff -repo /path/to/real/checkout -out /tmp/pd-base)
go run ./cmd/pilot-diff -out /tmp/pd-head
diff <(jq -S . /tmp/pd-base/pilot-diff.json) <(jq -S . /tmp/pd-head/pilot-diff.json)
```

`-repo <real checkout>` is what makes this work: it points the corpus roots (and the default
validator path) at the provisioned checkout. **Do not** try to symlink `examples/sysml-v2-training`
or `examples/pilot-corpora` into the worktree — the walker does not follow symlinks, so the roots
silently report `skipping ...: no .sysml files` and the file count drops (277 → 177), which looks
like a regression but is the missing corpus.

Check doc claims mechanically rather than by eye:

```bash
jq -c '.totals' build/pilot-diff/pilot-diff.json
jq -r '.roots[] | [.dir,.totals.files,.totals.filesFullyAgreeing,.totals.openSysMLDiagnostics,
        .totals.pilotDiagnostics,.totals.agreement,.totals.severityMismatch,
        .totals.openSysMLOnly,.totals.pilotOnly] | @tsv' build/pilot-diff/pilot-diff.json
jq -r '.unmapped[] | "\(.side)\t\(.count)\t\(.message)"' build/pilot-diff/pilot-diff.json
```

The JSON carries no message text per entry, so to check a *category* claim for one message
(e.g. "`Must invoke ...` is now `kind-mismatch`, not `unmapped`") read `pilot-diff.txt`: it lists
`line N  severity  category  xK` followed by the messages, grouped under the file path. Combine
both: the message must be absent from the JSON `unmapped[]` list *and* present under the new
category in the txt, with `totals.pilotOnly` unchanged (a category move must not create an
agreement).

## The KerML root and its bridge

`examples/pilot-corpora/kerml-examples` (58 `.kerml` files) is a root validated by
`build/pilot-kerml-validator/validate-kerml`, which drives the pilot's own `KerMLValidator`
through Xtext's `IResourceValidator` in **one** resource-set batch and prints GNU-format
diagnostics **relative to `--root`** (paths, not basenames — so no basename batching).
For the K6 `disjoint from` reproducer, the wrapper injects the library itself, so no
`--library` flag is needed: `validate-kerml --root /tmp/k6 /tmp/k6/Decl.kerml`.

Provisioning script checks that actually distinguish working from broken:

- Re-run with no args → `KerML validator already compiled at .../classes/ValidateKerML.class`,
  exit 0, and the `.class` mtime is unchanged. The **launcher is intentionally rewritten every
  run** (so a changed pin can never leave a stale jar path), so compare the `.class` mtime, not
  the launcher's.
- `--force` → prints `Compiling ...` and the `.class` mtime advances.
- `grep jupyter-sysml-kernel build/pilot-kerml-validator/validate-kerml` must show the pinned
  `PILOT_ARTIFACT_VERSION` from `scripts/pilot-pin.sh` (`0.62.0`) and no leftover
  `__PILOT_ARTIFACT_VERSION__` placeholder.
- `PILOT_ARTIFACT_VERSION=9.9.9-bogus ./scripts/download-pilot-kerml-validator.sh` → exit 1 with
  `error: pilot shaded jar not found at .../jupyter-sysml-kernel-9.9.9-bogus-all.jar`, and the
  existing launcher is left untouched (it does *not* silently reuse the stale jar). Note it first
  calls `download-pilot-validator.sh`, which no-ops in seconds when `build/pilot-validator` exists.
- `--bogus` → exit 1, `error: unknown option: --bogus (only --force is supported)`.

Oracle behaviour (`validate-kerml`, exit codes without a pipe):

| Input | Expected |
|---|---|
| a clean corpus file (`Address Book Example/AddressBookModel.kerml`) | exit 0, no `: error:` lines (only `log4j:WARN` noise on stderr) |
| a malformed `.kerml` | exit 1 with `<file>:<line>:<col>: error: no viable alternative at input ...` plus `Couldn't resolve reference to Type '...'` — the oracle must not be silent |
| no args | exit 2 + `usage: validate-kerml --library DIR [--root DIR] [--kernel-only] FILE...` |
| a `.sysml` file | exit 2 + `Error: File must have .kerml extension: <abs path>` |
| nonexistent file | exit 2 + `Error: File not found: <abs path>` |
| a directory | walks it for `.kerml` recursively and validates the batch |
| an empty directory | exit 0 + `Warning: No .kerml files found` |

Harness paths: `-kerml-validator /nonexistent` → exit 1,
`KerML pilot validator not found at /nonexistent: run ./scripts/download-pilot-kerml-validator.sh`,
no report written. The skip warning is language-aware (`no .kerml files` for that root). To
exercise a KerML-only run (e.g. a small `-timeout`), copy just
`examples/pilot-corpora/kerml-examples` into a temp dir and pass `-repo <tmp>` with absolute
`-validator`/`-kerml-validator`; otherwise the SysML roots time out first. Strongest
non-silence proof: drop a malformed `.kerml` into that copied corpus and confirm the JSON gains
pilot-side diagnostics for it (observed: 6 pilot-only + 1 agreed on that one file).

Since F34, language is a per-file property (`source.KindOf`), so a root collects both extensions
and runs one reference invocation per language over all of that language's files. stderr prints one
line per language per root (`testdata: 10 SysML file(s)` then `testdata: 1 KerML file(s)`), and our
own `.kerml` fixtures under `testdata/` and `examples/` are compared.

The control for a dispatch change: a synthetic repo with byte-identical `testdata/adv.sysml` and
`testdata/adv.kerml`, run at HEAD and in a parent worktree (`-repo` plus absolute validator flags).
The two files getting *different* pilot messages is the proof that two oracles ran; the parent
run reporting only `adv.sysml` is the proof the delta belongs to the change.

## Testing language-scoped (`.sysml` vs `.kerml`) diagnostic behaviour

Some checks are gated on the document's language via `source.KindOf(name)` (e.g. the KerML
type tier in `internal/core/passes/typecheck.go`). **Which surface you observe from decides
whether you see it at all**, because only some surfaces analyse under the real file name:

| Surface | Document name passed to `passes.Analyze` | Language honoured? |
|---|---|---|
| `cmd/pilot-diff` (`opensysml.go`, `ws.Open(rel, ...)`) | corpus-relative path with extension | **yes** |
| `sysml-lsp` / `sysml-grpc` (`internal/core/model/workspace.go`) | the opened file's URI/path | **yes** |
| `cmd/sysml -validate <file>` (`internal/repl/session.go`) | the real path — `session.go` branches on `source.KindOf(origin)` | **yes** (verified at `5ac8b6fb`) |
| `cmd/sysml` interactive REPL typing / stdin (`-`) | the constant `"<repl>"` | **no** — `KindUnknown`, so SysML rules |

The REPL caveat applies to text *typed into* the session (or piped on stdin), which lands in one
accumulated buffer named `<repl>` that `typecheck.go` deliberately reads as SysML. A file named on
the command line under `-validate` is **not** in that bucket: `internal/repl/session.go` (the
`source.KindOf(origin) == source.KindKerML` branch, line ~37 at `5ac8b6fb`) honours the extension,
so `bin/sysml -validate 'examples/pilot-corpora/kerml-examples/Simple Tests/Conjugation.kerml'`
**is** a valid, and by far the cheapest, KerML surface — it printed `no errors` / exit 0 there
while the same file under a reverted fix printed the KerML-only diagnostics. Re-check this branch
rather than trusting either claim blindly, but do not skip the CLI on the assumption it is
language-blind.

### Fixture-backed `internal/core/passes` tests can be silently vacuous

The shared helper `diagsIn` (`internal/core/passes/typecheck_kerml_language_test.go`) builds a bare
`symbols.NewIndex()` and loads **no standard library**. Because the passes are tiered, any fixture
that names a library type (`Base::Anything`, `Objects::Object`, …) collects `name-resolution`
errors, which **skip the type tier entirely** — so a test that asserts "zero `type` diagnostics"
over such a fixture passes no matter what the type checker does. Observed at `5ac8b6fb`:
`testdata/passes/f90_conjugation.kerml` yields 3 `unresolved reference: Base::Anything`
name-resolution diagnostics and 0 type diagnostics, and
`TestF90KerMLConjugationIsNotAPortTyping` therefore still **PASSED** with the fix reverted, while
its inline-source sibling `TestF90KerMLConjugationFormsAreClean` (short snippets naming no library
type) correctly failed with 5 errors.

So when validating a passes-layer fix:

- Never accept a fixture-file `t.Fatalf`-on-nonzero-`type`-diagnostics test as the regression
  detector. Dump **all** diagnostics (drop the `d.Source == diagSource` filter, or a throwaway
  `zz_probe_test.go` in that package) and confirm the fixture reaches the tier under test.
- Cross-check the same fixture through `bin/sysml -validate`, which *does* load the library, so the
  lower tiers are clean and the tier under test actually runs. That is what distinguished the two
  F90 tests above (8 rule errors under revert vs 0 at HEAD).
- Fixtures that need library types are best paired with a library-loading helper (see
  `f93_element_filter_scope_test.go`'s `f93LibraryDiags`, which loads `libs.DefaultSource()`).

Two cheap surfaces that *do* prove the split:

1. **A synthetic two-language mini-repo through pilot-diff.** Put byte-identical content in
   `<tmp>/testdata/adv.sysml` and `<tmp>/examples/pilot-corpora/kerml-examples/adv.kerml`, then
   `go run ./cmd/pilot-diff -repo <tmp> -validator <abs>/build/pilot-sysml-validator/validate-sysml-batch \
   -kerml-validator <abs>/build/pilot-kerml-validator/validate-kerml -out <tmp-out>`.
   The other five roots warn `skipping ...: no .sysml files` and are skipped, which is fine.
   Read `pilot-diff.txt`: the message must appear under `adv.sysml` and be absent under
   `adv.kerml`. Run the *same* command from the parent-revision worktree as the control —
   without it, an absent message proves nothing.
2. **A ~30-line stdio LSP driver.** `make build-lsp`, then spawn `bin/sysml-lsp`, send
   `initialize` (sleep ~2 s), `initialized`, and `textDocument/didOpen` with
   `uri: file:///tmp/x.kerml` (sleep ~4 s), and print the `textDocument/publishDiagnostics`
   payloads. `languageId` is irrelevant — the URI extension is what `KindOf` reads. This is the
   closest thing to the real editor-facing surface and takes seconds.

## The library index cache can hold *poisoned* records from an abandoned iteration

`internal/core/libs/record.go` invalidates on-disk records by a single integer, `formatVersion`,
and the record filename ends in `-v<N>.idx` under `$XDG_CACHE_HOME/sysml-ls/libs/`. The records
persist a symbol's **kind**, and for a cached library symbol (`sym.Decl == nil`) the runtime reads
that kind directly (`runtime/invoke_calc.go: isCalcSymbol`, `isActionSymbol`, …). Consequences when
testing a PR that changes how a library element is classified *and* bumps `formatVersion`:

- If any earlier build on the same machine already wrote records under the **same** version number
  (e.g. an abandoned iteration that classified `function` as `SymbolKerMLType` instead of
  `SymbolCalcDef`), the new binary happily reuses those records and misbehaves — observed as
  `sysml -constraint C model.sysml` → `not a calc: invalid symbol` for a library function call
  (`RealFunctions::sqrt`), while the same command in a fresh `XDG_CACHE_HOME` passes. Both caches
  contain the same file names and sizes, so `ls`/`stat` proves nothing.
- So always run the *same* command in (a) the ambient cache and (b) a fresh
  `XDG_CACHE_HOME=$(mktemp -d)`, and treat a difference as a cache-record problem, not a code bug.
  A green `go test ./...` will not catch it: tests use `t.TempDir()` caches.
- To find out *which* kind a record persisted, drop a throwaway `*_test.go` into
  `internal/core/libs` that `gob`-decodes each `*.idx` into `IndexRecord` and prints
  `symRecord.FQN` + `.Kind` (`symRecord` is unexported, so it must live in that package). Run it
  with `go test -v -run ... ./internal/core/libs` — plain `go test` swallows stdout. Delete the file
  afterwards and confirm `git status` is clean.
- Selective bisect: copy the cache aside and delete only `*-v<old>.idx` or only `*-v<new>.idx` to
  see which generation is responsible.
- Worth flagging to the author: a version bump only protects users who never ran an intermediate
  build of the same branch; if development churned through the same number, bumping once more is
  the cheap fix.
- Cross-check the harness too: re-run `go run ./cmd/pilot-diff` under a fresh `XDG_CACHE_HOME` and
  diff the JSON against the ambient-cache run (observed identical at `501d70fd`) — otherwise the
  differential numbers you reproduce may be a property of your cache.

## Refereeing a parser PR's accept/reject claims (over-acceptance is the real risk)

When a PR claims "form X now parses" *and* "neighbouring form Y stays rejected" (the shape
`docs/project/pilot-rejection.md` documents), neither half is provable from HEAD alone. Run every
scratch fixture through **three** surfaces and compare:

```bash
./bin/sysml -validate f.sysml; echo $?                  # HEAD
git worktree add /tmp/wt-base origin/main && (cd /tmp/wt-base && go build -o /tmp/sysml-main ./cmd/sysml)
/tmp/sysml-main -validate f.sysml; echo $?              # parent — proves acceptance is NEW
d=/tmp/ref && mkdir -p $d && cp f.sysml $d/
./build/pilot-sysml-validator/validate-sysml-batch --root $d $d/f.sysml; echo $?   # the oracle
```

- An accepted form is only *correctly* accepted when the pilot is also error-free on it. A form
  that HEAD accepts and the pilot rejects with `no viable alternative at input '…'` /
  `mismatched input '…'` is over-acceptance, regardless of what the PR's own docs assert.
  This caught a real 10D defect: `entry; then starting { … }` inside a `state def` body — the
  first row of the PR's own new "Forms kept rejected (W10D)" table — became **accepted** at HEAD
  (exit 0, and it survives a `-convert sysml` round trip) while `main` rejected it with
  `expected ';' after succession edge` and the pilot rejects it outright. Root cause shape: the
  body was added inside the shared `parseSuccessionEdge`, which state bodies reach for entry
  transitions too, so an `ActionTargetSuccession`-only body leaks into `EntryTransitionMember`
  (fixed in #462 by making the body an explicit per-call-site policy).
  **Always test every row of a "kept rejected" table**, not just the accepted forms — the negative
  test suite may deliberately not cover them yet (10D deferred them to "10G").
- **`exit 0` from `-validate` is "analyses clean", not "parses clean"**, and the two diverge on
  purpose when a parser change lands ahead of its passes readers. Separate them: `-convert sysml`
  (an AST round trip) is the parse-level surface and exits 0 while `-validate` exits 2. At 10D,
  `verify a.b { … }` round-tripped fine but `-validate` reported
  `Must be an accessible feature (use dot notation for nesting)` — a false positive, since the
  pilot accepts the same file. Isolate such an error from pre-existing ones with a one-edit
  control: `verify` outside an `objective { … }` errors with `A requirement verification must be
  in the objective of a verification case.` on the *plain* form too, so wrap it in an `objective`
  before blaming the chain.
- `-convert sysml` is also the cheapest structural assertion for a new AST field: it prints
  `@Meta about a, b;` and the bodied `then target { … }` verbatim, so a dropped `About`/`Members`
  is visible without a golden. It cannot distinguish `first a.b then c.d` as SuccessionAsUsage
  from InitialNode (both print identically) — for that read the committed `.golden`
  (`Usage kind="succession"` with two `FeatureChainExpr` ends vs `(InitialNode …)`).
- Golden fixtures under `internal/core/parser/testdata/parse/` are only non-vacuous if the parent
  binary *rejects* the same input; confirm that with `/tmp/sysml-main` rather than assuming it.

## Running the pilot validator directly

`build/pilot-validator/validate-sysml <file.sysml>` prints diagnostics on **stderr** in GNU
format (`<basename>:<line>:<col>: severity: message`) and library `Reading ...` noise on stdout.
Exit 1 means "the batch had errors"; anything else is the validator itself failing. **Capture the
exit code without a pipe** (`... > /tmp/out 2>&1; echo $?`, or `PIPESTATUS`), otherwise a pipeline
reports the exit of the last stage and a failure looks like a pass.

## Adversarial paths that actually distinguish working from broken

| Case | Expected at `3b2d14a3` |
|---|---|
| `-validator /nonexistent` | exit 1, `pilot validator not found at /nonexistent: run ./scripts/download-pilot-validator.sh` |
| `-out /tmp/pd-out` | exit 0, both reports written there, contents identical to the baseline |
| `-repo /tmp/notrepo` (default validator) | exit 1, validator-not-found under *that* repo |
| `-repo /tmp/notrepo -validator <abs path>` | exit **0**, one `skipping <root>: no .sysml files` warning per root, `files: 0` — warned but not fatal; worth flagging if a caller could mistake it for a clean run |
| `-timeout 1s` | exit 1, `... validate-sysml failed (signal: killed)`, no report written (the JVM needs ~10 s just to load the library) |
| `-repo <empty dir>` | exit 1, seven language-aware skip warnings then `no model files found under <dir>` |

Provisioning script (`scripts/download-pilot-validator.sh`):

- Already built → prints `Pilot validator already built at ... (pilot <tag>, <version>)` and exits
  0. Prove it did not rebuild by comparing
  `ls -l --time-style=full-iso build/pilot-validator/validate-sysml` before and after, not just by
  reading the message.
- Stale build → the early return is taken only when `build/pilot-validator/.pilot-pin` holds the
  current pin (tag, commit, repository, artifact version, wrapper commit) *and* the build is
  complete: `validate-sysml`, the wrapper jar, the versioned shaded jar
  `target/sysml-download/sysml/jupyter-sysml-kernel-<version>-all.jar` and `sysml.library` all
  present. Overwrite the stamp (`printf x > build/pilot-validator/.pilot-pin`) and the script
  prints `Stale build at ...: built from x, pin is now ...; rebuilding.`; delete it and it prints
  `Unstamped build at ...; rebuilding ...`; `rm -rf` only `sysml.library` and it prints
  `Incomplete build at ...; rebuilding ...`. All three rebuild for real (~20 s with a warm `~/.m2`,
  minutes cold), so `timeout 5` the run if the message is all you need — the build happens in a
  sibling `build/pilot-validator.build.XXXXXX` directory that is swapped in only once complete, so
  an interrupted or failed rebuild leaves the installed validator untouched (verify the
  `validate-sysml` mtime and that `ls build | grep pilot-validator` shows no leftover stage).
- Pin propagation → with a good build in place,
  `PILOT_TAG=9999-99 PILOT_ARTIFACT_VERSION=9.9.9 ./scripts/download-pilot-validator.sh`: it
  prints `Stale build at ...`, clones (fast), prints `Downloading the pilot 9999-99 (9.9.9)
  release ...` and Maven's `download-maven-plugin` fails with `Download failed with code 404`.
  Exit 1, and the `2026-08` validator is still there (`.pilot-pin` unchanged). The tag and version
  reach Maven as `-Dsysml.release.tag` / `-Dsysml.artifact.version`, overriding the wrapper's
  `pom.xml` defaults, so a wrapper commit pinned to an older release still builds against ours.
  The script aborts *before* Maven only if the wrapper's `pom.xml` stopped declaring those two
  properties (`error: <commit> no longer selects the pilot release through the sysml.release.tag
  property`).
- Missing tools → `env PATH=/tmp/nomvn ./scripts/download-pilot-validator.sh` where `/tmp/nomvn`
  holds symlinks to `git`, `java`, `sed`, `bash`, `dirname`, `ls` but **not** `mvn` gives
  `error: mvn is required to build the pilot validator`. If you forget `dirname`, the script
  first emits `line 16: dirname: command not found` — an artifact of the stripped PATH, not a bug.

**Avoid `rm -rf build/pilot-validator` while testing** — `mv` it aside and move it back if the
Maven cache is cold, since a from-scratch rebuild costs minutes. Watch out for the classic
`mv /tmp/pv-backup build/pilot-validator` when the target directory already exists again: that
nests the backup at `build/pilot-validator/pv-backup` instead of restoring it. `rm -rf` the *new*
directory first (or restore to a fresh path) and verify with `ls build/pilot-validator | tr '\n' ' '`.

## The optional SysIDE third column (F7)

`./scripts/download-syside.sh` builds Sensmetry SysIDE (`sensmetry/sysml-2ls`, pinned `0.9.1`,
`2024-12` standard library) into `build/syside/`, and `cmd/pilot-diff` picks
`build/syside/validate-syside` up automatically. Needs `node` (18+) and `pnpm`; ~15 s from a warm
pnpm store, ~2 min cold. It is **static only** — SysIDE executes nothing, so it is never evidence
about behavioral rows — and it never adjudicates: the two-way buckets and totals are byte-identical
either way.

The two checks that actually distinguish working from broken:

```bash
mv build/syside /tmp/syside-aside && go run ./cmd/pilot-diff -out /tmp/pd-two-way
diff <(jq -S . docs/project/pilot-differential-baseline.json) \
     <(jq -S . /tmp/pd-two-way/pilot-diff.json)             # must be empty (no third column)
mv /tmp/syside-aside build/syside && go run ./cmd/pilot-diff -out /tmp/pd-three-way
jq '.totals, .syside.totals' /tmp/pd-three-way/pilot-diff.json
```

The three-way run costs ~2m50s (SysIDE reloads its standard library per root). The `.syside` keys
are additive: `.syside.totals`, `.roots[].syside`, `.roots[].files[].syside.entries[]` with a
`sides` label (`opensysml+pilot+syside`, `opensysml+syside`, …). Observed at `b570dce8`: two-way
totals unchanged, `349 files, 248 where all three agree exactly, 690 syside diagnostics, allThree
20, withOpenSysMLAgainstPilot 7, withPilotAgainstOpenSysML 37`, byte-identical across runs.

- Entries match on `(line, severity, category)`, not on message — the same tuple-matching the
  two-way comparison uses. So read the verbatim messages under a row in `pilot-diff.txt` before
  calling it corroboration; a coincidental line match happens (`Simple Tests/StateTest.sysml:21`
  pairs our `unresolved member: s` with SysIDE's `Could not resolve reference to Feature named
  'new'`).
- `-syside <path>` overrides the launcher and is **fatal** when missing (a typo must not silently
  degrade to two columns); the default path merely warns
  `comparing against the pilot only; run ./scripts/download-syside.sh for a third column`.
- Pin checks: `SYSIDE_TAG=0.0.0-nope`, `SYSIDE_SPEC=2026-05` and `SYSIDE_STDLIB_BRANCH=release/nope`
  each exit 1 with a specific message and leave `build/syside` untouched (everything is staged in
  a `mktemp -d`). Verify "untouched" by `sha256sum` + mtime of `validate-syside`/`syside-pin.txt`,
  not by the directory listing alone.
- **Any provisioning-guard check needs `--force`**: without it the `already built` early return
  fires before the `git/node/pnpm` and pin checks, so `env PATH=/tmp/notools
  ./scripts/download-syside.sh` exits 0 and proves nothing. With `--force` a stripped PATH gives
  `error: pnpm is required to build SysIDE` (or `node`) plus the Node-18 hint. Build the stripped
  PATH with `ln -s "$(/usr/bin/which git)"` — `command -v git` can return a bare `git` and leave a
  dangling symlink.

### Additivity is per-entry, not byte-for-byte

`jq 'del(..|.syside?)'` on a three-way report does **not** equal the two-way report, and that is by
design (`attachSyside`): files both implementations are silent on but SysIDE is not get appended to
`roots[].files` (25 such files at `286f420f`), and SysIDE's unrecognised messages are appended to
`unmapped[]` with `"side": "syside"`. The checks that do hold exactly, and are the ones to run:

```bash
diff <(jq -S '.totals' two/pilot-diff.json)  <(jq -S '.totals' three/pilot-diff.json)
diff <(jq -S '[.roots[]|{name,totals}]' two/…) <(jq -S '[.roots[]|{name,totals}]' three/…)
diff <(jq -S '[.unmapped[]|select(.side!="syside")]' two/…) <(… three/…)
# every file entry with a non-empty two-way bucket, keyed by root|path, must be identical:
jq -S '[.roots[]|.name as $r|.files[]|{k:($r+"|"+.path),agreement,severityMismatch,openSysMLOnly,pilotOnly}]'
```

### Adversarial `-syside` launcher behaviour (all observed at `286f420f`)

| Fake launcher | Result |
|---|---|
| non-executable (`chmod 000`) | exit 1, `start <path>: fork/exec …: permission denied`, no report |
| exits 2 with a message | exit 1, `<path> failed (exit status 2); stderr:` + the message, no report |
| prints a GNU line for a file not in the batch | exit 0, but `pilot output not attributable to a corpus file: …` on stderr (not dropped) |
| launcher dir without `syside-pin.txt` | exit 1, `read the SysIDE pin: open …/syside-pin.txt: no such file` |
| exits 0 printing nothing | exit **0**, `sysideDiagnostics: 0`, our findings land in `openSysMLOnlyUncorroborated` — no false corroboration, but the harness cannot tell "SysIDE clean" from "SysIDE did nothing". Worth flagging, not a bug. |

To time out *SysIDE only* (the real pilot and SysIDE both take ~10 s, so a small `-timeout` kills
the pilot first): fake the pilot with a script that answers `--version` and exits 0, `cp
build/pilot-validator/pom.xml` next to it (`pilotVersion` reads `<dir>/pom.xml`), point
`-syside` at a `sleep 30` launcher and use `-timeout 3s` → exit 1,
`… failed (signal: killed)`, no report. Fast iteration for all of these: `-repo /tmp/mini` with a
copy of `cmd/pilot-diff/testdata` only (4 files, other roots just warn `skipping`).

Timings at `286f420f` (8 vCPU): two-way `1m14s`, three-way `2m44s`, SysIDE alone on 4 files ~11 s.
SysIDE prints `Collected standard library: [...]` on **stdout**; the harness discards stdout, so
only stderr matters.

## Refereeing the name-distinguishability rule (`internal/core/resolve/distinguishability.go`)

`build/pilot-validator/validate-sysml <one file>` is a faithful oracle for this rule — the four
messages reproduce on a single file with no `--root` — so hand fixtures are the right surface, and
`cmd/pilot-diff` is *not*: a corpus-scale scan at `2836471c` found 0 only-ours and 23 pilot-only
`Duplicate of …` diagnostics, and **all 23 sit on a line where the pilot itself emits a syntax
error** (`namespace` in `.sysml`, imports without visibility), i.e. recovery collateral. So the
harness cannot see gaps in this rule; only fixtures can.

Reference wording to match verbatim (all `warning:`): `Duplicate of other owned member name`
(owned-vs-owned), `Duplicate of owned member name` (alias-vs-owned), `Duplicate of other alias
name` (alias-vs-alias), `Duplicate of inherited member name '<n>' from <A>, <B>` (inherited).
`bin/sysml -validate` exits 0 and prints `✓ …: no errors` when duplicates are the only findings —
that exit code is the cheapest proof the rule is a warning, not an error.

The fixture shapes that actually distinguish working from broken (a "silent both sides" fixture
proves nothing on its own — always build the *positive* twin one edit away):

| Shape | Reference verdict |
|---|---|
| `part def L1 {attribute p;} part def R1 {attribute p;} part def D2 :> L1, R1;` | warns **on D2's declaration line**, `'p' from L1, R1` (inherited-vs-inherited diamond) |
| `action def L {in p;} action def R {in p;} action def D :> L, R;` | warns the same way — a parameter is not exempt when the two owners are unrelated |
| `A{in p}` / `Outer{a : A {in p}}` / `Sub :> Outer {:>> a : A}` | silent — the inner parameter implicitly redefines `A::p`, so the name arrives once |
| `part def Q { attribute portions; }` | warns `'portions' from Occurrence` — an inherited **library** member counts (OpenSysML does not walk library supertypes; known limitation) |
| `B{p}` / `M :> B {:>> p}` / `D :> M {p}` | warns `'p' from M` — a member that redefines is still inherited by *its* subtypes |
| `:>>` in the immediate subtype, or `q :> p`, or `alias X for X`, or `bind u = v;`, or a local name shadowing a `private import Q::*` | silent |

Pitfalls: write binding fixtures as `bind a = b;` — `binding b of u = v;` and `binding b = u;` are
OpenSysML extensions the reference rejects syntactically, and its recovery then emits duplicate
warnings of its own, which looks like the rule under test. Build the merge-base binary
(`git worktree add /tmp/wt-main <parent>; go build -o /tmp/sysml-main ./cmd/sysml`) and run every
fixture through it too: a severity change (error → warning) and a *coverage* loss look identical
from HEAD alone, and the second is the risk when a rule is rewritten to match a reference.

## Proving a new negative test is load-bearing (mutation control)

When a parser fix adds `TestNegative` rows for forms that must stay rejected, a passing row proves
nothing on its own — the input may be rejected by an unrelated earlier error. Flip the guard the
fix introduced (e.g. `if allowBody && p.accept2(lexer.LBrace)` → `if p.accept2(lexer.LBrace)`),
rerun `go test ./internal/core/parser -run TestNegative`, and check *which* rows fail. Rows that
still pass under the mutation are guarding a different code path (a package-level `then` is caught
by `expected a namespace member` before it ever reaches `parseSuccessionEdge`), which is worth
saying out loud rather than claiming all rows guard the new guard. Restore from a `cp` backup and
re-run before reporting, and never let a long `go test ./...` overlap the mutation — if it does,
the run is untrustworthy and must be repeated on the restored tree.

Where a guard is scoped by parser *position* (a flag that flips after some member), also probe the
over-rejection direction: build the file where the form appears *later* in the same body and see
whether the pilot accepts it. If the pilot rejects the whole file for an unrelated reason (it
rejects `entry;`/`entry action a;` inside `state def` bodies, for instance), say the case could not
be discriminated instead of scoring it a pass.

## Recording

This is CLI work: record a maximized Konsole on `DISPLAY=:0` (see the "Recording setup" section
of `testing-sysml-repl/SKILL.md`). A single `go run ./cmd/pilot-diff` prints only four progress
lines and a summary, so pair every run with the `jq`/`diff` command that turns it into a visible
pass/fail line (`&& echo '... IDENTICAL'`), otherwise the video shows nothing checkable.

## Devin Secrets Needed

None. Network access to `github.com` and Maven Central is required only for re-provisioning.
