# Execution performance, September 2026

A follow-up to the September 2026 performance census, measuring model
*execution* rather than validation: expression and calculation evaluation
throughput, absolute state-machine and instantiation throughput from the Go
benchmarks, and CPU and allocation profiles of the execution paths.

All figures were taken on one machine — `Intel Xeon Platinum 8559C`, 8 CPUs,
Go 1.25, Linux — at revision `af33e195`. Treat absolute numbers as ratios
elsewhere.

## Method: marginal cost per evaluation

A batch of identical cases is evaluated against the same model
(`calc def Fib` recursive over `if`, `calc def SumTo` linear-recursive, and
the literal `2 + 3`) in one process via repeated `-e` flags on `bin/sysml`.
The per-evaluation figure is the marginal cost between a 1-case and an
N-case invocation of the same process, which cancels process start-up,
parsing, and library loading. Minimum of the repetitions taken.

## Results

| workload | start-up + 1 eval | marginal cost/eval |
| -------- | ----------------- | ------------------ |
| `2 + 3` | 0.11 s | below noise (< 0.1 ms) |
| `Fib(18)` — ~8 400 recursive calc calls | 0.13 s | 12.2 ms |
| `SumTo(150)` — 150 recursive calc calls | 0.11 s | 0.06 ms |

Per calc invocation that is roughly 1.5 µs. Recursion depth is not a
practical limit at these scales: `SumTo(400)` evaluates in the same
sub-millisecond band.

## Absolute throughput: state machines and instantiation

`go test ./internal/frontend/repl -run '^$' -bench 'RunCalc|RunStateMachine|Instantiate' -benchmem -count 6`
over an already-loaded model (start-up excluded, medians):

| figure | 250 elements | 1 000 elements | 4 000 elements |
| ------ | -------------- | -------------- | --------------- |
| state-machine start | 13.6 µs / 9.2 KiB / 120 allocs | 40 µs | 205 µs |
| calc invocation (`Calc0(2.0, 3.0)`) | 12.6 µs / 4.8 KiB / 70 allocs | 36 µs | 220 µs |
| instantiate (`Comp0`) | 11.3 µs / 2.9 KiB / 39 allocs | 36 µs | 225 µs |

## The two optimization gaps the profiles show

**Per-run target lookup is O(model), and dominates.** Allocations per run are
constant across model sizes, yet wall time grows ~16× from 250 to 4 000
elements. The CPU profile of `BenchmarkRunCalc` at 4 000 elements puts 67%
of samples under `repl.collectInScopeTree` (with `symbols.Scope.LookupLocalAll`
at 25%): every `RunCalc`/`RunStateMachine`/`InstantiateNamed` re-walks the
whole document scope tree to find its target by simple name, even though the
session already holds a qualified-name index (`browseIndex().LookupQualified`
is consulted only for qualified names). Resolving simple names through the
index — or memoizing the tree walk per (document-version, name) — would make
a run's cost independent of model size. The runtime itself is cheap: the
size-independent floor is ~1–3 µs.

*Closed.* The session now tabulates the simple names of its documents once per
scope tree (`repl.nameTable`) and answers lookups from the table, rebuilt only
when a submission or reset replaces a document. Re-running the benchmark above
(same machine, `-count 6`, benchstat): state-machine start 6.4 µs, calc
invocation 4.5 µs, instantiate 3.3 µs — the same at 250, 1 000 and 4 000
elements, a 97–98% drop at 4 000. `collectInScopeTree` no longer appears in
the profile; the table's own cost is under 1% of samples.

**The evaluator allocates ~1.7 KiB per calc invocation, and GC eats half the
CPU.** Profiling 20 × `Fib(20)` (~440 000 calc invocations), 45% of CPU is
garbage collection, and the allocation profile puts 59% of all bytes in
`runtime.Context.bindCalcParameters`, with `NewEvalContext` (8%) and
`newStmtEngine` (3%) adding a fresh context and statement engine per
invocation. Reusing evaluation frames across calls — a free-list on the
`Context`, or binding parameters into a caller-provided frame instead of a
fresh map — is the standard fix and would directly lift recursive-calc
throughput. At ~1.5 µs per invocation this is not currently a user-visible
problem; it is the first thing to reach for if execution-bound models appear.

## Assessment

Execution is fast in absolute terms — microseconds per calc invocation or
state-machine start, sub-millisecond for deep recursive calculations — and
the two gaps the profiles show are internal and localized: an O(model)
per-run name lookup in the REPL session layer, and per-invocation allocation
in the calc evaluator. Both are fixable without architectural change.

## Follow-up: calc invocation frames are reused

Measured on the same machine, same method, after the calc evaluator began
keeping the frame an invocation runs in — parameter bindings, evaluation
context, statement engine — on a free list of the `Context` and reusing it for
the next invocation once the first has returned:

