---
name: testing-doc-counts
description: How to end-to-end test the generated documentation figures (cmd/doc-counts + internal/doccounts + `make docs-counts`, and the build-time suite figures of scripts/mkdocs_suite_figures.py) on Linux — proving `-check` is a real gate, that the block consumers cannot drift, that marker mutations fail loudly, that the site renders the tree's figures, and that no measured number moved.
---

# Testing the generated documentation figures (`cmd/doc-counts`)

`cmd/doc-counts` regenerates three kinds of derived documentation:

1. single-copy baseline lines in `README.md` (`**Reference differential:**`, `**Rejection oracle:**`),
   from the committed baselines;
2. the HTML-comment-delimited named block `<!-- doc-counts:begin refereed-figures -->` …
   `<!-- doc-counts:end refereed-figures -->`, rendered from **one** template in
   `internal/doccounts/doccounts.go` into **two** consumers (`README.md` and
   `docs/internals/architecture.md`), differing only by `Block.LinkPrefix`
   (`docs/project/` vs `../project/`);
3. the README's `**Behavioral execution:**` figure, the **inline** block
   `<!-- doc-counts:begin conformance-passing -->every conformance case passing<!-- doc-counts:end conformance-passing -->`
   — the one suite figure still committed, because it moves only with `known_failures.txt`.

The rest of the test-suite figures are **site blocks** (`doccounts.SiteBlocks()`, named in
`siteSuiteBlocks` in `internal/doccounts/suite_blocks.go`): the compliance map's `**Test Coverage:**`
inventory (`inventory-conformance`, `-robustness`, `-runtime-tests`, `-golden-asts`, `-traces`,
`-negatives`, `-grpc`, `-tests`) and the LSP `**Measured coverage:**` line (`lsp-tests`). In git each
holds a sentence naming what is counted and **no digit**; `go run ./cmd/doc-counts -check` refuses
one that states a figure (`the block named "inventory-robustness" states a figure`). The figures
are rendered when the site builds: `go run ./cmd/doc-counts -site-blocks` prints
`{"docs/project/spec-compliance.md": {"inventory-robustness": "470 runtime robustness cases (…)", …}}`
and `scripts/mkdocs_suite_figures.py` (an `on_pre_build` + `on_page_markdown` hook in
`mkdocs.yml`) splices the text into the blocks, dropping the markers. Their inputs are the
**tree**, read by `doccounts.ReadSuiteCounts` the way the gates enumerate them: `internal/fixtures`
lists the conformance cases (the same package `TestExecutionConformance` and the gRPC conformance
gate iterate), the parse and trace goldens are stat'ed against the case that owns them, and the
robustness, negative and `Test`-function figures are counted from the `_test.go` files with
`go/ast` (first-level `t.Run` calls across every `TestRuntimeRobustness*` / `TestGRPCRobustness*`
function, multiplied out over the table literal a `range` walks, read in statement order and
lexical scope, so a table rebound after the loop or shadowed by a `:=` in an inner block, branch or
clause does not leak into it). The test and subtest total of a run is **not** generated — only a
run can state it, so the prose does not quote one.

A third consumer, `<!-- doc-counts:begin analysis-libraries -->` in `docs/project/spec-compliance.md`,
renders the per-library table from `docs/project/analysis-library-census.json`, which
`TestAnalysisLibraryCensus` (`internal/core/runtime/library_census_test.go`) writes under
`-update-library-census` and otherwise asserts. Its inputs are `doccounts.ReadFigures`
(the refereed baselines plus the census); the same stale/marker/read-only checks below apply to it,
and a census JSON mutated by hand (a declaration dropped from `evaluated`) must fail both `-check`
(`has 0 verdicts, want 1`) and the runtime test.

The compliance map's own row census (`The map below tracks N semantic rules: …`) is **not** committed
anywhere: `scripts/mkdocs_census.py` counts it from the rows and fills the
`<!-- doc-counts:begin census -->` block in `docs/project/spec-compliance.md` while the site builds
(`make docs`). `doc-counts` and the `cmd/pilot-diff` guard only refuse a `🚧` row. Test the hook with
`python3 scripts/mkdocs_census-test.py`, and prove it live by grepping the built
`site/project/spec-compliance/index.html` for `semantic rules:` after adding a row.

