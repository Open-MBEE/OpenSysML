# Interface records

A document nobody is editing does not need its syntax tree in memory. What
the rest of a workspace can observe of it through the language — the elements
its qualified names and imports reach, and what each one is — is a small
fraction of what analyzing it produced. An **interface record** is that
fraction, written once from a full analysis and installed in the index as
tree-less scopes and symbols; the document's diagnostics are stored with it
and served verbatim. This note is the account of what a record carries, what
it leaves out, and why each line is where it is. The contract it serves is
`TestInterfaceRecordDifferential` (`tests/model/interface_record_test.go`):
with any one document of a root recorded, every other document's diagnostics
and the resolution of every reference into the recorded one are byte-for-byte
what they were with it loaded. `docs/project/large-model-scaling-design.md`
§4 is the design this implements; `docs/project/lossless-library-records.md`
is the account of what happened the last time a record dropped a fact a
reader needed, and the reason every line below argues from what a reader can
observe rather than from what looks small.

## Where it lives

| piece | where |
| ----- | ----- |
| the scope tree without trees | `symbols.DocumentRecord` / `ScopeRecord` / `SymbolRecord` (`internal/semantic/symbols/record.go`) |
| what a symbol's declaration yields | `symbols.LibraryFacts` (`internal/semantic/symbols/facts.go`), the same facts the library cache attaches |
| stable references between symbols | `symbols.ElementRef` (`internal/semantic/symbols/ref.go`) |
| what the workspace-wide audits read from bodies | `symbols.GatheredRelationships` (`internal/semantic/symbols/gathered.go`) |
| the record with its diagnostics, and the cache | `libs.InterfaceRecord`, `Cache.InterfaceKey/StoreInterface/LoadInterface` (`internal/workspace/libs/interface.go`) |
| the writer | `libs.WriteInterface`, called by `model.Workspace.InterfaceRecord` |
| installing a record | `symbols.BuildRecorded`, `Index.AddRecordedDocument`, `model.Workspace.OpenRecorded` |

