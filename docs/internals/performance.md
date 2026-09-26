# Performance and Memory

How to profile `sysml`, what a large model costs today, and what the measurements
say about where the remaining cost is. Figures below were taken on an
`Intel Xeon Platinum 8559C`, Go 1.25, `GOMAXPROCS=8`; treat them as
ratios rather than absolutes. Each release is measured against the one before it
in a record under `docs/project/`; the latest is
[release 0.8.0 against release 0.7.0](../project/performance-release-0.8-vs-0.7.0.md).
How far one realistic model scales — a satellite constellation with every
spacecraft modeled to its components, from 2 to 12 800 satellites — and where
validation, satisfaction checking and editing each stop being practical is in
the [satellite-network stress test](../project/satellite-network-stress-test.md);
the design for holding and editing models an order of magnitude larger than
that test reaches is
[scaling to very large models](../project/large-model-scaling-design.md).

## Profiling a run

The binary profiles itself, so a profile is taken of the same code a user runs
rather than of a test harness:

```bash
sysml -validate -memstats model.sysml               # what the run cost, on stderr
sysml -validate -memprofile heap.out model.sysml    # heap profile for go tool pprof
sysml -validate -cpuprofile cpu.out model.sysml     # CPU profile for go tool pprof

go tool pprof -sample_index=alloc_space -top heap.out
go tool pprof -top cpu.out
```

`-memstats` prints the wall time, the memory allocated over the whole run, the
number of allocations and collections, and the memory taken from the operating
system:

```
sysml: 684ms wall, 239.3 MiB allocated in 1638904 allocations over 16 collections, 104.0 MiB taken from the OS
```

Read the two memory figures for what they are. *Allocated* is cumulative
allocation, the pressure the run put on the collector — it is not how much memory
the process needs. *Taken from the OS* is a floor on peak resident size, which is
what bounds the model a machine can hold. Neither is the live size of a loaded
model: by the time a run ends the model is unreachable, so a heap profile written
at exit records where the run allocated, not what a held model occupies.

Peak resident size is measured from outside:

```bash
/usr/bin/time -f "%es wall, %MkB peak RSS" sysml -validate model.sysml
```

## Benchmarks

`internal/frontend/repl/bench_test.go` loads and runs synthetic models of a stated size,
so a cost that grows faster than the model is visible as a per-element figure
that grows with size:

```bash
go test ./internal/frontend/repl -run '^$' -bench . -benchmem
go test ./internal/frontend/repl -run '^$' -bench BenchmarkLoadModel -benchmem -memprofile heap.out
```

Beyond the standard figures they report:

- `live-B/op` — memory the loaded model holds, measured with the session still
  reachable, which is what bounds how large a model can be held at once.
- `B/element` — memory allocated per model element while loading.

`elements=0` is a model with no elements: its figures are what a session costs
before it holds anything, so reading a size against it separates the model's cost
from the session's.

`internal/syntax/parser/bench_test.go` times the parser alone over a real model: it
parses every `.sysml` and `.kerml` file under the directory `OPENSYSML_BENCH_MODEL`
names, with no library, no name resolution and no validation, and skips when the
variable is unset:

```bash
OPENSYSML_BENCH_MODEL=/path/to/model go test ./internal/syntax/parser -run '^$' -bench ParseModel -benchmem
```

`tests/perf/model_bench_test.go` times the whole load of the same directory
as one REPL session — library, resolution and validation included — and fails if the
model has errors, so the measured load is a clean one:

```bash
OPENSYSML_BENCH_MODEL=/path/to/model go test ./tests/perf -run '^$' -bench REPLLoadModel -benchmem
```

## A real model: Apollo 11

