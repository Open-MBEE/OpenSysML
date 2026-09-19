# OpenSysML — Roadmap

Baseline: `v0.8.0` (`238aed650`, `Merge pull request #277 from Open-MBEE/release/0.8.0`,
2026-09-13), verified locally with Go 1.25.0. It is the newest tag on `Open-MBEE/OpenSysML`, after
`v0.7.0` (`e0fbfea5b`, 2026-09-09) and `v0.6.0` (`30f103bb9`, 2026-09-07). The integration branch
`develop` (`fcfb0a7b9`, #352) carries 64 pull requests past the tag and nothing else is counted
ahead of it: every status below is what the tag carries unless the text names one of those as
having moved it — #267, #289 and #293 (Q2), #263 (A7), #286 (the release fold-back), #291
(the generated test figures), #292 (L7), #296 (A4), #335 and #352 (E1), #344 (modeled
randomness beside A3), #300 and #316 (the large-model design and its first step, under
"Proposed"), #319, #321 and #334 (the fUML referee for actions, under "Proposed"), #350 (R4's
installer wizard), #304 (the errata overlay over the bundled library, under "Upstream
follow-through"), and the state-executor fixes and findings the PSSM referee adjudicated (#295,
#297, #311, #313–#315, #317, #318, #322, #326, #336, #342; Track E) — and the pull requests open
against `develop` at this baseline are named where they touch a roadmap item.
Read `AGENTS.md` first; it governs everything below.

> **Labels.** This is an engineering record. The RDF items keep the `D` numbers (`D1`, `D2`,
> `D3.4`, `D7`, `D8`, `D9`, `D10`, `D11`, `D12`) that other records, the known-violations inventory
> and the ontology package's README cross-reference; `L` names the library items, `N` the native
> compilation track, `R` the release follow-through, `W` the diagram output formats a view
> rendering is written in, `F` the executor defects the conformance gate carried as
> known failures (closed), `S` the multiple-valid-executions work the executor needed before
> Track E (landed), `E` the behavior-execution semantics the runtime does not yet have, `X` the expression forms it
> parses but does not evaluate, `Q` the runtime query surface, `A` analysis and simulation
> execution, `V` the validation census, `I` the language integrations, `B` the bindings from
> modeled elements to external data and services, `M` the embedded target, and `P` the package
> layering of the Go module. Each is stated in
> full where it is introduced, and a reader who wants only the gap can ignore the label.
>
> **Status words.** *Landed* means in the tag named above, so a landed item is released; where a
> pull request merged to `develop` after the tag moved an item, the text names it and the item is
> landed but unreleased. *Open* names a pull request against `develop` that
> exists and is not merged; *conflicts* means it no longer merges cleanly onto `develop` and
> needs a rebase before review. *In progress* means the work is being implemented and has no
> pull request yet. *Not started* means exactly that. A status is taken from the pull request
> itself, never from a branch name or a commit message.

