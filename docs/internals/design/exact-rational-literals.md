# Exact Rational values beside a binary64 Real

A proposal, not a change: what it would take for a real literal and a `Rational`-typed
feature to hold an exact ratio while `Real` stays IEEE 754 binary64, and whether to do
it. Nothing here is implemented; the evaluator's arithmetic is as
[the adjudication record](../../project/exact-rational-evaluation.md) left it. That record
declined a *uniformly* exact evaluator; this note takes up the narrower design it set aside
as option (b) and reframes it by type rather than by consumer, because a second reference
implementation now computes that way.

## Where a number lives today

- **One value type, one real representation.** `semantics.Value`
  (`internal/semantic/semantics/eval.go`) is `{Kind, Bool, Int int64, Real float64, big *big.Int}`:
  an Integer is exact and unbounded, a Real or Rational is a `float64`. The runtime's
  `ValConst` (`internal/exec/runtime/value.go`) wraps that value; a quantity's magnitude
  (`ValQuantity`), a complex part, a vector component and a tensor component are `float64` too.
- **A literal rounds at parse time.** `ParseReal` is `strconv.ParseFloat(text, 64)`;
  the model-level folder (`evalConst`) and the runtime's `evalLiteralReal`, `compile.go` and
  `codegen/compile.go` all read a `LiteralReal` through it, so `0.1` is the nearest double
  before any operator sees it.
- **Arithmetic is binary64.** `RealArith(op, a, b float64)` carries `+ - * / **`;
  `IntQuotient` makes `1/3` by dividing the exact Integers and rounding once;
  comparisons and `==` compare doubles. `FormatReal` (`quantity.go`) prints the shortest
  decimal that reads back as the same double, which is why `0.1 + 0.2` prints
  `0.30000000000000004`, `0.1 + 0.2 == 0.3` prints `false` and `1/3` prints
  `0.3333333333333333`.
- **The type lattice already knows Rational.** `exprtype.go` ranks
  `Natural < Integer < Rational < Real < Number` (`PrimRational`), the write check admits a
  `LiteralReal` into a `Rational` feature (`valuetype.go`, conformance against
  `ScalarValues::Rational`), and the library operators bind an Integer or a Real as a
  Rational (`library_operators.go` `rationalOperand`). `RationalFunctions::rat`, `numer` and
  `denom` are implemented over the double a Rational is (`big.Rat.SetFloat64`), so
  `denom(0.1)` is `2^55`.
- **The wire carries a double.** `api/proto/sysml.proto` has `double real_value`,
  `double real_magnitude` and `double` quantity scales; the Python client documents
  `Real, Rational → float`.

## What the two references do

- **The pinned pilot** (`jupyter-sysml-kernel` 0.60.1) stores a `LiteralRational` as a Java
  `double` and answers every probe in binary64 — `0.30000000000000004`, `false`,
  `0.3333333333333333` — digit for digit what OpenSysML answers. The adjudication record
  holds the probe table and the `javap` evidence.
- **sysml-toolkit v0.10.0** evaluates with exact rationals: the same three expressions
  answer `0.3`, `true` and `1/3`. Its arithmetic is the mathematical one the KerML text
  describes; the pilot's is the one the only OMG-published executable oracle performs.

KerML itself is silent on precision (§9.3.2.2.8–9, §8.4.4.9.2: a `LiteralRational` is
*classified* as `Rational`; `RationalFunctions` declares signatures only). Either reference
is conformant. The choice is a conformance posture, not a defect to fix.

## The typed design

A new value kind, `ValRat` holding a `*big.Rat`, beside `ValReal`, with exactness a property
of the **value**, introduced by exactly two sources and removed by exactly two sinks:

| | Produces an exact value | Produces a binary64 |
|---|---|---|
| Literal | `0.1`, `1/3`, `rat(1, 3)` | — |
| Field operation | `Rat ⊕ Rat`, `Rat ⊕ Integer` for `+ - * /`, comparison, `==` | any operand already binary64 |
| Function | `abs`, `floor`, `round`, `max`, `min`, `numer`, `denom`, `**` with an Integer exponent | `sqrt`, trig, `exp`, `ln`, `**` with a fractional exponent — the irrational remainder |
| Binding | a `Rational`, `Number` or untyped feature keeps the value as it is | a `Real` feature, a quantity magnitude, a vector or tensor component rounds once |

