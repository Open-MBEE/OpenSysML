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
   writes. A held object reads the annotations of the declaration it stands
   for. Direct and nested bindings are emitted per annotation in declaration
   order, so the first value a single-valued surface reports is the first
   binding written.

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
  unchanged. Nested bindings have their own facts with their path from the body
  (`nested.value`); the direct facts keep their old shape. Inline and `about`
  sites are ordered by document and source position with their nested bindings
  kept beside them, so the first value is the one written first even when the
  annotation is stated in another document.
- **A model feature wins a shared name on every query surface.** The bare
  property name reads a declared or held feature first, then falls back to the
  evaluator's property, as `body`, `locale` and `language` already do. OSLC's
  `sysx:isActive` and `sysx:annotations` name that feature in filters and
  projections too when the model declares one. `@isActive` and `@annotations`
  (OSLC `sysx:@isActive`, `sysx:@annotations`) always read the evaluator's
  property. A translated OSLC request records which side each name came from,
  so the two sides stay separate on a row. When a model declares a feature
  under a bare built-in name, the translator defers type validation for that
  name to the row reader, so a filter on the feature is not judged by the
  built-in's type. An explicit built-in spelling keeps Boolean validation even
  when the bare name is shadowed. A filter decides operand validity from the
  rows the query started with, so reversing the terms of an `and` cannot change
  the verdict: an incompatible declared value under a shadowed name is skipped
  when another starting row supplies a comparable value, and a built-in defers
  only to such a comparable shadow. A shadow whose only value cannot take the
  operand never excuses it, even when an earlier term filtered that shadow's
  row out; a name with no comparable value reports the operator error. A state, verdict or event row delegating a
  dotted read keeps the terminal value's source, so built-in text is not
  mistaken for a declared feature. A union starts a new scope from the rows it
  emits; a row an arm filtered out cannot supply operand evidence above it.
  Every read carries its source into a projection: cells from a computed
  expression are compared from all of their values, where a comparison that
  returns false still counts as comparable and only a column with no comparable
  value reports the error, while a directly projected declared feature stays
  as strict as the same value before projection. An explicitly selected `@`
  built-in never probes a model feature of the same name, on element rows or on
  held objects, so reading declaration metadata cannot materialize a lazy part
  and start its behaviors.