| figure | before | after |
| ------ | ------ | ----- |
| marginal cost per `Fib(18)` eval (8 361 calc invocations) | 13.1 ms | 8.9 ms |
| marginal cost per `Fib(20)` eval (21 891 calc invocations) | 33.1 ms | 24.6 ms |
| bytes allocated per calc invocation (`Fib(20)` marginal) | ~1.7 KiB | ~130 B |
| GC share of CPU, 20 × `Fib(20)` | 50% | 17% |
| GC cycles, 20 × `Fib(20)` | 34 | 11 |

Per calc invocation that is roughly 1.1 µs. `bindCalcParameters` no longer
appears in the allocation profile; what remains per invocation is the
positional argument slice of `evalInvocation` and the boxed values themselves.

## Follow-up (2026-09-02): the evaluator's per-invocation overhead

Measured on the same machine, same method, at revision `d14896bc` before and
after four changes to the calc evaluator, made one at a time and measured
after each: `runtime.Value` shrank from 120 to 64 bytes (the scalar stays
inline; string, sequence, set, expression, quantity and symbol payloads share
one slot); parsed literals and resolved invocation targets are memoized per
`Context` keyed by the AST node, which a REPL or LSP edit replaces along with
the context; a calc's parameters bind into slot-indexed frames resolved once
per calc shape, and a bare name outside a body-local scope is answered from
the frames before the general resolution chain runs; and the positional
argument slice and the statement engine's frame stack are borrowed from
per-context storage instead of allocated per call. The benchmark is `Fib(25)`
(242 785 calc invocations), marginal between a 1- and a 6-case run.

| figure | before | after |
| ------ | ------ | ----- |
| marginal cost per `Fib(25)` eval | 246 ms | 158 ms |
| per calc invocation | 1.01 µs | 0.65 µs |
| allocations per `Fib(25)` eval (`-memstats` marginal) | 971 000 (4.0 per invocation) | ~160 (< 0.001 per invocation) |
| bytes allocated per `Fib(25)` eval | 38.9 MiB (168 B per invocation) | ~0.1 MiB |
| `BenchmarkRunCalc` (`-count 6`, geomean) | 2.15 µs / 888 B / 26 allocs | 1.88 µs / 768 B / 25 allocs |

Stage by stage, per calc invocation: 1.01 µs → 0.83 µs (smaller `Value`) →
0.65 µs (memoized literals and targets) → 0.65 µs (slot frames; the map
lookups it removed were already cheap once the target was memoized) →
0.65 µs (allocation trimming; the young garbage it removed was cheap to
collect). Every result, error message, trace and step count is unchanged; the
execution-trace goldens and the pilot differential's bucket counts are as
committed.

The profile of 3 × `Fib(27)` is now flat. Before, 26% of CPU was `duffcopy`
and 7% `duffzero` (the 120-byte `Value` returned through ten frames per
operation), 12% garbage collection and 7% string-keyed map access. After, no
function exceeds 12% flat: `EvalContext.Eval` 12%, `eval` 9%,
`evalFeatureReference` 7%, `evalOperator` 6%, `evalArithmetic` 5%,
`evalName` 5%, `duffcopy` 4%, `runCalcBody` 4%, `incrementStep` 3%,
garbage collection 2%. What remains is the tree walk itself: about twelve
`Eval` dispatches per `Fib` invocation, each returning a 64-byte `Value` and
an error through three or four frames, plus the invocation's own bookkeeping
(frame acquisition, activation, parameter binding and type checking) at
roughly 15% of the total.

That is a 1.55× improvement per invocation against the 2× this pass aimed for.
Closing the rest within the tree walk would mean flattening the
`(Value, error)` return chain into an out-parameter across every `eval*`
function and shrinking `semantics.Value` — a wide, mechanical change — and
the calc *usage* environment (`bindCalcUsage`) still binds into a map, as its
statements read and write outputs by name. The larger step, a compiled fast
path for pure calc bodies, is the planned follow-up and is out of scope here.

## Follow-up (2026-09-02): pure calc bodies compile to a closure fast path

