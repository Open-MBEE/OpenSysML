# Scaling to very large models: a design

> This is an engineering record, written for review before any of the code it
> describes. It rests on the measurements in the
> [satellite-network stress test](satellite-network-stress-test.md) and on the
> loading figures in [performance](../internals/performance.md); a figure quoted
> here without a source is from one of those two pages, taken on the machine
> they name (`Intel Xeon Platinum 8559C`, 8 CPUs, 31 GiB, Go 1.25, Linux).

A satellite constellation modeled to its components costs the toolchain about
55 µs and 8.5 KB of peak memory per element, and every keystroke in an editor
costs about 16 µs per element in the workspace. Those constants are linear and
small, and they are still the wrong shape: an operational constellation is
7 000–30 000 spacecraft, a model of one at the stress test's fidelity is
1.3–5.6 million elements, and at those sizes a validation takes minutes and
tens of gigabytes while an editor takes seconds per keystroke. Constant-factor
work moves the ceiling by a small multiple. Moving it by two orders of
magnitude means changing *what is held and what is recomputed*, which is what
this page designs.

The technique is the one every engine that renders a world larger than memory
uses: keep only what is being worked on at full detail, keep the rest as a
coarse representation that is exact for the questions asked of it from
outside, and never recompute for the whole world what a local change can only
affect locally. In this toolchain the "world" is the set of documents in a
workspace, "full detail" is a parsed, indexed and analyzed document with a
warm semantic model over it, and "coarse" is the record of a document that
other documents can observe. Two of the pieces already exist for the standard
library and only the standard library; the design below generalizes them and
adds what they lack.

---

## 1. What the cost is made of

The stress test's profiles separate four kinds of cost. They have different
causes and different remedies, and conflating them is how a plan ends up
speeding up the wrong one.

| cost | measured | where it comes from | avoidable? | approach |
| --- | --- | --- | --- | --- |
| **Per element, once.** Parsing, indexing, resolving each name once, checking each declaration once. | ~55 µs, 19 KiB allocated, 2.7 KiB retained per element in a batch validation | `parser`, `symbols.Index.AddDocument`, `resolve`, `passes` | Not in principle: a document has to be read once. Avoidable *per process* when the document has not changed since it was last read (§4). | 2 |
| **Per workspace, per edit.** Re-analyzing everything after any change. | ~16 µs per element in the workspace per keystroke; 1.5 s at 500 satellites | `model.(*Workspace).invalidateLocked` clears every cached diagnostic; every `passes.AnalyzeWithOptions` call builds a fresh `passes.Context` whose `Resolver()` and `Model()` start with empty memo tables; three audit passes gather over `Index.WorkspaceDocuments()` | Yes. A change can only reach the documents that can see the changed one. | 1 |
| **Per process, serial.** One core doing the work of eight. | `-validate` at 800 satellites: 8.2 s wall, 135% CPU on 8 cores | `Workspace.Diagnostics` analyzes documents one at a time; `Context` is single-threaded | Yes. Per-document analysis over a read-only index is independent work. | 3 |
| **Per modeled object, by construction.** One `part def` per spacecraft, each redefining the whole tree. | 187 elements and 1.6 MB of peak memory per satellite | The model, not the tool: 12 800 definitions that differ in a handful of values | Yes, by modeling the fleet as one definition with 12 800 occurrences and a value table — if the runtime can hold 12 800 occurrences cheaply. | 4 |

Two figures bound what is worth attempting. The **retained** cost of a loaded
element is 2.7 KiB; the **peak** cost is 8.5 KB, the difference being garbage
the collector has not yet reclaimed (19 KiB is allocated per element, seven
times what survives). A design that holds fewer elements at full detail
attacks the 2.7 KiB; one that streams attacks the 8.5 KB; one that allocates
less attacks the quarter of every profile spent in `runtime.scanobject`. Of the
2.7 KiB retained, the largest owners in the 1 600-satellite heap profile are
the syntax tree (`parser.(*Parser).parseUsage` and its callees, about 28%),
the symbol and scope objects (`symbols.newSymbol`, `symbols.NewScope`, 17%),
and the semantic side tables (`semantics.(*Model).lookupSources`,
`redefinitionClosure`, `buildMaskFromCandidates`, `resolve.(*Resolver).memoize`
and `resolvedPart`, most of the rest). A document held at less than full
detail has to shed all three, not only the tree.

