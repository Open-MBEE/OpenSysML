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
| `Alias` — the element an alias's own target names, an alias itself in a chain; zero when it names nothing | qualified-name resolution through the alias, an alias to nothing skipped as loaded |
| `Direction`, `Modifiers` — `in`/`out`/`inout`, `end`, `derived`, `variation`, `variant`, `abstract`, `individual`, `ordered`, `nonunique`, `constant`, `parallel`, `ModResult`, `ModValued` (the declaration binds a value), and the rest of the boolean traits a declaration states | typing, redefinition conformance, variation and individual checks, the parameter list of a behavior — which parameters a call binds, which it may leave unbound, its result |
| `ModNamesNothing` — a name borrowed from a referenced or redefined feature that resolved to no feature | `Resolver.BindsName`: whether the member is found by that name, and whether a specialization declaring it inherits a duplicate |
| `Multiplicity` — the declared bounds, evaluated; a bound written as the name of a feature the declaring scope values carries that value, marked so a reader evaluating without the scope sees it unknown, as it does loaded | multiplicity conformance of a redefinition, end multiplicities; whether a `multiplicity` member is a `MultiplicityRange` under a `@@` filter |
| `Unit`, `Dimension` — the reduced unit or dimension a library-style declaration denotes | quantity typing, unit conversion |
| `Annotations`, `About`, `Annotation` — the metadata declared on the element, its type by name and by reference, with its literal values and the span of the node stating it; the elements an `about` usage annotates, and the annotation it states on them | metadata filters on imports, `@`-annotated lookups, the identity-metadata audit |
| `MetadataType` — the resolved type named by a prefix metadata usage | reflective `MetadataFeature::type` and `metaclass` for recorded annotations |
| `Ends` — a connector's owned end features by position, a `connect a to b` end without a symbol of its own holding its place | the ends a specializing connector inherits and redefines by position, its end count against a binary link, its related features |
| `Default` — the values a feature of a metadata definition declares, one per element of a sequence | the value an annotation of that type carries for a feature it leaves unbound, which a filter reads |
| `BaseType`, `ModBindsBaseType` — the base type a metadata definition binds unconditionally | metadata typing |
| `Relationships` — the relationship members (dependency, satisfy, allocate, …) with their resolved targets | relationship queries, the OOSEM and MOSA audits |
| `Node`, `Keyword`, `Notation`, `UsageKind`, `DefKind` — what kind of declaration it was, and the notation that names it in a diagnostic | every reader that used to switch on the declaration's type; the reflective metaclass a `@@` filter classifies the element by (a KerML `struct` is a `Structure`, a transition a `TransitionUsage`, an interface's end a `PortUsage`) |
| `Relationship` — the kind a keyword-first relationship member declares (`specialization S subtype A :> B;`), and whether it is a conjugation | its reflective metaclass (`Specialization`, `Conjugation`) under a `@@` filter |
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
- **Connections and flows as trees.** What a connector attaches its ends to
  is a body fact; what other documents see — the connector symbol, its kind,
  its type, and the position, name and type of each of its ends, which a
  specializing connector inherits — is a registration with facts like any
  other. Where a body-local scope owned them (`Scope.bodyLocal`, annotated
  bodies), the scope is not recorded.
- **Constraint and requirement text.** The constraint's result expression is
  an expression.
- **Comments and documentation.** Documentation is read from the tree by
  hover and the document renderer, which hydrate (the language server's hover
  over a reference into a recorded document hydrates it first); completion
  lists a recorded declaration without its documentation rather than parse
  every closed file a candidate comes from. The record keeps only the
  document's top-level member list (`InterfaceRecord.Members`, a
  `libs.TopMember` per member: its keyword, name, import target and span),
  which is what a session lists of a file it loaded (`✓ package P`,
  `✓ comment`) and what the prompt's scope is found from; a recorded file is
  listed as a loaded one is without being hydrated for it.
- **Anything the resolver memoized.** The record holds facts, not the memo
  tables; a recorded document owns no resolver frame.

## Stable references

A fact that names another element cannot always do so by qualified name: an
anonymous supertype, an implicit end, a redefinition of an unnamed member.
`ElementRef` is the fully-qualified name of the nearest enclosing element that
name alone declares in its document, that document, and one member ordinal per
step down to the element the name does not reach. The document is always
named, not only while the name is ambiguous: a record is read after other
documents have come and gone, and a reference written while `P::T` had one
declaration must still restore that one when a later document declares
another. `Index.RefTo` writes one; `Index.Element` restores it against the
live index, so the reference joins whatever residency its target has. When
the named document no longer declares the name, the reference falls back to
the name alone: a copy of a library file standing in for the bundled one
answers the references the other library files hold into it. The
ordinals count members of scopes a record keeps, so `RefTo` writes no
reference to a member of a scope the record drops — a metadata body, a
control-flow or constraint body — since nothing could restore it. When an
element has no such reference — one declared twice in its own document, or one
of those members — the writer refuses the record (`libs.ErrUnrecordable`)
rather than writing an approximate one, and the document stays loaded.

