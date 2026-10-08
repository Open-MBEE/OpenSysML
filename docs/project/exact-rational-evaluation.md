# Exact-rational evaluation

A KerML `Rational` is a rational number, and OpenSysML evaluates it as one: `0.1 + 0.2 == 0.3`
is `true`, `1 / 3` is the Rational `1/3`, `RationalFunctions::numer(RationalFunctions::rat(1, 3))`
is `1`. `Real` stays IEEE 754 binary64. This record derives that behavior from the
specification, states the choices the specification leaves to an implementation and how each was
made, and keeps the probes of the pinned pilot, which evaluates a `Rational` as a Java `double` and
so differs from this implementation by design.

An earlier version of this record adjudicated the change and declined it, on the grounds that the
specification is silent on precision and the pilot computes in binary64. That decision is
superseded: the specification is not silent on what a `Rational` *is*, and the pilot's binary64 is
an implementation artifact, not a reading of the text. The pilot's answers are kept below as the
documented divergence.

## What the specification requires

KerML 1.0 (formal/2025-02-01) and the Kernel libraries vendored under `internal/workspace/libs`:

- **A Rational is a rational number.** §9.3.2.2.8: "Rational is the type of rational numbers,
  extended with values for positive and negative infinity." §9.3.2.2.9: "Real is the type of
  mathematical (extended) real numbers. This includes both rational and irrational numbers".
  `ScalarValues.kerml` orders them `Natural :> Integer :> Rational :> Real :> Complex`.
- **A decimal literal is a Rational.** §8.3.4.8.13 `LiteralRational`: `value : Real`, "the value
  whose rational approximation is the result of evaluating this LiteralRational"; §8.4.4.9.2: "only
  the rational-number subset of the real numbers can be represented using a finite literal. So the
  result of a LiteralRational is actually always classified in the KerML DataType Rational." A
  finite decimal — `0.1`, `1.5e3`, `25E-3` — names a rational exactly (1/10, 1500, 1/40), so the
  rational approximation of the value it spells is that value: the literal evaluates to it, not to
  a binary64 near it.
- **The library declares which operations stay Rational.** `RationalFunctions.kerml` (§9.4.10):
  `rat(numer: Integer, denum: Integer): Rational`, `numer`/`denom(rat: Rational): Integer`, `abs`,
  `'+'`, `'-'`, `'*'`, `'/'`, `max`, `min`, `sum`, `product` over Rationals returning `Rational`;
  `'<'`, `'>'`, `'<='`, `'>='`, `'=='` returning `Boolean`; `gcd`, `floor`, `round`, `ToInteger`
  returning `Integer`; `ToString` returning `String`; `ToRational(x: String): Rational`.
  `IntegerFunctions.kerml` (§9.4.11): `'/'(x: Integer, y: Integer): Rational` — an Integer
  quotient is a Rational — while `'+'`, `'-'`, `'*'`, `'%'`, `'**'`, `'^'` (Natural exponent)
  return `Integer`.
- **Classification follows the declared result, not the number held.** A function's result is
  classified by its declared return type (§8.4.4.9.2's rule for `LiteralRational`, applied to the
  library signatures above). So a whole-valued result of Rational arithmetic is still a
  `Rational` — `6 / 3` is the Rational `2`, `6 / 3 istype Integer` is `false` — and only an
  operation declared to return `Integer` (`floor`, `round`, `numer`, `denom`, `gcd`, Integer
  `+ - * % **`) yields one.
- **Real operations have no Rational result.** `sqrt`, the trigonometric functions, `exp`, `ln`
  and every function `RealFunctions` alone declares return `Real`; their results are in general
  irrational, so no Rational value is available to return. `RationalFunctions::'**'` declares
  `y: Rational` and `return : Rational`, but `(2 / 3) ** 0.5` is irrational: the declaration can
  hold only for an Integer exponent, which is what is evaluated exactly; any other exponent is the
  `RealFunctions::'**'` the declaration specializes.
- **Infinity.** §9.3.2.2.8 extends Rational with the two infinities. KerML's only notation for an
  infinite value is `LiteralInfinity` (`*`, §8.4.4.6, typed `Positive`); its existing rules apply
  unchanged to Rationals — `*` exceeds every finite Rational (`1 / 3 < *`), equals itself, and any
  arithmetic over it is a typed error. No finite Rational operation produces an infinity: a zero
  divisor is a division-by-zero error.