## 2. Residency: what a document can be held as

Today a workspace document has one state: parsed, indexed, and analyzed from
cold on every request. The design gives a document five, ordered by what they
cost and what they can answer. The standard library already occupies two of
them, which is the evidence that the levels are implementable in this
codebase rather than a proposal from first principles.

| level | holds | can answer | cost | exists today for |
| --- | --- | --- | --- | --- |
| **absent** | the name and content hash | nothing | 0 | — |
| **recorded** | the document's *interface record*: the names it exports, each with its kind, supertypes, features (type, direction, multiplicity), unit facts, and the document's last diagnostics | a reference *into* it from another document; its diagnostics, as last computed | hundreds of bytes per exported feature | the standard library's `libs.IndexRecord`, persisted by `libs.Cache` keyed by content hash and format version |
| **frozen** | the full parsed tree, scopes and symbols, decoded from a snapshot rather than parsed, read-only | everything a loaded document can, except being edited | full retained memory; a fraction of the load time (`stdlib.snapshot`: 7.8 ms to decode against 50–70 ms to parse and expand) | the standard library, `libs.SharedLibrary`, frozen and shared by every workspace through `symbols.NewOverlay` |
| **loaded** | the parsed tree, indexed, with diagnostics computed | everything, from a cold semantic model | full memory, full per-element time | every workspace document |
| **live** | loaded, plus a warm resolver and semantic model that survive edits to other documents | everything, at the cost of what changed | full memory plus memo tables | nothing: the semantic model is rebuilt on every request |

The levels are exact, not approximate. A recorded document answers a
cross-document reference with the same symbol facts the full tree would, and
reports the diagnostics a full analysis produced. What a record cannot answer
— a query that descends into a body, an instantiation of a type it declares, a
`-satisfy` over its requirements, opening it in an editor — *hydrates* the
document to loaded, it does not answer from the record. There is no level at
which the toolchain guesses.

Two axes come out of the table and the four approaches below each move a
document along one of them:

- **Residency** (§4): how many documents are loaded rather than recorded.
  This is what bounds memory, and it is the only thing that gets a workspace
  of 10 000 spacecraft into a laptop.
- **Recomputation** (§3, §5): how much of the loaded set is re-analyzed per
  change and on how many cores. This is what bounds latency, and it is what
  makes a workspace of any size editable.

The fourth approach (§6) does not move a document along either axis; it
shrinks the model.

## 3. Approach 1 — a persistent semantic model, invalidated per document

**What it removes:** the per-workspace, per-edit cost. Target: editing a small
file beside a 512-satellite constellation goes from 1.53 s per keystroke
(`BenchmarkEditBeside`) to the cost of the small file, tens of milliseconds;
editing a file *inside* a large model costs that file and the files that
import it.

### 3.1 Why every keystroke is a whole-workspace walk

Three separate mechanisms each make the cost proportional to the workspace,
and removing one without the others leaves the constant nearly unchanged:

1. `model.(*Workspace).invalidateLocked` clears the whole diagnostic cache
   and the reverse-reference index on any change. Its comment says why:
   "a change anywhere can alter what a name elsewhere resolves to". That is
   true of *some* change and *some* elsewhere, and the cache discards all of
   them.
2. `passes.AnalyzeWithOptions` builds a `passes.Context` per document per
   request, and `Context.Resolver()` and `Context.Model()` construct a new
   `resolve.Resolver` and `semantics.Model` for it. Every diagnostics request
   after an edit re-resolves every name and recomputes every specialization
   closure the passes touch, from empty tables. The 128-satellite edit
   profile spends 16% of its time resolving the two-line file's own imports —
   through a cold resolver, over a warm-in-principle index.