The synthetic models below scale one shape; a model written by systems engineers
leans on the quantity and unit libraries, nests deeply, and exercises constructs the
generator never emits. The largest public one is Airbus's
[Apollo 11 SysML v2 model](https://github.com/airbus/apollo-11-sysml-v2)
(Mozilla Public License 2.0): at commit `6e9c93f`, **28 files, 7,221 lines,
347 KB** of SysML v2 spanning the program, its requirements, the logical and
technical architectures, operations, and the trajectory calculations.

```bash
git clone https://github.com/airbus/apollo-11-sysml-v2 && git -C apollo-11-sysml-v2 checkout 6e9c93f
OPENSYSML_BENCH_MODEL=apollo-11-sysml-v2 go test ./internal/syntax/parser -run '^$' -bench ParseModel -benchmem
sysml -validate -memstats $(find apollo-11-sysml-v2 -name '*.sysml')
```

| what | wall | allocated |
| ---- | ---- | --------- |
| parse all 28 files | **8.2 ms** | 4.9 MiB in 31 000 allocations |
| `sysml -validate`: load the standard library, resolve, validate, report | **0.43 s** | 196 MiB in 1.39 million allocations, about 157 MiB taken from the OS |

Parsing is about 2% of the whole run — 880 lines a millisecond, 42 MB/s —
so the cost of loading a model is name resolution and validation, and that is
where the work described in the rest of this page goes. The largest single share
of the remainder is the lookups made through the model's wildcard imports of the
quantity libraries (`import ISQ::*`, `import SI::*`), which is why those are
answered by name (below).

### What it reports

The run reports **4 warnings and no error**, and every one of them is a finding
about the model. Three of the warnings are calculation invocations that leave an
input the calculation declares unbound, so the call cannot be evaluated — well-formed
SysML v2 the reference validator also accepts, hence advisories rather than errors — all
in `Analysis/CalculationsPackage.sysml`:

| Line | Expression as published | Finding |
|---|---|---|
| 111 | `return deltaV :> ISQ::speed = isp * g0 * ln(m0 / mf);` | `ln` is the alias of `CoSMAQuantitiesAndUnitsPackage::naturalLogarithm`, declared `calc <ln> naturalLogarithm { in x: DataValue[1]; in y: DataValue[1]; return : DataValue[1]; }` — two inputs, one argument. A natural logarithm takes one argument; the second `in` is the slip |
| 124 and 135 | `return deltaV :> ISQ::speed = calculateDeltaV(isp, initialMass, finalMass);` | `calculateDeltaV` declares `in isp`, `in g0`, `in m0`, `in mf` — four inputs, three arguments, so `g0` (standard gravity) is never supplied; bound by position, it is the last input, `mf`, that the warning names |

The fourth is dimensional: `calculateLoiDeltaV` declares the Moon's gravitational parameter
`in mu_Moon :> ISQ::force`, so `v_inf^2 + 2*mu_Moon/r_periapsis` adds a
velocity squared (L²·T⁻²) to a force over a length (M·T⁻²). A gravitational
parameter is L³·T⁻².

The pinned OMG pilot validator over the same files reports nothing: it does not
check an invocation's argument count against the invoked calculation, a gap
[omg-issues.md](../project/omg-issues.md#defects-in-the-pilot-implementation)
records. Nothing has been filed against the model's repository.

## What a large model costs

Loading is parse, scope building, name resolution and the validation passes —
what `sysml -validate` does. Each element is a part definition, a calculation, an
action, a state machine or a part usage.

| elements | load wall | allocated | live heap held |
| -------- | --------- | --------- | -------------- |
| 0        | 1.0 ms    | 695 KiB   | 32 KiB         |
| 250      | 16 ms     | 10.3 MiB  | 2.1 MiB        |
| 1 000    | 62 ms     | 40 MiB    | 8.1 MiB        |
| 4 000    | 260 ms    | 161 MiB   | 32 MiB         |

The `elements=0` row is what a session costs before it holds a model of its own:
**32 KiB held and 695 KiB allocated**. The standard library is no longer indexed
eagerly per session, so a process serving many sessions (the LSP, a server) no
longer pays a multi-megabyte floor for each one. Most of the current floor is
the `about`-metadata index (`semantics.Model.annotationsAbout`), whose walk
visits every document in the index — the bundled standard library included —
once per session; the walk's visited-symbol set is transient, so the held size
is unchanged.

Above that baseline a model costs roughly **8 KiB held per element**, and
allocates roughly **41 KiB per element** while loading, most of it short-lived
parser and resolution garbage.

Whole-binary scaling on `-validate` over generated models in standard notation,
which report nothing:

| elements | wall   | peak RSS |
| -------- | ------ | -------- |
| 3 000    | 0.30 s | 117 MiB  |
| 6 000    | 0.49 s | 159 MiB  |
| 12 000   | 0.92 s | 272 MiB  |

Both time and memory grow linearly with the model. Loading was quadratic once —
doubling the model roughly quadrupled the time — for the reasons below.

### What made it quadratic

The load path had several costs, including three scans over a namespace's
members, each performed once per member of that namespace:

- `passes.checkRedefinition` searched the enclosing scope for the symbol owning
  it, copying the scope's member-name list for every declaration checked. It cost
  70% of all memory allocated while loading a large model. The scope already
  records its owning symbol (`Scope.Owner()`), which every other pass uses, so
  the search was redundant; the same applies to the scans that asked whether a
  redefined member was declared locally or inherited, which the member's own
  `OwnerScope` answers.
- `resolve.childScope`, `passes.childScopeOf` and `symbols.bodyScopeChild` each
  walked a scope's children to find the one a declaration owns. A scope now
  answers that itself (`Scope.ChildFor`), scanning few children and indexing them
  by declaration node above a threshold.
- `resolve.importsOf` rebuilt a namespace's import list on every unqualified name
  lookup made in it. The tree is immutable after parsing, so the resolver
  memoizes it.
- Every scope allocated a member map even when it had no named members, and a
  populated key also paid map-bucket overhead plus a separate one-element slice.
  The eager per-scope member map is gone: scopes now store named members in
  declaration order, scan small scopes, and build a lookup index lazily only
  above the 12-entry threshold.

### What made reporting findings quadratic

Reporting a finding needs the line and column its offset falls on, which
`SourceFile.Lines()` answers from a line index over the whole file. The index was
rebuilt on every call, and the calls are made once per finding, so validating a
model that reports something cost the file's size times the number of findings: a
16 000-element model that warns on every usage took 123 s and allocated 258 GiB,
against 1.0 s over the same model that warns on nothing. A source file's content
is immutable, so the index is now built once per file and reused, and the REPL's
diagnostic paths hold one index per submission rather than one per finding. That
model now takes 2.1 s and allocates 1.4 GiB — around 570 B allocated per finding
reported, flat in the size of the file.

Materialization rollback had the same shape. A failed creation drops every object
it reached, which it used to identify by copying the whole live-instance key set
before each (also nested) materialization — 81% of all memory allocated while
instantiating. The context now records instance registrations in an append-only
log and marks its length, so a rollback walks only what the failed creation
added.

### Where a load's allocation goes

A load allocates substantially more than the model it leaves behind holds, and a
CPU profile of it is spent in the collector, so allocation volume is what a
load's wall time follows. The load's major allocation sources include:

- The validation passes each walked the document's symbol tree themselves, copying
  every scope's member list on the way — 17 traversals of the same tree per
  document. The passes of one run share a `Context`, which now holds the
  traversal, and the walk is made once.
- A scope used to allocate a member map and a separate member-order slice even
  when it held no named member. Flat declaration-ordered storage removes both
  costs from empty scopes and avoids per-key buckets and one-element slices from
  populated scopes.
- `parser.fill` grows the token buffer as it reads. The parser sizes that buffer
  from source length; the estimate suits dense models and over-allocates on
  sparse ones.
- A wildcard import surfaces a name by enumerating the target namespace's direct
  children (`symbols.Index.LookupDirectChildren`), which rebuilt and re-sorted
  that list on every unqualified lookup made through the import, and then
  compared the name against every member — the whole of `ISQ` for each name a
  model reaching `import ISQ::*` resolves. The index now keeps the list per
  namespace and visibility and indexes it by leaf and short name
  (`LookupDirectChildrenNamed`), so a lookup costs its matches rather than the
  namespace; it drops what it kept whenever a write lands in any of its tables, so
  a lookup after an edit is recomputed.

The scope representation is shaped by the actual models. In the standard
library, 10,666 scopes contain no named members 70.5% of the time, 99.3% contain
at most eight named members, and the largest contains 516. A generated model
reaches 16,000 named members in one scope. The former representation therefore
paid for maps on roughly 72% of scopes that never used one, while a populated
member cost roughly 80 bytes of map and slice overhead for 24 bytes of data. A
linear scan is appropriate for the common small scopes; the lazy index prevents
the large scopes from becoming quadratic.

### What a load repeated for nothing

Four costs were paid per element where the model needs them once. Removing them
together cut allocation count at 12 000 elements by 40% (7.70M to 4.62M), bytes
allocated by 32% (744.9 to 503.6 MiB), peak resident size by 23% (353 to
272 MiB) and wall time by 20% (1.14 s to 0.92 s):

- The parser recorded every run of whitespace as a trivia entry on the node that
  followed it. Whitespace is most of a file's trivia and no consumer reads it, so
  only notes and comments are recorded now.
- `source.SourceFile.Text` copied the bytes a span covers into a new string, once
  per name, keyword and literal the parser reads. Spans are now taken from one
  cached copy of the content, so a span costs no allocation of its own.
- Comparing a symbol's fully-qualified name against a known one built that name
  first — walking the owner chain and concatenating it — for every candidate
  supertype of every declaration. `symbols.HasFQN` compares the segments against
  the chain from the end instead, so a mismatch allocates nothing.
- `passes.W9CInheritedNameConflictPass` merged every member of every library base
  a declaration conforms to into one map keyed by name, per declaration, to
  answer two questions: whether a name the declaration owns is also contributed
  by a base, and whether two bases contribute one name. The first asks only about
  the few names the declaration owns, so it now asks each base for those; the
  second is only possible with two bases, so the map is built only then.
- A scope indexed its children by declaration node as it built them: a map per
  scope with children. Member lookup already scans small scopes and indexes
  lazily above a threshold, and `Scope.ChildFor` now does the same, so the common
  small scope holds no map.

Diagnostics, resolution results and exit status are byte-identical, over the
bundled `.sysml` and `.kerml` fixtures as well as the test suite.

### Per-token and per-name allocations the parser did not need

Three allocation sources were paid once per token or per name parsed, where one
allocation per file or none serves. Removing them cut allocation count on a
12 000-element model by 15% (4.33M to 3.69M) and bytes allocated by 2.8%
(487.7 to 474.1 MiB), with wall time unchanged — the savings are small objects,
so the win is collector pressure rather than bytes:

- The lexer materialized every keyword's text as a fresh string when it built
  the token. The keyword table now maps each keyword to one canonical string,
  so every `part`, `def` or `import` token shares it.
- Every qualified name allocated a parts slice, though most names have one
  segment. `ast.QualifiedName` now carries inline storage for a single segment
  (`SetSingleton`), capped so appending a second segment copies out to an
  ordinary slice; multi-segment names behave as before.
- The resolver's inherited-member check built a redefinition-closure map per
  inherited symbol and redefined-by maps per scope, though most symbols redefine
  nothing. A symbol with no redefinitions is now checked directly against its
  own entry, and the maps are built only when something redefines something.

Diagnostics and exit status were verified byte-identical against the previous
binary over the same models.

### What a batch of files costs

`sysml -validate a.sysml b.sysml …`, `-satisfy` and `%load` open the files as
one batch of workspace documents (`model.(*Workspace).OpenAll`): the files are
parsed and their scope trees built on a pool of workers, installed in the
shared index one after another, wildcard imports are expanded once for the
batch, and the documents are analyzed on the pool
(`model.(*Workspace).DiagnosticsAll`), each in a `passes.Context` of its own
with a private resolver and semantic model, over an index nothing writes while
the pool runs. What resolving a document would otherwise link into the scope
tree on first use — the owner of a metadata body — is linked for every document
of the batch before the pool starts (`passes.PrepareBatch`), so the workers
only read it. The batch carries a `passes.Gathers` of its own
(`passes.Batch.Gathers`): the first context that runs a workspace-wide audit
gathers every document's facts into it, under its lock, and every context reads
the same union afterwards, so the audits gather each document once per batch
rather than once per analysis. A private resolver records no dependencies, so
the diagnostics a batch computes are cached with none — dropped on any change
to the workspace (`Workspace.batched`) rather than per dependency — and what
its contexts gather never enters the workspace's own `passes.Gathers`, whose
entries are invalidated per document as the editor path's resolver reports.
The batch does settle the regathers an edit left pending before it consults
the diagnostics cache, so a verdict the editor path cached is not served once a
change to another document has undone it. Diagnostics come back in the order the
files were given and are the same at any job count; `-jobs` and
`OPENSYSML_JOBS`, the setting that bounds how many runs of one check go
concurrently, set the pool, default one worker per CPU. The earlier cost of
indexing files one at a time — re-expanding wildcard imports over every
document loaded so far, quadratic in the file count — is gone with it.

Measured on the satellite constellation split one file per orbital plane
(`stress-model -split-planes`; `Intel Xeon Platinum 8559C`, 8 CPUs, 31 GiB,
Go 1.25.0, one run each, `/usr/bin/time -v`):

| model | files | jobs | wall | CPU | peak RSS |
| ----- | ----- | ---- | ---- | --- | -------- |
| 1 600 satellites, one file | 1 | — | 20.3 s | 133% | 2.38 GB |
| 1 600 satellites, split | 34 | 1 | 22.0 s | 129% | 2.27 GB |
| | | 2 | 14.3 s | 213% | 2.12 GB |
| | | 4 | 10.7 s | 288% | 2.23 GB |
| | | 8 | 9.24 s | 365% | 2.63 GB |
| 200 satellites, split | 10 | 1 | 2.63 s | 129% | 358 MB |
| | | 8 | 1.19 s | 341% | 507 MB |

The split costs a little over the single file's time on one job and 2.2× less
on eight, within a tenth of the single file's peak RSS. What bounds the pool
is the gather: in the eight-job CPU profile (9.57 s wall, 34.2 s of samples)
the three audits' gather is 4.5 s — the OOSEM union 3.6 s, identity 0.52 s,
MOSA 0.37 s — run by one context over all 34 documents while the other
workers wait at the lock; the scan of the files for the root namespaces they
import from their siblings (`project.Dependencies`, 0.87 s) and installing the
scope trees and expanding wildcard imports before the pool (`commitBatch`,
1.0 s) are serial too; the rest — name resolution 9.0 s, the inherited-name
conflict pass 4.0 s, type checking 1.2 s, the collector 5.6 s — is spread over
the workers. Gathering on the pool as well, each worker gathering its own
document's facts into the union before analysis starts, is the step left to
the ~5 s the scaling design sets for this run
([scaling to very large models](../project/large-model-scaling-design.md)).
Before the audits gathered once per batch, each of the 34 analyses gathered
all 34 documents afresh in its own model: 129 s on one job, 30.9 s on eight
at 7.24 GB peak RSS, 118 s of a 128 s serial analysis in the three passes.
`BenchmarkAnalyzeSplitPerDocument` in `tests/stressmodel` measures the
pool's own speedup with the three audits left out, over the split's six files
at 512 satellites: 3.91 s → 1.09 s, one job against eight, the largest file
bounding it. The whole load of the same split, audits included, is
`BenchmarkValidateSplit`: 6.30 s → 2.14 s.

Parallelism does not reduce what a load allocates — the 34-file run allocates
7.6 GiB and 108 million objects at any job count, against the single file's
6.1 GiB and 97 million — and the collector marking eight workers' garbage at
once is where the pool loses efficiency beyond the gather
(`runtime.gcBgMarkWorker` 16.2% and `runtime.scanobject` 17.0% of the
eight-job samples; user time 27.6 s → 32.4 s). The allocation sites the
pool does not help, from the heap profile of an earlier build's single-file 1 600-satellite
run (62 million sampled objects and 4.3 GiB, of a run that counts 85.5 million
allocations and 5.4 GiB), by objects allocated:

| share of objects | site | what allocates |
| ---------------- | ---- | -------------- |
| 12.8% | `passes.(*w9cConflictChecker).specializes` | a slice per conformance question of the inherited-name conflict pass |
| 12.6% | `passes.contributionsOf` | the per-base member list the same pass compares |
| 9.8% | `symbols.FQNOf` (via `strings.Builder`) | a fully-qualified name built as a string, 78% of it from `symbols.(*Index).GetFQN`, 18% from the conflict pass |
| 7.3% | `resolve.(*Resolver).specializationChain` | a slice per walk of a type's generalizations |
| 3.5% | `semantics.(*Model).AllSupertypes` | a slice per supertype closure |
| 2.6% | `parser.(*Parser).parseQualifiedNameRelaxed` | a qualified-name node per reference |
| 1.9% | `parser.(*Parser).parseBase` | a node per specialization clause |

By bytes the parser leads — `parseUsage` and what it calls are 25% of the 5.4
GiB, `parseQualifiedNameRelaxed` alone 5% — with `contributionsOf` (6.6%),
`specializes` (4.9%) and `FQNOf` (4.4%) behind it. Each of these is one
allocation per token, per name or per lookup where one per file, or none, would
serve — the snapshot decoder's node table, allocated as one block, is the
model — and each is to be measured on its own before it is changed.

One parse per load is spent twice: the REPL parses each file to accept it (the
names it declares, whether it closes its own text) and the workspace parses the
same bytes again as the document. The 34 files of the split (17 MB) parse in
1.8 s serially (`repl.preparse` in the one-job profile), so the second parse
is ~1.8 s of the one-job 22.0 s and ~0.3 s of the eight-job wall, where it runs
on the pool. Carrying the accepted tree into the workspace
batch would recover it; it is a change to what `model.Input` owns and is left
to be measured on its own.

## What a process pays before the model

Every `sysml`, `sysml-lsp` and `sysml-grpc` start, and every test that builds a
model, first builds the standard library's index: `libs.SharedBase()` parsed the
97 bundled OMG files (1.7 MB), indexed them, expanded their wildcard imports and
installed derived facts. On the census machine that was the whole cost of
`bin/sysml -memstats -e "2+3" model.sysml` over a one-part model — about 100 ms,
of which a CPU profile put 40 ms in lexing and parsing, 40 ms in
`symbols.(*Index).ExpandWildcardImports` and 10 ms in facts and collection.
Three changes took it to under 20 ms, each measured over the same command
(external wall time is the shell's, around the process; the internal figures are
`-memstats`; five or more runs each, all shown):

| state | wall (external) | wall (`-memstats`) | allocated | allocations |
|---|---|---|---|---|
| before | 95–102 ms | — | 53.3 MiB | 466.9k |
| library files parsed concurrently | 69–72 ms | — | 51.9 MiB | 466.4k |
| plus cheaper wildcard expansion | 66–72 ms | — | 50.6 MiB | 452.5k |
| plus the embedded snapshot | 17–23 ms | 13–17 ms | 32.4 MiB | 67.1k |

- **Concurrent parsing.** `Loader.LoadAll` reads the files in order, hashes and
  parses them on `GOMAXPROCS` goroutines, then adds them to the index in the
  same order as before, so the index is what a serial load builds
  (`TestLoadAllMatchesSerialLoad` compares the two). The content digest the
  facts cache keys on is folded into the same pass.
- **Wildcard expansion.** A namespace's direct children are kept as a sorted
  slice rather than sorted out of a map on every enumeration, a claim is
  returned from the re-export step instead of looked up again, children a
  target declares itself skip the source-key lookup, an import's children share
  one direct-route slice, and subsumed routes are filtered in place.
  `BenchmarkExpandWildcardImports` over the library alone went from
  33.1–34.4 ms, 15.8 MB and 137k allocations per expansion to 30.5–31.9 ms,
  15.0 MB and 123k. The expansion stays inherently iterative — most of its cost
  is the re-export closure itself — which is what the snapshot removes.
- **The snapshot.** The library's frozen index is serialized at generation time
  into `internal/workspace/libs/stdlib.snapshot` (3.4 MB, embedded; the `sysml`
  binary grows from 16.9 to 20.5 MB) and decoded at start-up, so neither the
  parser nor the expansion runs for the library at all. The format is
  hand-rolled (`internal/syntax/pack`, `internal/syntax/ast/astcodec`,
  `symbols.WriteSnapshot`): varints over one string table, a node table per
  syntax-node type so each type's nodes are allocated in one block, and index
  references in place of pointers, so the decoded graph shares what the parsed
  one shares. Decoding (`BenchmarkDecodeSnapshot`) costs 7.8 ms, 32 MB and 64k
  allocations, most of it in reading node fields; the syntax trees, scopes,
  symbols and tables are separate sections decoded on separate goroutines, and
  the collector is paused for the decode, since everything it would mark is live
  for the whole process. Checking that the embedded files still match the
  snapshot's digest (`BenchmarkSetDigest`, SHA-256 over 1.7 MB) is 0.9 ms of the
  remainder; the CRC-32C over the 3.4 MB stream that refuses a damaged blob
  is 0.1 ms.

The snapshot changes nothing observable: `TestSnapshotIndexMatchesFreshLoad`
compares the decoded index with a fresh load structurally — every field, every
pointer, the same sharing between them — and the resolution, completion, REPL
and execution suites run over the decoded library.

## What running a model costs

Runs are measured against an already-loaded model, with the session's runtime
built before measurement starts — a session builds its runtime and indexes the
standard library on its first run, and that one-time cost otherwise swamps the
run being measured.

| elements | start state machine | evaluate calculation | instantiate part def |
| -------- | ------------------- | -------------------- | -------------------- |
| 250      | 12 µs, 5.3 KiB      | 12 µs, 4.5 KiB       | 11 µs, 2.8 KiB       |
| 1 000    | 36 µs, 5.3 KiB      | 36 µs, 4.5 KiB       | 34 µs, 2.8 KiB       |
| 4 000    | 202 µs, 5.3 KiB     | 199 µs, 4.5 KiB      | 198 µs, 2.8 KiB      |

Execution memory is **flat in the size of the model** — a run allocates a few
kilobytes whatever the surrounding model — so memory pressure from executing
models is not where a large model hurts; loading it is.

Execution *time*, however, grows with the size of the surrounding model: starting
the same three-state machine takes about 17 times longer in a 4 000-element model
than in a 250-element one, while allocating exactly the same memory. A CPU profile of
that benchmark is spent in the garbage collector — `runtime.scanobject` and its
neighbors account for over half of it — so the cost is collection scanning the
live model, paid by whatever allocates next, rather than work the run itself does.
What this says about a real workload is that the collector, not the run, is what
grows: a long-lived session over a large model tunes better with `GOGC` than with
a faster executor.

## What the persistent semantic model changes

A `model.Workspace` keeps one `resolve.Resolver` and one `semantics.Model`
for its lifetime, beside its index, and hands them to every `passes.Context`
it builds; a context built outside a workspace still gets fresh ones. The
resolver keeps a frame per document that owns what was memoized while that
document was analyzed, and records which documents each frame read: a
document depends on another when it imports a namespace the other contributes
to, when both contribute to one namespace, or when a resolution from its scope
returned the other's symbol. Replacing a document drops its frame and,
transitively, its dependents' frames — their memo entries, cached diagnostics
and reverse references — and nothing else. The three workspace-wide audits
(OOSEM, MOSA, identity metadata) and the coherent-quantity ranking gather each
document's facts once into the workspace, regather a document when it changes,
and judge each analyzed document over the union.

`TestIncrementalEqualsFresh` replays scripted and seeded random edit sequences
— edits, reverts to earlier versions, closes and opens — over the fixtures and
the four OMG corpora and, after every step, compares diagnostics, resolutions
and reverse references with a workspace built fresh from the same documents.

Measured on the satellite-network generator (`docs/project/satellite-network-stress-test.md`,
"Editing"; Intel Xeon Platinum 8559C, 8 CPUs, 31 GiB, Go 1.25, `-benchtime=5x
-count=3` medians), rebuilt on every edit → kept:

| measurement | rebuilt | kept |
| ----------- | ------- | ---- |
| `BenchmarkEditBeside`, 512 satellites beside a two-line file, per edit | 861 ms, 327 MiB | 8.7 ms, 2.0 MiB |
| `BenchmarkEditBeside`, 128 satellites | 189 ms, 83 MiB | 2.5 ms, 0.64 MiB |
| `BenchmarkEditBeside`, 32 satellites | 48 ms, 22 MiB | 0.82 ms, 0.31 MiB |
| `BenchmarkEditImported`, 512 satellites in 6 files, edit the library then every file's diagnostics | 8.78 s, 2.77 GiB | 5.26 s, 1.17 GiB |
| `BenchmarkLoadFiles`, 512 satellites in 6 files through one workspace | 9.20 s, 3.05 GiB | 5.26 s, 1.56 GiB |
| 1 600 satellites in 34 files through one workspace, open and analyze all | 126.5 s | 18.3 s |
| `BenchmarkLoad`, 512 satellites in one file | 5.13 s, 254 MiB held | 5.99 s, 478 MiB held |
| live heap after 1 000 edits beside 32 satellites, against after the first | — | 67.2 MB → 70.2 MB |

Editing the library every file imports costs one analysis of the whole model,
what loading it costs; the audits no longer gather every document once per
document analyzed, which is the whole of the 34-file difference. What the
model holds between edits nearly doubles — the memo tables that were allocated
and discarded during every analysis now stay — and a thousand edits grow it by
4.5%.

### What the bookkeeping costs a one-shot validation

Every memoized read records that the current document depends on the owner of
the entry it read. That is what makes invalidation sound: an entry keyed by two
symbols of two documents (`composed[(S, T)]`) must go when either changes, and
the reader of a cached answer must be re-analyzed when the answer's owner is
replaced, so the dependency has to be recorded on a hit as well as on a miss.
A validation that will never edit records about 25 million such reads at
roughly 8 ns each for nothing. `sysml -validate -memstats` on the 200-satellite
constellation, one file, three runs each:

| | rebuilt | kept |
| --- | ------- | ---- |
| wall | 1.85–1.96 s | 2.19–2.25 s |
| allocated | 738 MiB in 10.97 M allocations | 767 MiB in 10.98 M allocations |
| peak RSS (`/usr/bin/time`) | 421 MiB | 428 MiB |

At 1 600 satellites: 17.7 s and 5.5 GiB allocated became 20.5 s and 5.8 GiB.
The cost falls on whatever analyzes through the workspace's own context: the
LSP server, and a REPL session's typed submissions. `sysml -validate` and
`%load` analyze each file in a private `passes.Context` — its own resolver and
model over the read-only index, as a pool of workers must — with the audits'
facts gathered once for the batch ("What a batch of files costs"), and pay
none of it.

## Notes for further work

- The `about`-metadata index walks the bundled library's documents once per
  session, which is most of the empty-session floor above. The library is
  immutable once its index is built, so whether it declares any `about` usages
  — and which — is computable once at library-index build time; a session
  would then walk only workspace documents.
- Runs over a large model spend their time in collection, not in the executor
  (above). Reducing what a load leaves behind is the lever, since the live model
  is what each cycle scans.
- Resolving inherited library features in every definition body costs about 1.3x
  a load at 12 000 elements — 0.757s to 1.007s — and the factor grows with the
  model. Earlier notes here put it at 2x; that figure came from a much older
  baseline carrying unrelated changes, so do not repeat it. Allocated *bytes*
  went down (647.5 to 529.7 MiB) while allocation *count* went up (4.54M to
  6.03M), and it is the count the regression follows: parsing is unchanged at
  450ms, resolution goes 90ms to 200ms, the validation passes 130ms to 420ms and
  the collector's mark workers 440ms to 640ms. The work is linear in the model
  and semantically required, so what is left to win is in the scans it walks.
  Hot callers now use non-copying member iteration, while callers that need an
  owned slice continue to use `symbols.Scope.AllMembers`.
- Flattening each type's inherited members into an in-memory table was measured
  and rejected. The table was built by merging a type's direct-supertype tables
  with its own scope, name-indexed so a lookup became one probe, cached per index
  generation for library types only, with a per-symbol plan so a lookup on a user
  type cost one map hit and one probe. Against the same clean models it came out
  a wash: 0.981s to 0.983s at 12 000 elements, allocation count 6,077,232 to
  6,070,375, peak resident mixed (−5.7 MiB at 3 000, −10.2 MiB at 6 000, +5.8 MiB
  at 12 000). The reason is that `semantics.Model.MemberSources` already memoizes
  its breadth-first closure and hits that cache about 92% of the time, so a
  lookup was already close to a single map probe, and the whole inherited-member
  cluster is only ~3.5% of a clean load's allocated objects and ~7% of its CPU —
  parsing and symbol building dominate both. A table therefore relocates that
  work rather than removing it: on a clean 12 000-element model only eight
  library types are reached at all, and building their tables plus the per-type
  plans costs about what the closures cost. Two intermediate shapes were
  distinctly worse — recomputing the contributor set per lookup instead of
  hitting the memoized closure (+1.3M allocations), and rebuilding a table on
  every attempt when a build failed and cached nothing (+286k allocations).
  Do not revisit this without moving the tables off the load: built when the
  library index is built, persisted alongside the other library facts and shared
  by pointer, no table build and no per-type closure happens during an analysis
  at all, which is where the remaining win is.
- Two facts about the bundled library came out of that measurement.
  `Performances::Evaluation` appears among its own contributors, a self-edge the
  breadth-first walk hides by seeding its visited set with the starting symbol,
  and `Calculations::Calculation` and `Constraints::ConstraintCheck` are reached
  through a member that is not itself a library symbol.
- The “small scopes could use a slice instead of a map” lever is closed:
  flat declaration-ordered storage replaces the eager map, and scopes above the
  threshold build a lookup index lazily. The measured scope shape is why the
  threshold matters.
- Recycling parser token buffers across parses was measured and rejected. It
  reduced allocated bytes by only 1.6% while increasing wall time by 3.8% and
  peak RSS by 0.8%. The source-bytes-per-token ratio is 13.8 on the standard
  library but 5.6 on generated models, so no fixed presize divisor serves both;
  do not revisit this lever without new evidence.