`v0.8.0` is the newest tag on `Open-MBEE/OpenSysML` (`238aed650`, 2026-09-13), after `v0.7.0`
(`e0fbfea5b`, 2026-09-09) and `v0.6.0` (`30f103bb9`, 2026-09-07). The tag's CircleCI `release`
workflow succeeded: the release carries `sysml`, `sysml-lsp` and `sysml-grpc` for five platforms,
the Homebrew bundles, the cosign-signed manifest and — for the second release running — the
Windows installer `opensysml-0.8.0-windows-amd64.msi` (R4). The Python client's `opensysml-v0.5.0`
tag points at the same commit and PyPI serves `opensysml` 0.5.0 from it — its `release-python`
workflow failed twice in `Go coverage profile` and passed when rerun, so the post-tag checks in
`docs/project/releasing.md` are all met and nothing of the release step is left.
`CHANGELOG.md`'s **0.8.0** section is the tag's content (#268 and #277 folded it), its **0.7.0**
section is `v0.7.0`'s (#145, #154, #159), and `changes/unreleased/` on `develop` holds the
fragments of the pull requests merged after the tag alone, #286 having carried the fold back.
Between releases `.github/workflows/nightly.yml` publishes the newest green `develop` as the
prerelease `nightly` (#284), so an item landed after the tag is unreleased but not unbuilt.
Since `v0.7.0` the repository works git-flow: `develop` is the integration branch, `main`
receives only `release/x.y.z` and `hotfix/` pull requests (#151), and the roadmap's baseline is
therefore the tag rather than `main`'s head.
Everything in "Release follow-through" is maintainer- or account-gated; everything after it is
ordinary engineering work.

**What closed between `v0.6.0` and `v0.7.0`** — 50 merged pull requests (#110–#161, less #143
and #160, which were closed unmerged), in `CHANGELOG.md`'s 0.7.0 section for the detail, listed
here because each retires or narrows a roadmap line. The tracks below carry the detail and what each pull
request deliberately leaves.

- *Executor.* A node several successions reach performs once, after one token per succession,
  and a join one of whose successions can deliver nothing deadlocks instead of firing on a count
  (#116, closes F1 and F2); a merge is one performance per arrival, so a loop through a merge runs
  to its exit (#120, closes F3); a breakpoint on a synchronized node pauses once and a step moves
  each token at most once (#119); a via-less `accept` on a transition no longer takes a transfer
  addressed to a port (#126); a paused `%action`/`%state` run resumes with its own budgets, choice
  points and memo rather than another run's (#130); a long race-instrumented run no longer grows
  step by step (#146); a sourceless transition leaves the state declared before it, the guarded
  entry transition chooses by its guards, and a machine's own `entry`/`do`/`exit` run for every
  machine (#155, #161). `known_failures.txt` is empty.
- *Multiple valid executions.* A conformance case lists the outcomes the library admits and a
  `.trace.order` file states a partial order on its trace (#110, closes S1); every choice the
  library leaves open is reported as a `choice` trace line, a diagnostic and a REPL summary (#123,
  closes S2); the scheduling policy is selectable — `reverse` the default, `declared`, `seed:<n>`
  — on the CLI, in the REPL, on the wire and as a conformance-case pin (#125, closes S3); `explore`
  runs every linearization one token per step and tables the distinct outcomes, and the harness
  explores every case that lists `outcomes` (#134, #153, closes S4); the order two regions react
  to one event in is a choice point under every policy and `seed:<n>` varies it (#141); the
  oracle, the compliance record, the behavior guide and a design note state which orders are open
  and how each is checked (#138, #150).
- *Expressions and values.* `x as T` evaluates as classification, including composed types (#115,
  closes X3); `*` is a value and `elem.metadata` reads an element's metadata (#113, closes X4); a
  calc is a value — passed, returned, read off a part, applied through a `calc`-typed parameter —
  and crosses the wire as `Value.function` (#122, closes X6); `Collections::Set` is held as a set
  and tensor quantities take any rank, both crossing gRPC whole (#121, closes X7's value half); a
  collection operation's static type follows its body's result (#112, closes X8's typing half); a
  wire `Diagnostic` carries its `code` (#124); a worked example of the expression forms (#147).
- *Analysis.* A verification case runs and reports its body's verdict beside requirement
  satisfaction, and its objective checks the case's subject (#117, #156, closes A6); `-sweep`,
  `-samples -seed`, `%sweep`, `%samples`, `RunSweep` and `run_sweep` run a case or a calc once per
  value, typed by the parameter they bind, and table the rows (#118, #157, closes A3); a
  `TradeStudy` runs as the library writes it and reports each evaluation, with `%optimize`
  refusing an objective bound to the case's own calc (#133, closes A2); one simulation clock owned
  by the runtime context, `accept after`/`at` in action bodies, `Context.Advance`, the `due order`
  choice point and `final_time` on the wire (#136, closes A5); a worked walkthrough of analysis
  cases under `examples/analysis-demo` (#148).
- *Wire and clients.* Every value kind above crosses gRPC/Connect under its own capability
  (`infinity_value`, `function_values`, `set_values`, `tensor_values`, `verification_verdicts`,
  `case_evaluations`, `schedule`, `schedule_explore`, `final_time`, `diagnostic_codes`), read by
  the Go, Python, Node, Rust and Java clients; the Go and Python clients add sweeps, exploration,
  evaluations and the schedule option (#113, #117, #118, #121, #122, #124, #125, #133, #134, #136).
- *Release, CI and records.* The Windows MSI builds again on the GitHub runners and `v0.7.0` was
  the first release to publish one (#127, R4); pull requests run GitHub Actions only while CircleCI
  runs on `main` and tags, as four jobs under the plan's time limit, with the wire-compatibility
  baseline the merge's own parent or the previous tag (#108, #132); development moved to a
  `develop` integration branch (#151); the coverage the SonarCloud scan reads measures every suite
  the checks run (#142) and the findings are cleared (#109, #139); loading a large model merges
  the library's member set once rather than per declaration (#114); the pre-1.0 rule for which
  version segment a release bumps is stated in `CONTRIBUTING.md` (#131); the bounded-model-checking
  design record (#135, #158) and the bindings track below (#140) are written; the roadmap was
  refreshed to the `v0.6.0` baseline and then swept for what landed after it (#111, #149); and
  the repository's agent skills were extended (#128, #129, #137, #144, #152).

**What closed between `v0.7.0` and `v0.8.0`** — 113 merged pull requests (#162–#279, less #216
and #232, which were closed unmerged, and #263, #267 and #278, which merged to `develop` after the
tag), in `CHANGELOG.md`'s 0.8.0 section for the detail, grouped as the paragraph above is.

- *Analysis framework and engines.* Every analysis question goes through one framework
  (`internal/exec/analysis`, #172, #162): each plan runs on a resolver and semantic model of its own
  (#180), runs and sweep rows go in parallel under `-jobs` with the same answer whatever the count
  (#190, #193), a sweep over a held object runs on the object as the session holds it (#215), every
  verdict states its standing and the engines can be listed and chosen (#182), an action annotated
  `ToolExecution` runs through an external tool (#189), and an engine is a directory of executables
  speaking over standard input, bounded by `OPENSYSML_TOOL_MAX_OUTPUT` (#235; design note #171).
- *Model checking.* An SMT model checker for actions on concrete inputs (#185), over free inputs
  (#233), deciding whether the schedule decides a feature under its own solver budget (#262,
  #279), reading timed accepts, messages and nested flows (#260); `-engine check` searches every schedule of an action, of a state
  machine, and of the two together, with static reduction and time ties (#192, #244); the
  `replay:<file>` scheduling policy replays a witness, including a clock-retried step (#258); the
  OMG PSSM state-machine suite runs as an advisory referee (#230); the bounded model checker's
  independence relation counts message order (the design note, #217's companion).
- *Expressions and values.* The extent operator `all T` evaluates, statically `T[0..*]`, over the
  whole model, and a namespace-level usage of several occurrences denotes its objects (#211, #238,
  #239, closes X5 and the expression half of Q2); `x meta T` evaluates and `x.metadata` ends with
  the reflective metaobject, which crosses gRPC (#212); a `[0..*]` feature holding one value
  denotes that value wherever one value is expected, including through a feature chain, so
  `interpolateLinear` runs (#164, closes X2 and unblocks L7); function values compile natively in
  C and Go (#165, retiring the compiler's function-value refusal); a literal's direct type is its
  `ScalarValues` definition, scale arithmetic agrees with the library, a computed quantity is
  reported in its dimension's coherent unit, and classification against an enumeration is decided
  by its values (#205, #191, #179, #208, #177); a bare feature reference is typed statically by
  the feature it names, a computed value is judged against a scalar-typed feature, and an argument
  of unknown type keeps every overload (#213, #237, #241); a multi-valued feature not declared
  `nonunique` refuses a repeated value (#178).
- *Executor.* A deferred event outranks an enclosing transition, a history with nothing to
  restore performs the ordinary entry, a composite's completion fires its own completion
  transitions, and a `choice` reads its guards after the incoming effect (#228); a `do` behavior
  may be a full action that waits on the clock (#181); a body paused mid-statement is state a
  snapshot captures, and the runtime's `Context` is a shared `Model` plus a run's own state
  (#187, #259); `first a then b;` in action and state bodies is the succession it names, with
  feature-chain ends (#269, #270, #274); `%send` reaches an object whose performed action waits
  at an `accept` (#271); `entry point`/`exit point` — an extension with no SysML v2 notation —
  are gone (#184).
- *Diagrams and documents.* A view renders as Graphviz DOT with the `dot` form, in the pilot
  visualizer's Standard B&W style with an optional colourblind-safe palette, and a document
  renders its diagrams as DOT on request (#222, #248, closes W1 and W3's DOT half); a view renders
  as PlantUML with the `plantuml` form and a document as PlantUML on request (#264, closes W2 and
  W3's PlantUML half); a model can say where a view draws its elements and every rendering carries
  it, validated by `-validate` (#227); node labels lead with the name (#243); `sysml -render`
  accepts several files as one model (#167).
- *Editing.* The VS Code diagram panel is an editable canvas: an **Add…** menu, connection edits,
  a dragged node's position written back, **Move to…** across namespaces, and rename or delete
  following references into the other documents of the model, all through the
  `opensysml/applyModelEdit` LSP request over `internal/check/edit` (#247, #261, #265, #266).
- *Library and identity.* Named standard-library elements carry the normative element ids the
  KerML and SysML specifications assign, and a copy of a library file rooted at the library's
  packages converts as the library (#245, #276, #278 — the last past the tag); a `MOSA` library
  and its openness checks (#200); every bundled library file converts back from its graph without
  source text (#272).
- *Records, corpora and site.* The pinned OMG pilot implementation is release `2026-08` (#224);
  the precise-semantics alignment note (#217), the embedded-target design note (#226), the
  runtime showcase under `examples/runtime-showcase/` (#225), the architecture self-model's
  analysis flows (#194, #201), the execution-semantics vocabulary (#183) and the landing page's
  carousel (#203); SysML v1 migration reads the open model formats tool-neutrally (#188); the
  Python client answers `Model.find`, `Model.get` and indexing from the model (#207); the
  SonarCloud findings are cleared on each pass (#196–#199, #242, #253, #257, #275); the roadmap
  gained D12, Track W (#195) and A7 (#214) along the way; and `main` was merged back into `develop`
  after `v0.7.0` (#202).

The previous baselines' retirements — native compilation's first phase, the wire contract, L3–L6,
the nested-action frames, D3, Q4, the changelog fragments, the census and its gate, the
enumeration rules (#907, #909) — stay retired and are not repeated.

## Where the repository stands

Full gate green: `gofmt -l .` empty, `go build ./...`, `go vet ./...`, `go test ./...`, and the
corpus gates run locally clean at the baseline, with the pilot material re-fetched at its pin
(`./scripts/download-pilot-corpora.sh`, `./scripts/download-pilot-xpect.sh`). A stale local copy
of the pilot corpora fails `cmd/pilot-diff`, `cmd/pilot-xpect` and the `TestPilotCorpora` gate
with a provenance message naming the drift; that is the gate working, not a regression — re-fetch
before re-recording anything.

| Gate | Count at `v0.8.0` (`238aed650`, 2026-09-13, Go 1.25.0); the `v0.7.0` figure in brackets where it moved |
|---|---|
| OMG training corpus | **100/100 clean** — asserted, not ratcheted: no file reports a semantic error |
| OMG pilot corpora (ratchet) | 213 files; 7 report a diagnostic [6], each adjudicated in [pilot-corpora.md](pilot-corpora.md) and [omg-issues.md](omg-issues.md) |
| Stdlib parser conformance | 100/100 clean — 94 vendored OMG files and 6 non-normative OpenSysML extensions |
| Execution conformance cases | 894 under `TestExecutionConformance`, all run and pass, none skipped [770] |
| Known execution-conformance failures | **0** — `known_failures.txt` holds no case: "every derived case passes" |
| Cases admitting several outcomes | 26 `.expected.json` files list `outcomes`, each citing its derivation in the behavior semantic oracle; the harness explores every one of them under `explore` [19] |
| Trace partial orders | 8 `.trace.order` files, each a set of `a < b` lines the recorded trace must satisfy [5] |
| Golden execution traces | 276 `.trace.golden` files: 230 under the default schedule (`TestExecutionTrace`, one per case) and 46 per-policy goldens (`<case>.declared`, `<case>.seed-1`) [216: 182 and 34] |
| Runtime robustness cases | 459 first-level subtests of `TestRuntimeRobustness` [369] |
| gRPC conformance fixtures / robustness cases | 15 / 8 (`TestGRPCConformance`, `TestGRPCRobustness`; the authoring service adds 2 robustness cases of its own) |
| Golden AST fixtures | 205 (`TestGolden`: 177 SysML, 28 KerML) [197: 171 SysML, 26 KerML] |
| Negative parser subtests | 252 first-level subtests of `TestNegative` (396 across the `TestNegative*` functions, 454 across every `*Negative*` parser test) [249: 338 and 396] |
| Rejection oracle | 306 self-authored invalid models: 293 both reject by default and 297 when we are asked strictly, 4 the pilot alone by default and none strictly, 9 ours alone [285: 273 and 276, 3 the pilot alone] (the control-node rules the pilot leaves unimplemented and a non-Boolean succession guard) |
| Validation census | 162 of 217 named constraints reported (156 faithful, 6 approximate), 1 not implemented, 1 deliberate, 53 unknown |
| RDF corpus round trip | 353 of 353 models stable, none refused [346] |

The pilot differential, the Xpect oracle, the scope oracle and the rejection oracle are the
external conformance statement, and their figures are generated into `README.md` by `make
docs-counts` from the committed baselines; they are not repeated here.

The test-suite figures above are counted from `go test -v` at the tag. The other surfaces that
repeat them (`README.md`, `docs/project/spec-compliance.md`, `docs/internals/architecture.md`)
were typed in by hand at the tag and lagged it — last recounted at `074f9c4b7` (#245), they read
889 conformance cases where the tag has 894. Since #291, on `develop` after the tag, they are
generated: `tools/cmd/doc-counts` counts the conformance cases, golden ASTs, golden traces, negative
parser subtests, runtime and gRPC robustness cases and top-level `Test` functions from the tree
the way the gates enumerate them (the conformance cases through `tests/fixtures`, which the
runtime and gRPC conformance tests read too), `make docs-counts` writes them into marker blocks
beside the refereed pilot figures, and `go run -C tools ./cmd/doc-counts -check` fails in CI when a block
and the tree disagree, so the surfaces cannot drift from the gate table again. They have since
left git altogether: the site build counts them into the compliance map's test inventory
(`scripts/mkdocs_suite_figures.py` over `go run -C tools ./cmd/doc-counts -site-blocks`), the committed
pages name what is counted without a figure, and a branch adding a test rewrites no shared line.
The tests-and-subtests total of a run, which only a run can state, is no longer quoted anywhere. At
`develop`'s head the blocks read 1013 conformance cases, 307 default and 80 per-policy trace
goldens, 493 runtime robustness cases, 21 gRPC conformance and 8 gRPC robustness cases, 208 golden
ASTs and 252 negative subtests; the growth over the table is fixtures landed after the tag, all
unreleased. The robustness cases are counted across the `TestRuntimeRobustness*` and
`TestGRPCRobustness*` functions since #345 registered them per feature, so the table's
single-function figure and the block's are the same census under two spellings. The census,
rejection-oracle and RDF round-trip rows follow the committed baselines; none of the three moved
between `074f9c4b7` and the tag, the census did not move since `v0.7.0`, and the other two grew
with the corpus and the baseline between `v0.7.0` and `074f9c4b7` (bracketed above).

Statement coverage, re-measured with `go test -cover ./...` at this baseline with the corpora
present. It counts only each package's own tests, which understates a package consumed by others
(`internal/syntax/ast` is exercised by every parser test; `internal/translate/codegen`'s differential
runs from `internal/frontend/repl`). It is not the figure the SonarCloud scan reads: since #142 `make
coverage` also builds the `sysml`, `sysml-grpc` and `sysml-lsp` binaries instrumented and folds
the counters their test runs write into `coverage.txt`, so the scan's per-package figures for the
command packages are higher than these.

| Package | Coverage | Package | Coverage |
|---|---|---|---|
| `internal/core/quickfix` | 100.0% | `internal/semantic/symbols` | 84.7% |
| `internal/core/conformance` | 100.0% | `internal/exec/solve` | 84.3% |
| `internal/syntax/ast/astcodec` | 99.5% | `client/opensysml` | 84.2% |
| `internal/syntax/format` | 97.2% | `internal/semantic/identity` | 83.5% |
| `internal/doc/docrender` | 94.4% | `internal/workspace/project` | 81.0% |
| `internal/translate/rdf/ontology` | 92.1% | `internal/doc/queryexec` | 80.9% |
| `internal/frontend/grpc` | 90.9% | `internal/semantic/resolve` | 76.3% |
| `internal/workspace/libs` | 90.0% | `internal/semantic/query` | 73.2% |
| `internal/check/passes` | 89.9% | `internal/workspace/model` | 73.0% |
| `internal/syntax/parser` | 89.8% | `internal/ir/lower` | 71.3% |
| `internal/translate/export` | 89.3% | `cmd/sysml-lsp` | 70.0% |
| `internal/translate/migrate` | 89.1% | `internal/semantic/semantics` | 64.3% |
| `internal/frontend/lsp` | 88.3% | `cmd/pilot-exec-diff` | 41.6% (the pilot-evaluator half is gated) |
| `internal/frontend/repl` | 88.2% | `internal/translate/interop/flexo` | 39.5% (the live-stack half is gated) |
| `internal/exec/runtime` | 88.1% | `cmd/sysml-grpc` | 38.7% |
| `internal/translate/rdf` | 87.1% | `internal/syntax/ast` | 21.8% |
| `internal/ir/queryplan` | 85.6% | `cmd/sysml` | 17.8% |
| `internal/doc/docir` | 85.6% | `internal/translate/codegen` | 1.9% |

The corpus gate needs the corpus (`./scripts/download-training-examples.sh`) and never
re-baseline `internal/workspace/model/testdata/training_examples_expected.txt`: adjudicate each
drifted file and record the verdict in `docs/project/training-examples.md`.

A tag cannot be cut over a corpus regression. Pull requests run one CI, the GitHub Actions
workflow `.github/workflows/pr.yml`, which downloads the corpora (cached on the download scripts)
and runs the suite with `OPENSYSML_REQUIRE_TRAINING_CORPUS=1`, `OPENSYSML_REQUIRE_PILOT_CORPORA=1`
and `OPENSYSML_REQUIRE_SMT=1` before a merge, with the protobuf wire-compatibility check against
the branch the pull request merges into (#108). `.circleci/config.yml` runs on `main` and on `v*`
tags only, as four parallel jobs since #132 — `Go static checks`, `Go race tests`, `Go coverage
profile` and `Go gates and binaries` — each downloading the corpora it needs and requiring them
the same way; the client tests, the SonarCloud scan and every release workflow wait on all four.
Its wire-compatibility baseline is the merge's own first parent (`HEAD^1`) on `main` and the
previous release tag on a tag build, so the verdict cannot change with what merges afterwards
(#132).

---

# Release follow-through

Tagging a core release, publishing the Python client and the Homebrew bump are all proven paths:
`v0.8.0` is tagged and its `release` workflow completed with the full archive set, as `v0.7.0`,
`v0.6.0`, `v0.4.2` and `v0.4.3` did before it; `opensysml-v0.5.0`, tagged with `v0.8.0`, uploaded
the client to PyPI as `opensysml-v0.4.0` did before it (its `release-python` workflow needed a
rerun after two `Go coverage profile` failures, and passed); and the tap
`Open-MBEE/homebrew-tap` bumps itself from its own scheduled workflow on
each tag, rendering the formula from this repository's `scripts/render-homebrew-formula.sh` and
template. The procedure and its post-tag verification are in `docs/project/releasing.md`.

## R1 — the changelog agrees with the tags (done)

`CHANGELOG.md`'s **0.4.3** section holds what `99e02003` shipped (#877), its **0.5.0** section
what `0fdeb11e` shipped (#894), its **0.5.1** section what `d7b3eb45` shipped, its **0.6.0**
section what `30f103bb9` shipped (#988), its **0.7.0** section what `e0fbfea5b` shipped (#145,
#154, #159) and its **0.8.0** section what `238aed650` shipped (#268, #277), each dated to its
tag; **Unreleased** is empty in the file because since #852 a change adds a fragment under
`changes/unreleased/` instead, and `python3 scripts/changelog.py release X.Y.Z` folds the
fragments in at release time (`releasing.md`). The 0.5.0 fold was the first release cut that way
and every release since followed it; under git-flow the fold rides the `release/x.y.z` pull
request into `main` and returns to `develop` with the back-merge (#202 after `v0.7.0`, #286
after `v0.8.0`). Nothing remains here; the item is kept because the folds are the procedure's
proof.

## R2 — the Node, Java and Rust clients are unpublished

Each has its release workflow (`client-node-v*` to npm, `opensysml-java-v*` to Maven Central,
`opensysml-rust-v*` to crates.io) and a worked example the tests run, and none has ever been
tagged. The Java package name already moved to `org.openmbee.opensysml`, the DNS-verified
namespace, so nothing blocks Maven Central but the account. npm and crates.io need a publisher
token in CI; Maven Central needs the Sonatype account and a signing key. These are account gates
like R4, not engineering.

## R3 — Homebrew: install it on a real Mac

Everything about the tap is automated and verified on Linux (install, `brew test`,
`brew audit --strict --online`), and the manual pages (#699, `man/man1/*.1`, generated from
`internal/frontend/usage` and drift-gated by `make man-check`) are in the bundles the formula installs. The
one thing never done is running the darwin bottle on macOS: the darwin archives' checksums match
the release manifest and nothing more.

`homebrew/core` — which would drop the tap and the trust step entirely — is gated on
[notability](https://docs.brew.sh/Package-Acceptance-Policy#notability) (75 stars / 30 forks /
30 watchers, or 225 / 90 / 90 self-submitted), so it is not a near-term option.

## R4 — code signing

macOS binaries are not Developer ID signed or notarized, so a browser download trips
Gatekeeper. Root-caused in `docs/project/macos-distribution.md`: it is `com.apple.quarantine`,
not a missing signature — Go's linker already ad-hoc signs darwin/arm64 — so ad-hoc `codesign`
in CI would change nothing. Notarization needs an Apple Developer account, a Developer ID
certificate, an App Store Connect API key in CI and a macOS runner: a purchase, not a task.

Windows Authenticode signing is prepared, pending approval by
[SignPath Foundation](https://signpath.org), which signs open-source Windows binaries for free
from a build it can verify the origin of. The repository side is done: the README carries the
[Code signing policy](../../README.md#code-signing-policy) the terms require, the three Windows
executables embed the `VERSIONINFO` SignPath enforces (`ProductName` `OpenSysML`,
`ProductVersion` from the tag), and `.github/workflows/release-windows.yml` rebuilds them on a
`v*` tag under GitHub Actions — a trusted build system for SignPath, which CircleCI is not —
submits them for signing, and publishes the signed files as `*-signed*` release assets beside
the unsigned ones that the cosign-signed manifest keeps covering. What remains is a
maintainer's application at <https://signpath.org/apply>, the SignPath project and policy
setup, the `SIGNPATH_*` secret and variables in GitHub, and one manual approval per release;
until then the workflow builds and stops. Procedure: `docs/project/releasing.md`, "Windows
Authenticode signing".

Windows packaging is in place on the same workflow: a WiX v5 MSI (`packaging/msi`,
`scripts/build-msi.sh`) installing the three executables to `Program Files\OpenSysML` on `PATH`,
with the Z3 solver as an optional feature pinned by hash, published unsigned as
`opensysml-<x.y.z>-windows-amd64.msi`. `v0.6.0` shipped no `.msi`: its `msi` job failed with
`error: C:\Program is required`, because the script read the `wix` command from the `WIX`
environment variable, which the preinstalled WiX v3 on `windows-latest` exports as its
installation directory. #127 renamed the override to `WIX_CMD`, and `v0.7.0` and `v0.8.0` each
published their `.msi`. Those two ran as Windows Installer's bare progress window; #350, on
`develop` after the tag, gives the MSI a setup wizard — destination folder, the gRPC service
and Z3 as selectable components, a confirmation and a completion page, repair and remove on a
second run — so the next tag's `.msi` is the first a user is walked through.
Once SignPath is configured it is rebuilt from the signed executables and itself signed as
`*-signed.msi` (Z3 stays unsigned by SignPath's terms). Scoop,
winget and MSYS2 manifests that depend on Z3 rather than bundle it are maintained as templates
under `packaging/` with render scripts; what remains there is a maintainer submitting each to
its external repository (and, for winget, confirming the `OpenMBEE.OpenSysML` identifier and a
Z3 package to depend on). Procedure: `docs/project/releasing.md`, "The Windows installer".

## R5 — the VS Code extension is not released

`editors/vscode` builds only as a PR CI artifact: no `.vsix` is attached to a release and there
is no marketplace or Open VSX listing, so a user cannot install it without building it. The
client works against the shipped `sysml-lsp` (`--stdio` is accepted and `shutdown`/`exit` are
honoured). What remains is packaging and publishing: `vsce package` in the release workflow, a
`.vsix` on the release, and (for the marketplace) a publisher account and a PAT in CI — the same
class of account gate as R4.

## Upstream follow-through

Filed and waiting on the other side: the identity-annotation enhancement against SysML 2.0
(`INBOX-2510`, maintainer-approved 2026-09-01). Filed against the pilot and fixed there: the
`ownedDisjoining` EMF defect (`SysML-v2-Pilot-Implementation#790`, shipped in `2026-08`) and the
cross-subsetting validator throw (`#794`). Drafted and waiting on a maintainer to authorise
posting: the three dimensional-analysis errata in the pilot's example corpus, the question about
the `queryx/failing` Xpect fixtures, the unenforced control-node constraints, and the nine
dimension defects in the published `SI.sysml` and `USCustomaryUnits.sysml` — which #304, on
`develop` after the tag, put under the declared errata overlay, so the bundled library reads with
the three that have one correction applied and the six that do not documented, the vendored bytes
untouched. All are in [omg-issues.md](omg-issues.md), body and status; none needs code here until
an answer arrives.

---

# Track L — the standard library at run time

Loading is closed: every library file is parsed and indexed on every load path, built once and
frozen, read through a per-model overlay (`libs.Loader`, `libs.SharedBase`, `symbols.NewOverlay`)
rather than copied per model, and the serialized snapshot brings a process up in under 20 ms.
Name resolution is closed too: a qualified name through a `public import` (`ISQ::speed` or any
user façade package) evaluates (#801), an unqualified library call resolves at evaluation exactly
as the checker resolved it (#816), and an overloaded name reaches the declaration the argument
types select (#825, #861). The library review after `v0.4.3` measured *evaluation* against the
Kernel Function Library's 279 declarations in 17 packages and against the domain libraries; of its
findings L3–L6 have landed and L7 — the analysis libraries — is what remains, and it is a Track X
item in library clothing.

## L3 — evaluation does not reach standard-library-inherited features

A feature a user type inherits from the standard library has no value at run time. With `part def
Box :> Item; part b : Box;`, both `b.isSolid` (declared `isSolid = isEmpty(voids)` in `Systems
Library/Items.sysml`) and `b.voids` report `member … not found in instance`; the cause is
`runtime/shape.go` skipping every `libraryDeclared` feature when it lays out an object, so
`Model.Eval` never gets as far as folding the library's value expression. **Landed:**
[PR #830](https://github.com/JPL-Devin/OpenSysML/pull/830) materializes Systems and Domain library
features on objects in tiers (a feature's own value, then inherited value expressions); at this
baseline `%instantiate L::b; %eval in b : isSolid` answers `true` and `voids` `[]`. The solver has
the mirror gap and is not in that PR: its translatable subset does not take library-declared
conditions, and `solve`'s
differential harness still indexes the standard library as ordinary documents (`parseLibraries`)
to reach them. [lossless-library-records.md](lossless-library-records.md) records
what was measured.

## L4 — every Kernel Function Library declaration dispatches by name

At the previous baseline ~135 runtime names were registered explicitly (`runtime/builtin_names.go`,
`library_functions.go`), operators and the core builtins were handled separately, and **12 of the
17 KFL packages had no dispatch gate**: a call to one of their ~38 named functions — every
`ToString`/`ToInteger`/`ToReal`/… conversion in `BaseFunctions`/`ScalarFunctions`, `Rational
floor/round/gcd`, `sum0`, the generic `max`/`min`, the explicit operator-call spellings `'+'(a, b)`
— fell through to `no result expression` naming the wrong thing, instead of either computing or
failing as `library function X is not implemented`. **Landed:**
[PR #821](https://github.com/JPL-Devin/OpenSysML/pull/821) adds the gate and implementations for
every KFL declaration; a call now either computes the library's stated result or is refused by
name (`ToString(42)` is `"42"`, `max(1, 2)` is `2`, `max(1.0, 2.5)` is `2.5` at this baseline).
The named-reducer forms (`->reduce '+'`) are the same mechanism read from a different position and
are X5 below; which overload a name reaches is L6.

## L5 — `QuantityCalculations`, unit canonicalization and domain failures

`QuantityCalculations` (28 declarations) had no runtime registration at the previous baseline, and
a model that imported it lost `->sum()` over quantities (measured; the import brought a `sum`
declaration nothing dispatched into scope ahead of the scalar one). Composed units had no canonical
rendering. **Landed:** [PR #818](https://github.com/JPL-Devin/OpenSysML/pull/818) dispatches the
package and renders composed units canonically — with the import in scope, `(2 [kg], 3 [kg])->sum()`
is `5 [kg]` and `2 [m] * 3 [m]` is `6 [m**2]` at this baseline. What the same probe shows is L6's
gap: with `ScalarFunctions` and `QuantityCalculations` both imported, `ToReal("2.5")` reaches
`QuantityCalculations::ToReal` and is refused for its argument type instead of selecting the
scalar overload. Domain failures (`sqrt` of a negative quantity, a unit mismatch in `+`) must stay
typed errors naming the function and the units, as the scalar library's already do.

## L6 — invocation overloads selected by argument type (done)

The checker and the runtime used to pick a library overload by name and arity, so `max(1, 2)` and
`max(1.0, 2.0)` reached the same declaration and the result type was whatever that declaration
said. **Landed:** [PR #825](https://github.com/JPL-Devin/OpenSysML/pull/825) selects by argument
type with one selection shared between `semantics` and `runtime`, and
[PR #861](https://github.com/JPL-Devin/OpenSysML/pull/861) makes every reader of a call —
expression calls, document queries, `send` — use it; [PR #862](https://github.com/JPL-Devin/OpenSysML/pull/862)
keeps a calc that specializes a library function on its own signature. The `ToReal("2.5")` probe
under L5 reaches the scalar overload. On an ill-formed model the runtime package now agrees with
the checker about a named argument it cannot place (#904, #908): the label is kept as written and
refused as `ErrUnknownParameter` rather than bound by its last segment.

## L7 — the analysis libraries run in part, measured (landed, unreleased)

What runs at this baseline, against `bin/sysml`: a domain library's calc executes from its own
text as a model's does (#133), so `SampledFunctions::Sample(Sq, (1.0, 2.0, 3.0))` with `Sq` a
user `calc def` samples it and constructs the `SampledFunction` (X6, #122 — at `v0.6.0` this was
refused `cannot evaluate definition Sq`), `Domain(sampled)` reads `[1.0, 2.0, 3.0]` off it, and
`TradeStudies::TradeStudy` runs as the library writes it — `evaluationFunction` bound as a
function value, `MinimizeObjective`/`MaximizeObjective` computing `best` over the alternatives,
`selectedAlternative` found by the inherited `->selectOne` (A2); and since #164
`interpolateLinear(sampled, 1.5)` runs inside the library's own `Linear` — its `f` subtracts
`lowerSample.domainValue`, a singleton read through a feature chain on an object, which at
`v0.6.0` arrived as the sequence `[1.0]` and was refused `operator '-' is not defined for a Real
and a sequence`. That was the chain-read half of X2, and it is closed: a `[0..*]` feature holding
one value denotes that value wherever one value is taken, read off the feature itself or through
a chain, and the conformance case `calc_sample_pair_arithmetic` pins the arithmetic over
`SamplePair::domainValue`/`rangeValue` and the interpolation (`5.0`). `spec-compliance.md` no
longer lists a `SampledFunctions` row as not implemented. `StateSpaceRepresentation` depends on
A4. The vector half moved earlier: since #883 a `NumericalVector` and a `VectorQuantity` are
value kinds of their own, `VectorFunctions` computes over them and `OccurrenceFunctions`
evaluates over lifetimes (#884), the 0.6.0 release added the tensor quantity, coordinate-frame
and measurement-scale values the measurement reference libraries declare, and #121 added the set
kind and tensor quantities of any rank, so no value kind the analysis libraries declare is
missing. The *measured per-library conformance table* — package → declarations → evaluated →
refused by name → wrong — **landed** in #292 on `develop` after the tag, so the library's status
stops being anecdotal: `TestAnalysisLibraryCensus` (`internal/exec/runtime`) enumerates every
public callable declaration of the six packages, invokes each through the runtime against a
representative model and records whether the value passed its check, which typed error refused
it, or what the value got wrong; the verdicts are committed to
`docs/project/analysis-library-census.json`, `make docs-counts` renders them as the table in
`spec-compliance.md` with every refusal listed by name and error, and `go run -C tools ./cmd/doc-counts
-check` fails when the table and the file disagree. At `develop`'s head it reads 76 declarations,
53 evaluated, 23 refused by name, 0 wrong: `SampledFunctions` 5 of 5, `TradeStudies` 6 of 7,
`VectorFunctions` 30 of 39, `OccurrenceFunctions` 6 of 8, `StateSpaceRepresentation` 6 of 17
(4 before A4 landed in #296; what stays refused there is the abstract protocol probed directly —
`getNextState`, `getOutput`, `getDerivative` and `getDifference` are undetermined until a model
specializes them, and `Integrate`, the two event definitions and `StateSpaceDynamics` itself
have no body of their own to run), and `AnalysisTooling` declares nothing callable. Nothing is
left in the track; a verdict that moves is adjudicated in the change that moves it.

---

# Track N — native compilation

The interpreter in `internal/exec/runtime` is the reference semantics and is fast in absolute terms
(about a microsecond per calc invocation after the September pass), but a compute-bound analysis —
a recursive calc, a long numeric loop — is still three orders of magnitude off native code. The
goal is that a calc, and eventually a whole analysis case, can be compiled ahead of time into a
standalone native program that computes exactly what `sysml -calc` computes, prints it the same
way and fails on the same inputs, or refuses to compile with a typed error naming the construct
outside the subset. Nothing is compiled approximately.

## N1 — scalar and collection calcs to C or Go (landed)

`sysml model.sysml -compile Pkg::Fib -o fib` translates a `calc def` (or a calc usage) into an
executable through C (`cc -O3 -flto`, the default) or Go, with `-source` writing the generated
source alone; the design, the measurements and the phase plan are in
[native-compilation.md](native-compilation.md). The pipeline is `parser → resolve → semantics`
plus `lower.CalcBody` into `codegen.Compiler`, a typed IR (`codegen.Program`), and
`EmitC`/`EmitGo`. Landed in four pull requests: #778 the scalar subset (`Integer`/`Natural`/
`Positive`/`Real`/`Boolean` parameters, literals, checked arithmetic and comparison, the logical
and conditional operators, body-local attributes and assignment, `if`/`while`/`loop … until`,
direct and mutual recursion); #796 statement bodies, redefined parameters, the scalar library
intrinsics and named arguments; #828 homogeneous sequences of the scalar types with any
multiplicity, the shape rules, `for`, the sequence and control libraries and the element budget;
and, on the interpreter side, #781's closure fast path for pure calc bodies, which is the tier
between interpreting and compiling. Everything outside the subset refuses with
`codegen.UnsupportedError` naming the construct; a differential test runs every compiled program
against the interpreter in both backends, and the gate stays green with `cc` absent (the C tests
skip; the Go ones do not).

Measured (Xeon 8559C, GCC 11.4, 2026-09-02): `Fib(25)` 261 ms interpreted, 919 µs as Go, 221 µs
as C; `SumTo(1000000)` 1216 ms / 764 µs / 379 µs; `Collatz(27)` 206 µs / 5.2 µs / 0.98 µs. C beats
Go by 2–5× on every loop or recursion, which is what justified keeping two backends: C is the
default and Go the fallback where no C compiler is installed.

## N2 — the rest of the value phase, and the whole-model phases

[native-compilation.md](native-compilation.md) states the target as a dependency-free executable
or a library with a small C API that runs a whole model, in seven phases: values, instances,
constraints and requirements, documents, actions, state machines, embedding. Phase 1 is half done
(the collection half above). In the order they matter:

1. **Records, enums and record field access** — the remainder of phase 1. `type X is not
   Integer, Real or Boolean` is the refusal today, as is a sequence mixing `Integer` and `Real`
   elements, and the interpreter's new `Array`/`NumericalVector`/`VectorQuantity` kinds (#883)
   are refused by the compiler by the same rule until they have a native layout. Quantities with
   units and structured parameters follow from the record type. This is what stands between "a
   calc compiles" and "an analysis case compiles" (A1).
2. **The step budget.** The interpreter stops a runaway loop at `OPENSYSML_MAX_STEPS`; compiled
   code counts elements (the element budget landed with #828) but not steps, so a `while` that
   never terminates runs forever. Decide whether a compiled program carries an optional iteration
   counter or whether "a compiled program is a program" is the documented contract; N2.4 and M1
   both wait on the answer.
3. **The rest of the library.** What L4 registers in the interpreter needs C and Go equivalents
   with the interpreter's exact results and failure behaviour (domain errors, Integer range) as
   each package is reached; the interpreted/compiled differential is the gate.
4. **A stable C ABI.** One entry point per compiled calc — typed arguments in, a typed result or a
   typed failure out, no allocation the caller does not control — would let the REPL and
   `sysml-grpc` call a compiled calc in place of interpreting it, is what I4 (the C client) links
   against, and is what M4 (the embedded host interface) is a restriction of. Design it once for
   all three; not before N2.2 is decided, since a linked-in calc must honour the host's budget.
5. **Packaging.** The C backend needs GNU C (`__int128`, `__builtin_*_overflow`,
   `setjmp`/`longjmp`); document the toolchain requirement in the install guide, and decide whether
   the release bundles should carry a prebuilt runtime shim.
6. **Phases 2–7** — instances, constraints, documents, actions, state machines, embedding — in the
   record's order. Actions and state machines are token and event semantics, not arithmetic, and
   the interpreter's trace is their contract; their closed IR is Track M's first item (M1), so the
   two tracks meet there rather than duplicating the work.

---

# Track D — model persistence and RDF interchange

Saving and SysML ↔ RDF Turtle conversion landed (`internal/translate/rdf`,
`internal/translate/export`, `%save`, `sysml -convert`, `-sync-diff`); see
[the RDF mapping](../reference/rdf-mapping.md).

The RDF direction ships **experimental**, because of D1, D2 and D7 below: its vocabulary
may change without a compatibility path, and the one triplestore interop measured — Flexo — still
drops what those items carry. Every surface says so (`convert.ExperimentalNotice`), and promoting
it to stable is re-measuring the harness once those land, not a documentation change.

Measured by the per-file ratchet at this baseline (`TestCorpusRoundTrip`,
`internal/translate/export/testdata/corpus_roundtrip_expected.txt`), **all 353 models under
`examples/` convert** (the training corpus, the three pilot corpora and this repository's own
demos), every one round-tripping `notation → RDF → notation → RDF` byte-identically; there is no
whitespace-only, graph-diff, unwritable, unparseable or refused verdict left — each file is
pinned, so a movement in any direction has to be adjudicated
([rdf-corpus-roundtrip.md](rdf-corpus-roundtrip.md)). A model the mapping cannot write back is
still refused rather than converted lossily; the corpus simply no longer contains one. The
previous baseline's last refusal class — 40 declarations naming no element of their own (anonymous
`feature`, `event`, `snapshot`, `timeslice` and `assert`) — went in 0.6.0: the graph types the fact
the keyword states (`sysml:portionKind`, `sysml:EventOccurrenceUsage`, `sysml:AssertConstraintUsage`
with `sysml:isNegated`, the named occurrence or constraint as `sysml:references`) and records
`sysx:declaredKeyword`, so the decoder spells the head from the typed facts, and a graph whose
keyword contradicts its typing is refused naming the element. Before that, the 13
result-expression refusals went with #815 and #835 (a body's trailing `a - b` is a
`ResultExpressionMembership` owning the expression itself), the 19 prefix-metadata refusals with
#824 (metadata bodies and prefix metadata are owned `MetadataUsage`s), and the
duplicate-declaration refusals once the parser read the anonymous `connector a to b;` as ends
rather than as a connector named `a`. #827 spells a reference back so it re-resolves to the
element the graph named and #855 links a reference reached through an import or alias. Nothing is
in flight against these numbers.

## The target, stated precisely

The goal is that a graph OpenSysML writes can **stand in for the RDF
`flexo-mms-sysmlv2` produces**: loaded straight into `flexo-mms-layer1-service` as a
branch's model graph, and read back through the SysML v2 API surface as the same
elements, without that service having produced it. Two consequences shape D1–D3, and
both were read from the two services' sources rather than from our own docs:

- **Layer 1 imposes no vocabulary at all.** `routes/gsp/ModelLoad.kt` loads whatever
  triples the request body carries into a load graph, diffs it against staging and
  commits; sanitization (`sanitizeCrudObject`) applies to LDP CRUD objects — orgs,
  repos, branches, policies — not to model triples. So layer 1's requirements are
  transport-level: a Turtle body on `PUT .../branches/{branch}/graph` (or SPARQL update
  on `.../update`), the ETag precondition, an optional `?message=`, and the literal-size
  limit (`maximumLiteralSizeKib`). Named-graph layout, commits, locks and provenance are
  layer 1's own and are not ours to emit.
- **The vocabulary contract belongs to the reader in `flexo-mms-sysmlv2`.**
  `ElementApi.extractModelElementToJson` is what turns triples back into API payloads,
  and it is stricter than `Namespaces.kt` suggested. It keeps `sysml:` and
  `urn:sysmlv2:annotation:json:` predicates and **ignores everything else** (the
  unrecognized-predicate error is commented out), so every `sysx:` triple is dropped the moment
  a graph passes through that service. Whatever a model needs in order to survive has to be
  standard, which is what makes D1 and D2 part of the interop goal rather than refinements
  after it.

What matches today: `sysml:` = `https://www.omg.org/spec/SysML#` and `elmt:` =
`urn:sysmlv2:element:` are identical to `Namespaces.kt`; `rdf:type` plus `sysml:<property>` per
scalar field is the shape the reader expects; our typed literals fall in the datatypes it maps;
every element carries `sysml:elementId` equal to the id its IRI ends in, so listing by id and the
`@id` derivation agree; and ownership is materialized as the abstract syntax states it, so the
roots endpoint sees one root per document. Those were D3.1–D3.3 and D3.5, closed and now measured
rather than claimed — see the next section.

## D3 — make a converted graph readable through Flexo, and prove it

The harness is `internal/translate/interop/flexo`, the `FLEXO_INTEROP` gate `TestFlexoInterop`, documented
in `.agents/skills/flexo-interop`; it brings up the published `openmbee/*` images, `PUT`s our
Turtle to a branch graph, reads every element back through `flexo-mms-sysmlv2` and compares with
what the service's own commit path stores for the same model. It measures the gap instead of
asserting the fix, so every item below shows up as movement in
`internal/translate/interop/flexo/testdata/interop_expected.txt`. Keep it out of `go test ./...`.

What the current recording measures, for the identity-carrying fixture: **49 of 49 elements
listed and 369 of 452 properties delivered** on the graph-load side, against 33 of 33 and 158 of
158 for the same model posted through the service's own commit path; 9 of 49 read as roots, and 9
have no owner in the model; every element is readable directly by id; no subject of the graph is
outside the element namespace. **Every standard property is delivered, the 14 multi-valued ones
included** — `ownedMember`, `ownedMembership`, `ownedRelationship` (3/3 each), `ownedFeature`,
`ownedFeatureMembership` (2/2 each), `specializes` (1/1) — since D3.4 landed. The 83 lost
properties are one thing:

- **10 property keys in `sysx:`** — `sourceText`, `sourceTail`, `sourceLanguage`, `hasBody`,
  `memberIndex`, `argumentIndex`, `declaredKeyword`, `endForm`, `endIndex`, `relatedFeature` —
  dropped unread. That is the D1/D2 residue below, and it is the reason the expression trees and
  end structure the mapping now writes do not survive the hop. (The one multi-valued property
  still lost, `relatedFeature` on 0/1, is among them.)

The commit path delivers 6 of 6 of its own multi-valued properties, because it stores each array
whole as a JSON annotation literal alongside the typed triples; the graph now carries the same
literal, and the two paths deliver every array alike. Two deployed behaviours differ
from the sources: the element listing ignores `pageSize`/`pageAfter` and returns every subject,
and project delete is a soft annotation that leaves the Layer 1 branch behind.

### D3.4 — collection-valued properties need the JSON annotation (done)

The reader **skips** a `sysml:` predicate with more than one object and prefers the
annotation literal at `urn:sysmlv2:annotation:json:<key>`, which it parses as JSON.
Anything multi-valued emitted as bare repeated triples alone is silently dropped on read.
[PR #850](https://github.com/JPL-Devin/OpenSysML/pull/850) closed it, and D3 with it: one
`json:<key>` literal per multi-valued `sysml:` property beside the typed triples (shape taken from
the service's `CommitApi.kt`, cited in `rdf-mapping.md` § Collections), a decoder that accepts
either spelling or both and refuses a graph whose two spellings disagree, and `reposync` keeping
the literal in step when it mints ids. Re-recorded against the live stack, the multi-valued
standard properties went from 0 of 14 to 14 of 14 delivered, and the total from 355/424 to 369/452
(the denominator moved with the source-text properties the mapping added since the previous
recording; the one multi-valued property still lost is `sysx:relatedFeature`, D1/D2 residue).

## D1 — expression trees are standard in shape, non-standard in vocabulary

Every expression-valued position — a feature value, a multiplicity bound, a guard, a filter, a
condition, a send payload — is now a **tree of typed nodes** in the `expr:` namespace
(`rdf-mapping.md` § Expressions): standard metaclasses (`OperatorExpression`,
`FeatureReferenceExpression`, `LiteralRational`, …), `sysml:argument` and `sysml:referent`
linking operands and referents, a deterministic per-position id every node states in
`sysml:elementId`, and a decoder that reads a foreign tree from its structure alone. SPARQL can
see inside a value now; "every part whose mass exceeds 1000" is expressible.

What remains is what the Flexo hop still loses and the metamodel still does not recognise:

- the operator, the operand order and the source text ride in `sysx:` (`sysx:operator`,
  `sysx:argumentIndex`, `sysx:sourceText`), so after the hop a tree keeps its nodes and loses
  their meaning. The metamodel spells the operator `OperatorExpression::operator` and orders
  arguments through `ownedFeatureMembership`s; emit those;
- a node is not a model element — no `qualifiedName`, no ownership, reachable only from the
  position that holds it — where the abstract syntax makes an expression a `Feature` owned through
  a `FeatureMembership`. Writing expressions as owned elements is the same materialization D3.3
  did for ownership, and it is what the ontology gate's `value` → `FeatureValue` findings (D8) are
  waiting on;
- ~~an expression standing as a body member~~ — done: a calc's trailing result expression is a
  `ResultExpressionMembership` owning the expression (#815, #835,
  [rdf-mapping.md § Result expressions](../reference/rdf-mapping.md#result-expressions)), and no
  file in the ratchet is refused for an expression.

## D2 — end bindings are structure, but in `sysx:`

`connect`, `bind`, `flow`, `succession`, `transition`, `accept` and `satisfy` now state their ends
as structure beside the verbatim head — one expression node per end under `sysx:relatedFeature`
with `sysx:endIndex`/`sysx:endRole`, and `sysx:endForm` naming the notation the ends are written
in — so a graph from another tool converts to notation with no text at all, and a succession
carries both its ends including the unnamed member a `then` sequences (`rdf-mapping.md`
§ End-binding heads). The form is stated only when rebuilding it reproduces the head exactly;
heads that state more than their ends (a multiplicity, a `references` clause, an inline payload
declaration, a body) stay text-only and are reported, not guessed, when the text is absent.

What remains: the vocabulary is ours, so the hop drops it (`endForm`, `endIndex`,
`relatedFeature` are three of the eight lost keys). The metamodel's shape is
`Connector::connectorEnd` — end features owned through `EndFeatureMembership`s — with
`sourceFeature`/`targetFeature` over them, the same `Connector_sourceFeature`/`targetFeature`
domain findings the ontology gate records for transitions (D8). Emitting those is an
encoder/decoder change, not a parser one; the ends are already in hand. The ends that are not a
basic name (`drive vehicle`, `1stGear`) convert since #814 by quoting them in the head text; a real
end triple would name the element by IRI and need no quoting, which is the same change.

## D7 — reference-valued properties are emitted as strings, and one metaclass is abstract

The reader turns a resource-valued object into `{"@id": …}` and a literal into a string, so a
property the API defines as a reference has to be an element IRI in the graph. `imports.golden.ttl`
shows both halves of this gap: `sysml:importedNamespace "ISQ"` is a string where the API expects
a reference, and the metaclass is `sysml:Import`, which is abstract in KerML — the API's own
elements are `NamespaceImport` or `MembershipImport`.

The reference-vs-literal half is mechanized against the OWL ontology (D8):
`TestGoldenGraphsMatchOntology` (`internal/translate/export`) checks every SysML-namespace triple in
the 54 golden graphs against the metamodel's declared domain and range, finds **412 triples in 79
distinct metaclass/property violations** at this baseline (the count grew with the fixtures the
metadata, result-expression, reference and anonymous-declaration work added, not with new kinds of
disagreement), and
every one is inventoried key-by-key with a reason in
`internal/translate/export/testdata/ontology-known-violations.txt`, so any *new* disagreement fails the
build. The object-property-carrying-a-literal group is this item's own bug: `type` on
`AttributeUsage`, `ReferenceUsage` and `PartUsage`, `sourceFeature` on `SuccessionAsUsage` and
`sysx:InitialNode`, `referent` on `FeatureReferenceExpression` where the referent resolves outside
the graph, and `targetFeature` on `FeatureChainExpression`. #827 and #855 narrowed the *decoder*
side — a name written back re-resolves to the element the graph named, through imports and
aliases — but the encoder still writes the name as a literal, which is what this item is. Identity is stable (D3.1), so each is
mechanical: resolve the name and emit the IRI, and fall back to the literal only where the
referent is outside the graph, as feature references already do. The abstract-metaclass half is
not mechanizable from the ontology: `SysML.owl` records no ecore abstractness (see D8), so nothing
in the suite catches `sysml:Import` being abstract, and that audit against the API's own element
list stays manual.

## D8 — an optional second output profile: the Open-MBEE SysML v2 OWL ontology

[`Open-MBEE/sysmlv2-rdf-ontology`](https://github.com/Open-MBEE/sysmlv2-rdf-ontology) renders the
OMG metamodel (version 202407, from `SysML.ecore`) as OML and OWL: `SysML.owl`, 172 classes, 348
object properties, 63 datatype properties, with `rdfs:domain`/`rdfs:range` on each. It uses the
*same* namespace we do and its class IRIs are the plain metaclass names we already emit; the
difference is the properties, each qualified by the metaclass that defines it
(`sysml:Element_declaredName`, `sysml:Element_owner` with range `OwningMembership`), and a
conformant instance graph therefore materializes the abstract syntax's relationship elements
rather than collapsing them.

So this is a **second profile selected by a flag, not a superset**: the property IRIs differ, so
one graph cannot satisfy both conventions, and Flexo's convention stays the default. The encoder
already separates the term layer (`rdf.SysMLTerm`, `internal/translate/rdf/vocab.go`) from the
structural decisions (`internal/translate/export/rdf_out.go`), so the profile is mostly a term-mapping
layer: property name → defining metaclass.

**Done:** the table and the gate. `internal/translate/rdf/ontology` holds the term table generated
from `SysML.owl` by `tools/gen/ontology` from a local checkout (version `202407`,
upstream commit in the generated header): 411 properties spanning only **336 distinct unqualified
names — 59 names are declared by more than one metaclass** (`type`, `value`, `source`, `target`,
…), so the unqualified convention is genuinely lossy in the other direction and a profile encoder
has to pick by the subject's metaclass (`LookupProperty` returns every declaration;
`AmbiguousNames` reports the set). The gate is `TestGoldenGraphsMatchOntology`, whose inventory is
also the profile's work list, sorted into five causes: properties the metamodel declares on a
relationship or membership element that we collapse into the element (`value` → `FeatureValue`,
the multiplicity bounds → `MultiplicityRange`, `isNegated` → `Invariant`, a transition's ends →
`Connector`) — the same collapse D3.3 undid for ownership and D1/D2 will undo for expressions and
ends; names as literals (D7); metaclass names the 202407 rendering does not have (`FlowUsage`,
which it calls `FlowConnectionUsage`, and `TerminateActionUsage`); the metaclasses of our own
`sysx:` namespace; and the properties we write into the SysML namespace that no metaclass
declares, each either a relationship the metamodel reifies as an element (`specializes`,
`subsets`, `redefines`, `references`, `aliasedElement`, `via`) or a notation flag with no
metamodel property (`isAccept`, `isResult`, `isSnapshot`, `isTimeslice`, `isChain`) — arguably
those belong in `sysx:` regardless of this item.

**Not landed; no open pull request:** [PR #774](https://github.com/JPL-Devin/OpenSysML/pull/774)
on the previous repository (`JPL-Devin/OpenSysML`), which the `v0.6.0` baseline reported as open
with conflicts, is closed there unmerged, and no equivalent pull request has been opened on
`Open-MBEE/OpenSysML`. Nothing under `internal/translate/rdf/ontology` changed between `v0.6.0` and
this baseline other than #142's coverage wiring, and the tree has no `ontology/sysmlv2/`
directory, no `cmd/ontology-modules` and no `make ontology-modules-check`: the modules below
describe that pull request's content, which would have to be re-proposed against the current
`main` to land. It shipped the ontology itself as **41 leaf Turtle modules and 6 layer
ontologies** under `ontology/sysmlv2/`, cut along
the package hierarchy of the normative KerML/SysML XMI (`KerML/Root/Elements.ttl`,
`KerML/Kernel/Expressions.ttl`, `SysML/Systems/Requirements.ttl`, …) with a `catalog.tsv` from
term to declaring module and `owl:imports` computed from use. The generator
(`cmd/ontology-modules`, `internal/translate/rdf/ontology/modules`) errors rather than dropping data —
every source triple lands in exactly one module and the union is graph-isomorphic to upstream —
and `make ontology-modules-check` in CI keeps the committed output equal to the pinned sources
(`scripts/download-ontology-sources.sh`). It is additive to the term table and the Flexo export;
once merged, the profile can import the module a metaclass lives in rather than the monolith, and
a consumer wanting only, say, the requirements vocabulary has a file to import.

**Not started:** the profile plumbing itself — about one session once the table, the gate and
the modules exist. Conformance beyond that is gated on D1 and D2 rather than on this item: `sysx:`
has no place in the ontology, so an ontology-profile graph is conformant only as far as those
have landed, and the profile's documentation should say so.

## D9 — Flexo as a place models live, not only a place graphs are tested

What exists at the baseline, all landed: `-sync-diff <repo.ttl | endpoint>` computes an
identity-keyed change set between the model and a repository branch, reading the branch through
the SysML v2 API when given an endpoint and never writing; `-sync-apply <endpoint>` writes that
change set as one commit or a batched series through `flexo-mms-sysmlv2`, records the commit in
the sync state, refuses a set it cannot apply whole and leaves what did land committed (#791,
`internal/translate/interop/reposync`, `internal/translate/interop/flexo`); and the harness above loads a whole
converted graph into a Layer 1 branch by `PUT` and measures what the service reads back. Both
directions are refereed against the live stack, and both are element-keyed, which is what identity
bought.

What is missing is the round trip a modeller expects from a repository, and each piece is small
now that D3.4 is in:

1. **Read a branch as notation.** `-sync-diff` reads a branch to compare it; nothing converts a
   branch to `.sysml`. `sysml -convert sysml -from <endpoint or flexo:// URL>` is the decoder D3.4
   completed, applied to the graph the service serves, and `-sync-state` already knows the branch
   and the last commit.
2. **Push a whole graph.** The harness's `PUT .../branches/{branch}/graph` with the ETag
   precondition and `?message=` is the fast path for a first load or a re-baseline, where the
   element-wise commit of `-sync-apply` is the wrong shape. Expose it as the write half of the
   same flag, with the token from `flexo.EnvToken` as today.
3. **What survives the hop.** D2 and D1 decide how much of a pushed model the read path gets
   back; the harness's 369 of 452 is the number to move, and it is re-measured, not asserted,
   after each.

Nothing here is a new subsystem; the order is D9.1 → D9.2, and D9.3 is the RDF track's existing
order applied to this use.

## D10 — write-through from a view-only project to the projects it shows

A project under configuration management that consists entirely of views — `view` usages with
`expose` and `filter` over one or more imported projects, each bound by its own
`@ProjectRef` — owns no element worth editing: everything it shows is some other project's, and
with `@ElementId` on the materialized elements that ownership is explicit
([element-identity-annotations.md](element-identity-annotations.md), nested scopes). Today the
sync tooling stops at the document: `reposync.Diff` produces one `ChangeSet` for one project
scope, `GraphScope` refuses a graph carrying two, and an edit made to an element as it appears in
the view project has nowhere to go but the view project's own branch, which is the wrong owner.
The rendered artefacts (`-render`, `-doc`) stay one-way; the round trip in question is
notation-to-repository by id, and it needs three things:

1. **Per-scope fan-out.** A diff over a multi-scope document splits the local graph by enclosing
   `@ProjectRef`, diffs each part against its own branch (`Options.Base` per scope), and applies
   each as a commit to the owning project under its own `flexo.StaleBranchError` guard. No
   partial success across scopes: a set that is not appliable in one scope refuses the run before
   the first write, as `Appliable` does today within one. `-sync-state` records one last-seen
   commit per scope.
2. **Refusing the edit that would leave the view.** If a write-back would take an element outside
   the `filter` of every view that exposed it — dropping the `#systemRequirement` tag on a
   requirement shown only by `RequirementsView` — the element vanishes from the project the edit
   was made in. That is the classical view-update problem, and the answer is a typed refusal in
   the change set (the same tier as the existing *conflict* verdict), overridable only by an
   explicit confirmation like `ConfirmDeletes`. The resolver's inherited view conditions
   (`Resolver.inheritedViewConditions`) already compute the exposing filters, so the check is a
   re-admission test of the edited element against them.
3. **Variant-tagged edits under conditional configuration.** A view project whose exposure
   depends on a variant selection shows one variant's element at a time; an id says *which*
   element, not *under which selection it was shown*, so a write-back must carry the selection
   that was active and refuse when the target branch's selection differs. Elements with derived
   (name-keyed) identity are excluded from write-through altogether: a rename through a view
   would be a delete plus a create in the owner, which is the failure identity exists to prevent,
   so the path requires `@ElementId` and offers `-sync-mint-ids -sync-annotate` to get there.

The gate is the live-stack harness (`TestFlexoInterop`) with a two-project fixture: a view-only
project over two owned projects, an edit through the view lands as one commit in each owner and
nothing in the view project, the filter-violating edit and the variant mismatch are refused before
any write, and a re-run finds nothing to change. Depends on D9.1 (reading a branch as notation,
which is how the view project materializes what it exposes) and sits after D9.2 in the track
order; independent of D1/D2, since it moves whole elements by id and never inspects their
vocabulary.

## D11 — the SysML v2 API element form as a `Convert` format

`Convert` writes notation, `text` and Turtle, and reads the same three. The OMG SysML v2 API's
own element form — JSON objects with `@type`, `@id` and the metamodel's properties as keys — is
read today only as a *measurement*: `flexo.Elements` and `flexo.ElementByID` fetch what the
Flexo API serves after a Turtle load, so the harness can say what survived. Nothing produces
that form from a parsed model, and nothing parses it into one. A framework that wants the
normalized abstract syntax without an RDF store in the middle — a tool exchanging elements over
the standard API, a client comparing two implementations element by element — has no format to
ask for. Adding `api-json` to `Convert`, both directions, is the RDF mapping with a different
serializer: the graph the `ttl` path builds already carries the metaclass and the properties, so
the emitter walks it and the reader is `rdf_in.go`'s inverse over JSON. Reference-valued
properties are `@id` objects, which is D7's question answered a second time, and the collection
annotations D3.4 settled decide array-versus-object. Gate it as the RDF path is gated: a round-trip
ratchet over `examples/`, and the live-stack harness posting the emitted elements to the Flexo API
instead of a graph, reporting what that path keeps that the Turtle one loses or vice versa. After
D1 and D2, since the element form inherits their vocabulary; before D9.2 if the branch read is to
have a choice of representation.

## D12 — the normative element ids of the standard library (done)

KerML fixes the `elementId` of every **named** standard-library element as a name-based UUID
(RFC 4122 version 5): the library package's id is `uuid5(NAMESPACE_URL, <prefix> + <escaped
name>)` with the prefix `https://www.omg.org/spec/KerML/` or `https://www.omg.org/spec/SysML/`
for the KerML and SysML halves of the library, a named descendant's id is `uuid5(<package id>,
<qualified name>)`, and its owning membership's is the same with `/owningMembership` appended.
The pilot's XMI carries exactly these — `ScalarValues::Real` is
`14c0aa22-5489-59b5-b438-ded26e83ba31` under `ScalarValues`'s
`40bb440c-5036-58e1-8675-5afccb8b8f1d` — and so does every SysML v2 API server that serves the
library, so a reference to a library element agrees across tools without either side having
seen the other's model.

Before this item a library element's id was `rdf.EncodeElementID` over its qualified name
(`ScalarValues__Real`), the same derivation user elements get when no
`@IdentityMetadata::ElementId` declares one ([the RDF mapping](../reference/rdf-mapping.md),
*Element identity*). Nothing was invalid — the encoded id is a legal IRI tail and the alphabet
Flexo's `requireValidId` accepts — but a graph, an API payload or an element-by-element comparison
that named `Integer`, `kg` or `Performances::Performance` named an element no other tool had,
and a project on Flexo that types its parts by the library's ids did not resolve against ours.
The encoded name is **reversible** (`rdf.DecodeElementID` recovers the qualified name exactly)
and a version-5 UUID is not, since it is a SHA-1; that is the trade, and the item keeps both: the
normative UUID is the library element's `elementId` and IRI tail, and `sysml:qualifiedName` —
which reading a graph back already takes the name from — stays the readable form.

What landed:

1. **The derivation.** `internal/semantic/identity` (`normative.go`) derives the element and owning
   membership UUIDs from a qualified name, with the pilot's quoting of names; `identity` maps the
   bundled library's tiers to the two prefixes (kernel libraries to KerML, systems and domain
   libraries to SysML) and catalogs every named, non-aliased, non-shadowed library symbol once
   per library index. `TestPilotLibraryXMI` asserts the catalog against the pilot's own
   `sysml.library.xmi` at the pinned release commit (`scripts/download-pilot-library-xmi.sh`,
   through the same `scripts/pilot-pin.sh` the corpora use): every id it derives is an element
   the XMI carries under the same owning membership, and every named XMI element is derived. The
   last to fall in line was the payload an `accept` trigger declares inside a transition
   (`Actions::AcceptAction::aState::aTransition::apayload`): the symbol index now holds it as a
   member of the transition, as the XMI does, so its qualified name — and the id derived from
   it — agree with the pilot's. The test keeps an exact list of pilot-only elements, empty now,
   so a divergence either way fails the gate. CI downloads the XMI and
   sets `OPENSYSML_REQUIRE_PILOT_LIBRARY_XMI=1`, so the test fails rather than skips there.
   Unnamed and implied library elements are out of scope by design — the norm gives them
   positional ids that depend on each implementation's implied-relationship closure, so they do
   not agree even between the pilot and other conforming tools.
2. **The consumers.** The RDF writer's IRI, `sysml:elementId` and owning-membership IRI take the
   normative id for a library element and the encoded name for everything else, and the reader
   does not re-materialize an `@ElementId` for a normative id; the Flexo sync classifies a
   normative id as neither declared nor mintable; `identity.Info` reports which of the three
   sources an id came from (declared, normative, derived) and the language, and the LSP hover
   says so (`Element id \`14c0aa22-…\` (normative, KerML)`) while the minting code action stays
   off library elements. User elements are unchanged: `@ElementId` when declared, the encoded
   name otherwise.
3. **The ratchets.** Neither `TestCorpusRoundTrip` nor the Flexo live-stack expectation moved:
   the corpora convert user models, whose references reach the library by `sysml:qualifiedName`,
   not by id, and the interop fixtures own no library element.

Extends D3's identity work; D11's `api-json` payloads are the first surface where a foreign
reader compares our library ids to its own.

---

# Track F — the executor defects the conformance gate carried (closed)

At `v0.6.0`, `internal/exec/runtime/testdata/conformance/known_failures.txt` named three cases
that `TestExecutionConformance` skipped rather than ran, each a fixture whose expectation is
derived from the Kernel Semantic Library in
[behavior-semantic-oracle.md](behavior-semantic-oracle.md) ("What the executor gets wrong") and
not met by `internal/exec/runtime/action_executor.go`. **All three landed** (#116, #120) and the
file now lists no case — its one remaining line of prose reads "The list is empty: every derived
case passes" — so the gate runs all 770 cases and skips none, and the three rows in
`spec-compliance.md` moved from *known failure* to *faithful*. The track is closed; the items
below record what each fix did and what it deliberately left as it was.

## F1 — a node reached over two successions is performed once, after both (landed)

Fixture `action_node_with_two_incoming_successions_runs_once` (`hits = 1`). **Landed** with F2 in
#116: a plain action node two or more successions reach fires when every incoming succession has
delivered one token, the arrivals collapsing into the one token that performs it. A plain node in
a loop or behind a decision still re-performs once per pass — it awaits a succession only while
some token can still reach its source — so nothing that ran once per token by design now waits.

## F2 — a join counts one token per incoming succession (landed)

Fixture `action_join_one_token_per_incoming_succession` (`log = 12`). **Landed** in #116: a token
records the succession it travelled (`Token.Via`, the lowered `ActionEdge`), a join fires when
each incoming succession has delivered one token, a second token over an already-delivered
succession waits for the next firing instead of standing in for another, and a join one of whose
successions no token can travel deadlocks (`ErrActionDeadlock`) rather than firing on a count.
The REPL's `%tokens` says which succession a held token arrived over and which it awaits. #119
then made a breakpoint on such a node pause once, before its one performance, and made a step
move each token at most once, so traces of forks and joins gained a step boundary between the
last arrival and the node's performance — the same statements in the same order.

## F3 — a merge is re-entered on every traversal of a loop (landed)

Fixture `action_merge_loop_reenters` (`level = 100`, `passes = 3`). **Landed** in #120: a merge
is one `MergePerformance` per arrival, as `Actions::MergeAction` declares, so a loop re-enters it
as often as its guard sends the token back; a fork whose branches both reach a merge yields one
downstream token per branch (collapsing them is a join's job, and a merge is the one
multi-incoming node F2's synchronization does not wait at); and a merge's body runs before the
guard on its outgoing succession is read, so a write in the body decides its own guard. A loop
with no exit still ends on the action step budget (`ErrActionStepLimitExceeded`).

---

# Track S — multiple valid executions

The library states a **partial order** between performances and fixes some outcomes; it does not
order two steps no chain of `HappensBefore` connects, and it gives no conflict rule when two such
steps write one feature ([behavior-semantic-oracle.md](behavior-semantic-oracle.md), "What the
library fixes, and what a trace adds"). At `v0.6.0` the executor picked one linearization —
tokens stepped in descending index order within a step, a fork's branch declared last stepped
first — and every fixture pinned that one: `action_fork_branches_write_one_feature` recorded
`x = 1` as the executor's scheduling, not as a derived value, and its compliance row was
*approximate* for exactly that reason. A conformance case could not say "1 or 2", a trace could
not say where the executor chose, and nothing checked that the linearizations it did *not* take
would also have met the derivation. This track made the choice explicit and checkable. **All four items landed**
(#110, #123, #125, #134), with #141 and #138 as follow-ups; the fork case now admits both `x = 1`
and `x = 2`, 19 cases list `outcomes`, 5 carry a `.trace.order`, and the harness explores every
case that lists `outcomes`. What each leaves is stated under its item.

It was a **prerequisite to Track E**: E1–E7 edit the same executor loops
(`stepActionExecutionNode`, `stepJoinNode`, `stepMergeNode`, the fork and the token list) that
S1–S4 instrumented and parameterized, and a termination, interrupt or streaming semantics added
on top of an implicit scheduling order would have had to be re-derived once that order became a
policy. Track F's three fixes were the same loops again and came first, because a policy over a
join that miscounts is a policy over a defect. Both are in; E waits only on the release.

The unit these items are stated in is the **choice point** (#123): a pick among alternatives the
library leaves unordered — several steppable tokens in one action step, several holding guards
at a decision, several enabled transitions out of one state for one event or change, several
tokens writing one feature in one step, and since #136 and #141 the order several executors due
at one instant run in (`due order`) and the order orthogonal regions react to one event in. Each
is a `choice` trace line naming the alternatives and the one taken, an informational
`choice-point` diagnostic on `ExecuteAction`, `ExecuteState` and `RunAnalysis`, and a count after
`%step`, `%continue` and `%advance`. A guard that cannot be evaluated in the preview is not an
alternative and not an error but a `guard-unevaluable` diagnostic; the innermost-transition-wins
rule between a substate and its enclosing state is spec-defined order and is not reported.

## S1 — admissible outcomes in the conformance schema (landed)

**Landed** in #110. Where the library leaves more than one result open, `.expected.json` lists
every admissible result under `outcomes` and must cite, in `admissible`, the section of the
behavior semantic oracle deriving them — the run must match exactly one, and a missing or
unresolvable citation fails the case — and a `<case>.trace.order` file of `a < b` lines states
the partial order the recorded trace must satisfy, beside or instead of an exact golden. A case
with neither keeps its meaning; the default schedule and every exact golden were unchanged. What
it leaves: `outcomes` is a set of complete results, not a per-feature range, and a case that
lists it is graded by S4's exploration, not by one run.

## S2 — choice points in the execution trace (landed)

**Landed** in #123 as the choice point described above: each step where the library left a pick
open is recorded with the alternatives and the one taken, in the trace, on the wire and in the
REPL summary, while what the executor does — reverse token order, first holding guard, first
declared transition — is unchanged, so every existing result and trace is the same. The guards
and transitions after the first that holds are read in a preview that is undone. #141 added the
one kind the fixed policies had been silent about, the order orthogonal regions react to one
event in (`choice on <trigger>: states <a>, <b> react`), reported under every policy.

## S3 — a selectable scheduling policy, the default unchanged (landed)

**Landed** in #125. The policies are `reverse` (the default: reverse token order, first holding
guard, first enabled transition — every existing result and trace unchanged), `declared` (tokens
in spawn order, guards and transitions in declaration order) and `seed:<n>` (a pseudo-random order
the seed fixes, so one seed replays one run on every platform). One spelling everywhere: `sysml
-schedule <policy>` for `-action`, `-state` and `-analysis` (`-calc` has no choice to make),
`%schedule [<policy>]` in the REPL (applied to the runs started after it; a debugging session
under way keeps its own), a `schedule` field on the three execution requests advertised as the
`schedule` capability and taken as an option by the Go and Python clients, and a `schedule` pin
on a conformance case. A spelling naming no policy is refused before anything runs
(`INVALID_ARGUMENT` on the wire). The conformance suite also runs whole under `declared` and
`seed:1`, requiring every case that pins no policy and lists no `outcomes` to reproduce its
default outputs; that is where the 34 per-policy trace goldens come from. What it leaves: a send
to a same-named port pins `reverse` until the via-less accept that over-matches it is fixed, and
until #141 `seed:<n>` could not vary the order two regions react to one event in — it now draws
and replays it, while `reverse` and `declared` still take declaration order. A5's clock landed on
top of this as the `due order` choice point (#136), as this item said it would.

## S4 — bounded exhaustive exploration against the semantic oracle (landed)

**Landed** in #134. `sysml -schedule explore[:runs=N,depth=D]` runs a behavior once, then
replays it from the start on a fresh executor of the same loaded model, following the recorded
prefix and taking the next untried alternative at the frontier, the first run's choices varied
earliest first, until every choice sequence is spent or a budget is hit (1024 runs and 64 choice points per run by default; hitting
either is exit status `2`, never a silent truncation). Runs agreeing on the observables the
harness compares are one outcome; a run that fails under some order is an outcome of its own. The
responses gain repeated `outcomes` and an `exploration` status, advertised as `schedule_explore`;
the Go and Python clients gain `Explore*`/`explore_*` calls and the Node, Java and Rust clients
the capability name. The harness runs every conformance case that lists `outcomes` under
`explore` and fails when a listed outcome is unreachable or an unlisted one is reached, so the
list is exact; exploring the whole suite found one case pinning a scheduling artefact (two accepts
on one port addressed by two sends), restated as the two outcomes the oracle derives. What it
leaves, by design: the REPL refuses `%schedule explore` with a typed error naming the CLI and the
wire, since `%action` and `%state` step one run (confirmed against `bin/sysml` at this baseline);
cases without `outcomes` are not explored; and the budget is the author's to raise
(`"exploreBudget": {"runs": N, "depth": D}`), not the harness's to sample past. #138 wrote it
up: every open ordering in the oracle names the `outcomes` or `.trace.order` that encodes it and
the run and outcome counts `explore` reaches, and the behavior guide has a section on models with
more than one valid run. As landed, `explore` permuted the tokens of one lockstep step — every
steppable token moved once per step — so a branch of two nodes could never both run before a
concurrent branch's one, and a fork of `left1 { x := 1 } → left2 { y := x }` against
`right { x := 2 }` explored `complete` with two outcomes, missing `x = 2, y = 1`. An action step
under `explore` is now one token advancing one node, the tokens able to act are picked among
afresh after each move, and `complete` covers every interleaving at body granularity
(`action_explore_write_between_branch_nodes` pins the three); the fixed policies keep their
sweep, so no default trace moved, and the oracle's run counts were re-derived at the new
granularity (`action_merge_fork_branch_and_loop` now needs `explore:runs=10000` to complete).

---

# Track E — behavior execution

The runtime executes actions, state machines, calculations and constraints against the lowered
IR (`internal/ir/lower` `ActionGraph`/`StateGraph`, `internal/exec/runtime`), and
`docs/project/spec-compliance.md` § "What We Don't (Yet) Support" lists what it does not
execute: interruptible regions, expansion regions, streaming pins, protocol state machines,
operation invocation with positional arguments, and routing a send to a second object of one
usage; a `terminate` inside a body is refused by the runtime with a typed error and appears in no
list. The behavior-execution review after `v0.4.3` found nothing missing beyond those, and this
track records each as work with a stated scope, dependency order and acceptance gate rather than
as a bullet or an error message alone. **Eligible: the next executor track.** The condition was
"after F and S have landed and a release has shipped with them"; F and S landed (#116, #120,
#110, #123, #125, #134) and `v0.7.0` shipped them, `v0.8.0` followed, and nothing else waits in
front of E. No conformance fixture or trace golden exercises any of the seven items, and each
has a typed refusal or a documented limitation in place of a wrong result. The executor loops E
edits are now the ones F fixed and S instrumented and parameterized, and since #136 they also run
on A5's shared simulation clock (`Context.Clock()`, `Context.Advance`, `accept after`/`at`
parking a token in an action body, `due order` as a choice point), so every E item is written
against a named scheduling policy and a shared clock, not an implicit order. Each item below ends
with what would move it forward; until that happens, the honest status is the "not supported"
bullet or the refusal. E8–E10 are the three findings about the runtime's own conformance in
`docs/internals/design/precise-semantics-alignment.md` that concern state machines: E8 is open,
E9 and E10 landed with the change set that decided the note's open decisions.

Two things about the list's own terms. First, four of the seven items — interruptible regions,
expansion regions, streaming pins, protocol state machines — are UML 2.5.1 concepts that SysML v2
(`formal/2026-03-02`) does not carry as notation: its actions (§7.17) spell termination,
acceptance, loops and flows directly and its states (§7.18) have no protocol variant. Each of
those items therefore starts by naming the SysML v2 spelling it corresponds to, and the target is
that spelling's semantics in the Kernel Semantic Library and the Systems Library, never the UML
feature by name. Second, the proof for every item is the four-layer contract of `AGENTS.md`
§5.2 — a conformance fixture (`.sysml` + `.expected.json` under
`internal/exec/runtime/testdata/conformance/`), a trace golden where ordering matters
(`TestExecutionTrace`, `-update-traces`), a robustness case in `robustness_test.go` for the
failure mode, and the row in `spec-compliance.md` moving out of the "not supported" list — and
every item is self-assessed, since the pinned pilot evaluates expressions and executes no action
or state machine ([pilot-execution-referee.md](pilot-execution-referee.md)).

**Landed ahead of E1–E7**, all merged: the one structural finding of the review
— nested action nodes shared the enclosing action's flat feature space, so `p.v` and `q.v`
collided and same-named outputs overwrote each other — is
[PR #823](https://github.com/JPL-Devin/OpenSysML/pull/823), which gives each performance its own
frame with the callee's pins, precedence-ordered binding (flow payload, then `bind`, then the
declared value) and block-flow nodes inside `if`/loop bodies as real nodes. Message delivery
through a binding connector between an assembly's boundary port and a part's port
([PR #839](https://github.com/JPL-Devin/OpenSysML/pull/839)) fixes `accept … via i` never firing
on the inner part. On the debugging surfaces, [PR #843](https://github.com/JPL-Devin/OpenSysML/pull/843)
makes every expression surface read the instantiated object after `-instantiate` (the stale `0`
beside `%features`' `1`, and `%eval in #1` being an unresolved reference),
[PR #805](https://github.com/JPL-Devin/OpenSysML/pull/805) reads a valueless feature as `<unset>`
in `%eval in` and lists behaviors, [PR #808](https://github.com/JPL-Devin/OpenSysML/pull/808) adds
`%send` and [PR #809](https://github.com/JPL-Devin/OpenSysML/pull/809) one object-reference grammar
for every REPL command, reconciled against #810. What the review found *after* #810 was that
`%state <machine>` in its one-argument form attached a fresh, detached performance rather than the
one materialized object that exhibits the machine (after `%instantiate TA::Sys; %state lp; %advance
2.5 [s]`, object `#1` still read `n = 1` where `%state #1` gave `n = 3`). The cause was a
`StateExecutor` built with no `self`, so `entry`/`do` writes landed in the executor's own frame;
[PR #845](https://github.com/JPL-Devin/OpenSysML/pull/845) makes the machine form walk the held
objects and attach to the single exhibitor's running machine, refuses with a typed error naming the
objects (or, before any object exists, the types) when there are zero or several, and leaves a
truly unbound `state def` running detached; re-run at this baseline the same probe reads `n = 4`
on object `#1` after `%advance 2.5 [s]` (entry, then the timed re-entries at 1 and 2). The double
initial-`do` the review first reported is not reproducible on `main` since #810 and is retired.
Since then: `%state <machine>` on a state machine with a typed body attaches the same way (#856);
the state executor refuses the trigger arguments validation refuses (`accept after 5` without a
unit, a conditional `when`) instead of running them (#833, #871); `send` arguments are validated
and `send new Def(args)` constructs the message it sends (#838, #875); and a message through a
binding connector at a boundary port routes in both directions (#839).

Since the tag, on `develop`, eight state-executor fixes adjudicated in the PSSM referee
([pssm-referee.md](pssm-referee.md)) moved its baseline from 36 `pass` / 23 `fail` /
39 `not-expressible` at the tag to 45 `pass` / 15 `fail` / 38 `not-expressible`, the 3
`terminate-gap` tests (E1's) and 2 `differs-by-design` unchanged: a fork may enter orthogonal
regions that have no initial state (#297); a transition from a substate into its enclosing
composite does not re-enter it (#311); several completion transitions out of one state are one
choice point (#313); a junction with several enabled branches draws one as a choice point
(#318); a join runs the effect of every incoming segment, their order a region-order choice
(#317); a transition into a history pseudostate restores the configuration it is leaving (#295);
the referee's translation carries the values a test's constructor writes (#314) and refuses a
guard whose behavior acts on the model rather than dropping the call (#315). #322 (a join's
segments fire with their own trigger bound, and a refused join is undone whole) is open against
`develop`. E1 then landed: the referee translates the terminate pseudostate, its `terminate-gap`
bucket is retired, and the baseline is 46 `pass` / 17 `fail` / 38 `not-expressible` /
2 `differs-by-design` — *Terminate 003* passes, *Terminate 001* and *002* fail on the
region-entry order the same open finding already covers. E2–E7 have not moved; the referee's
17 `fail` are their measurement on the state side.

## E1 — `terminate` in a body (landed)

**Landed** in two change sets, the action half and then the state and occurrence half (SM38 in
`docs/internals/design/precise-semantics-alignment.md`). The parser had accepted a terminate
action usage in every position the grammar allows and lowering carried it losslessly; the runtime
refused every one with `'terminate' in a body is not executable`. That refusal is gone from
`runtime/action_statements.go` and `runtime/state_statements.go`, and `lower.Effect` names the
terminated occurrence rather than leaving the executors to re-derive it: `Terminates` classifies
the target (`TerminateContaining` for a bare `terminate;`, `TerminateEnclosing` for a terminate
action usage `then stop;` reaches, `TerminateNode` for a name that is an action node of a flow
around the statement, `TerminateOccurrence` for `this` or a feature chain) and `Target`/
`TargetExpr` carry the expression, in the scope it is written in.

*Actions* (`runtime/action_terminate.go`): `then terminate;` ends the performance whose flow it
is a node of with the outputs assigned so far, dropping every other token of that performance —
a forked sibling still running, one parked at an `accept` — lowest token ID first, which the
trace records (`terminate <performance>: dropped tokens …`); `terminate;` in a nested node's body
ends that node and the parent goes on along its succession; `terminate <node>;` ends every
ongoing performance of the named node in the flow around it, earliest begun first, the one
running the statement included, and a performance whose step a parked token had yet to begin is
begun and ended in the same activation (`beginPending`/`endPending`, traced as `ended before it
began` or `ended waiting`) so the result does not depend on which fork branch the scheduler
stepped first. *Occurrences* (`runtime/occurrence_terminate.go` `terminateOccurrence`):
`terminate this;` in a part's behavior and `terminate <feature chain>;` evaluate the target and
end the occurrence's lifetime, with its owned parts, the behaviors it exhibits or performs and
every action or state executor running on it (`Context.endOccurrence`, `endBehaviorsWith`; an
executor whose performer ended between two of its runs ends as terminated at its next,
`performerEnded`). *States* (`runtime/state_statements.go`, `state_route.go` `terminateAt`,
`state_executor.go` `terminateMachine`): a `terminate;` in an `entry`, `do` or `exit` body ends
that behavior at the statement, the state stays active and dispatch goes on; a transition whose
target is a terminate action usage (`accept Abort then stop; action stop terminate;`, §7.18.3)
exits its source and runs its effect, then ends the machine's performance with no further exit,
the running do behaviors abandoned (`abandonMachine`) and no state active — reached directly or
through a choice, junction or join, from a composite state or one region of an orthogonal one.
The outcome is `Outcome.Terminated` with an empty `FinalState`, the executor's state
`StateTerminated` (distinct from `StateCompleted`), the REPL's `State machine terminated` and
`Execution state: Terminated`, the LSP debug snapshot's `terminated`. A `terminate` naming a
performance that ended, a value that is no occurrence, or an occurrence `destroy` emptied is a
typed error (`ErrPerformanceEnded`, `ErrTerminateOccurrence`, `ErrTerminateTarget`,
`robustness_terminate_test.go`); a calculation still refuses it as `ErrCalcSideEffect`
(`robustness_test.go:calc_terminate_is_rejected`). The PSSM referee translates the suite's
terminate pseudostate to the same spelling, which retired its `terminate-gap` bucket
(`docs/project/pssm-referee.md`).

**What it leaves.** A fork branch of a state machine cannot target a terminate action: the
lowering's fork rule takes states as branch targets (`lower/fork_plan.go`), and the spelling
reported rather than routed. The training corpus's `19. Terminate Actions/Terminate Actions
Example-1.sysml` (`MonitoredActivity`) parses and every `terminate` it uses runs, but the model
still stops at initialize: its nested node `performCriticalActivity` is two `perform`s with no
`first` and no succession between them, so the flow-start rule (`lower/case_body.go`
`CaseFlowStart`, `runtime/action_subflow.go` `noFlowStart`) reports two possible starts — `no
succession leads to "monitorCriticalActivity" or to "criticalActivity"` — where §7.17.2 lets
unsequenced nested actions start concurrently when their container does. That is a flow-start
item, not a terminate one, and it is what the example waits on.

## E2 — interrupting an ongoing performance ("interruptible regions")

**Today.** SysML v2 has no interruptible-region notation; what UML models with one is spelled in
SysML v2 as an `accept` followed by a `terminate` in a forked branch (§7.17.10's
`MonitoredActivity`: "Terminates `performCriticalActivity` even if `monitorCriticalActivity` is
still ongoing"), or as a transition leaving a state whose `do` is running (§7.18.3: "If the
source state has a do action that is still being performed, that is interrupted."). The action
half is E1 and landed. The state half runs, with one documented approximation
(`spec-compliance.md` § Known Limitations, *Runtime*): an inline `do` body is one action, so
`runDoRound` advances it as a unit and an outgoing transition interrupts it only between rounds,
never between its statements; the one-action-per-statement `do { … }` form is the interruptible
spelling. There is no refusal here — the approximation is a documented ordering, not an error.

**Target.** §7.18.3's transition semantics, step 1: the source state's do action, "if it is
still being performed, is interrupted" when the transition is triggered — `StatePerformance::do`
is a `step` of the state's performance, and the `StateTransitionPerformance` that leaves it is
keyed on its `accept` step and `transitionLink` (`StatePerformances.kerml`,
`TransitionPerformances.kerml`) — so the do behavior's remaining statements do not run once the
trigger is accepted, whether the body was written as one action or several. For actions, the
target is E1's: the terminate ends the performance whatever else it has in flight.

**Work.** After E1: make an inline `do` body resumable between statements so a round can leave it
mid-body (the statement hosts already run a body statement at a time through `stmtEnv` frames;
what is missing is a do behavior that yields after each statement rather than after the whole
inline body), and drop the pending statements when the state exits. Depends on E1 for the shared
notion of ending an ongoing performance; nothing depends on it.

**Proof.** Conformance and a trace golden for a `do` body of three statements interrupted by a
signal after the first; the existing do-interruption fixtures unchanged; the Known Limitations
bullet removed and the `entry`/`do`/`exit` row in the State Machine map re-stated.
**Prioritize when** a model's result depends on a `do` body being interrupted between two of its
statements — until then the documented spelling (`do { … }` as one action per statement) gives
the same result.

## E3 — concurrent per-element performance ("expansion regions")

**Today.** SysML v2 has no expansion-region notation. Its iterative half is the `for` loop
(§7.17.12, `Actions::ForLoopAction`), which the runtime executes in every body position
(`runtime/action_statements.go`, `runtime/statements.go` `forLoop`), one iteration after the
other, with the step budget bounding it. Its parallel half — the body performed once per element
of a collection, all performances ongoing at once — has no spelling the runtime refuses, because
no spelling for it has been established here: a `for` is sequential by definition
(`ForLoopAction` walks `seq` by an `index`), and a `fork` duplicates control, not a collection.

**Target.** To be settled before any executor work, in a design record under `docs/project/`
like the others: whether SysML v2 §7.17.2's multiplicity on a performed action usage, with a
`flow` delivering the collection to its input, is the standard spelling of concurrent per-element
performance, and what `Performances.kerml` then says about the ordering of those performances.
If the reading is that no such spelling exists, the item closes as "the iterative form is `for`;
the parallel form is not SysML v2" and the bullet is re-worded to say so.

**Work.** The record first; then, if there is a spelling, one performance per element with its
own token and pins, joined when all complete, on top of the concurrency the fork/join executor
already has — and the interaction with E4 (a streaming consumer of the elements) decided with it.
No other item depends on this one.

**Proof.** Conformance for a collection of three performed concurrently with per-element outputs
collected; a trace golden for the interleaving; robustness for an empty collection and a body
that fails on one element. **Prioritize when** someone needs it: a model with a per-element
behavior whose sequential `for` result is wrong or too slow.

## E4 — streaming flows ("streaming pins")

**Today.** A `flow` between two action parameters executes as a *succession* flow: the value
at the source pin is moved to the target pin when the source node completes
(`runtime/action_executor.go` `applyDataFlows`, called from the node-completion paths), so the
target reads it when its own token arrives. SysML v2 §7.16 draws the distinction the runtime does
not: "the input and output parameters are streaming unless designated as succession flows" — a
streaming `flow` "can be ongoing while both the source and target action are being performed",
while a `succession flow` "cannot begin until the source completes". The parser keeps the
distinction (`ast.Usage.IsSuccessionFlow`, used by the control-node succession rule), but
`lower.ObjectFlow` carries no kind and no position refuses anything — both spellings run, both as
the succession reading. That is a wrong
result only for a model whose target reads before its source completes, and no conformance
fixture writes one.

**Target.** `Flows::Flow :> Message, FlowTransfer` for a streaming flow and
`Flows::SuccessionFlow :> Flow, FlowTransferBefore` for the succession form (`Systems
Library/Flows.sysml`, `Kernel Semantic Library/Transfers.kerml`): a streaming flow transfers each
value the source parameter takes while both performances are ongoing; a succession flow transfers
after the source completes. What runs today is the second, applied to both.

**Work.** Carry the kind from the AST (`ast.Usage.IsSuccessionFlow`) through
`lower.ObjectFlow` to the executor; keep the succession behavior for the `succession flow`
spelling; for a plain `flow`, deliver on each write to the source parameter while the target is
ongoing, which needs a node to be readable while it still holds a token — the same notion of an
ongoing performance E1 and E2 introduce. Depends on E1 for that notion; E3's parallel form would
feed it.

**Proof.** Conformance: a producer loop writing three values to an `out` streamed to a consumer
that accumulates them, with the `succession flow` variant of the same model receiving only the
last; a trace golden for the interleaving; robustness for a stream whose source never writes.
`spec-compliance.md`: the Actions map's object-flow rows split by kind and the bullet leaves the
list. **Prioritize when** a model's result differs between the two readings — a consumer that
reads before its producer completes.

## E5 — protocol state machines

**Today.** No SysML v2 notation exists for a protocol state machine (UML 2.5.1 §14.4), so nothing
is parsed, lowered or refused; the bullet in `spec-compliance.md` is the whole record. What SysML
v2 does have is a state machine exhibited by an occurrence (§7.18.4 `exhibit`), which the runtime
runs during materialization of an object of the exhibiting type (the Classifier Behaviors map).

**Target.** None is stated, and none should be invented here: a UML protocol state machine
constrains the order of operation calls on an interface, and the SysML v2 rendering of that
constraint is a design question — an exhibited state machine on a port definition, with an
out-of-order message refused as a typed error, is the obvious candidate — to be settled in a design
record if the need arises.

**Work.** The record; then whatever it concludes. Independent of every other item.

**Proof.** Set by the record. **Prioritize when** a user brings a model that needs the order of
messages on a port checked at run time; until then the bullet stays as it is.

## E6 — operation invocation with positional arguments

**Today.** `Context.InvokeOperation(inst, name, args map[string]Value)`
(`runtime/invoke_operation.go`) runs a member of an object's type with the object as performer,
whichever behavior the member is — an action through `ExecuteActionPerformedBy`, a calc through
the calc invocation with the object as its featuring object, a constraint through condition
evaluation — and `TestInvokeOperationPerformedByTheObject` (`runtime/classifier_behavior_test.go`)
covers all three. The compliance bullet used to name an operation "given as a `calc` or
`constraint`" beside the positional form; that half is closed by the Classifier Behaviors row that
says so, the bullet is re-worded to the positional form with this record, and this item is that
form only. Arguments bind by name and only by name: `operationInputs` takes a map, binds each
`in`/`inout` parameter by its name and refuses a missing one (`ErrUnboundParameter: parameter …
has no argument and no default`) and an unknown one (`ErrUnboundParameter: … is no input
parameter of operation …`). The REPL's `%invoke <object> <op> [<p>=<expr>]` (`repl/meta.go`
`operationArguments`) refuses an argument not written `<parameter>=<expression>`. There is no
positional form on either surface, and no refusal specific to one — the row says so: "no
invocation surface expresses positional operation arguments". An operation call *written in a
model* is an `InvocationExpression` and is bound by the expression machinery, where positional
arguments already work (`runtime/invoke_calc.go` `bindCalcParameter`; for a performed action,
`runtime/invoke_action.go` `bindArguments` binds `inv.args` in parameter order and refuses a
surplus with `action … takes N input parameter(s), got M argument(s)`).

**Target.** KerML 1.0 §8.2.5.8.3 gives an invocation's `ArgumentList` as either a
`PositionalArgumentList` or a `NamedArgumentList`, never a mix, and §8.4.4.9.5 binds a positional
list to the behavior's parameters in declaration order (`feature a redefines F::a = e1; feature b
redefines F::b = e2; …`) — as `bindArguments` and `bindCalcParameter` already do for a call in the
model. The API and the REPL should offer the same two forms, so `%invoke rover1 drive 10 20` binds
`10` and `20` to the first two `in` parameters, a list mixing the two forms is refused, and a
surplus is refused with the same arity error.

**Work.** Small and self-contained: an ordered argument list on `InvokeOperation` (or a second
entry point) sharing `bindArguments`'s rules, and `%invoke` accepting a list of bare expressions
in place of its `<p>=<expr>` pairs. Depends on nothing; nothing depends on it.

**Proof.** `runtime/classifier_behavior_test.go` and `repl/classifier_behavior_test.go` gain the
positional, mixed and surplus cases; `robustness_test.go` the arity failure; the bullet leaves the
list. **Prioritize when** a REPL or API user asks for it — it is the smallest item in the track
and the one most likely to be done on demand.

## E7 — an addressed send to a second object of one usage

**Today.** A `send … to <target>` resolves its target through `runtime/signal.go`
`resolveAddresses` → `featureAddresses` → `addressOwner`: a name that is a feature of the sending
object (or of an object holding it) reaches that feature's value, and otherwise the shortest prefix
naming an occurrence usage that `occursOnce` reaches *the* occurrence this context holds for it
(`ctx.occurrenceOf(sym)`, `runtime/instance.go`). A `via` send follows the connections
(`runtime/routing.go`, `signal.go` `postVia`) to the object at the other end. In both forms the
object reached is the one this context materialized as the usage's occurrence; a second object
instantiated of that same usage is a different object, which a send addressed to the usage does
not reach. There is no refusal: the send reaches the held occurrence, and
`spec-compliance.md` § Known Limitations (*Standard behavioral notation*) documents it.

**Target.** §7.17.7: a `SendAction` has three input parameters — the payload, a *sender*
occurrence (`via`) and a *receiver* occurrence (`to`) — and "the behavior of a `SendAction` is to
transfer the payload from the sender to the receiver"; the receiver is a value the `to` expression
(or a flow or binding into the `receiver` parameter) supplies, and the message is a
`MessageTransfer` between those two occurrences (`Transfers.kerml`). So a send whose receiver
expression yields a particular object reaches that object, whichever usage it was instantiated
from.

**Work.** Address by value rather than by usage: evaluate the `to` expression to an object
reference and post to that object's identity (`objectID`), with the by-usage resolution kept for a
target that is a name. That is only meaningful once a context can hold more than one object of one
usage, which is the "dynamic object creation/destruction" bullet beside this one in the compliance
list and outside this track: an object materialized by `new` or held in a feature the sender
reads. Depends on that object-model item; independent of E1–E6.

**Proof.** Conformance: two objects of one usage, a send addressed to the second, only the second's
accept fires (with the `via` form of the same model); a trace golden; robustness for a target
expression yielding no object. `spec-compliance.md`: the Known Limitations bullet and the "not
supported" bullet both leave. **Prioritize when** the object-model item lands, since without it
there is no second object to address.

## E8 — `isRunToCompletion` and `runToCompletionScope` redefinitions

**Today.** `Kernel Semantic Library/Occurrences.kerml` declares, on every `Occurrence`,
`isRunToCompletion: Boolean [1] default true` — "determines whether transition performances might
happen during state entry performances within the run to completion scope" — and
`runToCompletionScope: Occurrence [1] default self`, and `StatePerformances.kerml` redefines both
on `StatePerformance` (`default this.isRunToCompletion`, `default this.runToCompletionScope`) with
the invariant that under `isRunToCompletion` every `TransitionPerformance` within the scope
precedes or follows the state's `entry`. The runtime implements the defaults and only the
defaults: `runtime/state_executor.go` `runStep` → `processNextEvent` dispatches one occurrence
per step and `enterStateInto` runs a state's entry behavior and its regions' initial entries to
the end before the step returns, so no transition fires during an entry; the scope is always the
whole machine, since `run` is one loop over one `eventQueue`. A model that redefines either
feature to anything else — narrowing the scope to one composite, or switching run-to-completion
off so that a transition may fire while a sibling's entry is still performing — is refused when
the machine is lowered: `lower/run_to_completion.go` `refuseRunToCompletionRedefinitions` judges
the redefinition each body makes effective (its own, or the one it inherits from a specialized
definition, the target resolved to its library symbol so an alias is caught) and returns the typed
`lower.RunToCompletionRedefinition`, an `ErrUnsupportedStateContent` naming the feature, the
declaring state and the value written, for `false`, for a scope other than the machine itself, and
for a value it cannot verify restates the default. A redefinition restating the default (`= true`,
`= self` on the machine) runs. Before that, such a model was run under the defaults with no
diagnostic. The lowered `StateGraph` still carries neither feature; the precise-semantics
alignment note records the refusal, and the remaining gap, against its SM1 row.

**Target.** The two declarations above, read from the model. `isRunToCompletion = false` on a
scope means the library no longer orders transition performances within that scope against
entry performances, so the executor may — and under `explore` must — interleave a dispatch with
an ongoing entry there; `runToCompletionScope` names the occurrence within which the ordering
holds, so a scope narrower than the machine leaves transitions outside it free to fire while a
state inside it is entering. A redefinition that names no occurrence, or a scope that is not an
ancestor of the redefining state, is a typed error.

**Work.** Lower both features per state into the `StateGraph` (the value expression, or the
inherited default), resolved through the same redefinition walk `lower/state_graph.go` uses for
entry transitions; give the state executor a per-scope step boundary in place of the single
`runStep` one — a dispatch that arrives while an entry is performing is held at the boundary of
the innermost enclosing scope with `isRunToCompletion` true, and taken as a move where it is
false — and record the interleaving as a choice point (`scheduling.md`) so `explore` enumerates
it and `seed:<n>` replays it. The default configuration must run every existing fixture and
trace golden unchanged. Independent of E1–E7; touches the loop E1 and E2 also edit.

**Proof.** The refusal is pinned: conformance `state_run_to_completion_redefined_false`,
`_scope_narrowed`, `_inherited_redefinition`, `_region_redefinition`, `_unverified` and
`_alias_redefinition` expect the typed error, `_defaults_restated` and `_default_restored` run,
`robustness_test.go` `run_to_completion_*` match it with `errors.As`, and
the state rendering (`view/render_test.go`) and the REPL's `%state` (`repl/runtime_commands_test.go`)
show the same message. The implementation adds:
conformance for a redefinition to `false` on a composite whose entry sends a signal the composite
itself accepts, pinning that the transition fires during the entry where the default holds it
until after; a narrowed scope with a sibling region's transition firing during the scoped state's
entry; the default unchanged. A trace golden for the interleaving order. Robustness: a scope that
is not an ancestor. `spec-compliance.md`: the run-to-completion row gains the two features with
file:function in place of the refusal; the alignment note's SM1 row and its finding move to
agreement. **Prioritize when** a user model redefines either feature — none in the corpora does
today — or when the model checker, which searches a machine's transitions, region orders, event
and due orders but takes run-to-completion as the library fixes it, needs the interleaving as a
move.

## E9 — a composite state's completion fires its own completion transition (landed)

**Landed** with the state-machine rules of `docs/internals/design/precise-semantics-alignment.md`
(SM11). Before, a `then done;` in a composite state's body ended the whole machine
(`completeIfDone` → `machineComplete`) and a nil-trigger transition out of that composite was
never scheduled, so `state outer { … then done; } transition first outer then next;` never
reached `next`. `States.sysml` binds `done` to the `StatePerformance::endShot` of the state whose
body names it, and `TransitionPerformances.kerml` places a transition's effect and target after
its source's performance, so the machine's end there had no basis in the library. Now `done` in a
composite's body lowers to that composite's own completion vertex
(`lower/state_graph.go:completionOwner`); once its do behavior and every region have ended the
composite completes and `scheduleCompletedComposites` → `scheduleCompletionTransitions` queues its
nil-trigger transitions as completion events at the current instant, ordered as a leaf's are; and
`machineComplete` ends the machine only when its top-level regions are all at `done`. A completed
composite with no enabled completion transition stays completed and active — both PSSM
(§8.5.9, the completion event is lost) and SysML v2 §7.18.3 ("does not necessarily terminate
immediately") agree that nothing ends there, and neither says more. Pinned by
`state_outer_completion_to_next`, `state_composite_completion_then_machine_done`,
`state_composite_completion_nested`, `state_composite_completion_inside_region`,
`state_completion_nested_regions` and `state_entry_transition_nested_done` (each rewritten with a
completion transition out of the composite), their `_stay_active` siblings, and the unit tests in
`state_completion_test.go`; `spec-compliance.md`'s completion rows carry the rule.

## E10 — a choice's guards are read after the incoming effect; a junction's before (landed)

**Landed** with the same change set (SM30). `pseudostates.md` had always described a choice as a
dynamic branch whose guards are read when it is entered, while `resolveRoute` picked the branch
for choice and junction alike before the incoming transition's effect ran, and a code comment
called the two indistinguishable for a guard over state data. Now a route is settled before firing
only up to the first choice — junctions along it statically, as before, a junction with no
enabled branch still meaning the transition is not enabled — and firing exits the states every
branch of the choice leaves, runs the effects into it and only then reads its guards
(`state_route.go:travel` → `resolveChoice`); several enabled is the existing `ChoiceTransition`
point at `choice <name>`, enumerated by `explore`; none enabled is the typed
`ErrChoiceWithoutBranch`. On a chain each pseudostate follows its own rule at the point the route
reaches it. Pinned by `state_choice_after_incoming_effect` (`assign x := 1 then pick; … if x == 1
then seen` reaches `seen`), `state_choice_dynamic_conflict`, the three
`state_pseudostate_chain_*` fixtures, `TestExploreDynamicChoiceBranches` and
`robustness_test.go:state_choice_without_an_enabled_branch`; every other `state_choice_*` fixture
kept its outcome.

---

# Track X — expression forms the evaluator did not reach

Expression evaluation is the most externally refereed part of the runtime: when
`cmd/pilot-exec-diff` was re-run for the review (2026-09-02, at `1f136d27` — a snapshot of that
round, not the current baseline), 55 cases agreed with the pinned pilot, 1 agreed in kind only, 1
disagreement and 2 errors of ours were adjudicated, and 35 cases the pilot cannot evaluate; 70 of
the 71 grammar forms in `KerMLExpressions.xtext` have a corpus witness (only `%` lacks one).
Scalars, Booleans, strings, enumerations, quantities, `Complex` (#777, #788), arrays, vectors and
vector quantities (#883), rationals (#876), occurrence lifetimes (#884), collection bodies and the
named reducers (`->reduce '+'`, `builtinControlReduce`), indexing, qualified names, the lazy
conditionals, `??`, the Kernel Function Library with overloads by argument type, the step and
element budgets, RDF expression trees and the native scalar/collection fast path are done. The
review after `v0.4.3` found two defects, both landed (#794: `x @ T` on a value is `istype`, with
`@@` still the metadata classification; #795: a feature named `chain` resolves as a name where the
lookahead does not establish the `chain` modifier — `step chain …` still does not parse and is a
separate parser item). Since then the 0.6.0 release closed the first two items of this track:
constructors evaluate in a value position (X1, #981) and a singleton sequence takes part in
arithmetic as the scalar (X2, for a feature's own value), 0.7.0 closed five more — casts (X3,
#115), `*` and `.metadata` (X4, #113), function values (X6, #122), the set kind and rank-n tensor
quantities (X7's value half, #121) and the static type of a collection body (X8's typing half,
#112) — and 0.8.0 closed X2's chain-read half (#164), evaluated the `meta` cast (#212) and typed
a bare feature reference, an untyped collection body and an argument of unknown type statically
(#174, #213, #241). What follows is
measured against `runtime/eval.go` and `bin/sysml` at this baseline: each landed item is stated
with what it leaves, and what is still open in the track is X7's RDF literal form and native
layout, and X8's two harness halves.

## X1 — constructors: `new Pt(1, 2)` as a value (landed)

`ast.ConstructorExpr` parses with its positional and named arguments (#838, #875), types as the
definition it names with its arguments checked against that type, and since #981 evaluates in a
value position: `runtime/eval.go` dispatches it to `runtime/signal.go` `evalConstructor`, which
creates an object of the definition in the evaluating context with the arguments bound to its
features — positionally in declaration order (`new Pt(1.0, 2.0)`) or by name (`new Pt(y = 2.0,
x = 1.0)`), both reading back `p.x = 1.0`, `p.y = 2.0` — the same path `send new Def(args)` had
used for its payload. What depended on it moved: `new SampledFunction(samples = (new
SamplePair(…), …))` constructs and `Domain` reads it (L7), and an analysis case builds its own
records (A1). What remains is on the compiled side: the interpreted/native differential cannot
cover a constructor until N2.1 gives the compiled side a record type.

## X2 — a singleton sequence is the scalar (landed)

**Landed** in #164, completing what the feature's own value already did. KerML has no
scalar/sequence distinction: a one-element sequence *is* the value, and `[0..*]` features with
one value take part in arithmetic. Both directions hold for a feature read directly: `attribute
xs : Real[0..*] = (2.5);` gives `xs + 1.0 = 3.5`, and the mirror, `attribute ys : Real[0..*] =
one;` with `one : Real`, reads `1.0` where a sequence is expected. The half that was open at
`v0.6.0` — a singleton read *through a feature chain on an object*, `sp.domainValue` on a
constructed `SamplePair` (a feature redefined without its own multiplicity, so it inherits
`KeyValuePair::key`'s `[0..*]`), arriving as the sequence `[0.0]` and refused by every operator —
is closed by one binding rule serving every place one value is taken: an operator's operand, a
`[1]` parameter of a user calc or a library function, a cast or classification operand. A
collection of several stays what it is and is refused as before (`type mismatch`, `multiplicity
violation`), an empty one still holds no value, and the multiplicity checks are not weakened:
`attribute two : Real[2..*] = (1.0);` is still refused by the checker as `1 value(s) bound to a
feature with multiplicity lower bound 2` before anything evaluates. `calc_sample_pair_arithmetic`
pins the chain reads, and `interpolateLinear` runs (L7). What it leaves, by design: nothing — the
row in `spec-compliance.md` is faithful.

## X3 — casts: `r as Integer` (landed)

**Landed** in #115. `x as T` selects the values of `x` that `T` classifies, in order, and answers
the empty sequence when none does — against `bin/sysml` at this baseline `2.5 as Integer` is `()`
and `2.5 as Real` is `2.5` — with the semantics the metamodel gives: not a conversion
(`ToInteger` and its siblings remain the library functions that convert). Scalars are judged by
their magnitude against the `ScalarValues` hierarchy, quantities by whether their unit is
commensurable with the target's dimension, arrays, vectors, tensors, measurement references and
frames by shape, units and frame, objects and enumeration literals by the types they carry; a
composed target (union, intersection, difference, nested to any depth) classifies as its operands
do, and every type a value's feature is declared with counts among the types it is of, so a
custom scalar subtype keeps the values declared with it. `as`, `istype` and `hastype` are
model-level evaluable, so a metadata body may bind `x = 1 as Integer`. What it leaves, by design:
a target that neither a value's types nor its content settles is reported rather than the value
being silently dropped. Five cast cases were added to the pilot execution referee
(`cmd/pilot-exec-diff`) with the change.

## X4 — `*` as a value, and `.metadata` (landed)

**Landed** in #113. `*` in an expression position evaluates to the unbounded value: it exceeds
every finite Integer, Real and Natural (`* > 1000000` is `true` against `bin/sysml`), equals
itself, prints as `*` in the REPL and in traces, and crosses gRPC on its own `Value.infinity` arm
under the `infinity_value` capability, never as the string `"*"`. `elem.metadata` yields the
metadata annotating the element as a sequence of metadata instances in declaration order, with
the values the annotation body binds and the metadata type's defaults where it binds none; an
element with no metadata yields the empty sequence. What it leaves, by design: arithmetic over
`*` is refused with a typed error naming the operation rather than answering an infinity or a
NaN, and reading `.metadata` off a value rather than an element is a typed error.

## X5 — `all` (landed)

**Landed** in #211, #238 and #239. `all T` (KerML `ExtentExpression`, `BaseFunctions::'all'`)
answers the instances of the named type as an ordered sequence, statically `T[0..*]`, over the
whole loaded model: every object the run holds and every namespace-level object usage of every
document that may hold a `T` — an imported package's, an un-imported one's and the standard
library's alike, so `all Clock` answers `Time::universalClock` — materialized as the extent is
taken, with a variation's extent its variants, an enumeration's its literals, and a usage of
several occurrences (`part wheels : Wheel[2];`) denoting its lower bound of objects. What it
leaves, by design: a data type's extent (`all Integer`, `all Point`) is refused with
`ErrUnboundedExtent`, since a run creates no data values to enumerate; a namespace-level port or
open-count collection that may hold a `T` refuses the extent with `ErrExtentUnavailable` naming
it rather than answering short; `all T` is never model-level evaluable; and the native compiler
keeps refusing `all`, a compiled program having no run whose extent it could report. The
named-reducer half landed earlier: `->reduce '+'` and the other named spellings resolve to the
library function by name through L4's dispatch (`runtime/collections.go` `builtinControlReduce`;
conformance `calc_library_complex_sum_real_axis` exercises the Real-axis case). The `extent_*`
conformance cases pin the operator; the query side of a runtime population is Q2's (#267).

## X6 — function values (landed)

**Landed** in #122. A calc definition, a calc usage awaiting an input, or an `in calc` parameter
named where a value is expected is a function value — the calc together with the scope and object
it was read in — invoked through a calc-typed parameter, passed positionally or by name, read off
a part, returned from a calc, compared and adopted: a model's own `Apply(Sq, 3.0)` with `in calc
f { in x : Real; return : Real; }` answers `9.0` against `bin/sysml`, `SampledFunctions::Sample`
samples a user calc (L7), `TradeStudies::evaluationFunction` binds one (A2), and a library
function the runtime implements (`RealFunctions::sqrt`) is a value too. A calc declared in a
behavior body closes over the innermost active run of that behavior alone, never a caller's
parameters. Calling a non-function, an arity mismatch and an unbound calc parameter are typed
errors. On the wire `Value.function` carries the calc's qualified name and the id of the object
it was read off, under the `function_values` capability the five clients expose as a typed value.
What it leaves: a function closing over a behavior body's bindings crosses the wire as an
unsupported null, since no name reconstructs it; one read off an object is refused as an argument
to a later call, since that object lived only within the response that sent it; and native
compilation refuses a calc that binds or applies a function value with a typed error (confirmed
against `bin/sysml -compile` at this baseline). It is not an arbitrary closure over statements,
and was not meant to be.

## X7 — tensors and set-producing expressions (values landed; RDF literal and native layout open)

The representation decisions are all taken. Since #883 a `Collections::Array` is a `ValArray`
with its dimensions and row-major elements, a `NumericalVectorValue` a `ValVector`, and a
`VectorQuantityValue` a `ValVectorQuantity` with a unit per axis. **Landed** in #121: where the
Kernel Data Type Library declares a collection's `elements` unique and unordered — `Set`,
`UniqueCollection`, `Map` — the runtime holds a set value, each member once, `size` counting
members, equality ignoring the order the members were written in (confirmed against `bin/sysml`),
`contains`/`containsAll` as membership, and one canonical enumeration order when an ordered
operation consumes it; what the library declares ordered or nonunique is unchanged. A
`TensorMeasurementReference` with three or more `dimensions` builds a tensor of that rank whose
`#` takes one index per dimension, with the wrong index count, an index out of range, a non-Integer
index, a component count off the flattened size and arithmetic between two shapes each a typed
error; the shape survives `+`, `-` and the scalar multiplications. Both cross gRPC whole on `set`
and `tensor_quantity` arms under the `set_values` and `tensor_values` capabilities, decoded by the
five clients into native types that check their own invariants. **What remains open**, as #121
states it: neither value has an RDF literal form — the mapping writes the model's expressions,
which round trip exactly (`TestSetAndTensorValuesRoundTripAsExpressions`) — and neither compiles
natively: `sysml -compile` refuses a calc that uses one with a typed error naming the type
(confirmed at this baseline). Still last in the track, and now only those two halves.

## X8 — static element types through collection bodies (landed), and the two harnesses (open)

The typing half **landed** in #112: a collection operation's static type follows what its
declaration hands through, not the element type of the collection — `xs->collect { in x : C;
x.mass }` and `xs.{ in x : C; x.mass }` are typed by the body's result, a nested collect by its
innermost body, `xs->collect f` by the named function's result, `select`/`reject`/`selectOne`
keep the elements of `xs`, `reduce` follows its reducer (or the one element a one-element
collection hands back unreduced), `forAll`/`exists` stay `Boolean`, and a body whose result
cannot be typed keeps the library's `Anything`. Value conformance, bound values, invocation and
trigger arguments and enumerated values are judged by the specialized type, so `accept when
counts.{in n : Integer; n}` is refused where it was silent and `when counts.{in n; n > 3}` is
accepted where it was refused; `xs.?{…}` types as `xs->select {…}` does, and sibling element
types share their nearest common supertype. (#903 had made the typer terminate on the Apollo 11
mass rollup without changing what it answered; this changes the answer.)

The two harness halves are **open, unchanged since `v0.6.0`**. Interpreted and compiled
evaluation are held equal by one differential over the compiled subset; the pilot differential
(`cmd/pilot-exec-diff`) still needs its normalization of numeric spellings and an adjudication
file the harness reads to say why each remaining disagreement is ours or the pilot's — the
adjudications live in prose in `pilot-execution-referee.md`, and what `cmd/pilot-exec-diff`
gained since `v0.6.0` is cases (#115's five casts, #235's stdin-fed engines), not the
normalization; and the RDF expression trees have no round trip of their own — a tree is written,
read and rewritten only as part of a whole model, `internal/translate/export` having gained #121's set
and tensor values, the canonical predicate order (#166) and the library copies' normative identity
(#276) but no tree-level harness. Two hygiene items that make every X change measurable; do them
first when the track is next picked up.

---

# Track Q — queries over the running model

Four query surfaces exist and are landed: the standard API `Query` over a project's elements
(`internal/frontend/grpc`, the OSLC query grammar with its diagnostics — #798, #812), the native document
query (`query def` with parameters, planned by `internal/ir/queryplan` and run by
`internal/doc/queryexec`, from the CLI, the REPL and gRPC), `Evaluate`/`-eval`/`%eval in`, and the
solver's `solve`. Two of the four answer from the runtime: `Evaluate`/`-eval`/`%eval in` since 0.8.0 —
`all T` enumerates the objects a run holds (X5) and `%eval in #1` reads an object's current values
— and the document query since #267 and #293 on `develop`, whose rows may be the objects
`%instantiate`, `-instantiate` and the service's `Instantiate` create, whose operations read what
they hold now, and whose parameters bind to a held object from the REPL, the CLI and gRPC alike.
The API `Query` reads the *model* alone; the solver decides satisfiability rather than reading
what holds, though `%solve` pins the values a matching held object has before synthesising the
rest. No surface reaches the trace `-trace` prints. That is the rest of the track.

## Q1 — say which query is which (done)

One page distinguishing the four: document queries over elements, the API `Query` over a project,
`Evaluate` over one expression in one scope, and `solve` over constraints; what each returns, what
each cannot see, and where runtime queries (Q2–Q4) sit. **Done**: the document-generation manual's
[Which query is which](../manual/query-kinds.md) is that page — one row per surface with what it
reads, what it returns, what it cannot see and its entry points, then a section apiece — and the
manual's [interfaces](../manual/interfaces.md) and [outputs](../manual/outputs.md)
chapters carry the runtime side of the document query: a parameter bound to a held object by name,
`#id` or path from the CLI, the REPL and the Python client (`ObjectRef`), `-instantiate` beside
`-render-document`, `Verdicts` rows (`DocumentVerdict` in the Python client) and the `data-object`,
`data-verdict` and `data-path` attributes the HTML carries. Each later item points at that page
rather than re-explaining the boundary.

## Q2 — a runtime population: `all T`, and predicates over instances (landed; the query side unreleased)

`all Vehicle` (every object typed by `Vehicle` in the session), and the collection operations over
it — `all Vehicle->select { in v; v.mass > 1000 [kg] }` — are the expression form of a runtime
query, and that half **landed** with X5 (#211, #238, #239) in the tag: the extent is a value,
statically `T[0..*]`, the collection operations run over it, and it is taken over the whole loaded
model. The query side **landed** in #267, on `develop` after the tag: a document query's rows may
be the objects a session holds (`queryexec.ValueObject`, a `*runtime.Instance` under the path the
session reaches it by), every operation that takes an element row takes an object row and reads
what it holds now — `OwnedElements`/`Descendants`/`Ancestors` walk the objects held and holding,
`WhereType` tests the object's types, `WhereName` its path, `WhereFeature`/`Project`/`OrderBy`/
`Column` its current values — `DocumentQueries::Objects(type = T)` is the population as a query
operation, refused outside a session, `%run-query`/`-run-query` bind a parameter to the object held
under a usage's name, to `#id` or to a path (`car.wheels[2]`), `-instantiate` is accepted beside
`-render-document` so a document reports `car.wheels[2]`'s current value and not its declared
default, and objects render by path in Markdown, HTML (`span.sysml-object`, `data-object`) and PDF.
`RelatedElements` stays over elements and refuses an object row. Its companion #263 validates an
object as a whole (A7) over the same carrier walk, and #289 puts that sweep in a query:
`DocumentQueries::Verdicts(source, kind)` answers one row per assertion about each row's object —
the held object, or the element's declared one — and the objects it holds, the assertion as the
row's element with `kind`, `carrier`, `path`, `verdict`, `condition`, `reason` and `verification`
read by `WhereFeature`, `OrderBy`, `Project` and `Column`, so "which requirements does *this* car
violate" is a `WhereFeature` over `Verdicts` in a document; verdicts render in Markdown, HTML
(`span.sysml-verdict`) and PDF and cross `RunDocumentQuery` as the `verdict` arm of
`DocumentValue`. The last piece, the gRPC binding of a held object as a query parameter,
**landed** in #293 on `develop`: `Instantiate` keeps the object it creates, in one runtime per
cached model, for as long as the model stays cached (instantiating the same usage again denotes
the new object and keeps the earlier one by id); `RunDocumentQuery` binds a parameter to such an
object through the `object` arm of `DocumentValue` — a `DocumentObject` naming it by
`instance_id`, by `path` in `%run-query`'s grammar (`car`, `Garage::car`, `#2`, `car.wheels[2]`)
or by both — and answers an object row or an object-valued cell as the same arm with the
object's id, path and usage; `RenderDocument` renders over the same population; a binding while
nothing is held or naming an unknown object is `NOT_FOUND`, a path that reaches no object, an
out-of-range index or an id its path disagrees with `INVALID_ARGUMENT`;
`OPENSYSML_GRPC_MAX_HELD_OBJECTS` bounds what one model holds and an operation that would pass it
fails whole with `RESOURCE_EXHAUSTED`; the Go client binds with `ObjectByID`/`ObjectByPath` and
the Python client with `ObjectRef`, both decoding the object cell, and the Node, Java and Rust
clients carry the regenerated stubs. Nothing of Q2 is left; the rows in `spec-compliance.md` say
so.

## Q3 — state and event queries

"Which state is `#1.lp` in?", "which objects are in `run`?", "what did `#1` accept between
`t = 1 [s]` and `t = 2.5 [s]`?" — the state executor and the trace have the answers
(`StateExecutor.getCurrentState`, the event log `-trace` prints) and no query reads them. Q3 is a runtime
query vocabulary over current state and over the trace as a time-ordered relation, with the same
filter forms as Q2, so the trace stops being something one reads by eye. Its prerequisite on the
trace side is met: A5 landed (#136), so the clock Q3 reads is `Context.Clock()`, shared by every
executor in a context, and the trace now carries `choice` lines (S2) and `due order` draws that a
query over it would need to see; the representation Q3 would read is the one at the tag, not one
in flight. "Which objects are in `run`" has its population (X5) and its object rows (Q2, #267).
Unblocked; not started.

## Q4 — document-query parameter defaults evaluate (done)

A `query def` parameter with a default (`in limit : Natural = 10;`) was refused at execution with
`relies on a default not retained in the plan` — the plan recorded `HasDefault` and not the
expression. [PR #849](https://github.com/JPL-Devin/OpenSysML/pull/849) (**landed**) keeps the
compiled default in the immutable plan with its declaring query, evaluates it in the declaring scope
once per execution (not per row), honours inheritance and redefinition (`in redefines threshold
default "5"`), lets an explicit binding override it, refuses an unrepresentable default at planning
and a self-referencing one as a composition cycle; `ErrorDefaultUnavailable` is gone. Verified at
this baseline over `docrender/testdata/defaulted_queries.sysml` through `-run-query` and
`-render-document`. One adjacent rough edge, not a defaults defect: `-run-query "Q threshold=1"`
types the bare `1` as an integer and is refused against a `String` parameter — quote it
(`threshold="1"`).

---

# Track A — analysis and simulation execution

The Systems Library's analysis vocabulary loads and type-checks — `AnalysisCases`, `TradeStudies`,
`StateSpaceRepresentation`, `SampledFunctions`, `VerificationCases` — and the runtime executes
calcs, constraints, requirements, actions, state machines and, since 0.6.0, an *analysis case*
(A1, the keystone the review after `v0.4.3` found missing). Each simulation capability people
expect from "SysML v2 execution" hangs off that keystone, and `v0.7.0` shipped four of the five:
verification verdicts (A6, #117), parameter sweeps (A3, #118), the trade study (A2, #133) and the
shared clock (A5, #136). The two that were open at the tag landed on `develop` after it: A7, one
verdict for every assertion an object carries, in #263, and A4, the state-space runner A5
unblocked, in #296. Nothing in the track is open. Every item below is self-assessed (the pilot
executes none of this); the runner's two stated limitations are its own.

## A1 — an analysis case runs (landed)

**Landed** in [PR #979](https://github.com/JPL-Devin/OpenSysML/pull/979), shipped in 0.6.0. An
analysis definition or usage runs as the calculation it is: the `subject` is an `in` parameter
(bound by the usage, by the object the run is asked on, or from the enclosing case; a run with no
subject is refused naming it), the body's `action`/`perform`/nested `analysis` steps are one
`lower.Block` over the action graph they state (`then`, `first`, forks, joins, decisions, merges;
declaration order where none is stated) run by the action executor, `out` and `return` are
evaluated in the case's frame with their units, and the `objective` and every `assert constraint`
are checked afterwards by the requirement engine as satisfied / not satisfied / undecided.
`-analysis Pkg::Case`/`%analysis` beside `-calc` and `-action`, the `RunAnalysis` RPC with
Connect, Go and Python clients, and reads of an analysis usage's outputs as features
(`An::shipCost.total`, `holder.inner.total`, `attribute :>> x = a.result;`) with memoization and
invalidation are in; the follow-ups in the same release bind an objective's requirement subject by
keyword alone, read a case's result by its qualified name, report a recursive step once, keep
later objectives in position and hold an actor bound without `:>>` to the actor it inherits.

What A1 deliberately leaves, each stated where it is owned: `-calc`/`%calc`/`EvaluateCalc` still
refuse an analysis *by kind* and say to run it as one — that is the contract, not a gap; the
training corpus's `33. Analysis` fuel-economy cases and the `Analysis Examples` corpus state their
step outputs only through `assert constraint` (`solveForPower.power` has no computation to run),
so they end in the typed `no value for feature` refusal — the solver's territory, not the
executor's. Two of the things it left at 0.6.0 have since landed: a verification case body is run
and its verdict reported (A6, #117), and a `calc` named as the value of an `in calc` parameter is
a function value (X6, #122), which is what `10c-Fuel Economy Analysis.sysml` needed admitted for
its bodiless `calc cityScenario` (whether that case's steps then run to a value has not been
re-measured at this baseline); a trade study's iteration over its alternatives is A2 (#133). What
the corpora answer today: the pilot's `10d-Dynamics Analysis.sysml` runs
on a supplied subject and inputs (`accelerationProfile = [0.01, 0.01998…]`), and
`10a-Analysis.sysml` binds an untyped `part vehicle` to a `Vehicle` subject and states no mass
values, so it is refused at the binding (typed) and a copy with typed, valued parts answers
`200 [kg]`. Compiling a case (N2.1) was never part of this item.

## A2 — a trade study iterates, evaluates and selects (landed)

**Landed** in #133, on X6's function values. A `TradeStudies::TradeStudy` definition or usage run
through `-analysis`, `%analysis`, `RunAnalysis` or a sweep executes the library's own expressions
rather than a special case: the subject binds `studyAlternatives`, the case's `evaluationFunction`
binds `tradeStudyObjective.eval` as a function value, `MinimizeObjective`/`MaximizeObjective`
compute `best` with `->minimize {in x; eval(x)}`/`->maximize`, the inherited `require constraint
{ eval(selectedAlternative) == best }` is checked as the objective's condition, and
`selectedAlternative` is the first alternative `->selectOne` finds it holding for. Four general
rules carried it and are now the runtime's: a domain library's calc executes from its text as a
model's does; an inherited expression reads a feature through the running case's redefinition of
it; a requirement usage applies as a predicate with its subject as its first parameter; and a
redefinition stating no multiplicity inherits the redefined feature's. Every application of the
case's calc is reported with the run, in subject order, marked `[selected]` and `[tied]`, and
crosses gRPC as `RunAnalysisResponse.evaluations`/`SweepRow.evaluations` under the
`case_evaluations` capability, read by the Go (`Analysis.Evaluations`/`Selected()`), Python
(`AnalysisResult.evaluations`/`.selected`), Node, Java and Rust clients. What it leaves, by
design: an alternative whose evaluation fails, an `evaluationFunction` without a body and a
subject listing no alternative are typed errors that leave the objective undecided, never a
fabricated pick; and `%optimize <case>` (**experimental**, the z3 `(minimize …)`/`(maximize …)`
search over the *constraint* model) now refuses an objective whose `eval` is bound to the case's
own calc and points at the analysis run — a study whose alternatives are enumerated by a solver
rather than a list stays `%optimize`'s.

## A3 — parameter sweeps, Monte Carlo and result tables (landed)

**Landed** in #118. `sysml -sweep "<param>=<from>..<to>[:<step>]"` runs the `-analysis` case or
the `-calc` once per value of the range — several `-sweep` flags run their cartesian product, the
first varying slowest — and prints one row per run (the inputs bound, what it computed, the
objective's verdict, the wall time) as a text table or inside `-json`; `-samples <n> -seed <s>`
draws `n` uniform values from each range deterministically from the seed and echoes the seed with
the table; `%sweep` and `%samples` do the same in the REPL, `RunSweep` over gRPC, `Model.run_sweep`
in the Python client. A run that fails is a row carrying its error, not the end of the table, and
`OPENSYSML_MAX_SWEEP_RUNS` bounds a table. What it leaves, by design: a range between Reals with
no step, a step of zero or of the wrong sign, a parameter the target does not declare or the
arguments already bind, and a distribution asked for by name (only the uniform draw exists) are
typed refusals. Orchestration over A1 with no new semantics, as planned; a document query over the
table is Track Q's. #344, on `develop` after the tag, adds what the item left out: a model states
its own odds through two non-normative libraries (`Stochastic::Probability` weighting the
successions out of a decision, validated at lowering; `RandomFunctions` for `uniform`,
`uniformInteger`, `triangular` and `normal`), on a random stream of its own apart from the
token-shuffle stream, every draw recorded in the witness so `%replay` reproduces a run, and
`%runs` / `-runs <n> -seed <s>` run an action `n` times on the sweep machinery and report each
observable's min, mean, max, percentiles and histogram. The scheduling choice points stay
unweighted and `explore` still enumerates weighted branches as a set.

## A4 — continuous time: a state-space runner (landed, unreleased)

`StateSpaceRepresentation` declares the protocol (a state vector, its derivative, an output) and
at the tag nothing integrates it: no time-stepping runner, no integrator (the RK4 lunar-descent
conformance case is a calc that hand-rolls its stages), no zero-crossing detection to hand an
event to a state machine. **Landed** in #296 on `develop` after the tag, on A5's clock. An action
specializing `ContinuousStateSpaceDynamics` or `DiscreteStateSpaceDynamics` runs as a fixed-step
state-space simulation: the bundled `StateSpaceIntegration` library adds `FixedStepDynamics`
(`timeStep`, an optional `stopTime`, the `time` the run writes), the integrators `Euler` and `RK4`
that a model binds to `getNextState`'s `integrate` (RK4 when it binds none — the integrator is
the model's choice, not a run option) and the `ZeroCrossing` event; discrete dynamics step by
`getDifference`. Each step advances the shared clock, so a state machine exhibited beside the
dynamics sees the same time, its `accept after`/`at` triggers fire in step order, and a step and
a trigger due together are a `due order` choice point the scheduling policy decides. An `event
occurrence` typed by `ZeroCrossing` posts an event of its type when its `guard` changes sign at a
step, which a machine's `accept` takes, and ends the dynamics when `terminal`. The trace records
`state: <action> t=<instant> x=<state> y=<output>` per step and the run reports `stateSpace`,
`output` and `time` as the action's outputs. What the runner refuses, typed and naming the action
and the member at fault: a state, input, derivative or output that is not a vector, a protocol
calc left abstract, an integrator the runtime does not provide, a step absent, zero or negative,
a state a step leaves non-finite. Its stated limitations: a crossing is located to the step
boundary and not bisected within the step, so a guard that crosses and re-crosses inside one step
is not seen; `StateSpaceItem` and a `StateSpaceEventDef` that is not a zero crossing are not
executed. The OMG example `State Space Representation Examples/EVSample1.sysml` runs through the
runner once a model of its shape also states a `timeStep`, the library itself stating none. L7's
census reads `StateSpaceRepresentation` at 6 of 17 with the runner in, the rest the abstract
protocol probed directly. The lunar-descent case stands as it was.

## A5 — one clock for actions and states (landed)

**Landed** in #136. Simulation time is a property of the runtime context, no longer of one state
machine's executor: every state machine and action a context runs reads the same clock
(`Context.Clock()`, in `SI::s`), so two machines materialized in one context share time and a
nested performance runs on the enclosing clock. An action body waits on it — `accept after
<duration>` parks the token until the clock has moved that far, `accept at <instant>` until it
reads the instant — where before the ordinary action path refused a time trigger (`a
time-triggered accept with no clock`; that robustness case is replaced by `action_accept_time_waits`
and `clock_advance`). `Context.Advance(duration)` runs every state event, action token,
change-condition poll and do round due up to the new instant, instant by instant, within the
budgets; `-advance` no longer needs `-state` and runs the invocation's `-action` and `-state`
behaviors together on one clock; `%advance` moves the session runtime's clock, so an `%action`
and a `%state` debugger both move. Which executor runs first when several are due at one instant
is a choice point, `due order`, drawn by the scheduling policy (S3) — the executor started last
first under `reverse`, the first started under `declared`, a draw under `seed:<n>` — so it is
enumerated by `explore` (S4) and one executor alone due is no choice and is not reported, leaving
every single-behavior result and trace unchanged. `ExecuteActionResponse` and
`ExecuteStateResponse` report `final_time` under the `final_time` capability. What it leaves:
A4's runner is not yet on the clock, since it does not exist; Q3's queries over the trace it
changed are unblocked.

## A6 — verification cases give verdicts from their bodies (landed)

**Landed** in #117. `sysml -analysis`, `%analysis` and the `RunAnalysis` RPC accept a
`verification def` or `verification` usage and run it as they run an analysis case — the same
lowering, subject and input binding, and step execution — and report the `VerdictKind` the body
produced: `pass` or `fail` as the library's own `VerificationCases::PassIf` computes it, a
`VerdictKind` literal the body binds as it stands, `inconclusive` for a body that produced no
verdict, `error` with the message for a body whose run failed. The body verdict is reported
*beside* requirement satisfaction, not instead of it: `-requirement`, `-satisfy`, `%requirement`,
`%satisfy` and the `VerifyRequirement`/`VerifySatisfaction` RPCs add one line per verification
case verifying the requirement, and what the requirement engine decided and the exit status are
unchanged. Over gRPC the verdicts are `verification_verdicts` fields under the
`verification_verdicts` capability, each carrying the `requirement_id` it was reported for;
`-json` reports them under `verifications`; the Go and Python clients expose them as
`Verifications`/`verifications`. What it leaves, by design: a case performed as a step of another
is reported on its own, marked as a subcase, since the library states no roll-up.

## A7 — an object validates as a whole: every assertion it carries, in one verdict (landed, unreleased)

**Landed** in #263, on `develop` after the tag, as written below. `runtime.Context.ValidateObject`
walks an object's part tree — the composite features `Instantiate` materialized, collections
included, one-based paths such as `wheels[2]` — and reports one `ObjectVerdict` per (assertion,
carrier): the asserted constraints and invariants the carrier's type declares or inherits, the
requirement usages it carries and the `satisfy` assertions whose subject it is, each `holds`,
`violated` or `undecided` with the reason, then one verdict about the object itself, `valid` only
when every line holds *and* the walk was complete — a tree cut at the depth bound, a value that
could not be read, or an object no assertion is about leaves it not shown valid. Surfaces:
`-validate=<object>` (bare `-validate` keeps its meaning), `%validate <object>`, the
`ValidateInstance` RPC under the `verification` capability with `Verdict.instance_path` and a
`summary` verdict of kind `object`, and `ValidateInstance`/`validate_instance` in the Go and Python
clients. The evidence is the `instance_validate_*` conformance fixtures: three depths and a
collection part, a violated nested assertion, a same-type pair disambiguated by carrier, a
`satisfy` whose subject is a nested part, an undecidable condition, a shared object, an optional
recursion and a bounded walk. The rows in `spec-compliance.md` say so. The text below is the
design as it stood before the work began.

An object is checked today one assertion at a time. `-instantiate car -constraint C`,
`%constraint C`, `%requirement R`, `%satisfy` and the `VerifyConstraint`/`VerifyRequirement`/
`VerifySatisfaction` RPCs each name the element to judge, and the runtime finds the object that
carries it (`Context.conditionSubject`, `carriersUnder`): the constraint's or requirement's
conditions are then read from that object's feature values — `car`'s 1800 kg, not `Vehicle`'s
default — and structural conformance (multiplicity, type, uniqueness, binding conflicts) is
refused at materialization and on every write (`checkAdmits`, `write_conformance.go`). What no
surface offers is the question a modeler asks first: *is this object valid?* — every `assert
constraint` declared on the object's type or on any part nested under it, every requirement usage
it or its parts carry, and every `satisfy` assertion whose subject lies in the tree, judged on the
object that carries each and reported together. `-validate` is not that: it reports only that the
model analysed cleanly and that the objects `-instantiate` asked for could be built, and the CLI
reference says so. Reaching the whole set today means knowing every assertion's name, one command
each, and `%constraint C` refuses when two nested parts both carry `C` ("check it on one of
them"), so a `part wheels : Wheel[4]` with an asserted constraint cannot be judged on all four in
one step at all.

The semantics are already the runtime's. An `assert constraint` is KerML's `Invariant`
(`AssertConstraintUsage`, SysML v2 §8.3.19.2; `Invariant::isNegated`, §8.3.21.10): a Boolean
expression that must be true of every instance of its featuring type, negated by `assert not`,
which is exactly what `CheckConstraintOn` evaluates for one carrier; requirements and `satisfy`
assertions have `CheckRequirementOn` and `CheckSatisfactionOn` likewise. A7 is the sweep over
them: walk the object's part tree (the composite features `Instantiate` materialized, collections
included), collect on each object the invariants and requirement usages its type declares or
inherits and the `satisfy` assertions whose subject it is, evaluate each on that object, and
report one verdict per (assertion, carrier) — `holds`, `violated` with the condition, or
`undecided` with the reason, each with the object's path (`car.wheels[2]`) — beside one overall
verdict, `valid` only when every line holds. Surfaces: `-validate <object>` (the flag already
exists; given an object reference it does this and keeps its present meaning bare), `%validate
<object>`, a `ValidateInstance` RPC under its own capability, `-json` under `validation`, and the
Go and Python clients. Unasserted constraints (`constraint c { … }` with no `assert`) are declared,
not asserted, and are left out — they are what `%constraint` is for — and the verification-case
verdicts A6 reports beside a requirement are reported beside it here too. The pilot cannot referee
any of this, so the evidence is the conformance fixtures: one object with asserted constraints at
three depths and a collection part, a violated nested assertion, an ambiguous-by-name pair that
this sweep disambiguates by carrier, and a `satisfy` whose subject is a nested part. Depends on
nothing open; the population `all T` from X5 lets the same sweep run over every object of a
type rather than one tree, and it should share Q2's carrier walk rather than add one — which is
what #263 did, and #267 (Q2's query side) shares the walk.

---

# Track V — validation against the pilot's named constraints

The denominator is the pilot's 217 named `validate*` constraints, read from the pinned jar by the
census [validation-constraints.md](validation-constraints.md) (#822) and re-audited row by row
against the code and the corpus (#900, whose gate now also checks that the function each row cites
exists and that each cited case belongs to its row). With #900 the census read **148 of 217
reported — 137 faithful, 11 approximate — 6 not implemented, 0 deliberate, 0 known failure and 63
unknown**, against 143 / 133 / 10 / 68 at the tag; the adjudication of the unknown KerML rows then
moved it to 156 reported, and the census at this baseline reads **162 of 217 reported — 156
faithful, 6 approximate — 1 not implemented, 1 deliberate, 0 known failure and 53 unknown**, each
remaining unknown row citing why the pilot never reports it.
The oracle at the other end agrees: of 285
self-authored invalid models, the pinned pilot and we both reject 276 (3 of them only in strict
mode, by design), the pilot alone rejects 0, and of the 9 only we reject eight are control-node
succession rules the pinned pilot leaves unimplemented and one is a non-Boolean guard on an action
body's succession that the pilot's check leaves silent once the library types the guard
([pilot-rejection.md](pilot-rejection.md)). Everything the previous baselines listed as open here
has landed:

- **The census and its gate** (#822, #900); **negative cases**, one minimal pilot-refereed invalid
  model per SysML constraint name the corpus owns or a stated reason none can exist (#831), and
  the KerML rejection cases (#817).
- **The rules the sample found missing**: overriding a bound (`=`) feature value (#826, which also
  carries the `=`/`default =` distinction through the AST); control-node succession counts and
  placement (#832); trigger arguments — `after` a duration, `at` an instant, `when` a Boolean —
  in validation and in the state executor (#833, #871); `send` payload, `via` and `to` typing and
  constructor arguments (#838, #875); `enum def F :> E` — an enumeration definition is a
  variation (#811) and an enumerated value is typed only by its enumeration (#907, with the cast
  and body-expression positions in #909).
- **Rows the census audit itself moved**: cross-subsetting (#872), duplicate parameter bindings
  (#875), owning-body membership (#874), initial values and `constant` on variable features (#873),
  metadata typing and `annotatedElement` conformance (#881, #901, #905, #906), association arity
  and the type of a multiplicity bound (#882), requirement-member prefix metadata and short names
  (#868, #880), a chained generalization under the cycle guard (#902).
- **Adjudicated as not a defect**: the inherited-name warning beside a typing error stays (#837);
  the pilot reports both.

**What remains is the census's own tail**, and it is the whole of the track: the 1 row marked *not
implemented* and the 53 marked *unknown* — a row is unknown when neither a pass nor a negative case
can be pointed at for the pilot's name, which is a gap in evidence before it is a gap in checking.
The order is the same as before: take the 1, then work the 53 down by writing the pilot-refereed
negative case first and implementing the rule only where the case shows we accept what the pilot
rejects; each change moves exactly its census row and `make docs-counts` regenerates the summary.
The 6 *approximate* rows (a check under our own wording that covers the pilot's constraint
without matching it) are honest as they stand and become faithful one at a time as their
wording is aligned.

---

# Track I — language integrations

`sysml-grpc` speaks gRPC, Connect and Connect-JSON over one port, with optional TLS, exact-origin
CORS, a health port and stdio; there are clients in Go (the public API), Python (`opensysml` on
PyPI), Node, Java and Rust, each with a worked tour. Verified at this baseline against the rebuilt
binary: plain HTTP/1.1 + JSON reaches every method (`ParseSources` takes `documents`, not
`sources`; `Evaluate` on `2 * 1500` answered `{"result":{"realValue":3000}}`), which means MATLAB,
R, Julia and C can integrate *today* with their HTTP and JSON libraries and no generated code. What
is missing is the contract that makes such a client correct rather than lucky, and a thin package
per language so nobody re-derives it.

## I1 — the wire contract, written down

There was no page a hand-written client can be built from: which field names (proto3
lowerCamelCase), which `Value` arm is which (`intValue` as a JSON *string* for 64-bit,
`realValue`, `boolValue`, `stringValue`, quantity with unit, enumeration by identity not display
name, instance references, `Complex`), how *unset* differs from *absent* and from *no result*, how a
diagnostic and a Connect error arrive, the model-hash lifetime, and how a behavior call and a query
are made. [wire-contract.md](../reference/wire-contract.md) is that page (#848, **landed**): every
example captured from the running service, every `Value` arm (eleven when it was written; the
`infinity` (#113), `function` (#122), `set` and `tensorQuantity` (#121) arms added since `v0.6.0`
are on the page), the Connect code table, the
`Instantiate`/`ExecuteState`/`Verify*`/`Query` answer shapes, and R, Julia, MATLAB and C
illustrations marked untested, linked from the transports, clients and API pages. Everything else
in the track reads from it.

## I2 — shared conformance fixtures for handwritten clients

One directory of request/response pairs — every `Value` arm, a diagnostic, a Connect error, a
behavior call, a document query — that the five existing clients and any new one replay, so "the
client decodes the contract" is a test and not a claim. Depends on I1; small.

## I3 — thin R, Julia and MATLAB packages

Each is a few hundred lines over the language's HTTP+JSON: connect, parse, evaluate, query, run a
behavior, decode `Value` by the I1 rules (64-bit integers, quantities, enumerations, unset), and
surface diagnostics as the language's errors. Each ships with the I2 fixtures as its tests and a
page on starting `sysml-grpc` (binary provisioning from the release, the flags, TLS, the health
check). Not started; each is about a session once I1 and I2 exist, and they can go in parallel.
Publishing (CRAN, the Julia registry, File Exchange) is account-gated and goes with R2.

## I4 — a C client, and the C ABI

C is two different things. A **C client** is I3 for C — an HTTP+JSON client over `libcurl` and a
JSON library, and the natural base for anything that embeds by FFI. A **C ABI** is in-process:
calling a compiled calc (N2.4) or, later, an embedded state machine (M4) through a stable header
with no service. The client depends only on I1/I2; the ABI depends on N2.4's design and is the same
artifact as M4's host interface, so design it once and let the three consumers (host tools, the C
client's optional in-process mode, the embedded target) restrict it. The client comes first.

## I5 — the conformance suite as a kernel contract

`conformance/` is written as the contract between `sysml-grpc` and its clients, and that is the
only direction its runner exercises: `tools/cmd/conformance` builds `./cmd/sysml-grpc` (or takes
`-binary`), starts the process itself and drives it over the three protocols. Nothing runs it
against a service that is *not* this repository's Go binary, so a second implementation of
`sysml.proto` — a kernel in another language behind the same clients — has no way to state how
conformant it is. Two changes make the suite that statement. First, an `-address` mode that
speaks to a service already listening rather than spawning one, reporting the capabilities it
advertised and skipping by `requires_capabilities` exactly as the client runs do (`-allow-skips`
already decides whether a skip is a pass). Second, a wider corpus: the thirteen scenario files
cover one or two calls per RPC, which is enough to prove a client decodes the answer and far too
little to prove a kernel computes it. The execution conformance cases under
`internal/exec/runtime/testdata/conformance/` — a model, a call, an expected result — are the same
shape as a scenario and are the deepest semantic oracle the repository has; generating scenarios
from them (the `.expected.json` becomes the `response`) turns the wire suite into a kernel suite
without writing a second corpus, and any implementation's report then reads as a fraction of the
same cases the interpreter passes. Depends on I1 and I2; the generator is a session, and it is
refereed by running the generated scenarios against `sysml-grpc` itself, which must pass them all.

---

# Track B — bindings from modeled elements to external data and services

Every value the runtime produces comes from the model text and the bundled library. A calc
evaluates from its own body; a bodiless library function reaches a Go implementation only through
`builtinFor` (`internal/exec/runtime/builtins.go`), which requires the symbol to be
*library-declared* (`libraryDeclared` in `library_functions.go` asks the index) and then looks the
qualified name up in a table compiled into the binary. A feature without a value expression is
unset, and stays unset. An `accept` fires only for events the model itself sends, or that a host
injects by hand through `StateExecutor.SendSignal`, the REPL's `%state` commands or the
`ExecuteState` request's event list. The arguments to `Evaluate`, `EvaluateCalc`, `ExecuteAction`
and `RunAnalysis` are the entire surface through which anything outside the model reaches inside
it, and none of it survives the call. There is, in short, no way for a model to say *this function
is computed elsewhere*, *this value is read from there*, or *these events arrive from that*, and
no way for a host — the Go API, a service client, a REPL session — to supply the elsewhere.

That is the gap between an execution engine and a digital-twin substrate. The identity metadata
(`ElementId`, `ProjectRef`) says *where an element lives*; the Flexo interop says where a *graph*
is stored; neither says where a value comes from or who computes a function. The track adds that:
a notation for declaring an element externally bound, a provider contract the runtime consults at
the points it already dispatches, and the same contract over the service boundary so a client in
any language can be the provider. It deliberately does not add a scripting language, a plugin
loader or a foreign-function interface to the interpreter: the model names the binding, the host
supplies it, and the seam is the one the runtime already has.

## B1 — the binding vocabulary in the notation

A `Bindings` package under `OpenSysML Libraries/`, beside `IdentityMetadata`, as standard
user-defined metadata so every conforming tool reads a bound model as an annotated one: an
`@External` on a calc or function definition whose body is absent (the host computes it), an
`@ExternalValue` on an attribute or item usage with no value expression (the host supplies it,
once or on every read), and an `@ExternalEvent` on an event or signal definition (the host posts
it). Each carries a `binding : String` the host resolves and nothing else — the notation says
*that* and *which*, never *how*. A constraint-tier pass checks the annotated element is bindable:
the function has no body, the feature has no value, every parameter and result type has a
`Value` arm on the wire (I1), and an `@External` function is not also implemented by the library
table. The RDF mapping carries the metadata as it carries `ElementId` today, so a bound model
round-trips. Not started; the design record states the metadata, the pass and the refusals
before any of it is written.

## B2 — the provider contract in the runtime and the Go API

One interface, consulted at the three points the runtime already dispatches. Before `builtinFor`,
a function marked `@External` resolves to the host's `Call(binding, args)`; when a feature marked
`@ExternalValue` is first read (its `Materialized` flag is the existing seam), the host's
`Read(binding)` supplies it; a state executor whose machine accepts an `@ExternalEvent` drains the
host's `Events(binding)` into its `EventQueue` under the clock it already runs on. The bundled
builtin table becomes the first provider of that interface rather than a parallel path, so there
is one dispatch and not two that drift. A bound element with no provider is a typed refusal
naming the element and its binding, never an unset value; a provider's error is the calc's error,
with the binding in the message; every host call is a traced step, and the trace records the
value returned so a golden replays without the provider. On the Go API, `opensysml.WithProvider`
on the model handle, and a `Provider` interface a Go host implements in a page of code. Depends on
B1. Two sessions, refereed by conformance fixtures whose expected results only a fixture provider
can produce.

## B3 — providers over the service boundary

A Python, Node, Java or Rust process must be able to be the provider, not only the caller, or the
Go API is the only host that can bind anything. The direct shape is a bidirectional stream the
client opens (`BindProvider`), on which the service sends `Call`/`Read` requests and the client
answers; [service-transports.md](../reference/service-transports.md) records that bidirectional
streaming needs HTTP/2 end to end and, in a browser, TLS, which is acceptable for a provider and
not for a page. The alternative — the service dialling a Connect endpoint the client hosts — has
no such constraint and no such streaming, at the cost of the client running a listener. Decide by
prototyping both against the Python client; either way the request and answer messages are
defined once in `sysml.proto`, the capability is `bindings`, `GetServerInfo` advertises it, and a
service without it refuses `BindProvider` as `UNIMPLEMENTED` like every other capability. The
conformance suite gains scenarios with a fixture provider the runner hosts, and the I2 fixtures
gain the provider messages. Depends on B2 and I1; a session for the transport decision and a
session for the clients.

## B4 — data sources without code

B2 and B3 make a program the provider. Most values a twin reads are in a table, a file or a
service with a URL, and asking for a program to read them is asking for the same fifty lines in
every host. A small set of built-in providers, selected by the `binding` string's scheme — a CSV
or JSON file keyed by element, an HTTP endpoint returning JSON decoded by the I1 rules, and a
Flexo project's element values once D9.2 reads one — configured on the `sysml` and `sysml-grpc`
command lines and refused by name when the scheme is unknown. This is the item that makes a model
with `@ExternalValue` runnable from the REPL against a spreadsheet with no host program at all.
Depends on B2; each provider is small and independent, and the Flexo one waits for D9.2.

## B5 — bindings as a query, and in the REPL

`Query` answers *what is bound* — every annotated element, its binding string, whether a
provider currently satisfies it and, from the trace, what it last returned — so a document or a
client can report a twin's wiring rather than infer it. The REPL shows the same through a
`%bindings` command and marks an unbound external element in `%instantiate` output instead of
showing it unset. Small; after B2, and it belongs with Q1's page that says which query is which.

---

# Track W — diagram output formats

A view's rendering is a `view.Rendering` — typed nodes (`part def`, `state`, `fork`,
`decision`, a lifeline), edges with labels, notices for what was not represented — and a
**form** is only a writer over it: `text`, `markdown`, `mermaid` and, since W1 and W2 landed,
`dot` and `plantuml`, chosen by `-render-form`, `%render <name> <form>`, the `opensysml/render` request the VS Code
panel makes, and the document renderer, which embeds the Mermaid form in HTML and rasterizes it
through `mmdc` for PDF. The tree, interconnection, state, action and sequence kinds all render — the
state rendering from the lowered `StateGraph` (regions, entry transitions, triggers, guards,
effects), the action rendering from the `ActionGraph`, the sequence rendering as lifelines and
ordered messages — so what is missing is not a diagram kind but the **formats** a rendering can
be written in, and the fidelity the one machine form allows.

Mermaid was chosen because it draws where the models are read, with no installation. The cost
is what its grammars cannot say: a `flowchart` has no fork or join bar, no swimlane, no pin, and
names a decision only by the diamond shape the writer does not yet ask for; `stateDiagram-v2` has
no history pseudostate, no entry/exit/do compartments and no orthogonal-region separator beyond
`--`; `sequenceDiagram` has no found or lost message and no timing. Every one of those is a
notice in the rendering today rather than a drawing. Graphviz DOT and PlantUML both draw them,
both lay out large graphs Mermaid cannot, and both are what the documentation and publishing
pipelines this project is meant to feed already consume.

## W1 — a `dot` form

**Landed** — see [view rendering forms](view-rendering-forms.md). A DOT writer over `Rendering`
(`internal/ir/view/dot.go`): a rendering is a `digraph`, a node with children a `subgraph
"cluster_*"` (a tree keeps containment as edges, as its Mermaid form does), direction maps onto
`rankdir`, the `EdgeKind` styles parallel the Mermaid arrows, and a state rendering draws its
states as rounded boxes with `point`/`circle`/`doublecircle` pseudo-states. Every identifier and
label is quoted through one helper; a `// layout:` header names the engine the file is written
for. The DiagramLayout geometry the rendering carries is written as Graphviz reads it — a
positioned node pinned with `pos="x,y!"` at its centre and sized in inches, a route as a `pos`
spline, the canvas as the graph's `size`, y flipped from the library's y-down pixels — and the
header then names `neato` (`neato -n` when every node is placed, `-n2` when every edge is
routed too). The goldens beside every
`*.mermaid.golden` are checked by an in-test DOT syntax walker rather than by running `dot
-Tsvg`, so no Graphviz installation is involved anywhere.

Still open from the original sketch, each a writer change and nothing else: the shape per action
node kind (`diamond` for a decision, a filled bar for fork and join), `record` or HTML-like labels
for a part with its compartments, `note` for a notice, and `URL=`/`tooltip=` from the origin every
node carries so an SVG rendered from the DOT links back to the declaration the way the LSP panel
does. The node and edge attribute lists are each written by one method (`dotNodeAttributes`,
`dotEdgeAttributes`), where the position and the route already join the label and style.

## W2 — a `plantuml` form

**Landed** — see [view rendering forms](view-rendering-forms.md#plantuml). A PlantUML writer over
`Rendering` (`internal/ir/view/plantuml.go`), one grammar per kind: a tree is a class diagram
with containment as edges (as its Mermaid and DOT forms draw it), an interconnection nested
`rectangle` blocks with the Pilot's `-[thickness=3]-` connectors and dashed flows, a state
rendering the `state` grammar with composite states, `[*]` starts and PlantUML's pseudostate
stereotypes, and a sequence — the kind DOT has no grammar for — `participant`s and `->` messages
one for one with the Mermaid form. The action rendering takes the **state grammar too**, not the
activity syntax the sketch named: activity syntax is procedural and cannot hold an arbitrary
graph of successions and flows without inventing structure, so one grammar draws every action
golden losslessly, control nodes as pseudostates and flows dashed. Every file carries the Pilot's
Standard B&W style inline in a `<style>` block — the released PlantUML does not ship the
`sysmlbw` skin — honouring the three rules DOT could not (the usage corner radius, shadows off,
`wrapWidth 300`); the named palettes fill nodes with the same hex per node as the DOT form;
DiagramLayout geometry is kept as `'` comments (PlantUML pins no position — `dot` does), through
the geometry-comment helpers the Mermaid form shares. Goldens beside every `*.mermaid.golden` are
walked by an in-test PlantUML syntax check; a PlantUML jar is never needed — `OPENSYSML_PLANTUML_JAR`
turns on an extra `-checkonly` pass when one is at hand.

Still open from the sketch, each a writer change: `[H]`/`[H*]` history where the rendering
produces a history pseudostate (today drawn by stereotype), `state X : entry / …` compartments,
notes for notices, and `[[url]]` hyperlinks from the origin — to land with the DOT `URL=` once a
writer has a stable URL for an `Origin`.

## W3 — the forms where renderings surface

`dot` and `plantuml` join `text`, `markdown` and `mermaid` everywhere a form is chosen:
`-render-form`, `%render`, the `opensysml/render` request (the VS Code panel keeps Mermaid, which
it can draw in-process, and offers the others as *save as*), and the document renderer. **For
`dot` this landed** with W1 and **for `plantuml` with W2**: `-render-form dot|plantuml`
(`-render-all` writes `.dot` and `.puml` files), `%render <name> dot|plantuml [palette]`,
`"form"` on `opensysml/render`, and `-diagram-form dot|plantuml` on `-render-document`
(`%render-document <name> dot|plantuml`, `diagramForm` on `opensysml/renderDocument`), which
writes every graph-shaped diagram block as a ` ```dot ` or ` ```plantuml ` fence in Markdown and
`<pre class="dot">` or `<pre class="plantuml">` in HTML — a render-time choice, not a model
attribute; the PDF backend keeps a DOT or PlantUML block as source under a notice and looks for
no Graphviz or PlantUML tool. The form lists in the CLI help and man pages, the REPL's completion
and the LSP's errors derive from `Forms()`, so the form reached every one. The gRPC
surface has no view-render RPC — only `RenderDocument`, to Markdown — so
the wire contract did not change; if one is added later it takes the form as a string the same
way `-render-form` does.

Still open: rasterizing a DOT or PlantUML block for PDF through `dot` or the PlantUML
jar as Mermaid is rasterized through `mmdc` today — optional tools, located by environment
variable, skipping the tests with the reason when absent, as the PDF toolchain is handled now —
and the VS Code panel's *save as* for the non-Mermaid forms.

W1 landed first, being the smaller grammar and the one Graphviz-based pipelines want; W2 followed
over the same node kinds and the sequence; W3 landed with each. Independent of every other track:
nothing here touched the rendering model, only writers over it.

---

# Track M — an embedded, RTOS-compatible target

The question was whether OpenSysML models could run on a microcontroller under an RTOS, and what
"embedded SysML v2" would mean. The review's answer, from the code: not by shrinking the interpreter
— `internal/exec/runtime` depends on maps, allocation, `big.Rat`, and the parser and semantic
packages, and `lower.ActionGraph`/`lower.StateGraph` reference AST nodes, symbol scopes and
expression trees, so they are not closed artifacts that can leave the process (TinyGo is therefore
not a route). The route is Track N's discipline applied to behavior: a **closed, serializable
behavior IR**, an **AOT C backend** that emits static tables and no allocation, and a refusal —
typed, naming the construct — for any model the target cannot bound. What the runtime can promise
is *bounded and reproducible* execution; hard real-time guarantees (WCET) are properties of the
target, the compiler and the RTOS configuration, and the documentation must say so rather than
imply them. The design record for the track at the highest software class —
[docs/internals/design/embedded-target.md](../internals/design/embedded-target.md) — fixes the
freestanding C profile M2 emits, makes the IR's written semantics rather than the interpreter the
requirement basis, turns every admissible scheduling choice into a static refusal, lists the
artifacts under configuration control and restates M1–M6 as stages with exit criteria.

## M1 — a closed behavior IR

The lowered graphs, made self-contained: nodes, successions, guards, triggers, effects, states,
transitions, regions, pins and connections with every reference resolved to an index and every
expression carried as N2's typed IR, with no pointer into the AST or the symbol tables — so it can
be serialized, diffed, and handed to a backend. Lowering to it must be lossless over the existing
conformance corpus (the interpreter can run *from* it, which is the proof), and it is the meeting
point with N2.6: the native track's action and state phases start here rather than duplicating it.
This item gates everything else in the track.

## M2 — a state and action C backend over static tables

From M1: state and transition tables, a static succession scheduler for the action graph
(a token count per node, no dynamic node creation), fixed-size event queues sized from the model,
static port/connection routing, expression evaluation through N2's C emitter, and no `malloc`
after initialization. A model that cannot be bounded — unbounded multiplicity, recursion the
compiler cannot bound, `all T`, dynamic `new` — is refused by name. Differential against the
interpreter over the state and action conformance corpus, exactly as N1 is refereed.

## M3 — a resource report, and refusal of the unbounded

Every compiled model states its RAM (tables, queues, the state vector), its stack bound per entry
point and its code size, before it is flashed; a budget file the model must fit is a build error
when exceeded. This is what makes the target honest: the report is part of the artifact.

## M4 — the host interface: `init`, `tick(dt)`, `post(event)`, `read(feature)`

A generated model is a library the RTOS task calls: initialise, advance by `dt` (the shared clock of
A5, restricted to a fixed step), post an event into the fixed-size queue, read a feature. This is
the embedded restriction of the C ABI in N2.4/I4 — same header shape, no allocation, no callbacks
into the host except the ones the model declares. Designed with N2.4 and I4, implemented here.

## M5 — Zephyr on QEMU as the proof

One state machine and one action from the conformance corpus, compiled by M2, linked into a Zephyr
application, run under QEMU (`qemu_cortex_m3`) with the trace read back over the serial console and
compared with the interpreter's trace golden. CI runs it in a container with the Zephyr SDK; a
maintainer runs it on hardware. The claim "runs under an RTOS" is made only once this passes.

## M6 — embedded metadata and library definitions

`EmbeddedTarget` metadata (the MCU, the step, the queue depths, the budget), and a small library
package of fixed-width numeric types and bounded collections the compiler recognises, so a model
says what it is for and the checker can refuse a `Real` where the target has no FPU. Declared in
the notation, checked by the existing passes, read by M2/M3. Last, because it depends on knowing
what M2 needs.

---

# Proposed, not started

**The PDF path onto the HTML backend.** The backend
[html-document-backend.md](html-document-backend.md) designs is now implemented: `docrender.HTML`
renders `-doc-form html` straight from the document IR, with the semantic structure, the `sysml-`
classes and `data-` model facts, the default stylesheet in a cascade layer that reader CSS
overrides without specificity fights, `-html-css`, `-html-no-default-css`, `-html-default-css`,
`-html-fragment`, and linked HTML sets sharing one `sysml-document.css`. What remains is the
migration designed alongside it: point the HTML-input PDF engines (`weasyprint`, `prince`) at that
markup and retire `internal/doc/docpdf`'s Markdown re-parse and its own HTML writer, which splits the
print styling out as a shared asset and moves the PDF goldens. Pandoc keeps reading the Markdown,
and `-doc-form markdown` is unaffected. About one session, independent of every track above.

**The pilot as an execution referee.** [pilot-execution-referee.md](pilot-execution-referee.md)
established that the pinned pilot evaluates model-level expressions and nothing else, so
`cmd/pilot-exec-diff` can adjudicate the expression rows of `spec-compliance.md` and no external
implementation adjudicates actions or state machines. Widening that referee means finding one,
not more harness work. For state machines the OMG PSSM suite is that referee (#230, advisory);
for actions the fUML reference implementation is, in three changes modelled on the PSSM referee
and all on `develop` after the tag: #319 provisions the pinned implementation and commits what
it computes over its own test models, #321 reads and classifies the activities, and #334
translates every expressible one to a `fuml::<Activity>` action definition by rule, runs it
under every schedule the explorer reaches and requires the values left in its output parameters
to be the reference's. `cmd/fuml-referee` files 55 activities as 15 `pass` / 0 `fail` / 36
`not-expressible` / 4 `differs-by-design` (an action the reference fires once per object token),
pinned in `docs/project/fuml-referee-baseline.json` and checked in CI by
`go run -C tools ./cmd/fuml-referee -check`; [fuml-referee.md](fuml-referee.md) records the translation
rules and every row. What remains is the `not-expressible` bucket, which is the emitter's, not
the runtime's: object creation, structural-feature actions, accept-event actions and active
classes have no translation yet, and each one added moves rows into `pass` or `fail`.

**Scaling to very large models.** [large-model-scaling-design.md](large-model-scaling-design.md)
(#300, on `develop` after the tag) starts from the satellite-network stress test's profiles
([satellite-network-stress-test.md](satellite-network-stress-test.md), #299: about 55 µs and
8.5 KB per element to load, 16 µs per element in the workspace per keystroke) and separates the
four costs a model of millions of elements pays — per element once, per workspace per edit, per
process on one core, and per modeled object by construction — designing one approach against
each: a persistent resolver and semantic model owned by the workspace and invalidated through a
document dependency relation; closed documents held as interface records that hydrate to a tree
only when opened or queried, generalizing the standard library's snapshot and index cache;
parallel per-document analysis over a read-only index; and one definition with many occurrences
in the runtime, with sparse per-occurrence values. Each names its differential test against the
unoptimized path and the measurement that decides it. The first **landed** in #316: a
`model.Workspace` keeps its resolver and semantic model for its lifetime, records which documents
each analysis read, and drops on an edit only the edited document's frame and its dependents',
`TestIncrementalEqualsFresh` replaying edit sequences against a fresh workspace (a two-line edit
beside 512 satellites from 861 ms and 327 MiB to 8.7 ms and 2.0 MiB; a one-shot `-validate` about
a sixth slower for the recording it never uses). Open against `develop` on the same design: #312
(a load's files parsed, indexed and validated on a pool of workers), #309 (the REPL analyzing
each loaded file as a document of its own) and #308 (the satellite network generated as a fleet
of occurrences). Independent of every track above; the design's own sequence orders it.

# Track P — package layering (earmarked for 0.9.0)

The module is one Go module of 72 packages under `internal/`, 49 of them under `internal/core/`.
The import graph is acyclic, as Go requires, but it is not layered: the runtime imports the
validation suite and the parser, the RDF exporter imports the runtime, the REPL imports the gRPC
service, and test-only and referee packages sit beside the product packages with nothing telling
them apart. Nothing here changes what any binary does; the track is the separation of the
parser, the semantic engine, the runtime and the translation utilities into layers that can each
be built, tested and reasoned about without the layers above them, whether or not they ever ship
separately. It was deliberately **not started** before the 0.9.0 cycle: the moves touch files
every open pull request touches, so the track waited for the pull requests open against
`develop` at this baseline to land, with the small items first and the two large ones last; the
tooling module (P3) and the small moves (P1) are in, the tests tree (P2) is scheduled. The
figures below are the baseline the track started from. Measured at `develop` `530c04667`
(#354), non-test lines and `go list` import edges; the
binary, test and directory figures below at `develop` `206760826` (#384), where the module has
70 packages under `internal/`.

## Where the module stands

Transitive dependencies on other packages of the module: `internal/syntax/parser` 4 (`source`,
`lexer`, `ast`, `quickfix`) — the parser is separable today; `semantics` 9; `lower` 10;
`passes` 19; `runtime` 22, including `parser` and all of `passes`; `export` 29, including
`runtime`, `lower`, `migrate`, `parser` and `libs`; `model` 32. `internal/exec/runtime` is
71,520 non-test lines, 29% of the module's non-test code, and 84 of its 138 non-test files import
`internal/syntax/ast`.

The edges that break the layering, each with the files that carry it:

- **Runtime → validation.** `internal/exec/runtime` imports `internal/check/passes` in seven
  files, for two things: `passes.Diagnostic` and `passes.Severity*` as the runtime's own
  diagnostic type (`choice.go`, `tool.go`, `modeled.go`), and the invocation selection in
  `passes/invocation.go` — `SelectInvocation`, `ChainCallee`, `NewArgumentTyper`,
  `InvocationArgs` — which both the type checker and the evaluator need (`eval.go`, `holders.go`,
  `binding_reads.go`, `model.go`). Because `passes` itself imports `docplan`, `queryplan`, `view`,
  `rdf`, `identity` and `lower`, executing a model links document planning, diagram layout and
  the RDF vocabulary.
- **Runtime → parser.** `runtime/tool.go` calls `parser.New` to parse text at execution time; the
  runtime should receive a tree, not build one.
- **Translation → execution.** `internal/translate/export` imports `runtime` and `lower` in
  `graphs.go`, `graphs_action.go` and `graphs_state.go` for the `graphs:1` form, and `migrate`
  and `parser` in `convert.go` because `sysml -convert` was implemented inside the exporter. In
  the other direction `internal/exec/analysis` imports `export` (`engine_entry.go`,
  `external_question.go`) for the same form, so execution and translation depend on each other
  through two packages. An RDF exporter cannot be built without the runtime.
- **Validation → execution IR.** `passes` imports `lower` (`send_action.go`,
  `action_endpoint.go`, `state_transition.go`, `w8d_assignment_referent.go`, `typecheck.go`).
  [architecture.md](../internals/architecture.md) describes `lower` as "AST → execution IR for the
  runtime"; validation consuming it means it is a semantic IR shared by both, and the layering
  should say so rather than place it under the runtime alone.
- **Frontends sideways.** `internal/frontend/repl` imports `internal/frontend/grpc` for `InstanceGraphToProto` and
  `GraphBounds` (`repl/features.go`), so the terminal REPL links the Connect service layer;
  `cmd/sysml` imports nineteen `internal` packages directly rather than through `repl` or
  `model`; `internal/frontend/lsp` imports `internal/translate/interop/reposync`.
- **Type system → scanner.** `internal/semantic/semantics` imports `internal/syntax/lexer` for its
  notation-text helpers (`NameText`, `UnrestrictedNameText`, `StringValue`, `CommentBody`,
  `IsIdentifier`, `IsKeyword`) in `documentation.go`, `units.go`, `unit_product.go`,
  `annotations.go` and `filter.go`.

What is in `internal/` that is not product code, or is in the wrong place:

- `internal/hygiene` and `internal/perfbench` have no non-test files; they exist to host
  `TestNoProductionCodeImportsTesting` and the benchmarks.
- `internal/baseline`, `internal/fixtures`, `internal/junit`, `internal/doccounts`,
  `internal/stressmodel`, `internal/fuml`, `internal/pssm` and `internal/testutil` are reached
  only from `cmd/pilot-*`, `cmd/pssm-referee`, `cmd/fuml-referee`, `cmd/stress-model`,
  `cmd/doc-counts` and `cmd/validation-census`, never from `sysml`, `sysml-lsp` or `sysml-grpc`.
- `internal/errata` is both: `core/libs/source.go` applies its overlay to the bundled standard
  library (product), and the pilot tools read its registry (tooling). It can only move once the
  overlay is split from the oracle bookkeeping.
- `internal/xmi` (the XMI 2.5 reader of the UML test suites, for `fuml` and `pssm`) and
  `internal/translate/xmi` (the XMI reader of SysML v1 exports, for `migrate`) read the same format
  into two element trees.
- `internal/workspace/envvar` and `internal/workspace/project` are process configuration, not language
  core; `internal/doc/docpdf` is the PDF backend of the `core/docrender` pipeline and the only part of
  it outside `core/`.

### What the binaries carry

`sysml` built by `make build` on linux/amd64 is 50.0 MB (`sysml-grpc` 49.0 MB, `sysml-lsp`
29.0 MB). Of its symbol bytes 66% are `internal/*` code, 15.5% are gRPC-go, Connect, protobuf,
`net/http` and `crypto/tls`, and the rest is the Go runtime and standard library; the embedded
standard library is 5.4 MB of data (the 1.8 MB source tree and the 3.6 MB parsed snapshot).
Three things follow:

- **Test code is not in the binaries.** Go compiles no `_test.go` file into a non-test build,
  and `go list -deps` shows none of `hygiene`, `perfbench`, `testutil/*`, `fixtures`,
  `doccounts/doccountstest`, `junit` or `baseline` linked into `sysml`, `sysml-lsp` or
  `sysml-grpc`. Moving tests out of `internal/` tidies the tree; it changes no binary.
- **Debug data was.** The Makefile's `LDFLAGS` stamped versions and nothing else, so the
  binaries shipped their symbol table and DWARF; `-s -w` drops both and takes `sysml` to
  37.2 MB with the version stamps, build info and stack traces intact. That is a build-flag
  change, made outside this track.
- **The service edge cost almost nothing.** A probe linking every `internal` package `sysml`
  reaches except `repl` and `grpc` is 17.6 MB and the same probe plus `internal/frontend/grpc` is 30.3 MB,
  which suggested the `repl → grpc` edge above cost a quarter of the terminal binary. Removing
  it did not: the linker was already dropping the unreachable service, and the unstripped
  `sysml` on `develop` carried 450 KB of gRPC-go and 2.6 KB of Connect symbols. Moving the
  conversion to `internal/frontend/protoconv` took `sysml` from 37,224,632 to 37,220,536 bytes and its
  transitive gRPC-go and Connect packages from 53 to 51: Connect and `internal/frontend/grpc` are gone,
  and the 51 are gRPC-go, reached through `api/proto` itself, because the generated
  `sysml_grpc.pb.go` shares the Go package with the message types. Taking the count to 0 is a
  codegen change — the service stubs generated into a Go package of their own, with a
  `go_package` of its own and every generated client updated — and is the follow-up of P1, not a
  move. `net/http` stays because `interop/flexo` and `interop/reposync` are HTTP clients.

### What is test code

346k lines of the module are tests, against 304k lines of product, in 1,289 `_test.go` files;
1,207 of them declare the package they test (white-box, reaching unexported identifiers) and 82
declare an external `_test` package. 114 test files walk a `testdata` directory; the 22
`testdata` directories under `internal/` and `cmd/` hold 19 MB, 11 MB of it the runtime's
conformance and trace fixtures, and add 32 directories to the tree. Go requires a white-box test
to sit in its package's directory, so the unit tests stay where they are; what can move is the
black-box suites, every `testdata` tree, the two test-only packages and the four test-support
packages.

### What is duplicated

Identically named unexported helpers defined in four or more non-test packages at the baseline,
each a copy, and where each lives now (P4):

| helper | copies | where | home now |
|---|---|---|---|
| `qualifiedNameText(*ast.QualifiedName) string` | 4 | `docplan`, `lower`, `runtime`, `symbols` | `ast.QualifiedName.Text` |
| `ownerOf(*symbols.Symbol) *symbols.Symbol` | 6 | `codegen`, `edit`, `passes`, `resolve`, `semantics`, `view` | `symbols.Symbol.Owner` |
| `declMembers` | 4 | `lower`, `passes`, `runtime`, `semantics` | `ast.DeclMembers`; see below |
| `lastSegment`, `qualifiedName` | 5 each | `export`, `queryexec`, `runtime`, `symbols`, `repl`; `export`, `migrate`, `queryplan`, `rename`, `solve` | `symbols.LastSegment`, `ast.QualifiedNameOf`; see below |
| `sortedKeys` | 6 | across the module | `slices.Sorted(maps.Keys(m))` |
| `moduleRoot`, `writeReports`, `classify` | 5, 4, 5 | `cmd/pilot-diff`, `pilot-reject`, `pilot-xpect`, `grammar-coverage`, `validation-census` | `tools/oracle/repo`, `tools/oracle/report` (P3) |

The name-sharing was wider than the copying. Of the `declMembers` four, three were the same
function (a definition's or usage's `Members`) and are `ast.DeclMembers`; the runtime's also
walked owned-constraint bodies and unwrapped `Membership`, so it stays as
`runtime.unwrappedDeclMembers`, and `semantics`' takes a symbol and adds `AssumeMember` bodies,
so it stays too. Of the `lastSegment` five, `symbols` and `suggest` split a `::` name and are
`symbols.LastSegment`; `runtime`'s splits a dotted feature path, `queryexec`'s an object label
at `.` or `::` outside quotes, `repl`'s returns four values for completion, and `export`'s
takes a string — different functions under one name, kept. Of the `qualifiedName` five,
`solve`'s built an `*ast.QualifiedName` from segments and is `ast.QualifiedNameOf`,
`queryplan`'s and `rename`'s rendered one and are `QualifiedName.Text`, `migrate`'s reads an
XMI element, and `export`'s stays until the exporter's own track touches `graphs.go`.

The larger reuse gaps are the ones the edges above already name: two XMI readers, calc lowering
in `runtime` rather than `lower`, invocation selection in `passes` reused by `runtime`, and the
REPL reaching into `grpc` for a conversion.

## The syntax layer against the instance layer

The instance model is clean: `runtime.Instance` is an id, a `*symbols.Symbol` and a map of
`FeatureValue`s over `EffectiveFeature`s, `Value` wraps `semantics.Value`, instance ids,
collections and symbols, and `instance.go`, `value.go` and `features.go` together name `ast.`
24 times, all for the expression payload of a `ValExpr` or function value. There is no
execution-owned layer between it and the syntax tree, though: the lowered graphs in
`internal/ir/lower` are side tables keyed by AST nodes (`ActionGraph.Nodes []ast.Node`,
`Edges map[ast.Node][]ActionEdge`, `StateGraph.Behaviors map[*ast.StateNode]*StateBehaviors`,
`Transition{Source, Target, Trigger, Guard ast.Node}`), so a node's identity is its AST pointer
and `state_executor.go` alone names `ast.StateNode` 211 times; expressions are never lowered,
`runtime/eval.go` interpreting `ast.OperatorExpr`, `ast.InvocationExpr` and `ast.FeatureChainExpr`
directly and `compile.go` compiling the same tree into closures held in a side table; and calc
lowering (`calcShape` and its `Steps`) lives in `runtime/invoke_calc.go`, not in `lower`. This
is the immutable-AST, side-table design `AGENTS.md` §4 requires and is not a defect, but it is
why the runtime cannot be exercised without real trees, why a change to an AST node type reaches
`lower`, `passes`, `runtime` and `export/graphs_*` at once, and why a compiled tier would have
to introduce the missing layer first.

## The target

A package imports only the layers below it:

| layer | packages |
|---|---|
| foundation | `source` (with the notation-text helpers and `ReplaceFile`), `ast`, `ast/astcodec`, `pack`, `diag` (with the quick fixes and `ConformanceMode`) |
| syntax | `lexer`, `parser`, `format` |
| semantics | `symbols` (with `Origin`), `suggest`, `resolve`, `semantics` (with invocation selection), `identity` (with the normative ids) |
| semantic IR | `lower`, `queryplan`, `docplan` |
| validation | `passes` (the root registers; `passes/kit`, `passes/behavior`, `passes/document`, `passes/diagram`, `passes/identity`), `edit` (with the rename conflict check) |
| execution | `runtime`, `solve`, `smt`, `analysis`, `engines`, `objref`, the `graphs:1` form |
| translation | `rdf`, `export`, `migrate`, `simresults` (the migration's result sidecar), `convert` (the conversion entry point), one `xmi`, `codegen`, `interop/*` |
| documents | `queryexec`, `docir`, `docrender`, `docpdf` |
| workspace | `model`, `libs`, `project`, `envvar` |
| frontends | `protoconv`, `repl`, `lsp`, `grpc`, `stdiorpc`, `usage`, `cmd/*` |
| tooling | `baseline`, the errata registry, `junit`, `doccounts`, `fuml`, `pssm` under `tools/`; `fixtures`, `stressmodel`, `perf`, `hygiene`, `testutil` under `tests/`; never linked by a shipped binary |

The tree says the same thing as the table. `internal/` today is 121 directories: 70 packages,
32 `testdata` subtrees, the 17 data directories of the bundled standard library and two grouping
directories, with 21 entries at the top and 41 under `core/`. The target is nine directories under
`internal/`, one per layer — `syntax`, `semantic`, `ir`, `check`, `exec`, `translate`, `doc`,
`workspace`, `frontend` — holding about 45 packages; a `tests/` tree at the repository root for
the black-box suites and every fixture; and a `tools/` tree, a nested Go module of the same
repository, for the eleven `cmd/` programs that are never released (`conformance`, `doc-counts`,
`fuml-referee`, `grammar-coverage`, `pilot-diff`, `pilot-exec-diff`, `pilot-reject`,
`pilot-xpect`, `pssm-referee`, `stress-model`, `validation-census`) and the ten packages only they
link. The product `go.mod` then lists the product's dependencies and `go build ./...` at the root
builds the product.

A layering test beside `TestNoProductionCodeImportsTesting` — a table of layer → permitted layers
checked against `go list -f '{{.Imports}}'` — pins each edge as it is removed; `make lint` is
staticcheck and gosec and checks no import boundary today.

## P1 — the small moves (landed)

Each landed as a pull request of its own, cut from `develop`:

1. `semantics → lexer`: the notation-text helpers (`NameText`, `UnrestrictedNameText`,
   `StringValue`, `CommentBody`, `IsIdentifier`, `IsKeyword`) live in `source`; `lexer` and
   `semantics` both import them from there.
2. `runtime → passes` for diagnostics: `Diagnostic` and `Severity` live in `internal/syntax/diag`,
   a leaf package `passes` and `runtime` both import.
3. `runtime → passes` for invocation selection: the runtime selects through
   `semantics.Model.SelectCall`, and `ChainCallee` and `InvocationArgs` live in `semantics`. The
   checker's argument typing stays in `passes` — it is the whole expression checker — and the
   code that builds a model for execution installs it with `SetArgumentTyper`; a runtime that
   reaches selection with no typer installed returns `runtime.ErrNoArgumentTyper` rather than
   selecting with weaker typing than validation used, and a hygiene test checks every product
   construction site installs it.
4. `runtime → parser`: the runtime parses notation text through an `ExpressionParser` the model's
   builder installs with `SetExpressionParser`, and returns `ErrNoExpressionParser` without one.
   `go list -deps ./internal/exec/runtime` names neither `passes` nor `parser`.
5. `repl → grpc`: the value and instance-graph conversion lives in `internal/frontend/protoconv`, which
   depends on `api/proto`, `internal/*` and the protobuf runtime; `repl`, `grpc` and the Go
   client import it, and `sysml` no longer links `internal/frontend/grpc` or Connect. The gRPC-go packages
   still linked, and the codegen follow-up that removes them, are measured under "What the
   binaries carry" above.
6. `export → migrate, parser`: the conversion entry point is `internal/translate/convert` — the
   formats, `Convert`, `ConvertTolerant`, `SysMLElement`, `Migrate`, `SysMLToRDF` and
   `SyntaxError` — and `cmd/sysml`, `repl`, `grpc` and `interop/flexo` call it. `export` keeps
   `ToRDF` and `ToSysML` and no longer imports `migrate`; it still imports `parser`, because the
   graph → notation decoder parses by nature (it re-parses preserved source text to check it still
   encodes to the graph, parses an expression to judge its binding, and parses names), and
   translation sits above syntax in the table.

The layering test beside `TestNoProductionCodeImportsTesting` pins the layer table and the
removed edges, alongside per-edge tests of the runtime's parser and typer seams, `protoconv`'s
dependencies and the conversion entry point's ownership of migration.

## P2 — the tests tree (in review)

`tests/` at the repository root holds the black-box suites, one package per suite, each with its
fixtures beside it (#392, #397, #395, #401): `TestNoProductionCodeImportsTesting` (`tests/hygiene`,
where the layering test goes too); the benchmarks (`tests/perf`); the parser's golden ASTs and
negative cases (`tests/parser`); the four OMG corpus gates and the RDF round-trip ratchet
(`tests/corpus`); the gRPC conformance suite (`tests/grpc`); 81 of the 82 external test files, by
package (`tests/export`, `resolve`, `semantics`, `migrate`, `reposync`, `suggest`, `identity`,
`model`, `queryplan`, `ontology`); the top-level `testdata/` as `tests/testdata`; and `gobuild` and
`graphcmp` as `tests/testutil`. The drivers that reached into their package's unexported identifiers
were rewritten against the exported surface — the gRPC suite against `grpc.NewService` with its own
unit-term formatter, the export suite with its own pilot-corpus loader — not given an export shim.
The corpus policies did not move: the training corpus is an assertion, the pilot roots and the round
trip per-file ratchets, and the download scripts and require-variables are as `AGENTS.md` §2 states
them. Measured at the track's baseline `206760826` and after the four pull requests: `testdata`
subtrees under `internal/` 32 → 22, packages 70 → 66, entries at the top 21 → 18 (`hygiene`,
`perfbench` and `testutil` gone), external test files beside a product package 82 → 1; 34
directories and 17 packages under `tests/`. With P3's moves the tree under `internal/` is 121
directories → 97.

What stayed, and why. The runtime's execution conformance and trace suites
(`TestExecutionConformance`, `TestExecutionTrace`, `conformance_test.go`, `trace_test.go` and the 11
MB under `internal/exec/runtime/testdata`) are white-box in fact, not only by location: sixteen
runtime-internal test files run the same cases through the driver's schema types, loaders and case
runner, eleven of them reaching unexported `Context` state (snapshots, held images, replay, the
explore queue), and about 1,700 of the 2,300 lines of `conformance_test.go` depend on the runtime. A
`package runtime` test cannot import a package that imports `internal/exec/runtime` — Go rejects it
as an import cycle in test — so a shared harness under `tests/` cannot be reached from those files,
and a copy of the runner in each tree is the duplication this track removes. The driver and its
fixtures stay beside the runtime, and the census reads them there. The LSP and REPL suites stay too:
the files that touch no unexported identifier still share package-local helpers with the white-box
ones (`mustDebug`, `render` and `openRenameDoc` in `lsp`; `meta`, `submitModel` and `evalOK` in
`repl`), and moving them would copy those rather than share them;
`internal/frontend/grpc/oslc_query_repl_test.go` is the one external file left, reaching `mustNewService` and
`queryModel` through `export_test.go`. And the thirteen `testdata` trees still under `internal/`
belong to white-box drivers, so they sit beside them — a fixture moves with its driver, not on its
own. Of the support packages the plan named, `doccountstest` went to `tools/census/doccounts`
with its owner, as P3 records, and `fixtures` to `tests/fixtures` under P4.

## P3 — the tooling module (landed, unreleased)

The eleven unreleased `cmd/` programs and the packages only they link — `baseline`, `junit`,
`fuml`, `pssm`, `xmi`, `doccounts`, `core/libs/gensnapshot`, `core/rdf/ontology/gen` — are in
`tools/`, a nested Go module of the repository (`tools/go.mod`, `replace … => ../`), laid out
by what each tool does rather than one directory per OMG artefact, which is how `fuml`, `pssm`,
`xmi`, `baseline` and `junit` came to sit as five siblings of the compiler (#394, #400, #404):

```text
tools/
  referee/    fuml pssm xpect diff reject exec   the oracles, one package each
  oracle/     xmi baseline junit errata report repo    what every referee shares
  census/     validation grammar doccounts       the counting gates
  gen/        snapshot ontology                  generators
  cmd/        one main per program
```

Every program runs as `go run -C tools ./cmd/<name>`, and because `-C` starts it inside the
tools module, a relative path given to any of its flags (`-out`, `-baseline`, `-junit`, …) counts
from the repository root, which `repo.Resolve` applies; `report` took `writeReports` and the
four verdict buckets the fUML and PSSM referees share, `repo` took `moduleRoot`, and the copies
are gone. `errata` is split: the overlay the standard library applies is
`internal/workspace/libs/errata` (product), the registry the oracles read is `tools/oracle/errata`.
There is no `go.work`: `go build ./...` and `go test ./...` at the root are product-only, and
`make test` and `make lint` run both modules. Two of the listed packages could not follow the
tools into the nested module, because product tests import them and a test cannot import the
nested module without a cyclic requirement: `fixtures`, which also took the analysis-library
census schema that the runtime's own test writes and `tools/census/doccounts` counts, and
`stressmodel`, whose generator `internal/workspace/model`'s incremental test drives (only
`cmd/stress-model` moved). They stayed under `internal/` at first; P4 moved them to `tests/`,
which both the root module's tests and the tools module can import.
`docpdf` is `internal/doc/docpdf`, beside `docrender`; `envvar` and `project` already sat beside
`model`, and whether they fold into it is P4's question. Measured at `develop` `1e44c746c`
before and after the four pull requests: `go list -deps ./cmd/sysml` 368 → 368 (nothing moved
was on a shipped binary's path, and `go list -deps` of the three binaries names no package of
`tools/`); directories under `internal/` 119 → 110, 21 → 13 entries at the top and 70 → 61
packages. The validation-pass split is recorded below, with registration left central.

`passes` is split by domain, in five pull requests stacked bottom-up. `passes/kit` is the
framework every check is written against — `Pass`, `PassLevel`, `Context`, `Options`, `Gathers`
and the symbol and member walkers — a leaf the root re-exports as type aliases so `passes.Context`
and `passes.NewContext` read as before. `passes/document` holds the document-plan and
document-query checks, `passes/diagram` the layout and view-rendering checks, `passes/identity`
the identity-metadata check with the workspace-wide identity gather, and `passes/behavior` the
state-transition, succession-endpoint and control-node checks. Registration is unchanged and
stays central: `passes.DefaultRegistry` is the one place a check is registered, in the same order
as before, so the root imports every domain package and the layering test pins that no domain
package imports the root. The remainder of the root is the core: the registry, the static
expression typer and the checks that call into it — among them the three behavior checks
`send_action.go`, `transition_guard.go` and `typecheck_trigger.go`, which follow the typer when it
moves — the constraint checker and the structural rules, and the OOSEM and MOSA audits. Every
diagnostic code, message, range and quick fix is byte-identical; the corpus gates, the golden
tests and the validation census are the assertion, and statement coverage over the tree is unchanged. Measured before and after the five pull requests:
`go list -deps ./cmd/sysml` 363 → 368, `go list ./internal/...` 54 → 59 packages, directories
under `internal/` 95 → 100 — the five new packages and nothing else; 82 non-test files in one
package become 71 in the root, 5 in `kit`, 4 in `behavior` and 3 in each of `document`, `diagram`
and `identity`.

## P4 — one-file packages and shared helpers (in review)

The helpers first, one pull request per family, each deleting every copy and routing the
callers to one home: `ast.QualifiedName.Text` and `ast.DeclMembers` (#411);
`symbols.Symbol.Owner` (#415, which also retired the `fqnOf`, `elementID` and `contentName`
wrappers of `symbols.FQNOf` whose fallback could not fire); `symbols.LastSegment` and
`ast.QualifiedNameOf` (#418, on #415); and `slices.Sorted(maps.Keys(m))` for the `sortedKeys`
copies of `passes`, `symbols`, `interop/flexo` and `interop/reposync` (#417, cut from
`develop` on its own). The table under "What is duplicated" records which same-named functions
were not copies and stayed.

Then the one-file packages, each into the package that owns the concept, with the layering
table consulted first so no fold adds an edge: `quickfix` into `diag` as `diag.Fix`, `Edit`,
`Replace` and `InsertLine` (#419) — both are foundation, and every importer of one already
imported the other; `conformance` into `diag` as `diag.ConformanceMode`, `ConformanceDefault`,
`ConformanceStrict`, `ConformanceModeOf` and `ParseConformanceMode` (#430), the mode being the
switch that decides a finding's severity; `fsutil` into `source` as `source.ReplaceFile`
(#431), the one filesystem write the product does atomically; `identity/normative` into
`identity` as `normative.go` (#420), which was its only importer; `provenance` into `symbols` as
`symbols.Origin`, `Symbol.Origin`, `NodeOrigin` and `OriginAt` (#426), the symbol table being
where a declaration's location already lives; and `rename` into `edit` as `edit.CheckRename`,
`RenameConflict` and `RenameOccurrence` (#422), beside the layout edits that apply a rename.
Where a fold moved a self-model unit, `examples/self-model/surfaces.sysml` names the new
package, and the pilot differential baseline records the examples digest that follows.

The test-support packages P3 could not take into the tools module moved to `tests/` (#427):
`internal/fixtures` is `tests/fixtures` and `internal/stressmodel` is `tests/stressmodel`, both
still in the root module, so `tests/model`, the census in `tools/census/doccounts` and
`tools/cmd/stress-model` import them from there. `go list -deps` of `sysml`, `sysml-lsp` and
`sysml-grpc` names neither before nor after, and the layering test pins that no production
package imports anything under `tests/`.

Three of the folds the plan named did not happen, each for a reason the import graph gives:

- `engines → analysis` is a cycle. `engines` imports `analysis` and `smt`, and `smt` imports
  `analysis`; folding the registry into `analysis` would make `analysis` import `smt`, which
  imports it. `engines` stays as the leaf that links the engines to the framework.
- `envvar → model` is a cycle too: `runtime` imports `envvar` (the tolerated edge the layering
  test lists), and `model` imports `runtime`. `project → model` was left on ownership: `model` is
  the workspace, with no filesystem or input-discovery surface, and `project` is the command
  line's input discovery, used by `cmd/sysml` and `repl` only. Both stay beside `model` in the
  workspace layer.
- `enginewire → analysis` is not a package move. Eight of the wire's types — `Question`,
  `Model`, `Result`, `Budget`, `Bound`, `Witness`, `Input`, `Description` — carry the names of
  the framework's own types in `analysis`, deliberately: the wire's vocabulary is the external
  engine protocol's, not the framework's, and it is what the schema test and the stand-in engine
  under `testdata/` speak. Folding would rename the protocol to fit beside the framework, so
  `analysis/enginewire` stays a package of its own, and the layering test keeps its row and its
  `export` edge.

Measured at `develop` `b270ebbc9` before and with the eleven pull requests applied:
`go list -deps ./cmd/sysml` 367 → 361; packages under `internal/` 60 → 52; directories under
`internal/` 100 → 92, entries at the top 11 → 8 (`fsutil`, `fixtures` and `stressmodel` gone)
and under `core/` 44 → 40. The remaining distance to the target's 45 is the three packages
above and `passes` split by domain, which P3 records as still to do.

The pull requests land in this order, each cut from the one before it except where said:
#411 → #415 → #418 → #419 → #430 → #431 → #420 → #426 → #427 → #422, and then this
document's update; #417 is cut from `develop` and merges anywhere in the sequence.

## P5 — a runtime-free translation module (in review)

Three pull requests, each stacked on the one before:

1. `analysis → export`: the `graphs:1` and `sources` forms an external engine receives —
   `GraphsVersion`, `Graphs`, `GraphsOf`, `MarshalGraphs`, `Sources`, `SourcesOf` and their
   refusals — live in `internal/exec/analysis/modelform`, in the execution layer, with a golden
   per graph kind pinning the emitted bytes. `export` keeps `ToRDF` and `ToSysML` and names no
   runtime type: `go list -deps ./internal/translate/export` (141 → 135 packages) lists neither
   `runtime` nor `lower`, so the audit of `graphs_*.go` had nothing left to move.
2. One XMI reader: `internal/translate/xmi` is the generic XMI 2.5 element tree (moved from the
   tooling module, where the fUML and PSSM referees still import it), and
   `internal/translate/xmi/sysmlv1` is the SysML v1 interpretation `migrate` reads — stereotype
   applications, href proxies, MagicDraw archives — as a walk over that tree rather than a
   second parser. The one reader rejects an `xmi:id` declared twice, as the referees' reader
   always had.
3. The layering test pins `analysis → export`, `export → runtime` and `export → lower` as
   removed edges.

## P6 — the layer directories (landed)

Every package under `internal/` sits in one of the nine layer directories of the target —
`syntax`, `semantic`, `ir`, `check`, `exec`, `translate`, `doc`, `workspace`, `frontend` — and
the layering test names every layer with the layers it may import; `internal/core/` is gone.
The item was written above as one pull request; it landed as a stack of nine, one directory per
pull request in the table's order (#440 syntax, #441 semantic, #442 ir, #443 check, #444 exec,
#445 translate, #446 doc, #447 workspace, #448 frontend), each based on the one before it and
rewriting only the import paths its own move forces, root module and `tools/` module alike,
followed by this documentation pull request. Every package keeps its last path element; nothing
was renamed, merged or split, and no behavior changed. Three packages the target table did not
name are placed by their imports: `highlight` and `query` under `semantic`, `view` under `ir`;
`analysis/enginewire` stays nested under `exec/analysis`. The one edit beyond paths is
`semantic/query`, which declares the three namespace IRIs its OSLC prefix map needs rather than
importing `translate/rdf`, so the semantic layer no longer reaches translation; the layering test
pins that edge as removed. The edges the test tolerates are the seven `develop` carried into the
stack, path-rewritten, and `exec/runtime` still receives its parser and argument typer from the
caller.

Measured at `refactor/layer-frontend` `e0ecd2069` against `develop` `659391162`:
`go list ./internal/...` 59 packages before and after; `internal/` is 108 directories
(was 100); `go list -deps ./cmd/sysml` 368 before and after; 9006 `Test`
functions and <OK> packages reporting `ok` before and after; no shipped binary links `tests/`,
`stressmodel` or `fixtures`. The `examples` digest of the pilot differential and the errata
registry digest of the pilot baselines moved because the self-model's `goPackage` strings and
`libs/errata`'s `LibraryRoot` embed package paths; no count moved.

## P7 — an execution-owned IR (not started)

Give `lower`'s graphs their own node identities (an opaque id with the originating `ast.Node`
kept for diagnostics), lower expressions once rather than interpreting the tree in two
evaluators, and move `calcShape` lowering from `runtime` into `lower`. This is the item that
creates a layer between the syntax tree and the instances; it is feature-sized work under
`AGENTS.md` §8 and goes last, when the graph the layering test guards is otherwise clean.

# Suggested sequencing

Two orders, because there are two kinds of item. The **track-local** orders say where to start
inside a track; the **cross-cutting** order says which tracks' first items go first when a session
must choose. The order agreed at the `v0.6.0` baseline had F and S first and A6/X6/A2 second, and
owed X2's chain-read half; all of it landed and `v0.7.0` and `v0.8.0` shipped it, so both orders
below are rewritten again around what remains. Of the 64 pull requests merged to `develop` after
the tag, these move a roadmap item and are named where they do: #267, #289 and #293 (Q2, now
closed), #263 (A7), #286 (the release fold-back), #291 (the generated test figures), #292 (L7,
closing Track L), #296 (A4, closing Track A), #335 and #352 (E1), #344 (modeled randomness
beside A3), #300 and #316 (the large-model design and its first step), #319, #321 and #334 (the
fUML referee for actions), #350 (R4's installer wizard), #304 (the errata overlay, under
"Upstream follow-through") and the state-executor fixes and findings the PSSM referee
adjudicated (Track E). Open against `develop` and touching an item: #308, #309 and #312 (the
large-model design's next steps) and #327 (the SysML v1 migration's remaining items, written as
a Track D item). Open against `main`: #347, a `release/0.8.1` cut from `v0.8.0` by cherry-pick,
carrying the bug fixes since the tag and nothing that moved a roadmap item.

## What `v0.7.0` and `v0.8.0` released

Everything the previous baseline listed as unreleased is in a tag, and the two releases since then
carried more than that list. By track, with the pull requests the tracks cite:

- **Track F** — closed: F1 and F2 (#116), F3 (#120); `known_failures.txt` is empty. #119 moved the
  synchronized step boundary the debugger stops at.
- **Track S** — landed: S1 (#110), S2 (#123), S3 (#125), S4 (#134); #141 made region order a
  choice point, #138 wrote the guide.
- **Track X** — landed: X2 in full (#164 closed the chain-read half), X3 (#115), X4 (#113), X5
  (#211, #238, #239), X6 (#122), X7's values (#121), X8's typing (#112); the `meta` cast (#212)
  and static expression typing landed beside them.
- **Track A** — landed: A6 (#117), A3 (#118), A2 (#133), A5 (#136); A7 (#263) and A4 (#296) are
  on `develop` after the tag, and the track is closed; #344 added modeled randomness and Monte
  Carlo runs beside A3 there too.
- **Track Q** — landed: Q4 (#849), Q2's expression half (X5); Q2's query side (#267, #289, #293)
  is on `develop` after the tag and closes Q2.
- **Track L** — landed: L3–L6; L7's measured table (#292) is on `develop` after the tag and
  closes the track.
- **Track W** — landed: W1 (`dot`), W2 (`plantuml`) and W3's plumbing for both. The VS Code
  diagram panel grew on `develop` after the tag without touching a form item: it runs the
  behavior it draws through the language server's `opensysml/debug/*` requests (#294), opens a
  document's several views (#349) and opens on demand (#348), writes layout into the document
  that declares the element across the workspace (#307), and reparents by drag (#305).
- **Track E** — E1 landed (`terminate` runs in every position, the PSSM `terminate-gap` bucket
  retired); E9 and E10 landed as conformance findings; E8's refusal landed (#229), the item
  itself is open.
- **Track D** — D12 (the standard library's normative element ids) is done.
- **Release follow-through** — R4's Windows installer is published by `v0.7.0` and `v0.8.0`
  alike; the release procedure runs git-flow (#151); `opensysml` 0.5.0 is on PyPI; the
  test-suite figures are generated and gated (#291, on `develop` after the tag).

## What remains open

The open items, by track, with the item that gates each where one does. Everything not named here
is landed or is a track the previous baseline left as it stands (D, N, M, I, V, B, R2–R5);
Tracks F, S, L and A are closed.

- **Track E** — eligible and first: E2, then E4 (E1 landed), then E6 on request, E3/E5 behind
  their design records, E7 behind its object-model item, E8 behind a model that needs it. The
  PSSM referee's 17 `fail` tests are the state side's measurement, every one attributed (#326):
  eleven wait on the region-order choice point whose design record #342 wrote and left at two
  maintainer decisions — the nine the record names to move `fail` → `pass`, plus *Terminate 001*
  and *002*, which E1 added at the same region-entry site; *Transition 017* is the record's second
  decision (two admitted traces no reading of the model produces); the remaining five cite a
  *differs, v2 silent* alignment row.
- **Track Q** — Q3, unblocked by A5 and by Q2's object rows, not started; Q1 is written and Q2
  and Q4 are done.
- **Track X** — X7's RDF literal form and native layout for sets and tensors; X8's two harness
  halves (pilot-differential numeric normalization with an adjudication file, and a standalone
  RDF expression-tree round trip).
- **Track W** — richer DOT/PlantUML node shapes and compartments, PDF rasterization of both forms
  through optional tools, and the VS Code panel's *save as* for the non-Mermaid forms.
- **Release follow-through** — R2, R3, R5 (account- and hardware-gated), and nothing else: the
  `v0.8.0` post-tag checks are met and the test figures are generated.
- **Tracks D, N, M, I, V, B** — as the tracks state them; nothing in them moved since `v0.6.0`
  except D12 and the four `Value` arms Track I's clients carry (#113, #121, #122).
- **Proposed** — the PDF path onto the HTML backend (not started); scaling to very large models
  (the design and its first step landed, #300 and #316; #308, #309, #312 open); the fUML referee
  for actions (landed, #319, #321, #334; its `not-expressible` bucket is the emitter's open
  work); recording the order of orthogonal regions (the design record #342 landed, its two
  decisions open, no code).

## Cross-cutting order

The release housekeeping the previous order opened with is done — #286 folded `main` back into
`develop`, PyPI serves the Python client's 0.5.0 — and its steps 2 (L7, #292), 4's first half
(Q2, #293) and 5 (A4, #296) landed on `develop`, so the order is shorter by three.

1. **Track E** — E2, then E4, in the track's own order below; E1 landed. F and S landed and two
   releases shipped them, so the condition the previous baseline set is met; the loops E edits
   carry A5's clock and S2's choice points, and every E item is written against a named scheduling
   policy. The state-executor fixes since the tag (#295, #297, #311, #313–#315, #317, #318, #322,
   #336) and E1 (#335, #352) moved the PSSM referee to 46 `pass` / 17 `fail`; E1 gave E2 and E4
   the notion of ending an ongoing performance they build on. Beside E2, the largest single
   lever on the state side is not a lettered item: eleven of the 17 failures wait on the
   region-order choice point, whose design record (#342) is written and stops at two decisions
   for the maintainers — whether default-policy trace goldens may gain `choice` lines, and two
   admitted traces of *Transition 017* no reading of the model produces. Those decisions, then
   the code, run beside E2 without touching it.
2. **Q3** — state and event queries, on A5's clock and Q2's object rows. Q1, the page that says
   which query is which, is written in the document-generation manual now that the set is
   complete (#267, #289 and #293 closed Q2 on `develop`); Q4 (#849) landed independently ahead
   of it.
3. **I2, I3, then I4's client** — the shared fixtures, the thin R, Julia and MATLAB packages, the
   C client, each derived from the wire contract (I1, landed in #848); the C *ABI* half of I4 is
   not here — it is step 7.
4. **D2 and D1, then D9.1 and D9.2** — Flexo: the standard vocabulary for expression trees and
   end structure, then the authenticated push and the branch read (the collection JSON annotations,
   D3.4, landed in #850). Push and read depend on the vocabulary quality, which is why they come
   last in the step; re-record the live-stack harness after D1/D2.
5. **X8's harness halves, then X7's RDF and native layout** — normalization and adjudication in
   the pilot differential and a standalone RDF expression-tree round trip, so every later
   expression item is measured; the set and tensor layouts when something needs them.
6. **B1, then B2** — the binding vocabulary, then the provider contract in the runtime and Go
   API. The two items depend on nothing outstanding (the wire contract is landed, the dispatch and
   materialization seams exist) and touch only the metadata library, one pass and the runtime's
   dispatch, so they can run beside steps 3 and 4; B2 drains events into loops that carry the
   clock (A5) and the choice points (S2). **B3** follows the transport decision, **I5** goes with
   step 3 since it is built from the same fixtures, and **B4**/**B5** come whenever a model needs
   them.
7. **M1, then M2 (with N2.6)** — the closed behavior IR, then the state and action C backend over
   static tables. Embedded behavior compilation depends on the closed IR; the native track's
   action and state phases start from the same IR rather than a second one. M3 and M5 prove it;
   M6 last.
8. **The shared C ABI** — N2.4 / I4 / M4 designed once, after N2.2 (the budget) is decided and
   after M1 fixes what an embedded entry point looks like, so a stable native/embedded calling
   contract exists to design against rather than three.

Beside the order, whenever a session has room: Track V's census rows (1 *not implemented*, 53
*unknown*), the PDF path onto the HTML backend (about one session, independent of every track),
Track W's remaining writer and rasterization work, and the large-model design's next steps in its
own sequence (#308, #309 and #312 are the open ones). Two releases are in view. `release/0.8.1`
(#347, open against `main`) is a patch cut from `v0.8.0` by cherry-pick, carrying the bug fixes
since the tag and the three VS Code extension fixes, and deliberately none of the state-executor
series or the features; it moves no roadmap item, and once tagged `main` is folded back into
`develop` as after `v0.8.0`. The next cut from `develop` carries everything else that landed
since the tag — Q2 closed, L7, A4, E1, the generated figures, modeled randomness, the fUML
referee, the state-executor fixes and the workspace's persistent semantic model — and by
`CONTRIBUTING.md` § Versioning it bumps the minor segment, not the patch: features are patch
material there, but #302 refuses a construct `v0.8.0` accepted (a body inside a nested definition
reaching the enclosing definition's features by their bare names, now `Must be an accessible
feature`), the completion, junction and join fixes (#313, #318, #317) add `choice` lines to the
traces of models that exercise them, and E1 makes a `terminate` statement that `v0.8.0` ran as
an empty action end its performance. The decision is the release checklist's, recorded there.

## Track-local orders

- **Release follow-through.** **R1** is done. **R2**–**R5** as the accounts and hardware appear:
  publisher tokens for npm, Maven Central and crates.io, a real Mac for the tap, an Apple Developer
  and an OV/EV certificate to sign with, and a marketplace publisher for the extension. None gates
  the others or anything below; **R4**'s Windows installer is proven by two tagged releases. The
  engineering item that sat beside them — the test-suite figures in `README.md` and
  `spec-compliance.md`, hand-typed and lagging the gate table — is closed by #291 on `develop`:
  `tools/cmd/doc-counts` generates and checks them as it does the pilot figures.
- **Track L.** L3–L6 landed (#830, #821, #818, #825/#861) and L7 in #292 on `develop` after the
  tag. Nothing remains in the track; its census moves with the runtime, adjudicated per change.
- **Track N.** N2.1 (records and enums) first, since compiling an analysis case and the
  differential's record and constructor coverage both need it; decide N2.2 (the budget) before
  N2.4; N2.3 tracks L4 package by package; N2.6's actions and states are Track M's M1/M2.
- **Track D.** The RDF ratchet is 353/353 with no refusal left; step 4 above is next; **D7** is
  mechanical now that identity is stable and fits anywhere; the ontology modules (#774 on the
  previous repository) have to be re-proposed against this repository before **D8**'s profile,
  which only becomes conformant behind D1 and D2; **D12** (the standard library's normative
  element ids) is done; **D11** (the API element form) after D1 and D2, and before D9.2 if the
  branch read is to offer it; **D10** (write-through from a view-only project) after D9.1 and
  D9.2, which it reads and writes through.
- **Track F.** Closed. F1 and F2 landed together (#116) as the token-per-succession model, F3
  (#120) as the per-traversal merge on top of it; `known_failures.txt` has no line left to delete.
- **Track S.** Landed in the order agreed: S1 (#110), S2 (#123), S3 (#125), S4 (#134); #141 added
  the region-order choice point afterwards. Nothing remains in the track.
- **Track E.** Eligible — step 1 above. **E1** (termination of an ongoing performance, which
  **E2** and **E4** build on) is landed; the order is **E2**, then **E4**; **E6** whenever asked,
  being a day's work; **E3** and **E5** only after their design records; **E7** after the
  object-model item it depends on; **E8** when a model redefines run-to-completion, its refusal
  (#229) standing until then; **E1**, **E9** and **E10** are landed.
- **Track X.** X2, X3, X4, X5, X6, X7's values and X8's typing landed (#164, #115, #113, #211,
  #122, #121, #112). What is left, in order: X8's harness halves (normalization and adjudication
  in the pilot differential, a standalone RDF expression-tree round trip) so every later X item is
  measured; X7's RDF literal form and native layout for sets and tensors last, when something
  needs them — step 5 above.
- **Track A.** A6, A2, A3 and A5 landed (#117, #133, #118, #136); A7 (#263) and A4 (#296) landed
  on `develop` after the tag, A7's carrier walk shared with Q2's query side (#267), and #344 gave
  A3's Monte Carlo the modeled randomness it had refused by name. Nothing remains in the track.
- **Track Q.** Q4 is done; Q2's expression half landed with X5 and its query side in #267, #289
  and #293 on `develop` after the tag, which closes Q2; Q1 is written; Q3 is step 2 above.
- **Track V.** Everything queued has landed (#822, #900, #831, #817, the rule pull requests, #811
  reconciled with #907, #909); work the census's 1 *not implemented* and 53 *unknown* rows,
  negative case first, each change moving its row.
- **Track B.** B1, B2, then B3 — step 6 above; nothing holds B1 or B2 back; B4's file and HTTP
  providers whenever asked, its Flexo provider after D9.2; B5 with Q1.
- **Track W.** W1 (`dot`), W2 (`plantuml`) and W3's plumbing for both have landed. What is left
  — richer node shapes and compartments in both writers, DOT and PlantUML rasterized for PDF
  through optional tools, the VS Code panel's *save as* — is independent of every other track and
  runs beside any step above.
- **Track I, M.** Entirely given by the cross-cutting order above.