**What the specification is silent on**, and so is decided here rather than derived: the internal
representation, any bound on a Rational's size, the precision of `Real` (the text describes the
mathematical reals; it has no conformance clause on precision), how a Rational meets a `Real` that
an implementation holds approximately, and the text a Rational prints as. UML, fUML and PSSM say
nothing about KerML Rationals and were not consulted.

## What is implemented

- **Representation** (`internal/semantic/semantics/rational.go`). `ValRational` is held exactly, in
  lowest terms with a positive denominator. A Rational whose terms fit `int64` (denominator within
  32 bits) lives inline in `Value`; any other in an immutable `big.Rat`, so each Rational has one
  representation and `Value` stays within its 64-byte bound. Integer stays `ValInt` (`IntValue`/
  `BigIntValue`).
- **Literals.** `0.1`, `.5`, `1.5e3`, `25E-3`, `1.0e400` parse exactly (`semantics.ParseRational`);
  an integer literal stays an Integer. A literal declared `Real` — the value of an attribute typed
  `Real` — is held as the binary64 nearest it, which is that declaration's boundary (below).
- **Arithmetic** (`semantics.RatArith`, `RatPow`, `RatNeg`, `RatAbs`). `+ - * /` over Integer and
  Rational operands are exact; Integer `/` is the exact Rational quotient (`1 / 3` is `1/3`, which
  replaces the once-rounded `IntQuotient`); `**` and `^` with an Integer exponent are exact, a
  negative exponent inverting (`(2 / 3) ** -2` is `2.25`); `0 ** -1` is a domain error.