The documentation site's landing band (`overrides/home.html`) is **not** a consumer: it names the
four oracles and links to their records without quoting a figure, so it is hand-written markup that
never goes stale. Its record links are `{{ record('project/x.md', base_url) }}`, the global
`scripts/mkdocs_landing.py` installs: it resolves to the page when the site publishes that record
and to the file on GitHub when it does not, the way `scripts/mkdocs_repo_links.py` resolves such a
link in Markdown. Keep it that way — a number typed into the band is exactly the drift `doc-counts`
exists to prevent.

**Know which records the site publishes before writing a landing link.** `mkdocs.yml`'s
`exclude_docs` keeps the engineering records in the repository rather than on the site: as of
this writing `omg-issues`, `pilot-xpect`, `pilot-rejection`, `pilot-corpora`,
`pilot-execution-referee`, `grammar-coverage`, `training-examples` and `wave*` are **not
published**; only `spec-compliance`, `pilot-differential`, `roadmap` and `project/README`
are. A `|url` filter naming an unpublished page is a hard 404 — under MkDocs 1.6
`exclude_docs` leaves the page *in* `files` with `inclusion.is_included() == False`, so the
hook's presence test asks for publication, not mere existence, and `record()` is what a link
to a possibly-unpublished record must use. `scripts/check-doc-links.py` only walks Markdown,
so it never sees `overrides/*.html`; the hook is the only guard, and both of its warnings
(`which no page publishes`, `which does not exist`) fail `--strict`.

Inputs to the refereed figures are the three committed baselines
`docs/project/pilot-{differential,xpect,rejection}-baseline.json` (`doccounts.ReadRefereedCounts`);
`docs/project/spec-compliance.md` is read to refuse a `🚧` row and, since it carries the inventory
blocks, is also a consumer.

`make docs-counts` = generate → `go run ./cmd/doc-counts -check` → `go run ./cmd/validation-census
-check` → `go test -count=1 ./cmd/pilot-diff ./cmd/pilot-reject ./cmd/doc-counts
./cmd/validation-census`.

Adding a test or fixture anywhere in the module moves a site figure and **nothing committed**:
`-check` stays `already current`, `-site-blocks` and the built site change. Only a baseline, the
library census or `known_failures.txt` moving makes `TestCheckCommittedTreeIsCurrent` fail until
`make docs-counts` runs. That is by design — CI runs `-check` and builds the site with `--strict`.

## Never test in a checkout someone else is using

Use `git worktree add /home/ubuntu/wt-<name> <sha>`. Put it **under `/home/ubuntu`, not `/tmp`**
(tmpfs is a different device, so the `cp -al` hardlink provisioning below fails there), then
hardlink-copy the gitignored provisioning in from a provisioned checkout — this takes seconds and
costs no disk:

```bash
mkdir -p /home/ubuntu/wt-x/build
for d in pilot-validator pilot-sysml-validator pilot-kerml-validator pilot-xpect-corpus pilot-evaluator; do
  cp -al /home/ubuntu/repos/OpenSysML/build/$d /home/ubuntu/wt-x/build/$d; done
cp -al /home/ubuntu/repos/OpenSysML/examples/sysml-v2-training /home/ubuntu/wt-x/examples/
# examples/pilot-corpora already exists in a worktree: stage the hardlink copy aside and swap it in
```
Copy **all** `build/pilot-*` dirs together: the validator launchers resolve the shaded jar through
`$SCRIPT_DIR/../pilot-validator/...`, so copying one alone gives the silent-zero
`jar not found` run that still exits 0 (see `testing-pilot-differential`).

## The checks that actually distinguish working from broken

- **Idempotence:** `make docs-counts` twice; both must print `doc-counts: already current` for the
  generate *and* the `-check` step, and `git status --short` must stay empty.
- **`-check` is a gate, not decoration:** perturb one number *inside* the block in one consumer.
  `go run ./cmd/doc-counts -check` must exit **1**, print `doc-counts: <that path> is stale` plus a
  `--- <path> (current) / +++ <path> (generated)` diff with `@@ line N @@` hunks, name **only** that
  file, and leave the file's `sha256sum` unchanged. Then the plain generator must restore it
  byte-identically to the committed hash.
- **Read-only `-check`:** `chmod 444` the stale file — `-check` must still exit 1 with the same
  stale message (it must not attempt a write). The plain generator on the same read-only file must
  exit 1 with `open <path>: permission denied` and write nothing (writability of *all* pending files
  is checked before any is written).
