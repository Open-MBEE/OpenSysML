# Making library records lossless (roadmap L3): the design

> This is an engineering record. `L3-<n>` names a row of the table in §7, and `F<n>` a row of the
> divergence census in [pilot-differential.md](pilot-differential.md).
>
> **Oracle figures in this record are as measured when it was written; they are not the current
> baseline.** The current baseline is the generated block in [README](../../README.md) and
> [architecture](../internals/architecture.md), regenerated and gated by `make docs-counts`.

Roadmap item **L3** asks for library records to become *lossless*: a restored library must equal a
parsed one, so a library keeps its members, declared values and condition bodies on both load paths.
This page is the design, written for review **before** any of the format changes, because the
measurements below moved the plan twice.

First: the cheapest lossless design turns out not to serialize member, value and condition structure
at all (§2, §4). Second, and the reason the item's order changed: the memory cost of keeping library
bodies was priced against the wrong population. It is not `DefaultIndexPoolSize = 4` library indexes
but **one per cached model** — 100 at the gRPC default, measured at 1.56 GiB of heap in §3. So the
**first stage of L3 is sharing one library index across models**, and the record format follows it,
where the cost of keeping bodies is a one-time ~17 MiB rather than ~17 MiB × 100.

Nothing in this page changes behaviour. The three oracles are reproduced on the base commit as
controls, and they are unchanged because no code moved.

| Oracle | `main` at `0d4eb14f`, fresh cache | This branch |
|---|---|---|
| Xpect | 428 `.xt` files, 0 unparsed; 1269 agree (246 wording-only), 54 disagree | identical |
| Differential | 353 files, 317 fully agreeing; 25 agreed, 119 only ours, 73 only the pilot's | identical |
| Rejection | 120 cases: 116 both reject, 4 only the pilot rejects, 0 only we reject | identical |

Reported as a multiset rather than a total, the 119 only-ours differential rows are, before and after,
`syntax`/warning 63, `unresolved-reference`/error 33, `unmapped`/error 15, `unmapped`/warning 5,
`kind-mismatch`/error 1, `multiplicity`/error 1, `units`/warning 1 — equal to
`pilot-differential-baseline.json` category for category, so no only-ours row was added.

Every number on this page was measured on this machine, on `0d4eb14f`, with a fresh cache
(`XDG_CACHE_HOME` pointed at an empty directory per run).

---

## 1. What is being replaced, and what the replacement owes

A standard library is **index-only**: `libs.Loader.LoadAll` parses what the cache missed and then
`reduce` replaces every parsed document with the same `IndexRecord` a hit restores
(`internal/core/libs/loader.go`). A library symbol therefore carries a name, a kind, a span,
specialization and `featured by` targets, an alias target, unit/dimension facts, behavior parameter
lists, annotation facts and compiled filter predicates — and **no `Decl`**.

That contract exists because the cache was previously observable: a hit produced a poorer state than
a miss, so `solve` evaluated library-inherited invariants and gRPC reported dozens of inherited
library attributes only until the cache warmed, and the conformance oracles gave two answers for one
tree. `internal/core/libs/index_only_test.go` is the proof that the cache is now free of semantic
effect, and it proves it the strong way: **no symbol of a library document has a `Decl` on either
path**, including with no cache at all.

Any replacement owes the same proof in the same enforceable form. Being lossless is not enough on its
own; the property to preserve is that **the presence, age or absence of a cache entry cannot change
one query's answer**. Concretely, the successor test must assert:

1. **Path equality.** For every symbol of every library document, every fact any consumer can read —
   kind, span, supertypes, members, multiplicity, declared value, condition body, annotations,
   filters — is equal cold and warm.
2. **No-cache equality.** The same holds with `cache == nil`, so disabling persistence is not a way
   to see a different library.
3. **Staleness is a miss.** A record produced by different code, a different file, or a different
   sibling file is never restored. Build identity (`libs.buildID`) and the set digest
   (`Loader.setDigest`) already do this and stay.

---

## 2. What "lossless" can mean, measured

Three designs satisfy "a restored library equals a parsed one". They differ in *what is persisted*,
and the measurements decide between them.

The load costs of the bundled library (95 files, 1.35 MiB of source, 19288 registered FQNs, 10010
distinct declaring symbols):

| Operation | Time | Heap held | On disk |
|---|---|---|---|
| Parse all 95 files, discard the trees | 41 ms | — | — |
| Parse **and index** all 95 files as ordinary documents, wildcard imports expanded | 83–90 ms | 32.7 MiB | — |
| Today's cold load (parse → derive facts → replace with records) | 1.90–1.94 s | 15.7 MiB | 1.6 MiB of records |
| Today's warm load (restore 95 records) | 47 ms | 15.7 MiB | 1.6 MiB of records |

