# Exact-rational evaluation: adjudicated and declined

The solver work left one asymmetry standing: the evaluator computes `Real`/`Rational`
arithmetic in IEEE 754 binary64 while the SMT translation reasons over the exact `Real`
(rational) sort, so the two can disagree wherever a value is not float64-representable.
The [solver soundness record](spec-compliance.md#exact-reals-against-a-rounding-evaluator--what-agreement-is-claimed) made the
*verdict* sound rather than exact — sat witnesses are replayed through the evaluator's
own arithmetic, and a query the evaluator rounds does not report an exact-real `unsat`
as an evaluator verdict — at the cost of completeness: rounded queries answer undecided
where a float64-aware solver might have decided them.

The remaining way to close that gap would be to make the evaluator itself exact — a
`big.Rat`-backed value representation, so there is nothing for the solver to disagree
with. This record adjudicates that change: what the reference implementation actually
computes, what the specification actually requires, what the change would touch, what
it would cost, and why it is **declined**.

## What the reference computes: binary64, observably and structurally

The pinned pilot (`jupyter-sysml-kernel` 0.60.1, provisioned by
`scripts/download-pilot-validator.sh` + `scripts/download-pilot-evaluator.sh`, driven
by `build/pilot-evaluator/eval-sysml --cases`) was probed with cases chosen to
distinguish exact-rational from binary64 evaluation. Its answers, verbatim:

| Case | Pilot answer | Exact-rational answer would be |
|------|--------------|--------------------------------|
| `0.1 + 0.2` | `LiteralRational 0.30000000000000004` | `0.3` |
| `0.1 + 0.2 == 0.3` | `LiteralBoolean false` | `true` |
| `0.1 + 0.2 <= 0.3` | `LiteralBoolean false` | `true` |
| `0.3 < 0.1 + 0.2` | `LiteralBoolean true` | `false` |
| `0.1 + 0.2 - 0.3` | `LiteralRational 5.551115123125783E-17` | `0` |
| `(1.0 / 49.0) * 49.0 == 1.0` | `LiteralBoolean false` | `true` |
| `(1.0 / 3.0) * 3.0 == 1.0` | `LiteralBoolean true` (double rounding happens to land on 1.0) | `true` |
| `0.1` ten-fold sum `== 1.0` | `LiteralBoolean false` | `true` |
| `1.0 / 3.0`, `1 / 3` | `LiteralRational 0.3333333333333333` | an exact third |

`5.551115123125783E-17` is exactly the binary64 value of `0.1 + 0.2 - 0.3`; every row is
the IEEE 754 double answer, none is the exact-rational one. The evidence is structural
too: in the pinned jar, `LiteralRationalImpl.value` is a Java `double`
(`javap`: `protected double value; public double getValue(); public void setValue(double)`),
so a rational literal is rounded to binary64 at parse time, before any arithmetic runs.
(Its integer literals are narrower still: `9007199254740993` answers
`ERROR:For input string: "9007199254740993"` — a Java 32-bit `parseInt`, the limit the
division work had already recorded.)

OpenSysML today answers **identically on every one of these probes** (`0.30000000000000004`,
`false`, `false`, `true`, `5.551115123125783e-17`, `false`, `true`, `false`,
`0.3333333333333333`). An exact-rational evaluator would therefore not close a gap with
the reference — it would **open one**, flipping the observable answer of every probe row
above against the pilot, and `tools/referee/exec` would report each as a disagreement.
This inverts the premise of the change: the evaluator's binary64 arithmetic *is* the
reference behavior.

## What the specification requires: nothing about precision

KerML 1.0 (formal/2025-02-01; the 1.1 RTF has not published changes to these clauses)
describes the data types mathematically:

- §9.3.2.2.8 Rational: "Rational is the type of rational numbers, extended with values
  for positive and negative infinity."
- §9.3.2.2.9 Real: "Real is the type of mathematical (extended) real numbers. This
  includes both rational and irrational numbers, and values for positive and negative
  infinity."
- §8.3.4.8.13 LiteralRational: the abstract-syntax `value` attribute is typed `Real`
  ("The value whose rational approximation is the result of evaluating this
  LiteralRational"), and §8.4.4.9.2 notes that "only the rational-number subset of the
  real numbers can be represented using a finite literal. So the result of a
  LiteralRational is actually always classified in the KerML DataType Rational."

That is a statement about what the *values are*, not about what arithmetic an
implementation must perform. The Kernel Function Library (§9.4; `RealFunctions.kerml`,
`RationalFunctions.kerml`) declares only signatures — `function '+' … in x: Real[1]; in
y: Real[0..1]; return : Real[1];` — with no precision, rounding, or exactness clause,
and no conformance clause elsewhere in the specification constrains numeric precision.
The spec is **silent on precision**; the reference implementation chose binary64. There
is no clause an exact-rational evaluator could point to as mandating it, and the only
executable oracle contradicts it.

## What the change would touch

For completeness of the adjudication, the blast radius of a `big.Rat`-backed (or
exact-until-formatted) `Real`/`Rational` value, mapped concretely:

- `internal/semantic/semantics`: `Value` carries `Real float64` (`eval.go`); the constant
  folder's `evalRealArith`/`RealArith`, `IntQuotient`, `Pow`, comparisons and equality,
  and the numeric-widening lattice all move to a rational representation.
- `internal/exec/runtime`: the evaluator (`eval.go`, `toReal`), `value.go`
  (`FormatReal` and all printing), `library_functions.go` (34 `math.*` call sites —
  `sqrt`, trig, `floor`/`round`, `exp`/`ln` — which have no exact form), quantities and
  unit scaling, collections, overflow handling; 69 `float64` sites in the package.
- Wire and clients: `api/proto/sysml.proto` carries `double real_value`,
  `double real_magnitude`, unit `scale_num`/`scale_den`, `double exponent`; an exact
  value needs a new wire form and migrations in the Go service and the Python client
  (`Real, Rational → float` is a documented mapping in the Python API reference).
- Fixtures: conformance `.expected.json` files and golden traces that print reals, the
  REPL output contract, and the pilot execution referee's normalization (which today
  matches the pilot digit-for-digit because both sides are binary64).
- The solver seam: `solve/replay.go` — the witness replay and `Query.Rounded` marking —
  encodes the evaluator's rounding; an exact evaluator rewrites that contract and its
  tests (`TestSolvedWitnessRejectedByEvaluatorIsUndecided`,
  `TestRoundedMarksFloatComputingQueries`, …), the very behavior the soundness work
  just pinned down.
- The irrational remainder: `sqrt`, trig, `**` with a fractional exponent and the
  transcendentals cannot be exact, so the result is necessarily a **hybrid** — exact
  for the field operations, rounded for irrational functions — and any query touching
  an irrational operation re-enters exactly the rounded-query incompleteness this
  change was meant to remove. Exactness only ever covers the rational-closed fragment.

## What it would cost, measured

Microbenchmarks on this machine (Go 1.23, `math/big`; float64 loop vs the equivalent
`big.Rat` loop, reusing allocated `Rat`s):

| Workload | float64 | big.Rat | Ratio |
|----------|---------|---------|-------|
| mul + quo + add per iteration | 0.33 ns/op | 731 ns/op | ~2,200× |
| accumulating sums of distinct small fractions | 0.30 ns/op | 10,380 ns/op | ~35,000× |

The second row is the structural problem, not just a constant factor: exact rational
accumulation grows denominators without bound (the running sum's denominator tends
toward the lcm of everything added), so operand size — and per-operation cost and
memory — grows with the computation. A simulation loop accumulating quantities is the
runtime's hot path.

## The options, compared

| Option | Solver agreement bought | Pilot agreement | Cost |
|--------|------------------------|-----------------|------|
| (a) Full exact-rational values | Closes the gap on the rational-closed fragment only; irrational ops re-open it | **Diverges** on every probe row above | Every package listed above, a wire-format migration, fixture rewrites, 3–4 orders of magnitude on numeric hot paths |
| (b) Exact-rational only where it closes the solver gap, binary64 elsewhere | Same fragment as (a) | Diverges wherever the exact path is used | The same expression yields different values depending on whether a solver looks at it — a worse contract than either uniform choice |
| (c) Keep binary64 + sound-but-incomplete verdicts (status quo) | Sound verdicts; rounded queries stay undecided | **Agrees** (verified digit-for-digit on the probes) | Zero; already landed and tested |
| (d) Narrow `Query.Rounded` by proving exactness per term | Recovers decided verdicts for provably-exact float64 computations (dyadic constants, small-magnitude sums) without touching the value representation | Agrees (no evaluator change) | Contained in `solve/replay.go` `roundedTerm`; subtle — a term over free Real variables can round on some witness values and not others, so unmarking is only sound for terms whose *whole value set* is exact |

## Decision

**Declined — the evaluator stays binary64; option (c) stands.** The reference
implementation computes in binary64 (behaviorally and structurally verified above), the
specification is silent on precision, and this project's conformance posture is
agreement with the pinned pilot wherever it can speak. An exact-rational evaluator
would diverge from the reference on observable answers, cost measured orders of
magnitude on hot paths, force a hybrid contract that re-admits the same solver gap at
every irrational operation, and rewrite the wire format and the just-landed soundness
seam. The incompleteness it would remove is narrow (rounded queries answer undecided
rather than wrong) and already documented as the deliberate trade.

Option (d) is recorded as the one refinement worth holding open: it narrows the
undecided surface without moving the value contract or the pilot agreement, and it is
contained in one function — but it is subtle enough (exactness must hold for a term's
whole value set, not one witness) that it should wait until the undecided verdicts are
observed to bite in practice.

## RationalFunctions::rat, numer and denom over a binary64 Rational

The Kernel Function Library declares three functions that presuppose an exact ratio:
`rat(numer: Integer, denum: Integer): Rational`, `numer(rat: Rational): Integer` and
`denom(rat: Rational): Integer`. They were registered as unevaluable, citing this record,
and that verdict was re-adjudicated: not whether to make the evaluator exact — the
decision above stands — but what the three should compute *given* a binary64 Rational.

The pinned pilot (`jupyter-sysml-kernel` 0.60.1) was probed first, each call written both
qualified at the prompt and as the value of a model attribute. Its answers, verbatim
(UUIDs elided), are all the unevaluated invocation — it has no implementation of any of
the three:

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
referee any of the three, the standing decision is not contradicted (nothing here shows an
exact numerator/denominator pair for `1/3`), and the semantics are self-assessed. The
probes are committed as `tools/referee/exec/testdata/cases/rational_terms.cases`, where
every call lands in `pilot-unevaluated` and the operator quotient agrees.

**Adjudicated: implement all three over the binary64 the runtime already holds.**

- `rat(numer, denum)` is the binary64 quotient — the value `RationalFunctions::'/'` and
  the `/` operator compute, the exact Integer ratio rounded once (`semantics.IntQuotient`).
  `rat(1, 0)` is the typed division-by-zero error `1 / 0` reports, never an infinity. The
  library's own `RationalFunctions::sum`/`product` bodies start from `rat(0, 1)`/`rat(1, 1)`,
  which is one more reason the constructor must have a value.
- `numer(rat)` and `denom(rat)` are the exact numerator and positive denominator, in
  lowest terms, of the rational the binary64 *is* (`math/big.Rat.SetFloat64`, which is
  exact for every finite double); an Integer is itself over `1`. So `numer(0.75) = 3`,
  `denom(0.75) = 4`, `numer(-0.75) = -3`, a whole value reads as `n/1` (`numer(2) = 2`,
  `denom(2.0) = 1`, `denom(0.0) = 1`), and `rat(numer(x), denom(x)) == x` holds exactly
  for every finite `x` whose terms are Integers. A term past the Integer range — `denom(0.0001)` is `2^66`,
  `numer(1.0e19)` — is `semantics.ErrArithmeticOverflow`; an infinity or NaN has no finite
  ratio and is `semantics.ErrArithmeticDomain`.

**The binary64 consequence, stated plainly.** A Rational here holds the double nearest
the value written, so `numer`/`denom` answer the terms of that double, not of the
rational the model meant: `numer(rat(1, 3))` is `6004799503160661` and `denom(rat(1, 3))`
is `2^54 = 18014398509481984`; `numer(0.1)` is `3602879701896397` and `denom(0.1)` is
`2^55 = 36028797018963968`. The pure-spec answers — `1` and `3`, `1` and `10` — need an
exact Rational value kind, which this record declines. The 16-digit numerator is the same
class of artifact as `0.1 + 0.2 != 0.3` in the probe table at the top: a faithful
reading of the binary64 the reference implementation also stores (`LiteralRationalImpl.value`
is a Java `double`), accepted as pilot parity rather than hidden behind a decimal-derived
pair that `rat` could not reproduce. The alternatives were weighed and set aside: the
reduced pair of the shortest decimal rendering (`numer(0.1) = 1`, `denom(0.1) = 10`) would
make `rat(numer(x), denom(x))` a different double from `x` for most `x`, breaking the one
identity the three functions owe each other; keeping them unevaluable would leave the
library's `sum`/`product` bodies without a starting value and report a typed error where
the runtime can honestly compute one.

## Option (d) measured: the rounded-query census

Whether the narrowing is worth building is an empirical question — how many queries
does the conservative `Query.Rounded` marker sweep in that are in fact provably exact?
`TestRoundedCensus` (`internal/exec/solve/rounded_census_test.go`) answers it
reproducibly: it enumerates every constraint, requirement and analysis case in the
repository's solver-facing corpora, translates each through the same
`Condition`/`Analysis` path the REPL's `%check`/`%solve`/`%configure all`/`%optimize`
commands use, and classifies every translated query.

```
OPENSYSML_SMT=/usr/bin/z3        go test -count=1 -run TestRoundedCensus -v ./internal/exec/solve
OPENSYSML_SMT=/usr/local/bin/cvc5 go test -count=1 -run TestRoundedCensus -v ./internal/exec/solve
```

A marked query is *recoverable* only if every asserted or optimized term is exact over
its **whole value set**: exact-float64 real literals, and real-valued arithmetic only
when it folds to a constant whose every intermediate is exactly representable.
Anything over free variables — real arithmetic, division, integer→real widening —
stays conservative, because a witness-dependent value set cannot be proven exact
statically.

Counted (both solvers give identical numbers): the training corpus, the three pilot
corpora, the runtime conformance fixtures, the repository examples and manual
examples, the solver test fixtures, and the bundled standard library — 890 files,
2,170 candidate elements, 415 translated queries. Excluded: elements the translator
refuses (unsupported operations, unresolved names), which never reach the solver and
so never see the marker.

| Corpus | Files | Translated queries | Marked rounded | Provably exact |
|--------|-------|--------------------|----------------|----------------|
| Training corpus | 100 | 13 | 2 | 0 |
| Pilot corpora | 212 | 82 | 8 | 0 |
| Conformance fixtures | 444 | 17 | 0 | 0 |
| Examples + manual | 27 | 56 | 4 | 0 |
| Solver fixtures | 10 | 54 | 7 | 0 |
| Standard library | 97 | 193 | 0 | 0 |
| **Total** | **890** | **415** | **21** | **0** |

Every one of the 21 marked queries is genuinely inexact: 4 contain a real literal with
no exact float64 (e.g. a `0.4` efficiency, a degree-unit scale factor), and 17 perform
real arithmetic, division, or integer widening over free variables — value sets no static
analysis can prove exact. The false-undecided rate over the real corpus is **zero**.

**Adjudication: option (d) stays unbuilt.** The narrowing would recover no verdict in
any realistic model in the repository; the recoverable class (constant-folded dyadic
arithmetic asserted directly) does not occur in practice, because models constrain
free attributes, not constants. The census harness remains as the reproducible
instrument: re-run it if future corpora accumulate undecided verdicts, and revisit
only if it reports a material provably-exact population.
