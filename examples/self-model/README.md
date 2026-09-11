# OpenSysML modelled in SysML v2

This is OpenSysML's own architecture written in the language OpenSysML implements: the
analysis pipeline as parts, ports and item flows, the analysis framework that answers
questions over it, the tier ladder and the two execution engines as state machines, the
invariants of
[AGENTS.md §4](../../AGENTS.md) as requirements the tool itself checks, and views that
render the diagrams. Nothing here is a special case in the tool — it is analysed,
executed and rendered by the same `bin/sysml` any other model goes through.

Two things follow from that, and are the reason it lives in the repository rather than in
the documentation as prose. The architecture diagrams are *generated* from one model, so
they cannot drift apart from each other; and the invariants are *evaluated*, so a claim
that stops describing the implementation fails a test
([`../self_model_test.go`](../self_model_test.go)) instead of quietly reading as true in a
diagram.

The seven files:

| File | What it holds |
| --- | --- |
| [pipeline.sysml](pipeline.sysml) | `OpenSysMLArtifacts` — what travels between stages (source text, tokens, tree, spans, symbol index, the library snapshot, side tables, diagnostics, IR graphs, traces, RDF, document trees), the ports and channels it travels over, and the layer metadata the filtered views select on. `OpenSysMLPipeline` — the thirteen stages from `internal/core/source` to `internal/core/solve`, each naming the Go package that implements it; `PassRegistry`, holding all fifty registered validation passes with the tier each runs at and whether it gates itself per element; the standard library with the embedded snapshot its index is decoded from, the codec and generator units behind it and the variable that overrides it; the runtime's six independent budgets with their defaults and environment variables, and the evaluator with the compiled tier beside it and the variable that switches that tier off; the runtime's split of model-derived from run-derived state, with the snapshot store and the exploration queue that split makes possible; `AnalysisFramework` — the seven question kinds and three freedoms, the five-step evidence scale and ten claims, the per-owner engine registry, the dispatcher with its three selections, the seven-field plan budget with the `OPENSYSML_JOBS` variable, the worker fleet, and the four engines this build registers (`run`, `explore`, `sweep`, `solve`), each declaring what it answers, what bounds it, and the strongest evidence it can produce; and `AnalysisPipeline`, which wires the stages together and puts the framework's questions to the runtime and the solver |
| [behavior.sysml](behavior.sysml) | one document analysed end to end (`AnalyzeDocument`, whose four decision nodes are the tier gates), the editor's edit-then-sweep path (`ServeEdit`), the library loaded once per process (`LoadLibrary`, whose two decision nodes are the digest and checksum checks that decide between the snapshot and the files), one calc invoked (`InvokeCalc`, whose three decision nodes — tracing, body, arguments — send it to the compiled tier or to the evaluator), and five state machines: the validation tier ladder, the runtime's five tiers, token flow over the action graph with its deadlock and budget exits, run-to-completion event dispatch with deferral, and the six ways a run ends early when a budget is exhausted; then the analysis framework: one question answered (`AnswerQuestion`, whose decision nodes are the selection, the coverage check, the refusal and the fault), a behavior's outcomes explored over a fleet of workers (`ExploreOutcomes`, which proves when nothing cut it short and observes otherwise), and three more state machines — the evidence ladder from not covered to proved, a worker's life in a plan, and a snapshot as a mark between steps with the two asks it refuses |
| [surfaces.sysml](surfaces.sysml) | the five interfaces over one pipeline (REPL, LSP with every capability it advertises, gRPC/Connect service, stdio service, CLI), the three that ask questions of the analysis framework with the engine selector, jobs setting and engine listing each exposes (`%engine`/`%jobs`/`%engines`, the `engine` request field, `OPENSYSML_JOBS` and `ListEngines`, `-engine`/`-jobs`/`-engines`), the protobuf schema they are generated from with its twenty-one RPCs, the five generated clients and the VS Code extension, the editor pipeline (highlighting, quick fixes, suggestions, source edits, formatting, provenance), the view engine with the eight rendering kinds it recognises and the six it produces, the document path from a query in the model through the plan, the backend-agnostic tree and the two backends to Markdown or PDF (`RenderDocument` branches on the form, and on whether the PDF converters are installed), the exporter and its accepted format names, the eight conformance oracles with their committed baselines and the pin, errata and census infrastructure behind them, and `Toolchain`, which holds all of it |
| [identity.sysml](identity.sysml) | the element-identity path: the `IdentityMetadata` library the ids are carried by, the encoder that derives an id from a qualified name, the side table that computes each element's effective id, the constraint-tier pass that checks the generated id space, the RDF writer and reader that carry identity through a graph, the Flexo harness that measures a live round trip, the repository sync that diffs a local model against its repository by effective id (`SyncModel`: scope, state, diff, conflicts, minting, write-back) with the `sysml -sync-*` flags that drive it, and the one phase of the [design record](../../docs/project/element-identity-annotations.md) not built — the notation extension filed with OMG |
| [quality.sysml](quality.sysml) | fourteen architecture invariants as `requirement def`s bound to the modelled parts, the test runs that verify them as `verification def`s, the contributor's use case, and the allocation of every logical unit onto its directory in the source tree |
| [document.sysml](document.sysml) | the architecture document itself, written in the notation: the queries it runs over the model, the sections and prose it is made of, the diagrams it embeds from [views.sysml](views.sysml), and the tables it generates from the model — so the document is a model element rather than a file someone maintains alongside one |
| [views.sysml](views.sysml) | thirty-five views — the pipeline, toolchain, analysis framework, editor pipeline and identity path as interconnection diagrams, the stage, engine, rendering-kind and invariant tables, the action and state flows including the library load, the calc invocation, the question answered and the outcomes explored, the document and identity round trips, the sync diff, the budget exits, the evidence ladder and the worker and snapshot lifecycles, the architectural layers as filtered exposes, and an overview that frames a maintainer's concern |

