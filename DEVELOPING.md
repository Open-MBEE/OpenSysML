# Developing OpenSysML Core

This guide is for contributors changing the OpenSysML implementation. It is
kept at the repository root deliberately: the public site is built from
`docs/`, so this file is not part of the published documentation.

The guide focuses on the Go implementation under `internal/`. It explains
how source text becomes an abstract syntax tree (AST), how names and semantics
are derived without mutating that tree, and how executable behavior is lowered
and run. The LSP and REPL are covered only where they help trace a symptom back
to the core layer that owns it.

For contribution policy, setup, and pull-request requirements, also read
[CONTRIBUTING.md](CONTRIBUTING.md). For a higher-level architectural overview,
see [docs/internals/architecture.md](docs/internals/architecture.md).

## Contents

1. [Start here](#start-here)
2. [Core architecture](#core-architecture)
3. [Source files, spans, and language selection](#source-files-spans-and-language-selection)
4. [Lexer](#lexer)
5. [Parser](#parser)
6. [AST](#ast)
7. [Symbols, scopes, and the index](#symbols-scopes-and-the-index)
8. [Name resolution](#name-resolution)
9. [Semantic model and validation passes](#semantic-model-and-validation-passes)
10. [Workspace and incremental analysis](#workspace-and-incremental-analysis)
11. [Lowering executable behavior](#lowering-executable-behavior)
12. [Runtime](#runtime)
13. [REPL and LSP integration](#repl-and-lsp-integration)
14. [How to implement a change](#how-to-implement-a-change)
15. [How to diagnose a bug](#how-to-diagnose-a-bug)
16. [Testing contracts](#testing-contracts)
17. [Definition of done](#definition-of-done)

## Start here

### Build and test

OpenSysML requires Go 1.25 or later. From the repository root:

```bash
make build
make test
make lint
```

The corresponding direct Go checks are useful while iterating:

```bash
go build ./...
go vet ./...
go test ./...
gofmt -l .
```

The full test suite includes an OMG training-corpus gate. Download the corpus
before treating a local result as equivalent to CI:

```bash
./scripts/download-training-examples.sh
```

The pilot corpora used by additional gates can be downloaded with:

```bash
./scripts/download-pilot-corpora.sh
```

### Profile-guided optimization

Each main package (`cmd/sysml`, `cmd/sysml-lsp`, `cmd/sysml-grpc`) carries a
`default.pgo`, a merged CPU profile that `go build` and `go install` apply
automatically (`-pgo=auto` is the default; pass `-pgo=off` to compare against an
unoptimized build). The three files are identical: one profile of a
representative mix — the core test suites, the calc and REPL benchmarks, the
gRPC service, the `tests/perf` harness, and the `sysml` CLI validating
every example and corpus model in the checkout.

`make test` and `make coverage` pass `-pgo=off`: a coverage-instrumented
`go test ./...` that includes a main package with a `default.pgo` fails to link
its test binary (`fingerprint mismatch`, [golang/go#80891](https://github.com/golang/go/issues/80891)).
PGO only steers optimization decisions (inlining, devirtualization), not
semantics, so the tests lose nothing running against the unoptimized code; a
plain `go test ./...` links fine either way.

The profile does not need to track every change: the toolchain tolerates a
stale profile and only loses the optimization for code whose hot paths moved.
Regenerate it when a release nears or after a change that reshapes hot paths
(the parser, resolver, passes, runtime, or workspace). Fetch the corpora first
so real models are in the mix, then:

```bash
./scripts/download-training-examples.sh
./scripts/download-pilot-corpora.sh
make pgo-profile          # or: scripts/pgo-profile.sh [-keep DIR]
git diff --stat cmd/*/default.pgo
```

`make pgo-profile` runs the workloads under `-cpuprofile`, merges them with
`go tool pprof -proto`, and overwrites `cmd/*/default.pgo`. It takes a few
minutes and must run on an otherwise idle machine, since a profile taken under
load weights the wrong paths. `-keep DIR` retains the individual profiles and
logs for inspection with `go tool pprof`. Commit the regenerated files with
`-pgo=off` versus default before/after figures, taken as the "Profile-guided
optimization" section of `docs/project/performance-profile-2026-09.md` does.

### Learn one path end to end

Before changing a subsystem, trace a small model through this path:

```text
source bytes
  → source.SourceFile
  → lexer.Token stream
  → parser.Parser
  → immutable AST
  → symbols.Index and scope tree
  → resolve.Resolver
  → semantics.Model and validation passes
  → lower.ActionGraph or lower.StateGraph
  → runtime executor
```

Not every model reaches the last two stages. Static analysis stops after
semantic validation. Execution lowers only the selected behavior.

### Find the owning layer

Start a change in the narrowest layer that owns the invariant:

| Symptom or change | Start in |
| --- | --- |
| A character sequence becomes the wrong token | `internal/syntax/lexer` |
| Valid syntax is rejected or the AST shape is wrong | `internal/syntax/parser` and `internal/syntax/ast` |
| A declaration is absent from lookup | `internal/semantic/symbols` |
| A reference resolves to the wrong declaration | `internal/semantic/resolve` |
| A valid tree violates a language rule | `internal/semantic/semantics` or `internal/check/passes` |
| Valid behavior loses guards, triggers, data flow, or structure | `internal/ir/lower` |
| Correct lowered behavior executes incorrectly | `internal/exec/runtime` |
| Open files and disk files disagree | `internal/workspace/model` |
| Only an editor operation is wrong | `internal/frontend/lsp`, after checking core results |
| Only an interactive command is wrong | `internal/frontend/repl`, after checking core results |

Avoid fixing a frontend symptom by duplicating parser, resolver, semantic, or
runtime logic in the frontend.

### Core invariants

Several rules shape almost every implementation decision:

- **The AST is immutable after parsing.** Put resolved symbols, inferred types,
  inherited members, and other derived facts in side tables.
- **Parsing always makes progress and returns a tree.** Malformed input produces
  diagnostics and `ast.ErrorNode` values; it must not panic or loop forever.
- **Resolution and semantic queries are lazy and memoized.** Reuse the resolver
  and semantic model supplied by the analysis context.
- **Validation is tiered.** Do not emit downstream type or constraint noise for
  syntax or resolution failures that already block meaningful analysis.
- **Execution consumes lowered IR.** Runtime executors must not reparse or
  reinterpret declaration ASTs independently of `internal/ir/lower`.
- **Failure timing is observable behavior.** Preserve whether an error is
  reported during construction, initialization, stepping, or completion.
- **Tests are executable contracts.** Do not weaken a test or normalize away a
  meaningful difference to make a change pass.

## Core architecture

The core packages have intentionally separate responsibilities:

```text
internal/
├── syntax/      source bytes and spans, lexer, parser, AST, packed files, format
├── semantic/    symbols, scopes, name resolution, semantics, identity, highlight, query
├── ir/          lowered execution IR, query/doc plans, views
├── check/       ordered validation passes and diagnostics, workspace edits
├── exec/        expression, action, and state execution; solving; analysis
├── translate/   RDF, XMI and notation conversion, code generation, interop
├── doc/         query execution, document IR, Markdown/HTML/PDF backends
├── workspace/   documents, workspaces, reindexing, diagnostic caches, libraries
└── frontend/    LSP, REPL, gRPC and stdio transports, protobuf conversion, usage
```

The dependency direction matters. For example, a validation pass may ask the
semantic model for conformance, but the semantic model should not depend on one
particular diagnostic pass. The runtime may consume a lowered graph, but should
not duplicate the graph-building rules.

Two related pipelines run over the same parsed model:

```text
Static analysis:
source → lexer → parser → AST → symbols → resolution → semantic passes

Execution:
selected symbol + AST + resolution → lowering → execution graph → runtime
```

The AST is the shared syntax representation. Everything after it is derived and
can be rebuilt when documents change.

## Source files, spans, and language selection

The source layer is in `internal/syntax/source`.

### `SourceFile`

`source.SourceFile` owns a file name and its raw bytes. Lexer tokens, AST nodes,
and diagnostics refer to byte offsets through `source.Span` rather than copying
source text. Use the source file to recover text when a parser or diagnostic
needs the original spelling.

Offsets are byte offsets, not rune counts. Preserve this convention when
creating spans or converting them for an editor protocol.

### Line and column lookup

`lineindex.go` converts byte offsets to line and column positions. Core syntax
and semantic code should continue to use byte spans; convert to user-facing
positions at the boundary that needs them, such as LSP diagnostics.

When a diagnostic points at the wrong location, determine whether the producer
created the wrong span or the consumer converted the correct span incorrectly.
Do not compensate for a bad parser span in the LSP.

### SysML and KerML selection

`source.KindOf` and related code in `kind.go` select the source language from
the file name. The lexer and parser use that kind for language-sensitive
keywords and productions.

When adding a word or production:

1. Decide whether it is shared by SysML and KerML or belongs to one language.
2. Update the language-specific keyword/contextual-word handling.
3. Add both positive and negative coverage where the two languages differ.

Do not assume that syntax accepted in a `.sysml` file is also legal in a
`.kerml` file.

## Lexer

The lexer is a handwritten, pull-based scanner in `internal/syntax/lexer`.
`lexer.New` accepts a `SourceFile`; callers repeatedly call `Next`.

### Token model

`token.go` defines token kinds and the `Token` structure:

```go
type Token struct {
    Kind         Kind
    Span         source.Span
    KeywordID    string
    Unterminated bool
}
```

`KeywordID` distinguishes individual keywords while keeping a single keyword
token kind. `Unterminated` marks comments or notes that reached end of file
without a closing delimiter.

The token owns a span, not copied lexeme text. Use the source file and span when
the spelling is needed.

### Scanning order

`Lexer.Next` first handles categories whose first byte is enough to choose a
scanner:

- whitespace;
- single-line and multiline notes;
- regular block comments;
- identifiers and keywords;
- unrestricted names;
- decimal and real literals;
- strings.

It then recognizes operators and punctuation, matching the longest operator
first. This order is important: `::>` must be recognized before `::` and `:`,
and `..` must not be confused with a fractional part.

Unknown bytes become `Error` tokens. `scanError` consumes at least one byte and
may coalesce a run of bytes that cannot start any valid token. That progress
guarantee is required for malformed and fuzz-like input.

### Keywords and contextual words

Keyword sets and language-specific registration live in `keywords.go`.
Contextual words live in `contextual.go`. A word should be a lexical keyword
only when the grammar requires it to be reserved in that language. Words whose
meaning depends on a production should remain contextual and be interpreted by
parser lookahead.

Promoting an identifier to a keyword can break declaration names throughout the
language, so search parser usage and add regression cases before changing the
keyword table.

### Trivia

The lexer emits whitespace, notes, and regular comments. The parser consumes
these tokens separately from grammar tokens and records them as `ast.Trivia`.
Regular comments also participate in comment-to-element association.

Trivia is not harmless parser noise. It supplies source fidelity and editor
features. A parser backtrack must not discard trivia already pulled from the
lexer, because the lexer will not emit it a second time.

### Numeric literals

Number scanning distinguishes decimals and reals. A dot begins a fractional
part only when followed by a digit, which prevents `1..2` from becoming one
real token. Exponents are consumed only when they contain the required digits.

When changing number syntax, test neighboring punctuation and incomplete
forms, not only the accepted literal.

### Adding or changing a token

1. Add or adjust the token kind in `token.go`.
2. Implement longest-match scanning in `lexer.go`.
3. Update keyword or contextual-word tables if the token is word-based.
4. Add focused lexer tests for valid, adjacent, and malformed forms.
5. Update parser productions that consume the token.
6. Run parser negative and recovery tests as well as lexer tests.

Do not add a token merely to simplify one parser function if the source form is
better represented by existing tokens and contextual lookahead.

## Parser

The parser is a handwritten recursive-descent parser in
`internal/syntax/parser`. It consumes non-trivia tokens from the lexer, constructs
nodes from `internal/syntax/ast`, and records syntax diagnostics.

### Entry point and parser state

The normal entrypoint is:

```go
sf := source.New(name, content)
p := parser.New(sf)
root := p.ParseFile()
```

The parser buffers every non-trivia token it has requested. `pos` is a cursor
into that append-only buffer, which allows bounded backtracking without
rewinding the lexer.

`ParseFile` parses a brace-less root namespace until EOF. It records the cursor
and source offset before each member; if a production consumes nothing, it
advances one token. Preserve that last-resort progress guarantee.

### Grammar organization

Parser files are grouped by grammar domain rather than one generated production
per file:

- `namespace.go` parses root and namespace members, names, imports, and
  qualified names.
- `defusage.go` dispatches definition and usage kinds, modifiers,
  specializations, and common declaration structure.
- `behavior.go` parses action, state, calculation, constraint, requirement, and
  related behavioral bodies.
- `expr.go` parses expressions and postfix chains.
- Other domain files cover connectors, metadata, dependencies, multiplicity,
  and specialized productions.

When looking for a production, search for the AST type, the leading keyword,
and the diagnostic text. The parser function name may follow the grammar
concept rather than the concrete spelling.

### Lookahead and dispatch

Use `peek`, `peekN`, `at`, `atKeyword`, and related helpers for lookahead.
Member dispatch commonly combines:

- a leading keyword or modifier;
- a small number of following tokens;
- the current language kind;
- the enclosing body context.

Prefer a narrow lookahead predicate over consuming tokens and restoring state.
Use try-parse backtracking when the alternatives genuinely share a prefix that
cannot be distinguished cheaply.

### Checkpoints and backtracking

`checkpoint` captures:

- the buffered-token cursor;
- diagnostic and warning lengths;
- pending regular-comment association state.

`restore` rewinds those values after an abandoned parse attempt. It deliberately
does not rewind the lexer or clear collected trivia. The lexer is pull-based,
and discarding that trivia would lose source information.

Code using a checkpoint must have a clear success condition. Do not restore
after a production has committed to an alternative or after recovery has
intentionally consumed input.

### Expectations, diagnostics, and fixes

`expect` consumes the requested token or records a diagnostic without
consuming. Some unambiguous errors, such as a missing semicolon, include
quick-fix edits through `errorWithFixes`.

Parser findings have two channels:

- `Diagnostics` contains syntax errors that make the parse ill-formed.
- `Warnings` contains input that parsed as intended but is not well-formed,
  such as a reserved keyword used where an unrestricted name was required.

Choose the channel based on whether downstream code has the intended tree.
Do not turn an ill-formed partial parse into a warning merely to let a
conformance gate pass.

Diagnostics should:

- point at the token or insertion location that explains the error;
- describe the expected construct in user terms;
- avoid duplicating a lower-level error on every enclosing production;
- include a quick fix only when the edit is unambiguous.

### Error recovery

The parser returns a tree even for invalid input. A failed member production
generally creates an `ast.ErrorNode` and synchronizes at a boundary such as a
semicolon, closing brace, or plausible next member.

Recovery has two goals:

1. Never panic, hang, or repeatedly report the same token.
2. Preserve later independent declarations so the editor can continue to
   analyze them.

Test both goals. A test that only checks for one diagnostic can miss a loop or
the loss of all following declarations.

### Expressions

`ParseExpression` begins at the lowest-precedence conditional production.
`expr.go` then descends through precedence levels to unary and primary
expressions before applying postfix operations.

When adding an operator:

1. Identify its associativity and relative precedence from the grammar/spec.
2. Add it at the correct precedence layer.
3. Preserve the operator span and source ordering in the AST.
4. Add mixed-operator tests that distinguish precedence and associativity.
5. Update semantic evaluation and runtime evaluation if the operator is
   executable.

Do not implement precedence by repairing the tree after parsing.

### Adding a parser production

A syntax feature usually requires coordinated changes:

1. Confirm the SysML/KerML grammar and applicable language kind.
2. Add lexer support only if the source introduces a genuinely new token.
3. Add or extend AST node fields for the syntax that must be preserved.
4. Add a lookahead predicate and dispatch from the owning parent production.
5. Parse the production while preserving spans and trivia.
6. Add synchronization for malformed and incomplete forms.
7. Add golden, negative, and recovery tests.
8. Run the standard-library conformance gate.

If later semantics need a fact that is directly written in the source, preserve
that syntax in the AST. If the fact is derived, keep it out of the AST.

## AST

AST nodes live in `internal/syntax/ast`. They represent syntax and source
structure, not resolved or inferred meaning.

### Node contract

Nodes expose source spans and trivia through the `ast.Node` interface.
`NodeBase` is embedded in concrete nodes to hold common syntax data.

Examples of syntax-level information appropriate for the AST include:

- the written name and modifiers;
- the exact kind of definition or usage;
- written specialization and relationship clauses;
- expression operands and operators;
- body members and their source order;
- source spans and comments.

Examples of information that does not belong in the AST include:

- the symbol to which a qualified-name segment resolves;
- inferred or inherited types;
- effective multiplicity;
- conformance results;
- cached evaluation values;
- execution tokens or active states.

Those facts belong in symbols, resolver maps, semantic-model caches, lowered
graphs, or runtime state.

### Memberships and ownership

Namespace contents are wrapped in `ast.Membership`, which carries visibility
and membership-specific flags separately from the owned element. Code walking
a namespace commonly needs to inspect both the membership and its `Member`.

Do not flatten membership information into the child declaration. The same
element shape can participate through different membership semantics.

### `ErrorNode`

`ast.ErrorNode` preserves the source region of syntax that could not be parsed
into a normal node. Downstream traversals must tolerate it. Semantic code should
gate analysis downstream of a blocking parse failure rather than assuming every
member has a valid declaration shape.

### Extending the AST

When adding a field or node:

- model the written grammar directly;
- follow neighboring node naming and embedding conventions;
- make zero values safe for incomplete parses;
- update AST dumping so golden tests expose the new structure;
- update all relevant walkers and type switches;
- do not add mutable semantic caches.

An AST change is often intentionally visible in golden files. Review every
golden diff; do not update snapshots blindly.

## Symbols, scopes, and the index

`internal/semantic/symbols` derives declarations and lexical lookup structure from
the AST.

### Symbols

A `symbols.Symbol` identifies a declaration and records facts needed for
lookup, such as:

- symbol kind and effective name;
- declaration AST node;
- declaring and owned scopes;
- visibility and aliases;
- owner relationships;
- source span and document association.

Symbols are derived objects. The builder must not write them back into AST
nodes.

### Scope construction

`symbols.Build` walks a root AST and constructs its local scope tree.
`builder.go` decides which syntax creates a scope and where declarations are
defined.

Scope-producing constructs include ordinary packages and namespaces as well as
many less obvious bodies:

- definitions and usages with nested members;
- states, transitions, and control nodes;
- loop and branch-local bodies;
- metadata and requirement bodies;
- connector ends;
- transition effects and trigger-related scopes.

When a reference fails only inside a specialized body, inspect scope
construction before changing general name resolution.

Imports are registered for later resolution and expansion; they are not merely
ordinary declaration symbols. Connector ends and other body-local elements may
be intentionally visible only from their owning scope.

### Scope lookup

`scope.go` manages parent/child links, declaration order, named members, and
anonymous members. Local lookup should remain local. Walking parents, imports,
inheritance, and visibility belongs in the resolver rather than being
duplicated in scope primitives.

### Global index

`symbols.Index` combines document scope trees and records fully qualified names.
The normal document path is:

```go
idx.AddDocument(name, root)
idx.ExpandWildcardImports()
```

Wildcard imports and re-exports are expanded to a stable result, independent of
document insertion order. Incremental replacement must refresh derived import
state rather than accumulating stale names.

Library indexes can be frozen with `Freeze`. A workspace can create an overlay
with `NewOverlay`, sharing the immutable library base while keeping project
writes separate.

The bundled standard library's frozen index is not built at start-up but decoded
from `internal/workspace/libs/stdlib.snapshot`, a generated artifact embedded in the
binary (`symbols.WriteSnapshot`/`ReadSnapshot` over `internal/syntax/pack` and
`internal/syntax/ast/astcodec`). The OMG files under `internal/workspace/libs/stdlib/`
remain the source of truth: a process falls back to parsing them whenever their
digest or the snapshot's format version differs from what the snapshot records.
After editing a bundled library file, the snapshot's format, or anything the
frozen index holds, regenerate and commit it:

```bash
make stdlib-snapshot          # go generate ./internal/workspace/libs
make stdlib-snapshot-check    # what CI runs; TestEmbeddedSnapshotIsCurrent fails too
```

Never edit the snapshot by hand.

### Adding a new declaration kind

If parser output contains a new declaration that users can name or reference:

1. Add or reuse the appropriate `SymbolKind`.
2. Teach `builder.go` how to create the symbol.
3. Decide whether the declaration owns a scope.
4. Define its effective name, visibility, and owner scope.
5. Register special body-local declarations in the correct scope.
6. Update index or fully qualified-name handling if ownership is unusual.
7. Add scope and lookup tests before adjusting the resolver.

If the symbol exists but lookup fails, inspect which scope owns it and where the
reference starts.

## Name resolution

`internal/semantic/resolve` resolves syntax references against a `symbols.Index`.
Resolution is lazy and memoized.

### Resolver lifecycle

Construct a resolver with:

```go
resolver := resolve.New(index)
```

For full semantic analysis, attach the shared semantic model:

```go
sem := semantics.NewModel(resolver)
resolver.SetModel(sem)
```

Analysis contexts and workspaces already construct this pair. Reuse it rather
than creating a resolver per node or per pass.

### Qualified and unqualified names

The main entrypoints include:

```go
ResolveName(scope, name, at)
ResolveQualified(scope, qualifiedName)
```

Resolution considers more than lexical parent scopes. Depending on the
reference and mode, it may account for:

- aliases;
- explicit and wildcard imports;
- public re-exports;
- private, protected, and public visibility;
- inherited members;
- feature chains;
- filters and redefinitions;
- transition endpoints;
- body-local declarations.

Avoid adding a one-off lookup path in a caller. If a language reference follows
normal name-resolution rules, extend the resolver and its tests.

### Segment results and immutable syntax

For a qualified name, the resolver records the symbol and alias associated with
each segment in resolver-owned maps. APIs such as `PartSymbol` and `PartAlias`
serve editor features without annotating the `ast.QualifiedName`.

This pattern is intentional. The same AST may be read concurrently and analyzed
against rebuilt indexes. Never cache resolved symbols directly on syntax nodes.

### Memoization and modes

Resolution results are keyed by reference nodes and, where necessary, by lookup
mode or starting scope. Specialized caches exist for imports, aliases,
feature-chain results, filtered lookups, and endpoints.

When adding a cached query:

- include every input that can change the answer in the key;
- cache successful and failed results when appropriate;
- retain cycle detection;
- invalidate by replacing the resolver when its index changes.

Do not reuse a memoized answer across document-index revisions.

### Cycles and diagnostics

Imports, aliases, inheritance, and feature references can form cycles. Resolver
code must detect a currently resolving query and report a stable failure rather
than recurse indefinitely.

Resolution diagnostics should identify the unresolved or inaccessible source
reference and, where available, provide useful suggestions. Do not emit a
second type error whose only cause is the unresolved reference.

## Semantic model and validation passes

Semantic analysis is split between reusable model queries in
`internal/semantic/semantics` and diagnostic-producing passes in
`internal/check/passes`.

### Semantic model

`semantics.Model` answers questions such as:

- specialization and type conformance;
- inherited and effective members;
- usage typing;
- multiplicity ranges;
- variation and role relationships;
- dimensions and units;
- model-level constant values.

Keep generally useful language facts here rather than burying the same logic in
multiple validation passes or runtime functions.

The semantic model is lazy and shares the resolver. Queries must tolerate
incomplete or unresolved input and return an explicit “not known” result where
the language fact cannot be established.

### Model-level evaluation

`Model.Eval` evaluates the supported subset of expressions that must be known
during static analysis. Supported values include integers, reals, booleans,
infinity, and selected operators.

An unsupported or nonconstant expression returns `ok=false`; this is different
from proving that the expression has an invalid value. Callers should skip a
check that requires a constant rather than invent a value.

Runtime evaluation is broader and stateful. Do not call the runtime from a
semantic pass to evaluate a model-level constant.

### Multiplicity

Multiplicity helpers compute declared, assumed, and effective ranges. A range
can distinguish a known bound from an unknown or unevaluable one. Use helpers
such as `LowerLeUpper` and `CountViolation` rather than reproducing interval
rules in callers.

### Pass levels

Validation passes implement:

```go
type Pass interface {
    Level() PassLevel
    Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic
}
```

The levels are:

```text
syntax → name resolution → type → constraint
```

The registry in `registry.go` defines the default pass set and order. Analysis
sorts final diagnostics by location, source, and message for deterministic
output.

### Blocking and element-scoped passes

A document-scoped pass at a later level is skipped when an earlier blocking
failure makes its result unreliable. This prevents a malformed declaration from
producing cascades of misleading errors.

Some passes implement the `ElementScoped` marker. They may still analyze
independent elements in a partially invalid document, but must use context
gating such as `DownstreamOfFailure` before checking an affected element.

When adding a pass, decide explicitly:

- which level owns the rule;
- whether one blocking error invalidates the whole document or only one element;
- which semantic-model query supplies the reusable fact;
- what source span best explains the violation.

### Diagnostics

Pass diagnostics carry stable source, code, severity, span, and message data.
Use a code and source consistent with neighboring rules. Diagnostics are part
of the LSP, REPL, tests, and user workflows, so changing their timing, span, or
text may require updating more than one test.

Do not use a type or constraint pass to report malformed syntax that the parser
can identify more accurately.

### Adding a semantic rule

1. Identify the specification rule and the earliest reliable pass level.
2. Add a reusable semantic query if more than one caller needs the fact.
3. Add a focused pass or extend the pass that owns the rule.
4. Use the context’s shared resolver and semantic model.
5. Gate unresolved and parser-invalid input.
6. Emit one stable diagnostic at the explanatory source span.
7. Add valid, invalid, and incomplete-input tests.
8. Check that the LSP and REPL receive the result through the workspace rather
   than adding frontend-specific validation.

## Workspace and incremental analysis

`internal/workspace/model` coordinates documents, the index, and analysis for the
frontends.

### Document construction

`newDocument` is the canonical per-file parse path:

```go
sf := source.New(name, content)
p := parser.New(sf)
root := p.ParseFile()
scope := symbols.Build(root)
symbols.SetDocName(scope, name)
```

The resulting `Document` stores content, version, AST, parser findings, local
scope, and source-file data.

### Workspace reindexing

When content changes, the workspace reparses it, replaces the document in the
index, expands wildcard imports, and invalidates cached diagnostics. Resolver
and semantic-model instances are rebuilt over the current index.

Open buffers and on-disk content are tracked separately so an editor can
analyze unsaved text without losing the disk version.

When an incremental bug appears:

1. Compare behavior after a clean workspace load and after an edit.
2. Check whether the correct content and version reached `newDocument`.
3. Check index replacement and wildcard-import refresh.
4. Check diagnostic-cache invalidation.
5. Only then inspect the LSP synchronization layer.

Do not mutate an existing AST or resolver cache to “update” a document.
Replacement is the invalidation boundary.

## Lowering executable behavior

`internal/ir/lower` converts selected declarations into explicit
runtime-facing intermediate representations. This is where syntactic and
resolved model structures become execution structure.

### Why lowering is separate

The AST preserves how a model was written. The runtime needs a graph with
resolved control flow, endpoints, guards, triggers, bodies, and data flow.
Keeping that transformation in one layer:

- prevents executors from reparsing declaration ASTs;
- makes execution structure testable independently;
- lets multiple runtime entrypoints share one interpretation;
- exposes information accidentally dropped between syntax and execution.

### Action graphs

`ToActionGraph` lowers an action declaration and its scope to `ActionGraph`.
The graph carries:

- action nodes and control-flow edges;
- guards and succession declarations;
- object/data flows;
- executable statement bodies;
- accept behavior;
- initial and final nodes;
- connections;
- body-local statement-run information;
- the scope used for execution lookup.

If an action parses correctly but a guard, flow, or nested body disappears at
runtime, inspect the graph before changing the executor.

### State graphs

`ToStateGraph` and `ToStateGraphWithEndpoints` lower state-machine structure.
Lowering collects states, regions, pseudostates, transitions, triggers,
deferred triggers, and state behaviors. Endpoint resolution may use results
already computed by the resolver.

Trigger lowering distinguishes completion, time, change, signal, and call
triggers. Transition lowering must preserve guards, effects, source/target
vertices, and region or hierarchy relationships.

### Scope and endpoint helpers

Files such as `scope.go`, `endpoints.go`, `vertices.go`, `connection.go`, and
`binding.go` centralize recurring lowering decisions. Extend these shared
helpers instead of teaching one executor a special AST shape.

### Adding executable syntax to lowering

1. Confirm that the parser and AST preserve all written information.
2. Confirm symbols and resolution expose referenced declarations and endpoints.
3. Add semantic validation for structurally invalid models.
4. Extend the relevant graph type only with runtime-relevant derived data.
5. Populate that data in lowering.
6. Add direct lowering tests for graph shape and error cases.
7. Update runtime code to consume the graph field, not the original AST.

A behavior feature is not implemented merely because the parser accepts it.
The complete path must preserve its meaning through lowering and execution.

## Runtime

`internal/exec/runtime` executes expressions, actions, states, calculations,
constraints, requirements, and model instances.

### Runtime context

`runtime.Context` is the high-level execution entrypoint. It owns or references
the symbol index, resolver, semantic model, runtime data, and configuration
needed to execute selected symbols.

Common entrypoints include:

```go
ExecuteAction(...)
ExecuteActionWithInputs(...)
ExecuteActionPerformedBy(...)
ExecuteState(...)
ExecuteStateWithEvents(...)
ExecuteStatePerformedBy(...)
```

Action and state executor construction validates the symbol kind and calls the
lowering layer with the declaration and its scope.

### Values and expression evaluation

Runtime values represent constants, null, strings, instances, sequences, sets,
deferred expressions, quantities, variants, and enumeration literals.
Evaluation may depend on inputs, the current instance, local variables,
bindings, and runtime state.

Keep model-level constant evaluation in `semantics.Model.Eval` and executable,
stateful evaluation in `runtime`. If both need the same pure operator rule,
factor it without making semantic analysis depend on runtime state.

### Action execution

`ActionExecutor` executes an `ActionGraph` with token flow. It maintains:

- active tokens and stable token identifiers;
- node execution state;
- input and produced data;
- breakpoint and trace state;
- step-budget and completion state.

`Step` advances available work. `RunToCompletion` continues until completion or
a terminal failure.

An accept node with no matching message can be a valid waiting state during a
step. If no future progress is possible when running to completion, that
waiting state becomes an accept deadlock. Preserve this distinction.

Forks, joins, decisions, loops, nested execution, send/accept behavior, and
object flow all depend on deterministic token and data handling. Add a trace
golden whenever scheduling order is part of the observable behavior.

### State execution

`StateExecutor` executes a `StateGraph` using event-driven transitions,
hierarchical states, regions, pseudostates, entry/exit/do behaviors, deferred
events, guards, and transition effects.

State execution is split across files for statement behavior, regions and
transitions, and specialized triggers. Fix the smallest owner:

- graph construction and endpoint identity belong in lowering;
- event eligibility and transition selection belong in state execution;
- expression truth and values belong in evaluation;
- state entry/exit ordering belongs in executor scheduling.

### Budgets and robustness

Runtime budgets bound nonterminating loops, recursion, and other pathological
models. A budget is reset for each run. Exhaustion should return a typed error,
not hang or panic.

Robustness tests cover missing references, unbound parameters, cycles,
deadlocks, invalid triggers, invalid sends, state-history errors, quantity
errors, and overflow. New execution paths need equivalent failure coverage.

### Construction, initialization, and run-time errors

Do not move an error earlier merely because it is convenient. Some structurally
empty graphs can be constructed successfully, while initialization reports the
missing initial node or state. Callers and tests can rely on this timing.

When changing validation:

1. Identify the phase that has enough information to report the error.
2. Preserve existing constructor versus initialization behavior unless the
   contract is intentionally changing.
3. Add a test that calls the relevant phases separately.

### Adding executable behavior

An execution feature normally crosses several layers:

1. Parse the source into an explicit AST shape.
2. Build symbols and scopes for any new declarations.
3. Resolve referenced behaviors, endpoints, features, or events.
4. Validate language constraints in semantic passes.
5. Lower the behavior into `ActionGraph`, `StateGraph`, statements, bindings,
   or connections.
6. Execute only the lowered representation.
7. Add conformance, trace, and robustness coverage.

Do not stop at a runtime special case that searches the original declaration
for syntax the lowering layer omitted.

## REPL and LSP integration

The frontends share `model.Workspace` and core results. They should adapt core
data to protocol or interactive presentation, not define alternate language
semantics.

### REPL

`internal/frontend/repl` merges snippets into model content, reparses through the core
workspace, displays located diagnostics, resolves query targets, and invokes
runtime entrypoints.

For a REPL-only failure, compare:

1. the merged source content;
2. parser and workspace diagnostics;
3. symbol and resolver results;
4. direct runtime execution of the same target;
5. command-specific formatting or argument handling.

Snippet merging and selection are legitimate REPL concerns. Parsing,
resolution, conformance, and execution rules are core concerns.

### LSP

`internal/frontend/lsp` synchronizes buffers with the workspace and translates core
results into LSP responses:

- diagnostics come from workspace analysis;
- completion uses visible workspace members;
- definition uses reference resolution;
- rename uses resolver segment and alias information.

For an editor symptom, first reproduce the core query without the protocol
adapter. A wrong span may originate in the parser; a missing definition may
originate in scopes or resolution; stale results may originate in workspace
invalidation or LSP synchronization.

Do not add an LSP-only name resolver or semantic checker.

## How to implement a change

### New lexical form

Use this workflow for a new operator, literal form, delimiter, or reserved word:

1. Locate the nearest token and scanner implementation.
2. Confirm language-specific reservation rules.
3. Add longest-match scanning and progress on malformed input.
4. Add lexer tests for the accepted form, prefixes, suffixes, and EOF.
5. Consume the token in the parser and preserve its span.
6. Add parser golden, negative, and recovery coverage.
7. Extend semantic and runtime operator handling if applicable.

### New declaration or syntax production

1. Find the owning parent production and AST family.
2. Model written syntax in the AST.
3. Add minimal lookahead and parser dispatch.
4. Preserve visibility, membership, modifiers, spans, and trivia.
5. Add recovery at a stable grammar boundary.
6. Build a symbol and scope if the declaration is referenceable.
7. Add resolver behavior for any new reference form.
8. Add semantic validation at the appropriate tier.
9. Add lowering/runtime support if it is executable.
10. Complete the parser and behavior test contracts.

### New semantic rule

1. Reproduce the valid and invalid cases with the smallest models.
2. Determine whether the rule is syntax, resolution, type, or constraint level.
3. Put reusable facts in `semantics.Model`.
4. Put diagnostic policy in a pass.
5. Gate incomplete and downstream-failed elements.
6. Return stable diagnostics from workspace analysis.
7. Add positive, negative, and no-cascade tests.

### New action behavior

1. Lock the expected AST with a parser fixture.
2. Confirm action-local symbols and references.
3. Add structural/type validation.
4. Extend `ActionGraph` and action lowering.
5. Update token, data-flow, statement, or messaging execution.
6. Add an execution conformance case.
7. Add a trace when ordering is visible.
8. Add deadlock, missing-reference, unbound-value, cycle, and budget tests as
   applicable.

### New state-machine behavior

1. Lock states, regions, pseudostates, transitions, and triggers in the AST.
2. Confirm endpoint and event reference resolution.
3. Validate illegal topology or trigger use.
4. Extend `StateGraph` lowering.
5. Update transition selection, event handling, or entry/exit scheduling.
6. Add final-output conformance and ordering traces.
7. Add invalid endpoint, cycle, deferred-event, and nontermination coverage.

### Change to names, imports, or inheritance

1. Write a multi-file test with explicit scopes.
2. Inspect local symbols and fully qualified names.
3. Inspect imports, aliases, visibility, and re-exports.
4. Test insertion-order independence.
5. Test incremental replacement of an imported document.
6. Preserve resolver cycle detection and memoization keys.
7. Check definition, rename, and completion only after core resolution passes.

## How to diagnose a bug

### Reduce before editing

Create the smallest input that still fails and run the narrowest package test.
Record:

- source language and exact text;
- parser diagnostics and AST dump;
- relevant scope and symbol names;
- resolved target or resolution diagnostic;
- semantic diagnostic;
- lowered graph shape;
- runtime result, trace, and phase of failure.

This separates evidence from a theory about the cause.

### Trace upstream to downstream

Use this order:

1. **Source:** Is the correct file name, language kind, and content in use?
2. **Lexer:** Are token kinds and spans correct?
3. **Parser:** Is the intended AST present, and did recovery consume too much?
4. **Symbols:** Was the declaration registered in the correct scope?
5. **Resolver:** Did lookup start in the correct scope and apply visibility,
   imports, aliases, and inheritance?
6. **Semantics/passes:** Is the derived fact correct, and is the pass at the
   right level?
7. **Lowering:** Does the graph preserve all required execution information?
8. **Runtime:** Does execution correctly consume that graph?
9. **Frontend:** Is the correct core result converted or displayed?

Fix the earliest layer whose output is wrong. Later layers should not compensate
for a broken upstream representation.

### Common failure patterns

#### Valid text is tokenized incorrectly

Check longest-match order, contextual versus reserved words, numeric lookahead,
and language-specific keyword registration.

#### Parser reports many errors after one typo

Check whether `expect` failed without a recovery path, whether synchronization
stopped at the right boundary, and whether the outer parse loop still makes
progress.

#### A later declaration disappears after malformed input

Recovery probably consumed beyond the next declaration boundary. Add a test
that asserts both the diagnostic and the presence of the later AST member.

#### A name resolves in one body but not another

Compare owned scopes, parent scopes, body-local declarations, and the starting
scope supplied to the resolver. Do not broaden global lookup first.

#### Results depend on file insertion order

Inspect wildcard expansion, re-exports, index replacement, and memoized
resolver lifetime. The final index should be stable regardless of add order.

#### A semantic pass emits cascades

Move the rule to the correct level or gate elements downstream of parser or
resolution failures. Do not suppress the diagnostic by matching its text.

#### Behavior parses but does nothing

Inspect the lowered graph. If the node, edge, body, guard, trigger, or binding is
missing there, fix lowering. If the graph is correct, inspect executor state and
trace.

#### A runtime hangs

Run the robustness test with a timeout, inspect step-budget use, and determine
whether the executor is in a valid waiting state, a deadlock, or a progress loop.
Never remove a budget to hide the failure.

#### LSP results are stale

Compare a clean load with the edit sequence. Inspect document version/content,
workspace reindexing, diagnostic invalidation, and resolver replacement before
changing the protocol handler.

## Testing contracts

Run focused tests while iterating, then the full repository checks.

### Lexer and parser

Parser changes use four complementary layers:

```bash
go test -run TestStdlibConformance ./internal/workspace/libs
go test -run TestGolden ./tests/parser
go test -run TestNegative ./tests/parser ./internal/syntax/parser
go test ./internal/syntax/parser
```

Golden fixtures live in `tests/parser/testdata/parse`. Update them only
after an intentional AST change:

```bash
go test -run TestGolden -update ./tests/parser
```

Review every generated diff. A widespread snapshot change often identifies an
unexpected AST-shape or span regression.

Negative tests prove malformed input reports diagnostics. Recovery tests also
prove termination and preservation of later declarations.

### Symbols, resolution, semantics, and workspace

Run the directly changed package and its immediate consumers:

```bash
go test ./internal/semantic/symbols
go test ./internal/semantic/resolve
go test ./internal/semantic/semantics
go test ./internal/check/passes
go test ./internal/workspace/model
```

Cross-file changes should include imports, aliases, visibility, insertion order,
and incremental replacement where relevant.

### Execution

Behavior changes use:

```bash
go test -run TestExecutionConformance ./internal/exec/runtime
go test -run TestExecutionTrace ./internal/exec/runtime
go test -run TestRuntimeRobustness -timeout 60s ./internal/exec/runtime
```

Execution fixtures live in `internal/exec/runtime/testdata/conformance`:

```text
case.sysml
case.expected.json
case.trace.golden    # when ordering is observable
```

The harness supports action, state, calculation, constraint, requirement, and
satisfaction cases. Qualified entrypoints can select nested behavior.

Update traces only after intentionally changing scheduling or observable
ordering:

```bash
go test -run TestExecutionTrace -update-traces ./internal/exec/runtime
```

### Corpora

The training and pilot corpus gates exercise real, multi-file language input.
Use the download scripts described in [Build and test](#build-and-test). Their
policies deliberately differ: the training corpus is asserted clean and must
not be turned into a per-file ratchet, while the pilot corpora use adjudicated
per-file expectations. Do not weaken either policy to accept a regression.

### Documentation

For Markdown changes:

```bash
python3 scripts/check-doc-links.py
python3 scripts/check-doc-ids.py
python3 scripts/check-doc-figures.py
```

`make docs-check` runs the repository documentation checks. The last of them keeps
an oracle total from being read as current: outside the `doc-counts` generated
block every figure is a snapshot of the round that wrote it, so a page quoting one
has to say that it is not the current baseline. This guide remains
outside `docs/` and must not be added to `mkdocs.yml`.

### Full validation

Before opening a pull request:

```bash
make build
make test
make lint
```

If a focused package is slow, keep using focused tests while iterating, but run
the repository gates before considering the change complete.

## Definition of done

### Syntax change

- The lexer produces correct tokens and spans.
- The parser preserves the complete written structure.
- Invalid and incomplete forms recover without panic or loops.
- AST goldens are reviewed.
- The standard library still parses.
- New referenceable declarations are represented in scopes.

### Semantic change

- The rule lives at the correct validation level.
- Reusable facts live in the semantic model.
- Resolver and model instances are shared and memoized.
- Invalid upstream input does not produce cascades.
- Positive, negative, and partial-input tests pass.

### Behavioral change

- The AST fixture proves the syntax shape.
- Symbols and resolution identify all runtime references.
- Semantic validation rejects invalid models.
- Lowering preserves all runtime-relevant information.
- Runtime consumes lowered IR rather than reparsing declarations.
- Conformance output, ordering trace, and robustness failures are covered.
- Construction, initialization, and execution error timing is intentional.

### Any core change

- The earliest incorrect layer was fixed.
- No semantic state was added to the AST.
- No frontend-specific duplicate of core behavior was introduced.
- No test, security check, corpus gate, or execution budget was weakened.
- Formatting, build, tests, lint, and documentation checks pass.
- User-facing architecture or compliance documentation is updated only when the
  implementation actually changed.