The decisive number is the first one: **parsing the whole standard library costs 41 ms**. Parsing is
not what the cache buys.

Where the rest of the cold load goes, measured over three runs on one shared `semantics.Model` — the
shape the loader itself uses:

| Component of the 1.90 s cold load | Time |
|---|---|
| Parse, index and expand wildcard imports | 83–90 ms |
| **Every fact family** (`DirectSupertypes`, `AllSupertypes`, unit reduction, dimension, annotation and behavior facts) over all 10010 symbols | 497–507 ms |
| Residual: record construction, filter compilation, gob encoding, `AddRecords` and the re-expansion that follows it | 1.32–1.34 s |

So the derivation the cache exists to memoize is about **0.5 s**, and roughly **1.33 s of the cold
load is the record machinery itself** — work that exists only because records exist. Measured, not
attributed further: the residual is a subtraction, and apportioning it between encoding, filter
compilation and the second index rebuild is a task for whoever implements step 2.

Within that 0.5 s the memo is live rather than missed, which the reviewer was right to ask about:

| | First pass, fresh model | Second pass, same model |
|---|---|---|
| `DirectSupertypes` over 10010 symbols | 360 ms | 0.13 ms |
| `AllSupertypes` over 10010 symbols | 6.5 ms | 0.13 ms |
| `IsMeasurementUnit` over 10010 symbols | 1.9 ms | 0.57 ms |
| `UnitTermOf` over the 1277 units | 98 ms | 0.55 ms |

An earlier draft of this page reported unit reduction (480 ms) and `DirectSupertypes` (354 ms) as two
costs summing to 834 ms. That double-counted: on a fresh model `IsMeasurementUnit` alone costs 361 ms,
but after `AllSupertypes` has been walked it costs **1.9 ms** — the 480 ms was the supertype closure
being built inside the unit check, not unit work. The one-time cost is therefore ~365 ms of
specialization-graph construction plus ~98 ms of genuine unit reduction, and a memo hit is a map
lookup. The per-symbol first-derivation cost, ~36 µs, is dominated by name resolution of each
generalization target, not by re-derivation — but it is still 36 µs for one edge lookup, which is
why it is carried as its own row (**L3-7**) rather than left inside this argument.

### Option A — persist member, value and condition structure

The reading L3 was written with: grow `symRecord` with members, multiplicity, declared values and
condition bodies, and teach the consumers to read them.

This is the most expensive option and the least provable. A declared value or a condition body is an
expression tree; persisting it "structurally" means either persisting the tree (which is option B) or
inventing a second representation of expressions that the evaluator, the solver translation and hover
all have to understand *in addition to* `ast`. Every semantic rule then has two implementations — one
over `Decl`, one over the record — and path equality becomes a claim about two hand-written
implementations agreeing, re-argued for every rule added afterwards. It also contradicts
`AGENTS.md` §4 (semantics derive from the immutable AST; no parallel structures that can drift).

**Rejected**, on cost and on provability.

### Option B — persist the AST

Serialize the parse tree, restore it, `AddDocument` it. Equality is structural and provable by
round-tripping (`ast.Dump` is already a canonical rendering, and the dumps of the bundled library
total 4.1 MiB).

Two concrete obstacles, both verified in the tree rather than assumed:

- `ast.NodeBase` holds `leading`/`trailing` trivia in **unexported** fields, which `encoding/gob`
  silently drops. Doc-comment hover — one of the two reasons L3 exists — would be lost exactly where
  it is wanted, and lost *quietly*. Fixing it means exporting trivia or hand-writing
  `GobEncode`/`GobDecode` for every node.
- The tree is interface-valued (`Members []Node`, expression operands), so gob needs a
  `gob.Register` for each of the ~83 concrete node types in `internal/core/ast`, and a node type
  added later without a registration is a decode error — i.e. a silent miss at best.

And the payoff is inverted: this pays a large new on-disk format, a real version-compatibility
surface and ~83 registrations to avoid a **41 ms** parse, while still needing the ~0.5 s of fact
derivation. **Rejected**, on cost/benefit.

### Option C — parse always; cache only the derived facts (recommended)

Stop treating the record as a *substitute* for the document and treat it as a *memo* of what the
document's analysis derives:

- `Loader.load` always parses and always `AddDocument`s. The library keeps its bodies on every path
  because they were parsed on every path — there is no restored-vs-parsed distinction left to be
  lossy about.
- The cache holds exactly the facts whose derivation is expensive: unit reduction, dimension,
  direct supertypes, `featured by` targets, behavior parameter lists, annotation facts, compiled
  filter predicates. On a hit these are installed as a pre-filled memo of `semantics.Model`; on a
  miss they are derived as today and stored.
