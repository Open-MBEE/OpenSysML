# Native compilation of calcs

`sysml -compile` translates a `calc def` (or a calc usage) into a standalone native executable,
ahead of time, through C or Go. The interpreter in `internal/core/runtime` stays the reference
semantics: a compiled program computes what `sysml -calc` computes, prints it the same way, and
fails on the same inputs — or the calc refuses to compile with a typed error saying which construct
is outside the subset. Nothing is compiled approximately.

This is a justification spike: it measures whether a native backend earns its place and, in
particular, whether the C toolchain dependency earns its place over pure Go. The numbers are in
[Measured results](#measured-results); the verdict is that it does, by a wide margin.

## Usage

```
sysml model.sysml -compile Pkg::Fib -o fib              # C, via cc -O3 -flto (default)
sysml model.sysml -compile Pkg::Fib -target go -o fib   # Go, via the Go toolchain
sysml model.sysml -compile Pkg::Fib -source -o fib.c    # write the generated source only

./fib 20             # 6765
./fib --repeat 100 20   # run 100 times, print once (for timing)
```

The executable takes the calc's parameters as command-line arguments, positionally, and prints
the result on one line in the interpreter's notation (`6765`, `1.75`, `2.0`, `1e21`, `true`). An
input the interpreter would reject — Integer overflow, division or modulo by zero, a non-finite
Real, a Real argument written outside the Real range (`1e400`, or `1e-400` underflowing to
zero), recursion past the calc depth budget — exits with status 1 and the reason on stderr; an
argument that is not the notation of its type at all (`inf`, `nan`, a hexadecimal `0x1p-2`,
`1_000.5`) exits with status 2 — a Real argument is decimal notation only, as the interpreter's
literals and `ToReal` are.

The generated source is always written beside the executable (`fib.c` / `fib.go`), so what was
compiled is inspectable. `OPENSYSML_CC` names the C compiler (default `cc`) and `OPENSYSML_GO`
the go command (default `go`).

Programmatically: `Session.CompileCalc(name)` in `internal/repl` yields a `codegen.Program`, which
`codegen.Source` renders and `codegen.Build` compiles.

## The compiled subset

A calc compiles when everything it reaches is in this subset:

| Construct | Compiled as |
|---|---|
| `in` parameters typed `Integer`, `Natural`, `Positive`, `Real`/`Rational`, `Boolean`, with no multiplicity or `[1]` | `int64_t` / `double` / `bool` |
| The same types with any multiplicity (`[0..*]`, `[2..3]`, `[0..1]`, …), as parameters, results and body-local attributes | a sequence of the element type with its shape (null, one value, many); the bounds are checked where the interpreter checks them |
| Result: the body's trailing expression, or `return : T = <expr>;` | function result |
| `attribute x : T;` with no value | null, until assigned |
| `(a, b, …)`, `()`, `null`, `lo..hi`, `s#(i)`, `??`, `==`/`!=` and `===`/`!==` over sequences | sequence literals (nested ones flatten, null contributes nothing), inclusive ranges, one-based indexing, coalescing, elementwise and identity comparison |
| `for v in s { … }` | a loop over the elements |
| `CollectionFunctions`/`SequenceFunctions` `size isEmpty notEmpty head tail last contains containsAll includes includesOnly excludes including includingAt excluding excludingAt subsequence union intersection equals same`; `ControlFunctions` `allTrue anyTrue select reject selectOne collect forAll exists reduce minimize maximize` with `{in v; …}` bodies; `sum product` in the numeric libraries | the collection runtime in the prelude, with the interpreter's index, multiplicity and element-budget errors |
| Literals; parameter and body-local attribute references | as written |
| `+ - * / % **`, unary `-`, comparison, `== !=`, `and or xor not`, `implies`, `if c ? a else b` | checked native operations |
| `attribute x : T = e;`, `x = e;` / `assign x := e;` | locals and stores |
| `if` / `else`, `while … [until]`, `loop { … } until` | control flow |
| Invocation of another compilable calc, positional or named; direct and mutual recursion | native call |
| `calc c : D;`, `calc def E :> D;` adding no member of its own | compiles as `D` |
| Scalar library functions: `RealFunctions`/`RationalFunctions`/`NumericalFunctions` `sqrt floor round abs max min isZero isUnit`, `IntegerFunctions`/`NaturalFunctions` `abs max min`, `TrigFunctions` (`sin cos tan cot arcsin arccos arctan deg rad pi`), `OpenSysMLMathFunctions` (`exp ln log atan2`) | `libm` / Go `math` with the interpreter's domain, overflow and `Natural` errors |

Everything else refuses: String, record (`attribute def`) and enum parameters, results or
attributes, a `Collections::Set` (or any collection object) and a `TensorQuantityValue` wherever
they appear (`type Collections::Set is not Integer, Real or Boolean`; a set has no native layout
and a tensor's components are quantities), parameter defaults, a calc that `:>`/`:>>`/`redefines` another *and* declares members
(redefining inherited parameters or body is not compiled), sequences whose elements mix Integer
and Real (`==`, `same`, `union` between an `Integer[0..*]` and a `Real[0..*]`, `Integer[0..*] ?? 5.5`),
a `collect` body that yields null, a `select` body that is not Boolean, `===` between a Real and an
Integer, library functions over strings, quantities and units, and `Integer ** <non-literal Integer>` (whether the
result is an Integer depends on the exponent's sign at run time, which a static type cannot
express; write the exponent as a literal or make the base Real). The refusal names the calc and
the construct (`codegen.UnsupportedError`, `errors.Is(err, codegen.ErrUnsupported)`).

## Semantics the generated code preserves

The runtime the generated program carries (`cPrelude` / `goPrelude`) reproduces the interpreter's
arithmetic rather than the host language's:

- **Integer** is `int64` with `+ - *`, negation and `**` checked for overflow (`__builtin_*_overflow`
  in C; widened checks in Go). `/` and `%` by zero are errors.
- **Integer `/`** is the exact rational quotient rounded once to binary64, as the interpreter's
  `IntQuotient` does — `7 / 2` is `3.5`, `1 / 3` is `0.3333333333333333`, and
  `9007199254740993 / 1` rounds the way the interpreter rounds. C does this with `__int128`
  remainder refinement; Go uses `math/big.Rat`.
- **Real** is binary64 and every result is checked finite; `1.0 / 0.0` and `1e308 * 10.0` are
  errors, not `inf`. `0.1 + 0.2` prints `0.30000000000000004`, exactly as the interpreter
  (see `exact-rational-evaluation.md`; no exact arithmetic is introduced here).
- **Mixed** Integer/Real operands widen the Integer, in comparisons too.
- **`and`/`or`/`implies`** short-circuit; the right operand's errors are not raised when the left
  decides. Every other operator, and every invocation, evaluates its operands left to right —
  named arguments in the order written, a parameter named twice taking the later value — so when
  two could fail the leftmost failure is the one reported. Generated C sequences operands through
  temporaries (GNU statement expressions) because C leaves argument order unspecified; Go's
  evaluation order already matches.
- **`Natural` and `Positive`** are checked exactly where the interpreter checks them: a negative
  Integer bound to such a parameter, assigned to such an attribute, or returned as such a result
  fails with the interpreter's `type mismatch` reason. Two interpreter behaviours are mirrored
  rather than corrected: `Positive` admits `0` (the interpreter's type lattice folds it into
  `Natural`), and an attribute's initializer is not checked, only later assignments.
- **Identifiers** of generated functions encode each name of the owner chain injectively
  (letters and digits verbatim, every other rune as `_hex_`, names joined by `_s_`), so `X::Y`,
  `X__Y`, the unrestricted name `'X::Y'` and a Unicode name never share a function.
- **Recursion** is bounded by the same depth as the interpreter's default
  (`runtime.DefaultMaxCalcDepth`), reported as the interpreter reports it.
- **Library functions** dispatch as the interpreter does: `NumericalFunctions::max(a, b)` keeps
  Integer operands Integer, `RealFunctions::floor` returns an Integer and fails when the value
  exceeds `int64`, `IntegerFunctions`/`NaturalFunctions` refuse Real operands at compile time and
  report negative Naturals at run time, `ln`/`log`/`sqrt`/`arcsin` report the interpreter's domain
  errors. Named and positional arguments bind and evaluate as for model calcs.
- **Output** uses the interpreter's `FormatReal` convention: positional notation with a `.0` on
  whole values, exponent notation below `1e-4` and from `1e21`, `-0.0` preserved. A sequence
  prints as `[1, 2]`, an empty one as `[]`, an unbound value as `null`.
- **Collections** keep the interpreter's three shapes — null (unbound), one value, many — and its
  rules: a one-valued sequence is a scalar wherever a scalar is expected (`(3) + 1` is `4`), a
  many-valued or null one is the interpreter's `type mismatch` at the same operator; `for` iterates
  null zero times and refuses a scalar; indexing is one-based and out-of-range is an error; nested
  sequence literals flatten and null contributes no element; `lo..hi` is inclusive and empty when
  descending; `reduce` of an empty sequence is null and `minimize`/`maximize` of one is an error;
  `==` compares elementwise while `===` also distinguishes shape and Integer from Real, and an
  empty collection of any shape is null to both and to `??`. Every
  sequence a program builds or is given counts against the interpreter's element budget
  (`OPENSYSML_MAX_ELEMENTS`, default 1,000,000), reset per run under `--repeat` with the
  arguments still charged. A local a `{in v; …}` body declares is read on demand, as the
  interpreter reads it, so an initializer the result never names never runs. The C program's
  memory is bounded the same way: its arena is released at the end of every statement that stores
  no collection and at the end of every loop pass, the collections a pass stored into longer-lived
  variables being copied down first (`TestCompiledCLoopMemoryIsBounded`); Go leaves this to its
  collector. On the command
  line a sequence argument is written as the interpreter would read it: `null`, `4`, `(4)`,
  `(1, 2)`, `()`.

`internal/repl/compile_test.go:TestCompiledCalcsAgreeWithInterpreter` is the differential contract:
every calc in `testdata/compile_calcs.sysml` is compiled by both backends and run over a matrix of
values and failure inputs (overflow, zero divisors, non-finite Reals, deep recursion, null and
many-valued operands, out-of-range indexes, multiplicity and element-budget violations), and each
value must equal the interpreter's; a scalar failure must be of the same class and a collection
failure must carry the interpreter's message verbatim. `TestCompileRefusesWhatItCannotCompile`
pins the refusals.

### Known differences

- **No step budget.** The interpreter counts evaluation steps (`OPENSYSML_MAX_STEPS`, default
  10,000,000) and stops a runaway loop with `ErrStepLimitExceeded`. Compiled code counts nothing:
  a `while` that never terminates runs forever, and a long loop the interpreter would cut short
  runs to completion. The differential test lifts the interpreter's budget for this reason. This
  is the one bound the interpreter has that compiled code lacks; a compiled program is a program.
- **Widened copies are charged.** An Integer collection bound to a Real slot is copied into
  Reals and the copy is charged to the element budget; the interpreter keeps the Integers and
  holds no copy. At the limit the program can therefore fail where the interpreter runs, never
  the reverse. `TestCompiledBudgetChargesInputsAndWidening` pins both sides.
- **Transcendental last bits.** `sin`, `cos`, `tan`, `exp`, `ln`, `log`, `atan2` and the inverse
  trigonometric functions come from glibc's `libm` in C and Go's `math` in Go and the interpreter;
  the two libraries agree to within an ulp but not bit-for-bit (Go's own `Exp` differs between
  amd64 and arm64). The differential test allows the C target 2 ulps on these calcs and requires
  everything else — `sqrt`, `floor`, `round`, `abs`, `max`, `min`, `deg`, `rad` — to be exact.
- **No evaluation trace.** There is nothing to `%trace`; the result is all the program produces.
- **GNU C.** The C backend uses `__int128`, `__builtin_*_overflow` and `setjmp`/`longjmp`, so it
  needs GCC or Clang, not an arbitrary ISO C compiler. Tested with GCC 11.4.

## Design

```
parser → resolve → semantics ─┐
                              ├→ codegen.Compiler ─→ codegen.Program (typed IR) ─→ EmitC / EmitGo ─→ cc / go build
lower.CalcBody (statements) ──┘
```

- `internal/core/codegen/ir.go` — the typed IR: `Func`, `Param`, expressions (`IntLit`, `Var`,
  `Binary`, `Call`, `ToReal`, …) and statements (`Declare`, `Assign`, `If`, `While`, `Return`).
  Every expression carries its scalar `Type`; the emitters never infer.
- `compile.go` — the front end. It walks the resolved symbol's members through
  `lower.CalcBody`, types every expression against the resolver and semantic model, inserts
  `ToReal` where the interpreter would widen, follows invocations into the callee's `calc def`
  and compiles that too, and refuses anything outside the subset with an `UnsupportedError`. The
  AST and semantic side tables are read, never mutated.
- `emit_c.go`, `emit_go.go` — one backend each. Both emit a self-contained program with the
  checked-arithmetic prelude, the calcs as functions, a `sysml_run` entry that turns an error into
  a status, and a `main`.
- `build.go` — `Source`, `Build`, `Targets`; C is compiled with
  `-O3 -flto -std=gnu11 -Wall -Wextra`, Go in a throwaway module.

## Benchmark methodology

`internal/repl/compile_bench_test.go:BenchmarkCompiledCalc` times the same invocation three ways
in one process: interpreted (`Session.RunCalc`), and as the C and Go executables run once with
`--repeat b.N`, so process start-up is amortized and each figure is per invocation.

```
go test ./internal/repl -run '^$' -bench BenchmarkCompiledCalc -benchtime 2s
```

The C loop is confirmed to do the work each iteration rather than being hoisted: `SumTo` scales
linearly (`1e6`: 0.39 ms, `1e7`: 3.8 ms, `1e8`: 38 ms per call).

## Measured results

Intel Xeon Platinum 8559C, 8 vCPUs, Go 1.25, GCC 11.4 `-O3 -flto`, 2026-09-02. Per invocation.

| Calc | Interpreted | Compiled Go | Compiled C | C vs interpreted | C vs Go |
|---|---:|---:|---:|---:|---:|
| `Fib(25)` — 242,785 recursive calls | 261 ms | 919 µs | 221 µs | 1180× | 4.2× |
| `SumTo(1000000)` — a `while` loop | 1216 ms | 764 µs | 379 µs | 3200× | 2.0× |
| `Collatz(27)` — 111 iterations of Real arithmetic | 206 µs | 5.2 µs | 0.98 µs | 210× | 5.3× |
| `Hypot(3.0, 4.0)` — one expression | 3.7 µs | 12 ns | 13 ns | 290× | 1.0× |

Reading the table:

- **Compilation is worth three orders of magnitude** on compute-bound calcs. That matches the
  interpreter census in `execution-performance-2026-09.md` (~1.1 µs per calc invocation, ~130 B
  allocated each): generated C spends about a nanosecond per `Fib` call.
- **C beats Go by 2–5× on every loop or recursion**, which is the justification asked for. The gap is
  most likely the checked arithmetic: GCC lowers `__builtin_add_overflow` to a flag test after the
  add, while Go's widened checks and function prologues stay in the hot path (inferred from the
  ratios, not from disassembly). The Go backend remains useful as
  a pure-Go fallback where no C compiler is installed, and as a second implementation the
  differential test checks the C against.
- **Trivial calcs are bound by the run harness**, not arithmetic: `Hypot` is 12 ns in either
  backend, most of it the `setjmp` (C) or `defer`/`recover` (Go) that turns an error into a
  status. A future C-ABI library entry point would drop that too.

## What this spike does not do

- Compile actions, state machines, constraints, requirements, parts, or instance graphs; the
  subset is scalar calcs. The [roadmap](#roadmap-compiling-the-whole-model) below extends it.
- Enforce the step budget (see [Known differences](#known-differences)).
- Link the compiled calc into the REPL or gRPC service; the output is a standalone executable.
- Offer a stable C ABI. The generated `sysml_run` signature is an implementation detail.

## Roadmap: compiling the whole model

The spike fixes the shape of the compiler; the rest of the language is reached by widening the IR
and its emitters, never by a second front end. Three rules hold throughout:

1. **One lowering, two consumers.** Every construct lowers exactly once, into `internal/core/lower`
   (`CalcBody`, `ActionGraph`, `StateGraph`), `queryplan` or `docplan`, and both the interpreter
   and the compiler read that form. Nothing may be interpreter-only by accident: a construct the
   compiler does not yet handle is refused with an `UnsupportedError` naming it.
2. **The interpreter is the oracle.** Each phase lands with a differential test running the same
   model compiled (C and Go) and interpreted, comparing results, verdicts and traces. The
   `runtime` conformance corpus is the primary fixture source.
3. **Refuse, never approximate.** No construct compiles until its full semantics do (masking,
   redefinition, multiplicity, error timing). A phase may narrow *which* constructs compile, never
   *how faithfully*.

### Target

A model compiles to one dependency-free executable, or to a library with a small C API, whose
behavior is the interpreter's:

```
sysml system.sysml -compile Vehicle::Sim -o sim         # a part, action, state machine, calc, constraint, requirement or document
./sim --in speed=30 --until 10s                        # run to quiescence or a time bound; stream the event trace
./sim --step                                           # events on stdin, state on stdout
./sim --verify                                         # evaluate every satisfy/assert in scope; exit 1 on any failure
./sim --render Reports::MassReport                     # write the document as Markdown
```

| Construct | Compiled form |
|---|---|
| `part`, `attribute`, `port`, `item` | C structs laid out at compile time from the flattened, redefinition-resolved shape (`runtime/shape.go` is the source of truth); no maps, no lazy materialization |
| `calc` | functions (this spike), widened to collections, records and library functions |
| `constraint`, `assert constraint` | Boolean functions, evaluated at the points the interpreter checks, with the interpreter's verdicts (`true`/`false`/unresolved) |
| `requirement`, `satisfy` | one record per requirement: `assume` gates `require`, nested requirements roll up, `satisfy … by P` specializes the predicates to `P`'s struct; a `--verify` report per assertion |
| `action` | the `ActionGraph`: straight-line code where token flow is deterministic, a scheduler loop where fork/join/accept make it concurrent |
| `state` | the `StateGraph` as an event loop: dispatch on state × trigger, guards and effects inlined, `defer` as a per-state mask, routing pseudostates as edges |
| `document def`, `view def` | the `docplan` and its `queryplan` programs emitted as code over the compiled structs; output is Markdown or the `docir` tree; PDF remains the external converter's job |
| Library functions (OMG `RealFunctions`, `TrigFunctions`, `CollectionFunctions`; OpenSysML `OpenSysMLMathFunctions`) | a precompiled runtime (`libm` / Go `math`) with the interpreter's domain and arity errors, not re-lowered per model |
| `metadata`, `IdentityMetadata` | constant tables, so a compiled program still reports identities and tags |
| Extension notations (`defer`, `choice`, `junction`, `history`) | already lowered into the `StateGraph`; compile as any other vertex or edge. `-strict` gates them before codegen, as today |

Interpreter-only, refused by the compiler with a named error: SMT-backed satisfiability
(`internal/core/solve`), REPL introspection and `%trace`, instance adoption across edits, and
the step budget.

### Phases

Each phase is a session-sized unit with its own PR, its own differential test and a benchmark
checkpoint that must still show the C backend ahead of the interpreter by two orders of
magnitude on its own fixtures; a phase that loses the speedup is redesigned, not merged. The
phases are ordered by dependency, and documents come before behavior because they need only
values, structs and verdicts, so compiled report generation arrives after four phases.

**Phase 1 — Values: collections, records, library functions.**
IR: `Type` becomes structural — scalars, `Seq[T]` with a multiplicity bound, `Record{fields}`
from a flattened shape, `Enum`. Expressions gain `Index`, `Field`, `Seq` literals and the
collection operations the interpreter implements in `runtime/collections.go` (`size`,
`includes`, `select`, `collect`, `reduce`, …). Library calls become `LibCall{Fn}` against a
table shared by both emitters; the OMG and OpenSysML function libraries are compiled once into
the prelude with `runtime/library_functions.go`'s domain errors. Body-local declarations without
an initializer become representable (the null the interpreter uses) so the refusal added in the
spike is lifted. The multiplicity and element budgets are enforced at the same points.
Exit: every calc in the runtime conformance corpus either compiles and matches, or is refused
with a documented reason; `OPENSYSML_CALC_COMPILE`'s closure tier and the native tier share the
eligibility rule.
Status: the collection half is done — homogeneous sequences of the scalar types with any
multiplicity, the shape rules, `for`, the sequence and control libraries and the element budget, in
both backends, under the differential test described above. Records, enums and record field access
are still refused (`type X is not Integer, Real or Boolean`), as are sequences mixing Integer and
Real elements; they are the remainder of this phase.

**Phase 2 — Instances: parts, attributes, ports, connections.**
IR: `Program` gains `Struct` layouts derived from the flattened shape (redefinitions, subsetting,
feature chains resolved to offsets; variations to a tagged union with a selected variant).
Default values and bindings compile to an `init` function per struct; connections and flows to
pointer fields fixed at initialization. Instance materialization is eager: the whole tree exists
before `main` runs, which is what makes lookups free. Exit: `-e` expressions over parts and
attributes give the interpreter's values; feature-chain and redefinition robustness cases give
the same errors.

**Phase 3 — Constraints, requirements, satisfies.**
IR: `Predicate` (a `Func` with a Boolean result and an unresolved verdict when an operand is
unbound), `Requirement{Assume, Require, Nested}`, `Satisfaction{Requirement, Subject}`. The
`--verify` entry evaluates every assertion in the compiled scope and prints one line per
assertion in the interpreter's report form (`runtime/satisfy.go`, `CheckResult`). SMT-only
constraints are refused by name. Exit: `TestSatisfy*` and the requirement conformance fixtures
agree across all three evaluators; a sweep benchmark (N parameter sets × M satisfactions) is
added to the checkpoint.

**Phase 4 — Documents.**
IR: `docplan.Plan` and `queryplan.Program` emitted as code over Phase 2 structs; `docir`
construction and the Markdown renderer become a shared runtime. `--render` writes what
`-render-document` writes today, byte for byte; PDF conversion is unchanged. Exit: every fixture
in `docrender`'s tests renders identically compiled and interpreted.

**Phase 5 — Actions.**
IR: `ActionGraph` is consumed directly. Where the graph is a series–parallel DAG with no `accept`,
it lowers to straight-line statements in the spike's IR; otherwise to a `Scheduler` with a token
table, a ready queue and the interpreter's ordering rule (`action_executor.go`), so traces match
`TestExecutionTrace` exactly. `perform`, `send`, `accept` (signal, time and change triggers) and
sub-flows compile; the step budget stays interpreter-only and is documented as such. Exit: the
action conformance corpus and golden traces match; deadlock and unbound-parameter robustness
cases give the same typed errors.

**Phase 6 — State machines.**
IR: `StateGraph` → `Machine{States, Regions, Transitions, Deferred}`; the emitted event loop
dispatches on `(state, trigger)`, evaluates guards, runs exit/effect/entry in the interpreter's
order, tracks history and deferral, and reports quiescence. `--step` and `--until` drive it.
Exit: the state conformance corpus, golden traces and the pseudostate robustness cases match; a
long-run simulation benchmark (events/second) joins the checkpoint.

**Phase 7 — Embedding.**
A stable C API (`sysml_new`, `sysml_set`, `sysml_send`, `sysml_step`, `sysml_get`, `sysml_verify`,
`sysml_free`) and `-compile -lib` producing a static library and header; a Go package wrapping
it so the REPL and gRPC service can run a compiled model in place of the interpreter when a
model is compilable. Exit: the Python and Node clients run the same scenario against both.

### Cross-cutting work, folded into the phase that first needs it

- **Error parity.** Each phase adds its interpreter errors to the prelude's message table; the
  differential test compares messages, not only failure.
- **Step budget.** Kept interpreter-only. A later `--budget N` on compiled programs is possible
  (a counter per loop back-edge) but costs the speedup on tight loops; decide with numbers.
- **Documentation.** `docs/project/spec-compliance.md` gains a "compiled" status per rule as
  phases land; the guide gains a chapter once Phase 3 makes `--verify` useful to a modeller.
- **Estimate.** One session per phase on this foundation, two for instances and for actions,
  whose layout and scheduling rules are the interpreter's most involved. Phases 1–3 unlock
  verification sweeps, the workload the spike was asked to justify; Phase 4 compiled documents.
