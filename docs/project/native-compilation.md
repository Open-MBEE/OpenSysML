# Native compilation of calcs

`sysml -compile` translates a `calc def` (or a calc usage) into a standalone native executable,
ahead of time, through C or Go. The interpreter in `internal/exec/runtime` stays the reference
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
input the interpreter would reject — an Integer beyond the Integer size limit, division or modulo by zero, a non-finite
Real, a Real argument written outside the Real range (`1e400`, or `1e-400` underflowing to
zero), recursion past the calc depth budget — exits with status 1 and the reason on stderr; an
argument that is not the notation of its type at all (`inf`, `nan`, a hexadecimal `0x1p-2`,
`1_000.5`) exits with status 2 — a Real argument is decimal notation only, as the interpreter's
literals and `ToReal` are.

The generated source is always written beside the executable (`fib.c` / `fib.go`), so what was
compiled is inspectable. `OPENSYSML_CC` names the C compiler (default `cc`) and `OPENSYSML_GO`
the go command (default `go`).

Programmatically: `Session.CompileCalc(name)` in `internal/frontend/repl` yields a `codegen.Program`, which
`codegen.Source` renders and `codegen.Build` compiles.

## The compiled subset

A calc compiles when everything it reaches is in this subset:

| Construct | Compiled as |
|---|---|
| `in` parameters typed `Integer`, `Natural`, `Positive`, `Real`/`Rational`, `Boolean`, `String` or an `enum def`, with no multiplicity or `[1]` | Integer: `int64_t` in C, an `int64` promoted to `math/big` in Go ([Integers](#integers)); a Real-typed value as a number that holds an Integer or a binary64 ([Numbers](#numbers)); `bool`; a String as UTF-8 text; an enumeration literal as its index ([Strings and enumerations](#strings-and-enumerations)) |
| The same types with any multiplicity (`[0..*]`, `[2..3]`, `[0..1]`, …), as parameters, results and body-local attributes | a sequence of the element type with its shape (null, one value, many); the bounds are checked where the interpreter checks them, and a sequence bound to a feature not declared `nonunique` is refused where it repeats a value, with the interpreter's `uniqueness violation` reason and positions |
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
| `in calc f { in v : Real; return : Real; }` and `in calc f : Sq` parameters; a calc def, a calc usage with an unsupplied input, or a compiled scalar library function (`RealFunctions::sqrt`, `RealFunctions::floor`, …) passed for one; `f(a)` and `f(v = a)` in the body | one function per calc *and* per tuple of function values its `in calc` parameters are bound to ([Function values](#function-values)); `f(a)` is a direct call; a typed parameter takes only a calc conforming to its type, as the interpreter's binding does |
| `SampledFunctions::Sample(f, xs)` bound to an `attribute s : SampledFunction`, or read at once by `Domain(…)`/`Range(…)`; `Domain(s)`, `Range(s)` | two hidden locals: the domain as a sequence and `f` collected over it in order, taken when the sample is (at each read of `s` when a body expression declares it); `Domain`/`Range` read them; a literal `null` domain is the empty sequence of `f`'s parameter type (the type a library function declares for its parameter, Real when that is any `NumericalValue`) |
| String literals, `+`, `<` `<=` `>` `>=`, `==` `!=` `===` `!==`; `StringFunctions::Length`, `Substring`, `ToString`; `ToString` of `IntegerFunctions`, `NaturalFunctions`, `RealFunctions`, `BooleanFunctions` and `BaseFunctions` | the interpreter's String operations: concatenation, ordering by code point, `Length` and one-based `Substring` counted in characters, each number formatted as the interpreter formats it |
| Enumeration literals (`Color::red`), `==` `!=` `===` `!==` between them, `BaseFunctions::ToString` of one | the literal's identity; equal only to itself, never to a literal of another enumeration, a number or a String |
| A function value — a calc def, a calc usage with an unsupplied input, a compiled library function, or a calc declared in the body being compiled — read where a value is expected: returned, bound to an attribute, assigned, compared with `==`/`===`, chosen by `if`, held in a sequence, passed to an `in calc` parameter from any of these, and invoked (`Apply(g, a)`, `f(a)`) | a function value of the program ([Function values](#function-values)): the calc it denotes and, for a closure, the run that read it and what it captured; invoked by dispatch over the calcs it may denote |
| Scalar library functions: `RealFunctions`/`RationalFunctions`/`NumericalFunctions` `sqrt floor round abs max min isZero isUnit`, `IntegerFunctions`/`NaturalFunctions` `abs max min`, `TrigFunctions` (`sin cos tan cot arcsin arccos arctan deg rad pi`), `OpenSysMLMathFunctions` (`exp ln log atan2`) | `libm` / Go `math` with the interpreter's domain, overflow and `Natural` errors |

Everything else refuses: a record (an `attribute def` or `item def` with features) as a
parameter, result or attribute (`type Refused::Point is not Integer, Real, Boolean, String or an
enumeration`; see [Records](#records)), a `Collections::Set` (or any collection object) and a
`TensorQuantityValue` wherever they appear (a set has no native layout and a tensor's components
are quantities), an enumeration that specializes another type, has an unnamed literal, inherits a
literal or gives one a value, parameter defaults, a calc that `:>`/`:>>`/`redefines` another *and*
declares members (redefining inherited parameters or body is not compiled), a `collect` body that
yields null, a `select` body that is not Boolean, library functions over quantities and units, and
`Integer ** <non-literal Integer>` (whether the result is an Integer depends on the exponent's sign
at run time, which a static type cannot express; write the exponent as a literal or make the base
Real). Of function values: an `in calc` parameter of the calc being compiled itself (`which a
program cannot take on its command line`), a calc owned by a part or read off an object through a
feature chain (`whose function value closes over that object`, see [Records](#records)), a
function value invoked with a receiver (`x->f()`), a calc declared in the body of a calc that
specializes another, a calc declared in one body read from another (`a calc declared in the body
of …, read from the body of …`), a control operation such as `ControlFunctions::collect` read as a
value (it binds its arguments unevaluated), a function value bound to an `in calc f : Sq`
parameter whose calc does not specialize `Sq` (`cannot bind the function value … to a parameter
typed by …`, the interpreter's `type mismatch` at the same binding), a `SampledFunction` used as
anything but the operand of `Domain` or `Range`, and `Range(Sample(NumericalFunctions::abs, null))`
where an `Integer[0..*]` is declared (the compiler fixes a null domain's element type from the
sampled function alone, and a function declared over any `NumericalValue` gives Real; the
interpreter, which types nothing, computes `[]`). The C target alone also refuses a closure that
captures a String, a sequence or another function value (`… for the C target: a C closure holds
its captures inline …`); the Go target computes it. A `meta` cast (`x meta KerML::Feature`), whose
result reflects a model element as a metaobject that a native program has no representation of,
is refused as *a meta cast, whose metaobject reflects a model element and has no native
representation*; metaobjects stay interpreter-only. The refusal names the calc and the construct
(`codegen.UnsupportedError`, `errors.Is(err, codegen.ErrUnsupported)`).

## Integers

KerML's `ScalarValues::Integer` is the mathematical integers, and the interpreter computes them
exactly (an `int64` while a value fits, `math/big` beyond it), refusing only a result past the
Integer size limit (`OPENSYSML_MAX_INTEGER_BITS`, default 2^20 bits). The two targets keep that
contract differently:

- **Go** carries the same hybrid: `sysmlInt` is an `int64` until a result leaves it, then a
  `*big.Int`, demoted again whenever a result fits. Arithmetic, `**`, `sum`, `product`, `abs`,
  `floor`, `round`, ranges, ordering, `==` and uniqueness are exact, so `9223372036854775807 + 1`
  prints `9223372036854775808` and `2 ** 70` prints `1180591620717411303424`. The program reads
  `OPENSYSML_MAX_INTEGER_BITS` and refuses a larger result with the interpreter's
  `integer size limit exceeded` reason; an index beyond `int64` fails as the interpreter's does
  (`index … addresses no position`), and a range whose count the element budget cannot hold
  fails on the budget. Integer arguments of any size are accepted.
- **C** has no arbitrary-precision integer of its own, so it keeps `int64_t` and refuses, at
  compile time, any calc with an Integer construct whose result is not provably within `int64`:
  Integer `+`, `-`, `*` (unless both operands are literals and the result fits), unary `-`, `**`
  by anything but the literal `0` or `1`, Integer `sum`/`product`, `IntegerFunctions::abs`,
  `floor` and `round`, and an Integer literal beyond `int64`. The refusal names the first such
  construct in evaluation order, e.g. ``in calc Compiled::Fib: Integer `-` for the C target: the
  interpreter's Integers are unbounded and a C program holds int64 (the Go target computes them
  exactly)``. What remains — comparisons, sizes, indexes, ranges, `/` (a Real), `%`, `min`/`max`,
  and Real arithmetic over Integers — cannot leave `int64` and compiles as before. An Integer
  argument beyond `int64` on a C program's command line exits with status 2
  (`… is beyond int64, the Integers a compiled C program holds`), as any argument the program
  cannot represent does.

## Numbers

The interpreter keeps an Integer an Integer when it is written to a Real-typed feature: with
`in a : Real; return : Real = a`, the argument `3` prints `3`, `r === 3` holds for `r : Real = 3`,
and `(1, 2.5)->collect {in v; v * 2}` is `[2, 5.0]`. KerML's `ScalarValues` library makes this the
faithful reading — `datatype Integer specializes Rational; datatype Rational specializes Real;`
(KerML 1.0 §9.3.2) — so an Integer *is* a Real and nothing converts it on a write. A compiled
Real-typed value is therefore a *number*: an Integer or a binary64, decided at run time, and every
operator splits on what its operands hold, as the interpreter's dispatch does (`compile_num.go`).
Integer operands stay exact (`r * 2 + 1` over `r = 3` is `7`), any Real operand gives a Real,
`/` is the interpreter's `IntQuotient`, `**` by a negative exponent is Real, `===` distinguishes an
Integer from an equal Real while `==` compares them exactly (`CompareIntReal`), and a sequence of
numbers prints each element in its own notation. Mixed Integer/Real sequences — literals, `==`,
`same`, `union`, `includes`, `sum`, `minimize`, `??` between an `Integer[0..*]` and a
`Real[0..*]` — follow the same rules element by element; an Integer collection bound to a Real
slot keeps its Integers.

The Go target reads any Integer for a Real parameter and computes it exactly. The C target holds
an Integer in `int64` and refuses arithmetic that may leave it ([Integers](#integers)), so a C
program reads a Real parameter only in Real notation and exits with status 2 on an Integer
argument (`argument a: 3 is an Integer, which a compiled C program reads for a Real parameter only
in Real notation (as 3.0)`), as it does for an Integer beyond `int64`.

KerML's `Rational` is the exact rationals (§9.3.2.2.8); this tree, like the interpreter, still
computes a `Rational`-typed value as a Real ([exact-rational-evaluation.md](exact-rational-evaluation.md)),
and the compiler does the same, no more and no less. When the interpreter computes Rationals
exactly, the compiler refuses that arithmetic with an `UnsupportedError` until it computes it
exactly too; it never rounds an exact Rational to binary64.

## Strings and enumerations

`ScalarValues::String` is a scalar data value (KerML 1.0 §9.3.2) whose operations the Kernel
Function Library declares in `StringFunctions` (§9.4; `StringFunctions.kerml`: `'+'`, `Length`,
`Substring`, the four orderings, `'=='`, `ToString`). The compiler implements exactly the ones the
interpreter does (`runtime/library_functions.go`), with the interpreter's semantics: `+`
concatenates, the orderings compare by code point, `Length` counts characters, `Substring(s, l,
u)` takes the one-based inclusive characters `l..u` and fails with the interpreter's `index out
of range` reason outside `1..Length(s)`, and the `ToString` of each numeric library and of
`BooleanFunctions` formats as the interpreter prints. A String compared with `==` to a value of
another kind is false, as `DataFunctions::'=='` over different data types is in the interpreter.
A String argument is written in String notation (`"héllo"`, with the interpreter's escapes) and a
String result prints in it, so output and input round-trip.

An enumeration (SysML v2 §8.3.7 EnumerationDefinition: "an AttributeDefinition all of whose
instances are given by an explicit list of enumerated values") compiles when its literals are
named, its own and valueless. A literal is identified by itself: `==` and `===` hold only between
a literal and itself — never against a literal of another enumeration of the same name
(`Color::red` and `Shade::red`), a number, or a String — and arithmetic and ordering over literals
are the interpreter's `type mismatch` at the same operator. `BaseFunctions::ToString` of a literal
is its qualified name, the result prints it, and an argument names one by qualified name
(`Compiled::E::Color::red`); any other text exits with status 2 (`… is not a literal of …`).

## Records

KerML does not settle how two separately constructed data values with equal features compare, nor
how one prints. KerML 1.0 §7.4.2 says data types "classify things that do not exist in time or
space", which suggests a value distinguished only by its features; but `DataFunctions::'=='` is
`abstract` for a user `attribute def` (only the scalar libraries define it), and
`DataFunctions::'==='` is defined as `x == y`, so the library leaves record equality to whatever
`'=='` is. `BaseFunctions::ToString` is likewise `abstract`. SysML v2 adds nothing here.

The interpreter builds a record (`new Point(a, 2.0)`, an attribute with nested features) as an
instance with a session-wide identity: two `new Point(1.0, 2.0)` are not `==`, and a record prints
as `Instance(ID: N)`, where `N` depends on what the session made before. A program cannot reproduce
`N`, and equality by identity versus by features is a choice the specification leaves open, so
records stay refused natively until that choice is made; the same holds for a calc read off an
object (`twice.scale`), whose function value is identified by that object.

## Step budget

The interpreter bounds every evaluation by a step budget (`runtime.DefaultMaxSteps`, 10,000,000,
raised by `OPENSYSML_MAX_STEPS`), charging one step per expression node it evaluates and per loop
pass and flow node it reaches, and stops with `ErrStepLimitExceeded`: `evaluation step limit exceeded (N steps;
raise OPENSYSML_MAX_STEPS to allow more)`. KerML and SysML are silent on any such bound; it is a
resource limit of the implementation, so the interpreter's accounting is the contract. A compiled
program carries the same counter, reads `OPENSYSML_MAX_STEPS` at start-up, charges each compiled
node the steps the interpreter spends on its source node (a constant the interpreter folds spends
one, a function value read by name one, a call its frame and argument reads), at the point the
interpreter spends them relative to anything that can fail, and fails with the interpreter's
message and status 1 at the same count. Each `--repeat` run starts from zero. Both check a charge
against the steps left before spending it and stop a spent counter one past the limit, saturating
at the int64 maximum, so a budget of the int64 maximum binds without the counter overflowing; `TestCompiledStepBudgetAtTheInt64Limit`
builds the C program with the signed-overflow sanitizer to hold it to that.
`TestCompiledStepBudgetMatchesInterpreter` finds, for every differential case, the least budget
the interpreter needs and requires the compiled program to succeed with exactly that budget and
fail with the interpreter's error one step below it.

## Semantics the generated code preserves

The runtime the generated program carries (`cPrelude` / `goPrelude`) reproduces the interpreter's
arithmetic rather than the host language's:

- **Integer** is unbounded, as the interpreter's is ([Integers](#integers)): Go computes it
  exactly, C refuses at compile time a calc whose Integer result may leave `int64`. `/` and `%`
  by zero are errors.
- **Integer `/`** is the exact rational quotient rounded once to binary64, as the interpreter's
  `IntQuotient` does — `7 / 2` is `3.5`, `1 / 3` is `0.3333333333333333`, and
  `9007199254740993 / 1` rounds the way the interpreter rounds. C does this with `__int128`
  remainder refinement; Go uses `math/big.Rat`.
- **Real** is binary64 and every result is checked finite; `1.0 / 0.0` and `1e308 * 10.0` are
  errors, not `inf`. `0.1 + 0.2` prints `0.30000000000000004`, exactly as the interpreter
  (see `exact-rational-evaluation.md`; no exact arithmetic is introduced here).
- **Mixed** Integer/Real operands widen the Integer in arithmetic. A comparison between them is
  exact, as the interpreter's `CompareIntReal`: `9007199254740993 > 9007199254740992.0` holds
  although the Integer rounds to that Real.
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
  Integer operands Integer, `RealFunctions::floor` returns the exact Integer (in Go; C refuses
  it, as its result may leave `int64`), `IntegerFunctions`/`NaturalFunctions` refuse Real operands at compile time and
  report negative Naturals at run time, `ln`/`log`/`sqrt`/`arcsin` report the interpreter's domain
  errors. Named and positional arguments bind and evaluate as for model calcs.
- **Function values** known at compile time are specialized: `Apply(Sq, a)` calls a specialization
  of `Apply` in which `f(a)` is the direct call `Sq(a)`, so `f`'s arguments bind, evaluate and fail
  exactly as a direct invocation of `Sq` does — by `Sq`'s own parameter names, with `Sq`'s own
  arity, at the same depth against the recursion budget. The parameter is `f` or its qualified
  name through the calc declaring it, `Apply::f` and `Pkg::Apply::f`, as the interpreter reads
  it from that calc's run. `Sample(f, xs)` computes `f` at each domain value in order when the
  sample is taken, so the first failing element is the one reported and an unbound `xs` samples
  to `[]` as the library's `collect` does. `Sample`, `Domain` and `Range` are the library calcs
  they are in the interpreter: each is one frame against the recursion budget, entered after its
  arguments are computed, so a sampled calc recursing to the limit fails at the same depth and a
  domain computed by a calc at the limit succeeds; each `SamplePair` is the three elements the
  library's `new SamplePair` in a `collect` holds (its domain value, its range value and its
  place among the samples), charged as it is taken, and each `Domain` or `Range` read collects a
  fresh sequence charged to the element budget.
- **Function values chosen at run time** dispatch over the calcs they may denote, each arm the
  callee's specialized direct call, so binding, arity and depth are the callee's own; `==` and
  `===` are the interpreter's identity (calc, and the run a closure was read in), and a function
  value that is not a valid operand (`Plus`, `Neg`, `Not`, `MulR`, `CondF`, `StmtIf` in the `Closure` package) is the
  interpreter's `type mismatch` at the same operator, after its operands are evaluated
  ([Function values](#function-values)).
- **Output** uses the interpreter's `FormatReal` convention: positional notation with a `.0` on
  whole values, exponent notation below `1e-4` and from `1e21`, `-0.0` preserved. A sequence
  prints as `[1, 2]`, an empty one as `[]`, an unbound value as `null`.
- **Collections** keep the interpreter's three shapes — null (unbound), one value, many — and its
  rules: a one-valued sequence is a scalar wherever a scalar is expected (`(3) + 1` is `4`), a
  many-valued or null one is the interpreter's `type mismatch` at the same operator; `for` iterates
  null zero times and refuses a scalar; indexing is one-based and out-of-range is an error; nested
  sequence literals flatten and null contributes no element; `lo..hi` is inclusive and empty when
  descending; `reduce` of an empty sequence is null and `minimize`/`maximize` of one is an error;
  `==` compares elementwise while `===` also distinguishes shape and is element-wise identity,
  so an Integer element is never `===` an equal Real one (`(1, 2.5) === (1.0, 2.5)` is false), and an
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

`internal/frontend/repl/compile_test.go:TestCompiledCalcsAgreeWithInterpreter` is the differential contract:
every calc in `testdata/compile_calcs.sysml` is compiled by both backends and run over a matrix of
values and failure inputs (overflow, zero divisors, non-finite Reals, deep recursion, null and
many-valued operands, out-of-range indexes, multiplicity, uniqueness and element-budget violations), and each
value must equal the interpreter's; a scalar failure must be of the same class and a collection
failure must carry the interpreter's message verbatim. `TestCompileRefusesWhatItCannotCompile`
pins the refusals.

### Known differences

- **Widened copies are charged.** An Integer collection bound to a Real slot is copied into
  Reals and the copy is charged to the element budget; the interpreter keeps the Integers and
  holds no copy. At the limit the program can therefore fail where the interpreter runs, never
  the reverse. `TestCompiledBudgetChargesInputsAndWidening` pins both sides.
- **A call's result stays charged to the end of its statement.** The interpreter releases what
  a calc's return statement built as soon as the calc answers, so `size(Mk(k)) + size(1..k)`
  holds `k` elements at a time there and `2k` in the program; a `Domain` or `Range` read, and
  the `Sample` a `Range(Sample(f, xs))` takes inline or a `{in v; …}` body declares, are held the same way. Again the program
  can fail where the interpreter runs, never the reverse.
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

- `internal/translate/codegen/ir.go` — the typed IR: `Func`, `Param`, expressions (`IntLit`, `Var`,
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

### Function values

The interpreter's function value (`runtime/function_value.go`) is a closure: the calc's shape
together with the lexical frames and the object it was read in (KerML 1.1 §7.4.4: a Function is a
Behavior with a `result`, and a feature reference to one denotes it; SysML v2 §7.17: a calc def
is a Function, a calc usage an Expression). Its `==` and `===` compare the calc, the object and
the run it closes over (`eval.go`, the `ValFunction` case of value equality): a static calc is equal to itself
wherever it is read, a calc declared in a body is equal only to a value read in the same run of
that body, so two reads of one closure are equal and two calls of a maker returning it give two
unequal values. KerML says nothing more: `BaseFunctions::'=='` and `'==='` are `abstract` over
`Anything`, and no clause defines when two function values are the same, so the interpreter's
identity is the contract. `BaseFunctions::ToString` is likewise `abstract`, and the interpreter
gives a function value no String notation (`type mismatch: function BaseFunctions::ToString
parameter "x" has no String notation for the function P::sqU`); a result or sequence holding one
prints its qualified name.

The compiler keeps two representations (`compile_fn.go`, `compile_fnval.go`):

- **Specialization**, when the value is fixed at compile time — a calc def, a calc usage with an
  unsupplied input, a compiled library function, or an `in calc` parameter bound to one. The
  callee is compiled once per distinct tuple of such values (`Apply_fn_Sq`, `Apply_fn_Half`;
  `codegen.Compiler.funcs` is keyed by calc and tuple), and inside the specialization `f(a)` is the
  direct call the interpreter would make after looking `f` up, so a call through `f` binds,
  evaluates, counts against the depth budget and fails exactly where a call of `Sq` does. The type
  an `in calc f : Sq` parameter declares travels with it and is checked where the interpreter
  checks a written value.
- **A run-time function value**, when the value is chosen at run time, stored, returned, compared,
  held in a sequence, or is a closure. Its compiled type is the *set* of calcs it may denote
  (`FnSet`, found by a fixpoint over the program, `Compiler.widenFn`); the value is the index of
  one of them, the run identity that read it (`0` for a static calc, a fresh identity per run of a
  body declaring closures), and for a closure its captured bindings. Invoking one dispatches over
  the set (`FnDispatch`), each arm the specialized direct call above, so argument binding, arity,
  names and depth are again the callee's own; the arms' results unify as an `if`'s branches do.
  `==` and `===` compare index and run, which is the interpreter's identity. A closure captures
  the enclosing bindings it reads when it is read (`capture`), as the interpreter's frames hold
  them; one calling itself, or a sibling declared in the same body, recompiles with its captures
  bound from the start, so recursion and mutual reference see the run that read them. In Go the
  captures are an environment slice the value points to, kept alive by the collector after the
  declaring body returns. C keeps captures inline in the value, which suffices for Integer, Real,
  Boolean and enumeration captures; a String, sequence or function capture would outlive the arena
  that holds it, so the C target refuses it by name and the Go target computes it.

`TestCompiledCalcsAgreeWithInterpreter` covers each shape in the `Closure` package of
`compile_calcs.sysml`: a function chosen by `if` and returned (`Pick`, `PickId`), stored and
reassigned (`Stored`, `Reassigned`), compared by `==`, `!=`, `===` (`Eq`, `Ident`, `Neq`,
`SameSq`), held in sequences with `includes`, `select`, indexing and uniqueness (`Pair`,
`PairUntyped`, `Includes`, `Selected`, `Indexed`, `Unique`, `SeqFmt`, `ClosureSeq`, `ClosureUnique`), mixed with a library function (`MixedKind`), refused
by `ToString` with the interpreter's error (`ToStr`), and closures over parameters and locals —
returned (`Mk`, `MkApply`), compared within and across runs (`MkSame`, `MkTwice`), recursive
(`Rec`), sibling (`Sib`, `SibEq`), nested (`Inner`), chosen against a static calc
(`ChooseClosure`), over unbounded Integers (`IntClosure`), and capturing Strings, sequences,
functions, Booleans and enumeration literals (`StrClosure`, `SeqClosure`, `FnClosure`,
`BoolClosure`, `EnumClosure`).

## Benchmark methodology

`internal/frontend/repl/compile_bench_test.go:BenchmarkCompiledCalc` times the same invocation three ways
in one process: interpreted (`Session.RunCalc`), and as the C and Go executables run once with
`--repeat b.N`, so process start-up is amortized and each figure is per invocation.

```
go test ./internal/frontend/repl -run '^$' -bench BenchmarkCompiledCalc -benchtime 2s
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
- Link the compiled calc into the REPL or gRPC service; the output is a standalone executable.
- Offer a stable C ABI. The generated `sysml_run` signature is an implementation detail.

## Roadmap: compiling the whole model

The spike fixes the shape of the compiler; the rest of the language is reached by widening the IR
and its emitters, never by a second front end. Three rules hold throughout:

1. **One lowering, two consumers.** Every construct lowers exactly once, into `internal/ir/lower`
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
| `state` | the `StateGraph` as an event loop: dispatch on state × trigger, guards and effects inlined, routing pseudostates as edges |
| `document def`, `view def` | the `docplan` and its `queryplan` programs emitted as code over the compiled structs; output is Markdown or the `docir` tree; PDF remains the external converter's job |
| Library functions (OMG `RealFunctions`, `TrigFunctions`, `CollectionFunctions`; OpenSysML `OpenSysMLMathFunctions`) | a precompiled runtime (`libm` / Go `math`) with the interpreter's domain and arity errors, not re-lowered per model |
| `metadata`, `IdentityMetadata` | constant tables, so a compiled program still reports identities and tags |
| Extension notations (`choice`, `junction`, `history`) | already lowered into the `StateGraph`; compile as any other vertex or edge. `-strict` gates them before codegen, as today |

Interpreter-only, refused by the compiler with a named error: SMT-backed satisfiability
(`internal/exec/solve`), REPL introspection and `%trace`, instance adoption across edits, the
step budget, and the extent operator `all T` (KerML 1.0 §7.4.9.2, `BaseFunctions::'all'`). The
interpreter answers `all T` with the extent of the run it is evaluated in — the objects the run
has materialized and the usages typed by `T` its context reaches, a variation's variants, an
enumeration's literals — because objects materialize lazily and no run holds the instances the
spec's Object semantics describe in the abstract. A compiled program has no run to consult: its
structs are the values its statements build, so the compiler refuses `all` with a typed
`UnsupportedError` (`operator 'all'`) rather than answering a smaller extent than the
interpreter would (`internal/frontend/repl/compile_test.go` `Refused::Extent`).

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
both backends, under the differential test described above, with Strings, enumerations, Real-typed
values holding Integers and mixed Integer/Real sequences. Records and record field access are
still refused (`type X is not Integer, Real, Boolean, String or an enumeration`) pending the
equality and formatting choice described under [Records](#records); they are the remainder of
this phase.

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
IR: `StateGraph` → `Machine{States, Regions, Transitions}`; the emitted event loop
dispatches on `(state, trigger)`, evaluates guards, runs exit/effect/entry in the interpreter's
order, tracks history, and reports quiescence. `--step` and `--until` drive it.
Exit: the state conformance corpus, golden traces and the pseudostate robustness cases match; a
long-run simulation benchmark (events/second) joins the checkpoint.

**Phase 7 — Embedding.**
A stable C API (`sysml_new`, `sysml_set`, `sysml_send`, `sysml_step`, `sysml_get`, `sysml_verify`,
`sysml_free`) and `-compile -lib` producing a static library and header; a Go package wrapping
it so the REPL and gRPC service can run a compiled model in place of the interpreter when a
model is compilable. Exit: the Python and Node clients run the same scenario against both.
The embedded restriction of this API — no allocation, no callbacks, a fixed step — and the
freestanding C profile a flight target needs, which this backend's GNU-C prelude does not meet,
are designed in [embedded-target.md](../internals/design/embedded-target.md) as a second emitter
over the same IR.

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