- `AddRecords`, `symbols.RecordEntry`, and the `Decl`-less symbol shape they create are **deleted**.
  Nothing in the system then has a second, poorer notion of what a library symbol is.

This satisfies losslessness by construction, and it makes the cache's freedom from semantic effect a
*narrower* claim than today's: instead of "the record must reproduce every consumer-visible fact", it
is "a restored fact equals the fact the model would have derived", which is decidable per fact family
and is exactly what §5 states as a test.

Costs, measured, not speculative:

- **Warm load 47 ms → ~90 ms plus fact restore** (the parse-and-index floor). The cache still saves
  the ~0.5 s of derivation, and the ~1.33 s residual of the cold path goes away with the record
  machinery that causes it, so cold load should improve too.
- **Heap 15.7 MiB → 32.7 MiB per index holding the library** (+17 MiB), because the trees stay
  reachable. An earlier draft of this page priced that against `DefaultIndexPoolSize = 4` and put it
  at ~68 MiB. That was wrong: the pool is prewarm headroom, not the population, and the live count is
  one index per cached model — measured at 100 in §3, i.e. **+1.7 GiB, not +68 MiB**. §3 is therefore
  the stage that comes first; after it the +17 MiB is paid once per process (open row **L3-4**).
- **The on-disk format does not grow.** It shrinks: names, kinds, spans, short names, alias targets
  and wildcard-import targets stop being persisted, because the parsed document states them.

---

## 3. The population, and the shared-library-index design

### 3.1 The population, measured

The reviewer's reading is right, and it is worse than the +68 MiB the earlier draft priced.
`Service.ParseFile` takes an index from the pool and adds the user document to it, so the model that
took it owns it for its lifetime (`internal/grpc/service.go:229`, `CachedModel.Index`). The live count
is therefore bounded by the model cache, which `cmd/sysml-grpc/main.go` defaults to
`--cache-size 100`.

Measured by parsing 100 distinct documents through a `Service` with a 100-model cache, counting the
distinct `*symbols.Index` values the cache still holds, twice in one process:

| Consumer | Library-bearing indexes retained | Cost, measured |
|---|---|---|
| gRPC, `--cache-size 100` full | **100** (one per cached model) | heap 1595.5 MiB, RSS 2022–2043 MiB; 1594.6 MiB over the prewarmed baseline, **15.95 MiB per index** |
| LSP | **1** — one `model.NewWorkspace()` per process (`cmd/sysml-lsp/main.go:112`), one index in it | 15.4 MiB |
| REPL | **2** — the workspace's, plus the one `Session.symbolIndex` builds with `model.LoadStdlibInto` (`internal/repl/session.go:998`, `discover.go:25`) | 30.7 MiB after one submission (15.3 + 15.4) |