3. Three audit passes (`OOSEMMethodPass`, the MOSA audit, the identity
   metadata pass) and the semantic model's coherent-quantity ranking walk
   `Index.WorkspaceDocuments()`, gathering from every document to analyze
   one. The OOSEM audit alone is 24% of that profile. They do this because
   their rule genuinely spans documents — a derivation relationship can
   cross files — but they recompute the gathered facts from scratch per
   analyzed document per request.

The LSP server compounds all three: `refreshOpenDiagnostics` re-analyzes
every open document after a change settles, so `n` open files cost `n` times
the figures.

### 3.2 Design

**One resolver and one semantic model per workspace**, owned by `Workspace`
beside the index and handed to every `passes.Context` the workspace builds,
instead of one pair per `AnalyzeWithOptions` call. Both are already memo
tables keyed by `ast.Node` and `*symbols.Symbol`; the change is their
lifetime, not their shape. The REPL already works this way one level
up: a submission supersedes only the earlier declarations whose names it
redeclares (`internal/frontend/repl/session.go`, `acceptFrom`), a debugging session
ends only when the behavior it steps or the object it runs over is among
them (`dropStaleDebugSessions`), and object identities survive the rebuilt
runtime context (`keepIdentitiesOf`) — so the repository has a precedent for
keeping derived state across changes and invalidating by what a change
reached.

**A dependency relation over documents**, maintained by the index and the
resolver, that says which documents a change to document *D* can reach. It
is sound (never misses a dependent) and coarse (document-granular). *E*
depends on *D* when any of the following holds:

- *E* imports a namespace *D* contributes to — the index already records
  each document's import targets (`lastTargets`) and each namespace's
  contributing documents, so this is a lookup, not a computation;
- *E* contributes to a namespace *D* contributes to — a package spans
  documents when two files both open `package P`, and a declaration added
  in one shadows or conflicts with names in the other;
- *E* resolved a name into *D* — recorded by the resolver as it memoizes: the
  owning document of every symbol a resolution returns is known from its
  scope's root (`Index.DocumentOfRoot`), and the resolving document from the
  scope the resolution ran in. This is what makes qualified references that
  bypass imports (`SatelliteNetwork::Constellation::Sat0`) reach the
  relation.

The relation is transitive: a change to *D* invalidates *D*, everything that
depends on *D*, and so on. A satellite file that neither imports the
operations file nor is imported by it is unreachable from an edit there and
keeps its diagnostics, its resolutions and its semantic facts.

**Invalidation by owning document.** On `reindexLocked(D)`:

- memo entries whose key node or key symbol belongs to *D*'s previous tree
  are dropped — the tree is being replaced, so the keys are unreachable
  anyway, and dropping them lets the old tree be collected;
- memo entries owned by any document in the dependent set of *D* are
  dropped, because their results may have been computed through *D*'s old
  symbols;
- the diagnostic cache and the reverse-reference index are cleared for *D*
  and its dependents only.

Ownership tagging is the mechanism the resolver already has for a different
lifetime: `Resolver.Scratch` journals what a request expression's resolution
memoized and drops it when the request ends (`journalNew`, `Journal`). A
per-document journal is the same bookkeeping with the document as the frame,
and side tables that join the resolver's lifecycle through `Journal` (the
semantic model's, the argument typer's) join this one without a second
mechanism.

**Workspace-wide passes over cached per-document facts.** Each audit pass
that gathers over the workspace splits into a gather step, run once per
document and cached in the workspace beside the diagnostics, and a judge
step, run per analyzed document over the union of the cached gathers. A
change to *D* recomputes *D*'s gather alone. The gathers are small — the kind
and relationships of the symbols a method's rules name — and a workspace
that declares no artefact of a method's kinds has empty gathers and pays
nothing for that audit, which is the stress test's first noted follow-up.
The coherent-quantity ranking in `semantics` ranks documents' unit
declarations and is the same shape.