Measured on the same machine, same method, at revision `8e4c84f0` (the
evaluator above) before and after the compiled calc tier
(`internal/exec/runtime/compile.go`, `compiled_ops.go`). On its first
invocation a calc definition whose body is one expression — a lone `return`,
or a bound result over an otherwise empty body — over Integer, Real and
Boolean literals, its own `in` parameters, the arithmetic,
comparison, equality, identity, logical and conditional operators, and
invocations of other such calcs is compiled once per `Context` into a tree of
Go closures over an unboxed scalar frame: parameters are slot indexes into a
per-context scalar stack, callees are resolved at compile time (a cycle through
a lazily filled cell), and arguments are unboxed and the result boxed only at
the invocation boundary. Everything else — calc usages and `out` features,
feature chains, `self`/`that`/`this`, collections, quantities, enumerations,
strings, body-local declarations, non-literal defaults, a parameter typed
outside the scalar lattice or redeclared along the specialization chain —
leaves the calc on the reference evaluator, as does
any caller of such a calc; a traced context, a named-argument or non-scalar
call, and `OPENSYSML_CALC_COMPILE=0` take the reference path for the whole
invocation. The compiled body charges the step budget per node exactly as the
evaluator does (a branch not taken is not charged), so `OPENSYSML_MAX_STEPS`
and `OPENSYSML_MAX_CALC_DEPTH` trigger at the same invocation with the same
error. The benchmark is again `Fib(25)` (242 785 calc invocations), marginal
between a 1- and a 6-case run, minimum of five repetitions.

| figure | before (evaluator) | after (compiled) |
| ------ | ------------------ | ---------------- |
| marginal cost per `Fib(25)` eval | 126–129 ms | 5.1–5.4 ms |
| per calc invocation | 519–532 ns | 21–22 ns |
| CPython 3.12 `fib(25)`, same machine, min of 5 | 27.2 ns per call | — |
| allocations per `Fib(25)` eval (`-memstats` marginal) | ~159 (< 0.001 per invocation) | ~156 (< 0.001 per invocation) |
| `BenchmarkRunCalc` (`-count 6`, geomean) | 1.88 µs / 768 B / 25 allocs | 1.40 µs / 768 B / 25 allocs |

That is a 24× improvement per invocation, past CPython on the same function
(the ~1–2 ns of V8 and HotSpot are the domain of a JIT, out of reach of a
closure tree). The `RunCalc` benchmark moves 26%: its `Calc0(2.0, 3.0)` is a
single invocation, so what it measures is the REPL's parse, resolve and format
path around it, which is unchanged.

The CPU profile of `Fib(33)` (11.4 million invocations, 290 ms of samples) is
the closure tree and little else: `compiledCalc.invoke` 38% flat (arity,
argument unboxing, the frame push, the depth budget), the `+` node 10%, the
leaf pair `k - 1`/`k - 2` 10%, `chargeSteps` 7%, the invocation node 7%, the
parameter type check 7%, the `if` node 3%. The garbage collector's 10% is spent
over the start-up heap: the compiled path allocates nothing per invocation.

Compliance was checked three ways. The differential test
(`compile_differential_test.go`) compiles every eligible calc definition in the
repository's fixtures, examples and the OMG corpora — 42 eligible of 263 calc
definitions across 739 files, 16.0%; the rest have statement bodies, `out`
features, parameters typed outside the scalar lattice or redeclared, or call
library functions — and invokes each through both tiers over generated
Integer, Real and Boolean argument vectors including 0, ±1 and the Integer
extremes, asserting identical values or identical errors and identical step
counts: 8 920 invocations, none differing. The execution conformance,
trace and robustness suites pass with the tier on and with
`OPENSYSML_CALC_COMPILE=0`. The pilot execution referee's bucket counts are as
committed.

## Follow-up (2026-09-02): statement bodies, redefined parameters, library intrinsics, named arguments

Measured on the same machine, same method, at revision `1d98f6e5` (the tier
above) before and after widening the tier's subset
(`compile.go`, `compiled_ops.go`, the new `compiled_stmts.go`). Four
constructs that analysis models write joined the subset:

- **Statement bodies.** A body of body-local scalar declarations, `return`
  and `if`/`else` statements is compiled from the lowered `calcShape.Steps`
  into the same closure tree: each declaration takes a fresh frame slot at
  compile time and masks an earlier binding of its name for the rest of its
  block, so a local named like a parameter, a later local reading an earlier
  one, and a local declared in one branch alone read exactly as
  `stmtEngine`/`LookupBodyLocal` read them; a name read before its
  declaration, a local without a value, a body that may run off its end, an
  `out` feature, a loop, an `assign`, a non-scalar local stay on the
  evaluator. A body that is one `return` compiles to the expression itself,
  as before, so the pure-expression path is unchanged.
