---
name: testing-element-filters
description: How to observe SysML element filters (`filter <expr>;`, `import P::*[@T]`, `expose X::**[@T]`) end to end through the sysml REPL/CLI — which surfaces actually apply a filter, how to build a library-backed model so the on-disk index cache is exercised, and the fixture shapes that avoid false negatives.
---

# Testing element filters through the CLI/REPL

## The only observable surface is diagnostics, not `%instantiate`

`internal/frontend/repl/lookup.go` resolves a qualified name with `idx.LookupQualified` and a simple name with
`resolve.New(idx).LookupName` **without** `SetModel`, so no filter is ever evaluated on that path:
`%instantiate Facade::hiddenPart` succeeds even for an element a filter rejects. Do not read that as
"filters do not work".

What *does* apply filters is document analysis (`passes/pass.go` calls `Resolver().SetModel`). So the
test surface is the **unresolved-reference diagnostics** the REPL prints when a document is loaded:

```
package Client { private import Facade::*; part b :> hiddenPart; }   // -> error: unresolved reference: hiddenPart
package Client { part c :> Facade::hiddenPart; }                     // -> the qualified route
```

Fixture shapes that matter (copy them; the alternatives produce misleading results):

- Reference an imported *usage* with subsetting `part x :> seatBelt;`. Typing it (`part x : seatBelt;`)
  fails for an unrelated reason (`type must be a definition, found partUsage`).