**Result of stage A ([#504](https://github.com/JPL-Devin/OpenSysML/pull/504)), measured the same way
on `1af78d94`:** the same 100 cached models cost **1.1 MiB of heap and 76.5 MiB RSS** (0.01 MiB per
model) against 1598.3 MiB / 2180.3 MiB before, and **1** library index is built rather than 104. The
REPL's two indexes (row **L3-10**) become two overlays of one base: 4 sessions with a submission and
a browse index cost 17.1 MiB, against 122.7 MiB. The LSP's single index is unchanged in size and now
shares the process-wide base. Rows **L3-4** and **L3-10** are closed by that stage; **L3-9** kept
its per-operation churn until the stage recorded in its row below.

**The pool's `Built` counter does not track one build per cached model.** Over the 100-model run it
reached 104 with `Pooled: 53, Inline: 47`: 53 requests took a prewarmed index, 47 built one inline
because the pool was empty, and the 4 extra builds are the prewarm slots refilled in the background.
`Built` counts *pool and inline construction*, not retention — the pool caps nothing, since every
index it hands out leaves it for good.

One further build site, not retained but not free either: `ApplyEdits` gave `edit.reparseModel` a
fresh library index per *operation* (`internal/grpc/edit.go:43`, and `reparseModel` is called inside
the operation loop, `internal/core/edit/edit.go:145`). Measured, a 5-operation request took **6**
indexes from the pool — ~96 MiB of allocation and ~0.5 s of index building for one request (row
**L3-9**, now closed: an `Apply` call takes one).

### 3.2 Why the copy exists

The library is immutable once loaded and independent of the model, so nothing about the *library*
requires a copy. What requires it is that adding the user document mutates the index it is added to:
`addDocument` registers the document's names in index-wide maps (`fqn`, `children`, `contributions`,
`declaredAt`, `docRoots`), records the wildcard imports it states (`wildcardMeta`) and its element
filters (`nsFilters`), and `ExpandWildcardImports` then registers re-exports index-wide
(`reexported`, `hidden`, `reexportDocs`, `docReexports`) and tracks expansion state (`dirtyNS`,
`lastTargets`). `Index` has 15 such maps, read from ~123 non-test field accesses across `index.go`,
`filter.go` and `records.go`.

### 3.3 The design: a frozen base index, a per-model overlay

```go
// NewOverlay returns an index whose reads fall through to a frozen base, and
// whose writes are its own: one library index serves every model.
func NewOverlay(base *Index) *Index

// Freeze marks an index read-only, so a shared base cannot be mutated.
func (idx *Index) Freeze()
```

The overlay *is* a `*symbols.Index` — same type, same exported API — so no consumer signature
changes and no interface is introduced. It holds the same 15 maps, empty, plus a `base` pointer.
**Every read consults the overlay and then the base; every write touches only the overlay.** The base
is the fully loaded, fully expanded library index, `Freeze()`d once per process, built by
`sync.Once`.

The read-merge rule per map, which is the audit the stage consists of:

| State | Merge on read | Why it is sound |
|---|---|---|
| `fqn[k]` | base entries then overlay entries | a fresh build loads library documents before the user's, so the order matches — and the proof of §3.4 asserts it rather than assuming it |
| `children[p]` | union of key sets; `childKeys` already sorts | enumeration order is name order, not map order |
| `declaredAt`, `docRoots`, `docOfRoot`, `docKinds`, `contributions`, `libraryDocs`, `librarySyms` | overlay first, then base | keyed by document or symbol, and the two sets are disjoint: a symbol is declared by exactly one document |
| `wildcardMeta[pkg][doc]`, `nsFilters[ns][doc]` | merge the inner per-document maps | already per-document, which is why a user document stating `import ISQ::*` through a library package is representable without touching the base |
| `reexportDocs[key][doc]`, `docReexports[doc]` | merge per document | a claim belongs to the document whose import made it |
| `reexported[fqn][sym]`, `hidden[fqn][sym]` | union, minus the overlay's suppression set (below) | the marks are derived from the claims, so they merge the same way |
| `dirtyNS`, `lastTargets` | read base's `lastTargets` as the starting point, write locally | an importer the user document cannot reach is not re-derived, which is what keeps adding a document cheap |

**The one subtractive channel, which is the sharp edge of this design.** A user document can only
*add* names, so almost everything is additive — but it can invalidate a re-export the base derived,
in one way: by making a base import target ambiguous. `wildcardTargetAt` resolves a wildcard target
only when exactly one symbol owns the key, so a user file declaring a top-level `package ISQ` makes
`ISQ` ambiguous, and every re-export the library derived through `import ISQ::*` has to disappear —
which an overlay cannot do by deleting from the base. It therefore keeps
`suppressed map[reexportKey]bool`, written by the expansion when a base re-export no longer follows
from its imports and consulted by the merged reads (including `fqn` and `children`, so suppressing
the last claim on a key hides the registration too). The set is bounded by the importers a change can
reach, which `importersToRefresh` already computes. This case is legal SysML and today's per-model
index handles it, so it is not in scope to declare unsupported; it is the case the proof must hit
first.

**`RemoveDocument`.** On the overlay it drops the overlay's own contributions, claims and
suppressions and re-expands, exactly as today. A *base-owned* document cannot be removed through an
overlay — that would mutate shared state — and nothing removes a library document in any consumer:
gRPC and the LSP only ever add and re-add user documents. `RemoveDocument` of a base document is
therefore a programming error the frozen flag names, not a silent partial removal. Model eviction
drops the overlay and nothing else; the base is never reference-counted per model.

**Concurrency.** The base is frozen after construction, and nothing in `symbols` mutates on read:
scopes and symbols are built eagerly by `Build`, and the memoized state a query needs lives in
`resolve.Resolver` and `semantics.Model`, which are per model (`AGENTS.md` §4 — semantics in side
tables). So N overlays may read one base concurrently with no lock, and an overlay itself is never
shared between models. The frozen flag turns an accidental write into a named failure rather than a
race the detector has to catch in the field.

**Rejected, one line each:**

- *Copy-on-write of the whole index* — that is today's behaviour, priced at 1.56 GiB in §3.1.
- *One mutable shared index, locked per request* — a request would observe another's document between
  add and remove, and every re-expansion would serialize the service.
- *An `IndexReader` interface with a shared read-only implementation* — the same fall-through, but as
  a signature change across every consumer of `*symbols.Index`; the overlay gets it inside the
  package.
- *Sharing only the AST and rebuilding the index per model* — the index build, not the parse, is the
  cost (§2: 41 ms parse versus 83–90 ms parse-and-index, and 15.9 MiB retained).

### 3.4 The proof the sharing stage owes

`TestPooledIndexMatchesFreshlyBuiltIndex` is the shape but not the strength. The successor asserts,
for a model resolving against an overlay versus the same model in an index of its own:

1. **Identical diagnostics** — `passes.Analyze` over the user document, compared as a multiset of
   (rule, span, severity, message).
2. **Identical qualified lookups over the whole index** — for every name in the union of `FQNs()`:
   `LookupQualified`, `LookupQualifiedFrom` from the declaring namespace and from outside,
   `LookupDirectChildren`, `LookupDirectChildrenFrom`, `HiddenFrom`, `Declaring`, `ReexportGates` and
   `TopLevelBindings`. Symbol identity is compared by FQN and kind, since a shared base hands out the
   *same* pointers by design.
3. **The ambiguity case explicitly** — a user document declaring a top-level package that a library
   package wildcard-imports, which is the suppression path of §3.3, plus its removal restoring the
   base's re-exports.
4. **Isolation under concurrency** — two models over one base, run concurrently under `-race`,
   neither seeing the other's document in any of the lookups of (2) nor in its diagnostics.
5. **Eviction** — after `RemoveDocument` of the user document, the overlay answers every lookup of
   (2) exactly as a fresh overlay over the same base does.

And the oracles: the sharing stage is a memory change, so all three fresh-cache runs must be
identical, with the differential multiset reported rather than its total.

### 3.5 Where this splits, and why

Two sessions, and the split is at the package boundary:

- **A — `symbols`.** `NewOverlay`, `Freeze`, the read-path audit of the 15 maps and ~123 accesses, the
  suppression channel, and the five proofs of §3.4. No consumer changes, so no oracle can move.
- **B — the consumers.** gRPC's pool becomes one shared base (`internal/grpc/libindex.go` largely
  deleted), the edit path stops taking an index per operation (**L3-9**), the REPL's second index
  becomes an overlay of the workspace's base, then re-measure §3.1 and re-run the three oracles.