- **Comparison and equality** (`semantics.CompareRat`, `CompareReal`) are exact between exact
  values. Between a Rational and a binary64 Real they are at Real precision, the Rational rounded
  once; see [the rule](#comparing-a-rational-with-a-binary64-real) below.
- **Library functions** (`internal/exec/runtime/library_conversions.go`,
  `library_functions.go`). `rat`, `numer`, `denom`, `gcd`, `abs`, `floor`, `round`, `max`, `min`,
  `sum`, `product`, `ToString`, `ToRational`, `ToInteger` compute exactly over exact operands;
  `RationalFunctions::sum((0.1, 0.2))` is `0.3`. The `RealFunctions` versions take `Real`
  operands, so `RealFunctions::sum((0.1, 0.2))` is binary64 `0.30000000000000004`. A finite
  binary64 Real passed to a Rational function is the rational number that double is — every finite
  binary64 is one — so for `x : Real = 0.1`, `numer(x)` is `3602879701896397`: the declaration,
  not the function, rounded.
- **Size budget.** A Rational result whose numerator and denominator together need more bits than
  the numeric size budget (`DefaultMaxIntegerBits`, shared with Integers, settable per run) is
  `semantics.ErrRationalSizeLimit`, never rounded. A power is refused from a lower bound before it
  is computed. A long accumulation of distinct fractions — a harmonic sum — therefore stops with a
  typed error instead of growing without bound (`TestRuntimeRobustnessExactRational`).
- **Printing** (`Value.FormatRational`). A Rational whose denominator has no prime factor other
  than 2 and 5 is a terminating decimal and prints as one, in the layout a Real prints in (`0.3`,
  `2.0`, `1.5e-05`, `1e+400`); any other prints as `numer/denom` (`1/3`, `25/12`). So every value
  that was already a binary fraction (`0.5`, `1.5`, `2.0`) prints as before, and REPL, JSON and
  trace output change only where the old output was a rounding.
- **Quantities and units** (`semantics/units.go`, `quantity_eval.go`). A quantity's magnitude is
  whatever number it holds, exact or binary64. Unit scale factors are ratios, composed exactly
  (`Scale.Times`, `DividedBy`, `Pow`) while their terms are whole and within 2^53, so `0.1 [m] + 1
  [mm]` is exactly `0.101 [m]` and `1 [km] / 3` is `1/3 [km]`; a scale beyond that, or a conversion
  through a binary64 magnitude, is binary64 as before.
- **The Real boundary** (`semantics.RealOf`). A value written to a feature typed `Real`, the
  operands of `RealFunctions`, and a Rational meeting a binary64 Real in arithmetic are converted to
  the nearest binary64 once. A nonzero Rational below the least positive binary64 is an overflow
  error rather than a silent zero.

### Every boundary

- **gRPC** (`api/proto/sysml.proto`, `internal/frontend/protoconv`, `internal/frontend/grpc`). The
  service answers a Rational binary64 holds exactly as the existing `real_value`/`real_magnitude`,
  so old clients see what they saw. Any other crosses as the additive `Rational` message
  (numerator and denominator as decimal text) in `Value`, `Quantity` and `DocumentValue`,
  negotiated by the `rational_values` capability exactly as `big_int_values` negotiates
  `big_int_value`: a service that does not advertise it answers such a Rational as an unsupported
  value naming the capability, refuses a request or document query carrying or answering one with
  `UNIMPLEMENTED`, and never sends a nearest double. An answered Rational is canonical (positive
  denominator, lowest terms, not a binary64); an inbound one need only be in lowest terms over a
  positive denominator, and an inbound `real_value` is a binary64 Real (see
  [the wire rule](#a-rational-a-double-holds-on-input)).
- **JSON** (`internal/frontend/engine`, `internal/frontend/core`): `"rationalValue": {"numerator":
  "1", "denominator": "3"}` and `"rationalMagnitude"`, with the same canonical rule
  ([wire contract](../reference/wire-contract.md)).
- **Clients.** Go: `opensysml.Rational` over `*big.Rat`. Python: `fractions.Fraction`, a
  binary64-exact Rational arriving as `float`; generated typed classes declare a `Rational`
  feature `Fraction` and decode it exactly (`as_rational`). Node: `{kind: "rational", numerator:
  bigint, denominator: bigint}` with `rational()`, `formatRational()`, `rationalToNumber()` and
  `rationalOfDouble()`; generated classes declare `RationalValue` (`asRational`). Java and Rust
  carry an exact numerator/denominator pair, Julia `Rational{BigInt}`, MATLAB a struct of decimal
  strings. Each sends every exact Rational as `rational_value` under `rational_values`, rewrites
  one a double holds to `real_value` for a service without it and refuses any other, and refuses
  an answered Rational that is not canonical.
- **RDF** (`internal/translate/export/rdf_expr.go`). A literal is written as the exact Rational it
  denotes: a decimal token as `xsd:decimal`, an exponent token binary64 holds exactly as
  `xsd:double`, any other exponent token as its exact `xsd:decimal` (`1E-1` → `"0.1"^^xsd:decimal`).
  An imported `xsd:double`/`xsd:float` is the binary value it names. The round trip is exact.
- **SMT** (`internal/exec/solve`). The encoding was always exact; now the evaluator is too for
  Integer and Rational arithmetic, so replay (`solve/replay.go`) replays it exactly and only
  arithmetic a binary64 Real takes part in is replayed, and marked rounded (`Query.Rounded`,
  `RoundingSound`), as binary64. A declared `Real` variable is binary64 (`Var.Binary64`).
- **Compiled calcs** (`internal/translate/codegen/rational.go`, `sysml -compile`). Compiled code
  holds numbers as `int64` and binary64, so it compiles exact Rational arithmetic only where one
  binary64 rounding gives the exact answer: constants are folded exactly, a single operation over
  values binary64 holds exactly is compiled with guards, an Integer quotient compared with a whole
  number is compared exactly. Anything else — a `Rational` parameter, `a * 0.1` over a variable `a`,
  `(a * 0.5) ** 2`, `a / 3 < 0.1`, collection operations over exact Rationals — is refused at
  compile time with a typed error naming the construct, never compiled to a rounding.

## Comparing a Rational with a binary64 Real

With exact Rationals and binary64 Reals, `attribute x : Real = 0.1; x == 0.1` meets the double
nearest 1/10 (held by `x`, since a `Real` declaration rounds) with 1/10 itself. KerML has Rational
⊂ Real (§9.3.2.2.8) and so asks for the mathematical comparison of two reals; but the binary64
Real is this implementation's approximation (§9.3.2.2.9 leaves precision open), not the text's,
and the text does not say how an approximate Real meets an exact Rational. The rule is therefore
tool-defined, and chosen so the approximation does not give a wrong answer elsewhere:

- **A Rational meeting a binary64 Real is rounded once, to the nearest double, before it is
  compared** (`semantics.CompareReal`): `==`, `!=`, `<`, `<=`, `>`, `>=` between them compare
  `RealOf(rational)` with the Real. So `x == 0.1` is `true`, `x : Real = 1 / 3; x == 1 / 3` is
  `true`, `rat(1, 3) == 1.0 / 3.0` is `true` (`1.0 / 3.0` is exact `1/3` too) and `rat(1, 4) ==
  0.25` is `true`. A Rational whose magnitude no finite double holds compares as the infinity of
  its sign; a NaN compares as it does with any number (unordered, `!=` true).
- This is the rule mixed arithmetic already follows (the Rational is rounded once where a
  binary64 Real takes part), and the one a binding needs: KerML §7.4.9 asserts that both ends of
  a binding connector hold equal values, so a Rational bound to a `Real` feature — rounded by the
  declaration — must equal the Rational it was bound from. Exact comparison would make that
  binding's own equality `false`, a wrong answer rather than a choice
  (`instance_real_binding_meets_rational`).
- Rational with Rational stays exact (`0.1 + 0.2 == 0.3` is `true`), and so does Integer with
  anything: an Integer keeps its kind in a `Real` feature, so `CompareIntReal` compares a large
  Integer with a Real exactly as before.
- Equality of values (sets, `includes`, query `==`, solver witness replay, compiled `==`) uses the
  same rule, so a Rational and a Real that compare equal are one member of a set.

A long `Real`-typed accumulation of a decimal literal (`x := x + 0.01` a thousand times, `x :
Real`) is binary64 at every step and so never reaches `ErrRationalSizeLimit`
(`action_real_accumulation_stays_binary64`, and `TestRuntimeRobustnessRationalRealComparison`
under a lowered budget); the same accumulation over exact Rationals is exact and stays within the
budget only while its denominators do.

## A Rational a double holds, on input

The service answers a Rational binary64 holds exactly (`1/4`) as `real_value`, so its answers
never spell one number two ways. KerML says nothing about a wire, so how a client tells the
service that a `0.25` it sends is the Rational rather than the Real is tool-defined:

- **Clients send every exact Rational as `rational_value`** to a service that advertises
  `rational_values`, one a double holds included (`Fraction(1, 4)`, `1//4`, `Rational.of(1, 4)`).
- **The service accepts a binary64-exact `rational_value` on input** as that exact Rational
  (`protoconv.LowestTermsRational`; still refused unless in lowest terms over a positive
  denominator), while every answer stays canonical (`CanonicalRational`): `in x : Rational; x +
  1 / 3` with `x` sent as the `rational_value` `1/4` is exact `7/12`.
- **An inbound `real_value` is always a Real**: the same `x` sent as `real_value` `0.25` is
  binary64 `0.5833333333333333`. The declaration does not reinterpret it.
- Runtime semantics do not depend on the encoding beyond that: once read, an exact Rational is
  the same value whether it arrived on the wire or was written in the model.
- To a service without `rational_values`, which would read the arm as unknown, a client sends a
  Rational a double holds exactly as that `real_value` — the only form such a service reads — and
  refuses any other Rational before the call.

Both directions are covered by the gRPC conformance cases (`execute_action_rational_input_meets_
rational`, `execute_action_real_input_meets_rational`, `evaluate_calc_rational_wire`,
`evaluate_calc_rational_wire_canonical_answer`), `internal/frontend/grpc/convert_rational_test.go`
and each client's tests (`client/python/tests/test_rational.py` against a live service).

## The pilot differs by design

The pinned pilot (`jupyter-sysml-kernel`, provisioned by `scripts/download-pilot-validator.sh` +
`scripts/download-pilot-evaluator.sh`) holds a `LiteralRational` as a Java `double`
(`LiteralRationalImpl.value`: `protected double value`). Its answers, verbatim, beside ours:

| Case | Pilot answer | OpenSysML |
|------|--------------|-----------|
| `0.1 + 0.2` | `LiteralRational 0.30000000000000004` | `0.3` |
| `0.1 + 0.2 == 0.3` | `LiteralBoolean false` | `true` |
| `0.1 + 0.2 <= 0.3` | `LiteralBoolean false` | `true` |
| `0.3 < 0.1 + 0.2` | `LiteralBoolean true` | `false` |
| `0.1 + 0.2 - 0.3` | `LiteralRational 5.551115123125783E-17` | `0.0` |
| `(1.0 / 49.0) * 49.0 == 1.0` | `LiteralBoolean false` | `true` |
| `(1.0 / 3.0) * 3.0 == 1.0` | `LiteralBoolean true` (double rounding happens to land on 1.0) | `true` |
| `0.1` ten-fold sum `== 1.0` | `LiteralBoolean false` | `true` |
| `1.0 / 3.0`, `1 / 3` | `LiteralRational 0.3333333333333333` | `1/3` |

Every pilot row is the binary64 answer, `5.551115123125783E-17` being exactly the double `0.1 +
0.2 - 0.3`; every OpenSysML row is the rational one §9.3.2.2.8 and §8.4.4.9.2 define. (The pilot's
integer literals are narrower still: `9007199254740993` answers `ERROR:For input string:
"9007199254740993"`, a 32-bit `parseInt`.) The probes are committed as
`tools/referee/exec/testdata/cases/exact_rationals.cases`, marked `by-design` with those clauses:
the five whose answers differ land in the [execution referee's](pilot-execution-referee.md)
`differs-by-design` bucket, the clause recorded beside both raw outputs, and the other five agree
(the referee compares reals to two decimal places). None is hidden or normalized away.

## RationalFunctions::rat, numer and denom

`rat(numer: Integer, denum: Integer): Rational`, `numer(rat: Rational): Integer` and
`denom(rat: Rational): Integer` presuppose an exact ratio, and now have one:

- `rat(n, d)` is the Rational `n/d` in lowest terms, the value `n / d` computes: `rat(1, 3)` is
  `1/3`, `rat(6, 4)` is `1.5`, `rat(1, 0)` is the typed division-by-zero error `1 / 0` reports,
  never an infinity. The library's `RationalFunctions::sum`/`product` bodies start from
  `rat(0, 1)`/`rat(1, 1)`.
- `numer(x)` and `denom(x)` are the numerator and positive denominator of `x` in lowest terms; an
  Integer is itself over `1`. `numer(0.1)` is `1` and `denom(0.1)` `10`, `numer(rat(1, 3))` is
  `1` and `denom(1.0 / 3.0)` `3`, `denom(0.0001)` is `10000`, and `rat(numer(x), denom(x)) == x`
  holds for every finite `x`. A binary64 Real argument is the Rational it is exactly
  (`big.Rat.SetFloat64`); an infinity or NaN has no ratio and is `semantics.ErrArithmeticDomain`.

The pilot implements none of the three. Each call, qualified at the prompt and as an attribute's
value, answers the unevaluated invocation (UUIDs elided):

| Case | Pilot answer |
|------|--------------|
| `RationalFunctions::rat(1, 3)` | `InvocationExpression rat` |
| `RationalFunctions::rat(6, 4)` | `InvocationExpression rat` |
| `RationalFunctions::rat(1, 0)` | `InvocationExpression rat` |
| `RationalFunctions::numer(0.75)`, `denom(0.75)` | `InvocationExpression numer`, `InvocationExpression denom` |
| `RationalFunctions::numer(RationalFunctions::rat(6, 4))` | `InvocationExpression numer` |
| `RationalFunctions::denom(1.0 / 3.0)` | `InvocationExpression denom` |
| `RationalFunctions::numer(2)`, `denom(2)` | `InvocationExpression numer`, `InvocationExpression denom` |
| `RationalFunctions::numer(0.1)`, `denom(0.1)` | `InvocationExpression numer`, `InvocationExpression denom` |
| `RationalFunctions::rat(1, 3) == 1.0 / 3.0` | `LiteralBoolean false` |
| `RationalFunctions::rat(1, 3) == RationalFunctions::rat(1, 3)` | `LiteralBoolean false` |
| `RationalFunctions::rat(1, 3) != RationalFunctions::rat(1, 3)` | `LiteralBoolean true` |
| `6 / 4` (operator syntax) | `LiteralRational 1.5` |
| `1 / 0` | no output |

The `==`/`!=` rows are not verdicts: the pilot compares two *unevaluated* invocation nodes
and finds them unequal, so `rat(1, 3)` is "not equal" even to itself. That is the same
unevaluated-operand artifact `w6d:complex-is-zero-qualified` records in the
[execution referee](pilot-execution-referee.md). `RationalFunctions::abs`, `floor` and
`'/'` called by name are unevaluated too; only operator syntax folds. So the pilot cannot
referee any of the three, and the semantics are self-assessed. The
probes are committed as `tools/referee/exec/testdata/cases/rational_terms.cases`, where
every call lands in `pilot-unevaluated` and the operator quotient agrees.


## Cost

Small Rationals are held inline — an `int64` numerator over a `uint32` denominator in the
`Value` itself — and only terms beyond that use a normalized `math/big.Rat`, so the `Value` stays
32 bytes and ordinary arithmetic allocates nothing it did not allocate as binary64. Literals, a
double converted exactly, and Integer arithmetic beyond `int64` take the same allocation-free paths.

Interleaved runs on one machine (8 cores, six samples each side, `benchstat`), against the
binary64 implementation:

| Benchmark | Time | Bytes/op | Allocs/op |
|-----------|------|----------|-----------|
| `internal/exec/runtime`, all seven existing benchmarks | geomean +2.1%, no significant change | geomean +3.5% | unchanged |
| `IntegerArithmeticLoopBeyondInt64` | no significant change | +27% (227.6 KiB → 290.1 KiB) | unchanged |
| `tests/perf` (`REPLEvalExpr`, `ExecuteAction`, `BatchConstraints`, `SameConstraintManyInstances`, `BatchSatisfy`, `Instantiate`, `GRPCEvaluate`) | no significant change on any | at most +2.8% (`GRPCEvaluate`) | unchanged |
| `RationalDecimalLoop` (1,000 steps of exact decimal arithmetic) | no significant change | unchanged | unchanged |
| `RealDecimalLoop` (the same loop over a feature declared `Real`) | no significant change | unchanged | unchanged |
| `RationalHarmonicLoop` (200 exact terms of 1/i) | +47% to +55% | ×4.3 | +69% |

```
go test ./internal/exec/runtime -run '^$' -bench . -benchmem -count=6
go test ./tests/perf -run '^$' -bench 'Instantiate|BatchConstraints|BatchSatisfy|ExecuteAction$|GRPCEvaluate|REPLEvalExpr|SameConstraint' -benchtime=20x -benchmem -count=6
```

An Integer beyond `int64` is the numerator of a `big.Rat` over one, which is the byte increase in
`IntegerArithmeticLoopBeyondInt64`. The harmonic sum is what exactness costs where it is real:
its denominator outgrows `int64` within a few dozen terms and reaches hundreds of bits, where
binary64 kept 53. An accumulation whose terms keep growing is stopped by
`ErrRationalSizeLimit` at the configured size budget rather than slowing without bound.

## The rounded-query census

The solver reasons over SMT-LIB's exact `Real` sort, so a query is marked rounded
(`Query.Rounded`) where the evaluator rounds: an exact-real `unsat` about it is reported
undecided, and `%configure all` and `%optimize` decline the completeness claim (see
[spec-compliance](spec-compliance.md#exact-reals-against-a-rounding-evaluator--what-agreement-is-claimed)).
With exact Rationals only arithmetic a binary64 `Real` takes part in still rounds.
`TestRoundedCensus` (`internal/exec/solve/rounded_census_test.go`) translates every constraint,
requirement, satisfaction assertion and analysis case in the repository's solver-facing corpora
through the path `%check`/`%solve`/`%configure all`/`%optimize` use, and counts the marked ones:

```
OPENSYSML_SMT=/usr/bin/z3        go test -count=1 -run TestRoundedCensus -v ./internal/exec/solve
OPENSYSML_SMT=/usr/local/bin/cvc5 go test -count=1 -run TestRoundedCensus -v ./internal/exec/solve
```

| Corpus | Files | Translated queries | Rounded, binary64 Rationals | Rounded, exact Rationals |
|--------|-------|--------------------|-----------------------------|--------------------------|
| Training corpus | 100 | 18 | 3 | 0 |
| Pilot corpora | 213 | 95 | 9 | 2 |
| Conformance fixtures | 1,361 | 17 | 0 | 0 |
| Examples + manual | 53 | 177 | 17 | 3 |
| Solver fixtures | 10 | 65 | 7 | 1 |
| Standard library | 107 | 203 | 0 | 0 |
| **Total** | **1,844** | **575** | **36** | **6** |

Both solvers give the same counts. Thirty of the 36 queries a binary64 Rational marked were
rounded only by a decimal literal or an Integer quotient, which is now exact, so their `unsat`
verdicts are reported as verdicts. The six left are genuinely binary64 — arithmetic over a
feature declared `Real` (`powerMargin` in `examples/verdicts-demo/rover.sysml`, `pwr` and `acc` in
the pilot's `HSUVDynamics.sysml`) or a quotient of one (`MassPerCrate` in the solver fixtures) —
and none of them is provably exact.