**The LSP refresh sweep** publishes for every open document, but the
documents outside the changed one's dependent set hit the cache, so the
sweep costs one analysis rather than `n`.

### 3.3 What must stay true

- *Incremental equals fresh.* For any sequence of opens, edits and closes,
  the diagnostics, resolutions and references a workspace reports equal
  those of a fresh workspace built from the same final documents. This is
  the correctness contract and it is a test: replay recorded edit sequences
  over the fixture and corpus models, and a randomized replay that edits a
  random document to a random earlier version of itself, and compare against
  a from-scratch workspace after every step. The soundness of the dependency
  relation is what this test checks; a missed edge shows as a diagnostic
  that a fresh workspace reports and the incremental one does not.
- *Conformance mode is a cache key.* `SetConformanceMode` changes the
  question every diagnostic answers; it invalidates everything, as today.
- *The REPL's debugger contract holds.* An action or state debugging session
  over a declaration a submission left alone keeps running. A persistent
  model that invalidates by document must not invalidate the REPL's
  transcript document on every submission; the REPL's declaration-granular
  supersession (`acceptFrom`) is finer than document-granular, and the two
  granularities have to be reconciled — the REPL treating each
  top-level submission as its own document is the simplest reconciliation.
- *The architecture's invariants are unchanged.* The tree stays immutable,
  derived state stays in side tables, resolution stays lazy and memoized;
  this approach is more memoization with a longer lifetime, not a new place
  for semantic state.

### 3.4 Cost, payoff, risk

One to two focused pull requests: the dependency relation and per-document
invalidation in `symbols`/`resolve`/`model`; the audit passes' gather cache
in `passes`. The measurable payoff is `BenchmarkEditBeside` at 512 satellites
from 1.53 s to under 50 ms, plus a new benchmark that edits a file *imported*
by the constellation, which is the worst case and should cost one analysis of
the constellation, not two. The risk is soundness of the relation — a missed
edge is a stale diagnostic — which is why the differential replay test is
written first, and why the relation is document-granular rather than
declaration-granular: coarse and sound is the right first step, and the
replay test is what would license refining it later.

## 4. Approach 2 — closed documents held as interface records

**What it removes:** the per-element cost of documents nobody is editing or
querying. Target: a workspace of 10 000 spacecraft, split over files, opens
in the time it takes to read the records of its unchanged files and to load
the files in the editor, and holds the record cost — not 1.6 MB per
spacecraft — for everything else.

### 4.1 What already exists, and what it is not

Two artefacts of the standard library are the two halves of this approach,
and neither is the whole of it:

- `stdlib.snapshot` is the **frozen** level: the library's parsed trees,
  scopes, symbols and expanded imports serialized by `symbols.WriteSnapshot`
  and decoded at start-up, 3.6 MB for 1.8 MB of source, decoded in a fraction of
  the time parsing takes. It holds *everything*; it saves time, not memory. Generalized
  to any document whose content hash matches a cached snapshot, it makes a
  reopened workspace load in a fraction of the time. It does not let a workspace
  hold more than fits.
- `libs.IndexRecord` is the **recorded** level: per document, each declared
  element's qualified name, its semantic supertypes, whether it is abstract,
  and its unit and dimension facts — the facts the library's readers need
  from it — persisted by `libs.Cache` under a content-hash key with a format
  version. It is small. It is also, at present, lossy for anything beyond
  those facts, and the design record for
  [making library records lossless](lossless-library-records.md) is the
  cautionary tale: a record that drops
  something a reader needs produces divergent diagnostics, and finding out
  which facts readers need by observing divergence is slow.

### 4.2 Design

**The interface record is defined by visibility, not by size.** A document's
record holds exactly what another document can observe of it through the
language: every element reachable by a qualified name or an import from
outside the document, with the facts a reference to it needs — kind,
name and aliases, supertypes and the specialization kind, features with
their type, direction, multiplicity, redefinitions and subsetting,
whether abstract or variation, unit and dimension facts, metadata that
imports filter on. It excludes what only the document's own body uses:
behavior bodies, connections and flows, expressions, constraint and
requirement text, private members, comments and documentation. For the
stress test's spacecraft, whose components and ports are public members of
its `part def`, the record keeps the component tree's shape and types and
drops its connections, its expressions and its state machine; the saving per
spacecraft is a measurement to take before building further, and the honest
prior is a factor of 10–50 on retained memory rather than a thousand,
because a `part def` exposes most of what it declares.