- **Baseline propagation:** mutate one figure in each baseline JSON in turn; `-check` must name
  **every** consumer stating it — both Markdown pages — the guard tests must fail while the tree is stale
  (`TestCheckCommittedTreeIsCurrent`, `TestPilotDifferentialDocumentCountsMatchBaseline`,
  `TestW6FXpectDocumentCountsMatchBaseline`, `TestPilotRejectionDocumentCountsMatchBaseline`), and
  regeneration must write the new number into every one of them. Restore with `git checkout -- .`.
  **Pick a figure that is not cross-constrained.** Mutating the xpect `errors` kind's `rows`
  (510→509) makes `Silent = rows - agree - sameLocation - sameLine - severityDiffers -
  elsewhereInFile` negative, so the tool correctly *errors*
  (`errors agreements and tolerances exceed 509 rows`) instead of propagating. To exercise
  propagation use the `scope` kind's `agree`, the differential's `totals.filesFullyAgreeing`, or
  the rejection baseline's `totals.bothReject`.
- **Marker mutations must fail loudly:** delete the end marker, duplicate the begin marker, delete
  the whole block. Each must make *both* the generator and `-check` exit 1 with
  `named block "refereed-figures" is missing or unterminated` or
  `duplicate "<!-- doc-counts:begin refereed-figures -->" marker`, and `wc -c` on the file must be
  unchanged (no truncation, no `already current`). For an inline block, also: put the end marker
  before the begin marker on the line (`ends before it begins`), repeat the pair on one line or
  add a second copy on a line of its own (`duplicate markers of the block named`), and drop the
  end marker (`missing or unterminated`).
- **Tree propagation goes to the site, not to git:** drop a `state_probe.expected.json` into
  `internal/core/runtime/testdata/conformance/`, or a
  `robustness_zz_probe_test.go` with a two-subtest `TestRuntimeRobustnessProbe` into
  `internal/core/runtime/`, or a `TestSomething` into any `_test.go`: `-check` must still print
  `already current` and `git status --short` must show only the probe, while `-site-blocks`
  moves the matching figure (`state×228` → `state×229`, `470 runtime robustness cases` → `472`,
  the `Test`-function figure by one) and `make docs` renders the new number into
  `site/project/spec-compliance/index.html`. `go test -run TestRuntimeRobustnessProbe -v` must run
  both subtests — the counter follows Go discovery, not the other way round. List a real case in
  `known_failures.txt`; now the **committed** README block goes stale (`every conformance case
  passing` → `1 listed in known_failures.txt`) and `-check` must name `README.md`. Remove the
  probes afterwards (`git status --short` must be empty again).
- **A typed figure in a site block is refused:** put a digit inside any `inventory-*` or
  `lsp-tests` block; `-check` must exit 1 naming the block and leave the file unchanged. Break a
  site block's marker (drop the end marker, duplicate the pair): `-check` fails, and `make docs`
  must abort under `--strict` with the hook's warning rather than publish the placeholder
  sentence. Hide `go` from `PATH`: `make docs` must abort with `go: not found; the test-suite
  figures need the Go toolchain`, and `python3 scripts/mkdocs_suite_figures-test.py` must fail
  its real-tree case (not skip it).
- **The site shows the tree's figures:** after `make docs`, grep
  `site/project/spec-compliance/index.html` for `runtime robustness cases` and
  `functions in <code>internal/lsp`; each must carry a number, no `doc-counts:begin inventory-`
  or `lsp-tests` marker may remain (only the committed `analysis-libraries` markers do), and
  none of the placeholder sentences (`the runtime robustness cases`) may be visible. Open the
  served page in a browser to confirm the inventory reads naturally with the numbers spliced
  into the sentences.
- **The counters refuse what they cannot count:** a `.trace.golden` owned by no case, a
  `<case>.typo.trace.golden` under no sweep policy, a `<case>.declared.trace.golden` of a case
  with no `outcomes` (or no default golden), a `.sysml` under `testdata/parse/` with no `.golden`,
  a `known_failures.txt` entry naming no case, a `for i := 0; i < n; i++ { t.Run(...) }` loop,
  a `range` over a table the function `append`s to or rebinds under a condition before the loop, or
  an `if cond { t.Run(...) }` in `TestRuntimeRobustness` must each make the generator and `-check`
  exit 1 naming the file, rather than print a smaller (or larger) number. A `range` or `if` that
  runs no subtest is passed over, and so are the goldens of a case `known_failures.txt` lists,
  since `TestExecutionTrace` skips the case.