## What the writer refuses

`WriteInterface` returns `ErrUnrecordable` for a document whose interface it
cannot state without the tree, and the document is held loaded. Today that is:
a supertype, redefinition, alias, annotation, annotation type, relationship, subsetted
multiplicity or base-type target no `ElementRef` reaches; supertypes the semantic model marked provisional; a
quantity-valued annotation or metadata default; and a metadata definition
whose `baseType` is bound conditionally. In the four OMG corpora one document of 313 is refused
(`kerml-examples/Simple Tests/MetadataTest.kerml`, a conditional `baseType`
binding). A refusal is never silent: the error names the fact and the element.

## Cache key

`Cache.InterfaceKey(name, content, libraryIdentity, mode)` is the document's
name and content hash, the identity of the library the workspace's index holds
(`Index.LibraryIdentity`: the language, tier and text of every library
document, whatever name holds it), the conformance mode, the build identifier
and the record format version. A record written under any other library, mode,
build or format is never found; nothing has to be invalidated. A workspace over
an index whose library identity is unknown — a library document states no text
digest, so it may hold anything — has no key and holds every document loaded.
The name is in the key because the record names its document: two files of
equal content are two documents, each with its own record. `interfaceFormatVersion` in `libs/interface.go` moves whenever
`LibraryFacts` or the record structs change shape or meaning.

The record also carries the digest of the content it was written from, and
`Workspace.OpenRecorded` takes that content with the record, refusing other
bytes (`model.ErrRecordMismatch`): the stored diagnostics locate in the text,
so a recorded document holds its text, digest and line index as a loaded one
does, and only its tree, resolver frame and analysis are gone. The workspace
copies the content it is given, so the caller's buffer is its own afterwards.

The record carries the conformance mode its diagnostics answer (`Mode`), and
`OpenRecorded` refuses a record written under another mode than the
workspace's, as the key never finds one. The stored diagnostics cannot be
re-asked under a different mode without the tree, so while a workspace holds a
recorded document `SetConformanceMode` leaves the mode where it is and returns
a `symbols.NeedsHydration` naming that document; the REPL's `%strict` reports
it and the language server shows it to the client. Hydrating the document
lifts the restriction.

The key says nothing about the other documents of the workspace. A record's
references are restored against the live index when read, so a target that
another document renamed or removed is simply not found; but the diagnostics
stored with the record, and facts the analysis derived from its siblings, are
those of the analysis that wrote it. So the record also carries where that
analysis got its answers: its **provenance** (`libs.Provenance`,
`internal/workspace/libs/provenance.go`) is the read set of the analysis —
the names, namespaces, short-name segments and documents its resolver frame
and dependents read (`resolve.Reads`, `Resolver.ReadsOf`), the same
dimensions §3's `symbols.Changes` invalidates a live frame by — and, per read,
the workspace documents that took part in the answer (`Index.Answerers`,
`NamespaceAnswerers`, `SegmentAnswerers`; for a read of shared audit state,
`passes.Contributors`) with their content digests. Library documents are left
out: the key already names the library as a whole. A record installs
(`Provenance.Valid`) only where every read is still answered by the same
documents with the same content; anywhere else the file is parsed in its
place (`model.ErrRecordStale` from `OpenRecorded`; `OpenAll` and the on-disk
paths parse silently). In-process, a change to a sibling that a recorded
document's reads name is judged the same way: if the changed documents still
answer those reads as they did, the record stands, otherwise the document is
hydrated as a live frame would have been dropped (`hydrateStaleLocked`, run
from `invalidateLocked`). A record is thus valid across processes and edits
that do not touch what it read, and never serves a diagnostic its siblings no
longer justify.

## Hydration and demotion

Hydration is an invalidation. `Workspace.Hydrate(name)` parses a recorded
document's text, installs its tree-backed scopes and symbols in place of the
recorded ones, expands imports and runs `invalidateLocked` for the name, so
its dependents drop their entries through `Index.TakeChanges` and
`Resolver.Invalidate` exactly as after an edit; `HydrateAll` does it for
every recorded document at once, parsing on the workers. Every public
operation that needs a body hydrates before answering rather than returning
`NeedsHydration`: opening a document in the editor (`Open`), building a
runtime or debugger (`modelrt.New` and the debugger's private index hydrate
everything recorded, since the runtime's model retains symbols and must never
evaluate against a record), the reverse-reference index (`ReferencesTo`,
`NameReferencesTo`, `RenameConflict`, and so the language server's references
and rename), and the REPL's `%print`, queries and private symbol index.