**A recorded document is indexed as symbols without a tree.** The index holds
its scopes and symbols with `Decl` unset and the record's facts attached, the
way library facts already ride on symbols (`symbols.LibraryFacts`); the
resolver resolves into them as into any symbol, and the semantic model reads
their supertypes and features from the facts rather than from a declaration.
The passes never analyze a recorded document; its diagnostics are the ones
stored with the record, computed by the last full analysis, and reported
verbatim. The reverse-reference index does not cover it, so a rename that
reaches into a recorded document hydrates it first.

**Hydration is an invalidation.** Opening a recorded document, or asking a
question that needs its body, parses it, replaces its record symbols in the
index with tree-backed ones, and invalidates its dependents through the
relation of §3 — because the identity of its symbols changed, and every memo
entry that reached one of them is stale. Without §3 this would be a
whole-workspace invalidation on every file opened, which is why §3 comes
first. Closing a document that is clean on disk demotes it back to its
record.

**The record is written when the full analysis is.** A `-validate` in CI, a
save in the editor, a REPL load: each has just analyzed the document and
writes its record and diagnostics under the content hash, the format version,
the library digest and the conformance mode. The next process that sees the
same bytes loads the record. A workspace that opens with no cache is a
workspace as today, one file at a time.

**Correctness is a differential test, per corpus file, before any consumer
is switched.** For every document in the fixtures and the four OMG corpora:
analyze every *other* document with this one recorded, and with it loaded;
the diagnostics must be identical. This is the test the lossless-records
work needed and did not have at the start, and it is what turns "what does a
reader need" from an empirical question into an assertion the gate holds.

### 4.3 What the approach requires of models

A record is per document, so the level of detail a workspace can vary is
per document. A constellation written as one 147 MB file is one document
and is held whole or not at all. Real projects are already split — one
package per subsystem, one file per package — and a constellation would be
split by plane or by spacecraft class; the guide should say so, with the
import graph as the reason. The CLI has to follow: `sysml -validate` over
many files currently reindexes after each, which is quadratic in the file
count (noted in the performance page's further work), and a batch of files
has to be added to the index once and expanded once.

### 4.4 Cost, payoff, risk

Two to three pull requests after §3: the record format and its writer over
`libs.IndexRecord` (`symbols.LibraryFacts` is the attachment point);
tree-less symbols in the index and the resolver and semantic model reading
facts from them; hydration and demotion in `Workspace`; the cache key and
the write points. The measurable payoff is retained memory per spacecraft at
record level against 490 KiB loaded, and the open time of a 10 000-spacecraft
workspace from a warm cache. The risk is the record dropping a fact a reader
needs, which the differential test converts from a runtime surprise into a
build failure; the second risk is symbol identity across hydration, which §3's
invalidation handles if and only if every consumer that retains a `*Symbol`
across requests joins the invalidation — the runtime's `Model` is the largest
such consumer and is rebuilt today on every REPL submission, which is the
safe default to keep.

## 5. Approach 3 — parallel and streaming batch validation

**What it removes:** the serial per-process cost. Target: `-validate` over
1 600 satellites from 19 s to about 5 s on 8 cores; peak memory bounded by
the index plus the documents in flight rather than by every tree.

### 5.1 Design

**Parse in parallel, index once, analyze in parallel.** The library loader
already hashes and parses its files on `GOMAXPROCS` goroutines and adds them
to the index serially (`Loader.LoadAll`); the workspace does the same for
its documents. Indexing stays single-writer, and `ExpandWildcardImports`
runs once after the batch, which also retires the quadratic per-file
reindex. Analysis is then independent per document over a read-only index:
a pool of workers, each with its own `passes.Context`, and the diagnostics
gathered in document order so the output is deterministic. The resolver and
semantic model are not safe for concurrent use and are not made so; each
worker's are private, at the cost of resolving shared names once per worker
rather than once — bounded by the worker count, and the measurement that
decides whether a shared, locked resolver is worth its contention.

**Stream when nothing needs the body.** Once a document is analyzed, the only
reason to keep its tree is that a later document may resolve into it. With
records (§4) that reason is gone: the analyzed document demotes to its
record and its tree is collected, so peak memory is the index's records
plus the trees in flight. Without records, streaming can drop only what the
index does not point at — the parse diagnostics and the source text, a small
fraction — so the memory payoff of this approach is contingent on §4.

**Allocate less.** Nineteen KiB allocated per element for 2.7 KiB retained
puts the collector at a quarter of every profile. The parser's per-token and
per-name allocations and the resolver's per-lookup slices are the largest
sources; the snapshot decoder's node-table allocation (one block per node
type) is the pattern that reduces them. This is ordinary profiling work and
is listed here because it is the part of batch cost that parallelism does
not touch: the collector's marking is proportional to the live heap, however
many cores are analyzing.

