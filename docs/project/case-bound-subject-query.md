# A case's bound subject on the Query surface

## Summary

A verification, analysis or use-case usage can bind its `subject` parameter to
a declaration in the model — for example
`subject :>> s = Structure::boilerAsMaintained;`. Until this change the Query
surface could name the subject *parameter* (a `ReferenceUsage` child) and the
ends of a `verify` relationship, but it could not answer the *declaration* the
parameter is bound to. This adds a `boundSubject` query property that reports
that declaration's fully qualified name on the case's query row, so a consumer
can join a case to the thing it verifies without resolving names itself.

## Why a downstream consumer needs it

A consumer that correlates a requirement with the modelled elements it affects
needs the subject of the verification case that checks the requirement. The
subject parameter alone is not enough: it is a feature declared by the case,
not the part, item or block usage the case checks. The consumer therefore has
to walk from the case row to the parameter, read the parameter's binding
expression, resolve that name in the case's scope, and follow any alias — once
per case. With a query that returns many case rows this is an N+1 pattern: one
query for the rows, then one lookup for each row's subject. Answering the bound
declaration on the row itself removes that second round trip and makes the join
a single query.

The absence case matters as much as the presence case. A case that binds no
subject must answer no value, not the parameter element and not some unrelated
element that happens to share a name; otherwise a consumer silently anchors a
finding to the wrong model element.

## What the baseline answered

At the pinned baseline `4d67c7a98f` (a nightly of `v0.12.1`, which itself is
`b2f0e9b802` of 2026-11-06):