- **Parameters redefined along the specialization chain.** The compiled slot
  layout is the effective parameter list `calcShape` flattens (a `:>>`
  redefinition masks its base's slot in place), the same list
  `bindCalcParameters` binds, so a 3-level chain, a redefinition adding a
  default, a qualified `in x :>> Base::x` all compile; a Real→Integer
  narrowing compiles and refuses a non-integral Real with the evaluator's
  own message, since parameter checks are the declaration's.
- **Library intrinsics.** A call whose resolved symbol *is* a standard
  library scalar function (`RealFunctions::sqrt`/`abs`/`floor`/`round`/
  `min`/`max`, the `IntegerFunctions`, `NaturalFunctions`,
  `RationalFunctions` and `NumericalFunctions` members, `TrigFunctions::sin`/
  `cos`/`tan`/`cot`/`arcsin`/`arccos`/`arctan`/`deg`/`rad`,
  `OpenSysMLMathFunctions::exp`/`ln`/`log`/`atan2`) dispatches to the Go
  implementation the evaluator's `libraryFunction` table already holds — the
  same function, so values are bit-identical — and a library constant read
  (`TrigFunctions::pi`; the library declares no `e`) is folded to the value
  the evaluator's library seam supplies. Resolution is by symbol, so an
  alias, an import or a qualified name reach the intrinsic and a model's own
  `sqrt` or `pi` is an ordinary calc or attribute. `sum`/`product` and the
  other collection functions over a lone scalar run the evaluator's
  collection built-in over a one-element sequence; a call over a collection,
  a String or a vector keeps the evaluator.
- **Named arguments.** `Fib(k = n - 1)` binds to slots at compile time,
  arguments evaluated in source order, a name given twice taking the later
  value as the evaluator's map does; a call mixing positional and named
  arguments, or missing an argument for a parameter without a default,
  declines; an unknown name and a receiver beside named arguments are
  reported by the evaluator's argument check that precedes dispatch, so the
  message is the evaluator's verbatim.

The benchmarks are `Fib(25)` again (242 785 invocations, no statement body,
to show the pure-expression path is unchanged) and `HypotTree(16)`: a
recursive tree of 65 535 invocations whose leaves call `Hypot`, a three-local
statement body (`aa = a * a; bb = b * b; h = sqrt(aa + bb); return h;`)
65 536 times — 131 071 invocations, half of them statement bodies calling an
intrinsic. Before, `Hypot` was ineligible (statement body, library callee),
so the whole tree ran on the evaluator.

| figure | before | after |
| ------ | ------ | ----- |
| marginal cost per `Fib(25)` eval | 5.0–5.8 ms (compiled) | 5.0–5.5 ms (compiled) |
| per `Fib` invocation | 21–24 ns | 21–23 ns |
| marginal cost per `HypotTree(16)` eval | 132–133 ms (evaluator) | 10.4–10.8 ms (compiled) |
| per `HypotTree(16)` invocation, averaged | ~1 010 ns | ~80 ns |
| `HypotTree(16)` with `OPENSYSML_CALC_COMPILE=0` | 132–133 ms | 132–133 ms |

The `Fib` figures are within run-to-run noise of each other. `HypotTree`
improves 12.6×; the remaining ~137 ns per `Hypot` is the intrinsic call,
which boxes its argument into the evaluator's library-call path and back
(the closure tree, a slot write per local, and the `+`/`*` nodes are the
same ~20 ns as `Fib`). Unboxing the library table is the natural next step
and is out of scope here.

Eligibility over the repository's fixtures, examples and the OMG corpora
(`go test -run TestCompiledCalcDifferential -v ./internal/exec/runtime`),
before → after: 42 eligible of 263 calc definitions (16.0%) → 127 of 343
(37.0%), the new fixtures under `internal/exec/runtime/testdata/compiled/`
included; 84 449 invocations compared through both tiers over the generated
vectors — 0, ±1, the Integer extremes, ±0.0, 1.5, −1e300, ±Inf, NaN, true
and false — positionally and by name, none differing in value (Reals
compared bit for bit), error text or step count. What remains ineligible,
and why:

```
   67 parameter _ declares a type outside the scalar lattice   quantities with units, strings, collections
   65 output feature _ beside the result                       calcs with out features (bound by the usage)
   27 *ast.SequenceExpr is outside the pure subset            collections
    9 statement _ is outside the compiled subset              loops, assignments, sends
    5 *ast.FeatureChainExpr / 5 *ast.LiteralString / 1 IndexExpr / 1 ConstructorExpr
    5 name _ is not bound in the frame                        self, that, a calc usage, a read before declaration
    3 result declares a type outside the scalar lattice
   14 library function … is not over scalars alone            String, Vector, Complex, Sequence and Control functions
    2 library feature _ is not a scalar / 1 has no value      vector constants
    2 local _ declares no value · 2 body without a result expression · 1 body may end without returning
    1 default of _ is not a literal · 1 operator _ is outside the pure subset · 1 called without an argument for _
```

Compliance was checked as before: the execution conformance, trace,
robustness and REPL suites pass with the tier on and with
`OPENSYSML_CALC_COMPILE=0`; the pilot execution referee's bucket counts are as
committed; the training and pilot corpus gates and the SMT suite are
unchanged.