Demotion is the reverse. `Close` of a document whose buffer equals the file on
disk, whose record the cache holds for that content and whose provenance is
valid, installs the record in place of the tree (`demoteLocked`) and
invalidates the name; a close of a changed buffer holds the disk bytes instead,
as a closed file is held. `SetOnDisk` of a closed file, and `OpenAll` and
`SetOnDiskAll` for each input, take the record for the content when the cache
holds a valid one and parse otherwise (`holdOnDiskLocked`, `cachedRecords`). A
record's reads are answered by its own document too (its identity judgment, its
own names), so both install the record first and check its provenance among the
documents then held, parsing it in place where that does not hold; a file set
from disk alone, before the siblings its analysis read, is parsed, and a later
sibling does not demote it. A batch installs the siblings together, and the
language server's folder scan is one such batch (`SetOnDiskAll`).

Records are written where a document has just been fully analyzed: the batch
path (`DiagnosticsAll`, so `sysml -validate` and `-satisfy` and the REPL's
file load) and the language server's `didSave` (`WriteRecord`). The REPL's typed
transcript is a transient input and is never recorded. A batch records what
each analysis read (`passes.Batch.Record`, a recording resolver per worker)
only when the workspace has a cache to write to. `-no-record-cache` on
either command, or `OPENSYSML_RECORD_CACHE=0`, gives a workspace no cache: it
reads and writes no record, holds every file loaded, and analyzes as it did
before records, recording nothing.

A recorded document is a closed file. Installing its record drops any open
buffer of its name, and the content the record was written from is taken as
what the file holds on disk: a later change to the file (`SetOnDisk`) reindexes
the document from the changed text as for any closed file, and deleting the
file removes it.

## Readers of the tree

A recorded symbol has `Decl == nil`, `Facts != nil` and `Facts.Recorded`. Every
production reader of `Symbol.Decl` under `internal/semantic`, `internal/check`,
`internal/workspace` and `internal/frontend` falls in one of four classes:

1. **Fact readers.** The resolver and semantic model read the fact when the
   symbol is recorded and the tree when it is not — a declaration's kind from
   `Facts.Node`, `DefKind` and `UsageKind` (whether a redefining scope's owner
   is a feature, whether a satisfied element is a viewpoint), its modifiers
   from `Facts.Modifiers` (`variation`); these are the paths the differential
   test exercises for every fixture and corpus document, and
   `TestInterfaceRecordDeclarationReaders` for each such question.
2. **Readers of the document under analysis.** A check pass reads the
   declarations of the document it is analyzing, and a recorded document is
   never analyzed; a reader of a *foreign* symbol in a pass goes through the
   semantic model (class 1).
3. **Body facts that are not externally observable.** A connector's end
   attachments, control nodes, a state's transitions: a reader asked about a
   recorded symbol answers "none", which is what the foreign document could
   observe anyway.
4. **Questions the tree alone answers.** Editing, renaming, printing an
   element's notation, reading the value a feature declares (a document
   query's `Project` over a recorded part's `mass`: the record says the
   feature is valued, `ModValued`, not what its expression is), and building
   a runtime over the workspace return
   `symbols.ErrNeedsHydration` (`symbols.NeedsTree`, `symbols.NeedsHydration`
   with the document and the question) for a recorded document at the level
   that has no tree, and the workspace operation above them hydrates
   (previous section): `Workspace.Detach`, used by `modelrt.New`, hydrates every recorded
   document before its model is built, so the runtime never evaluates against
   a record; the reverse-reference index is built from the references a
   document's body writes, which its record does not carry, so
   `ReferencesTo`, `NameReferencesTo` and `RenameConflict` hydrate the
   recorded documents first and answer as over a loaded workspace. Only a
   record-level question with no operation to hydrate for — a document
   query's `Project` over a recorded value — still reports the
   `NeedsHydration` to its caller.

## What the record costs

`BenchmarkPlaneResidency` in `tests/stressmodel/residency_test.go` holds a
constellation split one file per plane with the planes loaded and with them
recorded, and reports the reachable heap in both states, what installing the
records alone added, and a plane's record size on disk;
`TestPlaneResidencyProcess` holds either state in a process of its own for an
external RSS measurement. `BenchmarkOpenSplit` opens the split constellation
cold and from a warm cache, and `BenchmarkHydratePlane` hydrates one plane
file and re-answers the constellation file that read it. The figures, with
`sysml -validate` over the 10 000- and 1 600-satellite splits with no cache,
a cold one and a warm one, are in `docs/internals/performance.md` and
`docs/project/satellite-network-stress-test.md`.