### 5.2 Cost, payoff, risk

One pull request for the pipeline, with the allocation work as follow-ups
measured one at a time. The payoff is a factor of four to six on wall time
for every batch run over a multi-file model; a single-file model gains only
from the allocation work, since one document is one unit of parallel
analysis — a second reason to split large models across files. The risk is
low: the pipeline changes no semantics, and its test is that the diagnostics
of a parallel run equal a serial one for every corpus.

## 6. Approach 4 — one definition, many occurrences

**What it removes:** elements the model declares that carry no information.
Target: a 12 800-spacecraft constellation at the stress test's fidelity is a
few thousand elements rather than 2.4 million, and the cost moves to the
runtime, where 12 800 occurrences of one definition share one shape.

### 6.1 The observation

The stress test's model declares `part def Sat0 :> Spacecraft` through
`Sat12799`, each redefining every component beneath it to set a serial
number and an as-built mass. That is a faithful rendering of how a
constellation is *documented* and an unfaithful rendering of how it is
*engineered*: a fleet is one bus design, or a few, with per-unit values
that live in a table. In SysML the fleet is `part sats : Spacecraft[12800]`
— one definition, one usage, a multiplicity — and the per-unit values are
either a value table the model binds to (the roadmap's bindings track,
from modeled elements to external data) or a `part def` per variant with an
occurrence count. Written that way, the definition side of the model is the
size of one spacecraft and the ground segment, whatever the fleet size, and
approaches 1–3 apply to it comfortably.

### 6.2 What the runtime has to do

The cost does not disappear; it moves to instantiation, which today
materializes an object per element with a feature value per feature. Three
things make 12 800 occurrences cheap, and the runtime has the first:

- **Shared shape.** `Context.FeaturesOf` computes the effective feature list
  once per type and caches it in `runtime.Model.features`, so every object
  of a type shares the list. This exists.
- **Sparse values.** An object whose every feature holds its declared default
  needs no per-object storage until a value diverges: the default is a
  property of the type. An occurrence table then costs one object header per
  occurrence plus the values that differ from the type's defaults — the
  serial number and the as-built mass — rather than a value slot for each of
  the 150-odd features of a spacecraft. Feature values are already lazy
  (`materialize.go` reads them on demand); making the unread default share
  storage with the type is the change.
- **Verification over distinct shapes.** A `satisfy` over 12 800 occurrences
  whose constraints read only type-level defaults has one distinct
  evaluation; one that reads a per-occurrence value has as many as there
  are distinct values. Evaluating per distinct input rather than per
  occurrence, with the verdict fanned out, is what keeps `-satisfy` from
  costing 12 800 times one spacecraft. The element budget
  (`runtime.DefaultMaxElements`, one million collection elements) is the
  bound this has to be designed under, not raised to escape.