## Analyse it

```bash
./bin/sysml examples/self-model/*.sysml -validate
```

```
✓ package OpenSysMLBehavior
✓ package OpenSysMLDocument
✓ package OpenSysMLIdentity
✓ package OpenSysMLArtifacts
✓ package OpenSysMLPipeline
✓ package OpenSysMLInvariants
✓ package OpenSysMLGates
✓ package OpenSysMLCodebase
✓ package OpenSysMLSurfaces
✓ package OpenSysMLViews
✓ examples/self-model/behavior.sysml, examples/self-model/document.sysml, examples/self-model/identity.sysml, examples/self-model/pipeline.sysml, examples/self-model/quality.sysml, examples/self-model/surfaces.sysml, examples/self-model/views.sysml: no errors
```

## Ask whether the invariants hold

Each invariant is a requirement whose subject is a part of the modelled toolchain, so its
condition is evaluated against that part rather than left abstract. `%requirement`
evaluates one; `%check` asks the solver whether it *can* hold and reports the assignment
that satisfies it (needs `z3` or `cvc5` — see
[installing a solver](../../docs/guide/01-install.md#installing-a-solver-optional)).

```bash
./bin/sysml examples/self-model/*.sysml
```

```
> %requirement OpenSysMLInvariants::treeIsImmutable
✓ Requirement OpenSysMLInvariants::treeIsImmutable satisfied

> %requirement OpenSysMLInvariants::tiersAreGated
✓ Requirement OpenSysMLInvariants::tiersAreGated satisfied

> %check OpenSysMLInvariants::executionIsBounded
✓ Requirement executionIsBounded is satisfiable (z3, 7ms)
  OpenSysMLInvariants::executionIsBounded::'runtime.stepBudgeted' = true
```

The solver timing is whatever your machine reports. The fourteen in `OpenSysMLInvariants` are
`treeIsImmutable`, `parserRecovers`, `resolutionIsLazy`, `tiersAreGated`,
`loweringIsLossless`, `executionIsBounded`, `libraryIsClean`, `snapshotIsDerived`,
`evaluatorIsReference`, `exportRoundTrips`, and four over the analysis framework —
`questionsHaveOneContract`, that every question goes through one per-owner registry and a
dispatcher that records every engine it consults; `evidenceIsHonest`, that no engine claims
above its declared authority and an existential claim is only ever witnessed;
`runsAreIsolated`, that each job has a worker of its own, each run a fresh context, and the
result is the same at any job count; and `snapshotsAreRunState`, that a snapshot captures
what a run made and no part of the model it ran over; four
more in `OpenSysMLIdentity` state what the identity design turns on —
`identityRoundTrips`, `idsDoNotCollide`, `identityIsBesideTheTree` and `syncIsExplicit`; and
two in `OpenSysMLSurfaces`: `documentsAreTraceable`, that every rendered node can be traced
back to the element it came from, and `viewsAreHonest`, that a rendering kind the engine
recognises but cannot produce says so. [`../self_model_test.go`](../self_model_test.go)
evaluates all twenty, so an invariant the implementation stops satisfying — the standard
library growing past its clean file count, say — fails `go test ./examples/`.

## Read a view

```
> %view OpenSysMLViews::overview
view OpenSysMLViews::overview
  exposes
    OpenSysMLSurfaces::opensysml (part)
    OpenSysMLSurfaces::Toolchain::lsp (part)
  nested views
    OpenSysMLViews::overview::pipelineSubview (view)
  viewpoint conformance
    satisfy maintainerPerspective: conforms
      concern latency: conforms
```

The concern the viewpoint frames is keystroke latency, and it conforms because the exposed
language server declares incremental synchronisation. Expose a server that does not, and
the overview stops conforming.

## Render the diagrams

```bash
make self-model
```

That writes every view into `build/self-model/` — Mermaid for the structure, action and
state views, Markdown for the tables. Override the destination with
`make self-model SELF_MODEL_OUT=/tmp/views`, or render one view at a time in the REPL
(`-render` takes a single file, and this model is seven):

```
> %render OpenSysMLViews::tierStates mermaid
```

```
%% OpenSysMLViews::tierStates — state rendering (view def StateTransitionView)
stateDiagram-v2
  state "state def OpenSysMLBehavior::TierProgression" as n0 {
    state "state syntaxTier (initial)" as n1
    ...
    [*] --> n1
  }
  n1 --> n2 : [failures == 0]
  n1 --> n8 : [failures #gt; 0]
```

The stage table is the model's answer to "which package implements this stage"; like the
Mermaid above, its first line is a comment naming the view, elided here:

```
> %render OpenSysMLViews::stageTable markdown
```

| Element | Kind | Type | Declared in |
| --- | --- | --- | --- |
| OpenSysMLPipeline::AnalysisPipeline | part def |  |  |
| sources | part | SourceStore | OpenSysMLPipeline::AnalysisPipeline |
| lexer | part | Lexer | OpenSysMLPipeline::AnalysisPipeline |
| parser | part | Parser | OpenSysMLPipeline::AnalysisPipeline |
| … | | | |

To turn the Mermaid into images, pipe it through the Mermaid CLI:

```bash
npx -y @mermaid-js/mermaid-cli -i build/self-model/OpenSysMLViews.pipelineStructure.mmd \
  -o pipeline.svg
```

## Render the architecture document

[document.sysml](document.sysml) declares `OpenSysMLDocument::ArchitectureDocument`, an
architecture document written in the notation: its prose is authored, its diagrams are the
views above, and its tables are queries evaluated over the model. `make self-model` renders
it beside the views, or render it alone with:

```bash
./bin/sysml examples/self-model/*.sysml -render-documents build/self-model
```

```
wrote build/self-model/OpenSysMLDocument-ArchitectureDocument.md (markdown, …)
```

The stage table in it is written nowhere; it is what the query returned:

| name | goPackage |
| --- | --- |
| sources | internal/core/source |
| lexer | internal/core/lexer |
| parser | internal/core/parser |
| … | |

So moving a stage to another package rewrites that table on the next render, and a stage
added to the model appears in it without anyone editing the document.

The same document renders to semantic HTML:

```bash
./bin/sysml examples/self-model/*.sysml \
    -render-document OpenSysMLDocument::ArchitectureDocument \
    -doc-form html -doc-toc -o build/self-model/architecture.html
```

and to PDF, converters installed:

```bash
./scripts/download-doc-pdf-toolchain.sh   # prints the variables to export
./bin/sysml examples/self-model/*.sysml \
    -render-document OpenSysMLDocument::ArchitectureDocument \
    -doc-form pdf -doc-title-page -doc-toc -doc-number-sections \
    -o build/self-model/architecture.pdf
```

That writes thirteen pages with the views pre-rendered as vector diagrams.

## Keeping it honest

The model describes this implementation, so it goes stale the way documentation does. Three
things push back, all in [`../self_model_test.go`](../self_model_test.go): the model must
analyse clean, its invariant requirements must evaluate true, and the facts it declares are
read back out of the analysed model and compared against the implementation. The figures
first — the keyword count against `lexer.Keywords()`, the bundled library count against
`libs.DefaultSource()`, the tier count against `passes.PassLevel`, and every `goPackage` and
file path against the directory or file it names. Then each part of the model against the
package it describes: the pass registry against `passes.DefaultRegistry()` (every registered
pass modelled, at the level it declares, element-scoped only if it implements
`passes.ElementScoped`), the six budgets against `runtime.Budgets` (field, default,
environment variable and the error each exhaustion returns), the rendering kinds against
`view.Kinds()` and which of them `Supported()`, the standard library against `libs` (the
override variable it names, whether the embedded snapshot decodes for the bundled files, the
Make targets that write and check the snapshot, and that the pull request workflow runs the
check), the evaluator's memoization against the side tables `runtime.Context` keys by syntax
node, the compiled calc tier against `runtime.CalcCompileEnvVar` and the environment reference
that documents it, whether a fresh `runtime.Context` compiles calcs until that variable says
otherwise, and — invoking the model's own `StepBudget` calc through both tiers — that they agree
and that a traced run takes the evaluator, the export names against `export.FormatNames()`, the RPCs against the protobuf service descriptor, the language
server's capabilities against the ones its `initialize` result actually advertises, the
editor pipeline against `highlight.Classes()` and `edit.OpKind`, and the sync model against
`reposync`'s change and conflict kinds, its state-file suffix and the `-sync-*` flags
`cmd/sysml` defines. The analysis framework is checked against `internal/core/analysis`
itself: the engine names against `analysis.Default()`, the question kinds and their
spellings against `analysis.Kind`, the strengths and claims against `analysis.Strength` and
`analysis.Claim` in order, each engine's answered kind, bounds, replay support, external
process and authority against what its `Describe()` declares, the budget fields against
`analysis.Budget` by reflection, the jobs variable against `analysis.JobsEnvVar`, the default
against `runtime.NumCPU()` and the rejections against `analysis.ParseJobs`, the selections
against `analysis.ParseSelection`, the exploration defaults against
`runtime.DefaultExploreBudget`, the snapshot refusals against the runtime's two snapshot
errors, and the `%engine`/`%jobs`/`%engines` commands and `-engine`/`-jobs`/`-engines` flags
against the REPL and CLI that define them. Worker isolation is exercised rather than read:
two workers of one analysis model are checked to hold distinct runtime models over the same
frozen index, and two contexts on one worker to be distinct. The identity model is held to the same standard: the metadata
definitions it names are compared against `identity.ElementIdFQN` and
`identity.ProjectRefFQN`, the library file it points at must exist, and the tier it models
the identity pass at must be the tier `passes.IdentityMetadataPass` declares. So is the
document path: the converters it lists are compared against `docpdf.Engines()` and the
library it names must exist, and the architecture document must render with its tables
filled — a query that stops binding, or an embedded view that is renamed, fails the test
rather than silently dropping a section.

What that cannot do is re-verify behaviour: the invariants are conditions over the model's
own attributes, so they catch a claim edited out of agreement with itself or with the
implementation's declared shape, not a regression inside the parser. The `verification def`s
in [quality.sysml](quality.sysml) name the gates that do that — `TestGolden`/`TestNegative`,
`TestStdlibConformance`, `make stdlib-snapshot-check` with the snapshot tests of `libs` and
`symbols`, the `TestCompiledCalc` parity and differential tests,
`TestExecutionConformance`/`TestRuntimeRobustness`, the export tests, the race-enabled
analysis package tests, and the runtime's snapshot and `ExploreWith` tests.

When a stage moves, a pass is added or a client lands, the model is the place the change is
recorded once and every diagram picks it up.

The authoritative prose account of the same architecture is
[docs/internals/architecture.md](../../docs/internals/architecture.md); this model is the
structured view of it, not a replacement.