The honest reason for splitting there: the audit and the suppression case are where a mistake becomes
a wrong answer, and pairing them with the consumer migration in one stage is how a partial
copy-on-write shim gets landed to make the deadline. A is worth landing alone because its proof is
what makes B safe.

---

## 4. The record format under option C, and its version-compatibility surface

The persisted type stays `libs.IndexRecord`, gob-encoded, one file per library document, at
`<cache>/sysml-ls/libs/<key>.idx`. What changes is that it becomes a fact table keyed by FQN rather
than a symbol table:

```go
// FactRecord is the persisted derivation of one library document's analysis.
type FactRecord struct {
    Name  string
    Facts []symFacts // in document order, keyed by FQN
}

type symFacts struct {
    FQN         string
    Supers      []string                  // resolved generalization targets
    FeaturedBy  []string                  // resolved `featured by` targets
    Unit        *unitFacts                // measurement units only
    Dimension   *symbols.DimensionFacts   // measurement units only
    Behavior    *symbols.BehaviorFacts    // behaviors and steps only
    Annotations []symbols.AnnotationFacts
    Filters     []*symbols.FilterPredicate // compiled namespace filters
    Imports     []wildcardImportFacts      // compiled filters of `import X::*[…]`
}
```

Dropped relative to `symRecord`: `ShortName`, `Kind`, `Span`, `AliasTarget`, and the `Target`/
`Private` fields of a wildcard import — all of them re-derived from the document at no measurable
cost, because the document is now always there.

**Version compatibility.** The rule is unchanged and it is the strong one: *a stale record is a miss,
never a wrong answer.* Four independent components of the cache key enforce it, and all four stay:

| Key component | What it invalidates | Why it is needed |
|---|---|---|
| `sha256(content)` | an edit to the file itself | the facts are derived from it |
| `-s<setDigest>` | an edit to **any** file of the library set | a unit reduction follows a reference unit declared in a sibling file |
| `-b<buildID>` | a change to the code that derived the facts | a fact is a function of our code, which no input covers; a dirty tree gets a per-build identity, so a modified working copy never reuses a record |
| `-v<formatVersion>` | a change to the persisted shape | a decode of an old shape is not attempted |

Two consequences worth stating explicitly, because they are what makes "stale is a miss" true rather
than aspirational:

1. **Decode failure is a miss.** `Cache.Load` already treats an absent file, a read error and a
   decode error identically. A `FilterPredicate` or `DimensionFacts` whose shape changed without a
   `formatVersion` bump is caught by `buildID` first; the version exists for the case where the code
   did not change but the encoding did (e.g. a field reordering in a vendored type).
2. **A partial record is impossible to observe.** `Cache.Store` writes a per-key temp file and
   renames it, so a crashed or concurrent write is never read.

The format version becomes `24` when this lands. `maxIdleAge` pruning is unaffected.

---

## 5. Step 3 — the equality proof, as an enforceable test

`index_only_test.go` is replaced, not deleted, by a test of the same strength. The successor asserts
three properties over the bundled library and over a purpose-built fixture that declares what only a
declaration exposes (members, a declared value, a condition body, a `[0..1]` multiplicity):

1. **`TestLibraryFactsAreEqualColdAndWarm`** — load twice through one cache directory and compare,
   for every symbol of every library document: the fact families of §4 (deep-equal), plus the
   consumer-visible answers `EffectiveMultiplicityOf`, `MembersOfIncludingRedefined`,
   `AnnotationFactsOf`, `Conforms` over every recorded supertype edge, and `Eval` of every declared
   value. A single loop over `idx.FQNs()` covers 19288 names, so the test cannot rot by omission the
   way an enumerated list would.
2. **`TestRestoredFactsEqualDerivedFacts`** — the sharper form: restore a record, then re-derive the
   same facts from the parsed document with a fresh `semantics.Model`, and require equality. This is
   the property that replaces "the cache has no semantic effect": a pre-filled memo that differs from
   what the derivation would produce is a bug the test names.
3. **`TestLibraryBodiesArePresentOnEveryPath`** — the inverse of today's assertion: every library
   symbol *has* a `Decl`, cold, warm and with `cache == nil`. Today's test proves the absence is
   uniform; this proves the presence is.

`TestMultiplicityOfALibraryFeatureIsTheSameColdAndWarm`
(`tests/semantics/cached_library_test.go:253`) flips at this point: it stops asserting the
assumed `1..1` and asserts the declared `0..1`, on both paths. Its comment already says so.

### The one way this proof is weaker than today's, and how it is closed

"No symbol of a library document has a `Decl`" cannot rot: there is nothing to keep in step with. "A
restored fact equals the derived fact" can. Add a field to the persisted facts without adding it to
the comparison and the test still passes while the field is never checked — and the failure mode is a
wrong answer on a warm cache, which is exactly the class of bug the index-only contract was adopted
to end. Discipline in a later stage is not an answer, so the test carries the obligation itself:

```go
// Every persisted fact field must be compared, or this fails: a field added to
// symFacts without a comparison is an untested wrong-answer path.
func TestRestoredFactsEqualDerivedFacts(t *testing.T) {
    compared := map[string]func(a, b symFacts) bool{
        "FQN": ..., "Supers": ..., "FeaturedBy": ..., "Unit": ..., // ...
    }
    for i := 0; i < reflect.TypeFor[symFacts]().NumField(); i++ {
        name := reflect.TypeFor[symFacts]().Field(i).Name
        if _, ok := compared[name]; !ok {
            t.Fatalf("symFacts.%s is persisted but not compared: add it to compared", name)
        }
    }
    // ... then, for every symbol of every library document, require every
    // comparator to hold between the restored record and a fresh derivation.
}
```

The reflective guard is the load-bearing part: the failure for an uncovered field is a test failure
naming the field, not a silent gap. `TestLibraryFactsAreEqualColdAndWarm` gets the same treatment for
the consumer-visible answers it enumerates — that list cannot be derived reflectively, so it is
asserted against a named checklist in the test file, and adding a fact family to the record without
extending the checklist fails the reflective guard first.

---

## 6. Step 4 — the consumers, and what each one starts seeing

A library body becomes visible to everything that walks declarations. This is a diagnostics change to
adjudicate per consumer, not plumbing. What is measured today, over the bundled library parsed as
ordinary documents, marked as library content and with wildcard imports expanded — i.e. the setup the
loader itself produces:

- The parser reports **0 diagnostics** on all 95 files, so nothing arrives from the parse itself.
- `passes.Analyze` over those documents reports **95** diagnostics: 40 `usage-typing` errors,
  26 `unresolved` errors, 14 `feature-reference-featuring-types` errors, 5 `specialization-cycle`
  errors, 4 `invocation-not-behavior` errors, 3 `result-expression-at-most-one` errors, 2 `one-type`
  errors and 1 `redefinition-no-derived-name` warning.

None of the 95 reach a user by themselves: `Workspace` analyses only the documents it was given
(`w.docs`), and a library document is indexed, not opened. But the list is the honest size of the
residue this item uncovers.