### 6.3 Cost, payoff, risk

Guidance first: a page in the guide on modeling fleets and repeated
structure, with the stress test's constellation rewritten both ways and the
element counts beside each. That costs nothing and it is the largest single
change available to a modeler today. Runtime support — sparse values and
distinct-shape verification — is one to two pull requests, measured by
instantiating and checking the rewritten constellation at 12 800
occurrences. The risk is semantic: a value read through a sparse default has
to be indistinguishable from one read through a materialized slot,
including when the default is an expression over other features, and the
execution conformance suite is the gate that holds that.

## 7. What none of the approaches may do

- **Answer approximately.** Every level of detail is exact for the questions
  it answers and hydrates for the ones it cannot. There is no mode in which
  a diagnostic, a resolution, a verdict or a value is computed from less than
  the model says.
- **Change the results.** Every approach has a differential test against the
  unoptimized path — incremental against fresh, recorded against loaded,
  parallel against serial, sparse against materialized — over the fixtures
  and the four OMG corpora, and the corpus gates hold: the training corpus
  stays clean and the three ratchets do not move.
- **Weaken the invariants.** The tree stays immutable; semantic state stays
  in side tables; resolution stays lazy and memoized; the runtime consumes
  lowered IR; the parser never panics. Approaches 1 and 2 add lifetimes and
  levels to existing side tables, they do not add a second place for
  semantic state.
- **Depend on file layout for correctness.** A model in one file and the
  same model in ten produce the same diagnostics and the same values. File
  layout decides what the approaches can *save*, never what they report.

## 8. Sequence

| order | approach | what it buys | depends on | size |
| --- | --- | --- | --- | --- |
| 1 | §3 persistent semantic model, per-document invalidation | editing beside any model that fits in memory: keystroke cost is the file's, not the workspace's | — | 1–2 pull requests |
| 2 | §5 parallel batch validation | `-validate`/`-satisfy` wall time ÷ 4–6 on a multi-file model; the multi-file CLI path stops being quadratic | — | 1 pull request, plus allocation follow-ups |
| 3 | §4 interface records for closed documents | a workspace larger than memory; reopening in a fraction of the time | §3 (hydration is an invalidation); §5's batch path is where records are written | 2–3 pull requests |
| 4 | §6 one definition, many occurrences | the model itself shrinks by the fleet size; the runtime holds the fleet | guidance: none; runtime: none, but §3 makes the REPL that drives it usable | guide page now; 1–2 pull requests for the runtime |

§3 goes first because it is the cheapest, it is the largest interactive
gain, and §4 cannot be built without it. §5 goes second because it is
independent, low-risk, and its batch path is where §4's records get written.
§4 is the one that reaches the stated goal — tens of thousands of spacecraft
in one workspace — and it is third because it is the one whose correctness
rests on the other two. §6's guidance is written whenever; its runtime work
is measured against the rewritten constellation once §3 makes a REPL over it
comfortable.

## 9. The measurements that decide

Each approach names the figure that says whether it worked, taken on the
stress test's generator so it compares with the record there:

- **§3:** `BenchmarkEditBeside` at 512 satellites, 1.53 s → under 50 ms; a new
  benchmark editing a file the constellation imports, expected to cost one
  analysis of the constellation. Memory held by the persistent model after a
  thousand edits, to show the journals release what they should.
- **§5:** `-validate` over the 1 600-satellite constellation split by plane,
  19 s → about 5 s at 8 cores; CPU utilization from 135% toward 700%; peak
  RSS unchanged without records, then bounded with them.
- **§4:** retained bytes per spacecraft at record level against 490 KiB
  loaded; open time of a 10 000-spacecraft, 400-file workspace from a warm
  cache; hydration time of one file with its dependents' invalidation.
- **§6:** the constellation rewritten as `Spacecraft[12800]` with a value
  table: element count, `-validate` time, instantiation time and memory,
  `-satisfy` time against the 3 200-satellite figures the record holds today
  (83 s, 7.6 GB).