The record is the library `IndexRecord` generalized, not a second format:
`libs.IndexRecord` already carried a symbol's kind, supertypes, unit and
dimension facts as `LibraryFacts`; the interface record carries the same
struct with the fields a user document's readers need added, and the library
record's supertypes now use the same `ElementRef` the interface record does.
Both are persisted by `libs.Cache` in the same directory
(`$XDG_CACHE_HOME/sysml-ls/libs`, or the platform's user cache), with its
atomic writes and idle-age pruning.

## What is in

Every registration of every namespace of the document, in declaration order,
whether named or anonymous, public or private: `ScopeRecord.Members`.

*Argument from visibility.* A qualified name from another document can reach
any member of a namespace, and what it finds — a symbol, an anonymous
registration that occupies a position, or a private member it may not see —
decides the diagnostic the reader reports. "`P::x` is not visible" and "`P::x`
does not exist" are different diagnostics, so the private member has to be
there with its `Visibility`. Anonymous registrations keep their position
because an `ElementRef` names an unnamed element by its member ordinal.

Per symbol (`SymbolRecord`): `Name`, `ShortName`, `Kind`, `Visibility`,
`Naming` (how the name is derived, so a redefinition named by what it
redefines is still named that way), `DeclSpan` and `NameSpan` (a diagnostic
about the element, and go-to-definition, point into its file), and `Facts`.

Per symbol, the facts (`LibraryFacts`), each with the reader that needs it:

| fact | read by |
| ---- | ------- |
| `Supers` — direct supertypes, resolved | inheritance, member lookup through the type, conformance |
| `Redefines`, `References` — redefined features, the feature a reference subsets | masking, distinguishability, end typing |
| `Alias` — an alias's target | qualified-name resolution through the alias |
| `Direction`, `Modifiers` — `in`/`out`/`inout`, `end`, `derived`, `variation`, `variant`, `abstract`, `individual`, `ordered`, `nonunique`, `constant`, `parallel`, and the rest of the boolean traits a declaration states | typing, redefinition conformance, variation and individual checks, invocation shapes |
| `Multiplicity` — the declared bounds | multiplicity conformance of a redefinition, end multiplicities |
| `Unit`, `Dimension` — the reduced unit or dimension a library-style declaration denotes | quantity typing, unit conversion |
| `Annotations`, `About` — the metadata declared on the element with its literal values, and the elements an annotating usage is about | metadata filters on imports, `@`-annotated lookups, the identity-metadata audit |
| `BaseType`, `ModBindsBaseType` — the base type a metadata definition binds unconditionally | metadata typing |
| `Relationships` — the relationship members (dependency, satisfy, allocate, …) with their resolved targets | relationship queries, the OOSEM and MOSA audits |
| `Node`, `Keyword`, `UsageKind`, `DefKind` — what kind of declaration it was | every reader that used to switch on the declaration's type |
| `Abstract`, `ModAcceptPayload` | abstract-type checks, accept-action typing |
| `Recorded` — true for every recorded symbol | `Symbol.Recorded()`, which tells a reader it holds a record |

Per scope: the `import` declarations and `filter` conditions, kept as small
syntax trees in the record's node table (`DocumentRecord.Nodes`). An import
contributes to a namespace another document reads, and a filter condition is
evaluated against the metadata of what comes through; both are expressions
the resolver interprets, and both are a few tokens.

Per document: the relationships the workspace-wide OOSEM and MOSA audits
gather from bodies — derivations, allocations, conformances and satisfactions
with their ends — as `GatheredRelationships` over `ElementRef`s. These audits
report on *other* documents from what this one states, so the facts are
observable from outside even though they come from a body.

Beside the record: the document's **diagnostics**, verbatim. A recorded
document is never analyzed again; `Workspace.Diagnostics` and
`DiagnosticsAll` return the stored list.

## What is out

- **Behavior bodies, statements, expressions, values.** No other document
  reaches into an action body's control flow or a feature's value expression
  through the language; what a value *evaluates to* is the runtime's business,
  and the runtime never runs against a record (below).
- **Connections and flows as trees.** The connector's ends are body facts;
  what other documents see — the connector symbol, its kind, its type — is a
  registration with facts like any other. Where a body-local scope owned them
  (`Scope.bodyLocal`, annotated bodies), the scope is not recorded.
- **Constraint and requirement text.** The constraint's result expression is
  an expression.
- **Comments and documentation.** Documentation is read from the tree by
  hover and the document renderer, which hydrate.
- **Anything the resolver memoized.** The record holds facts, not the memo
  tables; a recorded document owns no resolver frame.

## Stable references

A fact that names another element cannot always do so by qualified name: an
anonymous supertype, an implicit end, a redefinition of an unnamed member.
`ElementRef` is the fully-qualified name of the nearest enclosing element that
name alone declares, then one member ordinal per step down to the element the
name does not reach, plus the declaring document when several documents
declare the same name. `Index.RefTo` writes one; `Index.Element` restores it
against the live index, so the reference joins whatever residency its target
has. When an element has no such reference — one declared twice in its own
document — the writer refuses the record (`libs.ErrUnrecordable`) rather than
writing an approximate one, and the document stays loaded.

## What the writer refuses

`WriteInterface` returns `ErrUnrecordable` for a document whose interface it
cannot state without the tree, and the document is held loaded. Today that is:
a supertype, redefinition, alias, annotation, relationship or base-type target
no `ElementRef` reaches; supertypes the semantic model marked provisional; a
quantity-valued annotation; and a metadata definition whose `baseType` is
bound conditionally. In the four OMG corpora one document of 313 is refused
(`kerml-examples/Simple Tests/MetadataTest.kerml`, a conditional `baseType`
binding). A refusal is never silent: the error names the fact and the element.

## Cache key

`Cache.InterfaceKey(content, libraryDigest, mode)` is the document's content
hash, the digest of the library set it was analyzed against, the conformance
mode, the build identifier and the record format version. A record written
under any other library, mode, build or format is never found; nothing has to
be invalidated. `interfaceFormatVersion` in `libs/interface.go` moves whenever
`LibraryFacts` or the record structs change shape or meaning.

## Readers of the tree

A recorded symbol has `Decl == nil`, `Facts != nil` and `Facts.Recorded`. Every
production reader of `Symbol.Decl` under `internal/semantic`, `internal/check`,
`internal/workspace` and `internal/frontend` falls in one of four classes:

1. **Fact readers.** The resolver and semantic model read the fact when the
   symbol is recorded and the tree when it is not; these are the paths the
   differential test exercises for every fixture and corpus document.
2. **Readers of the document under analysis.** A check pass reads the
   declarations of the document it is analyzing, and a recorded document is
   never analyzed; a reader of a *foreign* symbol in a pass goes through the
   semantic model (class 1).
3. **Body facts that are not externally observable.** Connector ends, control
   nodes, a state's transitions: a reader asked about a recorded symbol
   answers "none", which is what the foreign document could observe anyway.
4. **Questions the tree alone answers.** Editing, renaming, printing an
   element's notation, and building a runtime over the workspace return
   `symbols.ErrNeedsHydration` (`symbols.NeedsTree`, `symbols.NeedsHydration`
   with the document and the question) for a recorded document; the frontend
   that receives it hydrates (a later change) or reports it. `Workspace.NewRuntime`
   refuses a workspace holding a recorded document, so the runtime never
   evaluates against a record.

## What the record costs

`BenchmarkPlaneResidency` in `tests/stressmodel/residency_test.go` holds a
constellation split one file per plane with the planes loaded and with them
recorded, and reports the reachable heap in both states, what installing the
records alone added, and a plane's record size on disk;
`TestPlaneResidencyProcess` holds either state in a process of its own for an
external RSS measurement. The figures are in `docs/internals/performance.md`
and `docs/project/satellite-network-stress-test.md`.