- **The figures are the gates' figures** (read them from `-site-blocks`): `go test -count=1 -v
  -run 'TestExecutionConformance$' ./internal/core/runtime | grep -cE '^=== RUN   TestExecutionConformance/[^/]+$'`
  must equal the conformance figure; the same shape with `TestRuntimeRobustness` and
  `TestGRPCRobustness` (in `./internal/grpc`) — unanchored, summing the first-level `=== RUN`
  lines of every function the prefix matches — `TestGolden$` and `Negative` (in
  `./internal/core/parser`, summing per function) must equal theirs; and
  `go test -list '.*' ./... | grep -c '^Test'` must equal the `Test`-function figure. Test names
  carry digits (`TestF62F63Negative`), so match `[^/ ]+`, not `[A-Za-z_]+`.
- **Every landing link resolves on the built site:** grep the `href`s out of
  `/tmp/site/index.html` and check each one — a site-relative target must exist under
  `/tmp/site`, a repository target must exist under `docs/` — then click them in a browser
  against a served copy (`python3 -m http.server -d /tmp/site`; `file://` breaks directory
  URLs). Prove the guard is live: a `record()` link to a page that does not exist must abort
  `--strict`; a `record()` link to an excluded page builds clean by design (it resolves to the
  GitHub blob URL) — only a `{{ '…'|url }}` link to an excluded page aborts.
- **The landing band quotes no figure:** `grep -E '[0-9]+ of [0-9]+' overrides/home.html` must
  match nothing, and the rendered `site/index.html` must carry no literal `{{ … }}` from a Jinja
  mistake in the band's record links.
- **The two Markdown consumers cannot drift:** extract each block with
  `sed -n '/doc-counts:begin refereed-figures/,/doc-counts:end refereed-figures/p'`, normalise
  `(../project/` → `(docs/project/` in the architecture copy, and `diff` — must be empty. Then
  `test -f` all four link targets from each consumer's own directory.
- **No measured number moved:** compare digit *sequences*, not the files:
  `git show main:README.md | grep -o '[0-9][0-9]*'` vs the same on HEAD, `diff` must be empty (same
  for `docs/internals/architecture.md`), and `git diff main -- 'docs/project/pilot-*-baseline.json'`
  must be empty. This is the cheapest proof a "generate it instead of hand-maintaining it" refactor
  restated exactly what was there. The site figures are not in either file, so compare them
  against the gates instead (previous bullet): a figure `-site-blocks` prints that the matching
  `go test -v` enumeration does not reproduce is a counting bug, not a fixture landing.
- **Live oracle reproduction is a separate claim** from doc↔baseline consistency: the guards read
  only committed JSON. Run all three under a fresh cache
  (`XDG_CACHE_HOME=$(mktemp -d) go run ./cmd/pilot-{xpect,reject,diff} -out /tmp/oN`) and `cmp`
  each against its committed baseline.

## Gotchas

- `sed -i 's/…230 of 230…/…229 of 230…/'` silently matches nothing if you guessed the number: always
  `grep -c` the mutated text and confirm it is 1 before believing an `already current` result.
- Both errors and the stale-file diff go to different streams: the diff report is on **stdout**,
  hard errors on **stderr**. Capture both and the exit code without a pipe.
- Find a python with mkdocs before building: run `python3 -m mkdocs --version` first. `python3` on
  PATH may be another repo's venv (sometimes one that *does* have mkdocs), and the blueprint's
  `~/pv` venv may lack mkdocs even though the maintenance step installs `docs-requirements.txt`
  there — fall back to `pip install -r docs-requirements.txt` into whichever interpreter you use.
  The build prints a red mkdocs-2.0 deprecation banner and still exits 0. `rm -rf site` afterwards.

## Recording

The `cmd/doc-counts` checks are shell-only; no GUI, so no recording is needed. If the change
touches `overrides/home.html` or the site blocks, the built page must be verified in a browser
(band renders, links navigate, light/dark, narrow viewport; the compliance map's inventory shows
numbers, not placeholder sentences) — record that part. Serve the build
with `python3 -m http.server 8899 -d /tmp/site`, maximize Chrome with
`wmctrl -r :ACTIVE: -b add,maximized_vert,maximized_horz`, and force a narrow viewport with
`xdotool getactivewindow windowsize 620 1100` plus a couple of `ctrl+plus` page zooms
(Chrome will not size its window below roughly 500px).

## Devin Secrets Needed

None. Network to github.com / Maven Central is needed only to re-provision the pilot validators.