- **One activity index per execution, not one session walk per state.** The
  first read walks the session once, charges that walk to the visit budget once
  and merges the active declarations of every machine. Invoked document queries
  share the index, so a wide projection over a large population stays in budget.
  Activity aggregates over every graph copy of one declaration, including
  graph-only parallel region owners and the machine usage; synthetic nodes map
  to no declaration.
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
| Query property | `isActive` | `query.PropertyIsActive`; selectable in `Query.select`, filterable in `Query.where` and OSLC `sysx:isActive`; `Project` and `WhereFeature` in document queries; `query.PropertyIsActiveBuiltin` spells the evaluator's own property unambiguously |
| Query property | `annotations` | `query.PropertyAnnotations`; the same four surfaces, as `sysx:annotations` in OSLC text, with `query.PropertyAnnotationsBuiltin` as the unambiguous spelling |
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
| A declared or held `isActive`/`annotations` feature is read by its name on every surface, including OSLC filters and projections; the `@` spelling reaches the evaluator's property | `internal/doc/queryexec/metadata_feature_shadowing_test.go:TestQueryDeclaredFeaturesShadowBuiltinsByName`, `:TestQueryHeldFeaturesShadowBuiltinsByName`; `internal/frontend/grpc/query_metadata_surfaces_test.go:TestBuiltinSpellingSurvivesAFeatureOfTheSameName`, `internal/frontend/grpc/query_oslc_shadowed_features_test.go:TestOSLCQueriesReadFeaturesShadowingBuiltinNames`; `internal/doc/querytranslate/oslc_test.go:TestOSLCUnambiguousBuiltinSpelling`, `:TestOSLCShadowedBuiltinNamesReadTheModelFeature` |
| A running exhibit, its parallel region owners and every copy of a duplicated state declaration answer activity | `internal/doc/queryexec/is_active_regressions_test.go:TestQueryActivityOfExhibitAndParallelRegionOwners`, `:TestQueryActivityAggregatesCopiesOfOneDeclaration` |
| Activity over a large row set and held population stays inside the visit budget, including invoked queries | `internal/doc/queryexec/state_activity_budget_test.go:TestQueryStateActivityStaysWithinTheVisitBudget` |
| A nested metadata binding is reported with its path by the bulk read | `internal/doc/queryexec/annotations_query_test.go:TestQueryReadsNestedAnnotationBindings` |
| A held object projects and filters its declaration's annotations, with its held feature still owning the bare name | `internal/doc/queryexec/object_annotations_query_test.go:TestQueryObjectRowsReadDeclarationAnnotations`, `:TestQueryFiltersObjectRowsByDeclarationAnnotations`; `internal/frontend/grpc/query_held_object_annotations_test.go:TestQueryRPCReadsAnnotationsOfAHeldObject` |
| A declared string `isActive` projects and filters as text, while `@isActive` keeps Boolean validation | `internal/doc/querytranslate/shadowed_builtin_kind_test.go:TestTableDefersKindOfAShadowedBuiltin`; `internal/frontend/grpc/query_shadowed_is_active_test.go:TestQueryFiltersAndProjectsAStringShadowingIsActive` |
| A nested binding of an earlier annotation precedes a later direct binding in the bulk read and in the Query RPC | `internal/doc/queryexec/annotation_order_query_test.go:TestQueryOrdersNestedAndDirectAnnotationBindings`; `internal/frontend/grpc/query_annotation_order_test.go:TestQueryRPCReportsFirstAnnotationInDeclarationOrder` |
| An `about` site before or after an inline site, in one document or across documents, decides the first value; equal source offsets are broken by document order | `internal/frontend/grpc/query_annotation_site_order_test.go:TestQueryRPCOrdersAnnotationsBySiteInOneDocument`, `:TestQueryRPCOrdersAnnotationsByDocumentAndSpan` |
| A bare `isActive` filter matches a row's string feature while skipping a state's Boolean activity row, and an explicit built-in spelling still refuses a text operand | `internal/frontend/grpc/query_mixed_activity_filter_test.go:TestQueryFiltersShadowedActivityAcrossMixedRows`; `internal/doc/querytranslate/oslc_builtin_validation_test.go:TestOSLCExplicitActivityBuiltinRejectsTextOperand` |
| A scoped filter compares the path's final property and skips an owner's annotation text beside another owner's numeric feature | `internal/frontend/grpc/query_scoped_mixed_builtin_test.go:TestOSLCScopedFilterSkipsIncompatibleBuiltinRows` |
| A native filter on unshadowed `isActive` with a text operand reports an invalid value | `internal/doc/queryexec/bare_builtin_operand_test.go:TestQueryRejectsInvalidBareActivityOperandWithoutAShadow` |
| A computed column named like a built-in is compared from its cells: an incompatible text cell does not inherit a shadow from the unused model path, while a heterogeneous column keeps its matching cell | `internal/doc/queryexec/shadow_deferral_regressions_test.go:TestQueryComputedColumnDoesNotInheritAModelShadow`, `:TestQueryFilterSkipsIncompatibleComputedCells` |
| A state row's text `isActive` feature shadows activity in a filter, while `@isActive` still rejects a text operand | `internal/doc/queryexec/shadow_deferral_regressions_test.go:TestQueryStateDeclarationShadowsActivityInAFilter` |
| A scoped filter on a text feature named like a built-in rejects an ordering operand | `internal/frontend/grpc/query_shadow_deferral_regressions_test.go:TestOSLCScopedTextFeatureRejectsOrderingOperand` |
| Reversing the terms of an `and` does not change whether a shadowed operand is accepted | `internal/frontend/grpc/query_shadow_deferral_regressions_test.go:TestQueryConjunctionOrderDoesNotChangeShadowDeferral` |
| Reversing the terms of an `and` does not change an ordering operand over declared numeric and text features | `internal/frontend/grpc/query_shadow_deferral_regressions_test.go:TestOSLCConjunctionOrderDoesNotChangeDeclaredFeatureOperand` |
| A text-only shadow filtered out by an earlier term still rejects an ordering operand on the built-in | `internal/doc/queryexec/shadow_deferral_regressions_test.go:TestQueryFilteredTextShadowStillRejectsOrderingOperand` |
| A mixed computed column with only nonmatching comparable cells returns no rows, and a projected declared feature keeps strict operand validation | `internal/doc/queryexec/shadow_deferral_regressions_test.go:TestQueryValidNonmatchingComputedCellIsNotAnOperatorError`, `:TestQueryProjectedDeclaredFeatureKeepsStrictOperandValidation` |
| A delegated dotted read on a state row keeps its built-in value's source, and a union's empty arm supplies no operand evidence | `internal/doc/queryexec/shadow_deferral_regressions_test.go:TestQueryStateDelegatedDottedReadKeepsBuiltinProvenance`, `:TestQueryUnionEmptyArmDoesNotSupplyOperandEvidence` |
| An explicit `@annotations` read on a held object leaves a namesake lazy part unmaterialized | `internal/doc/queryexec/object_explicit_annotation_lazy_part_test.go:TestExplicitAnnotationsOnAHeldObjectDoNotMaterializeANamesakePart` |
| A live value written into a held object's otherwise uninitialized `annotations` feature supplies operand evidence beside a built-in annotation's text | `internal/doc/queryexec/shadow_deferral_regressions_test.go:TestQueryLiveHeldShadowSuppliesOperandEvidence` |
| A directly projected shadowing feature keeps the deferral of the same unprojected read | `internal/doc/queryexec/shadow_deferral_regressions_test.go:TestQueryProjectedShadowKeepsDeferralEligibility` |
| A successful comparison against a directly projected number clears an incompatible computed cell's error in the same column | `internal/doc/queryexec/shadow_deferral_regressions_test.go:TestQueryComparableProjectedCellClearsComputedColumnError` |
| A feature literally named `owner.annotations` reads its own value and rejects an ordering operand, without borrowing the owner's `annotations` feature | `internal/doc/queryexec/shadow_deferral_regressions_test.go:TestQueryLiteralDottedFeatureDoesNotBorrowAPathShadow` |
| A feature hidden beneath a computed column cannot excuse an invalid operand on a built-in cell | `internal/doc/queryexec/shadow_deferral_regressions_test.go:TestQueryComputedColumnDoesNotInheritAHiddenShadow` |
| A projected shadow removed by an earlier conjunction term still excuses an incompatible cell, so reversing the terms cannot turn no rows into an error | `internal/doc/queryexec/shadow_deferral_regressions_test.go:TestQueryProjectedShadowOperandIsTermOrderIndependent` |
| A filter before `Project` does not hide a shadow from the filter after it, because projection keeps the pre-filter rows for on-demand evidence | `internal/doc/queryexec/shadow_deferral_regressions_test.go:TestQueryShadowEvidenceSurvivesProjectionAfterAFilter` |
| Projecting a row a filter excluded does not read that row's features, so a lazy part stays unmaterialized and its behavior does not start | `internal/doc/queryexec/starting_evidence_regressions_test.go:TestQueryProjectionOfExcludedRowsKeepsLazyPartsLazy` |
| Operand evidence reads a held object's lazy feature through a rollback-safe probe, leaving no materialization in the live session | `internal/doc/queryexec/starting_evidence_regressions_test.go:TestQueryHeldShadowEvidenceDoesNotMaterializeLazyParts` |
| A failed activity scan is reported rather than published as an incomplete index | `internal/doc/queryexec/starting_evidence_regressions_test.go:TestQueryFailedActivityScanIsNotTreatedAsInactive`, `:TestStateActivityIndexDoesNotPublishAFailedScan` |
| An unevaluable sibling column on an excluded row does not erase evidence from another projected column | `internal/doc/queryexec/starting_evidence_regressions_test.go:TestQueryExcludedRowKeepsEvidenceFromAnotherColumn` |

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
- `go test ./internal/exec/runtime -run 'TestRuntimeRobustnessStateIsActive|TestState|TestExploreWithIsExplore' -count=1` — passed after the activity aggregation change.
- `go test ./internal/semantic/query/... ./internal/semantic/semantics/... ./internal/ir/lower/... -count=1` — passed.
- `go test ./internal/exec/runtime -run 'TestRuntimeRobustnessStateIsActive|TestState|TestExploreWithIsExplore' -timeout 20m -count=1` — passed.
- `go test ./internal/frontend/grpc -run 'TestQuery' -timeout 15m -count=1` — passed.
- `go test ./internal/frontend/grpc -run 'TestQuerySelectsAnnotationAndActivityProperties|TestQueryReadsAnnotationsWithNoPerElementSymbolLookups|TestQueryFiltersByAnnotationAndActivityProperties' -count=1` — passed.
- `go test ./internal/frontend/grpc -run 'TestQuerySymbolKind' -count=1` — passed.
- `GOMAXPROCS=8 go test -count=1 -p 8 ./internal/doc/queryexec/... ./internal/doc/querytranslate/... ./internal/frontend/grpc/...` — all three packages passed; these are the targeted gates for this worktree, not a full `go test ./...` run.
- `python3 scripts/changelog-test.py` — 30 tests passed; `python3 scripts/spec_compliance-test.py` — 16 tests passed.

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