Under this rule `0.1 + 0.2 == 0.3` at the prompt is `true` and `1/3` prints `1/3` (the
toolkit's answers), while `attribute m : Real = 0.1; m + 0.2 == 0.3` stays `false` (the
pilot's), and every ISQ quantity — whose magnitude is a `Real` — computes exactly as today.
The `Real` type is the sole place rounding enters beyond the irrational functions, which is
what the specification's type distinction suggests and what makes the hybrid a contract a
reader can state in one sentence.

## What it would cost

Measured against the tree, not estimated from the idea:

- **Semantics** (`internal/semantic/semantics`): `Value` gains a kind; `evalConst`,
  `RealArith`, `IntQuotient`, `Pow`, the comparison and equality folds and the widening
  lattice gain a `Rat` arm (22 `ValReal`/`.Real` sites in `eval.go`, more in `integer.go`,
  `quantity.go`, `units.go`, `dimension.go`). `FormatConst` needs a decision for a Rat:
  the toolkit prints `1/3`; a decimal when the denominator is `2^a·5^b` and `n/d`
  otherwise is the readable choice, and `ToString` must round-trip through `ToRational`.
- **Runtime** (`internal/exec/runtime`): `arithmeticValues`, `comparisonValues`,
  `equalityValues`, `toReal`, the library operator and conversion tables (`rationalOperand`,
  `rat`/`numer`/`denom` become the exact functions the spec names), the write check that
  rounds on binding to `Real`, the sweep's value typing, `FormatValue`; 40 files in the
  package mention `float64`, most of them (quantities, vectors, clocks, random draws) stay
  binary64 untouched because their values are `Real`.
- **Wire and clients**: a `rational_value { bytes numerator; bytes denominator }` form in
  `sysml.proto` beside `real_value`, the Go service and the Python client (`fractions.Fraction`
  or `float`, documented), the REPL's and the CLI's printing contracts.
- **Fixtures**: every conformance `.expected.json`, golden trace and REPL golden that prints
  a value computed from literals through untyped or `Rational` features — a reading of
  them is the first task, since a fixture that prints `0.30000000000000004` is pinning the
  pilot's answer deliberately.
- **The pilot referee** (`tools/referee/exec`): every probe row in the adjudication record
  becomes a disagreement with the pilot. The record's own standard — agreement with the
  pinned pilot wherever it can speak — would have to be amended to "agreement where the
  pilot is the only reference, the toolkit's exact answer where the pilot's is an artifact
  of its `double`". That is a project decision, not an engineering one.
- **The solver seam**: an exact value makes `Query.Rounded` *less* conservative — a term over
  exact literals and `Rational` features is exact over its whole value set — so the
  hybrid narrows the undecided surface the record's option (d) left unbuilt, at no change
  to the replay contract: a witness is still replayed through the evaluator's own arithmetic.
- **Performance**: `big.Rat` field operations are three to four orders of magnitude slower
  than `float64` and accumulation grows denominators without bound (the record measures
  both). The typed design confines that to `Rational`, `Number` and untyped features, so a
  simulation loop over `Real` quantities is unaffected — but an untyped
  `attribute total = 0.0;` accumulated in a loop becomes the pathological case, and the
  inference that makes it `Rational` is invisible to the author. A denominator bound with a
  fall-back to binary64, or inferring `Real` for an untyped feature whose value is a real
  literal, would each have to be chosen and documented.

Roughly: the semantics and runtime arms are a few hundred lines each plus their tests, the
wire form and clients a day, the fixture adjudication and the referee amendment the larger
and less predictable part.

## Recommendation

1. **Do not change the arithmetic now.** The adjudication record's decision stands on its
   own evidence: the pilot computes in binary64, the specification is silent, and the probes
   agree digit for digit. Divergence from the toolkit on `0.1 + 0.2` is a difference between
   two conformant implementations, and this document records why it exists.
2. **If the project adopts toolkit parity as a goal**, implement the typed design above, in
   this order: `ValRat` in `semantics` with the folder and `FormatConst` (model-level `-e`
   answers change first, behind the existing referee); the runtime arms and the `Real`
   binding sink; the wire form and clients; then the fixture reading, the referee
   normalization and an amendment to the adjudication record stating the new posture.
   Each step is a PR with its own conformance cases under
   `internal/exec/runtime/testdata/conformance/`.
3. **Independently of either**, two small things are worth doing: make `ExprResultType`
   report a `LiteralReal` as `Rational`, as the write check and KerML §8.4.4.9.2 already do
   (`valuetype.go` reports `Real` today), so the two type queries agree; and decide the
   untyped-literal inference rule in the guide, since it determines which features the hybrid
   would make exact: `attribute x = 0.1;` is `Real` today, the feature inheriting its value's
   `ExprResultType`, and would become `Rational` once that query follows the lattice.