**Two artefacts of how a library is set up, verified rather than assumed**, because a number quoted
out of this page later would otherwise mislead:

- Omit `idx.MarkLibrary`, and the same run reports **307** diagnostics — the 95 plus 94
  `library-package` warnings and 118 `name-conflict` warnings. For `library-package` the mechanism is
  read: `W9CUserStandardLibraryPass` returns early for a library document (`w9cIsLibraryDocument`),
  and the loader marks every file it loads (`loader.go:77,87`). For `name-conflict` the exemption is
  measured, not traced: both groups are exactly 0 when the documents are marked. 307 unmarked, 95
  marked.
- Omit `idx.ExpandWildcardImports()` — which `LoadAll` calls (`loader.go:56`) — and `unresolved`
  jumps from 26 to **664**, 570 of them in `SI.sysml` and `USCustomaryUnits.sysml`, because those
  files reach `ISQBase` through `public import ISQ::*` re-exporting it. Without the expansion
  `SI::gram` does not even type as a unit (`IsMeasurementUnit` is false and its only supertype is
  `Base::DataValue`); with it, and through the loader on both cold and warm paths, `SI::gram`
  specializes `ISQBase::MassUnit` and carries unit facts. Any future probe of library diagnostics has
  to reproduce both steps or it measures its own setup.

Two of the 95's rows are real work:

- **`solve`.** `internal/core/solve/differential_corpus_test.go` parses the library itself
  (`parseLibraries`) *because* records hold no conditions; that workaround becomes unnecessary and is
  deleted. In exchange, the translation must decide what to do with library invariants that are
  outside the SMT-translatable subset — the pre-index-only behaviour was to try, and that is what
  made a warm cache differ from a cold one. The gate is `solve`'s own subset check, which must skip a
  library condition it cannot translate rather than reporting on it.
- **`internal/grpc`.** `attributesOf` reads `sym.Decl` (`internal/grpc/attributes.go:142,167`), so
  inherited library attributes start appearing in responses again — dozens of them, which is the
  regression the index-only contract was adopted to stop. The difference is that now they appear on
  *both* paths, so the answer is stable. The policy is **decided** by the project maintainers (**L3-3**) and is
  stated below; it is not implemented in the sharing stage.

### The decided element-API policy (L3-3)

The element API reports an element's own attributes plus those it inherits from **non-library** types,
and withholds those inherited from library content. Two conditions are part of the decision:

1. **The omission is visible in the response.** A client must be able to tell that attributes were
   withheld, so the response carries a **count of withheld library-inherited attributes** — a count
   rather than a bare flag, because a flag cannot distinguish one withheld attribute from forty, and a
   client deciding whether to offer a "show inherited" affordance needs the size. That is a new proto
   field, i.e. a schema change with the Python client (`clients/python/opensysml`) and the VS Code extension
   downstream of it.
2. **"Library" means the index says so** — `Index.Library`/`MarkLibrary`, never a name or FQN prefix
   heuristic. A user's own imported library must not be dropped by a prefix guess.

Rejected alternatives: *report everything* — faithful, but buries the modeller's own attributes and
reproduces the pre-index-only blow-up; *label every attribute by its declaring type* — same row count
as reporting everything, so it does not solve the noise.

When that stage arrives, `TestAttributesAreReportedOwnThenInherited` keeps asserting its
`[mass, wheels, derived, label, inheritedOnly]` expectation, which stays true under this policy, and
gains a case whose supertype chain reaches library content, asserting the **withheld count** rather
than merely the absence.

A third consumer-visible gap is not a diagnostics question but a capability one:

- **Library value expressions must evaluate.** The roadmap names `isSolid = isEmpty(voids)`
  (`Systems Library/Items.sysml:105`) as a resolution failure. Measured, it is not one: with the body
  parsed, the callee resolves to `SequenceFunctions::isEmpty`, and it is `semantics.Model.Eval` that
  declines to fold the invocation (`ok == false`). The gap is in evaluation, not resolution, and the
  wording in `roadmap.md` should be corrected when this item is picked up.

---

## 7. Rows this page does not close

Every row below is open, with a category from
[the adjudication categories](adjudications.md) where it is a divergence, and an owner.