- Issue [#2009](https://github.com/Open-MBEE/OpenSysML/issues/2009), closed
  2026-11-09, made a subject a `ReferenceUsage` on the reflection and query
  surfaces. `subjectParameter` was therefore queryable as the parameter element.
- PR [#2393](https://github.com/Open-MBEE/OpenSysML/pull/2393) exposed the
  `verify` relationship ends (`verifiedRequirement` / `verifyingCase`).
- View PRs [#2530](https://github.com/Open-MBEE/OpenSysML/pull/2530) and
  [#2550](https://github.com/Open-MBEE/OpenSysML/pull/2550) draw subject, actor
  and stakeholder edges in rendered views.
- No query property answered the declaration a subject binds. Reflecting the
  parameter's metadata (`(case.param) meta IfcRef`) answered an empty sequence
  for every case, linked or not, because an annotation lives on the declaration
  while the metadata filter classified the runtime value. The gap was recorded
  against a shipped check whose subject `Structure::boilerAsMaintained` was
  deliberately unlinked; no upstream issue or pull request proposed a field
  for the bound declaration.

## What this change implements

A memoized semantic derivation reads the case's subject member and, when that
member states no value of its own, follows the subjects it redefines. It takes
the expression of a plain `=` binding (not a `default` and not an initial `:=`
value), resolves it in the scope where it is written, follows an alias to its
target, and returns the target element. It deliberately never returns a subject
parameter from the chain. The result is cached per element, so projecting the
property over every row of a query resolves each binding once.

That derivation is exposed as the `boundSubject` query property on the SysML v2
API `Query` and the `sysx:boundSubject` spelling in OSLC query text. It joins
the closed set of query properties, so `select`, `where` and `orderBy` all
accept it and an unknown name is still refused. It is element-valued: a
comparison against a qualified name selects cases by the declaration they bind,
and a `Project` reports the element.

## Design decisions and alternatives rejected

- **A query property, not a new proto field.** `QueryResultElement.properties`
  is a generic string map. Adding a named property to the closed set needs no
  wire change, so none of the client libraries, generated protobufs or the
  capability list move. This is the pattern already used for `symbolKind`,
  requirement `text`, and the `verify` ends. A dedicated `SymbolInfo` or row
  field would have required regenerating every client and a new capability flag
  for a value the existing map already represents; it was rejected.
- **The semantic layer owns the derivation.** Reading a binding expression and
  resolving it is a semantic fact, like `SatisfyEndsOf` and `VerifyEndsOf`. It
  lives in `internal/semantic/semantics`, not in the gRPC or query-execution
  layers, and the AST is untouched.
- **Resolve the binding expression, not the redefinition target as a declaration.**
  In `subject :>> s = Structure::boilerAsMaintained;`, the `:>> s` edge redefines
  the inherited subject parameter; the declaration is named by the `= …` value.
  Reading the redefinition target itself would answer the parameter — exactly
  the wrong element the absence rule forbids. When the redefining subject states
  no value of its own, the redefinition chain is followed to a general subject
  that does bind a declaration, so the binding is inherited. A visited set
  guards cycles, and no subject parameter in the chain can be the answer.
- **A plain `=` is a binding; `default` is not.** A subject that declares a
  default (the standard library's `subject subj default Case::result;` is the
  common form) is a fallback for a run, not an element the case is verifying;
  it is reported as absent.
- **Memoize per element, not per row.** The cache is keyed by the case symbol
  on the semantic model, so two queries over the same model and repeated rows
  share one answer. The resolver's own memoization covers repeated name
  lookups.

## Public surface

| Surface | Name | Notes |
|---|---|---|
| API `Query` property | `boundSubject` | Selectable, comparable and sortable; absent on a row with no binding |
| OSLC Query text | `sysx:boundSubject` | OpenSysML-derived namespace, like `sysx:symbolKind` |
| Semantic method (Go) | `semantics.Model.BoundSubjectOf` | Returns `(target *symbols.Symbol, ok bool)`; `ok == false` is the explicit absence |
| Internal query constant | `query.PropertyBoundSubject` | Exported by `internal/semantic/query` for the query surfaces |

There is **no new RPC, no new wire field, no new capability flag and no new
file format**. The existing `query` capability governs the property, and the
existing `oslc_query` capability governs its OSLC spelling. Row values keep the
qualified-name text already written in `QueryResultElement.properties`; there
is no versioned format to bump.

## Acceptance criteria and the tests that lock them

| Criterion | Named test |
|---|---|
| A query on a case whose subject is redefined/bound to a declaration answers that FQN | `internal/frontend/grpc/query_boundsubject_test.go:TestQueryBoundSubjectNamesTheBoundDeclaration` (pins `Structure::boilerAsMaintained` and `Structure::spareBoiler` from `tests/testdata/bound-subject/verification_subjects.sysml`) |
| A case binding nothing answers an explicit absence, never another element | the same test (`Verification::unboundSizing` has no property; the `s` parameter never appears as a value) and `internal/semantic/semantics/bound_subject_test.go:TestBoundSubjectOfNamesTheBoundDeclaration` |
| Both binding forms, a feature chain, a default, an initial value and an inherited binding are read | `TestBoundSubjectOfNamesTheBoundDeclaration` (`:>> s = …`, `s = …`, absence) and `TestBoundSubjectOfReadsChainsDefaultsInitialAndInheritedBindings` (chain FQN, absent default and initial value, redefining subject with no value inheriting the general's binding) |
| The property constrains rows as well as projecting them | `query_boundsubject_test.go:TestQueryBoundSubjectSelectsCasesByTheirBinding` and `:TestQueryOSLCBoundSubject` |
| An undeclared name near the new one is still refused | `query_boundsubject_test.go:TestQueryStillRefusesAPropertyNextToBoundSubject` |
| No per-row re-resolution explosion | `internal/semantic/semantics/bound_subject_test.go:TestBoundSubjectOfResolvesEachBindingOnce` (the resolver memo does not grow on a second read) and `client/opensysml/bound_subject_internal_test.go:TestQueryBoundSubjectNeedsNoPerRowLookup` (one `Query` RPC, zero `GetSymbol` RPCs for a listing) |
| The namespace table stays consistent with the derived-property set | `internal/doc/querytranslate/oslc_test.go:TestOSLCPropertyNamespacesFollowTheMetamodel` and `:TestParseOSLCPropertyDiagnosticsFollowTheActiveBindings` |

## Verification

Gates run on this head, with `GOMAXPROCS=2`, `-p 2` for builds and targeted
tests, `-p 1` for the hygiene suite, and the Go temp directory on the home
filesystem:

- `gofmt -l .` — no output.
- `go build -p 2 ./...` — success.
- `go vet -p 2 ./...` — success.
- `python3 scripts/changelog.py check` — success.
- `python3 scripts/check-doc-ids.py` — no work-item labels in reader-facing text.
- The added-comment gate over `git diff --merge-base origin/develop -- '*.go'` —
  no output.
- `go test -count=1 -p 1 ./tests/hygiene` — `ok`.
- Targeted tests, all `ok`: the `TestBoundSubjectOf*` tests in
  `internal/semantic/semantics`; `TestQueryBoundSubject*`,
  `TestQueryOSLCBoundSubject` and
  `TestQueryStillRefusesAPropertyNextToBoundSubject` in `internal/frontend/grpc`;
  `TestQueryBoundSubjectNeedsNoPerRowLookup` in `client/opensysml`; and the full
  packages `internal/semantic/query`, `internal/doc/querytranslate` and
  `internal/doc/queryexec`.

The whole-tree `go test ./...` is left to CI; it was not completed here. The
optional corpus and toolchain gates — the OMG training, pilot and
specification-example corpora, the pilot library XMI, the normative JSON schema,
the PSSM suite, the reference FMUs and the document-converter toolchains — were
not run because the inputs are absent and their `require` variables are unset;
those tests skip by design in that state.

## Known limitations and follow-ups

- A redefinition edge alone is not a declaration. A redefining subject with no
  value of its own inherits a general's binding when one exists; when no subject
  in the redefinition chain states a plain `=` binding, the property is absent.
  The redefinition target itself is never answered as the declaration.
- The property reports one declaration. A multiplicity of subjects bound to
  several declarations is outside this surface; the single-valued query row
  cannot represent all of them without changing the wire shape.
- The standard library's own `subject subj default Case::result;` is a default
  and is reported as absent, as above. Consumers that need the run-time object
  continue to use the instance and verification surfaces.
- A downstream consumer that joins a requirement to the affected elements must
  still follow its own relationship mapping from the case to the requirement;
  this change supplies the subject half of that join only.

## Evidence

- Baseline subject shape: [issue #2009](https://github.com/Open-MBEE/OpenSysML/issues/2009) and [PR #2393](https://github.com/Open-MBEE/OpenSysML/pull/2393).
- Subject edges in views: [PR #2530](https://github.com/Open-MBEE/OpenSysML/pull/2530) and [PR #2550](https://github.com/Open-MBEE/OpenSysML/pull/2550).
- The query-property pattern this follows is documented in `docs/reference/api.md` (property table) and the unreleased changelog fragment `changes/unreleased/query-bound-subject.added.md`.
