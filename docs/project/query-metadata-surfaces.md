# Query and metadata surfaces

This record describes four capabilities of the model-query and editing layer:
relationship edges for dependencies and allocations, writable annotation values,
a Boolean activity property on state usages, and a bulk metadata-read property.
It is written for a reader who has only this tree.

## Why a downstream consumer needs them

A tool that builds a model explorer, a traceability matrix or a verification
dashboard reads a model through queries rather than by walking declarations one
at a time. Four gaps made that expensive or impossible:

- A dependency or allocation between two elements could not be traversed as a
  row-set edge, so a traceability view had to resolve each declaration on its
  own and join the results outside the engine.
- A metadata annotation's bound value could not be edited. Setting the value of
  the metadata definition's feature changed the definition's default for every
  use, not the binding on the element that carried the annotation.
- The runtime could answer whether a state was active while an expression was
  being evaluated, but a query over state usages had no Boolean activity
  property, so a dashboard had to drive a separate evaluator.
- Reading metadata values for a set of elements required a symbol lookup for
  each element and another lookup for each metadata usage, turning one logical
  query into a round trip per row.

The first and fourth gaps are about doing one query's work in one query; the
second is about targeting the right declaration; the third is about exposing a
value the engine already computes.

## State at the baseline

The baseline is gitlink `4d67c7a98f` (described as `v0.12.1-2379-g4d67c7a98f`,
2026-11-18), with the query and metadata state re-verified against this tree at
`9a71c245cd` (2026-11-24). Upstream evidence for the parts that had already
landed: [issue 2339](https://github.com/Open-MBEE/OpenSysML/issues/2339) /
[PR 2391](https://github.com/Open-MBEE/OpenSysML/pull/2391) added the
`symbolKind` row property, the OSLC `sysx:symbolKind` spelling and requirement
`text`; [PR 2534](https://github.com/Open-MBEE/OpenSysML/pull/2534) added
text-only metadata inspection; [PR 2203](https://github.com/Open-MBEE/OpenSysML/pull/2203)
added `annotating_metadata_ids`; and
[PR 2012](https://github.com/Open-MBEE/OpenSysML/pull/2012) /
[PR 2013](https://github.com/Open-MBEE/OpenSysML/pull/2013) made
`GetElementProperties` read any declared metamodel property by its specification
name. `symbolKind` is present in `internal/semantic/query/properties.go` and
`PropertySymbolKind` in `internal/semantic/query/query.go`, with conformance in
`internal/frontend/grpc/query_symbolkind_text_test.go`; it is unchanged here.

What was still missing at that baseline:

- `supportedRelationship` in `internal/doc/queryexec/related.go` accepted the
  lineage kinds plus connection, allocation, satisfaction, verification,
  derivation and refinement. It refused `dependency` outright; neither
  `dependency` nor `allocate` was accepted as a kind, and the refusal did not
  say which kinds were accepted. The underlying semantic edge tables already
  knew both: `internal/semantic/semantics/relationship_edges.go` derives
  dependency edges from `dependency` declarations and allocation edges from
  `allocate` usages.
- `internal/check/edit/locate.go` resolved a set-value target only to a
  declaration. An annotation body's bound member resolved to no element for the
  names and positional ids tried, and the like-named feature on the metadata
  definition resolved to the definition's default instead.
- `StateUsage` had no query-row Boolean. The runtime read `isActive` from the
  `StateActivity` extension library
  (`internal/semantic/semantics/state_activity.go`,
  `internal/exec/runtime/state_activity.go`), but `StateExecutor.activity` was
  unexported and the query layer did not consult it.
- There was no query property that read annotations for a row set. A caller had
  to ask for `annotating_metadata_ids` and then call `GetSymbol` once per
  annotation to read its attributes.

## What this change implements

1. **Dependency and allocation edges.** `dependency`, `allocation` and the
   notation spelling `allocate` are accepted relationship kinds for
   `RelatedElements`, `RelatedColumn` and `WhereRelated`. `allocate` maps to the
   existing allocation edge table; `dependency` maps to the dependency edge
   table. A refusal names every accepted kind, in stable order, so the
   vocabulary is discoverable from the error.
2. **Annotation-value writes.** A new edit operation targets the annotation
   usage's body binding by the annotated element, the metadata type and the
   feature. It rewrites only the value expression's bytes. When the body does
   not bind a value yet, it inserts one before the declaration's terminator:
   a feature written as `label;` contributes no semantic binding, so the edit
   also looks for the member in the annotation body itself. The metadata
   definition's default is never touched.
3. **Boolean activity on state usages.** `isActive` is a query property whose
   value comes from the state machines of the query's runtime session. It reads
   the same executor configuration the expression evaluator uses, exposed as
   `StateExecutor.ActivityOf`. A state the machine entered answers `true`, a
   state it never entered answers `false`, and every state answers `false` when
   no session is supplied.
4. **Bulk metadata read.** `annotations` is a query property over a row set.
   For each row it reports the values its annotation bodies bind as
   `<metadata def>::<feature>=<value>` strings. It is readable by `Project`,
   `Column(path)`, `OrderBy` and `WhereFeature`, so one query answers the
   metadata of every row. Type defaults are not reported, only values the body
   writes.

## Design decisions and alternatives rejected

- **Reuse the semantic edge tables rather than add a second relationship
  walk.** The semantic model already resolves dependency and allocation ends,
  including `about`-clause annotations and inherited ends. Teaching
  `supportedRelationship` about the two new names and mapping `allocate` onto
  the existing allocation kind is the whole change; a parallel edge scan in the
  query layer would drift.
- **A dedicated edit operation rather than overloading `SetValue`.** A bound
  member's qualified name is not unique across annotation usages, so a new
  operation carrying the annotated element, the metadata type and the feature
  identifies the target without a name convention that depends on where the
  annotation was written. Rewriting the definition's default was the existing
  behavior and had to stay available for callers that want it.
- **Read activity from the executor, not from a second record.** The runtime
  already tracks the active configuration; the query layer asks it through one
  exported method. Recording activity in a side table would be a second source
  of truth. A simple attribute on `StateUsage` was rejected because activity is
  a property of a running machine, not of the declaration.
- **Facts carry a `Bound` flag rather than dropping defaults.** Annotation
  facts already report every value a reader can use, including values inherited
  from the metadata type. The bulk property needs only body bindings, so each
  fact now records whether the body wrote it. Consumers that read all facts are
  unchanged.
- **Text values in a row cell, not a new wire message.** The Query result
  element already reports a property's first value as text and document queries
  carry every value in a column. Encoding a binding as
  `<metadata def>::<feature>=<value>` keeps the same representation on both
  surfaces and needs no protocol change. A new `Query` message field would have
  required regenerating every client for a read that existing properties can
  express.

## Public surface added

| Surface | Name | Where |
|---|---|---|
| Query property | `isActive` | `query.PropertyIsActive`; selectable in `Query.select`, filterable in `Query.where` and OSLC `sysx:isActive`; `Project` and `WhereFeature` in document queries |
| Query property | `annotations` | `query.PropertyAnnotations`; the same four surfaces, as `sysx:annotations` in OSLC text |
| Relationship kinds | `dependency`, `allocate`, `allocation` | `RelatedElements`, `RelatedColumn`, `WhereRelated` `relationshipKind` argument; the accepted set is exported as `queryexec.SupportedRelationships()` |
| Edit operation | `OpSetAnnotationValue`, `edit.SetAnnotationValue(about, metadataType, feature, value)`, `Operation.AnnotationFeature`, `Operation.MetadataName` | `internal/check/edit`; `MetadataName` disambiguates several annotations of one type on one element |
| Go API | `runtime.StateExecutor.ActivityOf(*symbols.Symbol) (bool, bool)`, `lower.StateGraph.StateSymbol(*ast.StateNode) *symbols.Symbol` | exported from their layers for the query property and to remove a duplicate symbol lookup |
| Semantics fact | `symbols.AnnotationValueFacts.Bound` | distinguishes a body binding from a type default |

No protobuf field, capability flag or RPC was added. The Query RPC's existing
`select`/`where` properties carry the two new property names; no file format
changes. `AnnotationValueFacts` is computed from the parsed model and is not
part of the bundled library snapshot or the symbol-facts record encoding, so
the new field does not change a snapshot version. No client library needs an
update: the Go edit operation is available to in-process callers, and the
existing Query and OSLC surfaces carry the new properties without a schema
change. Exposing `SetAnnotationValue` through the edit RPC would need a new
protobuf oneof arm and regenerated clients; that is a follow-up, not part of
this change.

## Acceptance criteria and their tests

| Criterion | Test |
|---|---|
| One query over a model declaring a dependency and an allocation returns source and target rows from either end; the two kinds stay distinct | `internal/doc/queryexec/related_test.go:TestExecuteRelatedDependencyAndAllocate` |
| A kind outside the set is refused with a message naming every accepted kind | `internal/doc/queryexec/related_test.go:TestExecuteRelatedRefusalNamesTheSupportedKinds`, with the exact message also pinned by `internal/doc/queryexec/related_column_test.go:TestExecuteRelatedColumnReportsItsColumnInErrors` |
| `ApplyEdits` changes an annotation usage's binding from `old` to `new`, leaves the definition default intact, parses cleanly and pins the changed span | `internal/check/edit/annotation_value_test.go:TestSetAnnotationValueRewritesTheUsageBinding` |
| The inline body form is reachable and an unbound feature is refused with a typed error | `internal/check/edit/annotation_value_test.go:TestSetAnnotationValueRewritesAnInlineBinding`, `:TestSetAnnotationValueRefusesAnUnboundFeature` |
| A feature written without a value (`label;`) gains one on the usage while the definition's default stays intact | `internal/check/edit/annotation_value_test.go:TestSetAnnotationValueAddsValueToUnboundFeature` |
| A started state answers `isActive = true` and a never-entered one `false` in one query | `internal/doc/queryexec/is_active_query_test.go:TestQueryAnswersStateActivity` |
| `isActive` stays a Boolean property without a session and is absent on non-state elements | `internal/doc/queryexec/is_active_query_test.go:TestQueryStateActivityIsFalseWithoutASession` |
| One query over N annotated elements answers their annotation values and the same property filters a row set | `internal/doc/queryexec/annotations_query_test.go:TestQueryReadsAnnotationsForARowSet`, `:TestQueryFiltersByAnnotationValue` |
| The annotation read makes zero per-element symbol calls, counted by a client wrapping `GetSymbol` | `internal/frontend/grpc/query_metadata_surfaces_test.go:TestQueryReadsAnnotationsWithNoPerElementSymbolLookups` |
| The two properties are selectable and filterable through the Query RPC with no schema change | `internal/frontend/grpc/query_metadata_surfaces_test.go:TestQuerySelectsAnnotationAndActivityProperties`, `:TestQueryFiltersByAnnotationAndActivityProperties` |
| `symbolKind` remains present and is still governed by the same Query surface | `internal/frontend/grpc/query_symbolkind_text_test.go:TestQuerySymbolKindNamesEveryRow`, `:TestQuerySymbolKindSelectsRowsOfThatKind` |

The tests execute document queries over real `.sysml` fixtures through the
shared `queryexec` executor, which is the executor behind the CLI, the REPL, the
document-query RPC and the API Query RPC. The bulk-read test asserts the whole
row shape: each annotated element answers its binding and the unannotated
element answers none, from one execution. Per-element symbol lookups are not on
that path; the read goes through the memoized annotation facts of the semantic
model, and the counted client observes no `GetSymbol` calls.

## Verification

All commands were run from the repository root.

- `gofmt -l .` — no output.
- `go build ./...` — passed.
- `go vet ./...` — passed.
- `python3 scripts/changelog.py check` — passed.
- `python3 scripts/check-doc-ids.py` — "No internal work-item labels in reader-facing documentation".
- `python3 scripts/spec_compliance.py check` — "57 sources are well-formed".
- `go test ./internal/doc/queryexec/... ./internal/doc/querytranslate/... ./internal/check/edit/... -count=1` — passed.
- `go test ./internal/semantic/query/... ./internal/semantic/semantics/... ./internal/ir/lower/... -count=1` — passed.
- `go test ./internal/exec/runtime -run 'TestRuntimeRobustnessStateIsActive|TestState|TestExploreWithIsExplore' -timeout 20m -count=1` — passed.
- `go test ./internal/frontend/grpc -run 'TestQuery' -timeout 15m -count=1` — passed.
- `go test ./internal/frontend/grpc -run 'TestQuerySelectsAnnotationAndActivityProperties|TestQueryReadsAnnotationsWithNoPerElementSymbolLookups|TestQueryFiltersByAnnotationAndActivityProperties' -count=1` — passed.
- `go test ./internal/frontend/grpc -run 'TestQuerySymbolKind' -count=1` — passed.
- `go test ./... -timeout 30m -count=1` — 128 packages passed. The run was terminated by its 40-minute harness limit while other worktrees competed for the container's CPU and `/tmp` quota. The packages that failed did so for environmental reasons unrelated to this change: test-built binaries could not be written under `/tmp` ("disk quota exceeded") in `cmd/sysml`, its sibling commands and `tests/codegen`; downloaded corpora were absent in `tests/corpus` and `internal/translate/export`; a solver timed out in `internal/exec/smt` and `internal/exec/solve`; an exploration test exceeded its time budget in `internal/exec/runtime`; `internal/frontend/grpc` hit its package timeout; and `internal/ir/view` reported a `dot` warning about a non-ASCII glyph. Every package whose code this change touches passed in the targeted runs above.

## Known limitations and follow-ups

- The edit operation is a Go API only. Exposing it through the edit RPC and the
  client libraries needs a protobuf oneof arm and regenerated clients.
- `annotations` reports one value per binding as text and the Query result
  element carries the first value of a property. A consumer that needs every
  binding of a many-valued annotation on a single row should use a document
  query's `Project`, which carries every value in the column.
- `isActive` consults state machines exhibited by objects in the query's
  session. A state machine executed directly without an instantiated object is
  not part of that population, so its states answer `false`.
- An annotation with several matching bindings of the same metadata type on one
  element is refused until the caller names the annotation usage; the
  single-binding case is the common one.