- Prefer the metadata type **qualified** in the condition (`filter @Meta::Safety;`) when you want a
  known-good control. An unqualified `@Safety` reachable only through `public import Meta::*` used to
  be reported `filter-not-evaluable: the metadata type Safety does not resolve` (filter then **not
  applied**, so nothing is hidden and the test silently passes for the wrong reason). That path is
  expected to work now (a namespace's own filter must not gate lookups inside its own body), so
  assert both spellings and treat a `does not resolve` warning as a regression.
- A view's `expose` only surfaces names **inside the view body**; `import SomeView::*` from outside
  surfaces nothing at all, filtered or not. Put the `expose` and the `part a :> x;` references in the
  same `view { }`.
- A recursive `import X::**` does not surface grandchildren (members of a nested package/part) even
  unfiltered, so a "filtered recursive import hides X" assertion needs an unfiltered control run or
  it proves nothing.
- **Gating is per-filter-condition, not per-document** (`ElementFilterPass` runs at `LevelType` but
  declares `passes.ElementScoped`; wave 12A). An unrelated earlier-level error elsewhere in the file
  no longer suppresses `filter-not-boolean` / `filter-not-evaluable`. Only the filter's *own*
  classification `TypeRef` being unresolved gates it. So a fixture like
  `alias Bad for P::Missing;` + `filter 1 + 2;` **does** now report `Must have a Boolean result`.
  If you are testing on an older commit the document-wide behaviour applies instead — when a filter
  diagnostic mysteriously fails to appear, check which side of that change you are on by running the
  same fixture through a binary built from both commits. Don't assert "absent" without that control.
- Corollary: a filter whose condition names an **unresolvable** metadata type (`filter @NoSuchType;`)
  still produces only the name-resolution error (`unresolved reference: NoSuchType`) and *no*
  `filter-not-evaluable` — that specific suppression is the point of the per-element gate and holds
  for `@`, `@@`, `istype`, `hastype`, recursed through boolean composition. To assert the
  `filter-not-evaluable` warning, use a condition that resolves but is outside the supported subset
  (`filter @Meta::Safety + 1;`).
- **An operator that is Boolean whatever its operands are gates on any unresolved operand,** not just
  classification `TypeRef`s — the logical set (`not`, `and`, `or`, `xor`, `implies`) *and* the
  comparisons (`==`, `!=`, `===`, `!==`, `<`, `<=`, `>`, `>=`), and for `istype`/`hastype` the tested
  operand as well as the named type. So `filter @Safe and Undefined;`,
  `filter Undefined1 and Undefined2;`, `filter Undefined == 1;` and
  `filter Undefined istype Safe;` yield only the
  unresolved-reference error, matching the pinned pilot, which reports nothing at that line (its
  `Must have a Boolean result` is satisfied by the top-level operator). A *bare*
  `filter Undefined;` still reports — the pilot reports there too. These shapes are absent from the
  four OMG corpora, so the differential's only-ours count will not catch a regression here; probe
  them with hand-written fixtures plus
  `build/pilot-sysml-validator/validate-sysml-batch --root <dir> <file>` as the referee.
- **Syntactically malformed conditions are a distinct risk class, and the cheapest bug to reintroduce.**
  A classification test with no type (`filter x istype ;`, `x hastype ;`, `@;`, `@@;`) parses to an
  `*ast.OperatorExpr` whose `TypeRef` is a **typed nil** `*ast.QualifiedName`. `DownstreamOfFailure`
  only guards `ref == nil`, which a typed nil passes, so it reaches `ref.Span()` and panics — the
  `op.TypeRef == nil` arm of `gated()` (which treats the missing type as its own resolution failure,
  so the syntax error stands alone as the pilot has it) is the only thing preventing it. To prove
  that arm is load-bearing rather than decorative, delete it, rebuild, and check all four shapes
  panic; a plain `go test` alone will not tell you it matters. Note `gated()` also hands raw `ast.Node`
  interface values from `op.Operands` (and `f.Expr`) to `DownstreamOfFailure`/`gated` *without* that
  guard, so if the parser ever starts emitting typed-nil operands instead of `ErrorNode`s the same
  panic returns by a different door — sweep `filter x == ;`, `filter and x;`, `filter not ;`,
  `filter ((@)) ;` and friends after any parser change.
- Malformed shapes whose *operand* the parser recovered as an `ErrorNode` (`filter x == ;`,
  `filter and x;`) are still judged, so they can emit `Must be model-level evaluable` on a document
  whose only other fault is a parse error, where the pilot reports just the syntax error. The pilot
  does corroborate the well-formed-but-non-Boolean cases (`filter ;`, `filter not ;`,
  `filter x + ;`). Treat the rest as a known divergence confined to already-invalid documents, and
  referee any new one with the pilot rather than assuming it is fine.
- The gate's `default: return false` arm means operators whose result is **not** Boolean regardless
  (notably arithmetic `+`) do *not* gate on unresolved operands, so `filter Undefined + 1;` still
  reports — and the pilot reports there too, so that is correct, not noise. Don't "fix" it.
- Metadata annotations gate the same per-annotation way (`MetadataTypePass`), and unlike the filter
  pass its `pm.Type` is a concrete `*ast.QualifiedName` guarded by `pm.Type == nil`, so it has no
  typed-nil exposure; malformed annotations (`#;`, `# part p;`, `#(;`) are safe. Note our
  `Must have a concrete type` is emitted at the **file start (`1:1`)** rather than at the annotation
  line, where the pilot puts it. That mis-attribution predates wave 12A — don't score it as a
  regression, but don't key an assertion on the line number either; assert presence/absence by
  toggling the annotation in and out of the fixture.
- Filters at a document's **root** (no enclosing package) are a separate code path: both
  `import Lib::*[@Lib::Safety];` and a bare `filter @Lib::Safety;` written at file top level gate the
  resolver's top-level fallback (`resolve/qualified.go lookupGlobalTop`). Root filters are **per
  document**, so proving that needs a *second* document — and `%load` does not give you one (it just
  submits text into the single REPL session buffer). Supply the second document as a library file
  under `OPENSYSML_LIBRARY_PATH` (e.g. `$LIB/Roots/other.sysml` containing its own root
  `import Lib::*;`), then assert both directions: the other document's unfiltered root import must
  not defeat the session document's root filter, and the other document's root filter must not
  restrict the session document.
- A typo inside an import's filter clause (`import Lib::*[@NoSuchMeta]`) is reported as
  `unresolved reference: NoSuchMeta`; check for exactly one such diagnostic (no duplicate) and that a
  valid clause contributes none of its own.
- Strongest regression technique: build the **previous commit** into a second binary
  (`git worktree add /tmp/wtN <sha> && go build -o /tmp/sysml-<sha> ./cmd/sysml`), run the whole
  fixture sweep through both binaries and `diff` the outputs. An empty diff proves no diagnostic was
  gained or lost, and the old binary doubles as before/after evidence for the fix under test.
- A `filter` member declared in a **definition or usage body** (`view v { expose X::*; filter @T; ... }`,
  and the same in a `part def` / `part usage` / `package` body) restricts what the imports declared
  beside it bring into that body. Test all of `expose X::*`, `expose X::**` and plain
  `private import X::*`, each with an unfiltered twin as the control, and reference one annotated and
  one unannotated name in each body.

Expected diagnostic text:

```
filter 3;                 -> error:   a filter condition must be boolean-valued, but this one yields an integer (KerML 8.2.4)
filter @Meta::Safety + 1; -> warning: this filter condition cannot be evaluated, so it selects nothing and is not applied: the `+` operator is not supported in a filter condition
```

## Exercising the on-disk index cache (cache-hit vs cache-miss equivalence)

A REPL session document is never cached; only *library* files are. Make the model a library:

```bash
cp -r internal/workspace/libs/stdlib/* /tmp/flib/          # OPENSYSML_LIBRARY_PATH REPLACES the stdlib, so copy it
mkdir -p /tmp/flib/Filters && cp model.sysml /tmp/flib/Filters/
export OPENSYSML_LIBRARY_PATH=/tmp/flib XDG_CACHE_HOME=/tmp/fc
rm -rf /tmp/fc                                        # run 1 = cache miss (parsed), run 2+ = cache hit (restored)
printf '%%load /tmp/client.sysml\n%%quit\n' | timeout 180 ./bin/sysml
```

- **Pitfall:** `VAR=x printf ... | ./bin/sysml` sets the variable for `printf` only. `export` it, or
  the run silently uses the embedded stdlib and your library is absent (`symbol "Lib::X" not found`).
- Cache files land in `$XDG_CACHE_HOME/sysml-ls/libs/*.idx` (~96 files for the stdlib); the key
  includes the format version, so a bump makes old entries unreachable rather than stale.
- Compare runs by grepping just the verdicts, which keeps a diff readable:
  `... | grep -o "unresolved reference: [A-Za-z:]*" | sort | uniq -c`
- Two things are known to differ between a parsed and a restored library and are worth asserting
  every time: whether a **qualified** name into a filtered namespace resolves, and whether a
  *nested* facade (`Outer::Facade::x`, i.e. a 3+-segment qualified name) resolves at all. A restored
  index has populated scopes and a parsed one may not, and only one of the two routes applies the
  filter gate. Run at least three runs (miss, hit, hit) and A/B against a `main` binary built in a
  worktree: a nested-qualified difference between run 1 and run 2 also reproduces on `main`.
- **Also diff the *whole* diagnostic list, not just the filter verdicts.** A cache-miss run can emit
  an extra diagnostic whose span belongs to a library file but is printed against the user's document
  (e.g. `18:22937: error: unresolved reference: Feature` on a 17-line file). Sanity check: any
  reported line/column beyond `wc -l` of the loaded file is a misattributed library span. That leak
  was fixed (`resolve.Resolver.aside`: a lookup made for a semantic query does not report) and is
  locked by `model/element_filter_test.go` `TestFilterEvaluationReportsOnlyThisDocument`, but the
  whole-list diff is still the check that catches its return.

## Driving a filter being added and removed without an LSP client

The REPL re-analyses the **whole buffer** on every submission and a submission redeclaring an
existing name replaces the earlier one, so filter invalidation is testable at the prompt:

```
./bin/sysml -debug
package Facade { public import V::vehicle::*; }                        # no filter
package C1 { private import Facade::*; part x :> keylessEntry; }       # resolves
package Facade { public import V::vehicle::*; filter @Meta::Safety; }  # add filter
                                     # -> C1 (earlier line) now errors: routes re-derived
package Facade { public import V::vehicle::*; }                        # remove filter
                                     # -> 0 diagnostics over the whole buffer again
```

`-debug` prints `submission at buffer line N; K diagnostic(s) over the whole buffer`, which is the
clearest evidence that the retroactive re-derivation happened.

## `@` / `@@` classification as an *expression* (PR #209 onward)

`@`/`@@` are also evaluable in ordinary expressions (`%eval`, constraint/calc bodies), and the claim
under test is that they answer with the *same* predicate an element filter uses. Test the two paths
side by side on **one model**, and expect them to be able to disagree:

```
./bin/sysml /tmp/filt.sysml -validate     # filter path: `import Lib::*[@Meta::Safety]` must leave
                                          # `part b :> Radio;` unresolved and `:> Belt` clean
./bin/sysml            # then: %load /tmp/filt.sysml ; %eval Lib::Belt @ Meta::Safety
```

- **`-eval` refuses a model that does not analyse cleanly** (`did not analyse cleanly; no check was
  made`, exit 2). Since the filter fixture *deliberately* carries an unresolved reference, the
  agreement check has to go through the REPL's `%load` + `%eval`, not `<model> -eval '…'`.
- **Where the element is declared can change the answer, so run every case twice.** Once with the
  model `%load`ed, once with the same fixture under `OPENSYSML_LIBRARY_PATH=/tmp/flib` +
  `XDG_CACHE_HOME=/tmp/fc` (stdlib copied in alongside). A green library run is not evidence for the
  load path, and vice versa: the REPL reindexes on each submission, so one element exists as a
  distinct `*symbols.Symbol` per generation, and identity-based comparison of annotation types made
  annotations evaluate `false` in the loaded-document path while the restored-library path was
  `true` (fixed at f01b92f by `indexedElement` + name-based `conformsByName` fallback in
  `semantics/filter.go`; locked by `TestClassificationOfASubjectFromAnotherIndexGeneration`).
  The cheap canary for a regression of that class: after `%load`, submit two unrelated packages
  (`package extra { part def Later; }`) to force reindexing, then re-run the same `@` expression —
  it must still be `true`. Check subtype conformance in **both directions** too
  (`airBag @ CrashSafety` true, `seatBelt @ CrashSafety` false), since a name-based fallback that
  ignores direction would pass the positive case alone.
- **`go test -run TestExecutionConformance ./internal/exec/runtime` can pass while the binary fails
  the same fixture.** Conformance builds its own index; the REPL/CLI path does not. Always re-run
  conformance fixtures through the binary:
  `./bin/sysml <fixture> -instantiate test::Vehicle -constraint test::Vehicle::tagged`.
- **Contrast with the parent commit.** Before classification-as-expression existed the binary said
  `error: unsupported operator: '@': classification needs the runtime type of a value…`. A wrong
  `= false` on the new binary is therefore a *reported error turned into a silent wrong answer* —
  the strongest framing for the report, so build `/tmp/old-sysml` from the parent first.
- **Failure modes to cover** (all must be `error: filter condition cannot be evaluated: …`, never
  `= false`): `42 @ T` (`a constant denotes no element to classify`), `"s" @ T` (`a string …`),
  `x @ NoSuch` (`the metadata type … does not resolve`), bare `@ T` (`` `@` leaves its subject
  implicit and no object is being evaluated``). Garbage (`%eval @@`, `x @`, `x @@@ y`, `(@T`,
  `x @ 42`) must be parse errors; follow with `%eval 1 + 1` → `= 2` to prove the session survived.
- **Session-poisoning canary:** submit one garbage line (`oops garbage`) *after* a `%load`, then
  `%eval 1 + 1` (works) and `%eval X @ T` (observed: `error: parse failed: expected a namespace
  member`, permanently, for every later `@`/`@@`). The classification path re-parses session text
  that plain arithmetic does not, so run this ordering explicitly — a clean piped run without a
  preceding `%load` will *not* reproduce it.

## Views: `ExposedElements` / `NestedViews` (Go API only, no `%view` command)

There is no REPL command for a view's exposed set, so drive `semantics.Model` from a throwaway test
package (`internal/zzprobe/views_test.go`, deleted afterwards): `symbols.NewIndex()` →
`model.LoadStdlibInto` → `idx.AddDocument` → `idx.ExpandWildcardImports()` → `resolve.New` →
`semantics.NewModel` → `r.SetModel(m)` → `r.ResolveDocument`, then assert per view. Enumerate
*named fixture FQNs*, never `idx.FQNs()` — the latter drowns the output in stdlib views.

Fixture shapes worth having in one file: `expose Lib::*`, `expose Lib::**`, `expose Lib::*[@Meta::T]`,
`expose Lib::**[@Meta::T]`, a single-membership `expose Lib::Belt`, a body-less `view emptyView;`, a
view with a nested view, a `view viewOfView { expose V::outerView::**; }`, and a non-view `part def`.
Observed contract at bcc84c0: recursive expose includes the **namespace itself** (`Lib`) plus
grandchildren; a `private import` inside `Lib` still leaks its target (`Safety`) into `Lib::*` — and
the REPL agrees, so check body resolution (`view probeA { expose Lib::*; part y :> Radio; }` clean vs
the filtered `probeB` rejecting only `Radio`) before calling such a leak a bug. Non-views, packages,
metadata defs and `nil` must all give `ErrNotAView` from *both* APIs; an empty view is `[]`, nil error.