| Row | What it is | Category | Owner |
|---|---|---|---|
| **L3-1** | The design itself is unimplemented: records are still index-only, and a library still contributes no bodies. | unimplemented obligation | L3, next stage |
| **L3-2** | `Model.Eval` cannot fold a library value expression that invokes a Kernel Function Library function over a feature (`isEmpty(voids)`). Resolution is fine; evaluation declines. | unimplemented obligation | L3, before consumers migrate |
| **L3-3** | **Decided** (§6): the element API withholds library-inherited attributes and reports a count of what it withheld, keyed on `Index.Library`. Open only as unimplemented work, including the proto field and its two downstream clients. | adjudicated divergence, decided; implementation outstanding | `internal/grpc`, after the sharing and record stages |
| **L3-4** | **Closed by stage A** ([#504](https://github.com/JPL-Devin/OpenSysML/pull/504)): the population was one index per cached model — 100 at the gRPC default, 15.95 MiB each — and is now one shared base, so the +17 MiB of keeping the trees is a once-per-process cost. | not a divergence — a cost decision | project maintainers |
| **L3-5** | The 26 `unresolved` errors the passes report over the parsed library (`that`, feature chains such as `CartesianVectorOf::result::dimension`, and implicit-redefinition targets). Each needs a category of its own once a consumer actually surfaces it. | not yet adjudicated | L3, after step 2 |
| **L3-7** | First derivation of a specialization edge costs ~36 µs per symbol (~365 ms over the library) and is dominated by resolving each generalization target; the memo itself is live (a second pass is 0.13 ms). Whether 36 µs per edge is acceptable is a `semantics`/`resolve` question, and if it were cheap the cache would have little reason to exist. | performance defect, unadjudicated | `semantics` |
| **L3-8** | **Delivered** in [#504](https://github.com/JPL-Devin/OpenSysML/pull/504): `Freeze`/`NewOverlay`, the layered read paths, the suppression of an ambiguated import target, the five proofs of §3.4, and the gRPC/REPL/LSP/workspace migration — done as one stage rather than the A/B split of §3.5. | unimplemented obligation, now met | `symbols`, `internal/grpc`, `internal/repl` |
| **L3-9** | **Closed**: `edit.Apply` takes one index per call and reuses it, since adding a document a name already holds drops the previous contributions first (`internal/core/edit/edit.go`, `reindexer`). Measured on a fresh cache, 10 runs each: a 5-operation request took **6** indexes and 12.03 MiB of allocation in 37.6 ms, and now takes **1** and 3.04 MiB in 8.7 ms; a 10-operation request went from **11** indexes, 34.97 MiB and 100.3 ms to **1**, 5.68 MiB and 15.7 ms. An overlay of the shared base is 1.4 KiB retained (200 of them, 0.3 MiB), so the heap the per-operation indexes held was the analysis each carried, not the index: the same stage stopped analyzing the intermediate notation, whose diagnostics no caller reads — an edit is judged by the original's and the returned notation's. Equivalence with applying the operations one at a time — notation, applied edits, diagnostics, whole-index qualified lookups, refusal kinds, and no write into another model's index or the frozen base — is proved in `internal/grpc/edit_reindex_test.go`. | performance defect, closed | `internal/grpc` |
| **L3-10** | **Closed by stage A**: the REPL's two library indexes (30.7 MiB) are two overlays of one shared base; 4 sessions with a submission and a browse index cost 17.1 MiB against 122.7 MiB. | performance defect | `internal/repl` |
| **L3-6** | `roadmap.md`'s L3 text says library value expressions do not resolve; measured, they resolve and fail to evaluate. Also it says `TestMultiplicityOfALibraryFeatureIsTheSameColdAndWarm` skips, while it asserts. | our defect (documentation) | this page; corrected in the roadmap by this PR |

No oracle row moved, in either direction, so no conformance claim is made or implied here. The Xpect,
differential and rejection multisets are the controls in the table at the top of this page, measured
before and after, identical.

---

## 8. Sequencing

1. **This PR** — the design above, the measurements of §2 and §3.1, and the roadmap correction.
   Nothing behavioural.
2. **Stage A** — the shared index inside `symbols`: `NewOverlay`, `Freeze`, the read-path audit, the
   suppression channel, and the five proofs of §3.4 under `-race`. No consumer changes.
3. **Stage B** — the consumers of the shared index: gRPC's pool, its edit path (**L3-9**), the REPL's
   second index (**L3-10**); §3.1 re-measured and the three oracles re-run.
4. **Then** — option C's mechanism plus the three tests of §5, with `AddRecords` / `RecordEntry`
   deleted and `formatVersion` bumped to 24, the +17 MiB now paid once per process.
5. **Then** — the consumers of §6, one PR each: `solve`'s subset gate (and the deletion of
   `parseLibraries`), then the decided element-API policy of **L3-3**, then `Model.Eval` for
   **L3-2**.

If the reviewer prefers option B, §2 states what it additionally needs (trivia and ~83 gob
registrations). The +17 MiB of **L3-4** stops being a reason to reject option C once §3 lands, which
is the point of doing §3 first.
